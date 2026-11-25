import { Copy, ShieldCheck } from "lucide-react";
import { useState } from "react";
import {
  Button,
  Chip,
  Drawer,
  Empty,
  ErrorBox,
  Loading,
  MethodChip,
  PageHead,
  Panel,
  Seg,
  StatusChip,
  Tabs,
  useCopy,
} from "../components/ui";
import { post } from "../lib/api";
import { bytes, dateTime, num, pretty, time } from "../lib/format";
import { useAudit, useCall, useCalls, useUsage } from "../lib/hooks";
import { navigate, useLocation } from "../lib/router";
import type { Call } from "../lib/types";

type Tab = "calls" | "usage" | "audit";

export function Activity() {
  const { search } = useLocation();
  const [tab, setTab] = useState<Tab>((search.get("tab") as Tab) || "calls");
  return (
    <div className="page page-wide">
      <PageHead title="Activity" sub="Every call agents and you made, what it cost, and what changed." />
      <Tabs<Tab>
        value={tab}
        onChange={setTab}
        tabs={[
          { id: "calls", label: "Calls" },
          { id: "usage", label: "Usage" },
          { id: "audit", label: "Audit log" },
        ]}
      />
      {tab === "calls" && <Calls />}
      {tab === "usage" && <UsageTab />}
      {tab === "audit" && <Audit />}
    </div>
  );
}

const kinds = [
  { id: "", label: "All" },
  { id: "tool", label: "Tools" },
  { id: "build", label: "Build" },
  { id: "upstream", label: "API" },
  { id: "list", label: "tools/list" },
];

function Calls() {
  const { search } = useLocation();
  const [kind, setKind] = useState("");
  const [status, setStatus] = useState(search.get("status") ?? "");
  const [q, setQ] = useState("");
  const session = search.get("session") ?? "";
  const agent = search.get("agent") ?? "";
  const [open, setOpen] = useState(search.get("call") ?? "");
  const params = new URLSearchParams({ kind, status, q, session, agent, limit: "300" });
  if (session) params.set("all", "1");
  const calls = useCalls(params.toString());
  const list = calls.data ?? [];
  const t0 = list.length ? Math.min(...list.map((c) => new Date(c.startedAt).getTime())) : 0;
  const t1 = list.length ? Math.max(...list.map((c) => new Date(c.startedAt).getTime() + c.durationMs)) : 1;
  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <Seg label="Kind" value={kind} onChange={setKind} options={kinds} />
        <select
          className="select"
          style={{ width: 130 }}
          aria-label="Status"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
        >
          <option value="">Any status</option>
          <option value="ok">OK</option>
          <option value="error">Errors</option>
          <option value="blocked">Blocked</option>
        </select>
        <input
          className="input"
          style={{ width: 220 }}
          aria-label="Filter by name"
          placeholder="customer_360"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        {(session || agent) && (
          <Button size="sm" onClick={() => navigate("/activity")}>
            Clear {session ? "session" : "agent"} filter
          </Button>
        )}
        <span className="muted ml-auto text-[12px]">Tokens are those mcpDeck returned, counted with cl100k_base.</span>
      </div>
      <Panel>
        {calls.isLoading && <Loading />}
        {!calls.isLoading && list.length === 0 && (
          <Empty title="No calls match">Calls show up here as agents and you use mcpDeck.</Empty>
        )}
        {list.length > 0 && (
          <div style={{ overflowX: "auto" }}>
            <table className="table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Type</th>
                  <th>Who</th>
                  <th className="num">Time</th>
                  <th className="num">Size</th>
                  <th className="num">Tokens</th>
                  <th style={{ width: session ? 180 : 90 }}>{session ? "Waterfall" : "When"}</th>
                </tr>
              </thead>
              <tbody>
                {list.map((c) => {
                  const s = new Date(c.startedAt).getTime();
                  return (
                    <tr key={c.id} className="clickable" aria-selected={open === c.id} onClick={() => setOpen(c.id)}>
                      <td className="mono" style={{ paddingLeft: c.parentId ? 28 : undefined }}>
                        <span className="flex items-center gap-2">
                          {c.method && <MethodChip method={c.method} />}
                          {c.name}
                        </span>
                      </td>
                      <td>
                        {c.httpStatus ? (
                          <Chip tone={c.httpStatus < 400 ? "ok" : "bad"}>{c.httpStatus}</Chip>
                        ) : (
                          <StatusChip status={c.status} />
                        )}
                      </td>
                      <td className="muted">{c.kind}</td>
                      <td className="truncate" style={{ maxWidth: 170 }}>
                        {c.actorName}
                      </td>
                      <td className="num">{c.durationMs} ms</td>
                      <td className="num">{bytes(c.outputBytes)}</td>
                      <td className="num">{num(c.tokens)}</td>
                      <td>
                        {session ? (
                          <div className="tl">
                            <span
                              className={c.status === "ok" ? "" : "err"}
                              style={{
                                left: `${((s - t0) / Math.max(1, t1 - t0)) * 100}%`,
                                width: `${(c.durationMs / Math.max(1, t1 - t0)) * 100}%`,
                              }}
                            />
                          </div>
                        ) : (
                          <span className="muted whitespace-nowrap text-[12px]">{time(c.startedAt)}</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
      {open && <CallDrawer id={open} onClose={() => setOpen("")} />}
    </>
  );
}

function fixPrompt(c: Call) {
  return `A call to ${c.name} through mcpDeck failed.
Error: ${c.error}
Input: ${c.input || "{}"}

Read the error: it says what to do next. If the input was wrong, fix it and call once more. If a tool is missing or needs write access, call request_tool instead of calling the API another way.
Check: the next call succeeds, or you filed a tool request.`;
}

function CallDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const q = useCall(id);
  const copy = useCopy();
  const [tab, setTab] = useState<"output" | "input" | "upstream">("output");
  const c = q.data;
  return (
    <Drawer title={c ? <span className="mono">{c.name}</span> : "Call"} onClose={onClose}>
      {q.isLoading && <Loading />}
      {c && (
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2 text-[12.5px]">
            <StatusChip status={c.status} />
            <Chip>{c.kind}</Chip>
            <span className="muted">
              {c.actorName} · {dateTime(c.startedAt)} · {c.durationMs} ms · {bytes(c.outputBytes)} · {num(c.tokens)}{" "}
              tokens
            </span>
          </div>
          {c.url && (
            <div className="mono muted text-[12px]" style={{ wordBreak: "break-all" }}>
              {c.method} {c.url}
            </div>
          )}
          {c.error && (
            <div className="banner bad items-start">
              <span className="flex-1">{c.error}</span>
              <Button size="sm" icon={<Copy size={12} />} onClick={() => copy(fixPrompt(c), "Fix prompt copied")}>
                Copy fix prompt
              </Button>
            </div>
          )}
          <Tabs
            value={tab}
            onChange={setTab}
            tabs={[
              { id: "output", label: "Output" },
              { id: "input", label: "Input" },
              { id: "upstream", label: "API calls", n: c.children.length },
            ]}
          />
          {tab === "output" && (
            <pre className="code" style={{ maxHeight: 480 }}>
              {pretty(c.output) || "(no body kept)"}
            </pre>
          )}
          {tab === "input" && (
            <pre className="code" style={{ maxHeight: 480 }}>
              {pretty(c.input) || "{}"}
            </pre>
          )}
          {tab === "upstream" && (
            <div className="flex flex-col gap-2">
              {c.children.length === 0 && <span className="muted">No API calls.</span>}
              {c.children.map((u) => (
                <div key={u.id} className="panel p-2.5 text-[12.5px]">
                  <div className="flex items-center gap-2">
                    {u.method && <MethodChip method={u.method} />}
                    <span className="mono flex-1 truncate">{u.name}</span>
                    <Chip tone={u.status === "ok" ? "ok" : "bad"}>{u.httpStatus || u.status}</Chip>
                    <span className="muted">
                      {u.durationMs} ms · {bytes(u.outputBytes)}
                    </span>
                  </div>
                  {u.error && <div style={{ color: "var(--bad)" }}>{u.error}</div>}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </Drawer>
  );
}

function UsageTab() {
  const [range, setRange] = useState("7d");
  const [by, setBy] = useState<"tools" | "toolkits" | "agents">("tools");
  const q = useUsage(range);
  const u = q.data;
  const max = Math.max(1, ...(u?.daily ?? []).map((d) => d.calls));
  return (
    <>
      <div className="mb-3 flex items-center gap-2">
        <Seg
          label="Range"
          value={range}
          onChange={setRange}
          options={[
            { id: "24h", label: "24 h" },
            { id: "7d", label: "7 days" },
            { id: "30d", label: "30 days" },
          ]}
        />
        <Seg
          label="Group"
          value={by}
          onChange={setBy}
          options={[
            { id: "tools", label: "Per tool" },
            { id: "toolkits", label: "Per toolkit" },
            { id: "agents", label: "Per agent" },
          ]}
        />
      </div>
      {q.isLoading && <Loading />}
      {u && (
        <div className="flex flex-col gap-4">
          <Panel title="Calls per day">
            <div className="flex items-end gap-1 px-4 pt-4 pb-2" style={{ height: 150 }}>
              {u.daily.map((d) => (
                <div
                  key={d.day}
                  className="flex flex-1 flex-col justify-end"
                  style={{ height: 130 }}
                  title={`${d.day}: ${d.calls} calls, ${num(d.tokens)} tokens, ${d.errors} errors`}
                >
                  <div
                    style={{
                      height: Math.max(2, (d.calls / max) * 130),
                      background: d.calls ? "var(--accent)" : "var(--line)",
                      borderRadius: 3,
                    }}
                  />
                </div>
              ))}
            </div>
          </Panel>
          <Panel>
            {u[by].length === 0 ? (
              <Empty title="No calls in this range" />
            ) : (
              <table className="table">
                <thead>
                  <tr>
                    <th>{by === "tools" ? "Tool" : by === "toolkits" ? "Toolkit" : "Agent"}</th>
                    <th className="num">Calls</th>
                    <th className="num">Error rate</th>
                    <th className="num">p50</th>
                    <th className="num">p95</th>
                    <th className="num">Avg output</th>
                    <th className="num">API calls per call</th>
                    {by !== "tools" && <th className="num">tools/list</th>}
                  </tr>
                </thead>
                <tbody>
                  {u[by].map((r) => (
                    <tr key={r.key}>
                      <td className={by === "agents" ? "" : "mono"}>{r.label || r.key}</td>
                      <td className="num">{r.calls}</td>
                      <td className="num">{(r.errorRate * 100).toFixed(1)}%</td>
                      <td className="num">{r.p50} ms</td>
                      <td className="num">{r.p95} ms</td>
                      <td className="num">{r.avgTokens} tok</td>
                      <td className="num">{r.upstreamPerCall.toFixed(1)}</td>
                      {by !== "tools" && <td className="num">{r.listTokens ? `${r.listTokens} tok` : "–"}</td>}
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Panel>
        </div>
      )}
    </>
  );
}

function Audit() {
  const q = useAudit();
  const [verify, setVerify] = useState<{ ok: boolean; message: string } | null>(null);
  return (
    <Panel
      title="Every change, append-only"
      right={
        <>
          {verify && <Chip tone={verify.ok ? "ok" : "bad"}>{verify.message}</Chip>}
          <Button
            size="sm"
            icon={<ShieldCheck size={13} />}
            onClick={async () => setVerify(await post("/api/audit/verify"))}
          >
            Verify chain
          </Button>
        </>
      }
    >
      <ErrorBox error={q.error} />
      {q.data?.length === 0 && <Empty title="No changes yet" />}
      {!!q.data?.length && (
        <table className="table">
          <thead>
            <tr>
              <th>When</th>
              <th>Who</th>
              <th>Action</th>
              <th>Target</th>
              <th>Detail</th>
            </tr>
          </thead>
          <tbody>
            {q.data.map((e) => (
              <tr key={e.seq}>
                <td className="muted whitespace-nowrap">{dateTime(e.at)}</td>
                <td>{e.actorName}</td>
                <td className="mono">{e.action}</td>
                <td className="mono">{e.target}</td>
                <td className="muted">{e.detail}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Panel>
  );
}
