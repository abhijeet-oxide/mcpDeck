// Version history as a timeline (VER-4). Pick a version to see what it
// changed against the one before it (or the live or latest one), and
// restore it as a new version.
import { History, RotateCcw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { post } from "../lib/api";
import { dateTime } from "../lib/format";
import { useAction } from "../lib/hooks";
import type { SaveResult, Version } from "../lib/types";
import { diffLines } from "./Diff";
import { Button, Chip, ErrorBox, Select, StatusChip, useConfirm, useToast } from "./ui";

type Row = { kind: "same" | "add" | "del"; text: string; a?: number; b?: number };
type Hunk = { rows: Row[] } | { skipped: Row[] };

/** unified turns a line diff into rows with line numbers, folding long runs
 *  of unchanged lines down to `context` lines around each change. */
export function unified(from: string, to: string, context = 3): { hunks: Hunk[]; added: number; removed: number } {
  const rows: Row[] = [];
  let a = 0;
  let b = 0;
  for (const l of diffLines(from, to)) {
    if (l.kind === "same") rows.push({ kind: "same", text: l.a ?? "", a: ++a, b: ++b });
    else if (l.kind === "del") rows.push({ kind: "del", text: l.a ?? "", a: ++a });
    else rows.push({ kind: "add", text: l.b ?? "", b: ++b });
  }
  const near = rows.map(() => false);
  rows.forEach((r, i) => {
    if (r.kind === "same") return;
    for (let k = Math.max(0, i - context); k <= Math.min(rows.length - 1, i + context); k++) near[k] = true;
  });
  const hunks: Hunk[] = [];
  let i = 0;
  while (i < rows.length) {
    const keep = near[i];
    const run: Row[] = [];
    while (i < rows.length && near[i] === keep) run.push(rows[i++]);
    hunks.push(keep ? { rows: run } : { skipped: run });
  }
  return {
    hunks,
    added: rows.filter((r) => r.kind === "add").length,
    removed: rows.filter((r) => r.kind === "del").length,
  };
}

function DiffRow({ r }: { r: Row }) {
  const sign = r.kind === "add" ? "+" : r.kind === "del" ? "−" : " ";
  return (
    <div className={`ud-row ud-${r.kind}`}>
      <span className="ud-n">{r.a ?? ""}</span>
      <span className="ud-n">{r.b ?? ""}</span>
      <span className="ud-sign" aria-hidden>
        {sign}
      </span>
      <span className="ud-text">{r.text || " "}</span>
    </div>
  );
}

function Skipped({ rows }: { rows: Row[] }) {
  const [open, setOpen] = useState(false);
  if (open) return rows.map((r, k) => <DiffRow key={k} r={r} />);
  return (
    <button type="button" className="ud-skip" onClick={() => setOpen(true)}>
      {rows.length} unchanged {rows.length === 1 ? "line" : "lines"}, show
    </button>
  );
}

export function UnifiedDiff({ from, to }: { from: string; to: string }) {
  const d = useMemo(() => unified(from, to), [from, to]);
  if (d.added === 0 && d.removed === 0)
    return <div className="muted p-4 text-[13px]">The code is the same. Only the note or status differs.</div>;
  return (
    <div className="ud">
      {d.hunks.map((h, k) =>
        "rows" in h ? h.rows.map((r, j) => <DiffRow key={`${k}:${j}`} r={r} />) : <Skipped key={k} rows={h.skipped} />,
      )}
    </div>
  );
}

type Against = "previous" | "live" | "latest";

export function VersionTimeline({ versions, onRestored }: { versions: Version[]; onRestored?: () => void }) {
  // versions are newest first.
  const [sel, setSel] = useState<string | undefined>(versions[0]?.id);
  const [against, setAgainst] = useState<Against>("previous");
  const toast = useToast();
  const confirm = useConfirm();
  useEffect(() => {
    if (!versions.some((v) => v.id === sel)) setSel(versions[0]?.id);
  }, [versions, sel]);
  const restore = useAction((id: string) => post<SaveResult>(`/api/versions/${id}/rollback`), {
    onSuccess: (r) => {
      toast(r.message);
      setSel(undefined);
      onRestored?.();
    },
  });
  const i = Math.max(
    0,
    versions.findIndex((v) => v.id === sel),
  );
  const v = versions[i];
  if (!v) return null;
  const live = versions.find((x) => x.status === "live");
  const base = against === "previous" ? versions[i + 1] : against === "live" ? live : versions[0];
  const sameAsBase = base?.id === v.id;
  const options: { value: Against; label: string }[] = [
    { value: "previous", label: versions[i + 1] ? `the version before (v${versions[i + 1].n})` : "the version before" },
    ...(live ? [{ value: "live" as const, label: `the live version (v${live.n})` }] : []),
    { value: "latest", label: `the latest version (v${versions[0].n})` },
  ];
  const doRestore = async () => {
    const ok = await confirm({
      title: `Restore v${v.n}?`,
      body: (
        <>
          This saves v{v.n}'s code and tests as a new version, v{versions[0].n + 1}. Nothing is deleted: v
          {versions[0].n} stays in the history and you can restore it the same way.
        </>
      ),
      action: `Restore v${v.n}`,
    });
    if (ok) restore.mutate(v.id);
  };
  return (
    <div className="ver-grid">
      <div className="panel ver-list">
        <div className="panel-head">
          <History size={14} /> History
        </div>
        <ol className="timeline">
          {versions.map((x, k) => (
            <li key={x.id}>
              <button
                type="button"
                className={`tl-item${x.id === v.id ? " sel" : ""}`}
                aria-current={x.id === v.id}
                onClick={() => setSel(x.id)}
              >
                <span className={`tl-dot ${x.status}`} />
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2">
                    <strong>v{x.n}</strong>
                    <StatusChip status={x.status} />
                    {k === 0 && <Chip>latest</Chip>}
                  </span>
                  <span className="muted block truncate text-[12px]">
                    {x.createdByKind === "you" ? "You" : x.createdByName} · {dateTime(x.createdAt)}
                  </span>
                  {x.note && <span className="block truncate text-[12px]">{x.note}</span>}
                </span>
              </button>
            </li>
          ))}
        </ol>
      </div>
      <div className="panel min-w-0">
        <div className="panel-head flex-wrap gap-2">
          <span className="font-semibold">v{v.n}</span>
          {versions.length > 1 && (
            <span className="muted flex items-center gap-2">
              compared with
              <Select<Against> label="Compare with" size="sm" value={against} onChange={setAgainst} options={options} />
            </span>
          )}
          <span className="ml-auto" />
          {i > 0 && (
            <Button size="sm" icon={<RotateCcw size={12} />} onClick={doRestore} disabled={restore.isPending}>
              Restore this version
            </Button>
          )}
        </div>
        <ErrorBox error={restore.error} />
        {!base ? (
          <div>
            <div className="muted px-4 pt-3 text-[12.5px]">
              {versions.length === 1
                ? "The first version. Each save adds one here, and you can compare and restore any of them."
                : "The first version, so there is nothing before it. Its code:"}
            </div>
            <div className="ud">
              {v.source.split("\n").map((text, k) => (
                <DiffRow key={k} r={{ kind: "same", text, b: k + 1 }} />
              ))}
            </div>
          </div>
        ) : sameAsBase ? (
          <div className="muted p-4 text-[13px]">This is v{v.n} itself. Pick another version to compare with.</div>
        ) : (
          <>
            <DiffStat from={base.source} to={v.source} base={base.n} />
            <UnifiedDiff from={base.source} to={v.source} />
          </>
        )}
      </div>
    </div>
  );
}

function DiffStat({ from, to, base }: { from: string; to: string; base: number }) {
  const d = useMemo(() => unified(from, to), [from, to]);
  return (
    <div className="ud-stat">
      <span className="ud-plus">+{d.added}</span> <span className="ud-minus">−{d.removed}</span>{" "}
      <span className="muted">lines against v{base}</span>
    </div>
  );
}
