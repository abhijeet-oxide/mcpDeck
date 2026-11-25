import { FolderOpen, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";
import { useTheme } from "../components/Sidebar";
import { Button, Chip, ErrorBox, Field, Loading, PageHead, Panel, Seg, useToast } from "../components/ui";
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

export function Settings() {
  const q = useSettings();
  const toast = useToast();
  const { theme, set } = useTheme();
  const [vals, setVals] = useState<Record<string, string>>({});
  useEffect(() => {
    if (q.data) setVals(q.data.values);
  }, [q.data]);
  const save = useAction(() => put("/api/settings", vals), { onSuccess: () => toast("Settings saved") });
  const reveal = useAction(() => post("/api/reveal-data"));
  const demo = async () => {
    try {
      const r = await post<{ url: string }>("/api/demo");
      window.open(r.url, "_blank", "noopener");
    } catch (e) {
      toast((e as Error).message, true);
    }
  };
  if (q.isLoading) return <Loading />;
  const s = q.data!;
  return (
    <div className="page" style={{ maxWidth: 900 }}>
      <PageHead title="Settings" sub="How mcpDeck runs on this computer." />
      <div className="flex flex-col gap-4">
        <Panel title="General" bodyClass="panel-body grid gap-4 sm:grid-cols-2">
          <Field label="Theme">
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
          </Field>
          <Field
            label="Port"
            hint="Kept so client configs stay valid. Change with mcpdeck --port, then reconnect agents."
          >
            <input className="input mono" value={`127.0.0.1:${s.port}`} readOnly />
          </Field>
          <Field label="Tool timeout (seconds)">
            <input
              className="input"
              type="number"
              min={1}
              value={vals.toolTimeoutSeconds ?? ""}
              onChange={(e) => setVals({ ...vals, toolTimeoutSeconds: e.target.value })}
            />
          </Field>
          <Field label="Version">
            <input className="input mono" value={s.version} readOnly />
          </Field>
        </Panel>
        <Panel title="Calls" bodyClass="panel-body grid gap-4 sm:grid-cols-2">
          <Field label="Keep call bodies (days)" hint="Inputs and outputs">
            <input
              className="input"
              type="number"
              min={1}
              value={vals.retentionBodiesDays ?? ""}
              onChange={(e) => setVals({ ...vals, retentionBodiesDays: e.target.value })}
            />
          </Field>
          <Field label="Keep call records (days)" hint="Who, when, time, size, tokens">
            <input
              className="input"
              type="number"
              min={1}
              value={vals.retentionCallsDays ?? ""}
              onChange={(e) => setVals({ ...vals, retentionCallsDays: e.target.value })}
            />
          </Field>
          <div className="sm:col-span-2">
            <Button kind="ink" onClick={() => save.mutate(undefined)}>
              Save
            </Button>
            <ErrorBox error={save.error} />
          </div>
        </Panel>
        <Panel title="Security" bodyClass="panel-body" className="scroll-mt-4">
          <div id="security" className="mb-3 flex items-center gap-2">
            Key storage: <Chip tone={s.keychain === "os keychain" ? "ok" : "warn"}>{s.keychain}</Chip>
          </div>
          {s.keychain !== "os keychain" && (
            <div className="banner warn mb-3">
              No OS keychain was available, so the encryption key is in a file readable only by you:{" "}
              <code>{s.keyFile}</code>. Anyone who can read that file and the database can decrypt your keys.
            </div>
          )}
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <div className="mb-1 font-semibold">What mcpDeck protects</div>
              <ul className="text-2 m-0 pl-4 text-[13px]">
                {SECURITY_PROTECTS.map((t) => (
                  <li key={t} className="mb-1">
                    {t}
                  </li>
                ))}
              </ul>
            </div>
            <div>
              <div className="mb-1 font-semibold">What it doesn't</div>
              <ul className="text-2 m-0 pl-4 text-[13px]">
                {SECURITY_LIMITS.map((t) => (
                  <li key={t} className="mb-1">
                    {t}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </Panel>
        <Panel title="Data" bodyClass="panel-body flex flex-col gap-3">
          <Field label="Data folder" hint="Delete it to reset mcpDeck completely.">
            <div className="flex gap-2">
              <input className="input mono" value={s.dataDir} readOnly />
              <Button icon={<FolderOpen size={14} />} onClick={() => reveal.mutate(undefined)}>
                Reveal
              </Button>
            </div>
          </Field>
          <ErrorBox error={reveal.error} />
          {!s.demo && (
            <div className="flex items-center gap-3">
              <Button icon={<Sparkles size={14} />} onClick={demo}>
                Open the demo world
              </Button>
              <span className="muted text-[12.5px]">
                Mock APIs, toolkits and agents in their own throwaway data, in a new tab.
              </span>
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}
