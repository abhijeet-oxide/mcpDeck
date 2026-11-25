import { Bot, KeyRound, Plus } from "lucide-react";
import { useState } from "react";
import { useConnect } from "../App";
import { Button, Chip, CopyField, Dialog, Empty, ErrorBox, Loading, PageHead, Panel, useToast } from "../components/ui";
import { post } from "../lib/api";
import { ago, dateTime } from "../lib/format";
import { useAction, useAgent, useAgents } from "../lib/hooks";
import { Link, navigate } from "../lib/router";
import type { ConnectResponse } from "../lib/types";

export function Agents() {
  const q = useAgents();
  const connect = useConnect();
  return (
    <div className="page">
      <PageHead
        title="Agents"
        sub="One row per client you've connected, and what each may do."
        actions={
          <Button kind="ink" icon={<Bot size={14} />} onClick={() => connect()}>
            Connect agent
          </Button>
        }
      />
      <ErrorBox error={q.error} />
      {q.isLoading && <Loading />}
      {q.data && (
        <Panel>
          {q.data.length === 0 ? (
            <Empty title="No agents yet">
              <div className="mt-2">
                <Button kind="ink" onClick={() => connect()}>
                  Connect your first agent
                </Button>
              </div>
            </Empty>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Agent</th>
                  <th>Toolkits</th>
                  <th>Last seen</th>
                  <th className="num">Calls today</th>
                  <th className="num">Errors</th>
                </tr>
              </thead>
              <tbody>
                {q.data.map((a) => (
                  <tr key={a.id} className="clickable" onClick={() => navigate(`/agents/${a.id}`)}>
                    <td>
                      <div className="font-medium">{a.name}</div>
                      <div className="muted text-[12px]">
                        {a.clientLabel} · token <code>{a.tokenPrefix}…</code>
                      </div>
                    </td>
                    <td>
                      <div className="flex flex-wrap gap-1">
                        {a.grants.map((g) => (
                          <Chip key={g.id} tone={g.access === "read_write" ? "warn" : "info"}>
                            {g.toolkitName} · {g.access === "read_write" ? "read+write" : "read"}
                            {g.canBuild && " · builds"}
                          </Chip>
                        ))}
                      </div>
                    </td>
                    <td>
                      {a.revokedAt ? (
                        <Chip tone="bad">revoked</Chip>
                      ) : (
                        <span className="flex items-center gap-1.5">
                          <span className={`dot ${a.connected ? "on" : ""}`} />
                          {ago(a.lastSeenAt)}
                        </span>
                      )}
                    </td>
                    <td className="num">{a.callsToday}</td>
                    <td className="num">{a.errorsToday || "–"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Panel>
      )}
    </div>
  );
}

export function AgentPage({ id }: { id: string }) {
  const q = useAgent(id);
  const connect = useConnect();
  const toast = useToast();
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
        crumbs={
          <>
            <Link to="/agents">Agents</Link> › {a.name}
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
                onClick={() =>
                  confirm(`Revoke ${a.name}? Its token stops working at once.`) && revoke.mutate(undefined)
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
