import { Bot, Code2, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { useConnect } from "../App";
import { AskAgentButton, toolRequestPrompt } from "../components/AskAgent";
import {
  AccessChip,
  Button,
  Chip,
  CopyButton,
  CopyField,
  Dialog,
  Empty,
  ErrorBox,
  Field,
  Loading,
  PageHead,
  Panel,
  StatusChip,
  Tabs,
  Toggle,
  useToast,
} from "../components/ui";
import { del, patch, post } from "../lib/api";
import { ago, num } from "../lib/format";
import { useAction, useAPIs, useLibrary, useToolkit } from "../lib/hooks";
import { Link, navigate, useLocation } from "../lib/router";
import type { Rules, ToolkitDetail } from "../lib/types";

type Tab = "tools" | "agents" | "requests" | "settings";

export function ToolkitPage({ name }: { name: string }) {
  const q = useToolkit(name);
  const { search } = useLocation();
  const [tab, setTab] = useState<Tab>((search.get("tab") as Tab) || "tools");
  const [adding, setAdding] = useState(false);
  const connect = useConnect();
  if (q.isLoading) return <Loading />;
  if (q.error)
    return (
      <div className="page">
        <ErrorBox error={q.error} />
      </div>
    );
  const t = q.data!;
  const reviews = t.pendingList.length + t.toolRequests.filter((r) => r.state === "open").length;
  const firstTool = t.toolRows[0]?.name ?? "customer_360";
  return (
    <div className="page">
      <PageHead
        crumbs={
          <>
            <Link to="/toolkits">Toolkits</Link> › {t.name}
          </>
        }
        title={t.name}
        mono
        sub={t.description || "No description yet"}
        actions={
          <>
            <AskAgentButton
              toolkit={t.name}
              prompts={[
                {
                  title: "Use this toolkit",
                  text: `Use the mcpdeck-${t.name} tools to answer: <your question>.\nStart with ${firstTool} if it fits; otherwise search_tools.\nCheck: no raw API calls, and report_field_usage after a large response.`,
                },
                {
                  title: "Build a missing tool",
                  needsBuild: true,
                  text: `Hand this to the mcpdeck-${t.name}-builder subagent: build a tool that returns <fields> for <input>.\nIt should check search_tools and list_tool_requests first, then test_tool and save_tool with a test.\nCheck: the reply says live, or which approval rule held it.`,
                },
              ]}
            />
            <Button icon={<Plus size={14} />} onClick={() => navigate(`/toolkits/${t.name}/tools/new`)}>
              New tool
            </Button>
            <Button kind="ink" icon={<Bot size={14} />} onClick={() => connect(t.id)}>
              Connect agent
            </Button>
          </>
        }
      />
      <div className="mb-4 grid gap-3 md:grid-cols-3">
        <Field label="MCP endpoint (Streamable HTTP)">
          <CopyField value={t.endpoint} label="Endpoint" />
        </Field>
        <Field label="Builder endpoint (for the builder subagent)">
          <CopyField value={t.buildEndpoint} label="Builder endpoint" />
        </Field>
        <Field label="For clients that only run commands (stdio)">
          <CopyField value={`mcpdeck stdio --toolkit ${t.name}`} label="Command" />
        </Field>
      </div>
      <Tabs<Tab>
        value={tab}
        onChange={setTab}
        tabs={[
          { id: "tools", label: "Tools", n: t.toolRows.length },
          { id: "agents", label: "Agents", n: t.grants.length },
          { id: "requests", label: "Requests and reviews", n: reviews },
          { id: "settings", label: "Settings" },
        ]}
      />
      {tab === "tools" && (
        <Panel>
          {t.toolRows.length === 0 ? (
            <Empty title="No tools yet">
              <div className="mt-2 flex justify-center gap-2">
                <Button kind="ink" onClick={() => navigate(`/toolkits/${t.name}/tools/new`)}>
                  New tool
                </Button>
                <Button onClick={() => setAdding(true)}>Add from library</Button>
              </div>
            </Empty>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Tool</th>
                  <th>APIs</th>
                  <th>Access</th>
                  <th>Version</th>
                  <th className="num">Calls 7d</th>
                  <th className="num">p50</th>
                  <th className="num">Avg output</th>
                  <th>Made by</th>
                </tr>
              </thead>
              <tbody>
                {t.toolRows.map((r) => (
                  <tr key={r.id} className="clickable" onClick={() => navigate(`/toolkits/${t.name}/tools/${r.name}`)}>
                    <td style={{ maxWidth: 360 }}>
                      <div className="mono font-semibold">{r.name}</div>
                      <div className="muted truncate text-[12.5px]">{r.latest.description}</div>
                    </td>
                    <td>
                      {r.latest.apis.map((a) => (
                        <span key={a} className="tag mr-1">
                          {a}
                        </span>
                      ))}
                    </td>
                    <td>
                      <AccessChip access={r.latest.access} />
                    </td>
                    <td>
                      <span className="flex items-center gap-1.5">
                        v{r.latest.n} <StatusChip status={r.latest.status} />
                      </span>
                    </td>
                    <td className="num">{r.calls7d || "–"}</td>
                    <td className="num">{r.p50 ? `${r.p50} ms` : "–"}</td>
                    <td className="num">{r.avgTokens ? `${r.avgTokens} tok` : "–"}</td>
                    <td className="text-[12.5px]">
                      {r.latest.createdByKind === "you" ? "You" : r.latest.createdByName}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <div
            className="muted flex items-center gap-3 p-3 text-[12.5px]"
            style={{ borderTop: "1px solid var(--line)" }}
          >
            <Button size="sm" icon={<Plus size={13} />} onClick={() => setAdding(true)}>
              Add from library
            </Button>
            Agents load the tool list once per session: {num(t.listTokens)} tokens for this toolkit.
          </div>
        </Panel>
      )}
      {tab === "agents" && <AgentsTab t={t} />}
      {tab === "requests" && <RequestsTab t={t} />}
      {tab === "settings" && <SettingsTab key={t.id} t={t} />}
      {adding && <AddFromLibrary t={t} onClose={() => setAdding(false)} />}
    </div>
  );
}

function AddFromLibrary({ t, onClose }: { t: ToolkitDetail; onClose: () => void }) {
  const lib = useLibrary();
  const add = useAction((tool: string) => post(`/api/toolkits/${t.name}/tools`, { tool }));
  const have = new Set(t.toolRows.map((r) => r.id));
  const list = (lib.data ?? []).filter((x) => !have.has(x.id) && x.live);
  return (
    <Dialog title={`Add to ${t.name}`} onClose={onClose}>
      {list.length === 0 && <Empty title="Nothing to add">Every live tool in the library is already here.</Empty>}
      {list.map((x) => (
        <div key={x.id} className="list-row">
          <div className="min-w-0 flex-1">
            <div className="mono font-semibold">{x.name}</div>
            <div className="muted truncate text-[12.5px]">{x.latest.description}</div>
          </div>
          <Button size="sm" onClick={() => add.mutate(x.name)}>
            Add
          </Button>
        </div>
      ))}
      <ErrorBox error={add.error} />
    </Dialog>
  );
}

function AgentsTab({ t }: { t: ToolkitDetail }) {
  const connect = useConnect();
  const upd = useAction(({ id, body }: { id: string; body: object }) => patch(`/api/grants/${id}`, body));
  const remove = useAction((id: string) => del(`/api/grants/${id}`));
  return (
    <Panel right={null}>
      {t.grants.length === 0 ? (
        <Empty title="No agents use this toolkit">
          <div className="mt-2">
            <Button kind="ink" onClick={() => connect(t.id)}>
              Connect agent
            </Button>
          </div>
        </Empty>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Agent</th>
              <th>Access</th>
              <th>May build</th>
              <th>Last seen</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {t.grants.map((g) => (
              <tr key={g.id}>
                <td>
                  <Link to={`/agents/${g.agent.id}`} className="font-medium" style={{ color: "var(--text)" }}>
                    {g.agent.name}
                  </Link>
                  {g.agent.revokedAt && <Chip tone="bad">revoked</Chip>}
                </td>
                <td>
                  <select
                    className="select"
                    style={{ width: 150 }}
                    aria-label="Access"
                    value={g.access}
                    onChange={(e) => upd.mutate({ id: g.id, body: { access: e.target.value } })}
                  >
                    <option value="read">Read</option>
                    <option value="read_write">Read and write</option>
                  </select>
                </td>
                <td>
                  <Toggle
                    label="May build"
                    checked={g.canBuild}
                    onChange={(v) => upd.mutate({ id: g.id, body: { canBuild: v } })}
                  />
                </td>
                <td className="muted">
                  <span className="flex items-center gap-1.5">
                    <span className={`dot ${g.connected ? "on" : ""}`} />
                    {ago(g.agent.lastSeenAt)}
                  </span>
                </td>
                <td className="num">
                  <Button
                    size="sm"
                    kind="bad"
                    onClick={() => confirm(`Remove ${g.agent.name}'s access to ${t.name}?`) && remove.mutate(g.id)}
                  >
                    Remove
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <div className="muted p-3 text-[12px]" style={{ borderTop: "1px solid var(--line)" }}>
        Read agents never see or run tools that write, and their write requests are refused at the proxy. Agents that
        may build use a separate builder endpoint, so exploring APIs doesn't fill their main context.
      </div>
      <ErrorBox error={upd.error || remove.error} />
    </Panel>
  );
}

function RequestsTab({ t }: { t: ToolkitDetail }) {
  const close = useAction((id: string) => post(`/api/tool-requests/${id}/close`));
  const open = t.toolRequests.filter((r) => r.state === "open");
  return (
    <div className="flex flex-col gap-4">
      <Panel title="Waiting for review" icon={<Code2 size={15} className="muted" />}>
        {t.pendingList.length === 0 && (
          <Empty title="Nothing to review">
            Versions saved by agents that don't pass every rule, and every tool that writes, wait here.
          </Empty>
        )}
        {t.pendingList.map((v) => (
          <div key={v.id} className="list-row">
            <div className="min-w-0 flex-1">
              <span className="mono font-semibold">
                {v.name} v{v.n}
              </span>{" "}
              <AccessChip access={v.access} />
              <div className="muted text-[12.5px]">
                {v.createdByName} · {ago(v.createdAt)} · held by{" "}
                {(v.checks ?? [])
                  .filter((c) => !c.passed)
                  .map((c) => c.label)
                  .join("; ") || "auto-approval off"}
              </div>
            </div>
            <Button size="sm" kind="blue" onClick={() => navigate(`/review/${v.id}`)}>
              Review
            </Button>
          </div>
        ))}
      </Panel>
      <Panel title="Tool requests from agents">
        {open.length === 0 && <Empty title="No open requests">Agents ask with request_tool when no tool fits.</Empty>}
        {open.map((r) => (
          <div key={r.id} className="list-row items-start">
            <div className="min-w-0 flex-1">
              <div className="font-medium">{r.description}</div>
              {r.tried && <div className="muted text-[12.5px]">Tried: {r.tried}</div>}
              <div className="muted text-[12px]">
                {r.agentName} · {ago(r.createdAt)} · <code>{r.id}</code>
              </div>
            </div>
            <CopyButton text={toolRequestPrompt(r, t.name)} label="Copy prompt" what="Prompt copied" />
            <Button size="sm" onClick={() => close.mutate(r.id)}>
              Close
            </Button>
          </div>
        ))}
      </Panel>
    </div>
  );
}

const ruleLabels: { key: keyof Rules; label: string; hint: string }[] = [
  { key: "requireTests", label: "Has at least one test, and all tests pass", hint: "Tests run on save" },
  { key: "descriptionCheck", label: "Description check", hint: "Holds text that addresses agents or contains links" },
  { key: "noDuplicates", label: "Not a duplicate of a library tool", hint: "Same requests as an existing tool" },
];

function SettingsTab({ t }: { t: ToolkitDetail }) {
  const apis = useAPIs();
  const toast = useToast();
  const [desc, setDesc] = useState(t.description);
  const [scope, setScope] = useState<string[]>(t.buildScope);
  const [rules, setRules] = useState<Rules>(t.rules);
  const [hints, setHints] = useState(t.hints);
  const save = useAction(
    () => patch(`/api/toolkits/${t.name}`, { description: desc, buildScope: scope, rules, hints }),
    {
      onSuccess: () => toast("Toolkit saved; connected agents get the change"),
    },
  );
  const remove = useAction(() => del(`/api/toolkits/${t.name}`), { onSuccess: () => navigate("/toolkits") });
  return (
    <div className="grid items-start gap-4 lg:grid-cols-2">
      <Panel title="Toolkit" bodyClass="panel-body flex flex-col gap-3">
        <Field label="What it's for" hint="Sent to agents as the server's instructions.">
          <input className="input" value={desc} onChange={(e) => setDesc(e.target.value)} />
        </Field>
        <Field
          label="Build list"
          hint="APIs that building agents may explore. Empty means the APIs this toolkit's tools already use."
        >
          <div className="flex flex-wrap gap-3">
            {(apis.data ?? []).map((a) => (
              <label key={a.id} className="check">
                <input
                  type="checkbox"
                  checked={scope.includes(a.id)}
                  onChange={(e) => setScope(e.target.checked ? [...scope, a.id] : scope.filter((x) => x !== a.id))}
                />
                <code>{a.name}</code>
              </label>
            ))}
          </div>
        </Field>
        <label className="flex items-center gap-3">
          <Toggle label="Response hints" checked={hints} onChange={setHints} />
          <span>
            Response hints{" "}
            <span className="muted text-[12.5px]">
              · one short line on large responses asking for report_field_usage
            </span>
          </span>
        </label>
      </Panel>
      <Panel title="Approval rules for agent-saved versions" bodyClass="panel-body flex flex-col gap-3">
        <label className="flex items-center gap-3">
          <Toggle
            label="Auto-approve"
            checked={rules.autoApprove}
            onChange={(v) => setRules({ ...rules, autoApprove: v })}
          />
          <span>Go live without a person when every rule passes</span>
        </label>
        <div className="muted text-[12.5px]">
          Always on: only reads (tools that write always wait for you), and only APIs in the build list.
        </div>
        {ruleLabels.map((r) => (
          <label key={r.key} className="check">
            <input
              type="checkbox"
              checked={rules[r.key] as boolean}
              onChange={(e) => setRules({ ...rules, [r.key]: e.target.checked })}
            />
            <span>
              {r.label}
              <span className="muted block text-[12px]">{r.hint}</span>
            </span>
          </label>
        ))}
        <label className="flex items-center gap-2">
          Output under
          <input
            className="input"
            style={{ width: 90 }}
            type="number"
            min={0}
            value={rules.maxOutputTokens}
            onChange={(e) => setRules({ ...rules, maxOutputTokens: Number(e.target.value) })}
          />
          tokens on its tests (0 turns it off)
        </label>
      </Panel>
      <div className="flex gap-3 lg:col-span-2">
        <Button kind="ink" onClick={() => save.mutate(undefined)}>
          Save settings
        </Button>
        <Button
          kind="bad"
          icon={<Trash2 size={13} />}
          onClick={() => confirm(`Delete toolkit ${t.name}? Tools stay in the library.`) && remove.mutate(undefined)}
        >
          Delete toolkit
        </Button>
        <ErrorBox error={save.error || remove.error} />
      </div>
    </div>
  );
}
