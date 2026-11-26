import {
  Check,
  Compass,
  Database,
  FolderOpen,
  Globe,
  Palette,
  ShieldCheck,
  SlidersHorizontal,
  Timer,
} from "lucide-react";
import { useEffect, useState } from "react";
import { ACCENTS, useTheme } from "../components/Sidebar";
import {
  Button,
  Chip,
  ErrorBox,
  Loading,
  Panel,
  SaveBar,
  SecretField,
  Seg,
  secretChange,
  Toggle,
  useToast,
} from "../components/ui";
import { post, put } from "../lib/api";
import { useAction, useInfo, useSettings } from "../lib/hooks";
import { Link } from "../lib/router";

export const SECURITY_PROTECTS = [
  "Agents connected over MCP never receive your API keys. Keys are used only inside mcpDeck's proxy, when a request is sent.",
  "Keys are encrypted with AES-256-GCM. The encryption key lives in your OS keychain, so the database file on its own decrypts nothing.",
  "Agents with read access can't run tools that write, and their write requests are refused at the proxy.",
  "The server checks the Host and Origin of every request and refuses web pages on other sites; the UI needs its launch cookie.",
];
export const SECURITY_LIMITS = [
  "A program running as you on this computer, including an agent with a shell, can ask your keychain for the key or call the API directly. mcpDeck can't stop that.",
  "Agent tokens are stored in plain text in each client's config, as MCP clients require. Revoke a token if it leaks.",
];

// Editable settings saved by the Save bar.
const keys = [
  "toolTimeoutSeconds",
  "retentionBodiesDays",
  "retentionCallsDays",
  "requestTimeoutSeconds",
  "requestTimeoutMaxSeconds",
  "proxyMode",
  "proxyHttp",
  "proxyHttps",
  "noProxy",
  "proxyUser",
];

const sections = [
  { id: "appearance", label: "Appearance", icon: Palette },
  { id: "general", label: "General", icon: SlidersHorizontal },
  { id: "network", label: "Requests and proxy", icon: Globe },
  { id: "calls", label: "Call history", icon: Timer },
  { id: "guide", label: "Getting started", icon: Compass },
  { id: "security", label: "Security", icon: ShieldCheck },
  { id: "data", label: "Data", icon: Database },
];

function Row({ title, sub, children }: { title: string; sub?: string; children: React.ReactNode }) {
  return (
    <div className="settings-row">
      <div className="what">
        <strong>{title}</strong>
        {sub && <span>{sub}</span>}
      </div>
      <div>{children}</div>
    </div>
  );
}

function SectionHead({ title, sub }: { title: string; sub: string }) {
  return (
    <div className="mb-5">
      <h2 className="text-[19px] font-semibold tracking-tight">{title}</h2>
      <p className="muted m-0 mt-1">{sub}</p>
    </div>
  );
}

export function Settings({ section = "general" }: { section?: string }) {
  const q = useSettings();
  const info = useInfo();
  const toast = useToast();
  const { theme, set, accent, setAccent } = useTheme();
  const [vals, setVals] = useState<Record<string, string>>({});
  const [pw, setPw] = useState<string | null | undefined>(undefined);
  useEffect(() => {
    if (q.data) setVals(q.data.values);
    setPw(undefined);
  }, [q.data]);
  // Only what changed is sent; the proxy password only when replaced or removed.
  const changes = () => {
    const out: Record<string, string> = {};
    for (const k of keys) if ((vals[k] ?? "") !== (q.data?.values[k] ?? "")) out[k] = vals[k] ?? "";
    const p = secretChange(pw);
    if (p !== undefined) out.proxyPassword = p;
    return out;
  };
  const save = useAction(() => put("/api/settings", changes()), { onSuccess: () => toast("Settings saved") });
  const guide = useAction((hide: boolean) => put("/api/settings", { hideOnboarding: hide ? "true" : "false" }), {
    onSuccess: (_, hide) => toast(hide ? "Setup guide hidden" : "Setup guide shown in the sidebar"),
  });
  const reveal = useAction(() => post("/api/reveal-data"));
  const current = sections.find((x) => x.id === section) ?? sections[1];
  const nav = (
    <nav className="split-nav" aria-label="Settings sections">
      <h1>Settings</h1>
      <div className="sub">How mcpDeck runs.</div>
      {sections.map(({ id, label, icon: I }) => (
        <Link
          key={id}
          to={`/settings/${id}`}
          className="nav-item"
          aria-current={current.id === id ? "page" : undefined}
          style={{ height: 34 }}
        >
          <I size={16} strokeWidth={1.75} /> {label}
        </Link>
      ))}
    </nav>
  );
  if (q.isLoading)
    return (
      <div className="split-page">
        {nav}
        <Loading />
      </div>
    );
  if (q.error)
    return (
      <div className="split-page">
        {nav}
        <div className="page">
          <ErrorBox error={q.error} />
        </div>
      </div>
    );
  const s = q.data!;
  const dirty = Object.keys(changes()).length > 0;
  const num = (k: string, unit: string, label?: string) => (
    <div className="flex items-center gap-2">
      <input
        className="input"
        style={{ maxWidth: 120 }}
        type="number"
        min={1}
        aria-label={label}
        value={vals[k] ?? ""}
        onChange={(e) => setVals({ ...vals, [k]: e.target.value })}
      />
      <span className="muted">{unit}</span>
    </div>
  );
  const text = (k: string, label: string, placeholder: string) => (
    <input
      className="input mono"
      aria-label={label}
      placeholder={placeholder}
      value={vals[k] ?? ""}
      onChange={(e) => setVals({ ...vals, [k]: e.target.value })}
    />
  );
  const address = info.data?.baseUrl ?? `http://127.0.0.1:${s.port}`;
  return (
    <div className="split-page">
      {nav}
      <div className="split-body">
        <div className="page">
          {current.id === "appearance" && (
            <>
              <SectionHead title="Appearance" sub="How mcpDeck looks in this browser." />
              <Panel>
                <Row title="Theme" sub="System follows your computer's light or dark setting.">
                  <Seg
                    label="Theme"
                    value={theme}
                    onChange={set}
                    options={[
                      { id: "system", label: "System" },
                      { id: "light", label: "Light" },
                      { id: "dark", label: "Dark" },
                    ]}
                  />
                </Row>
                <Row title="Accent colour" sub="Used for links, selection and the main highlights.">
                  <div className="flex flex-wrap gap-2.5" role="group" aria-label="Accent colour">
                    {ACCENTS.map((a) => (
                      <button
                        key={a.id}
                        type="button"
                        className="swatch"
                        style={{ background: a.color }}
                        aria-pressed={accent === a.id}
                        aria-label={a.label}
                        title={a.label}
                        onClick={() => setAccent(a.id)}
                      >
                        {accent === a.id && <Check size={14} />}
                      </button>
                    ))}
                  </div>
                </Row>
              </Panel>
              <div className="panel mt-6 p-5">
                <div className="mb-3 text-[12px] font-semibold uppercase tracking-wide muted">Preview</div>
                <div className="flex flex-wrap items-center gap-3">
                  <Button kind="ink">Main action</Button>
                  <Button kind="blue">Confirm</Button>
                  <Button>Secondary</Button>
                  <a href="#preview">A link looks like this</a>
                  <Chip tone="info">selected</Chip>
                  <Toggle label="Preview toggle" checked onChange={() => {}} />
                </div>
              </div>
            </>
          )}
          {current.id === "general" && (
            <>
              <SectionHead title="General" sub="Where mcpDeck serves, and how long tools may run." />
              <Panel>
                <Row
                  title="Address"
                  sub="Agents connect to this address. Keep it stable so client configs stay valid; if it changes, reconnect agents."
                >
                  <input className="input mono" value={address} readOnly aria-label="Address" />
                </Row>
                <Row title="Tool timeout" sub="How long a tool may run before it's stopped.">
                  {num("toolTimeoutSeconds", "seconds")}
                </Row>
                <Row title="Version">
                  <span className="mono">{s.version}</span>
                </Row>
              </Panel>
            </>
          )}
          {current.id === "network" && (
            <>
              <SectionHead
                title="Requests and proxy"
                sub="How long a request may take, and how mcpDeck reaches your APIs. Applies to Send, tools and agents."
              />
              <Panel>
                <Row
                  title="Default timeout"
                  sub="How long a request may wait for a response. A request can set its own on its Settings tab."
                >
                  {num("requestTimeoutSeconds", "seconds", "Default timeout")}
                </Row>
                <Row title="Maximum timeout" sub="No request waits longer than this, whatever it sets.">
                  {num("requestTimeoutMaxSeconds", "seconds", "Maximum timeout")}
                </Row>
              </Panel>
              <Panel className="mt-6">
                <Row
                  title="Proxy"
                  sub="System uses HTTP_PROXY, HTTPS_PROXY and NO_PROXY from the environment mcpDeck started in. Requests to localhost never use a proxy."
                >
                  <Seg
                    label="Proxy"
                    value={(vals.proxyMode || "system") as "system" | "none" | "manual"}
                    onChange={(m) => setVals({ ...vals, proxyMode: m })}
                    options={[
                      { id: "system", label: "System" },
                      { id: "none", label: "No proxy" },
                      { id: "manual", label: "Manual" },
                    ]}
                  />
                </Row>
                {vals.proxyMode === "manual" && (
                  <>
                    <Row title="HTTP proxy" sub="For http:// URLs, like proxy.corp.example:3128.">
                      {text("proxyHttp", "HTTP proxy", "host:port")}
                    </Row>
                    <Row title="HTTPS proxy" sub="For https:// URLs. Leave empty to use the HTTP proxy.">
                      {text("proxyHttps", "HTTPS proxy", "Same as HTTP")}
                    </Row>
                    <Row title="Bypass" sub="Comma-separated hosts, domains or CIDR ranges that connect directly.">
                      {text("noProxy", "Bypass", "internal.example, .corp, 10.0.0.0/8")}
                    </Row>
                    <Row title="Proxy username" sub="Only if the proxy asks you to sign in.">
                      {text("proxyUser", "Proxy username", "")}
                    </Row>
                    <Row title="Proxy password" sub="Stored encrypted, like API keys, and never shown again.">
                      <SecretField
                        label="Proxy password"
                        isSet={s.values.proxyPasswordSet === "true"}
                        value={pw}
                        onChange={setPw}
                      />
                    </Row>
                  </>
                )}
                <Row
                  title="Certificate checks"
                  sub="Set per application, on its Settings tab under TLS: trusted CA certificates, a client certificate, or verification off for a test server."
                >
                  <Link to="/apps">Applications</Link>
                </Row>
              </Panel>
            </>
          )}
          {current.id === "calls" && (
            <>
              <SectionHead title="Call history" sub="What mcpDeck keeps about each call, and for how long." />
              <Panel>
                <Row title="Keep inputs and outputs" sub="The full bodies of calls, used for debugging.">
                  {num("retentionBodiesDays", "days")}
                </Row>
                <Row title="Keep call records" sub="Who called what, when, how long it took, its size and tokens.">
                  {num("retentionCallsDays", "days")}
                </Row>
              </Panel>
            </>
          )}
          {current.id === "guide" && (
            <>
              <SectionHead title="Getting started" sub="Help shown while you set mcpDeck up." />
              <Panel>
                <Row title="Setup guide" sub="The card at the bottom of the sidebar that shows your next step.">
                  <Toggle
                    label="Show the setup guide"
                    checked={s.values.hideOnboarding !== "true"}
                    onChange={(on) => guide.mutate(!on)}
                  />
                </Row>
              </Panel>
            </>
          )}
          {current.id === "security" && (
            <>
              <SectionHead title="Security" sub="How credentials are kept, and what that does and doesn't protect." />
              <Panel>
                <Row title="Secrets" sub="API keys, passwords and tokens you enter in mcpDeck.">
                  <span className="secret-set">Encrypted (AES-256-GCM)</span>
                  <div className="muted mt-2 text-[12.5px]">
                    Stored in <code className="inline-code">{s.database}</code>. Agents never receive them, and they are
                    masked in the call history.
                  </div>
                </Row>
                <Row title="Key storage" sub="Where the key that encrypts your secrets is kept.">
                  <Chip tone={s.keychain === "os keychain" ? "ok" : "warn"}>
                    {s.keychain === "os keychain" ? "OS keychain" : "Key file"}
                  </Chip>
                  {s.keyItem && (
                    <div className="muted mt-2 text-[12.5px]">
                      Keychain entry <code className="inline-code">{s.keyItem}</code>
                    </div>
                  )}
                  {s.keychain !== "os keychain" && (
                    <div className="banner warn mt-3">
                      No OS keychain was available, so the encryption key is in a file readable only by you:{" "}
                      <code>{s.keyFile}</code>. Anyone who can read that file and the database can decrypt your keys.
                    </div>
                  )}
                </Row>
              </Panel>
              <div className="mt-6 grid gap-4 lg:grid-cols-2">
                <Panel title="What mcpDeck protects">
                  <ul className="text-2 m-0 py-4 pr-5 pl-9 text-[13px]">
                    {SECURITY_PROTECTS.map((t) => (
                      <li key={t} className="mb-2">
                        {t}
                      </li>
                    ))}
                  </ul>
                </Panel>
                <Panel title="What it can't">
                  <ul className="text-2 m-0 py-4 pr-5 pl-9 text-[13px]">
                    {SECURITY_LIMITS.map((t) => (
                      <li key={t} className="mb-2">
                        {t}
                      </li>
                    ))}
                  </ul>
                </Panel>
              </div>
            </>
          )}
          {current.id === "data" && (
            <>
              <SectionHead title="Data" sub="Where mcpDeck stores everything." />
              <Panel>
                <Row title="Data folder" sub="Delete the folder to reset mcpDeck completely.">
                  <div className="flex gap-2">
                    <input className="input mono" value={s.dataDir} readOnly aria-label="Data folder" />
                    <Button icon={<FolderOpen size={14} />} onClick={() => reveal.mutate(undefined)}>
                      Open
                    </Button>
                  </div>
                  <div className="mt-2">
                    <ErrorBox error={reveal.error} />
                  </div>
                </Row>
              </Panel>
            </>
          )}
          <div className="mt-4">
            <ErrorBox error={save.error} />
          </div>
          <SaveBar
            dirty={dirty}
            saving={save.isPending}
            onSave={() => save.mutate(undefined)}
            onDiscard={() => setVals(s.values)}
          />
        </div>
      </div>
    </div>
  );
}
