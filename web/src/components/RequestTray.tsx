// The tray at the bottom of every page: requests still being sent, and ones
// that finished while you were elsewhere (NET-3).
import { CircleCheck, CircleX, Loader2, X } from "lucide-react";
import { useEffect, useState } from "react";
import { cancel, dismiss, type Flight, useFlights } from "../lib/inflight";
import { navigate, useLocation } from "../lib/router";
import { MethodChip } from "./ui";

/** seconds formats a duration like Postman: 850 ms, 3.2 s, 1 min 05 s. */
export function seconds(ms: number): string {
  if (ms < 1000) return `${Math.max(0, Math.round(ms))} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
  const m = Math.floor(ms / 60_000);
  const s = Math.floor((ms % 60_000) / 1000);
  return `${m} min ${String(s).padStart(2, "0")} s`;
}

/** Elapsed ticks from a start time until stopped. */
export function Elapsed({ since, until }: { since: number; until?: number }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (until) return;
    const t = setInterval(() => setNow(Date.now()), 100);
    return () => clearInterval(t);
  }, [until]);
  return <span className="tabular-nums">{seconds((until ?? now) - since)}</span>;
}

function Item({ f, here }: { f: Flight; here: boolean }) {
  const done = !!f.finishedAt;
  const failed = !!f.result?.error || (f.result?.status ?? 0) >= 400;
  const open = () => navigate(`/apps/${f.apiId}?req=${encodeURIComponent(f.reqId || "new")}`);
  return (
    <div className={`tray-item${done ? " done" : ""}`}>
      <button type="button" className="tray-main" onClick={open} title={here ? "Shown above" : "Open this request"}>
        {done ? (
          failed ? (
            <CircleX size={14} className="tray-bad" />
          ) : (
            <CircleCheck size={14} className="tray-ok" />
          )
        ) : (
          <Loader2 size={14} className="spin" />
        )}
        <MethodChip method={f.method} />
        <span className="mono min-w-0 flex-1 truncate">{f.label}</span>
        <span className="muted text-[12px]">
          {done && f.result?.status ? `${f.result.status} · ` : done && f.result?.error ? "failed · " : ""}
          <Elapsed since={f.startedAt} until={f.finishedAt} />
        </span>
      </button>
      {done ? (
        <button type="button" className="icon-btn" aria-label={`Dismiss ${f.label}`} onClick={() => dismiss(f.key)}>
          <X size={13} />
        </button>
      ) : (
        <button type="button" className="tray-cancel" onClick={() => cancel(f.key)} aria-label={`Cancel ${f.label}`}>
          Cancel
        </button>
      )}
    </div>
  );
}

export function RequestTray() {
  const flights = useFlights();
  const { path, search } = useLocation();
  const shown = flights.filter((f) => !f.finishedAt || !f.seen);
  if (shown.length === 0) return null;
  const running = shown.filter((f) => !f.finishedAt).length;
  return (
    <section className="tray" aria-label="Requests in progress">
      <div className="tray-head">
        {running > 0 ? `${running} ${running === 1 ? "request" : "requests"} running` : "Finished while you were away"}
      </div>
      {shown.map((f) => (
        <Item key={f.key} f={f} here={path === `/apps/${f.apiId}` && (search.get("req") ?? "") === f.reqId} />
      ))}
    </section>
  );
}
