// Web apps without an API: browser sign-in (AUTH-5), the files folder (HTTP-3), confirm each call (CONF-1), toolkit
// files (EXP-1) and building with an agent (CON-1).
import { useQuery } from "@tanstack/react-query";
import { Download, FolderOpen, LogIn, ShieldCheck, Upload, Wand2 } from "lucide-react";
import { useRef, useState } from "react";
import { ApiError, get, post, put } from "../lib/api";
import { useAction } from "../lib/hooks";
import { navigate } from "../lib/router";
import type { Auth } from "../lib/types";
import { Button, CopyField, ErrorBox, Field, Panel, Select, useToast } from "./ui";

/** The fields of the Browser sign-in auth type, with Sign in now. */
export function BrowserFields({ apiId, auth, set }: { apiId: string; auth: Auth; set: (p: Partial<Auth>) => void }) {
  const toast = useToast();
  const signIn = useAction(() => post(`/api/apis/${apiId}/signin`), { onSuccess: () => toast("Signed in") });
  return (
    <>
      <Field
        label="Sign-in page"
        hint="The app's start page. mcpDeck opens it in Edge or Chrome when a call needs you to sign in."
      >
        <input
          className="input mono"
          aria-label="Sign-in page"
          value={auth.startUrl ?? ""}
          placeholder="https://expenses.corp.example/OA_HTML/claims"
          onChange={(e) => set({ startUrl: e.target.value })}
        />
      </Field>
      <Field
        label="Signed in when the address contains"
        hint="Optional. By default: the start page, once it isn't a sign-in page."
      >
        <input
          className="input mono"
          aria-label="Signed in when the address contains"
          value={auth.signedIn ?? ""}
          placeholder="/OA_HTML/"
          onChange={(e) => set({ signedIn: e.target.value })}
        />
      </Field>
      <div className="flex items-center gap-3">
        <Button icon={<LogIn size={14} />} loading={signIn.isPending} onClick={() => signIn.mutate(undefined)}>
          Sign in now
        </Button>
        <span className="muted text-[12px]">
          Your password and MFA stay in the browser. mcpDeck keeps only this app's session cookies, never shown to
          agents.
        </span>
      </div>
      <ErrorBox error={signIn.error} />
    </>
  );
}

type FileInfo = { name: string; size: number; modified: string };

/** Settings › Files folder: where tools may upload from. */
export function FilesPanel({ apiId, dir }: { apiId: string; dir?: string }) {
  const [v, setV] = useState(dir ?? "");
  const toast = useToast();
  const files = useQuery({
    queryKey: ["files", apiId, dir],
    queryFn: () => get<FileInfo[]>(`/api/apis/${apiId}/files`),
    enabled: !!dir,
    retry: false,
  });
  const save = useAction((d: string) => put(`/api/apis/${apiId}/files-folder`, { dir: d }), {
    onSuccess: (_, d) => toast(d ? "Files folder saved" : "Files folder off"),
  });
  return (
    <Panel
      title="Files folder"
      icon={<FolderOpen size={15} className="muted" />}
      intro="Tools can list this folder and upload files from it, such as receipts. Agents see file names only, never the path or contents."
    >
      <div className="settings-row">
        <div className="what">
          <strong>Folder</strong>
          <span>The full path. Only files directly in it; no links out of it; 20 MB each.</span>
        </div>
        <div className="flex gap-2">
          <input
            className="input mono"
            aria-label="Files folder"
            value={v}
            placeholder="C:\Users\you\Receipts"
            onChange={(e) => setV(e.target.value)}
          />
          {dir && (
            <Button
              kind="ghost"
              onClick={() => {
                setV("");
                save.mutate("");
              }}
            >
              Turn off
            </Button>
          )}
          <Button kind="ink" disabled={v === (dir ?? "")} loading={save.isPending} onClick={() => save.mutate(v)}>
            Save
          </Button>
        </div>
      </div>
      <ErrorBox error={save.error} />
      {dir && (
        <div className="p-3">
          {files.data?.length === 0 && <div className="muted text-[12px]">The folder is empty.</div>}
          {(files.data ?? []).slice(0, 8).map((f) => (
            <div key={f.name} className="flex justify-between text-[12px]">
              <span className="mono">{f.name}</span>
              <span className="muted">
                {Math.max(1, Math.round(f.size / 1024))} KB · {new Date(f.modified).toLocaleDateString()}
              </span>
            </div>
          ))}
          <ErrorBox error={files.error} />
        </div>
      )}
    </Panel>
  );
}

/** Toolkit › Export: download the toolkit file (EXP-1). */
export function ExportButton({ toolkit }: { toolkit: string }) {
  const toast = useToast();
  const exp = useAction(() => get<unknown>(`/api/toolkits/${toolkit}/export`), {
    onSuccess: (b) => {
      const url = URL.createObjectURL(new Blob([JSON.stringify(b, null, 2)], { type: "application/json" }));
      const a = document.createElement("a");
      a.href = url;
      a.download = `${toolkit}.mcpdeck`;
      a.click();
      URL.revokeObjectURL(url);
      toast(`Saved ${toolkit}.mcpdeck: no secrets, sessions or files in it`);
    },
  });
  return (
    <Button icon={<Download size={14} />} loading={exp.isPending} onClick={() => exp.mutate(undefined)}>
      Export
    </Button>
  );
}

type ImportResult = { toolkit: string; message: string };

/** Toolkits › Import: add a shared toolkit file (EXP-1). */
export function ImportButton() {
  const ref = useRef<HTMLInputElement>(null);
  const toast = useToast();
  const [error, setError] = useState<unknown>(null);
  const imp = useAction((b: unknown) => post<ImportResult>("/api/toolkit-import", b), {
    onSuccess: (r) => {
      toast(r.message);
      navigate(`/toolkits/${r.toolkit}`);
    },
    onError: setError,
  });
  return (
    <>
      <input
        ref={ref}
        type="file"
        accept=".mcpdeck,application/json"
        className="hidden"
        aria-label="Toolkit file"
        onChange={async (e) => {
          const f = e.target.files?.[0];
          e.target.value = "";
          if (!f) return;
          try {
            imp.mutate(JSON.parse(await f.text()));
          } catch {
            setError(new ApiError(400, "bad_file", `${f.name} isn't a toolkit file`));
          }
        }}
      />
      <Button icon={<Upload size={14} />} loading={imp.isPending} onClick={() => ref.current?.click()}>
        Import
      </Button>
      {!!error && <ErrorBox error={error} />}
    </>
  );
}

type Confirmation = {
  id: string;
  title: string;
  fields?: Record<string, unknown>;
  agent: string;
  toolkit?: string;
  at: string;
};

const useConfirmations = () =>
  useQuery({
    queryKey: ["confirmations"],
    queryFn: () => get<Confirmation[]>("/api/confirmations"),
    refetchInterval: 2000,
  });

function ConfirmCard({ c }: { c: Confirmation }) {
  const toast = useToast();
  const decide = useAction((ok: boolean) => post(`/api/confirmations/${c.id}`, { confirm: ok }), {
    onSuccess: (_, ok) => toast(ok ? `Confirmed: ${c.title}` : `Declined: ${c.title}`),
  });
  return (
    <Panel
      title={c.title}
      icon={<ShieldCheck size={15} className="muted" />}
      intro={`${c.agent}${c.toolkit ? ` · ${c.toolkit}` : ""} waits for you. Nothing is sent until you confirm.`}
      footer={
        <>
          <Button
            kind="ghost"
            loading={decide.isPending && decide.variables === false}
            onClick={() => decide.mutate(false)}
          >
            Decline
          </Button>
          <Button
            kind="ink"
            loading={decide.isPending && decide.variables === true}
            onClick={() => decide.mutate(true)}
          >
            Confirm
          </Button>
        </>
      }
    >
      <table className="table">
        <tbody>
          {Object.entries(c.fields ?? {}).map(([k, v]) => (
            <tr key={k}>
              <td className="muted" style={{ width: 160 }}>
                {k}
              </td>
              <td className="mono">{typeof v === "string" ? v : JSON.stringify(v)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <ErrorBox error={decide.error} />
    </Panel>
  );
}

/** Home: writes waiting for the person (CONF-1). */
export function ConfirmBanner() {
  const q = useConfirmations();
  if (!q.data?.length) return null;
  return (
    <div className="flex flex-col gap-3">
      {q.data.map((c) => (
        <ConfirmCard key={c.id} c={c} />
      ))}
    </div>
  );
}

/** /confirm/:id, which mcpDeck opens when a write waits. */
export function ConfirmPage({ id }: { id: string }) {
  const q = useConfirmations();
  const c = q.data?.find((x) => x.id === id);
  return (
    <div className="page" style={{ maxWidth: 720 }}>
      {c ? (
        <ConfirmCard c={c} />
      ) : (
        <Panel title="Nothing to confirm">
          <p className="muted p-4 m-0">
            {q.isLoading ? "Loading…" : "It was confirmed, declined or timed out. Anything else waiting is on Home."}
          </p>
        </Panel>
      )}
    </div>
  );
}

type BuilderPlan = {
  plan: { link?: string; command?: string; snippet?: string; snippetPath?: string; firstPrompt: string };
  written: string[];
};

const clients = [
  { value: "vscode", label: "VS Code (GitHub Copilot)" },
  { value: "claude-code", label: "Claude Code" },
  { value: "cursor", label: "Cursor" },
  { value: "claude-desktop", label: "Claude Desktop" },
];

/** Home: add mcpDeck's admin MCP server to an agent, in one click (CON-1). */
export function BuildWithAgent() {
  const [client, setClient] = useState("vscode");
  const toast = useToast();
  const plan = useQuery({
    queryKey: ["builder-plan", client],
    queryFn: () => post<BuilderPlan>("/api/connect-builder", { client }),
  });
  const write = useAction(() => post<BuilderPlan>("/api/connect-builder", { client, write: true }), {
    onSuccess: (r) =>
      toast(
        r.written.length ? `Added to ${r.written.join(", ")}` : "Nothing to write for this client; run the command",
      ),
  });
  const p = plan.data?.plan;
  return (
    <Panel
      title="Build tools with your agent"
      icon={<Wand2 size={15} className="muted" />}
      intro="Add mcpDeck to your agent once. Then describe what to automate, for example a web app you sign in to; it records the task with you, builds the tools and asks you to approve them."
      footer={
        <>
          {client !== "claude-code" && (
            <Button kind="ghost" loading={write.isPending} onClick={() => write.mutate(undefined)}>
              Write config file
            </Button>
          )}
          {p?.link ? (
            <a className="btn btn-ink" href={p.link}>
              Add to {clients.find((c) => c.value === client)?.label.split(" (")[0]}
            </a>
          ) : null}
        </>
      }
    >
      <div className="flex flex-col gap-3 p-4">
        <Field label="Agent">
          <Select label="Agent" value={client} onChange={setClient} options={clients} />
        </Field>
        {p?.command && <CopyField label="Or run" value={p.command} />}
        {p && <CopyField label="Then ask" value={p.firstPrompt} />}
        <ErrorBox error={plan.error ?? write.error} />
      </div>
    </Panel>
  );
}
