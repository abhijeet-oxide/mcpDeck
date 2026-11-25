import { Check, X } from "lucide-react";
import { AccessChip, Button, Chip, ErrorBox, Loading, PageHead, Panel, StatusChip, useToast } from "../components/ui";
import { post } from "../lib/api";
import { dateTime } from "../lib/format";
import { useAction, useToolkit, useVersion } from "../lib/hooks";
import { Link, navigate } from "../lib/router";
import type { Version } from "../lib/types";

// Review a version like a pull request (AB-3).
export function Review({ id }: { id: string }) {
  const q = useVersion(id);
  const toast = useToast();
  const v = q.data;
  const tk = useToolkit(v?.toolkitId ?? "");
  const decide = useAction(
    (approve: boolean) => post<Version>(`/api/versions/${id}/${approve ? "approve" : "reject"}`),
    {
      onSuccess: (r) => {
        const agents = (tk.data?.grants ?? []).filter((g) => r.access === "read" || g.access === "read_write").length;
        toast(
          r.status === "live"
            ? `${r.name} is live for ${agents} agent${agents === 1 ? "" : "s"} of ${tk.data?.name}`
            : `${r.name} v${r.n} rejected`,
        );
        navigate(`/toolkits/${tk.data?.name}/tools/${r.name}`);
      },
    },
  );
  if (q.isLoading) return <Loading />;
  if (q.error || !v)
    return (
      <div className="page">
        <ErrorBox error={q.error} />
      </div>
    );
  const failed = (v.checks ?? []).filter((c) => !c.passed);
  const writers = (tk.data?.grants ?? []).filter((g) => g.access === "read_write");
  const getters = v.access === "write" ? writers : (tk.data?.grants ?? []);
  return (
    <div className="page">
      <PageHead
        crumbs={
          <>
            <Link to="/toolkits">Toolkits</Link> › <Link to={`/toolkits/${tk.data?.name}`}>{tk.data?.name}</Link> ›
            review
          </>
        }
        title={`${v.name} v${v.n}`}
        mono
        sub={
          <span className="flex flex-wrap items-center gap-2">
            <StatusChip status={v.status} /> <AccessChip access={v.access} /> saved by{" "}
            {v.createdByKind === "you" ? "you" : v.createdByName} · {dateTime(v.createdAt)}
            {v.sessionId && (
              <Link to={`/activity?session=${v.sessionId}`}>
                session <code>{v.sessionId.slice(0, 10)}</code>
              </Link>
            )}
          </span>
        }
        actions={
          v.status === "pending" && (
            <>
              <Button kind="bad" icon={<X size={14} />} onClick={() => decide.mutate(false)}>
                Reject
              </Button>
              <Button kind="blue" icon={<Check size={14} />} onClick={() => decide.mutate(true)}>
                Approve and publish
              </Button>
            </>
          )
        }
      />
      <ErrorBox error={decide.error} />
      <div className="grid items-start gap-4" style={{ gridTemplateColumns: "minmax(0, 1fr) 340px" }}>
        <Panel title="Code">
          <pre className="code" style={{ border: 0, borderRadius: 0, maxHeight: 640 }}>
            {v.source}
          </pre>
        </Panel>
        <div className="flex flex-col gap-4">
          <Panel title="Approval rules">
            {(v.checks ?? []).map((c) => (
              <div key={c.rule} className="list-row items-start text-[13px]">
                <Chip tone={c.passed ? "ok" : "bad"}>{c.passed ? "pass" : "fail"}</Chip>
                <span>
                  {c.label}
                  {c.detail && <span className="muted block text-[12px]">{c.detail}</span>}
                </span>
              </div>
            ))}
            {failed.length === 0 && v.status === "pending" && (
              <div className="muted p-3 text-[12.5px]">Every rule passed; auto-approval is off for this toolkit.</div>
            )}
          </Panel>
          <Panel title="Requests it sends">
            {v.calls.length === 0 && <div className="muted p-3">None</div>}
            {v.calls.map((c) => (
              <div key={c} className="list-row">
                <code>{c}</code>
              </div>
            ))}
          </Panel>
          <Panel title="Tests">
            {(v.testResults ?? []).length === 0 && <div className="muted p-3">No tests</div>}
            {(v.testResults ?? []).map((t) => (
              <div key={t.name} className="list-row text-[13px]">
                <Chip tone={t.passed ? "ok" : "bad"}>{t.passed ? "pass" : "fail"}</Chip>
                {t.name}
                <span className="muted ml-auto">{t.tokens} tok</span>
              </div>
            ))}
          </Panel>
          <Panel title="Who gets it when approved">
            <div className="p-3 text-[13px]">
              {getters.length === 0
                ? v.access === "write"
                  ? "No agent has read and write on this toolkit, so nobody gets it until you change access."
                  : "No agents use this toolkit yet."
                : getters.map((g) => g.agent.name).join(", ")}
            </div>
          </Panel>
        </div>
      </div>
    </div>
  );
}
