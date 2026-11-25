import { CheckCircle2, ExternalLink, FileCode2, Loader2, ToolCase } from "lucide-react";
import { useEffect, useState } from "react";
import { post } from "../lib/api";
import { useAction, useAgent, useAgents, useToolkits } from "../lib/hooks";
import { navigate } from "../lib/router";
import type { ConnectResponse, Plan } from "../lib/types";
import { ClientIcon } from "./ClientIcon";
import {
  Button,
  Chip,
  CopyButton,
  CopyField,
  Dialog,
  EmptyState,
  ErrorBox,
  Field,
  Seg,
  Select,
  Toggle,
  useToast,
} from "./ui";

const clients = [
  { id: "claude-code", label: "Claude Code", sub: "claude mcp add + builder subagent" },
  { id: "vscode", label: "VS Code", sub: "Install link or mcp.json + custom agent" },
  { id: "cursor", label: "Cursor", sub: "Install link or mcp.json" },
  { id: "claude-desktop", label: "Claude Desktop", sub: "Through the stdio bridge" },
  { id: "other", label: "Other client", sub: "URL and header" },
];

/** Connect an agent (AGT-3): client, toolkit, access, build, skills, then the token once. */
export function ConnectDialog({
  toolkitId,
  agentId,
  onClose,
}: {
  toolkitId?: string;
  agentId?: string;
  onClose: () => void;
}) {
  const tks = useToolkits();
  const agents = useAgents();
  const existing = agents.data?.find((a) => a.id === agentId);
  const [client, setClient] = useState(existing?.client ?? "claude-code");
  const [tk, setTk] = useState(toolkitId ?? "");
  const [name, setName] = useState("");
  const [access, setAccess] = useState<"read" | "read_write">("read");
  const [build, setBuild] = useState(true);
  const [skills, setSkills] = useState(true);
  const [scope, setScope] = useState<"user" | "project">("user");
  const [project, setProject] = useState("");
  const [res, setRes] = useState<ConnectResponse | null>(null);
  const [plan, setPlan] = useState<Plan | null>(null);
  const [written, setWritten] = useState<string[] | null>(null);
  const [startedAt] = useState(() => Date.now());
  const toast = useToast();
  useEffect(() => {
    if (!tk && tks.data?.[0]) setTk(tks.data[0].id);
  }, [tks.data]);
  const go = useAction(
    () =>
      post<ConnectResponse>("/api/connect", {
        client,
        toolkitId: tk,
        agentId: agentId ?? "",
        name,
        access,
        canBuild: build,
        scope,
        projectDir: project,
        skills,
      }),
    {
      onSuccess: (r) => {
        setRes(r);
        setPlan(r.plan);
      },
    },
  );
  const replan = async (s: "user" | "project", dir: string, sk: boolean) => {
    if (!res) return;
    try {
      const r = await post<{ plan: Plan }>(`/api/plans/${res.planId}`, { scope: s, projectDir: dir, skills: sk });
      setPlan(r.plan);
    } catch (e) {
      toast((e as Error).message, true);
    }
  };
  const write = useAction(() => post<{ written: string[] }>(`/api/plans/${res!.planId}/apply`), {
    onSuccess: (r) => {
      setWritten(r.written);
      toast(`Wrote ${r.written.length} files; backups end in .mcpdeck-backup`);
    },
  });
  const live = useAgent(res?.result.agent.id ?? "", !!res);
  const connected = res && live.data?.lastSeenAt && new Date(live.data.lastSeenAt).getTime() > startedAt - 2000;
  const tkName = tks.data?.find((t) => t.id === tk)?.name ?? "";

  if (tks.data && tks.data.length === 0) {
    return (
      <Dialog title="Connect an agent" onClose={onClose}>
        <EmptyState
          icon={<ToolCase size={22} />}
          title="Create a toolkit first"
          actions={
            <Button
              kind="ink"
              onClick={() => {
                onClose();
                navigate("/toolkits?new=1");
              }}
            >
              Create toolkit
            </Button>
          }
        >
          An agent connects to a toolkit and gets its tools. Create one, then connect Claude Code, VS Code or Cursor to
          it.
        </EmptyState>
      </Dialog>
    );
  }
  if (!res) {
    return (
      <Dialog
        title={existing ? `Add a toolkit to ${existing.name}` : "Connect an agent"}
        sub="Pick the client, the toolkit it gets and what it may do. You'll get a command or a one-click link next."
        onClose={onClose}
        wide
        footer={
          <>
            <Button kind="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button kind="ink" disabled={!tk} loading={go.isPending} onClick={() => go.mutate(undefined)}>
              Connect
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-4">
          {!existing && (
            <div className="grid gap-2 sm:grid-cols-3">
              {clients.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className="choice"
                  style={{ padding: 12 }}
                  aria-pressed={client === c.id}
                  onClick={() => setClient(c.id)}
                >
                  <span className="icon-tile plain">
                    <ClientIcon client={c.id} size={18} />
                  </span>
                  <span className="min-w-0">
                    <span className="block font-semibold">{c.label}</span>
                    <span className="muted block text-[12px]">{c.sub}</span>
                  </span>
                </button>
              ))}
            </div>
          )}
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Toolkit">
              <Select
                label="Toolkit"
                className="w-full"
                value={tk}
                onChange={setTk}
                options={(tks.data ?? []).map((t) => ({ value: t.id, label: <span className="mono">{t.name}</span> }))}
              />
            </Field>
            {!existing && (
              <Field label="Name" hint="Reuses this client's agent if you have one">
                <input
                  className="input"
                  placeholder={`${clients.find((c) => c.id === client)?.label} on this laptop`}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </Field>
            )}
            <Field label="Access" hint="Read agents can't run or see tools that write.">
              <Seg
                label="Access"
                value={access}
                onChange={setAccess}
                options={[
                  { id: "read", label: "Read" },
                  { id: "read_write", label: "Read and write" },
                ]}
              />
            </Field>
            <Field label="Where to install">
              <Seg
                label="Scope"
                value={scope}
                onChange={setScope}
                options={[
                  { id: "user", label: "For me (user)" },
                  { id: "project", label: "One project" },
                ]}
              />
            </Field>
            {scope === "project" && (
              <Field label="Project folder" hint="Absolute path">
                <input
                  className="input mono"
                  placeholder="/home/you/code/support-bot"
                  value={project}
                  onChange={(e) => setProject(e.target.value)}
                />
              </Field>
            )}
          </div>
          <label className="flex items-start gap-3">
            <Toggle label="May build tools" checked={build} onChange={setBuild} />
            <span>
              <strong className="font-medium">May build tools</strong>
              <span className="muted block text-[12.5px]">
                The agent gets a builder subagent with its own endpoint for search_apis, run_api and save_tool.
                Exploring happens there, so the main conversation only sees the finished tool. Tools it saves follow
                this toolkit's approval rules.
              </span>
            </span>
          </label>
          <label className="flex items-start gap-3">
            <Toggle label="Install skills" checked={skills} onChange={setSkills} />
            <span>
              <strong className="font-medium">Install the mcpDeck skills</strong>
              <span className="muted block text-[12.5px]">
                Four short skills (use tools, request a tool, trim output, build a tool). They load only when relevant,
                so they cost nothing until used.
              </span>
            </span>
          </label>
          <ErrorBox error={go.error} />
        </div>
      </Dialog>
    );
  }
  return (
    <Dialog
      title={connected ? "Connected" : `Add ${tkName} to ${clients.find((c) => c.id === client)?.label}`}
      onClose={onClose}
      wide
    >
      <div className="flex flex-col gap-4">
        <div className="banner warn">
          This token is shown once. It's inside the command and config below; copy what you need now. Revoke it on the
          Agents page if it leaks.
        </div>
        {plan && (
          <>
            {plan.command && (
              <Field label="Run this">
                <div className="flex items-start gap-2">
                  <pre className="code flex-1" style={{ whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
                    {plan.command}
                  </pre>
                  <CopyButton text={plan.command} what="Command copied" />
                </div>
              </Field>
            )}
            {plan.link && (
              <div className="flex items-center gap-2">
                <a className="btn btn-blue" href={plan.link}>
                  <ExternalLink size={13} /> Install in {client === "vscode" ? "VS Code" : "Cursor"}
                </a>
                <span className="muted text-[12.5px]">
                  Links can be blocked by the browser; the config below always works.
                </span>
              </div>
            )}
            <div className="panel">
              <div className="panel-head">
                <FileCode2 size={15} className="muted" />
                <h2>Or let mcpDeck write the files</h2>
                <div className="right">
                  <Seg
                    label="Scope"
                    value={scope}
                    onChange={(s) => {
                      setScope(s);
                      replan(s, project, skills);
                    }}
                    options={[
                      { id: "user", label: "User" },
                      { id: "project", label: "Project" },
                    ]}
                  />
                </div>
              </div>
              {scope === "project" && (
                <div className="flex gap-2 p-3" style={{ borderBottom: "1px solid var(--line)" }}>
                  <input
                    className="input mono"
                    aria-label="Project folder"
                    placeholder="/home/you/code/support-bot"
                    value={project}
                    onChange={(e) => setProject(e.target.value)}
                  />
                  <Button size="sm" onClick={() => replan("project", project, skills)}>
                    Update
                  </Button>
                </div>
              )}
              {plan.files.map((f) => (
                <div key={f.path} className="list-row text-[12.5px]">
                  <Chip tone={f.kind === "subagent" ? "info" : undefined}>{f.kind}</Chip>
                  <span className="mono min-w-0 flex-1 truncate" title={f.path}>
                    {f.path}
                  </span>
                  <span className="muted truncate" style={{ maxWidth: 260 }}>
                    {f.exists ? "update · " : "new · "}
                    {f.summary}
                  </span>
                </div>
              ))}
              <div className="flex items-center gap-3 p-3">
                <Button kind="ink" disabled={write.isPending || !!written} onClick={() => write.mutate(undefined)}>
                  {written ? "Written" : "Write config for me"}
                </Button>
                <span className="muted text-[12px]">Existing entries are kept; each changed config gets a backup.</span>
              </div>
              <ErrorBox error={write.error} />
            </div>
            <details>
              <summary className="cursor-pointer text-[13px] font-medium">
                Config snippet for {plan.snippetPath}
              </summary>
              <div className="mt-2 flex items-start gap-2">
                <pre className="code flex-1" style={{ maxHeight: 220 }}>
                  {plan.snippet}
                </pre>
                <CopyButton text={plan.snippet} what="Snippet copied" />
              </div>
            </details>
            {plan.notes.map((n) => (
              <div key={n} className="muted text-[12.5px]">
                {n}
              </div>
            ))}
          </>
        )}
        <div className="panel flex items-center gap-3 p-3">
          {connected ? (
            <>
              <CheckCircle2 size={18} style={{ color: "var(--ok)" }} />
              <div className="min-w-0 flex-1">
                <div className="font-medium">
                  {live.data?.clientName || res.result.agent.name} connected to {tkName}
                </div>
                <div className="muted text-[12.5px]">First prompt to try:</div>
                <CopyField value={plan?.firstPrompt ?? ""} label="Prompt" />
              </div>
            </>
          ) : (
            <>
              <Loader2 size={16} className="muted animate-spin" />
              <span className="muted">
                Waiting for the client to connect… restart or reload the client after adding the server.
              </span>
            </>
          )}
        </div>
      </div>
    </Dialog>
  );
}
