import {
  ArrowRight,
  BookmarkPlus,
  CheckCircle2,
  Code2,
  Cookie,
  Copy,
  Download,
  FileUp,
  KeyRound,
  Plus,
  Save,
  Send,
  Trash2,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AskAgentButton } from "../components/AskAgent";
import { KVEditor } from "../components/KVEditor";
import {
  AccessChip,
  Avatar,
  Button,
  Chip,
  Empty,
  EmptyState,
  ErrorBox,
  Field,
  Loading,
  Menu,
  MethodChip,
  PageHead,
  Panel,
  SaveBar,
  Seg,
  Tabs,
  useConfirm,
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
        icon={<Avatar name={a.name} size="lg" />}
        crumbs={
          <>
            <Link to="/apps">Applications</Link> / {a.title || a.name}
          </>
        }
        title={a.title || a.name}
        sub={
          <>
            Tools call it as{" "}
            <code>
              {a.name}.{first}()
            </code>{" "}
            · {a.requests.length} requests · from {a.source}
          </>
        }
        actions={
          <>
            <label className="flex items-center gap-2 text-[12.5px]">
              <span className="muted">Environment</span>
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
            </label>
            <AskAgentButton
              prompts={[
                {
                  title: `Explore ${a.name}`,
                  needsBuild: true,
                  text: `With the mcpDeck builder, run search_apis "${a.name}" and describe_api ${a.name}.${first}.\nTell me which three requests would make the most useful tool and what it should return.\nCheck: you only used run_api for reads.`,
                },
              ]}
            />
            <Button kind="ink" icon={<Download size={14} />} onClick={() => navigate(`/apps/${id}/import`)}>
              Import requests
            </Button>
            <AppMenu a={a} onTab={setTab} />
          </>
        }
      />
      {imported && (
        <div className="banner ok mb-4 items-center">
          <CheckCircle2 size={15} className="flex-none" />
          <span>
            {imported} requests added.{" "}
            {missing.length > 0
              ? "Add the credential next, then send a request to check it works."
              : "Next: pick a request and press Send to see what it returns."}
          </span>
          {missing.length > 0 && (
            <Button size="sm" className="ml-auto" icon={<KeyRound size={13} />} onClick={() => setTab("auth")}>
              Add credential
            </Button>
          )}
        </div>
      )}
      {!imported && missing.length > 0 && (
        <div className="banner warn mb-4 items-center">
          <KeyRound size={15} className="flex-none" />
          <span>
            {missing.map((m) => m.name).join(", ")} {missing.length === 1 ? "has" : "have"} no value yet, so requests
            that need it will fail.
          </span>
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
          { id: "auth", label: "Auth and credentials", n: missing.length || undefined },
          { id: "settings", label: "Settings" },
        ]}
      />
      {tab === "requests" && <Requests a={a} onTab={setTab} />}
      {tab === "environments" && <Environments a={a} />}
      {tab === "auth" && <AuthTab a={a} />}
      {tab === "settings" && <SettingsTab a={a} />}
    </div>
  );
}

function AppMenu({ a, onTab }: { a: APIDetail; onTab: (t: Tab) => void }) {
  const toast = useToast();
  const confirm = useConfirm();
  const cookies = useAction(() => post(`/api/apis/${a.id}/cookies/clear`), {
    onSuccess: () => toast("Cookie jar cleared; the next call logs in again"),
  });
  const remove = useAction(() => del(`/api/apis/${a.id}`), {
    onSuccess: () => {
      toast(`${a.title || a.name} deleted`);
      navigate("/apps");
    },
    onError: (e) => toast((e as Error).message, true),
  });
  const used = a.tools ?? [];
  return (
    <Menu
      label="More application actions"
      items={[
        { label: "Import requests", icon: <FileUp size={14} />, onClick: () => navigate(`/apps/${a.id}/import`) },
        { label: "Edit name and hosts", icon: <Code2 size={14} />, onClick: () => onTab("settings") },
        { label: "Clear cookie jar", icon: <Cookie size={14} />, onClick: () => cookies.mutate(undefined) },
        "sep",
        {
          label: "Delete application",
          icon: <Trash2 size={14} />,
          danger: true,
          onClick: async () => {
            if (used.length > 0) {
              toast(`Used by ${used.join(", ")}. Change those tools first.`, true);
              return;
            }
            if (
              await confirm({
                title: `Delete ${a.title || a.name}?`,
                body: `Its ${a.requests.length} requests, environments and credentials are deleted. This can't be undone.`,
                action: "Delete application",
                danger: true,
              })
            )
              remove.mutate(undefined);
          },
        },
      ]}
    />
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

export function nameFromURL(method: string, url: string): string {
  const parts = url
    .replace(/\{\{baseUrl\}\}/, "")
    .split("?")[0]
    .split("/")
    .filter((p) => p && !p.startsWith("{") && !p.startsWith(":"))
    .map((p) => p.replace(/\.[a-z]+$/i, "").replace(/[^A-Za-z0-9]+/g, " "));
  const last = parts.slice(-2).join(" ").trim() || "request";
  const verb = { GET: "get", POST: "create", PUT: "update", PATCH: "update", DELETE: "delete" }[method] ?? "call";
  const words = `${verb} ${last}`.split(" ").filter(Boolean);
  return words
    .map((w, i) => (i ? w[0].toUpperCase() + w.slice(1) : w.toLowerCase()))
    .join("")
    .slice(0, 48);
}

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

function Requests({ a, onTab }: { a: APIDetail; onTab: (t: Tab) => void }) {
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
            className="ml-auto"
            title="Add a request manually"
            icon={<Plus size={13} />}
            onClick={() => setSel("new")}
          >
            New
          </Button>
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
            {a.requests.length === 0 && <Empty title="No requests yet">Import them, or add one manually.</Empty>}
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
        <Editor key={draft.id || "new"} api={a} initial={draft} onSaved={(id) => setSel(id)} onTab={onTab} />
      ) : (
        <Panel>
          <EmptyState
            icon={<Send size={20} />}
            title="Add the first request"
            actions={
              <>
                <Button icon={<Plus size={14} />} onClick={() => setSel("new")}>
                  Add manually
                </Button>
                <Button kind="ink" icon={<FileUp size={14} />} onClick={() => navigate(`/apps/${a.id}/import`)}>
                  Import requests
                </Button>
              </>
            }
          >
            Import a Postman collection, an OpenAPI file or curl, or add a request manually: a method, a URL and any
            parameters.
          </EmptyState>
        </Panel>
      )}
    </div>
  );
}

function Editor({
  api,
  initial,
  onSaved,
  onTab,
}: {
  api: APIDetail;
  initial: Request;
  onSaved: (id: string) => void;
  onTab?: (t: Tab) => void;
}) {
  const [r, setR] = useState<Request>(initial);
  const [sub, setSub] = useState<"params" | "headers" | "body" | "docs">(initial.body ? "body" : "params");
  const [res, setRes] = useState<SendResult | null>(null);
  const [view, setView] = useState<"pretty" | "raw" | "headers">("pretty");
  const [sending, setSending] = useState(false);
  const copy = useCopy();
  const toast = useToast();
  const confirm = useConfirm();
  const tks = useToolkits();
  const strip = ({ example: _e, ...x }: Request) => JSON.stringify(x);
  const dirty = strip(r) !== strip(initial);
  const save = useAction(
    () => {
      // A request added manually gets a name from its method and path, like getAccount.
      const body = r.id || r.name !== "newRequest" ? r : { ...r, name: nameFromURL(r.method, r.url) };
      return r.id ? put<Request>(`/api/requests/${r.id}`, body) : post<Request>(`/api/apis/${api.id}/requests`, body);
    },
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
  const [exampleSaved, setExampleSaved] = useState(false);
  const saveExample = useAction(() => post(`/api/requests/${r.id}/example`, { body: JSON.parse(res!.body) }), {
    onSuccess: () => {
      setExampleSaved(true);
      toast("Saved as example; values that look like secrets were masked");
    },
  });
  const send = async () => {
    setSending(true);
    try {
      setExampleSaved(false);
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
          <Button
            kind="blue"
            icon={<Send size={13} />}
            loading={sending}
            onClick={send}
            title={`${mod()}+Enter`}
            data-tour="send"
          >
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
                Auth headers come from the application's Authentication setting, so keys never sit on a request.
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
                <div className="flex justify-end sm:col-span-2">
                  <Button
                    kind="bad"
                    size="sm"
                    icon={<Trash2 size={13} />}
                    onClick={async () =>
                      (await confirm({
                        title: `Delete ${api.name}.${r.name}?`,
                        body: "Tools that call it will fail until they change.",
                        action: "Delete request",
                        danger: true,
                      })) && remove.mutate(undefined)
                    }
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
              <Chip tone="bad">Couldn't send</Chip>
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
          <EmptyState icon={<Send size={20} />} title="Send the request to see its response">
            Press Send or {mod()}+Enter. It goes through mcpDeck, the same way an agent's call will, and shows the size
            in tokens an agent would read.
          </EmptyState>
        )}
        {res?.error && (
          <div className="banner bad m-3 items-center">
            <span className="min-w-0 flex-1">{res.error}</span>
            {res.code === "unresolved_variables" && onTab && (
              <Button
                size="sm"
                icon={<KeyRound size={13} />}
                onClick={() => onTab(/token|key|secret|password/i.test(res.error ?? "") ? "auth" : "environments")}
              >
                {/token|key|secret|password/i.test(res.error ?? "") ? "Add credential" : "Open environments"}
              </Button>
            )}
          </div>
        )}
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
        {res && !res.error && (res.status ?? 0) < 400 && r.id && (
          <div className="next-card m-3" style={{ borderLeftColor: exampleSaved ? "var(--ok-dot)" : undefined }}>
            {exampleSaved ? (
              <>
                <CheckCircle2 size={18} style={{ color: "var(--ok)" }} className="flex-none" />
                <div className="min-w-0 flex-1">
                  <div className="font-medium">Example saved. Turn this request into a tool?</div>
                  <div className="muted text-[12.5px]">
                    A tool calls it for an agent and returns only the fields that are needed.
                  </div>
                </div>
                <Button
                  size="sm"
                  kind="ink"
                  icon={<ArrowRight size={13} />}
                  onClick={() =>
                    tks.data?.length
                      ? navigate(
                          `/toolkits/${tks.data[0].name}/tools/new?from=${encodeURIComponent(`${api.name}.${r.name}`)}`,
                        )
                      : navigate("/toolkits?new=1")
                  }
                >
                  {tks.data?.length ? "Create tool" : "Create a toolkit first"}
                </Button>
              </>
            ) : (
              isJSON && (
                <>
                  <BookmarkPlus size={18} className="muted flex-none" />
                  <div className="min-w-0 flex-1">
                    <div className="font-medium">Looks right? Save it as an example.</div>
                    <div className="muted text-[12.5px]">
                      Tools can then be tested without the network, and agents see what this request returns.
                    </div>
                  </div>
                  <Button
                    size="sm"
                    kind="ink"
                    loading={saveExample.isPending}
                    onClick={() => saveExample.mutate(undefined)}
                  >
                    Save as example
                  </Button>
                </>
              )
            )}
          </div>
        )}
        <div className="flex flex-wrap justify-end gap-2 p-3" style={{ borderTop: "1px solid var(--line)" }}>
          <Button
            size="sm"
            kind="ghost"
            icon={<Copy size={13} />}
            onClick={() => copy(curlOf(r), "curl copied; variables stay as {{names}}")}
          >
            Copy as curl
          </Button>
          <Button
            size="sm"
            kind="ghost"
            icon={<Copy size={13} />}
            onClick={() => copy(toolCallOf(api.name, r), "Tool call copied")}
          >
            Copy as tool call
          </Button>
          <Button
            size="sm"
            icon={<Code2 size={13} />}
            disabled={!r.id}
            title={!r.id ? "Save the request first" : tks.data?.length ? "" : "Create a toolkit first"}
            onClick={() =>
              tks.data?.length
                ? navigate(
                    `/toolkits/${tks.data[0].name}/tools/new?from=${encodeURIComponent(`${api.name}.${r.name}`)}`,
                  )
                : navigate("/toolkits?new=1")
            }
          >
            Create tool
          </Button>
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
  const save = useAction(() => patch(`/api/apis/${a.id}`, { auth }), {
    onSuccess: () => toast("Authentication saved"),
  });
  const secrets = a.variables.filter((v) => v.source === "secret");
  const set = (p: Partial<Auth>) => setAuth({ ...auth, ...p });
  const dirty = JSON.stringify(auth) !== JSON.stringify(a.auth ?? { type: "none" });
  return (
    <div className="grid items-start gap-5 lg:grid-cols-2">
      <Panel
        title="Authentication"
        bodyClass="panel-body flex flex-col gap-4"
        footer={
          <>
            {dirty && (
              <Button kind="ghost" onClick={() => setAuth(a.auth ?? { type: "none" })}>
                Discard
              </Button>
            )}
            <Button kind="ink" disabled={!dirty} loading={save.isPending} onClick={() => save.mutate(undefined)}>
              Save authentication
            </Button>
          </>
        }
      >
        <Field label="Type" hint="Every request uses this unless it sets its own.">
          <select className="select" value={auth.type} onChange={(e) => set({ type: e.target.value })}>
            <option value="none">None</option>
            <option value="bearer">Bearer token</option>
            <option value="apikey">API key</option>
            <option value="basic">Username and password</option>
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
  const init = { title: a.title, desc: a.description, hosts: (a.allowedHosts ?? []).join("\n") };
  const [v, setV] = useState(init);
  const toast = useToast();
  const dirty = JSON.stringify(v) !== JSON.stringify(init);
  const save = useAction(
    () =>
      patch(`/api/apis/${a.id}`, {
        title: v.title,
        description: v.desc,
        allowedHosts: v.hosts.split(/[\s,]+/).filter(Boolean),
      }),
    { onSuccess: () => toast("Application saved") },
  );
  return (
    <div style={{ maxWidth: 880 }}>
      <Panel>
        <div className="settings-row">
          <div className="what">
            <strong>Display name</strong>
            <span>What people see in mcpDeck.</span>
          </div>
          <input className="input" value={v.title} onChange={(e) => setV({ ...v, title: e.target.value })} />
        </div>
        <div className="settings-row">
          <div className="what">
            <strong>Name in tool code</strong>
            <span>Fixed once created, because tools call it by this name.</span>
          </div>
          <input className="input mono" value={a.name} readOnly aria-readonly />
        </div>
        <div className="settings-row">
          <div className="what">
            <strong>Description</strong>
            <span>Agents that build tools read it to pick the right application.</span>
          </div>
          <textarea
            className="textarea prose"
            rows={3}
            placeholder="Customer accounts, contacts and owners"
            value={v.desc}
            onChange={(e) => setV({ ...v, desc: e.target.value })}
          />
        </div>
        <div className="settings-row">
          <div className="what">
            <strong>Allowed hosts</strong>
            <span>
              Calls to any other host are refused. Leave empty to allow the hosts in your environments:{" "}
              {(a.hosts ?? []).join(", ") || "none yet"}.
            </span>
          </div>
          <textarea
            className="textarea"
            rows={3}
            placeholder="api.acme.example"
            value={v.hosts}
            onChange={(e) => setV({ ...v, hosts: e.target.value })}
          />
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
