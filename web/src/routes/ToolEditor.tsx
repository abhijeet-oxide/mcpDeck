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
  Save,
  Search,
  Send,
  ShieldCheck,
  Trash2,
  Workflow,
} from "lucide-react";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { useConnect } from "../App";
import { useRunner } from "../components/AskAgent";
import { counts, FindingList } from "../components/Findings";
import { exampleOf, FlowBuilder, requestInfo } from "../components/FlowBuilder";
import {
  AccessChip,
  Button,
  Chip,
  CopyButton,
  Empty,
  EmptyState,
  ErrorBox,
  Field,
  Hint,
  Loading,
  MethodChip,
  PageHead,
  Panel,
  StatusChip,
  Tabs,
  Toggle,
  useConfirm,
  useToast,
} from "../components/ui";
import { VersionTimeline } from "../components/VersionTimeline";
import { get, post } from "../lib/api";
import { autoWire, type Flow, fieldsOf, generateFlow, matchBrace, parseFlow, type Step, stepId } from "../lib/flow";
import { bytes, dateTime, mod, num } from "../lib/format";
import { useAction, useAPIs, useTool } from "../lib/hooks";
import { applyTheme, monaco, setTypes } from "../lib/monaco";
import { Link, navigate, useLocation } from "../lib/router";
import { starterTemplate as template } from "../lib/starter";
import type { APIDetail, LintReport, OutputReport, RunResult, SaveResult, ToolTest, Version } from "../lib/types";

/** flowFrom builds a starting flow from requests picked in order (TOOL-10, FLOW-1). */
export function flowFrom(refs: string[], apis: APIDetail[]): Flow {
  const flow: Flow = {
    meta: { name: "", description: "", useWhen: "", dontUseWhen: "" },
    inputs: [],
    steps: [],
    output: null,
  };
  for (const ref of refs) {
    const [apiName, reqName] = ref.split(".");
    const api = apis.find((a) => a.name === apiName);
    const r = api?.requests.find((x) => x.name === reqName);
    if (!api || !r) continue;
    const step: Step = {
      id: stepId(
        reqName,
        flow.steps.map((s) => s.id),
      ),
      api: apiName,
      req: reqName,
      args: [],
    };
    const before = flow.steps.map((s) => {
      const sa = apis.find((a) => a.name === s.api);
      const sr = sa?.requests.find((x) => x.name === s.req);
      return { step: s, fields: sr ? fieldsOf(exampleOf(sr), "wire", 200) : [] };
    });
    const w = autoWire(step, requestInfo(api, r), before, flow.inputs, api.auth?.type ?? "none");
    step.args = w.args;
    flow.inputs.push(...w.newInputs);
    flow.steps.push(step);
  }
  const last = flow.steps[flow.steps.length - 1];
  if (last) {
    flow.output = { step: last.id, fields: [] };
    flow.meta.name = last.req.replace(/([A-Z])/g, "_$1").toLowerCase();
  }
  return flow;
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
  const stepsParam = search.get("steps") ?? search.get("from") ?? "";
  const mode = search.get("mode") ?? (stepsParam ? "visual" : "");
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
    if (stepsParam) {
      if (apis.isLoading || details.some((d) => d.isLoading)) return <Loading />;
      initial = generateFlow(
        flowFrom(stepsParam.split(",").filter(Boolean), details.map((d) => d.data).filter(Boolean) as APIDetail[]),
      );
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
            { id: "code", label: "Build" },
            { id: "versions", label: "Versions", n: versions.length },
            { id: "fields", label: "Output fields" },
          ]}
        />
      )}
      {/* The workbench stays mounted on other tabs so unsaved edits survive. */}
      <div hidden={tab !== "code"}>
        <Hint id="tool-editor">
          A tool is one function an agent can call. Build its steps in Visual or Code, try it with <b>Run</b>, check the{" "}
          <b>Guidelines</b>, then <b>Save</b> to make a new version.
        </Hint>
        <Workbench
          key={latest?.id ?? "new"}
          toolkit={toolkit}
          initial={initial}
          initialTests={latest?.tests ?? []}
          apis={details.map((d) => d.data).filter(Boolean) as APIDetail[]}
          types={types.data ?? ""}
          isNew={isNew}
          initialMode={mode === "code" ? "code" : "visual"}
        />
      </div>
      {tab === "versions" && <VersionTimeline versions={versions} onRestored={() => setTab("code")} />}
      {tab === "fields" && <Fields fields={detail.data?.fields ?? []} latest={latest} />}
    </div>
  );
}

type Start = "request" | "agent" | "code";

/** NewTool asks how to start, so the editor never opens on code nobody asked for. */
function NewTool({ toolkit, apis }: { toolkit: string; apis: APIDetail[] }) {
  const reqs = apis.flatMap((a) => a.requests.map((r) => ({ a, r })));
  const [chosen, setStart] = useState<Start | null>(null);
  // Chaining requests is the default; with no requests at all, its panel says to add an application.
  const start: Start = chosen ?? "request";
  const [filter, setFilter] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
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
      title: "Chain requests",
      sub: "Pick one or more in the order they run; connect them on a canvas.",
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
          title={
            <span className="flex items-center gap-2">
              Pick requests in order
              <span className="muted text-[12px] font-normal">
                login → list → details: each one can use the last one's output
              </span>
            </span>
          }
          footer={
            reqs.length > 0 && (
              <>
                <span className="muted mr-auto text-[12.5px]">
                  {picked.length === 0
                    ? "Nothing picked yet. You can also add steps on the canvas."
                    : picked.map((p, i) => `${i + 1}. ${p}`).join("  →  ")}
                </span>
                {picked.length > 0 && (
                  <Button kind="ghost" onClick={() => setPicked([])}>
                    Clear
                  </Button>
                )}
                <Button
                  kind="ink"
                  icon={<Workflow size={14} />}
                  onClick={() =>
                    navigate(
                      `/toolkits/${toolkit}/tools/new?mode=visual${picked.length ? `&steps=${encodeURIComponent(picked.join(","))}` : ""}`,
                    )
                  }
                >
                  {picked.length
                    ? `Build a flow with ${picked.length} ${picked.length === 1 ? "step" : "steps"}`
                    : "Open an empty canvas"}
                </Button>
              </>
            )
          }
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
              {shown.map(({ a, r }) => {
                const ref = `${a.name}.${r.name}`;
                const n = picked.indexOf(ref);
                const saved = (r.responses?.length ?? 0) > 0 || !!r.example;
                return (
                  <button
                    key={r.id}
                    type="button"
                    className="list-row w-full text-left"
                    aria-pressed={n >= 0}
                    style={{
                      background: n >= 0 ? "var(--accent-soft)" : "none",
                      border: 0,
                      borderBottom: "1px solid var(--line)",
                      cursor: "pointer",
                    }}
                    onClick={() => setPicked(n >= 0 ? picked.filter((x) => x !== ref) : [...picked, ref])}
                  >
                    <span className={`step-dot ${n >= 0 ? "next" : ""}`} aria-hidden>
                      {n >= 0 ? n + 1 : ""}
                    </span>
                    <MethodChip method={r.method} />
                    <span className="min-w-0 flex-1">
                      <code className="font-semibold">
                        {a.name}.{r.name}
                      </code>
                      <span className="muted block truncate text-[12.5px]">
                        {r.title || r.url.replace("{{baseUrl}}", "")}
                      </span>
                    </span>
                    {saved ? (
                      <Chip tone="ok">response saved</Chip>
                    ) : (
                      <span
                        className="muted text-[12px]"
                        title="Send it once and save the response so its fields are known"
                      >
                        no saved response
                      </span>
                    )}
                    <AccessChip access={r.access} flag={r.accessFlag} />
                  </button>
                );
              })}
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

type Mode = "visual" | "code";

function Workbench({
  toolkit,
  initial,
  initialTests,
  apis,
  types,
  isNew,
  initialMode,
}: {
  toolkit: string;
  initial: string;
  initialTests: ToolTest[];
  apis: APIDetail[];
  types: string;
  isNew: boolean;
  initialMode: Mode;
}) {
  const host = useRef<HTMLDivElement>(null);
  const ed = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const [src, setSrc] = useState(initial);
  const [mode, setMode] = useState<Mode>(() => (initialMode === "visual" && parseFlow(initial).ok ? "visual" : "code"));
  const [input, setInput] = useState(() => sampleInput(inputSchemaFrom(initial)));
  const [examples, setExamples] = useState(false);
  const [run, setRun] = useState<RunResult | null>(null);
  const [running, setRunning] = useState(false);
  const [tests, setTests] = useState<ToolTest[]>(initialTests);
  const [saved, setSaved] = useState<SaveResult | null>(null);
  const [problems, setProblems] = useState<monaco.editor.IMarker[]>([]);
  const toast = useToast();
  const confirm = useConfirm();
  const parsed = useMemo(() => parseFlow(src), [src]);
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
  /** setCode replaces the source, keeping Monaco's undo history. */
  const setCode = (code: string) => {
    const m = ed.current?.getModel();
    if (m && m.getValue() !== code) {
      m.pushEditOperations([], [{ range: m.getFullModelRange(), text: code }], () => null);
    }
    setSrc(code);
  };
  const onFlow = (f: Flow) => setCode(generateFlow(f));
  const switchTo = async (m: Mode) => {
    if (m === "visual" && !parsed.ok) {
      const ok = await confirm({
        title: "Rebuild this tool as a flow?",
        body: (
          <>
            The visual builder can't show this code: {parsed.reason} Keep it in Code, or rebuild it as steps. Rebuilding
            keeps the tool definition and the calls it can read, and drops the custom logic.
          </>
        ),
        action: "Rebuild as a flow",
        danger: true,
      });
      if (!ok) return;
      setCode(generateFlow(salvage(src)));
    }
    setMode(m);
  };
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
          source: src,
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
  const save = useAction(() => post<SaveResult>(`/api/toolkits/${toolkit}/save`, { source: src, tests }), {
    onSuccess: (r) => {
      setSaved(r);
      toast(r.message);
      if (isNew) navigate(`/toolkits/${toolkit}/tools/${r.version.name}`, true);
    },
  });
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
  // Keep the test input's keys in step with the tool's inputs.
  useEffect(() => {
    if (!parsed.ok) return;
    try {
      const cur = JSON.parse(input || "{}");
      const want = parsed.flow.inputs.map((i) => i.name);
      if (want.length && want.every((k) => k in cur) && Object.keys(cur).every((k) => want.includes(k))) return;
      const next: Record<string, unknown> = {};
      for (const k of want) next[k] = cur[k] ?? "";
      setInput(JSON.stringify(next, null, 2));
    } catch {
      /* the person is editing the JSON */
    }
  }, [parsed.ok && parsed.flow.inputs.map((i) => i.name).join(",")]);
  /** insert adds a call inside the function body, before its return. */
  const insert = (api: string, name: string, url: string) => {
    const e = ed.current;
    const m = e?.getModel();
    if (!e || !m) return;
    const params = [...url.replace(/\{\{[^}]*\}\}/g, "").matchAll(/\{([A-Za-z0-9_]+)\}/g)].map((x) => x[1]);
    const taken = [...m.getValue().matchAll(/const\s+([\w$]+)/g)].map((x) => x[1]);
    const id = stepId(name, taken);
    const text = `  const ${id} = await ${api}.${name}(${params.length ? `{ ${params.map((p) => `${p}: input.${p}`).join(", ")} }` : ""});\n`;
    const code = m.getValue();
    const fn = /export\s+default\s+async\s+function[^{]*\{/.exec(code);
    let line = e.getPosition()?.lineNumber ?? 1;
    if (fn) {
      const open = fn.index + fn[0].length - 1;
      const close = matchBrace(code, open);
      const startLine = code.slice(0, open).split("\n").length + 1;
      const endLine = close > 0 ? code.slice(0, close).split("\n").length : startLine;
      if (line < startLine || line > endLine) {
        const ret = code.slice(open, close).search(/\n\s*return\b/);
        line = ret >= 0 ? code.slice(0, open + ret + 1).split("\n").length + 1 : endLine;
      }
    }
    e.executeEdits("insert", [{ range: new monaco.Range(line, 1, line, 1), text }]);
    e.focus();
  };
  const errors = problems.filter((p) => p.severity === monaco.MarkerSeverity.Error);
  const saving =
    run && !run.error && run.upstreamBytes > 0 ? Math.round((1 - run.outputBytes / run.upstreamBytes) * 100) : null;
  const total = Math.max(1, ...(run?.calls ?? []).map((c) => c.startMs + c.durationMs), run?.durationMs ?? 1);
  const name = (src.match(/name:\s*["'`]([a-z0-9_]+)/) ?? [])[1] ?? "unnamed";
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <Panel
        title={
          <span className="flex items-center gap-2">
            Tool <span className="tag mono">{name}</span>
            {mode === "code" && !parsed.ok && (
              <span className="muted text-[12px] font-normal" title={parsed.reason}>
                custom code
              </span>
            )}
          </span>
        }
        right={
          <>
            <span className="muted hidden text-[12px] xl:inline">
              {mod()}+S save · {mod()}+Enter run
            </span>
            <div className="seg" role="group" aria-label="Editor mode">
              <button type="button" aria-pressed={mode === "visual"} onClick={() => switchTo("visual")}>
                <Workflow size={13} className="mr-1 inline" /> Visual
              </button>
              <button type="button" aria-pressed={mode === "code"} onClick={() => switchTo("code")}>
                <Code2 size={13} className="mr-1 inline" /> Code
              </button>
            </div>
            <Button kind="blue" size="sm" icon={<Play size={13} />} disabled={running} onClick={doRun}>
              {running ? "Running…" : "Run"}
            </Button>
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
        {mode === "visual" && parsed.ok && <FlowBuilder flow={parsed.flow} onChange={onFlow} apis={apis} run={run} />}
        <div
          className="grid"
          style={{ gridTemplateColumns: "220px minmax(0, 1fr)", display: mode === "code" ? "grid" : "none" }}
        >
          <div className="p-1.5" style={{ borderRight: "1px solid var(--line)", maxHeight: 520, overflow: "auto" }}>
            <div className="muted flex items-start gap-1.5 px-2 pt-1 pb-2 text-[12px]">
              <MousePointerClick size={13} className="mt-0.5 flex-none" /> Click a request to add a call to the
              function.
            </div>
            {apis.length === 0 && (
              <div className="muted px-2 pb-2 text-[12.5px]">
                No applications yet. <Link to="/apps/new">Create one</Link> to call it from tools.
              </div>
            )}
            {apis.map((a) => (
              <div key={a.id} className="mb-1">
                <div className="px-2 py-1 text-[12px] font-semibold">
                  {a.title || a.name} <span className="mono muted font-normal">{a.name}</span>
                </div>
                {a.requests.map((r) => (
                  <button
                    key={r.id}
                    type="button"
                    className="tree-row"
                    style={{ fontSize: 12.5 }}
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
          <div className="min-w-0">
            <div ref={host} style={{ height: 520 }} />
            <div className="px-3 py-2 text-[12px]" style={{ borderTop: "1px solid var(--line)" }}>
              {!types ? (
                <span className="muted flex items-center gap-1.5">
                  <Loader2 size={12} className="animate-spin" /> Loading types for this toolkit's applications…
                </span>
              ) : errors.length === 0 ? (
                <span className="flex items-center gap-1.5" style={{ color: "var(--ok)" }}>
                  <CheckCircle2 size={13} /> No problems
                  {parsed.ok && <span className="muted">· the visual builder can show this code</span>}
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
          </div>
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
      <div className="grid items-start gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)]">
        <div className="flex min-w-0 flex-col gap-3">
          <Panel
            title="Test input"
            intro={`Sample arguments, the way an agent would send them. Run (${mod()}+Enter) calls the tool with them.`}
            right={
              <label className="flex items-center gap-2 text-[12px]">
                <Toggle label="Use saved responses" checked={examples} onChange={setExamples} />
                Saved responses only
              </label>
            }
            bodyClass="p-3"
          >
            <textarea
              className="textarea"
              rows={6}
              aria-label="Input"
              value={input}
              onChange={(e) => setInput(e.target.value)}
            />
            <div className="muted mt-2 text-[12px]">
              Run sends every step through mcpDeck, like an agent's call. With saved responses only, nothing reaches the
              network.
            </div>
          </Panel>
          <TestsPanel tests={tests} setTests={setTests} input={input} />
        </div>
        <div className="flex min-w-0 flex-col gap-3">
          {run ? (
            <Panel
              title={run.error ? <span style={{ color: "var(--bad)" }}>Failed</span> : `Output · ${run.durationMs} ms`}
              intro="What the agent gets back from the last run, and each call it made."
            >
              {run.error && <div className="banner bad m-3">{run.error}</div>}
              {!run.error && (
                <div
                  className="grid grid-cols-2 gap-px"
                  style={{ background: "var(--line)", borderBottom: "1px solid var(--line)" }}
                >
                  <div className="p-2.5" style={{ background: "var(--surface)" }}>
                    <div className="muted text-[11.5px]">Fetched from APIs</div>
                    <div className="font-semibold">
                      {bytes(run.upstreamBytes)} · {num(run.upstreamTokens)} tok
                    </div>
                  </div>
                  <div className="p-2.5" style={{ background: "var(--surface)" }}>
                    <div className="muted text-[11.5px]">
                      Returned to the agent{saving !== null && saving > 0 ? ` · ${saving}% smaller` : ""}
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
                        <span className="mono truncate">
                          {i + 1}. {c.name}
                        </span>
                        <span className="muted flex-none">
                          {c.error ? "error" : c.status} · {c.durationMs} ms · {num(c.tokens)} tok
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
                <pre className="code" style={{ border: 0, borderRadius: 0, maxHeight: 320 }}>
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
          ) : (
            <Panel title="Output" intro="What the agent gets back from the last run, and each call it made.">
              <Empty title="Run it">
                Every step runs in order. You'll see each call on a timeline, and tokens fetched against tokens
                returned.
              </Empty>
            </Panel>
          )}
        </div>
        <GuidePanel toolkit={toolkit} src={src} run={run} />
      </div>
    </div>
  );
}

/** salvage keeps what a flow can use from code the builder can't show. */
function salvage(src: string): Flow {
  const meta = {
    name: (src.match(/name:\s*["'`]([\w]+)/) ?? [])[1] ?? "",
    description: (src.match(/description:\s*["'`]([^"'`]*)/) ?? [])[1] ?? "",
    useWhen: (src.match(/useWhen:\s*["'`]([^"'`]*)/) ?? [])[1] ?? "",
    dontUseWhen: (src.match(/dontUseWhen:\s*["'`]([^"'`]*)/) ?? [])[1] ?? "",
  };
  const steps: Step[] = [];
  for (const m of src.matchAll(/(?:const\s+([\w$]+)\s*=\s*)?await\s+([\w$]+)\.([\w$]+)\(/g)) {
    const id = stepId(
      m[1] || m[3],
      steps.map((s) => s.id),
    );
    steps.push({ id, api: m[2], req: m[3], args: [] });
  }
  return { meta, inputs: [], steps, output: steps.length ? { step: steps[steps.length - 1].id, fields: [] } : null };
}

/** GuidePanel checks the draft against the MCP guidelines as you build (GUIDE-2, GUIDE-3). */
function GuidePanel({ toolkit, src, run }: { toolkit: string; src: string; run: RunResult | null }) {
  const [draft, setDraft] = useState(src);
  useEffect(() => {
    const t = setTimeout(() => setDraft(src), 600);
    return () => clearTimeout(t);
  }, [src]);
  const lint = useQuery({
    queryKey: ["lint", toolkit, draft],
    queryFn: () => post<LintReport>(`/api/toolkits/${toolkit}/lint`, { draft }),
    placeholderData: (p) => p,
  });
  const out = useQuery({
    queryKey: ["lint-out", run?.outputJson?.length, run?.outputJson?.slice(0, 100)],
    queryFn: () => post<OutputReport>("/api/lint/output", { value: run!.output }),
    enabled: !!run && !run.error && run.output !== undefined,
  });
  const l = lint.data;
  const findings = [...(l?.draft ?? []), ...(out.data?.findings ?? [])];
  const c = counts(findings);
  return (
    <Panel
      title={
        <span className="flex items-center gap-2">
          <ShieldCheck size={14} className="muted" /> Guidelines
        </span>
      }
      intro="Checks that help agents pick and call the tool: name, description, input schema and output size. Blocking findings stop a save."
      right={
        <Link to={`/toolkits/${toolkit}?tab=guidelines`} className="text-[12.5px]">
          All rules
        </Link>
      }
    >
      {l && (
        <div
          className="grid grid-cols-3 gap-px text-[12px]"
          style={{ background: "var(--line)", borderBottom: "1px solid var(--line)" }}
        >
          <div className="p-2.5" style={{ background: "var(--surface)" }}>
            <div className="muted">Tools after saving</div>
            <div className="font-semibold">{l.tools}</div>
          </div>
          <div className="p-2.5" style={{ background: "var(--surface)" }}>
            <div className="muted">tools/list (est.)</div>
            <div className="font-semibold">{num(l.listTokens)} tok</div>
          </div>
          <div className="p-2.5" style={{ background: "var(--surface)" }}>
            <div className="muted">Toolkit score</div>
            <div className="font-semibold">{l.score}</div>
          </div>
        </div>
      )}
      {l?.draftError && <div className="banner warn m-3 text-[12.5px]">{l.draftError}</div>}
      <div className="flex gap-1.5 px-3 pt-3">
        {c.block > 0 && <Chip tone="bad">{c.block} blocking</Chip>}
        {c.warn > 0 && (
          <Chip tone="warn">
            {c.warn} {c.warn === 1 ? "warning" : "warnings"}
          </Chip>
        )}
        {c.hint > 0 && (
          <Chip tone="info">
            {c.hint} {c.hint === 1 ? "hint" : "hints"}
          </Chip>
        )}
        {findings.length === 0 && l && <Chip tone="ok">Nothing to fix</Chip>}
      </div>
      <div style={{ maxHeight: 420, overflow: "auto" }}>
        <FindingList
          findings={findings}
          empty={run ? "The draft and its output follow the guidelines." : "Run the tool to check its output too."}
        />
      </div>
    </Panel>
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
      intro="A test is a saved input plus checks on the output, written like Postman tests. Every save runs them; tools that write run them against saved responses only."
      right={
        <Button size="sm" icon={<Plus size={13} />} onClick={add}>
          Add from current input
        </Button>
      }
    >
      {tests.length === 0 && (
        <Empty title="No tests yet">
          Add one from the current input, then write a check such as{" "}
          <code className="inline-code">pm.expect(output.name).to.be.a("string")</code>. If the toolkit requires tests,
          a version an agent saves only goes live when they pass.
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
