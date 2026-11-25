import { Compass, Database, FolderOpen, Palette, ShieldCheck, SlidersHorizontal, Timer } from "lucide-react";
import { useEffect, useState } from "react";
import { useTheme } from "../components/Sidebar";
import { Button, Chip, ErrorBox, Loading, PageHead, Panel, SaveBar, Seg, Toggle, useToast } from "../components/ui";
import { post, put } from "../lib/api";
import { useAction, useSettings } from "../lib/hooks";

export const SECURITY_PROTECTS = [
  "Agents connected over MCP never receive your API keys. Keys are used only inside mcpDeck's proxy, when a request is sent.",
  "Keys are encrypted with AES-256-GCM. The encryption key lives in your OS keychain, so the database file on its own decrypts nothing.",
  "Agents with read access can't run tools that write, and their write requests are refused at the proxy.",
  "The server only listens on 127.0.0.1 and refuses requests from web pages on other sites.",
];
export const SECURITY_LIMITS = [
  "A program running as you on this computer, including an agent with a shell, can ask your keychain for the key or call the API directly. mcpDeck can't stop that.",
  "Agent tokens are stored in plain text in each client's config, as MCP clients require. Revoke a token if it leaks.",
];

const sections = [
  { id: "appearance", label: "Appearance", icon: Palette },
  { id: "general", label: "General", icon: SlidersHorizontal },
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

export function Settings() {
  const q = useSettings();
  const toast = useToast();
  const { theme, set } = useTheme();
  const [vals, setVals] = useState<Record<string, string>>({});
  useEffect(() => {
    if (q.data) setVals(q.data.values);
  }, [q.data]);
  const save = useAction(() => put("/api/settings", vals), { onSuccess: () => toast("Settings saved") });
  const guide = useAction((hide: boolean) => put("/api/settings", { hideOnboarding: hide ? "true" : "false" }), {
    onSuccess: (_, hide) => toast(hide ? "Setup guide hidden" : "Setup guide shown in the sidebar"),
  });
  const reveal = useAction(() => post("/api/reveal-data"));
  if (q.isLoading) return <Loading />;
  if (q.error)
    return (
      <div className="page">
        <ErrorBox error={q.error} />
      </div>
    );
  const s = q.data!;
  const keys = ["toolTimeoutSeconds", "retentionBodiesDays", "retentionCallsDays"];
  const dirty = keys.some((k) => (vals[k] ?? "") !== (s.values[k] ?? ""));
  const num = (k: string) => (
    <input
      className="input"
      style={{ maxWidth: 160 }}
      type="number"
      min={1}
      value={vals[k] ?? ""}
      onChange={(e) => setVals({ ...vals, [k]: e.target.value })}
    />
  );
  return (
    <div className="page" style={{ maxWidth: 1160 }}>
      <PageHead title="Settings" sub="How mcpDeck runs on this computer." />
      <div className="grid items-start gap-8 md:grid-cols-[200px_minmax(0,1fr)]">
        <nav className="sticky top-4 hidden flex-col gap-px md:flex" aria-label="Settings sections">
          {sections.map(({ id, label, icon: I }) => (
            <a key={id} href={`#${id}`} className="nav-item" style={{ height: 34 }}>
              <I size={16} strokeWidth={1.75} /> {label}
            </a>
          ))}
        </nav>
        <div className="flex min-w-0 flex-col gap-6">
          <Panel title="Appearance" id="appearance">
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
          </Panel>
          <Panel title="General" id="general">
            <Row
              title="Address"
              sub="Kept the same so client configs stay valid. Change it with mcpdeck --port, then reconnect agents."
            >
              <input className="input mono" value={`127.0.0.1:${s.port}`} readOnly aria-label="Address" />
            </Row>
            <Row title="Tool timeout" sub="Seconds a tool may run before it's stopped.">
              {num("toolTimeoutSeconds")}
            </Row>
            <Row title="Version">
              <span className="mono">{s.version}</span>
            </Row>
          </Panel>
          <Panel title="Call history" id="calls">
            <Row title="Keep inputs and outputs" sub="Days. The full bodies of calls, used for debugging.">
              {num("retentionBodiesDays")}
            </Row>
            <Row title="Keep call records" sub="Days. Who called what, when, how long it took, its size and tokens.">
              {num("retentionCallsDays")}
            </Row>
          </Panel>
          <Panel title="Getting started" id="guide">
            <Row title="Setup guide" sub="The card at the bottom of the sidebar that shows your next step.">
              <Toggle
                label="Show the setup guide"
                checked={s.values.hideOnboarding !== "true"}
                onChange={(on) => guide.mutate(!on)}
              />
            </Row>
          </Panel>
          <Panel title="Security" id="security">
            <Row title="Key storage" sub="Where the key that encrypts your credentials is kept.">
              <Chip tone={s.keychain === "os keychain" ? "ok" : "warn"}>{s.keychain}</Chip>
              {s.keychain !== "os keychain" && (
                <div className="banner warn mt-3">
                  No OS keychain was available, so the encryption key is in a file readable only by you:{" "}
                  <code>{s.keyFile}</code>. Anyone who can read that file and the database can decrypt your keys.
                </div>
              )}
            </Row>
            <div className="grid gap-6 p-5 sm:grid-cols-2">
              <div>
                <div className="mb-2 font-semibold">What mcpDeck protects</div>
                <ul className="text-2 m-0 pl-4 text-[13px]">
                  {SECURITY_PROTECTS.map((t) => (
                    <li key={t} className="mb-1.5">
                      {t}
                    </li>
                  ))}
                </ul>
              </div>
              <div>
                <div className="mb-2 font-semibold">What it can't</div>
                <ul className="text-2 m-0 pl-4 text-[13px]">
                  {SECURITY_LIMITS.map((t) => (
                    <li key={t} className="mb-1.5">
                      {t}
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          </Panel>
          <Panel title="Data" id="data">
            <Row title="Data folder" sub="Everything mcpDeck stores. Delete the folder to reset mcpDeck completely.">
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
          <ErrorBox error={save.error} />
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
