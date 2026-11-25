import { AppWindow, ArrowRight, Bot, Info, Plus, ToolCase, Wrench } from "lucide-react";
import { useState } from "react";
import { useConnect } from "../App";
import {
  AccessChip,
  Avatar,
  Button,
  Chip,
  Dialog,
  Empty,
  EmptyState,
  ErrorBox,
  Field,
  Loading,
  PageHead,
  Panel,
  StatusChip,
  Tabs,
} from "../components/ui";
import { post } from "../lib/api";
import { num } from "../lib/format";
import { useAction, useAPIs, useLibrary, useToolkits } from "../lib/hooks";
import { toolkitNameError, toolkitSlug } from "../lib/names";
import { Link, navigate, useLocation } from "../lib/router";
import type { Toolkit } from "../lib/types";

export function NewToolkitDialog({ onClose }: { onClose: () => void }) {
  const apis = useAPIs();
  const tks = useToolkits();
  const [raw, setRaw] = useState("");
  const [desc, setDesc] = useState("");
  const [scope, setScope] = useState<string[] | null>(null);
  const name = toolkitSlug(raw);
  const err = toolkitNameError(
    raw,
    (tks.data ?? []).map((t) => t.name),
  );
  const chosen = scope ?? (apis.data ?? []).map((a) => a.id);
  const create = useAction(() => post<Toolkit>("/api/toolkits", { name, description: desc, buildScope: chosen }), {
    onSuccess: (t) => {
      onClose();
      navigate(`/toolkits/${t.name}`);
    },
  });
  const ok = name.length >= 2 && !err;
  return (
    <Dialog
      title="New toolkit"
      sub="A toolkit is a set of tools for one job. Agents you connect to it get its tools, and nothing else."
      onClose={onClose}
      footer={
        <>
          <Button kind="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button kind="ink" disabled={!ok} loading={create.isPending} onClick={() => create.mutate(undefined)}>
            Create toolkit
          </Button>
        </>
      }
    >
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (ok) create.mutate(undefined);
        }}
      >
        <Field
          label="Name"
          error={err}
          hint={
            <>
              {raw && name !== raw ? (
                <>
                  Saved as <code>{name}</code>.{" "}
                </>
              ) : null}
              Its MCP server will be at <code>/mcp/{name || "support"}</code>
            </>
          }
        >
          <input
            className="input"
            placeholder="Support"
            aria-invalid={!!err}
            maxLength={48}
            value={raw}
            onChange={(e) => setRaw(e.target.value)}
          />
        </Field>
        <Field label="What it's for" hint="Agents read this when they connect, so write it for them.">
          <input
            className="input"
            placeholder="Customer support: one customer's account, invoices and tickets"
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
          />
        </Field>
        <Field
          label="Applications its agents may explore when building tools"
          hint="Agents only call applications through tools. This list limits what a building agent may look at."
        >
          <div className="flex flex-wrap gap-x-4 gap-y-2 pt-1">
            {(apis.data ?? []).map((a) => (
              <label key={a.id} className="check">
                <input
                  type="checkbox"
                  checked={chosen.includes(a.id)}
                  onChange={(e) => setScope(e.target.checked ? [...chosen, a.id] : chosen.filter((x) => x !== a.id))}
                />
                <span>{a.title || a.name}</span>
              </label>
            ))}
            {apis.data?.length === 0 && (
              <span className="muted">No applications yet. You can add them later in the toolkit's settings.</span>
            )}
          </div>
        </Field>
        <ErrorBox error={create.error} />
        <button type="submit" hidden />
      </form>
    </Dialog>
  );
}

export function Toolkits() {
  const q = useToolkits();
  const lib = useLibrary();
  const { search } = useLocation();
  const [tab, setTab] = useState<"toolkits" | "library">(search.get("tab") === "library" ? "library" : "toolkits");
  const [creating, setCreating] = useState(search.get("new") === "1");
  const connect = useConnect();
  const empty = q.data?.length === 0;
  return (
    <div className="page">
      <PageHead
        title="Toolkits"
        sub="A toolkit is a set of tools for one job, served as its own MCP server. Connect an agent to a toolkit and it gets those tools."
        actions={
          !empty && (
            <Button kind="ink" icon={<Plus size={14} />} onClick={() => setCreating(true)}>
              New toolkit
            </Button>
          )
        }
      />
      {!empty && (
        <Tabs
          value={tab}
          onChange={setTab}
          tabs={[
            { id: "toolkits", label: "Toolkits", n: q.data?.length },
            { id: "library", label: "All tools", n: lib.data?.length },
          ]}
        />
      )}
      <ErrorBox error={q.error} />
      {q.isLoading && <Loading />}
      {empty && (
        <Panel>
          <EmptyState
            icon={<ToolCase size={22} />}
            title="Create your first toolkit"
            actions={
              <Button kind="ink" icon={<Plus size={14} />} onClick={() => setCreating(true)}>
                New toolkit
              </Button>
            }
          >
            Group the tools for one job, like support or finance. Each toolkit gets one URL that you add to Claude Code,
            VS Code or Cursor.
          </EmptyState>
          <div
            className="grid gap-px sm:grid-cols-3"
            style={{ background: "var(--line)", borderTop: "1px solid var(--line)" }}
          >
            {[
              { icon: AppWindow, t: "1. Applications", d: "The systems tools call. Add them first." },
              { icon: Wrench, t: "2. Tools", d: "Short TypeScript functions that return only what's needed." },
              { icon: Bot, t: "3. Agents", d: "Connect a client; it sees the toolkit's tools and nothing else." },
            ].map(({ icon: I, t, d }) => (
              <div key={t} className="flex gap-3 p-4" style={{ background: "var(--surface)" }}>
                <I size={16} className="muted mt-0.5 flex-none" />
                <div>
                  <div className="font-medium">{t}</div>
                  <div className="muted text-[12.5px]">{d}</div>
                </div>
              </div>
            ))}
          </div>
        </Panel>
      )}
      {tab === "toolkits" && !!q.data?.length && (
        <div className="grid gap-4 lg:grid-cols-2">
          {q.data.map((t) => (
            <div key={t.id} className="panel card-link flex flex-col">
              <Link to={`/toolkits/${t.name}`} className="flex items-start gap-3 p-4" style={{ color: "inherit" }}>
                <Avatar name={t.name} />
                <div className="min-w-0 flex-1">
                  <div className="mono text-[14px] font-semibold">{t.name}</div>
                  <div className="muted line-clamp-2 text-[12.5px]">
                    {t.description || "No description yet. Add one in the toolkit's settings."}
                  </div>
                </div>
                <ArrowRight size={16} className="muted mt-1" />
              </Link>
              <div className="flex flex-wrap items-center gap-1.5 px-4">
                {t.pending > 0 && <Chip tone="warn">{t.pending} to review</Chip>}
                {t.requests > 0 && <Chip tone="info">{t.requests} tool requests</Chip>}
                {t.scopeNames.map((s) => (
                  <span key={s} className="tag">
                    {s}
                  </span>
                ))}
              </div>
              <div
                className="muted mt-4 flex items-center gap-4 px-4 py-3 text-[12px]"
                style={{ borderTop: "1px solid var(--line)" }}
              >
                <span>
                  <strong className="text-2 font-semibold">{t.tools}</strong> tools
                </span>
                <span>
                  <strong className="text-2 font-semibold">{t.agents}</strong> agents
                </span>
                <span title="Tokens an agent loads for the tool list, once per session">
                  <strong className="text-2 font-semibold">{num(t.listTokens)}</strong> tokens per session
                </span>
                <Button size="sm" className="ml-auto" icon={<Bot size={13} />} onClick={() => connect(t.id)}>
                  Connect agent
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
      {tab === "library" && (
        <Panel>
          <div
            className="muted flex items-center gap-2 px-4 py-3 text-[12.5px]"
            style={{ borderBottom: "1px solid var(--line)" }}
          >
            <Info size={14} /> Every tool lives here once and can be in several toolkits. Tools are created inside a
            toolkit.
          </div>
          {lib.data?.length === 0 && <Empty title="No tools yet">Open a toolkit and choose New tool.</Empty>}
          {!!lib.data?.length && (
            <table className="table">
              <thead>
                <tr>
                  <th>Tool</th>
                  <th>Toolkits</th>
                  <th>Access</th>
                  <th>Version</th>
                  <th className="num">Calls 7d</th>
                  <th className="num">Avg output</th>
                </tr>
              </thead>
              <tbody>
                {lib.data.map((t) => (
                  <tr
                    key={t.id}
                    className="clickable"
                    onClick={() => navigate(`/toolkits/${t.toolkits[0] ?? "_"}/tools/${t.name}`)}
                  >
                    <td>
                      <div className="mono font-semibold">{t.name}</div>
                      <div className="muted truncate text-[12.5px]" style={{ maxWidth: 420 }}>
                        {t.latest.description}
                      </div>
                    </td>
                    <td>
                      {t.toolkits.map((k) => (
                        <span key={k} className="tag mr-1">
                          {k}
                        </span>
                      ))}
                    </td>
                    <td>
                      <AccessChip access={t.latest.access} />
                    </td>
                    <td>
                      <span className="flex items-center gap-1.5">
                        v{t.latest.n} <StatusChip status={t.latest.status} />
                      </span>
                    </td>
                    <td className="num">{t.calls7d}</td>
                    <td className="num">{t.avgTokens ? `${t.avgTokens} tok` : "–"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Panel>
      )}
      {creating && (
        <NewToolkitDialog
          onClose={() => {
            setCreating(false);
            if (search.get("new")) navigate("/toolkits", true);
          }}
        />
      )}
    </div>
  );
}
