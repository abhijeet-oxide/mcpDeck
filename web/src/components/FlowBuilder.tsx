import {
  Background,
  type Connection,
  Controls,
  type Edge,
  Handle,
  type Node,
  type NodeProps,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  ArrowDown,
  ArrowUp,
  CircleAlert,
  CornerDownRight,
  FileOutput,
  GripVertical,
  KeyRound,
  LogIn,
  Plus,
  Search,
  Trash2,
} from "lucide-react";
import { createContext, type DragEvent, memo, useContext, useMemo, useState } from "react";
import {
  type Arg,
  argsOf,
  autoWire,
  type FieldInfo,
  type Flow,
  fieldsOf,
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
import { Button, Chip, Field, MethodChip, Select, Toggle } from "./ui";

type Ctx = {
  flow: Flow;
  reqOf: (s: Step) => { api: APIDetail; r: Request } | null;
  run?: RunResult | null;
  selected: string;
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
  const { flow, selected } = useContext(FlowCtx);
  return (
    <div className={`fnode fnode-input${selected === id ? " sel" : ""}`}>
      <div className="fnode-head">
        <LogIn size={14} />
        <span className="font-semibold">Agent input</span>
        <span className="mono faint ml-auto truncate text-[11px]">{flow.meta.name}</span>
      </div>
      {flow.inputs.length === 0 && <div className="fnode-empty">No inputs. Click to add one.</div>}
      {flow.inputs.map((i) => (
        <div key={i.name} className="fport out">
          <span className="mono">{i.name}</span>
          <span className="faint">
            {i.type}
            {i.required ? "" : "?"}
          </span>
          <Handle type="source" position={Position.Right} id={`in:${i.name}`} />
        </div>
      ))}
    </div>
  );
}

function StepNode({ id, data }: NodeProps) {
  const { flow, reqOf, run, selected } = useContext(FlowCtx);
  const [more, setMore] = useState(false);
  const idx = (data as { index: number }).index;
  const s = flow.steps[idx];
  if (!s) return null;
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
      <Handle type="target" position={Position.Left} id="seq" className="seq" />
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
      <div className="fnode-sub mono">const {s.id}</div>
      {!found && (
        <div className="fnode-warn">
          No request {s.api}.{s.req} in your applications.
        </div>
      )}
      <div className="fnode-sec">Inputs</div>
      {s.args.length === 0 && params.length === 0 && <div className="fnode-empty">No parameters</div>}
      {s.args.map((a) => (
        <div key={(a.header ? "h:" : "") + a.name} className="fport in">
          <Handle type="target" position={Position.Left} id={`${a.header ? "hdr" : "arg"}:${a.name}`} />
          {a.header && <KeyRound size={11} className="muted flex-none" />}
          <span className="mono">{a.name}</span>
          <span className="faint min-w-0 truncate">← {describeSource(a.source)}</span>
        </div>
      ))}
      {missing.map((p) => (
        <div key={p.name} className="fport in miss">
          <Handle type="target" position={Position.Left} id={`arg:${p.name}`} />
          <CircleAlert size={11} className="flex-none" />
          <span className="mono">{p.name}</span>
          <span className="min-w-0 truncate">needs a value</span>
        </div>
      ))}
      <div className="fnode-sec">
        Output fields
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
      <Handle type="source" position={Position.Right} id="all" className="seq" />
    </div>
  );
}

const FieldPort = memo(function FieldPort({ f }: { f: FieldInfo }) {
  return (
    <div className="fport out" title={f.sample ? `e.g. ${f.sample}` : f.type}>
      <span className="mono min-w-0 truncate">{f.path}</span>
      <span className="faint">{f.type}</span>
      <Handle type="source" position={Position.Right} id={`out:${f.path}`} />
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
      <Handle type="target" position={Position.Left} id="ret" className="seq" />
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

function Builder({ flow, onChange, apis, run }: Props) {
  const [selected, setSelected] = useState<string>("input");
  const [pos, setPos] = useState<Record<string, { x: number; y: number }>>({});
  const [filter, setFilter] = useState("");
  const rf = useReactFlow();
  const reqOf = (s: Step) => {
    const api = apis.find((a) => a.name === s.api);
    const r = api?.requests.find((x) => x.name === s.req);
    return api && r ? { api, r } : null;
  };
  const ctx: Ctx = { flow, reqOf, run, selected };

  const addStep = (apiName: string, reqName: string, at?: { x: number; y: number }) => {
    const api = apis.find((a) => a.name === apiName);
    const r = api?.requests.find((x) => x.name === reqName);
    if (!api || !r) return;
    const id = stepId(reqName, [...flow.steps.map((s) => s.id), ...flow.inputs.map((i) => i.name)]);
    const step: Step = { id, api: apiName, req: reqName, args: [] };
    const before = flow.steps.map((s) => {
      const f = reqOf(s);
      return { step: s, fields: f ? fieldsOf(exampleOf(f.r), "wire", 200) : [] };
    });
    const w = autoWire(step, requestInfo(api, r), before, flow.inputs, api.auth?.type ?? "none");
    step.args = w.args;
    const next: Flow = {
      ...flow,
      inputs: [...flow.inputs, ...w.newInputs],
      steps: [...flow.steps, step],
      output: { step: id, fields: [] },
    };
    if (!next.meta.name) next.meta = { ...next.meta, name: reqName.replace(/([A-Z])/g, "_$1").toLowerCase() };
    if (at) setPos((p) => ({ ...p, [`s:${id}`]: at }));
    onChange(next);
    setSelected(`s:${id}`);
  };

  const nodes: Node[] = useMemo(() => {
    const out: Node[] = [{ id: "input", type: "toolInput", position: pos.input ?? { x: 0, y: 40 }, data: {} }];
    flow.steps.forEach((s, i) => {
      const nid = `s:${s.id}`;
      out.push({
        id: nid,
        type: "step",
        position: pos[nid] ?? { x: 280 + i * 300, y: 20 + (i % 2) * 36 },
        data: { index: i },
      });
    });
    out.push({
      id: "output",
      type: "toolOutput",
      position: pos.output ?? { x: 280 + flow.steps.length * 300, y: 40 },
      data: {},
    });
    return out;
  }, [flow, pos]);

  const edges: Edge[] = useMemo(() => {
    const out: Edge[] = [];
    const ids = flow.steps.map((s) => s.id);
    flow.steps.forEach((s, i) => {
      if (i > 0)
        out.push({
          id: `seq:${i}`,
          source: `s:${flow.steps[i - 1].id}`,
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
            animated: false,
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
    if (o && "step" in o && ids.includes(o.step))
      out.push({
        id: "ret",
        source: `s:${o.step}`,
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
      if (flow.steps.findIndex((s) => s.id === sid) >= ti) return; // only earlier steps
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

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    const v = e.dataTransfer.getData("application/x-mcpdeck-request");
    if (!v) return;
    const [api, req] = v.split(".");
    addStep(api, req, rf.screenToFlowPosition({ x: e.clientX, y: e.clientY }));
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
      <div className="flow-shell">
        <aside className="flow-palette">
          <div className="flow-palette-head">
            <div className="font-semibold">Requests</div>
            <div className="muted text-[12px]">Drag onto the canvas, or click + to add as the next step.</div>
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
        </aside>
        <div className="flow-canvas" onDragOver={(e) => e.preventDefault()} onDrop={onDrop}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={(ch) => {
              for (const c of ch) {
                if (c.type === "position" && c.position) setPos((p) => ({ ...p, [c.id]: c.position! }));
                if (c.type === "select" && c.selected) setSelected(c.id);
              }
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
          />
        </aside>
      </div>
    </FlowCtx.Provider>
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
}: {
  selected: string;
  flow: Flow;
  onChange: (f: Flow) => void;
  reqOf: Ctx["reqOf"];
  setArg: (ti: number, a: Arg) => void;
  onSelect: (id: string) => void;
}) {
  if (selected === "input" || !selected) return <ToolInspector flow={flow} onChange={onChange} />;
  if (selected === "output") return <OutputInspector flow={flow} onChange={onChange} reqOf={reqOf} />;
  const idx = flow.steps.findIndex((s) => `s:${s.id}` === selected);
  if (idx < 0) return <ToolInspector flow={flow} onChange={onChange} />;
  return <StepInspector idx={idx} flow={flow} onChange={onChange} reqOf={reqOf} setArg={setArg} onSelect={onSelect} />;
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
        <div className="insp-title">The tool</div>
        <div className="muted text-[12px]">What agents see in tools/list.</div>
      </div>
      <Field label="Name" hint="Verb and object, like find_vulnerabilities">
        <input className="input mono" value={m.name} onChange={(e) => setMeta({ name: e.target.value })} />
      </Field>
      <Field label="Description" hint="What it returns, in one sentence">
        <textarea
          className="textarea prose"
          rows={3}
          placeholder="Lists the fixable vulnerabilities of one application's newest image, with CVE, severity and fixed version."
          value={m.description}
          onChange={(e) => setMeta({ description: e.target.value })}
        />
      </Field>
      <Field label="Use when">
        <input
          className="input"
          placeholder="A question is about security issues in one application"
          value={m.useWhen}
          onChange={(e) => setMeta({ useWhen: e.target.value })}
        />
      </Field>
      <Field label="Don't use when">
        <input className="input" value={m.dontUseWhen} onChange={(e) => setMeta({ dontUseWhen: e.target.value })} />
      </Field>
      <div>
        <div className="mb-2 flex items-center">
          <div className="insp-title">Inputs the agent fills</div>
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
        {flow.inputs.length === 0 && <div className="muted text-[12.5px]">None. Steps then use fixed values only.</div>}
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
                options={["string", "number", "boolean"].map((t) => ({ value: t, label: t }))}
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
}: {
  idx: number;
  flow: Flow;
  onChange: (f: Flow) => void;
  reqOf: Ctx["reqOf"];
  setArg: (ti: number, a: Arg) => void;
  onSelect: (id: string) => void;
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
    if (j < 0 || j >= flow.steps.length) return;
    const steps = [...flow.steps];
    [steps[idx], steps[j]] = [steps[j], steps[idx]];
    onChange({ ...flow, steps });
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
