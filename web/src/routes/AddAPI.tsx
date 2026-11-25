import { FileJson, FileUp, Globe, KeyRound, Loader2, PencilLine, Terminal, Trash2, Upload, X } from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import {
  AccessChip,
  Button,
  Chip,
  ErrorBox,
  Field,
  MethodChip,
  PageHead,
  Panel,
  useToast,
} from "../components/ui";
import { patch, post } from "../lib/api";
import { useAction, useAPI, useAPIs } from "../lib/hooks";
import { appName, appNameError, detectImport, type ImportKind } from "../lib/names";
import { Link, navigate } from "../lib/router";
import type { API, ImportPreview, ImportReport } from "../lib/types";

type Source = "files" | "url" | "curl" | "manual";
const sources: { id: Source; label: string; sub: string; icon: ReactNode }[] = [
  { id: "files", label: "Upload files", sub: "Postman, OpenAPI or Swagger", icon: <Upload size={17} /> },
  { id: "url", label: "From a link", sub: "Spec URL or docs page", icon: <Globe size={17} /> },
  { id: "curl", label: "Paste curl", sub: "One or more commands", icon: <Terminal size={17} /> },
  { id: "manual", label: "Add manually", sub: "Name, base URL and auth", icon: <PencilLine size={17} /> },
];

const kindLabel: Record<ImportKind, string> = {
  postman: "Postman collection",
  "postman-env": "Postman environment",
  spec: "OpenAPI spec",
  curl: "curl",
  unknown: "Unrecognised",
};

/** One thing to import: a file, a link or pasted curl, with its preview. */
type Item = {
  key: string;
  label: string;
  kind: ImportKind;
  text: string;
  url: string;
  preview?: ImportPreview;
  error?: string;
  loading?: boolean;
  skip: Set<number>;
};

let seq = 0;

export function AddAPI({ apiId }: { apiId?: string }) {
  const target = useAPI(apiId ?? "");
  const apis = useAPIs();
  const toast = useToast();
  const [src, setSrc] = useState<Source>("files");
  const [items, setItems] = useState<Item[]>([]);
  const [env, setEnv] = useState<{ name: string; text: string } | null>(null);
  const [url, setUrl] = useState("");
  const [curl, setCurl] = useState("");
  const [name, setName] = useState("");
  const [nameTouched, setNameTouched] = useState(false);
  const [title, setTitle] = useState("");
  const [over, setOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const taken = (apis.data ?? []).map((a) => a.name);
  const isImport = !!apiId;

  const previewItem = async (it: Item, envText = env?.text ?? "") => {
    setItems((l) => l.map((x) => (x.key === it.key ? { ...x, loading: true, error: undefined } : x)));
    try {
      const p = await post<ImportPreview>("/api/import/preview", {
        url: it.url,
        text: it.text,
        environment: it.kind === "postman" ? envText : "",
      });
      setItems((l) => l.map((x) => (x.key === it.key ? { ...x, preview: p, loading: false } : x)));
      if (!isImport && !nameTouched) {
        setName((n) => n || p.suggestName || appName(p.title));
        setTitle((t) => t || p.title);
      }
    } catch (e) {
      setItems((l) => l.map((x) => (x.key === it.key ? { ...x, loading: false, error: (e as Error).message } : x)));
    }
  };

  const addFiles = async (files: FileList | File[]) => {
    const added: Item[] = [];
    let envText = env?.text ?? "";
    for (const f of Array.from(files)) {
      const text = await f.text();
      const kind = detectImport(text);
      if (kind === "postman-env") {
        setEnv({ name: f.name, text });
        envText = text;
        continue;
      }
      added.push({ key: `f${++seq}`, label: f.name, kind, text, url: "", skip: new Set() });
    }
    setItems((l) => [...l, ...added]);
    for (const it of added) previewItem(it, envText);
    // A new environment applies to collections already added too.
    if (envText !== (env?.text ?? "")) for (const it of items.filter((x) => x.kind === "postman")) previewItem(it, envText);
  };

  const addURL = () => {
    if (!url.trim()) return;
    const it: Item = { key: `u${++seq}`, label: url.trim(), kind: "spec", text: "", url: url.trim(), skip: new Set() };
    setItems((l) => [...l.filter((x) => !x.url), it]);
    previewItem(it);
  };

  // curl previews update as you paste (UI-14).
  useEffect(() => {
    if (src !== "curl") return;
    const t = setTimeout(() => {
      const text = curl.trim();
      const rest = items.filter((x) => x.kind !== "curl");
      if (!text.startsWith("curl")) {
        if (rest.length !== items.length) setItems(rest);
        return;
      }
      const it: Item = { key: "curl", label: "Pasted curl", kind: "curl", text, url: "", skip: new Set() };
      setItems([...rest, it]);
      previewItem(it);
    }, 350);
    return () => clearTimeout(t);
  }, [curl, src]);

  const remove = (key: string) => setItems((l) => l.filter((x) => x.key !== key));
  const toggle = (key: string, i: number | "all", on: boolean) =>
    setItems((l) =>
      l.map((x) => {
        if (x.key !== key) return x;
        const n = x.preview?.requests.length ?? 0;
        if (i === "all") return { ...x, skip: on ? new Set() : new Set(Array.from({ length: n }, (_, k) => k)) };
        const s = new Set(x.skip);
        on ? s.delete(i) : s.add(i);
        return { ...x, skip: s };
      }),
    );

  const ready = items.filter((x) => x.preview);
  const count = ready.reduce((n, x) => n + (x.preview!.requests.length - x.skip.size), 0);
  const secrets = ready.flatMap((x) => (x.preview!.vars ?? []).filter((v) => v.secret));
  const warnings = ready.flatMap((x) => x.preview!.warnings ?? []);
  const nameErr = isImport ? "" : appNameError(name, taken);
  const shownName = (isImport ? target.data?.name : name) || "app";

  const doImport = async () => {
    setErr(null);
    setBusy(true);
    let id = apiId ?? "";
    let added = 0;
    try {
      for (const it of ready) {
        if (it.preview!.requests.length === it.skip.size) continue;
        const r = await post<ImportReport>("/api/import", {
          url: it.url,
          text: it.text,
          environment: it.kind === "postman" ? (env?.text ?? "") : "",
          apiId: id,
          name: id ? "" : name,
          skip: [...it.skip],
        });
        id = r.apiId;
        added += r.added.length + r.updated.length;
      }
      if (!isImport && title.trim()) await patch(`/api/apis/${id}`, { title: title.trim() }).catch(() => {});
      toast(`${added} requests ${isImport ? "imported" : "added"}`);
      navigate(`/apps/${id}?imported=${added}`);
    } catch (e) {
      setErr(e);
      // Part of a batch may already be saved: go there so nothing is lost.
      if (id && !apiId) navigate(`/apps/${id}?imported=${added}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="page" style={{ maxWidth: 1080 }}>
      <PageHead
        crumbs={
          <>
            <Link to="/apps">Applications</Link> /{" "}
            {isImport ? <Link to={`/apps/${apiId}`}>{target.data?.title || target.data?.name}</Link> : "New"}
          </>
        }
        title={isImport ? "Import requests" : "New application"}
        sub={
          isImport
            ? `Add more requests to ${target.data?.title || target.data?.name || "this application"}. Requests with the same name are updated; nothing is deleted.`
            : "An application is one system your tools call. Bring its requests in from files, a link or curl, or start empty. You'll see every request before anything is saved."
        }
      />

      <div className="mb-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {sources
          .filter((s) => !(isImport && s.id === "manual"))
          .map((s) => (
            <button
              key={s.id}
              type="button"
              className="choice"
              aria-pressed={src === s.id}
              onClick={() => {
                setSrc(s.id);
                setErr(null);
              }}
            >
              <span className={`icon-tile ${src === s.id ? "" : "plain"}`}>{s.icon}</span>
              <span className="min-w-0">
                <span className="block font-semibold">{s.label}</span>
                <span className="muted block text-[12px]">{s.sub}</span>
              </span>
            </button>
          ))}
      </div>

      {src === "manual" ? (
        <Manual taken={taken} />
      ) : (
        <div className="flex flex-col gap-5">
          {src === "files" && (
            <div>
              <div
                className={`dropzone${over ? " over" : ""}`}
                role="button"
                tabIndex={0}
                onClick={() => fileInput.current?.click()}
                onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && fileInput.current?.click()}
                onDragOver={(e) => {
                  e.preventDefault();
                  setOver(true);
                }}
                onDragLeave={() => setOver(false)}
                onDrop={(e) => {
                  e.preventDefault();
                  setOver(false);
                  addFiles(e.dataTransfer.files);
                }}
              >
                <span className="icon-tile lg mb-1">
                  <FileUp size={20} />
                </span>
                <span className="text-[14px] font-semibold">Drop files here, or click to choose</span>
                <span className="muted text-[12.5px]">
                  Postman collections (v2.1) with their environment, OpenAPI 3 or Swagger 2 in JSON or YAML. Add as many
                  as you like.
                </span>
              </div>
              <input
                ref={fileInput}
                type="file"
                multiple
                accept=".json,.yaml,.yml,.txt"
                className="hidden"
                aria-label="Choose files to import"
                onChange={(e) => {
                  if (e.target.files) addFiles(e.target.files);
                  e.target.value = "";
                }}
              />
              {env && (
                <div className="mt-3 flex items-center gap-2 text-[12.5px]">
                  <Chip tone="info">
                    <KeyRound size={11} /> Environment
                  </Chip>
                  <span className="mono">{env.name}</span>
                  <span className="muted">applies to the Postman collections above</span>
                  <Button
                    size="sm"
                    kind="ghost"
                    aria-label="Remove environment"
                    icon={<X size={13} />}
                    onClick={() => setEnv(null)}
                  />
                </div>
              )}
            </div>
          )}
          {src === "url" && (
            <Panel bodyClass="panel-body">
              <Field
                label="Link to an OpenAPI spec, or to a Swagger or Redoc docs page"
                hint="For a docs page, mcpDeck finds the spec the page loads."
              >
                <div className="flex gap-2">
                  <input
                    className="input mono"
                    placeholder="https://petstore3.swagger.io/api/v3/openapi.json"
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && addURL()}
                  />
                  <Button kind="blue" disabled={!url.trim()} onClick={addURL}>
                    Read link
                  </Button>
                </div>
              </Field>
            </Panel>
          )}
          {src === "curl" && (
            <Panel bodyClass="panel-body">
              <Field
                label="curl commands"
                hint="One or more, from your browser's dev tools or an API's docs. Keys in headers or the query move into an encrypted credential."
              >
                <textarea
                  className="textarea"
                  rows={6}
                  placeholder="curl -H 'Authorization: Bearer …' https://api.acme.example/v1/invoices?status=overdue"
                  value={curl}
                  onChange={(e) => setCurl(e.target.value)}
                />
              </Field>
            </Panel>
          )}

          {items.map((it) => (
            <ItemPreview
              key={it.key}
              it={it}
              name={shownName}
              onRemove={() => remove(it.key)}
              onToggle={(i, on) => toggle(it.key, i, on)}
            />
          ))}

          {(secrets.length > 0 || warnings.length > 0) && (
            <div className="flex flex-col gap-2">
              {secrets.length > 0 && (
                <div className="banner info">
                  <KeyRound size={15} className="mt-0.5 flex-none" />
                  <span>
                    These move into an encrypted credential, never onto a request:{" "}
                    {secrets.map((s) => (
                      <code key={s.name + s.env} className="mr-1.5">
                        {s.name}
                      </code>
                    ))}
                  </span>
                </div>
              )}
              {warnings.map((w) => (
                <div key={w} className="banner warn">
                  {w}
                </div>
              ))}
            </div>
          )}

          {ready.length > 0 && (
            <Panel
              title={isImport ? "Import" : "Name the application"}
              footer={
                <>
                  <span className="muted mr-auto text-[12.5px]">
                    {count} of {ready.reduce((n, x) => n + x.preview!.requests.length, 0)} requests selected
                  </span>
                  <Button kind="ghost" onClick={() => navigate(isImport ? `/apps/${apiId}` : "/apps")}>
                    Cancel
                  </Button>
                  <Button
                    kind="ink"
                    loading={busy}
                    disabled={count === 0 || (!isImport && (!name || !!nameErr))}
                    onClick={doImport}
                  >
                    {isImport ? `Import ${count} requests` : `Create application with ${count} requests`}
                  </Button>
                </>
              }
            >
              {!isImport ? (
                <div className="panel-body grid gap-4 sm:grid-cols-2">
                  <Field label="Display name" hint="What people see in mcpDeck.">
                    <input
                      className="input"
                      placeholder="Acme CRM"
                      value={title}
                      onChange={(e) => {
                        setTitle(e.target.value);
                        if (!nameTouched) setName(appName(e.target.value));
                      }}
                    />
                  </Field>
                  <Field
                    label="Name in tool code"
                    error={nameErr}
                    hint={
                      <>
                        Tools call it as <code>{name || "acmeCrm"}.getAccount()</code>. Letters and digits.
                      </>
                    }
                  >
                    <input
                      className="input mono"
                      placeholder="acmeCrm"
                      aria-invalid={!!nameErr}
                      value={name}
                      onChange={(e) => {
                        setNameTouched(true);
                        setName(e.target.value.replace(/[^A-Za-z0-9]/g, ""));
                      }}
                    />
                  </Field>
                </div>
              ) : (
                <div className="panel-body muted text-[13px]">
                  New requests are added to <code>{target.data?.name}</code>; ones with the same name are updated.
                </div>
              )}
              {err !== null && (
                <div className="px-4 pb-4">
                  <ErrorBox error={err} />
                </div>
              )}
            </Panel>
          )}
        </div>
      )}
    </div>
  );
}

function ItemPreview({
  it,
  name,
  onRemove,
  onToggle,
}: {
  it: Item;
  name: string;
  onRemove: () => void;
  onToggle: (i: number | "all", on: boolean) => void;
}) {
  const reqs = it.preview?.requests ?? [];
  return (
    <Panel
      icon={<FileJson size={15} className="muted" />}
      title={
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate">{it.preview?.title || it.label}</span>
          <Chip>{it.preview?.kind || kindLabel[it.kind]}</Chip>
        </span>
      }
      right={
        <>
          {it.loading && <Loader2 size={14} className="muted animate-spin" />}
          {it.preview && (
            <span className="muted text-[12.5px]">
              {reqs.length - it.skip.size} of {reqs.length}
            </span>
          )}
          <Button size="sm" kind="ghost" aria-label={`Remove ${it.label}`} icon={<Trash2 size={13} />} onClick={onRemove} />
        </>
      }
    >
      {it.error && (
        <div className="p-4">
          <ErrorBox error={it.error} />
        </div>
      )}
      {it.loading && !it.preview && <div className="muted p-4 text-[13px]">Reading…</div>}
      {it.preview && (
        <div style={{ maxHeight: 340, overflow: "auto" }}>
          <table className="table">
            <thead>
              <tr>
                <th style={{ width: 36 }}>
                  <input
                    type="checkbox"
                    aria-label="Select all requests"
                    checked={it.skip.size === 0}
                    onChange={(e) => onToggle("all", e.target.checked)}
                  />
                </th>
                <th>Request</th>
                <th>Path</th>
                <th>Folder</th>
                <th>Access</th>
              </tr>
            </thead>
            <tbody>
              {reqs.map((r, i) => (
                <tr key={`${r.name}${i}`}>
                  <td>
                    <input
                      type="checkbox"
                      aria-label={r.name}
                      checked={!it.skip.has(i)}
                      onChange={(e) => onToggle(i, e.target.checked)}
                    />
                  </td>
                  <td>
                    <span className="flex items-center gap-2">
                      <MethodChip method={r.method} />
                      <code className="truncate">
                        {name}.{r.name}()
                      </code>
                    </span>
                  </td>
                  <td className="mono muted truncate text-[12px]" style={{ maxWidth: 280 }}>
                    {r.url.replace("{{baseUrl}}", "")}
                  </td>
                  <td className="muted">{r.folder || "–"}</td>
                  <td title={r.accessReason}>
                    <AccessChip access={r.access} flag={r.accessFlag} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Panel>
  );
}

/** Manual: a name, a base URL and an auth type. Requests are added after. */
function Manual({ taken }: { taken: string[] }) {
  const [title, setTitle] = useState("");
  const [name, setName] = useState("");
  const [touched, setTouched] = useState(false);
  const [base, setBase] = useState("");
  const [authType, setAuthType] = useState("none");
  const [keyName, setKeyName] = useState("X-Api-Key");
  const [secret, setSecret] = useState("");
  const nameErr = appNameError(name, taken);
  const create = useAction(
    () => {
      const auth =
        authType === "bearer"
          ? { type: "bearer", token: "{{token}}" }
          : authType === "apikey"
            ? { type: "apikey", key: keyName, in: "header", value: "{{apiKey}}" }
            : authType === "basic"
              ? { type: "basic", username: "{{username}}", password: "{{password}}" }
              : { type: "none" };
      return post<API>("/api/apis", { name, title, baseUrl: base, auth, secret });
    },
    { onSuccess: (a) => navigate(`/apps/${a.id}?new=1`) },
  );
  return (
    <Panel
      title="Start empty"
      footer={
        <>
          <Button kind="ghost" onClick={() => navigate("/apps")}>
            Cancel
          </Button>
          <Button
            kind="ink"
            disabled={!name || !!nameErr}
            loading={create.isPending}
            onClick={() => create.mutate(undefined)}
          >
            Create application
          </Button>
        </>
      }
    >
      <div className="panel-body grid gap-4 sm:grid-cols-2">
        <Field label="Display name" hint="What people see in mcpDeck.">
          <input
            className="input"
            placeholder="Acme CRM"
            value={title}
            onChange={(e) => {
              setTitle(e.target.value);
              if (!touched) setName(appName(e.target.value));
            }}
          />
        </Field>
        <Field
          label="Name in tool code"
          error={nameErr}
          hint={
            <>
              Tools call it as <code>{name || "acmeCrm"}.getAccount()</code>
            </>
          }
        >
          <input
            className="input mono"
            placeholder="acmeCrm"
            aria-invalid={!!nameErr}
            value={name}
            onChange={(e) => {
              setTouched(true);
              setName(e.target.value.replace(/[^A-Za-z0-9]/g, ""));
            }}
          />
        </Field>
        <Field label="Base URL" hint="Stored as {{baseUrl}} in the Default environment.">
          <input
            className="input mono"
            placeholder="https://api.acme.example/v1"
            value={base}
            onChange={(e) => setBase(e.target.value)}
          />
        </Field>
        <Field label="Authentication">
          <select className="select" value={authType} onChange={(e) => setAuthType(e.target.value)}>
            <option value="none">None</option>
            <option value="bearer">Bearer token</option>
            <option value="apikey">API key in a header</option>
            <option value="basic">Username and password</option>
          </select>
        </Field>
        {authType === "apikey" && (
          <Field label="Header name">
            <input className="input mono" value={keyName} onChange={(e) => setKeyName(e.target.value)} />
          </Field>
        )}
        {authType !== "none" && (
          <Field
            label={authType === "basic" ? "Password" : "Key or token"}
            hint="Encrypted on this computer. It can't be shown again, only replaced."
          >
            <input
              className="input mono"
              type="password"
              autoComplete="off"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
            />
          </Field>
        )}
      </div>
      {create.error && (
        <div className="px-4 pb-4">
          <ErrorBox error={create.error} />
        </div>
      )}
    </Panel>
  );
}
