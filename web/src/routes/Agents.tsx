import { Bot, KeyRound, Plus, ShieldOff } from "lucide-react";
import { useState } from "react";
import { useConnect } from "../App";
import { ClientIcon, clientLabels } from "../components/ClientIcon";
import {
  Button,
  Chip,
  CopyField,
  Dialog,
  Empty,
  EmptyState,
  ErrorBox,
  Loading,
  PageHead,
  Panel,
  useConfirm,
  useToast,
} from "../components/ui";
import { post } from "../lib/api";
import { ago, dateTime } from "../lib/format";
import { useAction, useAgent, useAgents, useToolkits } from "../lib/hooks";
import { Link, navigate } from "../lib/router";
import type { ConnectResponse } from "../lib/types";

export function Agents() {
  const q = useAgents();
  const connect = useConnect();
  const tks = useToolkits();
  const empty = q.data?.length === 0;
  return (
    <div className="page">
      <PageHead
        title="Agents"
        sub="The AI clients you've connected, like Claude Code on this laptop. Each has its own token and its own access to toolkits."
        actions={
          !empty && (
            <Button kind="ink" icon={<Plus size={14} />} onClick={() => connect()}>
              Connect agent
            </Button>
          )
        }
      />
      <ErrorBox error={q.error} />
      {q.isLoading && <Loading />}
      {empty && (
        <Panel>
          <EmptyState
            icon={<Bot size={22} />}
            title="Connect your first agent"
            actions={
              tks.data?.length ? (
                <Button kind="ink" icon={<Plus size={14} />} onClick={() => connect()}>
                  Connect agent
                </Button>
              ) : (
                <Button kind="ink" icon={<Plus size={14} />} onClick={() => navigate("/toolkits?new=1")}>
                  Create a toolkit first
                </Button>
              )
            }
          >
            Add a toolkit to VS Code, Claude Code or Cursor with one command or one click. You choose whether the agent
            may only read, or read and write.
          </EmptyState>
          <div className="flex justify-center gap-6 pb-8">
            {["vscode", "claude-code", "cursor", "claude-desktop"].map((c) => (
              <span key={c} className="muted flex items-center gap-2 text-[12.5px]">
                <ClientIcon client={c} size={18} /> {clientLabels[c]}
              </span>
            ))}
          </div>
        </Panel>
      )}
      {!!q.data?.length && (
        <Panel>
          <table className="table">
            <thead>
              <tr>
                <th>Agent</th>
                <th>Toolkits and access</th>
                <th>Added</th>
                <th>Last used</th>
                <th className="num">Calls today</th>
                <th className="num">Errors</th>
              </tr>
            </thead>
            <tbody>
              {q.data.map((a) => (
                <tr key={a.id} className="clickable" onClick={() => navigate(`/agents/${a.id}`)}>
                  <td>
                    <div className="flex items-center gap-3">
                      <span className="icon-tile plain">
                        <ClientIcon client={a.client} size={17} />
                      </span>
                      <div className="min-w-0">
                        <Link to={`/agents/${a.id}`} className="font-semibold" style={{ color: "var(--text)" }}>
                          {a.name}
                        </Link>
                        <div className="muted text-[12px]">
                          {a.clientLabel} · token <code>{a.tokenPrefix}…</code>
                        </div>
                      </div>
                    </div>
                  </td>
                  <td>
                    <div className="flex flex-wrap gap-1">
                      {a.grants.length === 0 && <span className="muted">None</span>}
                      {a.grants.map((g) => (
                        <Chip key={g.id} tone={g.access === "read_write" ? "warn" : "info"}>
                          {g.toolkitName} · {g.access === "read_write" ? "read and write" : "read"}
                          {g.canBuild && " · builds"}
                        </Chip>
                      ))}
                    </div>
                  </td>
                  <td className="muted text-[12.5px]">
                    <div className="text-2">{dateTime(a.createdAt)}</div>
                    by you
                  </td>
                  <td>
                    {a.revokedAt ? (
                      <Chip tone="bad">revoked</Chip>
                    ) : (
                      <span className="flex items-center gap-2 text-[12.5px]">
                        <span className={`dot ${a.connected ? "on" : ""}`} />
                        {a.connected ? "Connected" : a.lastSeenAt ? ago(a.lastSeenAt) : "Never"}
                      </span>
                    )}
                  </td>
                  <td className="num">{a.callsToday || "–"}</td>
                  <td className="num" style={a.errorsToday ? { color: "var(--bad)" } : undefined}>
                    {a.errorsToday || "–"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Panel>
      )}
    </div>
  );
}

export function AgentPage({ id }: { id: string }) {
  const q = useAgent(id);
  const connect = useConnect();
  const toast = useToast();
  const confirm = useConfirm();
  const [rotated, setRotated] = useState<ConnectResponse | null>(null);
  const revoke = useAction(() => post(`/api/agents/${id}/revoke`), {
    onSuccess: () => toast("Revoked: the token is refused from now on"),
  });
  const rotate = useAction(() => post<ConnectResponse>(`/api/agents/${id}/rotate`, {}), {
    onSuccess: (r) => setRotated(r),
  });
  const write = useAction(() => post<{ written: string[] }>(`/api/plans/${rotated!.planId}/apply`), {
    onSuccess: (r) => toast(`Rewrote ${r.written.length} files`),
  });
  if (q.isLoading) return <Loading />;
  if (q.error)
    return (
      <div className="page">
        <ErrorBox error={q.error} />
      </div>
    );
  const a = q.data!;
  return (
    <div className="page">
      <PageHead
        icon={
          <span className="head-icon">
            <ClientIcon client={a.client} size={22} />
          </span>
        }
        crumbs={
          <>
            <Link to="/agents">Agents</Link> / {a.name}
          </>
        }
        title={a.name}
        sub={
          <>
            {a.clientLabel}
            {a.clientName && ` (${a.clientName})`} · added {dateTime(a.createdAt)} · token <code>{a.tokenPrefix}…</code>
          </>
        }
        actions={
          !a.revokedAt && (
            <>
              <Button icon={<Plus size={14} />} onClick={() => connect(undefined, a.id)}>
                Add a toolkit
              </Button>
              <Button icon={<KeyRound size={14} />} onClick={() => rotate.mutate(undefined)}>
                Rotate token
              </Button>
              <Button
                kind="bad"
                icon={<ShieldOff size={14} />}
                onClick={async () =>
                  (await confirm({
                    title: `Revoke ${a.name}?`,
                    body: "Its token stops working at once, for every toolkit. You can connect it again later.",
                    action: "Revoke",
                    danger: true,
                  })) && revoke.mutate(undefined)
                }
              >
                Revoke
              </Button>
            </>
          )
        }
      />
      {a.revokedAt && <div className="banner bad mb-3">Revoked {dateTime(a.revokedAt)}. Its token is refused.</div>}
      <div className="muted mb-4 text-[12.5px]">
        The token is stored in the client's config in plain text, as MCP clients require. Revoking stops it at once; it
        never gives access to your API keys.
      </div>
      <div className="grid items-start gap-4 lg:grid-cols-2">
        <Panel title="Toolkits">
          {a.grants.length === 0 && <Empty title="No toolkits" />}
          {a.grants.map((g) => (
            <div key={g.id} className="list-row">
              <Link to={`/toolkits/${g.toolkitName}?tab=agents`} className="mono font-semibold">
                {g.toolkitName}
              </Link>
              <Chip tone={g.access === "read_write" ? "warn" : "info"}>
                {g.access === "read_write" ? "read and write" : "read"}
              </Chip>
              {g.canBuild && <Chip>may build</Chip>}
              <Link to={`/activity?agent=${a.id}`} className="ml-auto text-[12.5px]">
                Calls
              </Link>
            </div>
          ))}
        </Panel>
        <Panel title="Sessions">
          {a.sessions.length === 0 && <Empty title="No sessions yet">A session starts when the client connects.</Empty>}
          {a.sessions.slice(0, 12).map((s) => (
            <Link
              key={s.id}
              to={`/activity?session=${s.id}`}
              className="list-row"
              style={{ color: "inherit", textDecoration: "none" }}
            >
              <Chip>{s.mode}</Chip>
              <span className="min-w-0 flex-1 truncate text-[13px]">
                {s.clientName || "client"} {s.clientVersion}
              </span>
              <span className="muted text-[12px]">
                {dateTime(s.startedAt)} → {ago(s.lastSeenAt)}
              </span>
            </Link>
          ))}
        </Panel>
      </div>
      <ErrorBox error={revoke.error || rotate.error} />
      {rotated && (
        <Dialog title="New token" onClose={() => setRotated(null)}>
          <div className="flex flex-col gap-3">
            <div className="banner warn">Shown once. The old token stopped working.</div>
            <CopyField value={rotated.result.token} label="Token" />
            <div className="flex items-center gap-3">
              <Button kind="ink" disabled={write.isPending || write.isSuccess} onClick={() => write.mutate(undefined)}>
                {write.isSuccess ? "Config rewritten" : "Rewrite the client config"}
              </Button>
              <span className="muted text-[12.5px]">
                Updates {rotated.plan.files.filter((f) => f.kind === "config" || f.kind === "subagent").length} files
                with a backup.
              </span>
            </div>
            <ErrorBox error={write.error} />
          </div>
        </Dialog>
      )}
    </div>
  );
}
