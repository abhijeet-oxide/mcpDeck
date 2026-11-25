import { Bot, Layers, Plus } from "lucide-react";
import { useState } from "react";
import { useConnect } from "../App";
import {
  AccessChip,
  Button,
  Chip,
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
} from "../components/ui";
import { post } from "../lib/api";
import { num } from "../lib/format";
import { useAction, useAPIs, useLibrary, useToolkits } from "../lib/hooks";
import { Link, navigate, useLocation } from "../lib/router";
import type { Toolkit } from "../lib/types";

export function NewToolkitDialog({ onClose }: { onClose: () => void }) {
  const apis = useAPIs();
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");
  const [scope, setScope] = useState<string[]>([]);
  const create = useAction(() => post<Toolkit>("/api/toolkits", { name, description: desc, buildScope: scope }), {
    onSuccess: (t) => {
      onClose();
      navigate(`/toolkits/${t.name}`);
    },
  });
  return (
    <Dialog
      title="New toolkit"
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button kind="ink" disabled={!name || create.isPending} onClick={() => create.mutate(undefined)}>
            Create toolkit
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field
          label="Name"
          hint={
            <>
              Its MCP endpoint will be <code>/mcp/{name || "support"}</code>
            </>
          }
        >
          <input
            className="input mono"
            placeholder="support"
            value={name}
            onChange={(e) => setName(e.target.value.toLowerCase())}
          />
        </Field>
        <Field label="What it's for" hint="Agents read this when they connect.">
          <input
            className="input"
            placeholder="Customer support: one customer's account, invoices and tickets"
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
          />
        </Field>
        <Field
          label="APIs agents may explore when building"
          hint="You can change this later in the toolkit's settings."
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
            {apis.data?.length === 0 && <span className="muted">Add an API first.</span>}
          </div>
        </Field>
        <ErrorBox error={create.error} />
      </div>
    </Dialog>
  );
}

export function Toolkits() {
  const q = useToolkits();
  const lib = useLibrary();
  const { search } = useLocation();
  const [tab, setTab] = useState<"toolkits" | "library">("toolkits");
  const [creating, setCreating] = useState(search.get("new") === "1");
  const connect = useConnect();
  return (
    <div className="page">
      <PageHead
        title="Toolkits"
        sub="Each toolkit is one MCP server. Tools live in one library and can be in several toolkits."
        actions={
          <Button kind="ink" icon={<Plus size={14} />} onClick={() => setCreating(true)}>
            New toolkit
          </Button>
        }
      />
      <Tabs
        value={tab}
        onChange={setTab}
        tabs={[
          { id: "toolkits", label: "Toolkits", n: q.data?.length },
          { id: "library", label: "All tools", n: lib.data?.length },
        ]}
      />
      <ErrorBox error={q.error} />
      {q.isLoading && <Loading />}
      {tab === "toolkits" && q.data && (
        <div className="grid gap-3 lg:grid-cols-2">
          {q.data.length === 0 && (
            <Panel className="lg:col-span-2">
              <Empty title="No toolkits yet">
                <div className="mt-2">
                  <Button kind="ink" onClick={() => setCreating(true)}>
                    Create a toolkit
                  </Button>
                </div>
              </Empty>
            </Panel>
          )}
          {q.data.map((t) => (
            <Panel key={t.id}>
              <div className="flex items-start gap-3 p-3.5">
                <div className="icon-tile">
                  <Layers size={15} />
                </div>
                <div className="min-w-0 flex-1">
                  <Link
                    to={`/toolkits/${t.name}`}
                    className="mono font-semibold"
                    style={{ color: "var(--text)", fontSize: 14 }}
                  >
                    {t.name}
                  </Link>
                  <div className="muted truncate text-[12.5px]">{t.description || "No description"}</div>
                </div>
                <Button size="sm" icon={<Bot size={13} />} onClick={() => connect(t.id)}>
                  Connect
                </Button>
              </div>
              <div className="px-3.5">
                <CopyField value={t.endpoint} label="Endpoint" />
              </div>
              <div className="muted flex flex-wrap gap-x-4 gap-y-1 p-3.5 text-[12.5px]">
                <span>{t.tools} tools</span>
                <span>{t.agents} agents</span>
                <span title="Tokens an agent loads for tools/list, once per session">
                  {num(t.listTokens)} tokens per session
                </span>
                {t.pending > 0 && <Chip tone="warn">{t.pending} to review</Chip>}
                {t.requests > 0 && <Chip tone="info">{t.requests} requests</Chip>}
              </div>
            </Panel>
          ))}
        </div>
      )}
      {tab === "library" && (
        <Panel>
          {lib.data?.length === 0 && <Empty title="No tools yet">Tools are created inside a toolkit.</Empty>}
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
      {creating && <NewToolkitDialog onClose={() => setCreating(false)} />}
    </div>
  );
}
