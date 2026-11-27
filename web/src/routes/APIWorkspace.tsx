import { useQuery } from "@tanstack/react-query";
import {
  ArrowRight,
  BookmarkPlus,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  CircleDot,
  Code2,
  Cookie,
  Copy,
  Download,
  FileUp,
  Gauge,
  KeyRound,
  Layers,
  Lock,
  Pencil,
  Plus,
  Save,
  Send,
  Settings2,
  ShieldAlert,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { AskAgentButton } from "../components/AskAgent";
import { byLogin, CaptureRows, loginScore } from "../components/CapturePicker";
import { counts, FindingList } from "../components/Findings";
import { KVEditor } from "../components/KVEditor";
import { Elapsed, seconds } from "../components/RequestTray";
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
  Menu,
  MethodChip,
  PageHead,
  Panel,
  SaveBar,
  Seg,
  Select,
  Tabs,
  Toggle,
  useConfirm,
  useCopy,
  useToast,
} from "../components/ui";
import { BrowserFields, FilesPanel, RecordDialog } from "../components/WebApp";
import { del, patch, post, put } from "../lib/api";
import { getDraft, setDraft, useDrafts } from "../lib/drafts";
import { ago, bytes, mod, num, pretty } from "../lib/format";
import { useAction, useAPI, useHistory, useToolkits } from "../lib/hooks";
import { cancel, flightKey, markSeen, send as sendRequest, useFlight } from "../lib/inflight";
import { Link, navigate, useLocation } from "../lib/router";
import type { APIDetail, Auth, EnvImport, LoginStep, OutputReport, Request, SendResult, Variable } from "../lib/types";

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
            <Select
              label="Environment"
              className="env-select"
              value={a.activeEnvId}
              onChange={(v) => setActive.mutate(v)}
              options={a.environments.map((e) => ({
                value: e.id,
                label: e.name,
                icon: <Layers size={14} />,
                hint: `${a.variables.filter((v) => v.envId === e.id).length} variables`,
              }))}
              footer={
                <button type="button" className="select-item w-full" onClick={() => setTab("environments")}>
                  <Settings2 size={14} /> Manage environments
                </button>
              }
            />
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
  const [recording, setRecording] = useState(false);
  return (
    <>
      {recording && <RecordDialog api={a.name} startUrl={a.auth?.startUrl} onClose={() => setRecording(false)} />}
      <Menu
        label="More application actions"
        items={[
          { label: "Import requests", icon: <FileUp size={14} />, onClick: () => navigate(`/apps/${a.id}/import`) },
          { label: "Record a task in the browser", icon: <CircleDot size={14} />, onClick: () => setRecording(true) },
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
    </>
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

export function Requests({ a, onTab }: { a: APIDetail; onTab: (t: Tab) => void }) {
  const { search } = useLocation();
  const asked = search.get("req");
  const [sel, setSel] = useState<string>(asked ?? a.requests[0]?.id ?? "");
  // The request tray links here with ?req= to reopen a request.
  useEffect(() => {
    if (asked) setSel(asked);
  }, [asked]);
  const [side, setSide] = useState<"requests" | "history">("requests");
  const [filter, setFilter] = useState("");
  const [draft, setDraft] = useState<Request | null>(null);
  const [resp, setResp] = useState("");
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const toggle = (id: string, open: boolean) => {
    const next = new Set(expanded);
    if (open) next.delete(id);
    else next.add(id);
    setExpanded(next);
    if (open && sel === id) setSel("");
  };
  const removeResp = useAction((x: { req: string; id: string }) => del(`/api/requests/${x.req}/responses/${x.id}`));
  const drafts = useDrafts();
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
    <div className="grid items-start gap-4" style={{ gridTemplateColumns: "280px minmax(0, 1fr)" }}>
      <Panel bodyClass="p-2" className="sticky top-0">
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
                {rs.map((r) => {
                  const n = r.responses?.length ?? 0;
                  const open = n > 0 && (expanded.has(r.id) || sel === r.id);
                  return (
                    <div key={r.id}>
                      <div className="tree-line">
                        {n > 0 ? (
                          <button
                            type="button"
                            className="tree-twisty"
                            aria-expanded={open}
                            aria-label={`${open ? "Hide" : "Show"} saved responses of ${r.name}`}
                            onClick={() => toggle(r.id, open)}
                          >
                            <ChevronRight size={12} />
                          </button>
                        ) : (
                          <span className="tree-twisty" />
                        )}
                        <button
                          type="button"
                          className="tree-row"
                          aria-current={sel === r.id || (!sel && r === a.requests[0]) ? "true" : undefined}
                          onClick={() => {
                            setSel(r.id);
                            setResp("");
                          }}
                        >
                          <MethodChip method={r.method} />
                          <span className="min-w-0 flex-1 truncate">{r.name}</span>
                          {n > 0 && (
                            <span
                              className="faint text-[11px]"
                              title={`${n} saved ${n === 1 ? "response" : "responses"}`}
                            >
                              {n}
                            </span>
                          )}
                          {r.accessFlag && <span className="dot warn" title="Check access" />}
                          {drafts.has(flightKey(a.id, r.id)) && (
                            <span className="dot on" title="Unsaved changes" role="img" aria-label="Unsaved changes" />
                          )}
                        </button>
                      </div>
                      {open &&
                        r.responses!.map((x) => (
                          <div key={x.id} className="tree-line tree-child">
                            <button
                              type="button"
                              className="tree-row"
                              aria-current={sel === r.id && resp === x.id ? "true" : undefined}
                              title={`Saved response: ${x.name}`}
                              onClick={() => {
                                setSel(r.id);
                                setResp(x.id);
                              }}
                            >
                              <span className={`dot ${x.status < 300 ? "on" : x.status < 500 ? "warn" : "bad"}`} />
                              <span className="mono text-[11.5px]">{x.status}</span>
                              <span className="min-w-0 flex-1 truncate">{x.name}</span>
                            </button>
                            <button
                              type="button"
                              className="icon-btn tree-del"
                              aria-label={`Delete saved response ${x.name}`}
                              onClick={() => removeResp.mutate({ req: r.id, id: x.id })}
                            >
                              <Trash2 size={12} />
                            </button>
                          </div>
                        ))}
                    </div>
                  );
                })}
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
                className="tree-row flex-col items-start gap-0"
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
        <Editor
          key={draft.id || "new"}
          api={a}
          initial={draft}
          onSaved={(id) => setSel(id)}
          onTab={onTab}
          openResponse={resp}
          onOpenResponse={setResp}
        />
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
  openResponse = "",
  onOpenResponse,
}: {
  api: APIDetail;
  initial: Request;
  onSaved: (id: string) => void;
  onTab?: (t: Tab) => void;
  /** A saved response picked in the sidebar. */
  openResponse?: string;
  onOpenResponse?: (id: string) => void;
}) {
  const key = flightKey(api.id, initial.id);
  // Unsaved edits survive opening another request or page.
  const [r, setR] = useState<Request>(() => getDraft(key) ?? initial);
  const [sub, setSub] = useState<"params" | "headers" | "body" | "docs" | "settings">(initial.body ? "body" : "params");
  const flight = useFlight(key);
  // The last response stays with the request while you look elsewhere (NET-3).
  const [res, setRes] = useState<SendResult | null>(flight?.result ?? null);
  const [view, setView] = useState<"pretty" | "raw" | "headers">("pretty");
  const sending = !!flight && !flight.finishedAt;
  useEffect(() => {
    if (flight?.finishedAt && flight.result) {
      setRes(flight.result);
      markSeen(key);
    }
  }, [flight?.finishedAt, flight?.result, key]);
  const copy = useCopy();
  const toast = useToast();
  const confirm = useConfirm();
  const tks = useToolkits();
  const strip = ({ example: _e, responses: _r, ...x }: Request) => JSON.stringify(x);
  const dirty = strip(r) !== strip(initial);
  useEffect(() => setDraft(key, dirty ? r : undefined), [key, dirty, r]);
  const save = useAction(
    () => {
      // A request added manually gets a name from its method and path, like getAccount.
      const body = r.id || r.name !== "newRequest" ? r : { ...r, name: nameFromURL(r.method, r.url) };
      return r.id ? put<Request>(`/api/requests/${r.id}`, body) : post<Request>(`/api/apis/${api.id}/requests`, body);
    },
    {
      onSuccess: (x) => {
        setDraft(key, undefined);
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
  const send = () => {
    if (sending) return;
    sendRequest(
      { apiId: api.id, reqId: initial.id, label: `${api.name}.${r.name}`, method: r.method },
      { apiId: api.id, draft: r },
    );
  };
  useEffect(() => {
    const k = (e: KeyboardEvent) => {
      if (e.key === "Escape" && sending) cancel(key);
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
          <Select
            label="Method"
            className="mono"
            value={r.method}
            onChange={(m) => setR({ ...r, method: m })}
            options={["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"].map((m) => ({
              value: m,
              label: <MethodChip method={m} />,
            }))}
          />
          <input
            className="input mono"
            aria-label="URL"
            value={r.url}
            onChange={(e) => setR({ ...r, url: e.target.value })}
          />
          {sending ? (
            <Button kind="bad" icon={<X size={13} />} onClick={() => cancel(key)} title="Cancel (Esc)">
              Cancel · <Elapsed since={flight!.startedAt} />
            </Button>
          ) : (
            <Button kind="blue" icon={<Send size={13} />} onClick={send} title={`${mod()}+Enter`} data-tour="send">
              Send
            </Button>
          )}
          <Button
            icon={<Save size={13} />}
            disabled={!dirty && !!r.id}
            onClick={() => save.mutate(undefined)}
            title={`${mod()}+S`}
          >
            Save
          </Button>
          {dirty && r.id && (
            <Button kind="ghost" onClick={() => setR(initial)} title="Drop unsaved changes">
              Revert
            </Button>
          )}
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
              { id: "settings", label: "Settings" },
            ]}
          />
        </div>
        <div className="px-3 pb-3">
          {sub === "params" && (
            <>
              <KVEditor rows={r.query} onChange={(q) => setR({ ...r, query: q })} keyLabel="Query param" />
              <PathVariables r={r} onChange={(pathParams) => setR({ ...r, pathParams })} />
            </>
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
          {sub === "settings" && (
            <div className="grid gap-3 sm:grid-cols-2">
              <Field
                label="Timeout"
                hint="Seconds to wait for a response. Empty uses the default in Settings → Requests and proxy."
              >
                <input
                  className="input"
                  type="number"
                  min={1}
                  step="any"
                  aria-label="Timeout in seconds"
                  placeholder="Default"
                  value={r.timeoutSeconds ?? ""}
                  onChange={(e) =>
                    setR({ ...r, timeoutSeconds: e.target.value === "" ? undefined : Number(e.target.value) })
                  }
                />
              </Field>
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
                  {seconds(res.durationMs ?? 0)} · {bytes(res.size)} · {num(res.tokens)} tokens
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
        {res && !res.error && isJSON && r.id && (
          <SaveResponseCard
            key={res.callId}
            api={api}
            req={r}
            res={res}
            onSaved={(x) => setR((cur) => ({ ...cur, responses: x.responses, example: x.example }))}
            hasToolkit={(tks.data?.length ?? 0) > 0}
            toolkit={tks.data?.[0]?.name}
          />
        )}
        {res && !res.error && isJSON && <OutputCheck body={res.body} />}
        {(r.responses?.length ?? 0) > 0 && (
          <SavedResponses
            req={r}
            open={openResponse}
            setOpen={(id) => onOpenResponse?.(id)}
            onChange={(x) => setR((cur) => ({ ...cur, responses: x.responses }))}
          />
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

function statusTone(s: number): "ok" | "warn" | "bad" {
  return s < 300 ? "ok" : s < 500 ? "warn" : "bad";
}

const statusNames: Record<number, string> = {
  200: "Success",
  201: "Created",
  204: "No content",
  400: "Bad input",
  401: "Not allowed",
  403: "Not allowed",
  404: "Not found",
  409: "Conflict",
  422: "Bad input",
  429: "Rate limited",
  500: "Server error",
};

/** SaveResponseCard offers to keep each response under its status (EX-3). */
function SaveResponseCard({
  api,
  req,
  res,
  onSaved,
  hasToolkit,
  toolkit,
}: {
  api: APIDetail;
  req: Request;
  res: SendResult;
  onSaved: (r: Request) => void;
  hasToolkit: boolean;
  toolkit?: string;
}) {
  const status = res.status ?? 0;
  const existing = req.responses?.find((x) => x.status === status);
  const [name, setName] = useState(
    existing?.name ?? statusNames[status] ?? (status < 300 ? "Success" : `Status ${status}`),
  );
  const [saved, setSaved] = useState(false);
  const toast = useToast();
  const save = useAction(
    () => post<Request>(`/api/requests/${req.id}/responses`, { name, status, body: JSON.parse(res.body) }),
    {
      onSuccess: (x) => {
        setSaved(true);
        onSaved(x);
        toast(`Saved as ${status} ${name}; values that look like secrets were masked`);
      },
    },
  );
  if (saved)
    return (
      <div className="next-card m-3" style={{ borderLeftColor: "var(--ok-dot)" }}>
        <CheckCircle2 size={18} style={{ color: "var(--ok)" }} className="flex-none" />
        <div className="min-w-0 flex-1">
          <div className="font-medium">
            Saved as {status} {name}.
          </div>
          <div className="muted text-[12.5px]">
            {status < 300
              ? "Its fields are now known, so tools can chain them into the next request."
              : "Tools and tests can now be checked against this failure without calling the API."}
          </div>
        </div>
        {status < 300 && (
          <Button
            size="sm"
            kind="ink"
            icon={<ArrowRight size={13} />}
            onClick={() =>
              hasToolkit
                ? navigate(`/toolkits/${toolkit}/tools/new?from=${encodeURIComponent(`${api.name}.${req.name}`)}`)
                : navigate("/toolkits?new=1")
            }
          >
            {hasToolkit ? "Build a tool" : "Create a toolkit first"}
          </Button>
        )}
      </div>
    );
  return (
    <div className="next-card m-3 flex-wrap">
      <BookmarkPlus size={18} className="muted flex-none" />
      <div className="min-w-0 flex-1">
        <div className="font-medium">
          {existing ? `Replace the saved ${status} response?` : `Save this ${status} response?`}
        </div>
        <div className="muted text-[12.5px]">
          Keep one response per outcome (success, not found, not allowed). Tools are typed and tested from them.
        </div>
      </div>
      <Chip tone={statusTone(status)}>{status}</Chip>
      <input
        className="input"
        style={{ width: 150, height: 30 }}
        aria-label="Response name"
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <Button size="sm" kind="ink" loading={save.isPending} onClick={() => save.mutate(undefined)}>
        {existing ? "Replace" : "Save response"}
      </Button>
      <ErrorBox error={save.error} />
    </div>
  );
}

/** pathNames are the {id} and :id parts of a URL, in order. */
export function pathNames(url: string): string[] {
  const out: string[] = [];
  for (const m of url.replace(/\{\{[^}]*\}\}/g, "").matchAll(/\{([A-Za-z0-9_]+)\}|\/:([A-Za-z_][A-Za-z0-9_]*)/g)) {
    const n = m[1] ?? m[2];
    if (!out.includes(n)) out.push(n);
  }
  return out;
}

/** PathVariables sets values for the URL's path parts when you send it,
 *  like Postman's Path Variables. Tools and agents pass their own. */
function PathVariables({ r, onChange }: { r: Request; onChange: (kv: Request["query"]) => void }) {
  const names = pathNames(r.url);
  if (names.length === 0) return null;
  const value = (n: string) => r.pathParams?.find((x) => x.key === n)?.value ?? "";
  const set = (n: string, v: string) =>
    onChange(names.map((k) => ({ key: k, value: k === n ? v : value(k) })).filter((x) => x.value !== ""));
  return (
    <div className="mt-3">
      <div className="mb-1.5 flex items-baseline gap-2">
        <span className="text-[12.5px] font-semibold">Path variables</span>
        <span className="muted text-[12px]">Used when you send it here. Tools and agents pass their own.</span>
      </div>
      <div className="kv-grid">
        {names.map((n) => (
          <div key={n} className="flex items-center gap-2">
            <code className="mono kv-key">{n}</code>
            <input
              className="input mono"
              aria-label={`Path variable ${n}`}
              placeholder="Value"
              value={value(n)}
              onChange={(e) => set(n, e.target.value)}
            />
          </div>
        ))}
      </div>
    </div>
  );
}

/** SavedResponses lists the kept responses of a request; click to view. */
function SavedResponses({
  req,
  onChange,
  open,
  setOpen,
}: {
  req: Request;
  onChange: (r: Request) => void;
  open: string;
  setOpen: (id: string) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (open) ref.current?.scrollIntoView?.({ block: "nearest", behavior: "smooth" });
  }, [open]);
  const remove = useAction((rid: string) => del<Request>(`/api/requests/${req.id}/responses/${rid}`), {
    onSuccess: (x) => onChange(x),
  });
  const shown = req.responses?.find((x) => x.id === open);
  return (
    <div ref={ref} style={{ borderTop: "1px solid var(--line)" }}>
      <div className="flex flex-wrap items-center gap-2 px-3 py-2.5">
        <span className="muted text-[12.5px] font-medium">Saved responses</span>
        {req.responses!.map((x) => (
          <button
            key={x.id}
            type="button"
            className="saved-resp"
            aria-pressed={open === x.id}
            onClick={() => setOpen(open === x.id ? "" : x.id)}
          >
            <span className={`dot ${x.status < 300 ? "on" : x.status < 500 ? "warn" : "bad"}`} />
            <span className="mono">{x.status}</span> {x.name}
          </button>
        ))}
      </div>
      {shown && (
        <div className="px-3 pb-3">
          <div className="mb-1.5 flex items-center gap-2 text-[12px]">
            <span className="muted">Saved {ago(shown.savedAt)} · masked</span>
            <Button
              size="sm"
              kind="ghost"
              className="ml-auto"
              icon={<Trash2 size={13} />}
              onClick={() => {
                setOpen("");
                remove.mutate(shown.id);
              }}
            >
              Delete
            </Button>
          </div>
          <pre className="code" style={{ maxHeight: 300 }}>
            {pretty(JSON.stringify(shown.body))}
          </pre>
        </div>
      )}
    </div>
  );
}

/** OutputCheck says what an agent would pay to read this response (GUIDE-3). */
function OutputCheck({ body }: { body: string }) {
  const [open, setOpen] = useState(false);
  const q = useQuery({
    queryKey: ["lint-output", body.length, body.slice(0, 200)],
    queryFn: () => post<OutputReport>("/api/lint/output", { value: JSON.parse(body) }),
    staleTime: Number.POSITIVE_INFINITY,
  });
  if (!q.data) return null;
  const r = q.data;
  const c = counts(r.findings);
  return (
    <div style={{ borderTop: "1px solid var(--line)" }}>
      <button
        type="button"
        className="flex w-full items-center gap-3 px-3 py-2.5 text-left"
        style={{ background: "none", border: 0, cursor: "pointer", color: "inherit" }}
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <Gauge size={15} className="muted flex-none" />
        <span className="text-[12.5px]">
          <strong>If an agent read this:</strong> {num(r.tokens)} tokens · {r.fields} fields · {r.depth} levels deep
        </span>
        <span className="ml-auto flex gap-1.5">
          {c.block > 0 && <Chip tone="bad">{c.block} blocking</Chip>}
          {c.warn > 0 && (
            <Chip tone="warn">
              {c.warn} {c.warn === 1 ? "warning" : "warnings"}
            </Chip>
          )}
          {c.block + c.warn === 0 && <Chip tone="ok">Fine for agents</Chip>}
        </span>
        <ChevronDown size={14} className="muted" style={{ transform: open ? "rotate(180deg)" : undefined }} />
      </button>
      {open && (
        <div
          className="grid gap-0 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]"
          style={{ borderTop: "1px solid var(--line)" }}
        >
          <FindingList findings={r.findings} empty="Small enough to return as is." />
          <div className="p-3" style={{ borderLeft: "1px solid var(--line)" }}>
            <div className="muted mb-2 text-[12px] font-semibold">Largest fields</div>
            {r.largest.map((f) => (
              <div key={f.path} className="flex items-center gap-2 py-1 text-[12.5px]">
                <span className="mono min-w-0 flex-1 truncate">{f.path}</span>
                <span className="muted">{num(f.tokens)} tok</span>
              </div>
            ))}
          </div>
        </div>
      )}
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
          className="input mono min-w-[7rem]"
          aria-label="Variable name"
          placeholder="baseUrl"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </td>
      <td style={{ width: 130 }}>
        <Select
          label="Source"
          size="sm"
          value={source}
          onChange={setSource}
          options={[
            { value: "literal", label: "Value" },
            { value: "secret", label: "Secret", hint: "Encrypted, write-only" },
            { value: "os_env", label: "OS env var", hint: "Read from mcpDeck's environment" },
          ]}
        />
      </td>
      <td>
        {source === "secret" && !editing ? (
          <span className="flex items-center gap-2">
            <span
              className="secret-set"
              title="Encrypted in mcpDeck's database with a key kept in the OS keychain or a key file (Settings → Security). It is never shown again."
            >
              <Lock size={12} /> Set · Encrypted
            </span>
            <Button size="sm" onClick={() => setEditing(true)} aria-label={`Replace ${name}`}>
              Replace
            </Button>
          </span>
        ) : (
          <span className="flex items-center gap-2">
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
            {source === "secret" && v?.isSet && (
              <Button
                size="sm"
                kind="ghost"
                onClick={() => {
                  setValue("");
                  setEditing(false);
                }}
              >
                Keep current
              </Button>
            )}
            {source === "secret" && v?.source === "secret" && !v.isSet && <Chip tone="warn">Not set</Chip>}
          </span>
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
  const [renaming, setRenaming] = useState("");
  const [importing, setImporting] = useState(false);
  const toast = useToast();
  const confirm = useConfirm();
  const add = useAction(() => post<{ id: string }>(`/api/apis/${a.id}/environments`, { name: newName }), {
    onSuccess: (e) => {
      setNewName("");
      setEnv(e.id);
    },
  });
  const rename = useAction((x: { id: string; name: string }) => patch(`/api/environments/${x.id}`, { name: x.name }), {
    onSuccess: () => {
      setRenaming("");
      toast("Environment renamed");
    },
    onError: (e) => toast((e as Error).message, true),
  });
  const setActive = useAction((id: string) => patch(`/api/apis/${a.id}`, { activeEnvId: id }), {
    onSuccess: () => toast("Active environment changed"),
  });
  const remove = useAction((id: string) => del(`/api/environments/${id}`), {
    onSuccess: () => setEnv(a.activeEnvId),
  });
  const vars = a.variables.filter((v) => v.envId === env);
  const cur = a.environments.find((e) => e.id === env);
  return (
    <div className="grid items-start gap-4" style={{ gridTemplateColumns: "250px minmax(0, 1fr)" }}>
      <Panel
        bodyClass="p-1.5"
        title="Environments"
        right={
          <Button size="sm" kind="ghost" icon={<Upload size={13} />} onClick={() => setImporting(true)}>
            Import
          </Button>
        }
      >
        <button
          type="button"
          className="tree-row"
          aria-current={env === "" ? "true" : undefined}
          onClick={() => setEnv("")}
        >
          <Layers size={14} className="muted" />
          <span className="flex-1">Shared by all</span>
          <span className="faint text-[11px]">{a.variables.filter((v) => v.envId === "").length}</span>
        </button>
        {a.environments.map((e) =>
          renaming === e.id ? (
            <form
              key={e.id}
              className="p-1"
              onSubmit={(ev) => {
                ev.preventDefault();
                const name = new FormData(ev.currentTarget).get("name") as string;
                rename.mutate({ id: e.id, name });
              }}
            >
              <input
                autoFocus
                name="name"
                className="input"
                aria-label={`New name for ${e.name}`}
                defaultValue={e.name}
                onBlur={() => setRenaming("")}
                onKeyDown={(k) => k.key === "Escape" && setRenaming("")}
              />
            </form>
          ) : (
            <button
              key={e.id}
              type="button"
              className="tree-row group"
              aria-current={env === e.id ? "true" : undefined}
              onClick={() => setEnv(e.id)}
              onDoubleClick={() => setRenaming(e.id)}
            >
              <span className={`dot ${e.id === a.activeEnvId ? "on" : ""}`} />
              <span className="min-w-0 flex-1 truncate">{e.name}</span>
              {e.id === a.activeEnvId && <Chip tone="ok">active</Chip>}
            </button>
          ),
        )}
        <form
          className="flex gap-1.5 p-1.5"
          onSubmit={(ev) => {
            ev.preventDefault();
            if (newName) add.mutate(undefined);
          }}
        >
          <input
            className="input"
            aria-label="New environment"
            placeholder="New environment, like Staging"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
          />
          <Button size="sm" disabled={!newName} type="submit" aria-label="Add environment" icon={<Plus size={13} />} />
        </form>
        <ErrorBox error={add.error || remove.error} />
      </Panel>
      <Panel
        title={
          cur ? (
            <span className="flex items-center gap-2">
              {cur.name}
              {cur.id === a.activeEnvId && <Chip tone="ok">active</Chip>}
            </span>
          ) : (
            "Shared by all environments"
          )
        }
        right={
          cur && (
            <>
              {cur.id !== a.activeEnvId && (
                <Button size="sm" onClick={() => setActive.mutate(cur.id)}>
                  Make active
                </Button>
              )}
              <Button size="sm" kind="ghost" icon={<Pencil size={13} />} onClick={() => setRenaming(cur.id)}>
                Rename
              </Button>
              <Menu
                label="Environment actions"
                items={[
                  {
                    label: "Delete environment",
                    icon: <Trash2 size={14} />,
                    danger: true,
                    onClick: async () => {
                      if (cur.id === a.activeEnvId) {
                        toast(`${cur.name} is active; make another environment active first`, true);
                        return;
                      }
                      if (
                        await confirm({
                          title: `Delete ${cur.name}?`,
                          body: `Its ${vars.length} variables, including secrets, are deleted. Agents whose grants use it fall back to the active environment.`,
                          action: "Delete environment",
                          danger: true,
                          typeToConfirm: cur.name,
                        })
                      )
                        remove.mutate(cur.id);
                    },
                  },
                ]}
              />
            </>
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
          shared ones. Secrets are encrypted and never shown again. Double-click an environment to rename it.
        </div>
      </Panel>
      {importing && <ImportEnvDialog a={a} onClose={() => setImporting(false)} onDone={(id) => setEnv(id)} />}
    </div>
  );
}

/** ImportEnvDialog reads a Postman, Bruno or .env export, shows it, then saves it (ENV-2). */
function ImportEnvDialog({ a, onClose, onDone }: { a: APIDetail; onClose: () => void; onDone: (id: string) => void }) {
  const [file, setFile] = useState<{ name: string; content: string } | null>(null);
  const [text, setText] = useState("");
  const [name, setName] = useState("");
  const [over, setOver] = useState(false);
  const toast = useToast();
  const src = file ?? (text.trim() ? { name: "", content: text } : null);
  const preview = useQuery({
    queryKey: ["env-preview", src?.name, src?.content.length, src?.content.slice(0, 64)],
    queryFn: () =>
      post<EnvImport>(`/api/apis/${a.id}/environments/import`, {
        filename: src!.name,
        content: src!.content,
        preview: true,
      }),
    enabled: !!src,
    retry: false,
  });
  useEffect(() => {
    if (preview.data && !name) setName(preview.data.parsed.name);
  }, [preview.data]);
  const save = useAction(
    () =>
      post<EnvImport>(`/api/apis/${a.id}/environments/import`, {
        filename: src!.name,
        content: src!.content,
        name,
      }),
    {
      onSuccess: (r) => {
        toast(`${r.environment!.name} imported with ${r.parsed.vars.length} variables`);
        onDone(r.environment!.id);
        onClose();
      },
    },
  );
  const read = async (f: File) => setFile({ name: f.name, content: await f.text() });
  const p = preview.data;
  return (
    <Dialog
      wide
      title="Import an environment"
      sub="From Postman (Export environment), Bruno (an environment's .bru or JSON), or a .env file. You'll see the variables before anything is saved."
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            kind="ink"
            disabled={!p || !name.trim()}
            loading={save.isPending}
            onClick={() => save.mutate(undefined)}
          >
            Import {p ? `${p.parsed.vars.length} variables` : ""}
          </Button>
        </>
      }
    >
      {!file ? (
        <>
          <label
            className={`dropzone${over ? " over" : ""}`}
            onDragOver={(e) => {
              e.preventDefault();
              setOver(true);
            }}
            onDragLeave={() => setOver(false)}
            onDrop={(e) => {
              e.preventDefault();
              setOver(false);
              const f = e.dataTransfer.files[0];
              if (f) read(f);
            }}
          >
            <Upload size={20} className="muted" />
            <span className="font-medium">Drop the exported file here, or click to choose</span>
            <span className="muted text-[12.5px]">.postman_environment.json · .bru · .json · .env</span>
            <input
              type="file"
              className="sr-only"
              accept=".json,.bru,.env,text/plain,application/json"
              onChange={(e) => e.target.files?.[0] && read(e.target.files[0])}
            />
          </label>
          <div className="muted my-3 text-center text-[12px]">or paste it</div>
          <textarea
            className="textarea"
            rows={5}
            aria-label="Environment file contents"
            placeholder={"vars {\n  baseUrl: https://api.acme.example\n}\nvars:secret [\n  token\n]"}
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
        </>
      ) : (
        <div className="mb-3 flex items-center gap-2">
          <FileUp size={15} className="muted" />
          <span className="mono">{file.name}</span>
          <Button size="sm" kind="ghost" className="ml-auto" onClick={() => setFile(null)}>
            Choose another
          </Button>
        </div>
      )}
      {preview.error && <ErrorBox error={preview.error} />}
      {p && (
        <div className="mt-4">
          <div className="mb-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Field label="Environment name" hint={`Found a ${p.parsed.kind} environment.`}>
              <input className="input" value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <div className="flex gap-1.5 pb-5">
              <Chip>{p.parsed.vars.length} variables</Chip>
              {p.secrets > 0 && <Chip tone="info">{p.secrets} secret</Chip>}
            </div>
          </div>
          <div className="panel" style={{ maxHeight: 260, overflow: "auto" }}>
            <table className="table">
              <tbody>
                {p.parsed.vars.map((v) => (
                  <tr key={v.name}>
                    <td className="mono">{v.name}</td>
                    <td className="mono muted truncate" style={{ maxWidth: 320 }}>
                      {v.preview || (v.secret ? "(empty: fill in after import)" : "")}
                    </td>
                    <td className="num">
                      {v.secret ? <Chip tone="info">secret, encrypted</Chip> : <Chip>value</Chip>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {p.emptySecrets.length > 0 && (
            <div className="banner warn mt-3">
              {p.emptySecrets.join(", ")} came without values (Bruno never exports secrets). Fill them in after the
              import.
            </div>
          )}
        </div>
      )}
      <ErrorBox error={save.error} />
    </Dialog>
  );
}

export function AuthTab({ a }: { a: APIDetail }) {
  const [auth, setAuth] = useState<Auth>(a.auth ?? { type: "none" });
  const toast = useToast();
  const save = useAction(() => patch<APIDetail>(`/api/apis/${a.id}`, { auth: cleanAuth(auth) }), {
    onSuccess: (saved) => {
      if (saved?.auth) setAuth(saved.auth);
      toast("Authentication saved");
    },
  });
  const secrets = a.variables.filter((v) => v.source === "secret");
  const set = (p: Partial<Auth>) => setAuth({ ...auth, ...p });
  const dirty = JSON.stringify(auth) !== JSON.stringify(a.auth ?? { type: "none" });
  return (
    <div className="grid items-start gap-5 min-[1400px]:grid-cols-2">
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
          <Select
            label="Auth type"
            value={auth.type}
            onChange={(v) => set({ type: v })}
            options={[
              { value: "none", label: "None" },
              { value: "bearer", label: "Bearer token" },
              { value: "apikey", label: "API key" },
              { value: "basic", label: "Username and password" },
              { value: "login", label: "Login request", hint: "Log in first; keep the cookie, a header or a token" },
              { value: "oauth2", label: "OAuth 2.0", hint: "Client credentials or password grant" },
              { value: "hmac", label: "Signed requests (HMAC)", hint: "Sign each request with a shared secret" },
              { value: "browser", label: "Browser sign-in", hint: "Company single sign-on and MFA, in your browser" },
            ]}
          />
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
              <Select
                label="Send in"
                value={auth.in ?? "header"}
                onChange={(v) => set({ in: v })}
                options={[
                  { value: "header", label: "Header" },
                  { value: "query", label: "Query" },
                ]}
              />
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
        {auth.type === "login" && <LoginFields a={a} auth={auth} set={set} />}
        {auth.type === "oauth2" && <OAuthFields auth={auth} set={set} />}
        {auth.type === "hmac" && <HMACFields auth={auth} set={set} />}
        {auth.type === "browser" && <BrowserFields apiId={a.id} auth={auth} set={set} />}
        {(auth.type === "oauth2" || auth.type === "hmac" || (auth.type === "login" && !auth.loginRequestId)) && (
          <Field
            label="Headers on every call"
            hint={
              auth.type === "hmac"
                ? "Use {{sign.signature}}, {{sign.timestamp}} and {{sign.nonce}}"
                : auth.type === "login"
                  ? "Use captures, e.g. Authorization: Bearer {{login.access_token}}"
                  : "Optional; the token is sent as a bearer token"
            }
          >
            <KVEditor rows={auth.apply ?? []} onChange={(apply) => set({ apply })} keyLabel="Header" />
          </Field>
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

type SetAuth = (p: Partial<Auth>) => void;

const authKeys: Record<string, (keyof Auth)[]> = {
  bearer: ["token"],
  apikey: ["key", "value", "in"],
  basic: ["username", "password"],
  login: ["loginRequestId", "capture", "steps", "refreshRequestId", "expiresFrom", "apply"],
  oauth2: [
    "grant",
    "tokenUrl",
    "clientId",
    "clientSecret",
    "clientAuth",
    "scope",
    "audience",
    "username",
    "password",
    "apply",
  ],
  hmac: ["algorithm", "secret", "stringToSign", "encoding", "apply"],
  browser: ["startUrl", "signedIn"],
};

/** cleanAuth keeps only the fields the auth type uses (a token left from
 * an earlier type would ask for a credential nothing uses) and drops
 * capture rows left blank. */
export function cleanAuth(au: Auth): Auth {
  const keys = authKeys[au.type];
  const out: Auth = keys ? { type: au.type } : { ...au };
  if (keys) for (const k of keys) if (au[k] !== undefined) (out as Record<string, unknown>)[k] = au[k];
  if (out.steps)
    out.steps = out.steps.map((st) => ({
      ...st,
      captures: (st.captures ?? []).filter((c) => c.name.trim() || !/^((json|header|cookie):?)?$/.test(c.from.trim())),
    }));
  return out;
}

function RequestSelect({
  a,
  label,
  value,
  onChange,
  placeholder = "Choose a request",
  suggest,
}: {
  a: APIDetail;
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  /** Put requests that look like a sign-in first. */
  suggest?: boolean;
}) {
  const rs = suggest ? byLogin(a.requests) : a.requests;
  return (
    <Select
      label={label}
      placeholder={placeholder}
      value={value}
      onChange={onChange}
      options={rs.map((r) => ({
        value: r.id,
        label: (
          <span className="flex items-center gap-2">
            <MethodChip method={r.method} /> {r.name}
          </span>
        ),
        hint: suggest && loginScore(r) >= 3 ? "Looks like a sign-in request" : undefined,
      }))}
    />
  );
}

function LoginFields({ a, auth, set }: { a: APIDetail; auth: Auth; set: SetAuth }) {
  // A new login starts with one step; the single-request shorthand stays
  // for logins saved that way.
  const steps = auth.steps?.length ? auth.steps : auth.loginRequestId ? [] : [{ requestId: "", captures: [] }];
  const bearer = (name: string) =>
    set({
      apply: [
        ...(auth.apply ?? []).filter((h) => h.key.toLowerCase() !== "authorization"),
        { key: "Authorization", value: `Bearer {{login.${name}}}` },
      ],
    });
  if (steps.length === 0) {
    return (
      <>
        <div className="grid grid-cols-2 gap-2">
          <Field label="Login request" hint="Run when needed; tools never handle the login">
            <RequestSelect
              a={a}
              label="Login request"
              suggest
              value={auth.loginRequestId ?? ""}
              onChange={(v) => set({ loginRequestId: v })}
            />
          </Field>
          <Field
            label="Capture"
            hint="cookie, a response header name, or json:access_token to send a body field as a bearer token"
          >
            <input
              className="input mono"
              value={auth.capture ?? "cookie"}
              onChange={(e) => set({ capture: e.target.value })}
            />
          </Field>
        </div>
        <div>
          <Button
            kind="ghost"
            onClick={() =>
              set({
                steps: [{ requestId: auth.loginRequestId ?? "", captures: [] }],
                loginRequestId: undefined,
                capture: undefined,
              })
            }
          >
            Use several steps
          </Button>
          <span className="muted ml-2 text-[12px]">For a login form with a CSRF token, or a token exchange.</span>
        </div>
      </>
    );
  }
  const setStep = (i: number, st: LoginStep) => set({ steps: steps.map((x, j) => (j === i ? st : x)) });
  return (
    <>
      {steps.map((st, i) => (
        <div key={i} className="login-step">
          <Field label={`Step ${i + 1}`} hint={i === 0 ? "Runs first" : "Can use {{login.<name>}} from earlier steps"}>
            <RequestSelect
              a={a}
              label={`Step ${i + 1} request`}
              suggest={i === 0}
              value={st.requestId}
              onChange={(v) => setStep(i, { ...st, requestId: v })}
            />
          </Field>
          <CaptureRows
            apiId={a.id}
            step={st}
            index={i}
            request={a.requests.find((r) => r.id === st.requestId)}
            onChange={(captures) => setStep(i, { ...st, captures })}
            onBearer={bearer}
          />
        </div>
      ))}
      <div className="flex gap-2">
        <Button kind="ghost" onClick={() => set({ steps: [...steps, { requestId: "", captures: [] }] })}>
          Add step
        </Button>
        {steps.length > 1 && (
          <Button kind="ghost" onClick={() => set({ steps: steps.slice(0, -1) })}>
            Remove last step
          </Button>
        )}
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Field label="Refresh request" hint="Optional; runs instead of the steps once the session expires">
          <RequestSelect
            a={a}
            label="Refresh request"
            placeholder="None"
            value={auth.refreshRequestId ?? ""}
            onChange={(v) => set({ refreshRequestId: v })}
          />
        </Field>
        <Field label="Session lifetime" hint="json:expires_in from the last response, or seconds; default 30 minutes">
          <input
            className="input mono"
            value={auth.expiresFrom ?? ""}
            placeholder="json:expires_in"
            onChange={(e) => set({ expiresFrom: e.target.value })}
          />
        </Field>
      </div>
    </>
  );
}

function OAuthFields({ auth, set }: { auth: Auth; set: SetAuth }) {
  const grant = auth.grant ?? "client_credentials";
  const text = (k: keyof Auth, label: string, placeholder: string, hint?: string) => (
    <Field label={label} hint={hint}>
      <input
        className="input mono"
        aria-label={label}
        value={(auth[k] as string | undefined) ?? ""}
        placeholder={placeholder}
        onChange={(e) => set({ [k]: e.target.value })}
      />
    </Field>
  );
  return (
    <>
      <div className="grid grid-cols-2 gap-2">
        <Field label="Grant">
          <Select
            label="Grant"
            value={grant}
            onChange={(v) => set({ grant: v })}
            options={[
              { value: "client_credentials", label: "Client credentials" },
              { value: "password", label: "Password" },
            ]}
          />
        </Field>
        {text("tokenUrl", "Token URL", "https://idp.example.com/oauth2/token")}
      </div>
      <div className="grid grid-cols-2 gap-2">
        {text("clientId", "Client ID", "{{clientId}}")}
        {text("clientSecret", "Client secret", "{{clientSecret}}", "Point it at a secret variable")}
      </div>
      {grant === "password" && (
        <div className="grid grid-cols-2 gap-2">
          {text("username", "Username", "{{username}}")}
          {text("password", "Password", "{{password}}")}
        </div>
      )}
      <div className="grid grid-cols-3 gap-2">
        {text("scope", "Scope", "read")}
        {text("audience", "Audience", "")}
        <Field label="Send client secret">
          <Select
            label="Send client secret"
            value={auth.clientAuth ?? "basic"}
            onChange={(v) => set({ clientAuth: v })}
            options={[
              { value: "basic", label: "Basic header" },
              { value: "body", label: "Request body" },
            ]}
          />
        </Field>
      </div>
    </>
  );
}

function HMACFields({ auth, set }: { auth: Auth; set: SetAuth }) {
  return (
    <>
      <div className="grid grid-cols-3 gap-2">
        <Field label="Secret" hint="Point it at a secret variable">
          <input
            className="input mono"
            aria-label="Signing secret"
            value={auth.secret ?? ""}
            placeholder="{{signingSecret}}"
            onChange={(e) => set({ secret: e.target.value })}
          />
        </Field>
        <Field label="Algorithm">
          <Select
            label="Algorithm"
            value={auth.algorithm ?? "sha256"}
            onChange={(v) => set({ algorithm: v })}
            options={[
              { value: "sha256", label: "HMAC-SHA256" },
              { value: "sha512", label: "HMAC-SHA512" },
              { value: "sha1", label: "HMAC-SHA1" },
            ]}
          />
        </Field>
        <Field label="Encoding">
          <Select
            label="Encoding"
            value={auth.encoding ?? "hex"}
            onChange={(v) => set({ encoding: v })}
            options={[
              { value: "hex", label: "Hex" },
              { value: "base64", label: "Base64" },
            ]}
          />
        </Field>
      </div>
      <Field
        label="String to sign"
        hint="{{method}} {{path}} {{query}} {{timestamp}} {{nonce}} {{bodySha256}} {{bodyMd5}} and variables, one per line"
      >
        <textarea
          className="textarea mono"
          rows={4}
          aria-label="String to sign"
          placeholder={"{{method}}\n{{path}}\n{{timestamp}}\n{{bodySha256}}"}
          value={(auth.stringToSign ?? "").replaceAll("\\n", "\n")}
          onChange={(e) => set({ stringToSign: e.target.value })}
        />
      </Field>
    </>
  );
}

export function TLSPanel({ a }: { a: APIDetail }) {
  const init = {
    insecure: a.tls?.insecure ?? false,
    caCerts: a.tls?.caCerts ?? "",
    clientCert: a.tls?.clientCert ?? "",
    clientKey: a.tls?.clientKey ?? "",
  };
  const secrets = a.variables.filter((v) => v.source === "secret");
  const [t, setT] = useState(init);
  const toast = useToast();
  const dirty = JSON.stringify(t) !== JSON.stringify(init);
  const save = useAction(() => patch(`/api/apis/${a.id}`, { tls: t }), {
    onSuccess: () => toast("TLS settings saved"),
  });
  return (
    <Panel
      title="TLS and certificates"
      icon={<ShieldAlert size={15} className="muted" />}
      className="mt-5"
      footer={
        dirty && (
          <>
            <Button kind="ghost" onClick={() => setT(init)}>
              Discard
            </Button>
            <Button kind="ink" loading={save.isPending} onClick={() => save.mutate(undefined)}>
              Save TLS settings
            </Button>
          </>
        )
      }
    >
      <div className="settings-row">
        <div className="what">
          <strong>Verify certificates</strong>
          <span>
            Turn off only for a dev or internal server with a self-signed certificate you trust. Anyone in between could
            read the traffic.
          </span>
        </div>
        <div className="flex items-center gap-3">
          <Toggle label="Verify certificates" checked={!t.insecure} onChange={(on) => setT({ ...t, insecure: !on })} />
          {t.insecure ? <Chip tone="warn">not verified</Chip> : <Chip tone="ok">verified</Chip>}
        </div>
      </div>
      <div className="settings-row">
        <div className="what">
          <strong>Trusted certificates</strong>
          <span>
            PEM certificates to trust for this application, on top of the system ones: your company's CA, or the
            server's own certificate. Safer than turning verification off.
          </span>
        </div>
        <textarea
          className="textarea"
          rows={5}
          aria-label="Trusted certificates"
          placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----"}
          value={t.caCerts}
          onChange={(e) => setT({ ...t, caCerts: e.target.value })}
        />
      </div>
      <div className="settings-row">
        <div className="what">
          <strong>Client certificate</strong>
          <span>
            For servers that require mutual TLS. Paste the PEM certificate; the key stays in a secret variable.
          </span>
        </div>
        <div className="flex w-full flex-col gap-2">
          <textarea
            className="textarea"
            rows={4}
            aria-label="Client certificate"
            placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----"}
            value={t.clientCert}
            onChange={(e) => setT({ ...t, clientCert: e.target.value })}
          />
          <Select
            label="Client key"
            placeholder={secrets.length ? "Secret variable holding the key" : "Add a secret variable for the key first"}
            value={t.clientKey.replace(/^\{\{\s*|\s*\}\}$/g, "")}
            onChange={(v) => setT({ ...t, clientKey: v ? `{{${v}}}` : "" })}
            options={secrets.map((v) => ({ value: v.name, label: v.name }))}
          />
        </div>
      </div>
      <div className="px-4 pb-3">
        <ErrorBox error={save.error} />
      </div>
    </Panel>
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
    <div style={{ maxWidth: 1040 }}>
      <Panel title="Application">
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
      <TLSPanel a={a} />
      <FilesPanel apiId={a.id} dir={a.filesDir} />
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
