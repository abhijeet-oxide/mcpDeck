import {
  Background,
  type Connection,
  Controls,
  type Edge,
  Handle,
  type Node,
  type NodeProps,
  Panel,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  closestCenter,
  DndContext,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  ArrowDown,
  ArrowUp,
  Braces,
  CircleAlert,
  CircleHelp,
  Columns3,
  CornerDownRight,
  FileOutput,
  GripVertical,
  KeyRound,
  LayoutGrid,
  LogIn,
  Maximize2,
  Minimize2,
  Pencil,
  Plus,
  Search,
  Trash2,
} from "lucide-react";
import { createContext, type DragEvent, memo, useContext, useEffect, useMemo, useState } from "react";
import {
  type Arg,
  argsOf,
  autoWire,
  codeDefines,
  type FieldInfo,
  type Flow,
  fieldsOf,
  moveStep,
  type ReqInfo,
  type Source,
  type Step,
  sourceCode,
  sourceInputs,
  sourceSteps,
  stepId,
  type ToolInput,
} from "../lib/flow";
import { num } from "../lib/format";
import type { APIDetail, Request, RunResult } from "../lib/types";
import { Button, Chip, Field, MethodChip, Select, Toggle, useToast } from "./ui";

type Ctx = {
  flow: Flow;
  reqOf: (s: Step) => { api: APIDetail; r: Request } | null;
  run?: RunResult | null;
  selected: string;
  select: (id: string) => void;
};
const FlowCtx = createContext<Ctx>(null!);

export function requestInfo(api: APIDetail, r: Request): ReqInfo {
  return {
    api: api.name,
    name: r.name,
    method: r.method,
    url: r.url,
    query: r.query,
    bodyType: r.bodyType,
    body: r.body,
    example: exampleOf(r),
  };
}

export function exampleOf(r: Request): unknown {
  const ok = r.responses?.find((x) => x.status < 300);
  return ok?.body ?? r.example ?? undefined;
}

function pickValue(v: unknown, paths: string[]): unknown {
  // Mirrors the sandbox's pick(), for size estimates in the builder.
  const tree: Record<string, any> = {};
  for (const p of paths) {
    let n = tree;
    for (const seg of p.replace(/\[\]/g, ".[]").split(".").filter(Boolean)) n = n[seg] ??= {};
  }
  const apply = (x: any, node: Record<string, any>): any => {
    const keys = Object.keys(node);
    if (!keys.length || x === null || typeof x !== "object") return x;
    if (Array.isArray(x)) return x.map((i) => apply(i, node["[]"] ?? node));
    const o: Record<string, any> = {};
    for (const k of keys) if (k !== "[]" && k in x) o[k] = apply(x[k], node[k]);
    return o;
  };
  return paths.length ? apply(v, tree) : v;
}

export const tokensOf = (v: unknown) => Math.ceil((JSON.stringify(v ?? null)?.length ?? 0) / 4);

function describeSource(s: Source): string {
  switch (s.kind) {
    case "input":
      return `input · ${s.field}`;
    case "step":
      return `${s.step} · ${s.path || "all"}`;
    case "literal":
      return JSON.stringify(s.value);
    case "template":
      return `\`${s.text}\``;
    case "expr":
      return s.code;
  }
}

// ───────── Nodes ─────────

function InputNode({ id }: NodeProps) {
  const { flow, selected, select } = useContext(FlowCtx);
  return (
    <div className={`fnode fnode-input${selected === id ? " sel" : ""}`}>
      <div className="fnode-head">
        <LogIn size={14} />
        <span className="font-semibold">Agent input</span>
        <button
          type="button"
          className="fnode-edit nodrag"
          title="Edit the inputs agents fill in"
          aria-label="Edit agent inputs"
          onClick={() => select("input")}
        >
          <Pencil size={12} /> Edit
        </button>
      </div>
      <div className="fnode-sub">What the agent passes when it calls the tool.</div>
      {flow.inputs.length === 0 && (
        <button type="button" className="fnode-more nodrag" onClick={() => select("input")}>
          + Add an input
        </button>
      )}
      {flow.inputs.map((i) => (
        <div key={i.name} className="fport out" title={i.description || `${i.name} (${i.type || "any"})`}>
          <span className="mono">{i.name}</span>
          <span className="faint">
            {i.type || "any"}
            {i.required ? "" : " · optional"}
          </span>
          <Handle
            type="source"
            position={Position.Right}
            id={`in:${i.name}`}
            title="Drag to a parameter of a step to pass this input"
          />
        </div>
      ))}
    </div>
  );
}

function CodeNode({ id, s, idx }: { id: string; s: Step; idx: number }) {
  const { selected } = useContext(FlowCtx);
  const lines = (s.code ?? "").split("\n");
  const defs = codeDefines(s.code ?? "");
  return (
    <div className={`fnode fnode-code${selected === id ? " sel" : ""}`}>
      <Handle
        type="target"
        position={Position.Left}
        id="seq"
        className="seq"
        title="Run order: runs after the step on its left"
      />
      <div className="fnode-head">
        <span className="fnode-n">{idx + 1}</span>
        <Braces size={14} />
        <span className="font-semibold">Custom code</span>
      </div>
      {s.comment && <div className="fnode-sub">{s.comment}</div>}
      <pre className="fnode-code-body">
        {lines.slice(0, 8).join("\n")}
        {lines.length > 8 ? `\n… ${lines.length - 8} more lines` : ""}
      </pre>
      {defs.length > 0 && (
        <div className="fnode-sub">
          Defines <span className="mono">{defs.join(", ")}</span>
        </div>
      )}
      <Handle
        type="source"
        position={Position.Right}
        id="all"
        className="seq"
        title="Run order: the next step runs after this one"
      />
    </div>
  );
}

function StepNode({ id, data }: NodeProps) {
  const { flow, reqOf, run, selected } = useContext(FlowCtx);
  const [more, setMore] = useState(false);
  const idx = (data as { index: number }).index;
  const s = flow.steps[idx];
  if (!s) return null;
  if (s.kind === "code") return <CodeNode id={id} s={s} idx={idx} />;
  const found = reqOf(s);
  const info = found ? requestInfo(found.api, found.r) : null;
  const params = info ? argsOf(info) : [];
  const fields =
    info?.example !== undefined ? fieldsOf(info.example, "wire", 80).filter((f) => f.type !== "array") : [];
  const shown = more ? fields : fields.slice(0, 6);
  const nth = flow.steps.slice(0, idx).filter((x) => x.api === s.api && x.req === s.req).length;
  const call = run?.calls?.filter((c) => c.name === `${s.api}.${s.req}`)[nth];
  const wired = new Set(s.args.map((a) => (a.header ? `h:${a.name}` : a.name)));
  const missing = params.filter((p) => p.where === "path" && !wired.has(p.name));
  return (
    <div className={`fnode${selected === id ? " sel" : ""}${!found ? " bad" : ""}`}>
      <Handle
        type="target"
        position={Position.Left}
        id="seq"
        className="seq"
        title="Run order: runs after the step on its left"
      />
      <div className="fnode-head">
        <span className="fnode-n">{idx + 1}</span>
        {found && <MethodChip method={found.r.method} />}
        <span className="mono min-w-0 flex-1 truncate font-semibold">
          {s.api}.{s.req}
        </span>
        {call && (
          <Chip tone={call.error || call.status >= 400 ? "bad" : "ok"}>{call.error ? "error" : call.status}</Chip>
        )}
      </div>
      <div className="fnode-sub mono">
        {s.discard ? "result not kept" : `const ${s.id}`}
        {s.group !== undefined && (
          <span className="fnode-par" title="Runs at the same time as the other steps in this column (Promise.all)">
            <Columns3 size={11} /> parallel
          </span>
        )}
      </div>
      {s.comment && <div className="fnode-sub">{s.comment}</div>}
      {!found && (
        <div className="fnode-warn">
          No request {s.api}.{s.req} in your applications.
        </div>
      )}
      <div className="fnode-sec" title="What this request needs. Connect an agent input or a field of an earlier step.">
        Parameters
      </div>
      {s.args.length === 0 && params.length === 0 && <div className="fnode-empty">No parameters</div>}
      {s.args.map((a) => (
        <div key={(a.header ? "h:" : "") + a.name} className="fport in">
          <Handle
            type="target"
            position={Position.Left}
            id={`${a.header ? "hdr" : "arg"}:${a.name}`}
            title="Connect an agent input or an earlier step's field here"
          />
          {a.header && <KeyRound size={11} className="muted flex-none" />}
          <span className="mono">{a.name}</span>
          <span className="faint min-w-0 truncate">← {describeSource(a.source)}</span>
        </div>
      ))}
      {missing.map((p) => (
        <div key={p.name} className="fport in miss">
          <Handle
            type="target"
            position={Position.Left}
            id={`arg:${p.name}`}
            title="This parameter needs a value: connect an input or a field here"
          />
          <CircleAlert size={11} className="flex-none" />
          <span className="mono">{p.name}</span>
          <span className="min-w-0 truncate">needs a value</span>
        </div>
      ))}
      <div
        className="fnode-sec"
        title="Fields of this request's saved response. Drag one to a parameter of a later step to pass it on."
      >
        Response fields
        {call && !call.error && <span className="faint ml-auto font-normal">{num(call.tokens ?? 0)} tok</span>}
      </div>
      {fields.length === 0 && (
        <div className="fnode-empty">Send it in its application and save the 200 response to see its fields.</div>
      )}
      {shown.map((f) => (
        <FieldPort key={f.path} f={f} />
      ))}
      {fields.length > 6 && (
        <button type="button" className="fnode-more nodrag" onClick={() => setMore(!more)}>
          {more ? "Fewer fields" : `${fields.length - 6} more fields`}
        </button>
      )}
      <Handle
        type="source"
        position={Position.Right}
        id="all"
        className="seq"
        title="Run order: the next step runs after this one. Drag to “Returns to the agent” to return this step."
      />
    </div>
  );
}

const FieldPort = memo(function FieldPort({ f }: { f: FieldInfo }) {
  return (
    <div className="fport out" title={f.sample ? `e.g. ${f.sample}` : f.type}>
      <span className="mono min-w-0 truncate">{f.path}</span>
      <span className="faint">{f.type}</span>
      <Handle
        type="source"
        position={Position.Right}
        id={`out:${f.path}`}
        title="Drag to a parameter of a later step to pass this field"
      />
    </div>
  );
});

function OutputNode({ id }: NodeProps) {
  const { flow, reqOf, run, selected } = useContext(FlowCtx);
  const o = flow.output;
  const step = o && "step" in o ? flow.steps.find((s) => s.id === o.step) : undefined;
  const found = step ? reqOf(step) : null;
  const ex = found ? exampleOf(found.r) : undefined;
  const fields = o && "fields" in o ? o.fields : [];
  const est = ex !== undefined ? tokensOf(pickValue(ex, fields)) : null;
  const full = ex !== undefined ? tokensOf(ex) : null;
  return (
    <div className={`fnode fnode-output${selected === id ? " sel" : ""}`}>
      <Handle
        type="target"
        position={Position.Left}
        id="ret"
        className="seq"
        title="Connect a step here to return it"
      />
      <div className="fnode-head">
        <FileOutput size={14} />
        <span className="font-semibold">Returns to the agent</span>
      </div>
      {!o && <div className="fnode-empty">Connect a step's right edge here.</div>}
      {o && "code" in o && <div className="fnode-sub mono truncate">{o.code}</div>}
      {o && "step" in o && (
        <>
          <div className="fnode-sub mono">
            {fields.length ? `pick(${o.step}, ${fields.length} fields)` : `${o.step} (everything)`}
          </div>
          {fields.slice(0, 8).map((f) => (
            <div key={f} className="fport">
              <span className="mono truncate">{f}</span>
            </div>
          ))}
          {fields.length > 8 && <div className="fnode-empty">+{fields.length - 8} more</div>}
          {est !== null && (
            <div className="fnode-foot">
              ≈ {num(est)} tokens{full && full > est ? ` of ${num(full)}` : ""}
              {est > 2000 && <Chip tone="warn">large</Chip>}
            </div>
          )}
        </>
      )}
      {run && !run.error && (
        <div className="fnode-foot">
          Last run: {num(run.outputTokens)} tok returned of {num(run.upstreamTokens)} fetched
        </div>
      )}
    </div>
  );
}

// Not "input"/"output": those are React Flow's built-in types, with their own styles.
const nodeTypes = { toolInput: InputNode, step: StepNode, toolOutput: OutputNode };

// ───────── Builder ─────────

type Props = {
  flow: Flow;
  onChange: (f: Flow) => void;
  apis: APIDetail[];
  run?: RunResult | null;
};

export function FlowBuilder(props: Props) {
  return (
    <ReactFlowProvider>
      <Builder {...props} />
    </ReactFlowProvider>
  );
}

const COL = 320;

/** layout places the tool's input, its steps left to right in run order
 *  (parallel steps share a column) and the output, using measured heights. */
export function layout(flow: Flow, height: (id: string) => number): Record<string, { x: number; y: number }> {
  const pos: Record<string, { x: number; y: number }> = { input: { x: 0, y: 40 } };
  let col = 0;
  for (let i = 0; i < flow.steps.length; i++) {
    const s = flow.steps[i];
    col++;
    let y = 20;
    const members = [s];
    while (s.group !== undefined && flow.steps[i + 1]?.group === s.group) members.push(flow.steps[++i]);
    for (const m of members) {
      pos[`s:${m.id}`] = { x: col * COL - 40, y };
      y += height(`s:${m.id}`) + 28;
    }
  }
  pos.output = { x: (col + 1) * COL - 40, y: 40 };
  return pos;
}

function Builder({ flow, onChange, apis, run }: Props) {
  const [selected, setSelected] = useState<string>("input");
  // Positions someone dragged a node to; everything else follows the layout.
  const [pos, setPos] = useState<Record<string, { x: number; y: number }>>({});
  const [filter, setFilter] = useState("");
  const [left, setLeft] = useState<"steps" | "add">(flow.steps.length ? "steps" : "add");
  const [full, setFull] = useState(false);
  const [legend, setLegend] = useState(false);
  const [animate, setAnimate] = useState(false);
  const [sizes, setSizes] = useState<Record<string, { width: number; height: number }>>({});
  const toast = useToast();
  const rf = useReactFlow();
  const reqOf = (s: Step) => {
    const api = apis.find((a) => a.name === s.api);
    const r = api?.requests.find((x) => x.name === s.req);
    return api && r ? { api, r } : null;
  };
  const ctx: Ctx = { flow, reqOf, run, selected, select: setSelected };

  useEffect(() => {
    if (!full) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setFull(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [full]);
  useEffect(() => {
    const t = setTimeout(() => rf.fitView({ padding: 0.15, duration: 250, maxZoom: 1 }), 60);
    return () => clearTimeout(t);
  }, [full, rf]);

  /** tidy snaps every node back to the layout, with a short animation. */
  const tidy = (fit = false) => {
    setAnimate(true);
    setPos({});
    setTimeout(() => setAnimate(false), 320);
    if (fit) setTimeout(() => rf.fitView({ padding: 0.15, duration: 250, maxZoom: 1 }), 40);
  };

  const reorder = (from: number, to: number) => {
    const r = moveStep(flow, from, to);
    if ("error" in r) {
      toast(r.error, true);
      tidy();
      return false;
    }
    onChange(r.flow);
    tidy();
    return true;
  };

  const addStep = (apiName: string, reqName: string, at?: number) => {
    const api = apis.find((a) => a.name === apiName);
    const r = api?.requests.find((x) => x.name === reqName);
    if (!api || !r) return;
    const taken = [
      ...flow.steps.flatMap((s) => [s.id, ...(s.kind === "code" ? codeDefines(s.code ?? "") : [])]),
      ...flow.inputs.map((i) => i.name),
    ];
    const id = stepId(reqName, taken);
    const step: Step = { id, api: apiName, req: reqName, args: [] };
    const idx = at === undefined ? flow.steps.length : at;
    const before = flow.steps
      .slice(0, idx)
      .filter((s) => s.kind !== "code")
      .map((s) => {
        const f = reqOf(s);
        return { step: s, fields: f ? fieldsOf(exampleOf(f.r), "wire", 200) : [] };
      });
    const w = autoWire(step, requestInfo(api, r), before, flow.inputs, api.auth?.type ?? "none");
    step.args = w.args;
    const steps = [...flow.steps];
    steps.splice(idx, 0, step);
    const last = idx === flow.steps.length;
    const next: Flow = {
      ...flow,
      inputs: [...flow.inputs, ...w.newInputs],
      steps,
      output: last && (!flow.output || "step" in flow.output) ? { step: id, fields: [] } : flow.output,
    };
    if (!next.meta.name) next.meta = { ...next.meta, name: reqName.replace(/([A-Z])/g, "_$1").toLowerCase() };
    onChange(next);
    setSelected(`s:${id}`);
    setLeft("steps");
    tidy();
  };

  const auto = useMemo(() => layout(flow, (id) => sizes[id]?.height ?? 240), [flow, sizes]);
  // Fit once every card has been measured, so nothing starts cut off.
  const allMeasured = Object.keys(sizes).length >= flow.steps.length + 2;
  useEffect(() => {
    if (allMeasured) rf.fitView({ padding: 0.12, maxZoom: 1 });
  }, [allMeasured, rf]);

  const nodes: Node[] = useMemo(() => {
    const out: Node[] = [{ id: "input", type: "toolInput", position: pos.input ?? auto.input, data: {} }];
    flow.steps.forEach((s, i) => {
      const nid = `s:${s.id}`;
      out.push({ id: nid, type: "step", position: pos[nid] ?? auto[nid], data: { index: i } });
    });
    out.push({ id: "output", type: "toolOutput", position: pos.output ?? auto.output, data: {} });
    // Controlled nodes have to carry their measured size, or xyflow keeps them hidden.
    for (const n of out) if (sizes[n.id]) n.measured = sizes[n.id];
    return out;
  }, [flow, pos, auto, sizes]);

  const edges: Edge[] = useMemo(() => {
    const out: Edge[] = [];
    const ids = flow.steps.map((s) => s.id);
    flow.steps.forEach((s, i) => {
      const prev = flow.steps[i - 1];
      if (i > 0 && !(s.group !== undefined && prev.group === s.group))
        out.push({
          id: `seq:${i}`,
          source: `s:${prev.id}`,
          sourceHandle: "all",
          target: `s:${s.id}`,
          targetHandle: "seq",
          className: "seq-edge",
        });
      for (const a of s.args) {
        const th = `${a.header ? "hdr" : "arg"}:${a.name}`;
        for (const f of sourceInputs(a.source))
          out.push({
            id: `${s.id}:${th}:in:${f}`,
            source: "input",
            sourceHandle: `in:${f}`,
            target: `s:${s.id}`,
            targetHandle: th,
            className: a.source.kind === "input" ? "wire-edge" : "wire-edge tmpl",
          });
        for (const r of sourceSteps(a.source)) {
          if (!ids.includes(r.step)) continue;
          out.push({
            id: `${s.id}:${th}:${r.step}:${r.path}`,
            source: `s:${r.step}`,
            sourceHandle: `out:${r.path}`,
            target: `s:${s.id}`,
            targetHandle: th,
            className: a.source.kind === "step" ? "wire-edge" : "wire-edge tmpl",
          });
        }
      }
    });
    const o = flow.output;
    const last = flow.steps[flow.steps.length - 1];
    if (o && "step" in o && ids.includes(o.step))
      out.push({
        id: "ret",
        source: `s:${o.step}`,
        sourceHandle: "all",
        target: "output",
        targetHandle: "ret",
        className: "ret-edge",
      });
    else if (o && "code" in o && last)
      out.push({
        id: "ret",
        source: `s:${last.id}`,
        sourceHandle: "all",
        target: "output",
        targetHandle: "ret",
        className: "ret-edge",
      });
    return out;
  }, [flow]);

  const onConnect = (c: Connection) => {
    if (c.target === "output" && c.source?.startsWith("s:")) {
      const sid = c.source.slice(2);
      onChange({ ...flow, output: { step: sid, fields: [] } });
      return;
    }
    if (!c.target?.startsWith("s:") || !c.targetHandle || !c.sourceHandle) return;
    const tid = c.target.slice(2);
    const ti = flow.steps.findIndex((s) => s.id === tid);
    let src: Source | null = null;
    if (c.source === "input" && c.sourceHandle.startsWith("in:"))
      src = { kind: "input", field: c.sourceHandle.slice(3) };
    if (c.source?.startsWith("s:") && c.sourceHandle.startsWith("out:")) {
      const sid = c.source.slice(2);
      if (flow.steps.findIndex((s) => s.id === sid) >= ti) {
        toast("Connect a field to a step that runs later.", true);
        return;
      }
      src = { kind: "step", step: sid, path: c.sourceHandle.slice(4) };
    }
    if (!src) return;
    const header = c.targetHandle.startsWith("hdr:");
    const name = c.targetHandle.slice(4);
    if (header && name.toLowerCase() === "authorization" && src.kind === "step")
      src = { kind: "template", text: `Bearer \${${sourceCode(src)}}` };
    setArg(ti, { name, header, source: src });
  };

  const setArg = (ti: number, a: Arg) => {
    const steps = flow.steps.map((s, i) => {
      if (i !== ti) return s;
      const k = (x: Arg) => `${x.header ? "h" : "a"}:${x.name}`;
      const has = s.args.some((x) => k(x) === k(a));
      return { ...s, args: has ? s.args.map((x) => (k(x) === k(a) ? a : x)) : [...s.args, a] };
    });
    onChange({ ...flow, steps });
  };

  /** Where a node dropped at x lands in the run order. */
  const indexAt = (x: number, skip?: string) => {
    const xs = flow.steps
      .map((s, i) => ({ i, id: `s:${s.id}`, x: (pos[`s:${s.id}`] ?? auto[`s:${s.id}`]).x }))
      .filter((n) => n.id !== skip);
    let k = 0;
    for (const n of xs) if (x > n.x) k++;
    return k;
  };

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    const v = e.dataTransfer.getData("application/x-mcpdeck-request");
    if (!v) return;
    const [api, req] = v.split(".");
    addStep(api, req, indexAt(rf.screenToFlowPosition({ x: e.clientX, y: e.clientY }).x));
  };

  const shown = apis
    .map((a) => ({
      a,
      reqs: a.requests.filter((r) =>
        `${a.name}.${r.name} ${r.title} ${r.url}`.toLowerCase().includes(filter.toLowerCase()),
      ),
    }))
    .filter((x) => x.reqs.length > 0);

  return (
    <FlowCtx.Provider value={ctx}>
      <div className={`flow-shell${full ? " full" : ""}`}>
        <aside className="flow-palette">
          <div className="flow-palette-tabs" role="tablist" aria-label="Left panel">
            <button type="button" role="tab" aria-selected={left === "steps"} onClick={() => setLeft("steps")}>
              Steps <span className="faint">{flow.steps.length}</span>
            </button>
            <button type="button" role="tab" aria-selected={left === "add"} onClick={() => setLeft("add")}>
              <Plus size={12} /> Add request
            </button>
          </div>
          {left === "steps" ? (
            <StepList
              flow={flow}
              selected={selected}
              onSelect={setSelected}
              onMove={reorder}
              onAdd={() => setLeft("add")}
            />
          ) : (
            <>
              <div className="flow-palette-head">
                <div className="muted text-[12px]">
                  Drag onto the canvas where it should run, or click + to add it last.
                </div>
                <div className="relative mt-2">
                  <Search size={13} className="muted absolute top-1/2 left-2.5 -translate-y-1/2" />
                  <input
                    className="input"
                    style={{ height: 30, paddingLeft: 28 }}
                    placeholder="Filter"
                    aria-label="Filter requests"
                    value={filter}
                    onChange={(e) => setFilter(e.target.value)}
                  />
                </div>
              </div>
              <div className="flow-palette-list">
                {apis.length === 0 && <div className="muted p-3 text-[12.5px]">Add an application first.</div>}
                {shown.map(({ a, reqs }) => (
                  <div key={a.id} className="mb-2">
                    <div className="px-2 py-1 text-[11.5px] font-semibold uppercase tracking-wide muted">
                      {a.title || a.name}
                    </div>
                    {reqs.map((r) => {
                      const saved = (r.responses?.length ?? 0) > 0 || !!r.example;
                      return (
                        <div
                          key={r.id}
                          className="flow-req"
                          draggable
                          onDragStart={(e) => {
                            e.dataTransfer.setData("application/x-mcpdeck-request", `${a.name}.${r.name}`);
                            e.dataTransfer.effectAllowed = "copy";
                          }}
                          title={
                            saved
                              ? `${a.name}.${r.name}`
                              : `${a.name}.${r.name}: no saved response yet, so its fields are unknown`
                          }
                        >
                          <GripVertical size={12} className="faint flex-none" />
                          <MethodChip method={r.method} />
                          <span className="mono min-w-0 flex-1 truncate">{r.name}</span>
                          {!saved && <span className="dot warn" />}
                          <button
                            type="button"
                            className="icon-btn"
                            style={{ width: 24, height: 24 }}
                            aria-label={`Add ${a.name}.${r.name} as a step`}
                            onClick={() => addStep(a.name, r.name)}
                          >
                            <Plus size={13} />
                          </button>
                        </div>
                      );
                    })}
                  </div>
                ))}
              </div>
            </>
          )}
        </aside>
        <div
          className={`flow-canvas${animate ? " flow-animate" : ""}`}
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
        >
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={(ch) => {
              for (const c of ch) {
                if (c.type === "position" && c.position) setPos((p) => ({ ...p, [c.id]: c.position! }));
                if (c.type === "select" && c.selected) setSelected(c.id);
                if (c.type === "dimensions" && c.dimensions) {
                  const d = c.dimensions;
                  setSizes((m) =>
                    m[c.id]?.width === d.width && m[c.id]?.height === d.height ? m : { ...m, [c.id]: d },
                  );
                }
              }
            }}
            onNodeDragStop={(_, n) => {
              // Dragging a step past another changes the run order.
              if (!n.id.startsWith("s:")) return;
              const from = flow.steps.findIndex((s) => `s:${s.id}` === n.id);
              const to = indexAt(n.position.x, n.id);
              if (from >= 0 && to !== from) reorder(from, to);
              else tidy();
            }}
            onConnect={onConnect}
            onNodeClick={(_, n) => setSelected(n.id)}
            onPaneClick={() => setSelected("")}
            defaultViewport={{ x: 16, y: 20, zoom: 0.72 }}
            minZoom={0.3}
            proOptions={{ hideAttribution: true }}
            deleteKeyCode={null}
          >
            <Background gap={18} size={1} />
            <Controls showInteractive={false} />
            <Panel position="top-right" className="flow-tools">
              <Button
                size="sm"
                icon={<LayoutGrid size={13} />}
                onClick={() => tidy(true)}
                title="Line steps up in run order"
              >
                Arrange
              </Button>
              <Button
                size="sm"
                icon={<CircleHelp size={13} />}
                aria-pressed={legend}
                onClick={() => setLegend(!legend)}
              >
                Legend
              </Button>
              <Button
                size="sm"
                icon={full ? <Minimize2 size={13} /> : <Maximize2 size={13} />}
                onClick={() => setFull(!full)}
                title={full ? "Exit full screen (Esc)" : "Full screen"}
              >
                {full ? "Exit full screen" : "Full screen"}
              </Button>
            </Panel>
            {legend && (
              <Panel position="bottom-right" className="flow-legend" aria-label="Legend">
                <div className="font-semibold">Reading the canvas</div>
                <div>
                  <span className="lg-dot seq" /> Gray dots on a card's top corners: run order. Steps run left to right.
                </div>
                <div>
                  <span className="lg-dot port" /> Blue rings: fields. Left of a card, what it needs; right, what it
                  returns.
                </div>
                <div>
                  <span className="lg-line solid" /> Solid line: passes a value as it is.
                </div>
                <div>
                  <span className="lg-line dashed" /> Dashed line: used inside text, like{" "}
                  <code>Bearer ${"{token}"}</code>.
                </div>
                <div>
                  <span className="lg-line dotted" /> Dotted gray line: the next step in run order.
                </div>
                <div>
                  <span className="lg-line ret" /> Green line: what the tool returns.
                </div>
                <div className="muted">Drag a card left or right to change when it runs.</div>
              </Panel>
            )}
          </ReactFlow>
          {flow.steps.length === 0 && (
            <div className="flow-hint">
              <CornerDownRight size={16} />
              Drag a request here to make the first step. Then add the next one: mcpDeck connects fields with the same
              name, like an <code>id</code> from a list into the next request's <code>appId</code>.
            </div>
          )}
        </div>
        <aside className="flow-inspector">
          <Inspector
            selected={selected}
            flow={flow}
            onChange={onChange}
            reqOf={reqOf}
            setArg={setArg}
            onSelect={setSelected}
            onMove={reorder}
          />
        </aside>
      </div>
    </FlowCtx.Provider>
  );
}

function SortableStep({ s, i, selected, onSelect }: { s: Step; i: number; selected: boolean; onSelect: () => void }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: s.id });
  return (
    <li
      ref={setNodeRef}
      className={`step-row${selected ? " sel" : ""}${isDragging ? " dragging" : ""}`}
      style={{ transform: CSS.Transform.toString(transform), transition }}
    >
      <button type="button" className="step-grip" aria-label={`Move ${s.id}`} {...attributes} {...listeners}>
        <GripVertical size={13} />
      </button>
      <button type="button" className="step-main" onClick={onSelect}>
        <span className="fnode-n">{i + 1}</span>
        {s.kind === "code" ? (
          <>
            <Braces size={13} className="muted flex-none" />
            <span className="min-w-0 flex-1 truncate">Custom code</span>
          </>
        ) : (
          <>
            <span className="mono min-w-0 flex-1 truncate">
              {s.api}.{s.req}
            </span>
            {s.group !== undefined && <Columns3 size={12} className="muted flex-none" aria-label="parallel" />}
          </>
        )}
      </button>
    </li>
  );
}

/** StepList is the run order as a list you can drag (or move with the keyboard: Space, arrows, Space). */
function StepList({
  flow,
  selected,
  onSelect,
  onMove,
  onAdd,
}: {
  flow: Flow;
  selected: string;
  onSelect: (id: string) => void;
  onMove: (from: number, to: number) => boolean;
  onAdd: () => void;
}) {
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const onDragEnd = (e: DragEndEvent) => {
    if (!e.over || e.active.id === e.over.id) return;
    const from = flow.steps.findIndex((s) => s.id === e.active.id);
    const to = flow.steps.findIndex((s) => s.id === e.over!.id);
    if (from >= 0 && to >= 0) onMove(from, to);
  };
  return (
    <div className="flow-palette-list">
      <div className="muted px-2 pb-2 text-[12px]">Run order, top to bottom. Drag the handle to change it.</div>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <SortableContext items={flow.steps.map((s) => s.id)} strategy={verticalListSortingStrategy}>
          <ol className="step-list">
            {flow.steps.map((s, i) => (
              <SortableStep
                key={s.id}
                s={s}
                i={i}
                selected={selected === `s:${s.id}`}
                onSelect={() => onSelect(`s:${s.id}`)}
              />
            ))}
          </ol>
        </SortableContext>
      </DndContext>
      {flow.steps.length === 0 && <div className="muted px-2 text-[12.5px]">No steps yet.</div>}
      <Button size="sm" kind="ghost" className="mt-2" icon={<Plus size={13} />} onClick={onAdd}>
        Add a request
      </Button>
    </div>
  );
}

// ───────── Inspector ─────────

function Inspector({
  selected,
  flow,
  onChange,
  reqOf,
  setArg,
  onSelect,
  onMove,
}: {
  selected: string;
  flow: Flow;
  onChange: (f: Flow) => void;
  reqOf: Ctx["reqOf"];
  setArg: (ti: number, a: Arg) => void;
  onSelect: (id: string) => void;
  onMove: (from: number, to: number) => boolean;
}) {
  if (selected === "input" || !selected) return <ToolInspector flow={flow} onChange={onChange} />;
  if (selected === "output") return <OutputInspector flow={flow} onChange={onChange} reqOf={reqOf} />;
  const idx = flow.steps.findIndex((s) => `s:${s.id}` === selected);
  if (idx < 0) return <ToolInspector flow={flow} onChange={onChange} />;
  if (flow.steps[idx].kind === "code")
    return <CodeInspector idx={idx} flow={flow} onChange={onChange} onSelect={onSelect} onMove={onMove} />;
  return (
    <StepInspector
      idx={idx}
      flow={flow}
      onChange={onChange}
      reqOf={reqOf}
      setArg={setArg}
      onSelect={onSelect}
      onMove={onMove}
    />
  );
}

function CodeInspector({
  idx,
  flow,
  onChange,
  onSelect,
  onMove,
}: {
  idx: number;
  flow: Flow;
  onChange: (f: Flow) => void;
  onSelect: (id: string) => void;
  onMove: (from: number, to: number) => boolean;
}) {
  const s = flow.steps[idx];
  const update = (p: Partial<Step>) =>
    onChange({ ...flow, steps: flow.steps.map((x, i) => (i === idx ? { ...x, ...p } : x)) });
  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-start gap-2">
        <span className="fnode-n">{idx + 1}</span>
        <div className="min-w-0 flex-1">
          <div className="font-semibold">Custom code</div>
          <div className="muted text-[12px]">
            Logic the canvas has no card for, like an if, a loop or a calculation. It runs in order with the steps.
          </div>
        </div>
        <Button
          size="sm"
          kind="ghost"
          aria-label="Move earlier"
          icon={<ArrowUp size={13} />}
          disabled={idx === 0}
          onClick={() => onMove(idx, idx - 1)}
        />
        <Button
          size="sm"
          kind="ghost"
          aria-label="Move later"
          icon={<ArrowDown size={13} />}
          disabled={idx === flow.steps.length - 1}
          onClick={() => onMove(idx, idx + 1)}
        />
      </div>
      <Field label="Code" hint="TypeScript, as in the Code view">
        <textarea
          className="textarea mono"
          rows={Math.min(18, Math.max(4, (s.code ?? "").split("\n").length + 1))}
          spellCheck={false}
          value={s.code ?? ""}
          onChange={(e) => update({ code: e.target.value })}
        />
      </Field>
      <Field label="Note" hint="Shown above the code as a comment">
        <input
          className="input"
          value={s.comment ?? ""}
          onChange={(e) => update({ comment: e.target.value || undefined })}
        />
      </Field>
      <div className="flex pt-1">
        <Button
          size="sm"
          kind="bad"
          className="ml-auto"
          icon={<Trash2 size={13} />}
          onClick={() => {
            onChange({ ...flow, steps: flow.steps.filter((_, i) => i !== idx) });
            onSelect("input");
          }}
        >
          Remove code
        </Button>
      </div>
    </div>
  );
}

function ToolInspector({ flow, onChange }: { flow: Flow; onChange: (f: Flow) => void }) {
  const m = flow.meta;
  const setMeta = (p: Partial<typeof m>) => onChange({ ...flow, meta: { ...m, ...p } });
  const setInput = (i: number, p: Partial<ToolInput>) => {
    const old = flow.inputs[i];
    const inputs = flow.inputs.map((x, j) => (j === i ? { ...x, ...p } : x));
    let steps = flow.steps;
    if (p.name && p.name !== old.name) {
      // Renaming an input renames its uses.
      const ren = (s: Source): Source =>
        s.kind === "input" && s.field === old.name
          ? { ...s, field: p.name! }
          : s.kind === "template"
            ? { ...s, text: s.text.replaceAll(`input.${old.name}`, `input.${p.name}`) }
            : s;
      steps = steps.map((st) => ({ ...st, args: st.args.map((a) => ({ ...a, source: ren(a.source) })) }));
    }
    onChange({ ...flow, inputs, steps });
  };
  return (
    <div className="flex flex-col gap-4 p-4">
      <div>
        <div className="mb-2 flex items-center">
          <div>
            <div className="insp-title">Agent inputs</div>
            <div className="muted text-[12px]">What an agent passes when it calls this tool.</div>
          </div>
          <Button
            size="sm"
            kind="ghost"
            className="ml-auto"
            icon={<Plus size={13} />}
            onClick={() =>
              onChange({
                ...flow,
                inputs: [
                  ...flow.inputs,
                  { name: `input_${flow.inputs.length + 1}`, type: "string", description: "", required: true },
                ],
              })
            }
          >
            Add
          </Button>
        </div>
        {flow.inputs.length === 0 && (
          <div className="muted text-[12.5px]">
            None yet. Add one for each value the agent should choose, like an id.
          </div>
        )}
        {flow.inputs.map((i, n) => (
          <div key={n} className="insp-card">
            <div className="flex gap-2">
              <input
                className="input mono"
                aria-label="Input name"
                value={i.name}
                onChange={(e) => setInput(n, { name: e.target.value.replace(/[^\w$]/g, "_") })}
              />
              <Select
                label="Type"
                size="sm"
                value={i.type}
                onChange={(v) => setInput(n, { type: v })}
                options={["string", "integer", "number", "boolean", "array", "object"].map((t) => ({
                  value: t,
                  label: t,
                }))}
              />
            </div>
            <input
              className="input mt-2"
              aria-label="Input description"
              placeholder="Description, with an example"
              value={i.description}
              onChange={(e) => setInput(n, { description: e.target.value })}
            />
            <div className="mt-2 flex items-center gap-2 text-[12.5px]">
              <Toggle label="Required" checked={i.required} onChange={(v) => setInput(n, { required: v })} /> Required
              <Button
                size="sm"
                kind="ghost"
                className="ml-auto"
                aria-label={`Remove input ${i.name}`}
                icon={<Trash2 size={13} />}
                onClick={() => onChange({ ...flow, inputs: flow.inputs.filter((_, j) => j !== n) })}
              />
            </div>
          </div>
        ))}
      </div>
      <div>
        <div className="insp-title">About the tool</div>
        <div className="muted text-[12px]">What agents see in tools/list.</div>
      </div>
      <Field label="Name" hint="Verb and object, like find_vulnerabilities">
        <input className="input mono" value={m.name} onChange={(e) => setMeta({ name: e.target.value })} />
      </Field>
      <Field label="Description" hint="What it returns, in one sentence">
        <textarea
          className="textarea prose"
          rows={3}
          placeholder="What it returns, and for which input. For example: the open vulnerabilities of one application, with CVE and fixed version."
          value={m.description}
          onChange={(e) => setMeta({ description: e.target.value })}
        />
      </Field>
      <Field label="Use when">
        <input
          className="input"
          placeholder="The question this tool answers"
          value={m.useWhen}
          onChange={(e) => setMeta({ useWhen: e.target.value })}
        />
      </Field>
      <Field label="Don't use when">
        <input className="input" value={m.dontUseWhen} onChange={(e) => setMeta({ dontUseWhen: e.target.value })} />
      </Field>
    </div>
  );
}

type SrcKind = "input" | "step" | "literal" | "template";

function SourceEditor({
  value,
  onChange,
  flow,
  before,
  reqOf,
}: {
  value: Source;
  onChange: (s: Source) => void;
  flow: Flow;
  before: Step[];
  reqOf: Ctx["reqOf"];
}) {
  const kind: SrcKind = value.kind === "expr" ? "template" : value.kind;
  const stepFields = before.flatMap((s) => {
    const f = reqOf(s);
    return (f ? fieldsOf(exampleOf(f.r), "wire", 200) : [])
      .filter((x) => x.type !== "array")
      .map((x) => ({ value: `${s.id}|${x.path}`, label: `${s.id}.${x.path}`, hint: x.sample }));
  });
  return (
    <div className="flex flex-col gap-1.5">
      <Select<SrcKind>
        label="Comes from"
        size="sm"
        value={kind}
        onChange={(k) => {
          if (k === "input") onChange({ kind: "input", field: flow.inputs[0]?.name ?? "" });
          if (k === "step") {
            const [st, p] = (stepFields[0]?.value ?? "|").split("|");
            onChange({ kind: "step", step: st, path: p });
          }
          if (k === "literal") onChange({ kind: "literal", value: "" });
          if (k === "template")
            onChange({
              kind: "template",
              text: value.kind === "template" ? value.text : sourceCode(value).replace(/^`|`$/g, ""),
            });
        }}
        options={[
          { value: "input", label: "Agent input" },
          { value: "step", label: "An earlier step", hint: stepFields.length ? undefined : "No earlier fields known" },
          { value: "literal", label: "Fixed value" },
          { value: "template", label: "Text with ${…}" },
        ]}
      />
      {value.kind === "input" && (
        <Select
          label="Input"
          size="sm"
          value={value.field}
          placeholder="Choose an input"
          onChange={(f) => onChange({ kind: "input", field: f })}
          options={flow.inputs.map((i) => ({ value: i.name, label: i.name }))}
        />
      )}
      {value.kind === "step" && (
        <Select
          label="Field"
          size="sm"
          value={`${value.step}|${value.path}`}
          placeholder="Choose a field"
          onChange={(v) => {
            const [st, p] = v.split("|");
            onChange({ kind: "step", step: st, path: p });
          }}
          options={
            stepFields.length
              ? stepFields
              : [{ value: `${value.step}|${value.path}`, label: `${value.step}.${value.path}` }]
          }
        />
      )}
      {value.kind === "literal" && (
        <input
          className="input mono"
          aria-label="Value"
          value={String(value.value)}
          onChange={(e) => onChange({ kind: "literal", value: e.target.value })}
        />
      )}
      {(value.kind === "template" || value.kind === "expr") && (
        <input
          className="input mono"
          aria-label="Template"
          value={value.kind === "template" ? value.text : value.code}
          onChange={(e) =>
            onChange(
              value.kind === "template"
                ? { kind: "template", text: e.target.value }
                : { kind: "expr", code: e.target.value },
            )
          }
        />
      )}
    </div>
  );
}

function StepInspector({
  idx,
  flow,
  onChange,
  reqOf,
  setArg,
  onSelect,
  onMove,
}: {
  idx: number;
  flow: Flow;
  onChange: (f: Flow) => void;
  reqOf: Ctx["reqOf"];
  setArg: (ti: number, a: Arg) => void;
  onSelect: (id: string) => void;
  onMove: (from: number, to: number) => boolean;
}) {
  const s = flow.steps[idx];
  const found = reqOf(s);
  const params = found ? argsOf(requestInfo(found.api, found.r)) : [];
  const unused = params.filter((p) => !s.args.some((a) => !a.header && a.name === p.name));
  const before = flow.steps.slice(0, idx);
  const [hdr, setHdr] = useState("");
  const update = (st: Step) => onChange({ ...flow, steps: flow.steps.map((x, i) => (i === idx ? st : x)) });
  const move = (d: -1 | 1) => {
    const j = idx + d;
    if (j >= 0 && j < flow.steps.length) onMove(idx, j);
  };
  const usedLater = flow.steps
    .slice(idx + 1)
    .some((x) => x.args.some((a) => sourceSteps(a.source).some((r) => r.step === s.id)));
  const login = found?.api.auth?.type !== "login" && /login|token|auth/i.test(s.req);
  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-start gap-2">
        <span className="fnode-n">{idx + 1}</span>
        <div className="min-w-0 flex-1">
          <div className="mono truncate font-semibold">
            {s.api}.{s.req}
          </div>
          <div className="muted truncate text-[12px]">{found?.r.title || found?.r.url}</div>
        </div>
        <Button
          size="sm"
          kind="ghost"
          aria-label="Move earlier"
          icon={<ArrowUp size={13} />}
          disabled={idx === 0}
          onClick={() => move(-1)}
        />
        <Button
          size="sm"
          kind="ghost"
          aria-label="Move later"
          icon={<ArrowDown size={13} />}
          disabled={idx === flow.steps.length - 1}
          onClick={() => move(1)}
        />
      </div>
      <div className="flex items-center gap-2 text-[12.5px]">
        <Toggle
          label="Keep the result"
          checked={!s.discard}
          onChange={(v) => update(v ? { ...s, discard: undefined } : { ...s, discard: true })}
        />
        Keep the result for later steps
      </div>
      {s.group !== undefined && (
        <div className="flex items-center gap-2 text-[12.5px]">
          <Toggle label="Run in parallel" checked onChange={() => update({ ...s, group: undefined })} />
          Runs at the same time as the steps next to it
        </div>
      )}
      <Field label="Step name" hint="The variable later steps read from">
        <input
          className="input mono"
          value={s.id}
          onChange={(e) => {
            const id = e.target.value.replace(/[^\w$]/g, "");
            if (!id || flow.steps.some((x, i) => i !== idx && x.id === id)) return;
            const ren = (src: Source): Source =>
              src.kind === "step" && src.step === s.id
                ? { ...src, step: id }
                : src.kind === "template"
                  ? { ...src, text: src.text.replace(new RegExp(`\\$\\{${s.id}([.[])`, "g"), `\${${id}$1`) }
                  : src;
            const steps = flow.steps.map((x, i) =>
              i === idx ? { ...x, id } : { ...x, args: x.args.map((a) => ({ ...a, source: ren(a.source) })) },
            );
            const o =
              flow.output && "step" in flow.output && flow.output.step === s.id
                ? { ...flow.output, step: id }
                : flow.output;
            onChange({ ...flow, steps, output: o });
            onSelect(`s:${id}`);
          }}
        />
      </Field>
      <div>
        <div className="insp-title mb-2">Parameters</div>
        {s.args.filter((a) => !a.header).length === 0 && unused.length === 0 && (
          <div className="muted text-[12.5px]">This request takes no parameters.</div>
        )}
        {s.args.map((a) =>
          a.header ? null : (
            <div key={a.name} className="insp-card">
              <div className="mb-1.5 flex items-center gap-2">
                <span className="mono font-semibold">{a.name}</span>
                <span className="faint text-[11.5px]">{params.find((p) => p.name === a.name)?.where ?? "param"}</span>
                <Button
                  size="sm"
                  kind="ghost"
                  className="ml-auto"
                  aria-label={`Remove ${a.name}`}
                  icon={<Trash2 size={13} />}
                  onClick={() => update({ ...s, args: s.args.filter((x) => x !== a) })}
                />
              </div>
              <SourceEditor
                value={a.source}
                flow={flow}
                before={before}
                reqOf={reqOf}
                onChange={(src) => setArg(idx, { ...a, source: src })}
              />
            </div>
          ),
        )}
        {unused.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {unused.map((p) => (
              <button
                key={p.name}
                type="button"
                className="saved-resp"
                onClick={() =>
                  setArg(idx, { name: p.name, source: { kind: "input", field: flow.inputs[0]?.name ?? p.name } })
                }
              >
                <Plus size={12} /> <span className="mono">{p.name}</span>
                <span className="faint">{p.where}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      <div>
        <div className="insp-title mb-2">Headers</div>
        {s.args.map((a) =>
          a.header ? (
            <div key={`h:${a.name}`} className="insp-card">
              <div className="mb-1.5 flex items-center gap-2">
                <KeyRound size={12} className="muted" />
                <span className="mono font-semibold">{a.name}</span>
                <Button
                  size="sm"
                  kind="ghost"
                  className="ml-auto"
                  aria-label={`Remove header ${a.name}`}
                  icon={<Trash2 size={13} />}
                  onClick={() => update({ ...s, args: s.args.filter((x) => x !== a) })}
                />
              </div>
              <SourceEditor
                value={a.source}
                flow={flow}
                before={before}
                reqOf={reqOf}
                onChange={(src) => setArg(idx, { ...a, source: src })}
              />
            </div>
          ) : null,
        )}
        <form
          className="flex gap-1.5"
          onSubmit={(e) => {
            e.preventDefault();
            if (!hdr.trim()) return;
            setArg(idx, { name: hdr.trim(), header: true, source: { kind: "template", text: "" } });
            setHdr("");
          }}
        >
          <input
            className="input"
            placeholder="Header name, like Authorization"
            aria-label="New header"
            value={hdr}
            onChange={(e) => setHdr(e.target.value)}
          />
          <Button size="sm" type="submit" disabled={!hdr.trim()} aria-label="Add header" icon={<Plus size={13} />} />
        </form>
        {login && usedLater && (
          <div className="banner info mt-3 text-[12px]">
            Tip: set this application's auth to <strong>Login request</strong> with capture{" "}
            <code>json:access_token</code>. mcpDeck then logs in and adds the token itself, and no tool handles it.
          </div>
        )}
      </div>
      <div className="flex gap-2 pt-1">
        <Button
          size="sm"
          icon={<FileOutput size={13} />}
          onClick={() => {
            onChange({ ...flow, output: { step: s.id, fields: [] } });
            onSelect("output");
          }}
        >
          Return this step
        </Button>
        <Button
          size="sm"
          kind="bad"
          className="ml-auto"
          icon={<Trash2 size={13} />}
          onClick={() => {
            const steps = flow.steps.filter((_, i) => i !== idx);
            const o =
              flow.output && "step" in flow.output && flow.output.step === s.id
                ? steps.length
                  ? { step: steps[steps.length - 1].id, fields: [] }
                  : null
                : flow.output;
            onChange({ ...flow, steps, output: o });
            onSelect("input");
          }}
        >
          Remove step
        </Button>
      </div>
    </div>
  );
}

function OutputInspector({ flow, onChange, reqOf }: { flow: Flow; onChange: (f: Flow) => void; reqOf: Ctx["reqOf"] }) {
  const o = flow.output;
  if (o && "code" in o)
    return (
      <div className="flex flex-col gap-3 p-4">
        <div className="insp-title">Returns</div>
        <pre className="code">{o.code}</pre>
        <div className="muted text-[12.5px]">
          This return is custom code; edit it in Code, or pick a step to return here.
        </div>
        <Select
          label="Return a step"
          value=""
          placeholder="Return a step instead"
          onChange={(v) => onChange({ ...flow, output: { step: v, fields: [] } })}
          options={flow.steps.map((s) => ({ value: s.id, label: s.id }))}
        />
      </div>
    );
  const step = o ? flow.steps.find((s) => s.id === o.step) : undefined;
  const found = step ? reqOf(step) : null;
  const ex = found ? exampleOf(found.r) : undefined;
  const fields = ex !== undefined ? fieldsOf(ex, "pick", 300) : [];
  const picked = new Set(o?.fields ?? []);
  const full = ex !== undefined ? tokensOf(ex) : 0;
  const now = ex !== undefined ? tokensOf(pickValue(ex, [...picked])) : 0;
  const leaves = fields.filter((f) => f.type !== "object" && f.type !== "array");
  const toggle = (p: string, on: boolean) => {
    const next = new Set(picked);
    // Ticking a parent ticks its leaves; unticking removes them.
    const under = leaves.filter((f) => f.path === p || f.path.startsWith(`${p}.`) || f.path.startsWith(`${p}[]`));
    for (const f of under.length ? under : [{ path: p }]) on ? next.add(f.path) : next.delete(f.path);
    onChange({ ...flow, output: { step: o!.step, fields: leaves.map((f) => f.path).filter((x) => next.has(x)) } });
  };
  const size = (p: string) => {
    if (ex === undefined) return 0;
    return tokensOf(pickValue(ex, [p]));
  };
  return (
    <div className="flex flex-col gap-3 p-4">
      <div>
        <div className="insp-title">What the agent gets back</div>
        <div className="muted text-[12px]">Tick only what the task needs; every field costs tokens on every call.</div>
      </div>
      <Select
        label="Return step"
        value={o?.step ?? ""}
        placeholder="Choose a step"
        onChange={(v) => onChange({ ...flow, output: { step: v, fields: [] } })}
        options={flow.steps.map((s) => ({ value: s.id, label: `${s.id} · ${s.api}.${s.req}` }))}
      />
      {ex === undefined ? (
        <div className="banner warn text-[12.5px]">
          No saved response for this step yet. Send it in its application and save the 200 response; its fields show
          here.
        </div>
      ) : (
        <>
          <div className="panel p-3">
            <div className="flex items-baseline justify-between text-[12.5px]">
              <span>
                <strong>{num(picked.size ? now : full)}</strong> tokens per call
              </span>
              <span className="muted">of {num(full)} in the response</span>
            </div>
            <div className="meter mt-2">
              <span
                style={{
                  width: `${Math.min(100, ((picked.size ? now : full) / Math.max(1, full)) * 100)}%`,
                  background: (picked.size ? now : full) > 2000 ? "var(--warn-dot)" : "var(--accent)",
                }}
              />
            </div>
            {picked.size === 0 && (
              <div className="muted mt-2 text-[12px]">Returning everything. Tick fields to keep only those.</div>
            )}
          </div>
          <div className="flex gap-2">
            <Button size="sm" kind="ghost" onClick={() => onChange({ ...flow, output: { step: o!.step, fields: [] } })}>
              Return everything
            </Button>
          </div>
          <div className="field-tree">
            {fields.map((f) => {
              const depth = f.path.split(/\.|\[\]/).filter(Boolean).length - 1;
              const isLeaf = f.type !== "object" && f.type !== "array";
              const on = isLeaf
                ? picked.has(f.path)
                : leaves.some((l) => l.path.startsWith(f.path) && picked.has(l.path));
              return (
                <label key={f.path} className="field-row" style={{ paddingLeft: 6 + depth * 14 }}>
                  <input type="checkbox" checked={on} onChange={(e) => toggle(f.path, e.target.checked)} />
                  <span className="mono min-w-0 flex-1 truncate">{f.path.split(".").pop()}</span>
                  <span className="faint text-[11px]">{isLeaf ? f.type : f.type === "array" ? "list" : ""}</span>
                  {isLeaf && <span className="muted w-14 text-right text-[11px]">{num(size(f.path))} tok</span>}
                </label>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}
