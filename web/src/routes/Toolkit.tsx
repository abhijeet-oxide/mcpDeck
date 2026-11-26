import { useQuery } from "@tanstack/react-query";
import {
  Bot,
  ChevronDown,
  Code2,
  Library,
  MoreHorizontal,
  Plus,
  ScanEye,
  Settings2,
  ShieldCheck,
  Trash2,
  Wrench,
} from "lucide-react";
import { useState } from "react";
import { useConnect } from "../App";
import { AskAgentButton, toolRequestPrompt } from "../components/AskAgent";
import { ClientIcon, clientLabels } from "../components/ClientIcon";
import { counts, FindingList, ScoreRing } from "../components/Findings";
import {
  AccessChip,
  Avatar,
  Button,
  Chip,
  CopyButton,
  CopyField,
  Dialog,
  Empty,
  EmptyState,
  ErrorBox,
  Loading,
  Menu,
  PageHead,
  Panel,
  SaveBar,
  Seg,
  Select,
  StatCard,
  StatusChip,
  Tabs,
  Toggle,
  useConfirm,
  useToast,
} from "../components/ui";
import { del, get, patch, post } from "../lib/api";
import { ago, num } from "../lib/format";
import { useAction, useAPIs, useLibrary, useToolkit } from "../lib/hooks";
import { Link, navigate, useLocation } from "../lib/router";
import type { AgentView, LintReport, Rules, ToolkitDetail } from "../lib/types";
import { GuidelinesBody } from "./Guidelines";

type Tab = "tools" | "agents" | "requests" | "guidelines" | "settings";

export function ToolkitPage({ name }: { name: string }) {
  const q = useToolkit(name);
  const { search } = useLocation();
  const [tab, setTab] = useState<Tab>((search.get("tab") as Tab) || "tools");
  const [adding, setAdding] = useState(false);
  const [preview, setPreview] = useState(false);
  const connect = useConnect();
  const toast = useToast();
  const confirm = useConfirm();
  const remove = useAction(() => del(`/api/toolkits/${name}?force=1`), {
    onSuccess: () => {
      toast(`Toolkit ${name} deleted; its tools stay in the library`);
      navigate("/toolkits");
    },
    onError: (e) => toast((e as Error).message, true),
  });
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
  const newTool = () => navigate(`/toolkits/${t.name}/tools/new`);
  return (
    <div className="page">
      <PageHead
        icon={<Avatar name={t.name} size="lg" />}
        crumbs={
          <>
            <Link to="/toolkits">Toolkits</Link> / {t.name}
          </>
        }
        title={t.name}
        mono
        sub={t.description || "No description yet. Agents read it when they connect: add one in Settings."}
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
            <Button icon={<Plus size={14} />} onClick={newTool}>
              New tool
            </Button>
            <Button kind="ink" icon={<Bot size={14} />} onClick={() => connect(t.id)}>
              Connect agent
            </Button>
            <Menu
              label="More toolkit actions"
              items={[
                { label: "Edit toolkit settings", icon: <Settings2 size={14} />, onClick: () => setTab("settings") },
                { label: "Add a tool from the library", icon: <Library size={14} />, onClick: () => setAdding(true) },
                "sep",
                {
                  label: "Delete toolkit",
                  icon: <Trash2 size={14} />,
                  danger: true,
                  onClick: async () =>
                    (await confirm({
                      title: `Delete toolkit ${t.name}?`,
                      body: (
                        <>
                          {t.grants.length > 0 ? (
                            <div className="banner warn mb-3">
                              {t.grants.length} {t.grants.length === 1 ? "agent uses" : "agents use"} it:{" "}
                              {t.grants.map((g) => g.agent.name).join(", ")}. Their access to this toolkit is removed at
                              once; their other toolkits keep working.
                            </div>
                          ) : null}
                          Its endpoint stops working. Its {t.toolRows.length} tools stay in the library. This can't be
                          undone.
                        </>
                      ),
                      action: "Delete toolkit",
                      danger: true,
                      typeToConfirm: t.name,
                    })) && remove.mutate(undefined),
                },
              ]}
            />
          </>
        }
      />
      <ConnectCard t={t} onPreview={() => setPreview(true)} onConnect={() => connect(t.id)} />
      <Tabs<Tab>
        value={tab}
        onChange={setTab}
        tabs={[
          { id: "tools", label: "Tools", n: t.toolRows.length },
          { id: "agents", label: "Agents", n: t.grants.length },
          { id: "requests", label: "Reviews and requests", n: reviews },
          { id: "guidelines", label: "Guidelines" },
          { id: "settings", label: "Settings" },
        ]}
      />
      {tab === "tools" && (
        <Panel>
          {t.toolRows.length === 0 ? (
            <EmptyState
              icon={<Wrench size={22} />}
              title="Add the first tool"
              actions={
                <>
                  <Button icon={<Library size={14} />} onClick={() => setAdding(true)}>
                    Add from library
                  </Button>
                  <Button kind="ink" icon={<Plus size={14} />} onClick={newTool}>
                    New tool
                  </Button>
                </>
              }
            >
              A tool is a short TypeScript function that calls your applications and returns only what an agent needs.
              Start from a request, ask a connected agent to build it, or write it yourself.
            </EmptyState>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Tool</th>
                  <th>Applications</th>
                  <th>Access</th>
                  <th>Version</th>
                  <th className="num">Calls 7d</th>
                  <th className="num">p50</th>
                  <th className="num">Avg output</th>
                  <th>Made by</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {t.toolRows.map((r) => (
                  <tr key={r.id} className="clickable" onClick={() => navigate(`/toolkits/${t.name}/tools/${r.name}`)}>
                    <td style={{ maxWidth: 360 }}>
                      <Link
                        to={`/toolkits/${t.name}/tools/${r.name}`}
                        className="mono font-semibold"
                        style={{ color: "var(--text)" }}
                      >
                        {r.name}
                      </Link>
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
                    <td onClick={(e) => e.stopPropagation()} style={{ width: 44 }}>
                      <ToolMenu t={t} tool={r.name} toolkits={r.toolkits} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {t.toolRows.length > 0 && (
            <div
              className="muted flex items-center gap-3 px-4 py-3 text-[12.5px]"
              style={{ borderTop: "1px solid var(--line)" }}
            >
              Agents load this tool list once per session: {num(t.listTokens)} tokens.
              <Button
                size="sm"
                kind="ghost"
                className="ml-auto"
                icon={<ScanEye size={13} />}
                onClick={() => setPreview(true)}
              >
                See what they see
              </Button>
            </div>
          )}
        </Panel>
      )}
      {tab === "agents" && <AgentsTab t={t} />}
      {tab === "requests" && <RequestsTab t={t} />}
      {tab === "guidelines" && <GuidelinesTab t={t} />}
      {tab === "settings" && <SettingsTab key={t.id} t={t} />}
      {adding && <AddFromLibrary t={t} onClose={() => setAdding(false)} />}
      {preview && <AgentPreview name={t.name} onClose={() => setPreview(false)} />}
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
  const confirm = useConfirm();
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
                  <Select
                    label="Access"
                    size="sm"
                    value={g.access}
                    onChange={(v) => upd.mutate({ id: g.id, body: { access: v } })}
                    options={[
                      { value: "read", label: "Read" },
                      { value: "read_write", label: "Read and write" },
                    ]}
                  />
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
                    onClick={async () =>
                      (await confirm({
                        title: `Remove ${g.agent.name} from ${t.name}?`,
                        body: "It can no longer see or call this toolkit's tools. Its other toolkits are unchanged.",
                        action: "Remove access",
                        danger: true,
                      })) && remove.mutate(g.id)
                    }
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
  const init = { desc: t.description, scope: t.buildScope, rules: t.rules, hints: t.hints };
  const [v, setV] = useState(init);
  const dirty = JSON.stringify(v) !== JSON.stringify(init);
  const rules = v.rules;
  const setRules = (r: Rules) => setV({ ...v, rules: r });
  const save = useAction(
    () =>
      patch(`/api/toolkits/${t.name}`, { description: v.desc, buildScope: v.scope, rules: v.rules, hints: v.hints }),
    { onSuccess: () => toast("Toolkit saved; connected agents get the change") },
  );
  return (
    <div style={{ maxWidth: 920 }}>
      <div className="section-title mt-0">Toolkit</div>
      <Panel>
        <div className="settings-row">
          <div className="what">
            <strong>Name</strong>
            <span>Part of the MCP server's URL, so it stays fixed once agents use it.</span>
          </div>
          <input className="input mono" value={t.name} readOnly aria-readonly />
        </div>
        <div className="settings-row">
          <div className="what">
            <strong>What it's for</strong>
            <span>Sent to agents as the server's instructions when they connect.</span>
          </div>
          <textarea
            className="textarea prose"
            rows={2}
            placeholder="Customer support: one customer's account, invoices and tickets"
            value={v.desc}
            onChange={(e) => setV({ ...v, desc: e.target.value })}
          />
        </div>
        <div className="settings-row">
          <div className="what">
            <strong>Applications agents may explore</strong>
            <span>When building tools. None checked means the applications its tools already use.</span>
          </div>
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {(apis.data ?? []).map((a) => (
              <label key={a.id} className="check">
                <input
                  type="checkbox"
                  checked={v.scope.includes(a.id)}
                  onChange={(e) =>
                    setV({ ...v, scope: e.target.checked ? [...v.scope, a.id] : v.scope.filter((x) => x !== a.id) })
                  }
                />
                <span>{a.title || a.name}</span>
              </label>
            ))}
            {apis.data?.length === 0 && <span className="muted">No applications yet.</span>}
          </div>
        </div>
        <div className="settings-row">
          <div className="what">
            <strong>Response hints</strong>
            <span>On large responses, one short line asks the agent to report which fields it used.</span>
          </div>
          <Toggle label="Response hints" checked={v.hints} onChange={(h) => setV({ ...v, hints: h })} />
        </div>
      </Panel>
      <div className="section-title">Tools that agents save</div>
      <Panel>
        <div className="settings-row">
          <div className="what">
            <strong>Go live without a person</strong>
            <span>
              When every rule below passes. Tools that write, or that use applications outside the list above, always
              wait for you.
            </span>
          </div>
          <Toggle
            label="Auto-approve"
            checked={rules.autoApprove}
            onChange={(x) => setRules({ ...rules, autoApprove: x })}
          />
        </div>
        <div className="settings-row">
          <div className="what">
            <strong>Rules</strong>
            <span>Checked on every save.</span>
          </div>
          <div className="flex flex-col gap-3">
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
            <label className="flex flex-wrap items-center gap-2">
              Output under
              <input
                className="input"
                style={{ width: 90 }}
                type="number"
                min={0}
                value={rules.maxOutputTokens}
                onChange={(e) => setRules({ ...rules, maxOutputTokens: Number(e.target.value) })}
              />
              tokens on its tests <span className="muted text-[12px]">(0 turns it off)</span>
            </label>
          </div>
        </div>
      </Panel>
      <ErrorBox error={save.error} />
      <SaveBar
        dirty={dirty}
        saving={save.isPending}
        onSave={() => save.mutate(undefined)}
        onDiscard={() => setV(init)}
      />
    </div>
  );
}

/** AgentPreview shows exactly what an agent with a given grant receives (TK-5). */
function AgentPreview({ name, onClose }: { name: string; onClose: () => void }) {
  const [access, setAccess] = useState<"read" | "read_write">("read");
  const [build, setBuild] = useState(false);
  const [client, setClient] = useState("claude-code");
  const [open, setOpen] = useState<string | null>(null);
  const q = useQuery({
    queryKey: ["agent-view", name, access, build, client],
    queryFn: () =>
      get<AgentView>(`/api/toolkits/${name}/agent-view?access=${access}&build=${build ? 1 : 0}&client=${client}`),
  });
  const v = q.data;
  return (
    <Dialog
      title="Preview as agent"
      sub="Exactly what an agent connected to this toolkit receives: the server's instructions and its tool list."
      onClose={onClose}
      wide
    >
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <Seg
          label="Access"
          value={access}
          onChange={setAccess}
          options={[
            { id: "read", label: "Read" },
            { id: "read_write", label: "Read and write" },
          ]}
        />
        <label className="flex items-center gap-2 text-[13px]">
          <Toggle label="May build tools" checked={build} onChange={setBuild} /> May build tools
        </label>
        <label className="ml-auto flex items-center gap-2 text-[13px]">
          <ClientIcon client={client} size={16} />
          <Select
            label="Client"
            size="sm"
            value={client}
            onChange={setClient}
            options={["claude-code", "vscode", "cursor", "claude-desktop", "other"].map((c) => ({
              value: c,
              label: clientLabels[c],
            }))}
          />
        </label>
      </div>
      <ErrorBox error={q.error} />
      {q.isLoading && <Loading />}
      {v && (
        <div className="flex flex-col gap-4">
          <div>
            <div className="mb-1.5 text-[12.5px] font-semibold">Instructions sent on connect</div>
            <pre className="code" style={{ whiteSpace: "pre-wrap" }}>
              {v.instructions}
            </pre>
          </div>
          <div>
            <div className="mb-1.5 flex items-center gap-2 text-[12.5px] font-semibold">
              Tool list <Chip>{v.tools.length} tools</Chip>
              <span className="muted font-normal">{num(v.tokens)} tokens, loaded once per session</span>
            </div>
            <div className="panel">
              {v.tools.map((x) => (
                <div key={x.name} className="list-row flex-col items-stretch gap-1">
                  <button
                    type="button"
                    className="flex items-center gap-2 text-left"
                    style={{ background: "none", border: 0, padding: 0, cursor: "pointer", color: "inherit" }}
                    aria-expanded={open === x.name}
                    onClick={() => setOpen(open === x.name ? null : x.name)}
                  >
                    <code className="font-semibold">{x.name}</code>
                    <span className="muted ml-auto text-[12px]">Input schema</span>
                    <ChevronDown
                      size={13}
                      className="muted"
                      style={{ transform: open === x.name ? "rotate(180deg)" : undefined }}
                    />
                  </button>
                  <div className="muted text-[12.5px] whitespace-pre-line">{x.description}</div>
                  {open === x.name && (
                    <pre className="code mt-1" style={{ maxHeight: 220 }}>
                      {JSON.stringify(x.inputSchema, null, 2)}
                    </pre>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </Dialog>
  );
}

/** ConnectCard: the toolkit's MCP endpoint, how to add it, and other transports (AGT-3). */
function ConnectCard({ t, onPreview, onConnect }: { t: ToolkitDetail; onPreview: () => void; onConnect: () => void }) {
  const [how, setHow] = useState<"http" | "builder" | "stdio">("http");
  const value = how === "http" ? t.endpoint : how === "builder" ? t.buildEndpoint : `mcpdeck stdio --toolkit ${t.name}`;
  const note = {
    http: "Streamable HTTP. Agents send their own token, so access and calls are tracked per agent.",
    builder: "For the builder subagent of agents that may build tools. Same token, separate tool list.",
    stdio: "For clients that only run commands. Reads the agent token from MCPDECK_TOKEN.",
  }[how];
  return (
    <section className="panel connect-card mb-5">
      <div className="connect-main">
        <div className="min-w-0 flex-1">
          <div className="mb-2 flex flex-wrap items-center gap-3">
            <span className="flex items-center gap-2 font-semibold">
              <span className="dot on" /> MCP endpoint
            </span>
            <Tabs
              variant="pill"
              value={how}
              onChange={setHow}
              tabs={[
                { id: "http", label: "HTTP" },
                { id: "builder", label: "Builder" },
                { id: "stdio", label: "Command (stdio)" },
              ]}
            />
          </div>
          <CopyField
            value={value}
            label={how === "stdio" ? "Command" : "Endpoint"}
            prefix={how === "stdio" ? "$" : how === "builder" ? "BUILD" : "POST"}
          />
          <div className="muted mt-2 text-[12.5px]">{note}</div>
        </div>
        <div className="connect-side">
          <div className="muted mb-2 text-[12px] font-medium">Add to a client</div>
          <div className="flex flex-wrap gap-2">
            {(["claude-code", "vscode", "cursor"] as const).map((c) => (
              <Button key={c} onClick={onConnect} icon={<ClientIcon client={c} size={15} />}>
                {clientLabels[c]}
              </Button>
            ))}
          </div>
          <div className="mt-3 flex gap-2">
            <Button size="sm" kind="ghost" icon={<ScanEye size={13} />} onClick={onPreview} data-tour="preview">
              Preview as agent
            </Button>
            <span className="muted self-center text-[12px]">tools/list: {num(t.listTokens)} tokens</span>
          </div>
        </div>
      </div>
    </section>
  );
}

/** ToolMenu: open, remove from this toolkit, or delete everywhere (type to confirm). */
function ToolMenu({ t, tool, toolkits }: { t: ToolkitDetail; tool: string; toolkits: string[] }) {
  const confirm = useConfirm();
  const toast = useToast();
  const removeHere = useAction(() => del(`/api/toolkits/${t.name}/tools/${tool}`), {
    onSuccess: () => toast(`${tool} removed from ${t.name}; it stays in the library`),
    onError: (e) => toast((e as Error).message, true),
  });
  const destroy = useAction(() => del(`/api/tools/${tool}`), {
    onSuccess: () => toast(`${tool} deleted`),
    onError: (e) => toast((e as Error).message, true),
  });
  const agents = t.grants.map((g) => g.agent.name);
  return (
    <Menu
      label={`Actions for ${tool}`}
      trigger={
        <button type="button" className="icon-btn" aria-label={`Actions for ${tool}`}>
          <MoreHorizontal size={16} />
        </button>
      }
      items={[
        { label: "Open", icon: <Code2 size={14} />, onClick: () => navigate(`/toolkits/${t.name}/tools/${tool}`) },
        {
          label: `Remove from ${t.name}`,
          icon: <Library size={14} />,
          onClick: async () =>
            (await confirm({
              title: `Remove ${tool} from ${t.name}?`,
              body: `Agents of ${t.name} stop seeing it on their next tools/list. It stays in the library${toolkits.length > 1 ? ` and in ${toolkits.filter((x) => x !== t.name).join(", ")}` : ""}.`,
              action: "Remove",
            })) && removeHere.mutate(undefined),
        },
        "sep",
        {
          label: "Delete tool",
          icon: <Trash2 size={14} />,
          danger: true,
          onClick: async () =>
            (await confirm({
              title: `Delete ${tool}?`,
              body: (
                <>
                  {agents.length > 0 && (
                    <div className="banner warn mb-3">
                      {agents.length} {agents.length === 1 ? "agent can" : "agents can"} call it now:{" "}
                      {agents.join(", ")}. Calls fail once it's deleted.
                    </div>
                  )}
                  It's removed from{" "}
                  {toolkits.length === 1 ? "this toolkit" : `${toolkits.length} toolkits (${toolkits.join(", ")})`} and
                  its versions are retired. Past calls stay in Activity.
                </>
              ),
              action: "Delete tool",
              danger: true,
              typeToConfirm: tool,
            })) && destroy.mutate(undefined),
        },
      ]}
    />
  );
}

/** GuidelinesTab: the toolkit's score, findings and the rules behind them (GUIDE-1, GUIDE-2, GUIDE-4). */
function GuidelinesTab({ t }: { t: ToolkitDetail }) {
  const q = useQuery({
    queryKey: ["lint", t.name, ""],
    queryFn: () => post<LintReport>(`/api/toolkits/${t.name}/lint`, {}),
  });
  const [rules, setRules] = useState(false);
  if (q.isLoading) return <Loading />;
  if (q.error) return <ErrorBox error={q.error} />;
  const r = q.data!;
  const c = counts(r.findings);
  return (
    <div className="flex flex-col gap-5">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div className="panel flex items-center gap-4 p-4">
          <ScoreRing score={r.score} />
          <div>
            <div className="font-semibold">Guideline score</div>
            <div className="muted text-[12.5px]">100 less 15 per must-fix, 5 per avoid</div>
          </div>
        </div>
        <StatCard
          icon={<Wrench size={14} />}
          label="Tools"
          value={r.tools}
          foot={r.tools > 15 ? "Over 15: consider splitting" : "Up to 15 keeps choices clear"}
        />
        <StatCard
          icon={<ScanEye size={14} />}
          label="tools/list"
          value={`${num(r.listTokens)} tok`}
          foot="Paid by every agent, every conversation"
        />
        <StatCard
          icon={<ShieldCheck size={14} />}
          label="Findings"
          value={c.block + c.warn}
          foot={
            <span>
              <span style={{ color: "var(--bad)" }}>{c.block} blocking</span> · {c.warn} warnings · {c.hint} hints
            </span>
          }
        />
      </div>
      <Panel title="What to fix" right={<span className="muted text-[12px]">Worst first. Open a tool to fix it.</span>}>
        <FindingList findings={r.findings} showTool empty="This toolkit follows the guidelines." />
      </Panel>
      <div>
        <Button
          kind="ghost"
          icon={<ChevronDown size={14} style={{ transform: rules ? "rotate(180deg)" : undefined }} />}
          onClick={() => setRules(!rules)}
        >
          {rules ? "Hide the rules" : "Show the rules and their sources"}
        </Button>
        {rules && (
          <div className="mt-3">
            <GuidelinesBody />
          </div>
        )}
      </div>
    </div>
  );
}
