import { FileJson, Globe, KeyRound, Plus, Terminal, Upload } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { AccessChip, Button, Chip, ErrorBox, Field, MethodChip, PageHead, Panel, useToast } from "../components/ui";
import { post } from "../lib/api";
import { useAction, useAPI } from "../lib/hooks";
import { Link, navigate } from "../lib/router";
import type { API, ImportPreview, ImportReport } from "../lib/types";

type Source = "page" | "openapi" | "postman" | "curl" | "empty";
const sources: { id: Source; label: string; sub: string; icon: ReactNode }[] = [
  { id: "page", label: "Swagger or Redoc page", sub: "The docs page URL", icon: <Globe size={15} /> },
  {
    id: "openapi",
    label: "OpenAPI file or URL",
    sub: "3.x or Swagger 2.0, JSON or YAML",
    icon: <FileJson size={15} />,
  },
  { id: "postman", label: "Postman collection", sub: "v2.1, with an environment", icon: <Upload size={15} /> },
  { id: "curl", label: "curl", sub: "One or more commands", icon: <Terminal size={15} /> },
  { id: "empty", label: "Start empty", sub: "Name, base URL, auth", icon: <Plus size={15} /> },
];

function readFile(f: File): Promise<string> {
  return f.text();
}

export function AddAPI({ apiId }: { apiId?: string }) {
  const target = useAPI(apiId ?? "");
  const [src, setSrc] = useState<Source>("page");
  const [url, setUrl] = useState("");
  const [text, setText] = useState("");
  const [env, setEnv] = useState("");
  const [name, setName] = useState("");
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [skip, setSkip] = useState<Set<number>>(new Set());
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  // empty
  const [title, setTitle] = useState("");
  const [base, setBase] = useState("");
  const [authType, setAuthType] = useState("none");
  const [keyName, setKeyName] = useState("X-Api-Key");
  const [secret, setSecret] = useState("");

  const body = () => ({
    url: src === "page" || (src === "openapi" && url) ? url : "",
    text: src === "page" ? "" : text,
    environment: src === "postman" ? env : "",
  });
  const doPreview = async () => {
    setErr(null);
    setBusy(true);
    try {
      const p = await post<ImportPreview>("/api/import/preview", body());
      setPreview(p);
      setSkip(new Set());
      if (!name && p.suggestName) setName(p.suggestName);
    } catch (e) {
      setErr(e);
      setPreview(null);
    } finally {
      setBusy(false);
    }
  };
  // curl previews update as you paste (UI-14).
  useEffect(() => {
    if (src !== "curl" || !text.trim().startsWith("curl")) return;
    const t = setTimeout(doPreview, 350);
    return () => clearTimeout(t);
  }, [text, src]);

  const doImport = useAction(
    () => post<ImportReport>("/api/import", { ...body(), apiId: apiId ?? "", name, skip: [...skip] }),
    {
      onSuccess: (r) => {
        toast(`${r.apiName}: ${r.added.length} added, ${r.updated.length} updated`);
        navigate(`/apis/${r.apiId}?imported=${r.added.length + r.updated.length}`);
      },
    },
  );
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
    { onSuccess: (a) => navigate(`/apis/${a.id}`) },
  );

  const reqs = preview?.requests ?? [];
  const secrets = (preview?.vars ?? []).filter((v) => v.secret);
  return (
    <div className="page">
      <PageHead
        crumbs={
          <>
            <Link to="/apis">APIs</Link> › {apiId ? <Link to={`/apis/${apiId}`}>{target.data?.name}</Link> : "Add"}
          </>
        }
        title={apiId ? `Import into ${target.data?.name ?? ""}` : "Add an API"}
        sub={
          apiId
            ? "Requests are added or updated by name. Nothing is deleted."
            : "Pick where the API's description comes from. You see every request before anything is saved."
        }
      />
      <div className="grid items-start gap-4" style={{ gridTemplateColumns: "230px minmax(0, 1fr)" }}>
        <Panel bodyClass="p-1.5">
          {sources
            .filter((s) => !(apiId && s.id === "empty"))
            .map((s) => (
              <button
                key={s.id}
                type="button"
                className="nav-item items-start"
                aria-current={src === s.id ? "page" : undefined}
                onClick={() => {
                  setSrc(s.id);
                  setPreview(null);
                  setErr(null);
                }}
              >
                <span className="mt-0.5">{s.icon}</span>
                <span>
                  <span className="block leading-tight">{s.label}</span>
                  <span className="muted block text-[11.5px]">{s.sub}</span>
                </span>
              </button>
            ))}
        </Panel>
        <div className="flex min-w-0 flex-col gap-4">
          {src === "empty" ? (
            <Panel title="Start empty" bodyClass="panel-body grid gap-3 sm:grid-cols-2">
              <Field
                label="API name"
                hint={
                  <>
                    Tools call it as <code>{name || "crm"}.getAccount()</code>
                  </>
                }
              >
                <input
                  className="input mono"
                  placeholder="crm"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </Field>
              <Field label="Title">
                <input
                  className="input"
                  placeholder="Acme CRM"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                />
              </Field>
              <Field label="Base URL" hint="Stored as {{baseUrl}} in the Default environment">
                <input
                  className="input mono"
                  placeholder="https://api.acme.example/v1"
                  value={base}
                  onChange={(e) => setBase(e.target.value)}
                />
              </Field>
              <Field label="Auth">
                <select className="select" value={authType} onChange={(e) => setAuthType(e.target.value)}>
                  <option value="none">No auth</option>
                  <option value="bearer">Bearer token</option>
                  <option value="apikey">API key header</option>
                  <option value="basic">Basic (username and password)</option>
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
                  hint="Encrypted on this machine. It can't be shown again."
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
              <div className="sm:col-span-2">
                <ErrorBox error={create.error} />
                <Button kind="ink" disabled={!name || create.isPending} onClick={() => create.mutate(undefined)}>
                  Create API
                </Button>
              </div>
            </Panel>
          ) : (
            <Panel title={sources.find((s) => s.id === src)?.label} bodyClass="panel-body flex flex-col gap-3">
              {(src === "page" || src === "openapi") && (
                <Field
                  label={src === "page" ? "Docs page URL" : "Spec URL"}
                  hint={src === "openapi" ? "Or paste or open a file below" : "mcpDeck finds the spec the page loads"}
                >
                  <div className="flex gap-2">
                    <input
                      className="input mono"
                      placeholder={
                        src === "page"
                          ? "https://petstore3.swagger.io/"
                          : "https://petstore3.swagger.io/api/v3/openapi.json"
                      }
                      value={url}
                      onChange={(e) => setUrl(e.target.value)}
                      onKeyDown={(e) => e.key === "Enter" && doPreview()}
                    />
                    <Button kind="blue" disabled={(!url && !text) || busy} onClick={doPreview}>
                      {busy ? "Reading…" : "Preview"}
                    </Button>
                  </div>
                </Field>
              )}
              {src !== "page" && (
                <Field
                  label={
                    src === "curl" ? "curl commands" : src === "postman" ? "Collection JSON" : "Spec (JSON or YAML)"
                  }
                >
                  <textarea
                    className="textarea"
                    rows={src === "curl" ? 6 : 8}
                    placeholder={
                      src === "curl"
                        ? "curl -H 'Authorization: Bearer …' https://api.acme.example/v1/invoices?status=overdue"
                        : '{ "openapi": "3.0.3", … }'
                    }
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                  />
                  <input
                    type="file"
                    accept=".json,.yaml,.yml,.txt,.sh"
                    aria-label="Open a file"
                    className="text-[12.5px]"
                    onChange={async (e) => e.target.files?.[0] && setText(await readFile(e.target.files[0]))}
                  />
                </Field>
              )}
              {src === "postman" && (
                <Field label="Environment (optional)" hint="Secret values become encrypted credentials.">
                  <input
                    type="file"
                    accept=".json"
                    aria-label="Open a Postman environment"
                    className="text-[12.5px]"
                    onChange={async (e) => e.target.files?.[0] && setEnv(await readFile(e.target.files[0]))}
                  />
                </Field>
              )}
              {src !== "page" && src !== "curl" && (
                <div>
                  <Button kind="blue" disabled={(!text && !url) || busy} onClick={doPreview}>
                    {busy ? "Reading…" : "Preview"}
                  </Button>
                </div>
              )}
              <ErrorBox error={err} />
            </Panel>
          )}
          {preview && src !== "empty" && (
            <Panel
              title={
                <>
                  {reqs.length - skip.size} of {reqs.length} requests
                  {preview.title && <span className="muted font-normal"> · {preview.title}</span>}
                </>
              }
              right={
                !apiId && (
                  <label className="flex items-center gap-2 text-[12.5px]">
                    API name
                    <input
                      className="input mono"
                      style={{ width: 140, height: 26 }}
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                    />
                  </label>
                )
              }
            >
              <div style={{ maxHeight: 380, overflow: "auto" }}>
                <table className="table">
                  <thead>
                    <tr>
                      <th style={{ width: 30 }}>
                        <input
                          type="checkbox"
                          aria-label="All"
                          checked={skip.size === 0}
                          onChange={(e) => setSkip(e.target.checked ? new Set() : new Set(reqs.map((_, i) => i)))}
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
                      <tr key={i}>
                        <td>
                          <input
                            type="checkbox"
                            aria-label={r.name}
                            checked={!skip.has(i)}
                            onChange={() => {
                              const s = new Set(skip);
                              s.has(i) ? s.delete(i) : s.add(i);
                              setSkip(s);
                            }}
                          />
                        </td>
                        <td>
                          <span className="flex items-center gap-2">
                            <MethodChip method={r.method} />
                            <code>
                              {name || apiName(target.data?.name)}.{r.name}()
                            </code>
                          </span>
                        </td>
                        <td className="mono muted truncate text-[12px]" style={{ maxWidth: 280 }}>
                          {r.url.replace("{{baseUrl}}", "")}
                        </td>
                        <td className="muted">{r.folder}</td>
                        <td title={r.accessReason}>
                          <AccessChip access={r.access} flag={r.accessFlag} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {(secrets.length > 0 || (preview.warnings ?? []).length > 0) && (
                <div className="flex flex-col gap-2 p-3.5" style={{ borderTop: "1px solid var(--line)" }}>
                  {secrets.length > 0 && (
                    <div className="banner info">
                      <KeyRound size={15} className="mt-0.5 flex-none" />
                      <span>
                        Moving into an encrypted credential, never onto a request:{" "}
                        {secrets.map((s) => (
                          <Chip key={s.name + s.env}>
                            {s.name} {s.preview && <span className="mono">{s.preview}</span>}
                          </Chip>
                        ))}
                      </span>
                    </div>
                  )}
                  {(preview.warnings ?? []).map((w) => (
                    <div key={w} className="banner warn">
                      {w}
                    </div>
                  ))}
                </div>
              )}
              <div className="flex items-center gap-3 p-3.5" style={{ borderTop: "1px solid var(--line)" }}>
                <Button
                  kind="ink"
                  disabled={doImport.isPending || reqs.length === skip.size || (!apiId && !name)}
                  onClick={() => doImport.mutate(undefined)}
                >
                  {apiId ? "Import" : "Add API"}
                </Button>
                <ErrorBox error={doImport.error} />
              </div>
            </Panel>
          )}
        </div>
      </div>
    </div>
  );
}

function apiName(n?: string) {
  return n ?? "api";
}
