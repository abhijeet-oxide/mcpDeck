import { useQueries, useQuery } from "@tanstack/react-query";
import {
  ArrowLeft,
  Bot,
  CheckCircle2,
  Code2,
  FlaskConical,
  Loader2,
  MousePointerClick,
  Play,
  Plus,
  RotateCcw,
  Save,
  Search,
  Send,
  Trash2,
} from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { useConnect } from "../App";
import { useRunner } from "../components/AskAgent";
import { Diff } from "../components/Diff";
import {
  AccessChip,
  Button,
  Chip,
  CopyButton,
  Empty,
  EmptyState,
  ErrorBox,
  Field,
  Loading,
  MethodChip,
  PageHead,
  Panel,
  StatusChip,
  Tabs,
  Toggle,
  useToast,
} from "../components/ui";
import { get, post } from "../lib/api";
import { bytes, dateTime, mod, num } from "../lib/format";
import { useAction, useAPIs, useTool } from "../lib/hooks";
import { applyTheme, monaco, setTypes } from "../lib/monaco";
import { Link, navigate, useLocation } from "../lib/router";
import type { APIDetail, RunResult, SaveResult, ToolTest, Version } from "../lib/types";

function template(from?: { api: string; req: string; params: string[] }) {
  if (!from) {
    return `// A tool has two parts.
// 1. \`tool\` is what agents see. Rename it
//    (lowercase_with_underscores) and say when to use it.
export const tool = {
  name: "new_tool",
  description: "What it returns, in one line.",
  useWhen: "When an agent should pick this tool",
  inputSchema: {
    type: "object",
    properties: { id: { type: "string" } },
    required: ["id"],
  },
};

// 2. The function calls your applications and returns
//    only what an agent needs. Type an application's name
//    and a dot (crm.) to see its requests, or click one
//    on the left to insert a call.
export default async function (input: Input) {
  return { id: input.id };
}
`;
  }
  const props = from.params.map((p) => `${p}: { type: "string" }`).join(", ");
  const args = from.params.map((p) => `${p}: input.${p}`).join(", ");
  return `export const tool = {
  name: "${from.req.replace(/([A-Z])/g, "_$1").toLowerCase()}",
  description: "Returns the fields of ${from.api}.${from.req} that the task needs.",
  inputSchema: {
    type: "object",
    properties: { ${props} },
    required: [${from.params.map((p) => `"${p}"`).join(", ")}],
  },
};

export default async function (input: Input) {
  const res = await ${from.api}.${from.req}(${args ? `{ ${args} }` : ""});
  // Return only what an agent needs; every field costs tokens on every call.
  return res;
}
`;
}

function sampleInput(schema: any): string {
  const props = schema?.properties ?? {};
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries<any>(props))
    out[k] = v?.examples?.[0] ?? v?.example ?? v?.default ?? (v?.type === "number" ? 0 : "");
  return JSON.stringify(out, null, 2);
}

function inputSchemaFrom(src: string): any {
  const m = src.match(/inputSchema:\s*(\{[\s\S]*?\n\s{2}\})/);
  if (!m) return null;
  try {
    return new Function(`return (${m[1]})`)();
  } catch {
    return null;
  }
}

export default function ToolEditor({ toolkit, tool }: { toolkit: string; tool: string }) {
  const isNew = !tool;
  const { search } = useLocation();
  const detail = useTool(tool, !isNew);
  const apis = useAPIs();
  const details = useQueries({
    queries: (apis.data ?? []).map((a) => ({
      queryKey: ["api", a.id],
      queryFn: () => get<APIDetail>(`/api/apis/${a.id}`),
    })),
  });
  const types = useQuery({
    queryKey: ["types", toolkit],
    queryFn: () => get<string>(`/api/toolkits/${toolkit}/types.d.ts`),
  });
  const [tab, setTab] = useState<"code" | "versions" | "fields">("code");
  const mode = search.get("mode") ?? (search.get("from") ? "code" : "");
  if (!isNew && detail.isLoading) return <Loading />;
  if (detail.error)
    return (
      <div className="page">
        <ErrorBox error={detail.error} />
      </div>
    );
  const versions = detail.data?.versions ?? [];
  const latest = versions[0];
  let initial = latest?.source;
  if (!initial) {
    const from = search.get("from");
    if (from) {
      const [api, req] = from.split(".");
      const r = details.flatMap((d) => d.data?.requests ?? []).find((x) => x.name === req);
      const params = r
        ? [...r.url.matchAll(/\{([A-Za-z0-9_]+)\}/g)].map((m) => m[1]).filter((p) => p !== "baseUrl")
        : [];
      if (!r && details.some((d) => d.isLoading)) return <Loading />;
      initial = template({ api, req, params });
    } else initial = template();
  }
  if (isNew && !mode) {
    return <NewTool toolkit={toolkit} apis={details.map((d) => d.data).filter(Boolean) as APIDetail[]} />;
  }
  return (
    <div className="page page-wide">
      <PageHead
        crumbs={
          <>
            <Link to="/toolkits">Toolkits</Link> / <Link to={`/toolkits/${toolkit}`}>{toolkit}</Link> /{" "}
            {isNew ? "New tool" : tool}
          </>
        }
        title={isNew ? "New tool" : tool}
        mono={!isNew}
        sub={
          latest ? (
            <span className="flex items-center gap-2">
              v{latest.n} <StatusChip status={latest.status} /> <AccessChip access={latest.access} /> ·{" "}
              {latest.createdByKind === "you" ? "saved by you" : `saved by ${latest.createdByName}`} ·{" "}
              {dateTime(latest.createdAt)}
            </span>
          ) : (
            "The definition agents see, and a function that calls your applications. Run it on the right, then save to make it live."
          )
        }
        actions={
          isNew ? (
            <Button
              kind="ghost"
              icon={<ArrowLeft size={14} />}
              onClick={() => navigate(`/toolkits/${toolkit}/tools/new`)}
            >
              Other ways to start
            </Button>
          ) : (
            latest?.status === "pending" && (
              <Button kind="blue" onClick={() => navigate(`/review/${latest.id}`)}>
                Review
              </Button>
            )
          )
        }
      />
      {!isNew && (
        <Tabs
          value={tab}
          onChange={setTab}
          tabs={[
            { id: "code", label: "Code" },
            { id: "versions", label: "Versions", n: versions.length },
            { id: "fields", label: "Output fields" },
          ]}
        />
      )}
      {tab === "code" && (
        <Workbench
          key={latest?.id ?? "new"}
          toolkit={toolkit}
          initial={initial}
          initialTests={latest?.tests ?? []}
          apis={details.map((d) => d.data).filter(Boolean) as APIDetail[]}
          types={types.data ?? ""}
          isNew={isNew}
        />
      )}
      {tab === "versions" && <Versions versions={versions} />}
      {tab === "fields" && <Fields fields={detail.data?.fields ?? []} latest={latest} />}
    </div>
  );
}

type Start = "request" | "agent" | "code";

/** NewTool asks how to start, so the editor never opens on code nobody asked for. */
function NewTool({ toolkit, apis }: { toolkit: string; apis: APIDetail[] }) {
  const reqs = apis.flatMap((a) => a.requests.map((r) => ({ a, r })));
  const [start, setStart] = useState<Start>(reqs.length ? "request" : "code");
  const [filter, setFilter] = useState("");
  const [what, setWhat] = useState("");
  const { builder } = useRunner(toolkit);
  const connect = useConnect();
  const prompt = `Hand this to the mcpdeck-${toolkit}-builder subagent: build a tool that returns ${what || "<what you need>"}.
It should search_tools first, then search_apis and describe_api, try calls with run_api, then test_tool and save_tool with at least one test.
Check: the reply says the tool is live, or which approval rule held it.`;
  const shown = reqs.filter(({ a, r }) =>
    `${a.name} ${a.title} ${r.name} ${r.title} ${r.url}`.toLowerCase().includes(filter.toLowerCase()),
  );
  const cards: { id: Start; icon: ReactNode; title: string; sub: string }[] = [
    {
      id: "request",
      icon: <Send size={17} />,
      title: "Start from a request",
      sub: "Pick one; the code that calls it is written for you.",
    },
    {
      id: "agent",
      icon: <Bot size={17} />,
      title: "Ask an agent to build it",
      sub: "Describe it; a connected agent explores, tests and saves.",
    },
    {
      id: "code",
      icon: <Code2 size={17} />,
      title: "Write it yourself",
      sub: "Open the editor with a short, commented starter.",
    },
  ];
  return (
    <div className="page" style={{ maxWidth: 980 }}>
      <PageHead
        crumbs={
          <>
            <Link to="/toolkits">Toolkits</Link> / <Link to={`/toolkits/${toolkit}`}>{toolkit}</Link> / New tool
          </>
        }
        title="New tool"
        sub={
          <>
            A tool is a short TypeScript function that agents connected to <code>{toolkit}</code> can call. It calls
            your applications and returns only the fields that are needed, so answers stay small and fast.
          </>
        }
      />
      <div className="mb-5 grid gap-3 md:grid-cols-3">
        {cards.map((c) => (
          <button
            key={c.id}
            type="button"
            className="choice"
            aria-pressed={start === c.id}
            onClick={() => setStart(c.id)}
          >
            <span className={`icon-tile ${start === c.id ? "" : "plain"}`}>{c.icon}</span>
            <span>
              <span className="block font-semibold">{c.title}</span>
              <span className="muted block text-[12.5px]">{c.sub}</span>
            </span>
          </button>
        ))}
      </div>
      {start === "request" && (
        <Panel
          title="Pick a request"
          right={
            <div className="relative">
              <Search size={13} className="muted absolute top-1/2 left-2.5 -translate-y-1/2" />
              <input
                className="input"
                style={{ height: 30, paddingLeft: 28, width: 240 }}
                placeholder="Filter requests"
                aria-label="Filter requests"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
              />
            </div>
          }
        >
          {reqs.length === 0 ? (
            <EmptyState
              icon={<Send size={20} />}
              title="No requests yet"
              actions={
                <Button kind="ink" onClick={() => navigate("/apps/new")}>
                  Create application
                </Button>
              }
            >
              Tools call requests in your applications. Add an application first, or write the tool yourself.
            </EmptyState>
          ) : (
            <div style={{ maxHeight: 460, overflow: "auto" }}>
              {shown.map(({ a, r }) => (
                <button
                  key={r.id}
                  type="button"
                  className="list-row w-full text-left"
                  style={{ background: "none", border: 0, borderBottom: "1px solid var(--line)", cursor: "pointer" }}
                  onClick={() =>
                    navigate(`/toolkits/${toolkit}/tools/new?from=${encodeURIComponent(`${a.name}.${r.name}`)}`)
                  }
                >
                  <MethodChip method={r.method} />
                  <span className="min-w-0 flex-1">
                    <code className="font-semibold">
                      {a.name}.{r.name}
                    </code>
                    <span className="muted block truncate text-[12.5px]">
                      {r.title || r.url.replace("{{baseUrl}}", "")}
                    </span>
                  </span>
                  <AccessChip access={r.access} flag={r.accessFlag} />
                  <span className="btn btn-sm">Use</span>
                </button>
              ))}
              {shown.length === 0 && <Empty title="No request matches" />}
            </div>
          )}
        </Panel>
      )}
      {start === "agent" && (
        <Panel title="Describe the tool" bodyClass="panel-body flex flex-col gap-4">
          <Field label="What should it return, and for what input?">
            <input
              className="input"
              placeholder="overdue invoices for one account, with number, due date and amount"
              value={what}
              onChange={(e) => setWhat(e.target.value)}
            />
          </Field>
          <div>
            <div className="mb-1.5 flex items-center gap-2 text-[12.5px] font-semibold">
              Prompt to paste into your agent
              <span className="ml-auto">
                <CopyButton text={prompt} label="Copy prompt" what="Prompt copied" />
              </span>
            </div>
            <pre className="code" style={{ whiteSpace: "pre-wrap" }}>
              {prompt}
            </pre>
          </div>
          {builder ? (
            <div className="banner ok items-center">
              <CheckCircle2 size={15} className="flex-none" /> Paste it into {builder.name}: it may build tools in{" "}
              {toolkit}. The tool appears here when it's saved.
            </div>
          ) : (
            <div className="banner warn items-center">
              <span>No connected agent may build tools in {toolkit} yet.</span>
              <Button size="sm" className="ml-auto" onClick={() => connect()}>
                Connect an agent
              </Button>
            </div>
          )}
        </Panel>
      )}
      {start === "code" && (
        <Panel bodyClass="panel-body flex flex-wrap items-center gap-4">
          <div className="min-w-0 flex-1">
            <div className="font-medium">Open the editor</div>
            <div className="muted text-[12.5px]">
              A starter tool with comments. Requests from your applications are listed beside it; click one to insert a
              call. Run it with a test input, then save.
            </div>
          </div>
          <Button
            kind="ink"
            icon={<Code2 size={14} />}
            onClick={() => navigate(`/toolkits/${toolkit}/tools/new?mode=code`)}
          >
            Open the editor
          </Button>
        </Panel>
      )}
    </div>
  );
}

function Workbench({
  toolkit,
  initial,
  initialTests,
  apis,
  types,
  isNew,
}: {
  toolkit: string;
  initial: string;
  initialTests: ToolTest[];
  apis: APIDetail[];
  types: string;
  isNew: boolean;
}) {
  const host = useRef<HTMLDivElement>(null);
  const ed = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const [src, setSrc] = useState(initial);
  const [input, setInput] = useState(() => sampleInput(inputSchemaFrom(initial)));
  const [examples, setExamples] = useState(false);
  const [run, setRun] = useState<RunResult | null>(null);
  const [running, setRunning] = useState(false);
  const [tests, setTests] = useState<ToolTest[]>(initialTests);
  const [saved, setSaved] = useState<SaveResult | null>(null);
  const [problems, setProblems] = useState<monaco.editor.IMarker[]>([]);
  const toast = useToast();
  useEffect(() => setTypes(types), [types]);
  useEffect(() => {
    if (!host.current) return;
    applyTheme();
    const model = monaco.editor.createModel(initial, "typescript", monaco.Uri.parse(`file:///tool-${Date.now()}.ts`));
    const e = monaco.editor.create(host.current, {
      model,
      fontFamily: "JetBrains Mono Variable, JetBrains Mono, monospace",
      fontSize: 12.5,
      lineHeight: 20,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      automaticLayout: true,
      tabSize: 2,
      padding: { top: 10 },
      renderLineHighlight: "line",
      overviewRulerBorder: false,
      fixedOverflowWidgets: true,
    });
    ed.current = e;
    const sub = model.onDidChangeContent(() => setSrc(model.getValue()));
    const mk = monaco.editor.onDidChangeMarkers(() =>
      setProblems(monaco.editor.getModelMarkers({ resource: model.uri })),
    );
    return () => {
      sub.dispose();
      mk.dispose();
      e.dispose();
      model.dispose();
    };
  }, []);
  const parsedInput = () => {
    try {
      return JSON.parse(input || "{}");
    } catch {
      throw new Error("The input isn't valid JSON.");
    }
  };
  const doRun = async () => {
    setRunning(true);
    try {
      setRun(
        await post<RunResult>(`/api/toolkits/${toolkit}/test`, {
          source: ed.current?.getValue() ?? src,
          input: parsedInput(),
          examples,
        }),
      );
    } catch (e) {
      setRun({ error: (e as Error).message, console: [], calls: [] } as unknown as RunResult);
    } finally {
      setRunning(false);
    }
  };
  const save = useAction(
    () => post<SaveResult>(`/api/toolkits/${toolkit}/save`, { source: ed.current?.getValue() ?? src, tests }),
    {
      onSuccess: (r) => {
        setSaved(r);
        toast(r.message);
        if (isNew) navigate(`/toolkits/${toolkit}/tools/${r.version.name}`, true);
      },
    },
  );
  useEffect(() => {
    const k = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      if (e.key === "Enter") {
        e.preventDefault();
        doRun();
      }
      if (e.key.toLowerCase() === "s") {
        e.preventDefault();
        save.mutate(undefined);
      }
    };
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  });
  const insert = (api: string, name: string, url: string) => {
    const params = [...url.matchAll(/\{([A-Za-z0-9_]+)\}/g)].map((m) => m[1]).filter((p) => p !== "baseUrl");
    const text = `const ${name.replace(/^(get|list|search)/, "").replace(/^./, (c) => c.toLowerCase()) || "res"} = await ${api}.${name}(${params.length ? `{ ${params.map((p) => `${p}: input.${p}`).join(", ")} }` : ""});\n`;
    const e = ed.current;
    if (!e) return;
    const pos = e.getPosition();
    e.executeEdits("insert", [{ range: new monaco.Range(pos!.lineNumber, 1, pos!.lineNumber, 1), text: `  ${text}` }]);
    e.focus();
  };
  const errors = problems.filter((p) => p.severity === monaco.MarkerSeverity.Error);
  const saving =
    run && !run.error && run.upstreamBytes > 0 ? Math.round((1 - run.outputBytes / run.upstreamBytes) * 100) : null;
  const total = Math.max(1, ...(run?.calls ?? []).map((c) => c.startMs + c.durationMs), run?.durationMs ?? 1);
  return (
    <div className="grid items-start gap-3" style={{ gridTemplateColumns: "210px minmax(0, 1fr) 380px" }}>
      <Panel title="Insert a call" bodyClass="p-1.5" className="self-stretch">
        <div className="muted flex items-start gap-1.5 px-2 pt-1 pb-2 text-[12px]">
          <MousePointerClick size={13} className="mt-0.5 flex-none" /> Click a request to add a call at the cursor.
        </div>
        {apis.length === 0 && (
          <div className="muted px-2 pb-2 text-[12.5px]">
            No applications yet. <Link to="/apps/new">Create one</Link> to call it from tools.
          </div>
        )}
        <div style={{ maxHeight: 600, overflow: "auto" }}>
          {apis.map((a) => (
            <div key={a.id} className="mb-1">
              <div className="px-2 py-1 text-[12px] font-semibold">
                {a.title || a.name} <span className="mono muted font-normal">{a.name}</span>
              </div>
              {a.requests.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className="nav-item"
                  style={{ padding: "3px 6px", fontSize: 12.5 }}
                  title={`Insert ${a.name}.${r.name}()`}
                  onClick={() => insert(a.name, r.name, r.url)}
                >
                  <MethodChip method={r.method} />
                  <span className="truncate">{r.name}</span>
                </button>
              ))}
            </div>
          ))}
        </div>
      </Panel>
      <div className="flex min-w-0 flex-col gap-3">
        <Panel
          title={
            <span className="flex items-center gap-2">
              Tool code <span className="tag">{(src.match(/name:\s*["'`]([a-z0-9_]+)/) ?? [])[1] ?? "unnamed"}</span>
            </span>
          }
          right={
            <>
              <span className="muted text-[12px]">
                {mod()}+S save · {mod()}+Enter test
              </span>
              <Button
                kind="ink"
                size="sm"
                icon={<Save size={13} />}
                disabled={save.isPending}
                onClick={() => save.mutate(undefined)}
              >
                Save
              </Button>
            </>
          }
        >
          <div ref={host} style={{ height: 470 }} />
          <div className="px-3 py-2 text-[12px]" style={{ borderTop: "1px solid var(--line)" }}>
            {!types ? (
              <span className="muted flex items-center gap-1.5">
                <Loader2 size={12} className="animate-spin" /> Loading types for this toolkit's applications…
              </span>
            ) : errors.length === 0 ? (
              <span className="flex items-center gap-1.5" style={{ color: "var(--ok)" }}>
                <CheckCircle2 size={13} /> No problems
              </span>
            ) : (
              errors.slice(0, 4).map((p, i) => (
                <button
                  key={i}
                  type="button"
                  className="block text-left"
                  style={{ color: "var(--bad)", background: "none", border: 0, padding: 0, cursor: "pointer" }}
                  onClick={() => {
                    ed.current?.setPosition({ lineNumber: p.startLineNumber, column: p.startColumn });
                    ed.current?.focus();
                  }}
                >
                  Line {p.startLineNumber}: {p.message}
                </button>
              ))
            )}
          </div>
        </Panel>
        {(saved || save.error) && (
          <Panel title={saved ? saved.message : "Not saved"}>
            <ErrorBox error={save.error} />
            {saved?.checks.map((c) => (
              <div key={c.rule} className="list-row text-[13px]">
                <Chip tone={c.passed ? "ok" : "bad"}>{c.passed ? "pass" : "fail"}</Chip>
                {c.label}
                {c.detail && <span className="muted">· {c.detail}</span>}
              </div>
            ))}
          </Panel>
        )}
        <TestsPanel tests={tests} setTests={setTests} input={input} />
      </div>
      <div className="flex min-w-0 flex-col gap-3">
        <Panel
          title="Test"
          right={
            <Button kind="blue" size="sm" icon={<Play size={13} />} disabled={running} onClick={doRun}>
              {running ? "Running…" : "Run"}
            </Button>
          }
          bodyClass="p-3 flex flex-col gap-2"
        >
          <textarea
            className="textarea"
            rows={5}
            aria-label="Input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
          />
          <label className="flex items-center gap-2 text-[12.5px]">
            <Toggle label="Use saved examples" checked={examples} onChange={setExamples} />
            Answer from saved examples (no network)
          </label>
        </Panel>
        {run && (
          <Panel
            title={run.error ? <span style={{ color: "var(--bad)" }}>Failed</span> : `Output · ${run.durationMs} ms`}
          >
            {run.error && <div className="banner bad m-3">{run.error}</div>}
            {!run.error && (
              <div
                className="grid grid-cols-2 gap-px"
                style={{ background: "var(--line)", borderBottom: "1px solid var(--line)" }}
              >
                <div className="p-2.5" style={{ background: "var(--surface)" }}>
                  <div className="muted text-[11.5px]">API responses</div>
                  <div className="font-semibold">
                    {bytes(run.upstreamBytes)} · {num(run.upstreamTokens)} tok
                  </div>
                </div>
                <div className="p-2.5" style={{ background: "var(--surface)" }}>
                  <div className="muted text-[11.5px]">
                    Tool output{saving !== null && saving > 0 ? ` · ${saving}% smaller` : ""}
                  </div>
                  <div className="font-semibold">
                    {bytes(run.outputBytes)} · {num(run.outputTokens)} tok
                  </div>
                </div>
              </div>
            )}
            {(run.calls ?? []).length > 0 && (
              <div className="flex flex-col gap-1.5 p-3" style={{ borderBottom: "1px solid var(--line)" }}>
                {run.calls.map((c, i) => (
                  <div key={i} className="text-[12px]">
                    <div className="flex justify-between gap-2">
                      <span className="mono truncate">{c.name}</span>
                      <span className="muted flex-none">
                        {c.error ? "error" : c.status} · {c.durationMs} ms · {bytes(c.bytes)}
                      </span>
                    </div>
                    <div className="tl" title={c.error || c.url}>
                      <span
                        className={c.error || c.status >= 400 ? "err" : ""}
                        style={{ left: `${(c.startMs / total) * 100}%`, width: `${(c.durationMs / total) * 100}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
            {!run.error && (
              <pre className="code" style={{ border: 0, borderRadius: 0, maxHeight: 300 }}>
                {JSON.stringify(run.output, null, 2)}
              </pre>
            )}
            {(run.console ?? []).length > 0 && (
              <pre
                className="code"
                style={{ border: 0, borderTop: "1px solid var(--line)", borderRadius: 0, maxHeight: 120 }}
              >
                {run.console.join("\n")}
              </pre>
            )}
          </Panel>
        )}
        {!run && (
          <Panel>
            <Empty title="Run it">
              See the output, its size against the API responses, and each call on a timeline.
            </Empty>
          </Panel>
        )}
      </div>
    </div>
  );
}

function TestsPanel({
  tests,
  setTests,
  input,
}: {
  tests: ToolTest[];
  setTests: (t: ToolTest[]) => void;
  input: string;
}) {
  const add = () => {
    let parsed: unknown = {};
    try {
      parsed = JSON.parse(input || "{}");
    } catch {
      /* keep empty */
    }
    setTests([
      ...tests,
      {
        name: `case ${tests.length + 1}`,
        input: parsed,
        script: 'pm.test("returns something", () => pm.expect(output).to.exist);',
      },
    ]);
  };
  return (
    <Panel
      title={
        <span className="flex items-center gap-2">
          <FlaskConical size={14} className="muted" /> Tests <span className="chip">{tests.length}</span>
        </span>
      }
      right={
        <Button size="sm" icon={<Plus size={13} />} onClick={add}>
          Add from current input
        </Button>
      }
    >
      {tests.length === 0 && (
        <Empty title="No tests">
          Agent-saved tools need at least one passing test to go live. Tests run on every save.
        </Empty>
      )}
      {tests.map((t, i) => (
        <div
          key={i}
          className="grid gap-2 p-3"
          style={{ gridTemplateColumns: "160px 1fr 1.4fr auto", borderBottom: "1px solid var(--line)" }}
        >
          <input
            className="input"
            aria-label="Test name"
            value={t.name}
            onChange={(e) => setTests(tests.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)))}
          />
          <textarea
            className="textarea"
            rows={2}
            aria-label="Test input"
            defaultValue={JSON.stringify(t.input)}
            onBlur={(e) => {
              try {
                const v = JSON.parse(e.target.value);
                setTests(tests.map((x, j) => (j === i ? { ...x, input: v } : x)));
              } catch {
                /* keep old */
              }
            }}
          />
          <textarea
            className="textarea"
            rows={2}
            aria-label="Test script"
            value={t.script ?? ""}
            onChange={(e) => setTests(tests.map((x, j) => (j === i ? { ...x, script: e.target.value } : x)))}
          />
          <Button
            size="sm"
            kind="ghost"
            aria-label="Remove test"
            icon={<Trash2 size={13} />}
            onClick={() => setTests(tests.filter((_, j) => j !== i))}
          />
        </div>
      ))}
    </Panel>
  );
}

function Versions({ versions }: { versions: Version[] }) {
  const [left, setLeft] = useState(versions[1]?.id ?? versions[0]?.id);
  const [right, setRight] = useState(versions[0]?.id);
  const toast = useToast();
  const roll = useAction((id: string) => post<SaveResult>(`/api/versions/${id}/rollback`), {
    onSuccess: (r) => toast(r.message),
  });
  const L = versions.find((v) => v.id === left);
  const R = versions.find((v) => v.id === right);
  return (
    <div className="grid items-start gap-4" style={{ gridTemplateColumns: "300px minmax(0, 1fr)" }}>
      <Panel>
        {versions.map((v, i) => (
          <div key={v.id} className="list-row items-start">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <strong>v{v.n}</strong> <StatusChip status={v.status} />
              </div>
              <div className="muted text-[12px]">
                {v.createdByKind === "you" ? "You" : v.createdByName} · {dateTime(v.createdAt)}
                {v.note && ` · ${v.note}`}
              </div>
              <div className="mt-1 flex gap-1">
                <Button size="sm" kind={left === v.id ? "ink" : "ghost"} onClick={() => setLeft(v.id)}>
                  Left
                </Button>
                <Button size="sm" kind={right === v.id ? "ink" : "ghost"} onClick={() => setRight(v.id)}>
                  Right
                </Button>
                {i > 0 && (
                  <Button size="sm" icon={<RotateCcw size={12} />} onClick={() => roll.mutate(v.id)}>
                    Roll back
                  </Button>
                )}
              </div>
            </div>
          </div>
        ))}
        <ErrorBox error={roll.error} />
      </Panel>
      <Panel title={L && R ? `v${L.n} → v${R.n}` : "Diff"}>
        {L && R ? <Diff left={L.source} right={R.source} /> : <Empty title="Only one version" />}
      </Panel>
    </div>
  );
}

function Fields({
  fields,
  latest,
}: {
  fields: { path: string; used: string[]; unused: string[] }[];
  latest?: Version;
}) {
  return (
    <Panel title="Output fields reported by agents (last 30 days)">
      {fields.length === 0 ? (
        <Empty title="No reports yet">
          Agents call report_field_usage after using a tool; mcpDeck's hint on large responses reminds them.
        </Empty>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Field</th>
              <th>Used by</th>
              <th>Reported unused by</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {fields.map((f) => (
              <tr key={f.path}>
                <td className="mono">{f.path}</td>
                <td>{f.used.join(", ") || <span className="muted">–</span>}</td>
                <td>{f.unused.join(", ") || <span className="muted">–</span>}</td>
                <td>
                  {f.unused.length > 0 && f.used.length === 0 && <Chip tone="warn">candidate to remove</Chip>}
                  {f.unused.length > 0 && f.used.length > 0 && <Chip>used by some agents</Chip>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <div className="muted p-3 text-[12px]" style={{ borderTop: "1px solid var(--line)" }}>
        Reports never change a tool. To drop a field, edit the code and save a new version
        {latest ? ` (now v${latest.n})` : ""}.
      </div>
    </Panel>
  );
}
