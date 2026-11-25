import { Code2, Copy, Download, KeyRound, Plus, Save, Send, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AskAgentButton } from "../components/AskAgent";
import { KVEditor } from "../components/KVEditor";
import {
  AccessChip,
  Button,
  Chip,
  Empty,
  ErrorBox,
  Field,
  Loading,
  MethodChip,
  PageHead,
  Panel,
  Seg,
  Tabs,
  useCopy,
  useToast,
} from "../components/ui";
import { del, patch, post, put } from "../lib/api";
import { ago, bytes, mod, num, pretty } from "../lib/format";
import { useAction, useAPI, useHistory, useToolkits } from "../lib/hooks";
import { Link, navigate, useLocation } from "../lib/router";
import type { APIDetail, Auth, Request, SendResult, Variable } from "../lib/types";

type Tab = "requests" | "environments" | "auth" | "settings";

export function APIWorkspace({ id }: { id: string }) {
  const q = useAPI(id);
  const { search } = useLocation();
  const [tab, setTab] = useState<Tab>((search.get("tab") as Tab) || "requests");
  const setActive = useAction((envId: string) => patch(`/api/apis/${id}`, { activeEnvId: envId }));
  if (q.isLoading) return <Loading />;
  if (q.error)
    return (
      <div className="page">
        <ErrorBox error={q.error} />
      </div>
    );
  const a = q.data!;
  const imported = search.get("imported");
  const missing = a.variables.filter((v) => v.source === "secret" && !v.isSet);
  const first = a.requests[0]?.name ?? "getAccount";
  return (
    <div className="page page-wide">
      <PageHead
        crumbs={
          <>
            <Link to="/apis">APIs</Link> › {a.name}
          </>
        }
        title={a.name}
        mono
        sub={
          <>
            {a.title} · {a.source} · tools call it as{" "}
            <code>
              {a.name}.{first}()
            </code>
          </>
        }
        actions={
          <>
            <select
              className="select"
              style={{ width: 150 }}
              aria-label="Environment"
              value={a.activeEnvId}
              onChange={(e) => setActive.mutate(e.target.value)}
            >
              {a.environments.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
            </select>
            <AskAgentButton
              prompts={[
                {
                  title: `Explore ${a.name}`,
                  needsBuild: true,
                  text: `With the mcpDeck builder, run search_apis "${a.name}" and describe_api ${a.name}.${first}.\nTell me which three requests would make the most useful tool and what it should return.\nCheck: you only used run_api for reads.`,
                },
              ]}
            />
            <Button icon={<Download size={14} />} onClick={() => navigate(`/apis/${id}/import`)}>
              Import
            </Button>
          </>
        }
      />
      {imported && (
        <div className="banner ok mb-3">
          Imported {imported} requests.
          {missing.length > 0 && " Add the credential on the Auth and credentials tab before sending."}
        </div>
      )}
      {!imported && missing.length > 0 && (
        <div className="banner warn mb-3 items-center">
          <KeyRound size={15} /> No value for {missing.map((m) => m.name).join(", ")} yet.
          <Button size="sm" className="ml-auto" onClick={() => setTab("auth")}>
            Add credential
          </Button>
        </div>
      )}
      <Tabs<Tab>
        value={tab}
        onChange={setTab}
        tabs={[
          { id: "requests", label: "Requests", n: a.requests.length },
          { id: "environments", label: "Environments", n: a.environments.length },
          { id: "auth", label: "Auth and credentials" },
          { id: "settings", label: "Settings" },
        ]}
      />
      {tab === "requests" && <Requests a={a} />}
      {tab === "environments" && <Environments a={a} />}
      {tab === "auth" && <AuthTab a={a} />}
      {tab === "settings" && <SettingsTab a={a} />}
    </div>
  );
}

const blank = (apiId: string): Request => ({
  id: "",
  apiId,
  folder: "",
  name: "newRequest",
  title: "",
  description: "",
  method: "GET",
  url: "{{baseUrl}}/",
  headers: [],
  query: [],
  bodyType: "none",
  body: "",
  access: "read",
  accessReason: "",
  accessFlag: false,
});

function curlOf(r: Request): string {
  const q = r.query.filter((k) => !k.disabled && k.value);
  const url = r.url + (q.length ? `?${q.map((k) => `${k.key}=${encodeURIComponent(k.value)}`).join("&")}` : "");
  let s = `curl -X ${r.method} '${url}'`;
  for (const h of r.headers.filter((h) => !h.disabled)) s += ` \\\n  -H '${h.key}: ${h.value}'`;
  if (r.body && r.bodyType !== "none") s += ` \\\n  --data '${r.body.replace(/'/g, "'\\''")}'`;
  return s;
}

function toolCallOf(api: string, r: Request): string {
  const params = [...r.url.matchAll(/\{([A-Za-z0-9_]+)\}/g)].map((m) => m[1]).filter((p) => p !== "baseUrl");
  const args = params.map((p) => `${p}: ""`).join(", ");
  return `await ${api}.${r.name}(${args ? `{ ${args} }` : ""})`;
}

function Requests({ a }: { a: APIDetail }) {
  const [sel, setSel] = useState<string>(a.requests[0]?.id ?? "");
  const [side, setSide] = useState<"requests" | "history">("requests");
  const [filter, setFilter] = useState("");
  const [draft, setDraft] = useState<Request | null>(null);
  const hist = useHistory(a.id);
  useEffect(() => {
    const r = a.requests.find((x) => x.id === sel);
    setDraft(
      r ? structuredClone(r) : sel === "new" ? blank(a.id) : (a.requests[0] && structuredClone(a.requests[0])) || null,
    );
  }, [sel, a]);
  const folders = useMemo(() => {
    const m = new Map<string, Request[]>();
    for (const r of a.requests) {
      if (filter && !`${r.name} ${r.url} ${r.title}`.toLowerCase().includes(filter.toLowerCase())) continue;
      const f = r.folder || "";
      m.set(f, [...(m.get(f) ?? []), r]);
    }
    return [...m.entries()];
  }, [a.requests, filter]);
  return (
    <div className="grid items-start gap-4" style={{ gridTemplateColumns: "270px minmax(0, 1fr)" }}>
      <Panel bodyClass="p-2">
        <div className="mb-2 flex gap-2">
          <Seg
            label="List"
            value={side}
            onChange={setSide}
            options={[
              { id: "requests", label: "Requests" },
              { id: "history", label: "History" },
            ]}
          />
          <Button
            size="sm"
            kind="ghost"
            className="ml-auto"
            aria-label="New request"
            icon={<Plus size={14} />}
            onClick={() => setSel("new")}
          />
        </div>
        {side === "requests" ? (
          <>
            <input
              className="input mb-2"
              placeholder="Filter requests"
              aria-label="Filter requests"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
            />
            {a.requests.length === 0 && <Empty title="No requests">Import some, or add one with +.</Empty>}
            {folders.map(([f, rs]) => (
              <div key={f} className="mb-1">
                {f && <div className="muted px-2 py-1 text-[11.5px] font-semibold uppercase tracking-wide">{f}</div>}
                {rs.map((r) => (
                  <button
                    key={r.id}
                    type="button"
                    className="nav-item"
                    style={{ padding: "4px 8px" }}
                    aria-current={sel === r.id || (!sel && r === a.requests[0]) ? "page" : undefined}
                    onClick={() => setSel(r.id)}
                  >
                    <MethodChip method={r.method} />
                    <span className="truncate">{r.name}</span>
                    {r.accessFlag && <span className="dot warn ml-auto" title="Check access" />}
                  </button>
                ))}
              </div>
            ))}
          </>
        ) : (
          <div className="flex flex-col">
            {(hist.data ?? []).length === 0 && <Empty title="No sends yet">Everything you send shows up here.</Empty>}
            {(hist.data ?? []).map((c) => (
              <button
                key={c.id}
                type="button"
                className="nav-item flex-col items-start gap-0"
                style={{ padding: "5px 8px" }}
                onClick={() => {
                  setSide("requests");
                  if (c.requestId) setSel(c.requestId);
                }}
              >
                <span className="flex w-full items-center gap-2">
                  <MethodChip method={c.method ?? "GET"} />
                  <span className="truncate">{c.name.split(".").pop()}</span>
                  <span
                    className={`ml-auto text-[11.5px] ${c.status === "ok" ? "muted" : ""}`}
                    style={c.status !== "ok" ? { color: "var(--bad)" } : undefined}
                  >
                    {c.httpStatus || c.status}
                  </span>
                </span>
                <span className="muted text-[11.5px]">{ago(c.startedAt)}</span>
              </button>
            ))}
          </div>
        )}
      </Panel>
      {draft ? (
        <Editor key={draft.id || "new"} api={a} initial={draft} onSaved={(id) => setSel(id)} />
      ) : (
        <Panel>
          <Empty title="Pick a request" />
        </Panel>
      )}
    </div>
  );
}

function Editor({ api, initial, onSaved }: { api: APIDetail; initial: Request; onSaved: (id: string) => void }) {
  const [r, setR] = useState<Request>(initial);
  const [sub, setSub] = useState<"params" | "headers" | "body" | "docs">(initial.body ? "body" : "params");
  const [res, setRes] = useState<SendResult | null>(null);
  const [view, setView] = useState<"pretty" | "raw" | "headers">("pretty");
  const [sending, setSending] = useState(false);
  const copy = useCopy();
  const toast = useToast();
  const tks = useToolkits();
  const dirty = JSON.stringify(r) !== JSON.stringify(initial);
  const save = useAction(
    () => (r.id ? put<Request>(`/api/requests/${r.id}`, r) : post<Request>(`/api/apis/${api.id}/requests`, r)),
    {
      onSuccess: (x) => {
        toast(`Saved ${api.name}.${x.name}`);
        onSaved(x.id);
      },
    },
  );
  const remove = useAction(() => del(`/api/requests/${r.id}`), {
    onSuccess: () => {
      toast("Request deleted");
      onSaved("");
    },
  });
  const setAccess = useAction((acc: string) => post<Request>(`/api/requests/${r.id}/access`, { access: acc }), {
    onSuccess: (x) => setR((cur) => ({ ...cur, access: x.access, accessFlag: false, accessReason: x.accessReason })),
  });
  const saveExample = useAction(() => post(`/api/requests/${r.id}/example`, { body: JSON.parse(res!.body) }), {
    onSuccess: () => toast("Saved as example; values that look like secrets were masked"),
  });
  const send = async () => {
    setSending(true);
    try {
      setRes(await post<SendResult>("/api/send", { apiId: api.id, draft: r }));
    } catch (e) {
      setRes({ body: "", tokens: 0, error: (e as Error).message });
    } finally {
      setSending(false);
    }
  };
  useEffect(() => {
    const k = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      if (e.key === "Enter") {
        e.preventDefault();
        send();
      }
      if (e.key.toLowerCase() === "s" && !e.shiftKey) {
        e.preventDefault();
        save.mutate(undefined);
      }
    };
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  });
  const unresolved = [
    ...`${r.url} ${r.body} ${r.headers.map((h) => h.value).join(" ")}`.matchAll(/\{\{\s*([\w.-]+)\s*\}\}/g),
  ]
    .map((m) => m[1])
    .filter((n) => !api.variables.some((v) => v.name === n && (v.envId === "" || v.envId === api.activeEnvId)));
  const isJSON = res?.body && /^[[{]/.test(res.body.trim());
  return (
    <div className="flex min-w-0 flex-col gap-4">
      <Panel>
        <div className="flex items-center gap-2 p-3">
          <select
            className="select mono"
            style={{ width: 104 }}
            aria-label="Method"
            value={r.method}
            onChange={(e) => setR({ ...r, method: e.target.value })}
          >
            {["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"].map((m) => (
              <option key={m}>{m}</option>
            ))}
          </select>
          <input
            className="input mono"
            aria-label="URL"
            value={r.url}
            onChange={(e) => setR({ ...r, url: e.target.value })}
          />
          <Button kind="blue" icon={<Send size={13} />} disabled={sending} onClick={send} title={`${mod()}+Enter`}>
            Send
          </Button>
          <Button
            icon={<Save size={13} />}
            disabled={!dirty && !!r.id}
            onClick={() => save.mutate(undefined)}
            title={`${mod()}+S`}
          >
            Save
          </Button>
        </div>
        {unresolved.length > 0 && (
          <div className="banner bad mx-3 mb-2">
            Unresolved: {unresolved.map((u) => `{{${u}}}`).join(", ")}. Add them on the Environments tab.
          </div>
        )}
        <div className="flex flex-wrap items-center gap-2 px-3 pb-2 text-[12.5px]">
          <span className="muted">Access</span>
          {r.id ? (
            <Seg
              label="Access"
              value={r.accessFlag ? ("" as "read") : r.access}
              onChange={(v) => setAccess.mutate(v)}
              options={[
                { id: "read", label: "Read" },
                { id: "write", label: "Write" },
              ]}
            />
          ) : (
            <AccessChip access={r.access} />
          )}
          <span className="muted">{r.accessReason}</span>
          {r.accessFlag && <Chip tone="warn">counts as write until you choose</Chip>}
        </div>
        <div className="px-3">
          <Tabs
            value={sub}
            onChange={setSub}
            tabs={[
              { id: "params", label: "Params", n: r.query.length },
              { id: "headers", label: "Headers", n: r.headers.length },
              { id: "body", label: "Body" },
              { id: "docs", label: "Name and docs" },
            ]}
          />
        </div>
        <div className="px-3 pb-3">
          {sub === "params" && (
            <KVEditor rows={r.query} onChange={(q) => setR({ ...r, query: q })} keyLabel="Query param" />
          )}
          {sub === "headers" && (
            <>
              <KVEditor rows={r.headers} onChange={(h) => setR({ ...r, headers: h })} keyLabel="Header" />
              <div className="muted mt-2 text-[12px]">
                Auth headers come from the API's auth, so keys never sit on a request.
              </div>
            </>
          )}
          {sub === "body" && (
            <div className="flex flex-col gap-2">
              <Seg
                label="Body type"
                value={r.bodyType as any}
                onChange={(v: string) => setR({ ...r, bodyType: v })}
                options={["none", "json", "xml", "form", "text"].map((x) => ({ id: x, label: x }))}
              />
              {r.bodyType !== "none" && (
                <textarea
                  className="textarea"
                  rows={8}
                  aria-label="Body"
                  value={r.body}
                  onChange={(e) => setR({ ...r, body: e.target.value })}
                />
              )}
            </div>
          )}
          {sub === "docs" && (
            <div className="grid gap-3 sm:grid-cols-2">
              <Field
                label="Name in tool code"
                hint={
                  <code>
                    {api.name}.{r.name}()
                  </code>
                }
              >
                <input className="input mono" value={r.name} onChange={(e) => setR({ ...r, name: e.target.value })} />
              </Field>
              <Field label="Folder">
                <input className="input" value={r.folder} onChange={(e) => setR({ ...r, folder: e.target.value })} />
              </Field>
              <Field label="Title">
                <input className="input" value={r.title} onChange={(e) => setR({ ...r, title: e.target.value })} />
              </Field>
              <Field label="Description">
                <input
                  className="input"
                  value={r.description}
                  onChange={(e) => setR({ ...r, description: e.target.value })}
                />
              </Field>
              {r.id && (
                <div>
                  <Button
                    kind="bad"
                    size="sm"
                    icon={<Trash2 size={13} />}
                    onClick={() => confirm(`Delete ${api.name}.${r.name}?`) && remove.mutate(undefined)}
                  >
                    Delete request
                  </Button>
                </div>
              )}
            </div>
          )}
          <ErrorBox error={save.error || remove.error || setAccess.error} />
        </div>
      </Panel>
      <Panel
        title={
          res ? (
            res.error ? (
              <Chip tone="bad">{res.code ?? "error"}</Chip>
            ) : (
              <span className="flex items-center gap-3">
                <Chip tone={res.status! < 400 ? "ok" : "bad"}>{res.statusText}</Chip>
                <span className="muted text-[12.5px] font-normal">
                  {res.durationMs} ms · {bytes(res.size)} · {num(res.tokens)} tokens
                </span>
              </span>
            )
          ) : (
            "Response"
          )
        }
        right={
          res &&
          !res.error && (
            <Seg
              label="View"
              value={view}
              onChange={setView}
              options={[
                { id: "pretty", label: "Pretty" },
                { id: "raw", label: "Raw" },
                { id: "headers", label: "Headers" },
              ]}
            />
          )
        }
      >
        {!res && (
          <Empty title="Not sent yet">
            Press Send or {mod()}+Enter. It goes through mcpDeck's proxy, like an agent's call.
          </Empty>
        )}
        {res?.error && <div className="banner bad m-3">{res.error}</div>}
        {res && !res.error && (
          <pre className="code" style={{ border: 0, borderRadius: 0, maxHeight: 440 }}>
            {view === "headers"
              ? Object.entries(res.headers ?? {})
                  .map(([k, v]) => `${k}: ${v.join(", ")}`)
                  .join("\n")
              : view === "pretty"
                ? pretty(res.body)
                : res.body}
          </pre>
        )}
        <div className="flex flex-wrap gap-2 p-3" style={{ borderTop: "1px solid var(--line)" }}>
          <Button
            size="sm"
            icon={<Code2 size={13} />}
            disabled={!r.id || !tks.data?.length}
            title={tks.data?.length ? "" : "Create a toolkit first"}
            onClick={() =>
              navigate(`/toolkits/${tks.data![0].name}/tools/new?from=${encodeURIComponent(`${api.name}.${r.name}`)}`)
            }
          >
            Create tool
          </Button>
          <Button size="sm" icon={<Copy size={13} />} onClick={() => copy(toolCallOf(api.name, r), "Tool call copied")}>
            Copy as tool call
          </Button>
          <Button
            size="sm"
            icon={<Copy size={13} />}
            onClick={() => copy(curlOf(r), "curl copied; variables stay as {{names}}")}
          >
            Copy as curl
          </Button>
          {isJSON && r.id && (
            <Button size="sm" icon={<Save size={13} />} onClick={() => saveExample.mutate(undefined)}>
              Save as example
            </Button>
          )}
        </div>
      </Panel>
    </div>
  );
}

function VarRow({ apiId, v, envId }: { apiId: string; v?: Variable; envId: string }) {
  const [name, setName] = useState(v?.name ?? "");
  const [source, setSource] = useState<string>(v?.source ?? "literal");
  const [value, setValue] = useState(v?.source === "secret" ? "" : (v?.value ?? ""));
  const [editing, setEditing] = useState(v?.source !== "secret" || !v.isSet);
  const toast = useToast();
  const save = useAction(() => put(`/api/apis/${apiId}/variables`, { id: v?.id ?? "", envId, name, source, value }), {
    onSuccess: () => {
      toast(source === "secret" ? `${name} encrypted and saved` : `${name} saved`);
      if (!v) {
        setName("");
        setValue("");
      }
      if (source === "secret") setEditing(false);
    },
  });
  const remove = useAction(() => del(`/api/variables/${v!.id}`));
  const changed =
    !v || name !== v.name || source !== v.source || (source === "secret" ? value !== "" : value !== v.value);
  return (
    <tr>
      <td>
        <input
          className="input mono"
          aria-label="Variable name"
          placeholder="baseUrl"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </td>
      <td style={{ width: 130 }}>
        <select className="select" aria-label="Source" value={source} onChange={(e) => setSource(e.target.value)}>
          <option value="literal">Value</option>
          <option value="secret">Secret</option>
          <option value="os_env">OS env var</option>
        </select>
      </td>
      <td>
        {source === "secret" && !editing ? (
          <span className="flex items-center gap-2">
            <span className="mono muted">••••••••••</span>
            <Chip tone="ok">set</Chip>
            <Button size="sm" onClick={() => setEditing(true)}>
              Replace
            </Button>
          </span>
        ) : (
          <input
            className="input mono"
            aria-label="Value"
            type={source === "secret" ? "password" : "text"}
            autoComplete="off"
            placeholder={
              source === "os_env"
                ? "ACME_CRM_URL"
                : source === "secret"
                  ? "new value, write-only"
                  : "https://api.acme.example"
            }
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
        )}
      </td>
      <td style={{ width: 120 }} className="num">
        <span className="flex justify-end gap-1">
          {changed && name && (
            <Button size="sm" kind="blue" onClick={() => save.mutate(undefined)}>
              {v ? "Save" : "Add"}
            </Button>
          )}
          {v && (
            <Button
              size="sm"
              kind="ghost"
              aria-label={`Delete ${v.name}`}
              icon={<Trash2 size={13} />}
              onClick={() => remove.mutate(undefined)}
            />
          )}
        </span>
        <ErrorBox error={save.error} />
      </td>
    </tr>
  );
}

function Environments({ a }: { a: APIDetail }) {
  const [env, setEnv] = useState(a.activeEnvId);
  const [newName, setNewName] = useState("");
  const add = useAction(() => post(`/api/apis/${a.id}/environments`, { name: newName }), {
    onSuccess: () => setNewName(""),
  });
  const remove = useAction((id: string) => del(`/api/environments/${id}`));
  const vars = a.variables.filter((v) => v.envId === env);
  return (
    <div className="grid items-start gap-4" style={{ gridTemplateColumns: "230px minmax(0, 1fr)" }}>
      <Panel bodyClass="p-1.5">
        <button
          type="button"
          className="nav-item"
          aria-current={env === "" ? "page" : undefined}
          onClick={() => setEnv("")}
        >
          All environments
        </button>
        {a.environments.map((e) => (
          <button
            key={e.id}
            type="button"
            className="nav-item"
            aria-current={env === e.id ? "page" : undefined}
            onClick={() => setEnv(e.id)}
          >
            {e.name}
            {e.id === a.activeEnvId && <span className="muted ml-auto text-[11.5px]">active</span>}
          </button>
        ))}
        <div className="flex gap-1.5 p-1.5">
          <input
            className="input"
            aria-label="New environment"
            placeholder="Staging"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
          />
          <Button
            size="sm"
            disabled={!newName}
            onClick={() => add.mutate(undefined)}
            aria-label="Add environment"
            icon={<Plus size={13} />}
          />
        </div>
        <ErrorBox error={add.error || remove.error} />
      </Panel>
      <Panel
        title={env ? a.environments.find((e) => e.id === env)?.name : "All environments (API variables)"}
        right={
          env &&
          env !== a.activeEnvId && (
            <Button size="sm" kind="bad" onClick={() => remove.mutate(env)}>
              Delete environment
            </Button>
          )
        }
      >
        <table className="table">
          <thead>
            <tr>
              <th>Variable</th>
              <th>Source</th>
              <th>Value</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {vars.map((v) => (
              <VarRow key={v.id} apiId={a.id} v={v} envId={env} />
            ))}
            <VarRow key={`new-${env}-${vars.length}`} apiId={a.id} envId={env} />
          </tbody>
        </table>
        <div className="muted p-3 text-[12px]" style={{ borderTop: "1px solid var(--line)" }}>
          Use them as <code>{"{{name}}"}</code> in URLs, params, headers, bodies and auth. Environment values win over
          API values. Secrets are encrypted with a key kept in your OS keychain and are never shown again.
        </div>
      </Panel>
    </div>
  );
}

function AuthTab({ a }: { a: APIDetail }) {
  const [auth, setAuth] = useState<Auth>(a.auth ?? { type: "none" });
  const toast = useToast();
  const save = useAction(() => patch(`/api/apis/${a.id}`, { auth }), { onSuccess: () => toast("Auth saved") });
  const secrets = a.variables.filter((v) => v.source === "secret");
  const set = (p: Partial<Auth>) => setAuth({ ...auth, ...p });
  return (
    <div className="grid items-start gap-4 lg:grid-cols-2">
      <Panel title="Auth" bodyClass="panel-body flex flex-col gap-3">
        <Field label="Type" hint="Requests inherit this unless they set their own.">
          <select className="select" value={auth.type} onChange={(e) => set({ type: e.target.value })}>
            <option value="none">No auth</option>
            <option value="bearer">Bearer token</option>
            <option value="apikey">API key</option>
            <option value="basic">Basic</option>
            <option value="login">Login request (cookie or header)</option>
          </select>
        </Field>
        {auth.type === "bearer" && (
          <Field label="Token" hint="Point it at a secret variable">
            <input
              className="input mono"
              value={auth.token ?? ""}
              placeholder="{{token}}"
              onChange={(e) => set({ token: e.target.value })}
            />
          </Field>
        )}
        {auth.type === "apikey" && (
          <div className="grid grid-cols-3 gap-2">
            <Field label="Name">
              <input
                className="input mono"
                value={auth.key ?? ""}
                placeholder="X-Api-Key"
                onChange={(e) => set({ key: e.target.value })}
              />
            </Field>
            <Field label="Value">
              <input
                className="input mono"
                value={auth.value ?? ""}
                placeholder="{{apiKey}}"
                onChange={(e) => set({ value: e.target.value })}
              />
            </Field>
            <Field label="In">
              <select className="select" value={auth.in ?? "header"} onChange={(e) => set({ in: e.target.value })}>
                <option value="header">Header</option>
                <option value="query">Query</option>
              </select>
            </Field>
          </div>
        )}
        {auth.type === "basic" && (
          <div className="grid grid-cols-2 gap-2">
            <Field label="Username">
              <input
                className="input mono"
                value={auth.username ?? ""}
                onChange={(e) => set({ username: e.target.value })}
              />
            </Field>
            <Field label="Password">
              <input
                className="input mono"
                value={auth.password ?? ""}
                placeholder="{{password}}"
                onChange={(e) => set({ password: e.target.value })}
              />
            </Field>
          </div>
        )}
        {auth.type === "login" && (
          <div className="grid grid-cols-2 gap-2">
            <Field label="Login request" hint="Run when needed; tools never handle the login">
              <select
                className="select"
                value={auth.loginRequestId ?? ""}
                onChange={(e) => set({ loginRequestId: e.target.value })}
              >
                <option value="">Choose…</option>
                {a.requests.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.method} {r.name}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Capture" hint="cookie, or a response header name">
              <input
                className="input mono"
                value={auth.capture ?? "cookie"}
                onChange={(e) => set({ capture: e.target.value })}
              />
            </Field>
          </div>
        )}
        <div>
          <Button kind="ink" onClick={() => save.mutate(undefined)}>
            Save auth
          </Button>
        </div>
        <ErrorBox error={save.error} />
      </Panel>
      <Panel title="Credentials" icon={<KeyRound size={15} className="muted" />}>
        {secrets.length === 0 && (
          <Empty title="No secrets">Choose an auth type that uses a variable like {"{{token}}"}.</Empty>
        )}
        <table className="table">
          <tbody>
            {secrets.map((v) => (
              <VarRow key={v.id} apiId={a.id} v={v} envId={v.envId} />
            ))}
          </tbody>
        </table>
        <div className="muted p-3 text-[12px]" style={{ borderTop: "1px solid var(--line)" }}>
          Write-only: there's no Reveal. Replacing takes effect on the next call. Every agent uses this credential
          through mcpDeck and never sees it, so the API sees one identity while access and call logs stay per agent.
        </div>
      </Panel>
    </div>
  );
}

function SettingsTab({ a }: { a: APIDetail }) {
  const [title, setTitle] = useState(a.title);
  const [desc, setDesc] = useState(a.description);
  const [hosts, setHosts] = useState((a.allowedHosts ?? []).join("\n"));
  const toast = useToast();
  const save = useAction(
    () => patch(`/api/apis/${a.id}`, { title, description: desc, allowedHosts: hosts.split(/[\s,]+/).filter(Boolean) }),
    { onSuccess: () => toast("Saved") },
  );
  const cookies = useAction(() => post(`/api/apis/${a.id}/cookies/clear`), {
    onSuccess: () => toast("Cookie jar cleared"),
  });
  const remove = useAction(() => del(`/api/apis/${a.id}`), { onSuccess: () => navigate("/apis") });
  return (
    <div className="grid items-start gap-4 lg:grid-cols-2">
      <Panel title="General" bodyClass="panel-body flex flex-col gap-3">
        <Field label="Title">
          <input className="input" value={title} onChange={(e) => setTitle(e.target.value)} />
        </Field>
        <Field label="Description">
          <textarea
            className="textarea"
            style={{ fontFamily: "var(--font)" }}
            rows={3}
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
          />
        </Field>
        <Field
          label="Allowed hosts"
          hint={
            <>
              Calls to any other host are refused. Empty means the hosts of your environments:{" "}
              {(a.hosts ?? []).join(", ") || "none yet"}.
            </>
          }
        >
          <textarea
            className="textarea"
            rows={3}
            placeholder="api.acme.example"
            value={hosts}
            onChange={(e) => setHosts(e.target.value)}
          />
        </Field>
        <div>
          <Button kind="ink" onClick={() => save.mutate(undefined)}>
            Save
          </Button>
        </div>
        <ErrorBox error={save.error} />
      </Panel>
      <Panel title="Data" bodyClass="panel-body flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <Button onClick={() => cookies.mutate(undefined)}>Clear cookie jar</Button>
          <span className="muted text-[12.5px]">Forces the next call to log in again.</span>
        </div>
        <div className="flex items-center gap-3">
          <Button
            kind="bad"
            onClick={() => confirm(`Delete ${a.name} and all its requests?`) && remove.mutate(undefined)}
          >
            Delete API
          </Button>
          <span className="muted text-[12.5px]">
            {(a.tools ?? []).length > 0
              ? `Used by ${(a.tools ?? []).join(", ")}; those tools must change first.`
              : "No tool uses it."}
          </span>
        </div>
        <ErrorBox error={remove.error} />
      </Panel>
    </div>
  );
}
