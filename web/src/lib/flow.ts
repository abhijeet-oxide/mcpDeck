// The flow engine: a tool's TypeScript ⇄ a list of steps (FLOW-1, FLOW-2).
//
// Code is the source of truth. A flow is code in a canonical shape:
//
//   export const tool = { name, description, useWhen, dontUseWhen, inputSchema };
//   export default async function (input: Input) {
//     const login = await scanner.login();
//     const apps = await scanner.listApplications({ name: input.app_name, headers: { Authorization: `Bearer ${login.access_token}` } });
//     return pick(apps, ["items[].id", "items[].name"]);
//   }
//
// parseFlow reads that shape back; anything else (loops, ifs, helper
// functions) is reported as not representable and stays editable as code.
// Nothing here evaluates code: agents can write tools, so the UI never runs
// their source.

export type Source =
  | { kind: "input"; field: string }
  | { kind: "step"; step: string; path: string }
  | { kind: "literal"; value: string | number | boolean }
  | { kind: "template"; text: string }
  | { kind: "expr"; code: string };

export type Arg = { name: string; source: Source; header?: boolean };
export type Step = { id: string; api: string; req: string; args: Arg[] };
export type ToolInput = { name: string; type: string; description: string; required: boolean };
export type ToolMeta = { name: string; description: string; useWhen: string; dontUseWhen: string };
export type Output = { step: string; fields: string[] } | { code: string };
export type Flow = { meta: ToolMeta; inputs: ToolInput[]; steps: Step[]; output: Output | null };
export type ParseResult = { flow: Flow; ok: true } | { flow: null; ok: false; reason: string; line?: number };

// ───────── A small, safe reader for JS object literals ─────────

type Tok = { t: "punc" | "str" | "num" | "ident"; v: string };

function tokenize(src: string): Tok[] {
  const out: Tok[] = [];
  let i = 0;
  while (i < src.length) {
    const c = src[i];
    if (/\s/.test(c)) {
      i++;
    } else if (c === "/" && src[i + 1] === "/") {
      while (i < src.length && src[i] !== "\n") i++;
    } else if (c === "/" && src[i + 1] === "*") {
      const e = src.indexOf("*/", i + 2);
      i = e < 0 ? src.length : e + 2;
    } else if ("{}[]:,".includes(c)) {
      out.push({ t: "punc", v: c });
      i++;
    } else if (c === '"' || c === "'" || c === "`") {
      let j = i + 1;
      let s = "";
      while (j < src.length && src[j] !== c) {
        if (src[j] === "\\" && j + 1 < src.length) {
          const n = src[j + 1];
          s += n === "n" ? "\n" : n === "t" ? "\t" : n;
          j += 2;
        } else {
          if (c === "`" && src[j] === "$" && src[j + 1] === "{")
            throw new Error("template expressions aren't allowed here");
          s += src[j++];
        }
      }
      if (j >= src.length) throw new Error("unterminated string");
      out.push({ t: "str", v: s });
      i = j + 1;
    } else if (/[-\d.]/.test(c)) {
      const m = /^-?(\d+\.?\d*|\.\d+)([eE][-+]?\d+)?/.exec(src.slice(i));
      if (!m) throw new Error(`unexpected ${c}`);
      out.push({ t: "num", v: m[0] });
      i += m[0].length;
    } else if (/[A-Za-z_$]/.test(c)) {
      const m = /^[A-Za-z_$][\w$]*/.exec(src.slice(i))!;
      out.push({ t: "ident", v: m[0] });
      i += m[0].length;
    } else {
      throw new Error(`unexpected ${c}`);
    }
  }
  return out;
}

/** parseLiteral reads a JS object/array/string/number literal without evaluating it. */
export function parseLiteral(src: string): unknown {
  const toks = tokenize(src);
  let p = 0;
  const peek = () => toks[p];
  const take = (v?: string) => {
    const t = toks[p++];
    if (!t || (v !== undefined && t.v !== v)) throw new Error(`expected ${v ?? "a value"}`);
    return t;
  };
  const value = (): unknown => {
    const t = take();
    if (t.t === "str") return t.v;
    if (t.t === "num") return Number(t.v);
    if (t.t === "ident") {
      if (t.v === "true") return true;
      if (t.v === "false") return false;
      if (t.v === "null") return null;
      if (t.v === "undefined") return undefined;
      throw new Error(`${t.v} isn't a literal`);
    }
    if (t.v === "[") {
      const arr: unknown[] = [];
      while (peek() && peek().v !== "]") {
        arr.push(value());
        if (peek()?.v === ",") take(",");
      }
      take("]");
      return arr;
    }
    if (t.v === "{") {
      const obj: Record<string, unknown> = {};
      while (peek() && peek().v !== "}") {
        const k = take();
        if (k.t !== "ident" && k.t !== "str" && k.t !== "num") throw new Error("expected a key");
        take(":");
        obj[k.v] = value();
        if (peek()?.v === ",") take(",");
      }
      take("}");
      return obj;
    }
    throw new Error(`unexpected ${t.v}`);
  };
  const v = value();
  if (p !== toks.length) throw new Error("unexpected text after the value");
  return v;
}

/** matchBrace returns the index of the bracket closing the one at `open`, skipping strings and comments. */
export function matchBrace(s: string, open: number): number {
  const pairs: Record<string, string> = { "{": "}", "(": ")", "[": "]" };
  const stack: string[] = [];
  for (let i = open; i < s.length; i++) {
    const c = s[i];
    if (c === '"' || c === "'" || c === "`") {
      const q = c;
      i++;
      while (i < s.length && s[i] !== q) {
        if (s[i] === "\\") i++;
        else if (q === "`" && s[i] === "$" && s[i + 1] === "{") i = matchBrace(s, i + 1);
        i++;
      }
    } else if (c === "/" && s[i + 1] === "/") {
      while (i < s.length && s[i] !== "\n") i++;
    } else if (c === "/" && s[i + 1] === "*") {
      i = s.indexOf("*/", i + 2) + 1 || s.length;
    } else if (pairs[c]) {
      stack.push(pairs[c]);
    } else if (c === "}" || c === ")" || c === "]") {
      if (stack.pop() !== c) return -1;
      if (stack.length === 0) return i;
    }
  }
  return -1;
}

/** splitTop splits on commas that aren't inside brackets, strings or templates. */
export function splitTop(s: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let cur = "";
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '"' || c === "'" || c === "`") {
      const q = c;
      let j = i + 1;
      while (j < s.length && s[j] !== q) {
        if (s[j] === "\\") j++;
        else if (q === "`" && s[j] === "$" && s[j + 1] === "{") j = matchBrace(s, j + 1);
        j++;
      }
      cur += s.slice(i, j + 1);
      i = j;
      continue;
    }
    if ("{([".includes(c)) depth++;
    if ("})]".includes(c)) depth--;
    if (c === "," && depth === 0) {
      if (cur.trim()) parts.push(cur.trim());
      cur = "";
    } else cur += c;
  }
  if (cur.trim()) parts.push(cur.trim());
  return parts;
}

// ───────── Sources ─────────

const ident = /^[A-Za-z_$][\w$]*$/;
const pathRe = /^([A-Za-z_$][\w$]*)((?:\.[A-Za-z_$][\w$]*|\[\d+\])+)$/;

export function parseSource(code: string, steps: string[]): Source {
  const c = code.trim();
  const inp = /^input\.([A-Za-z_$][\w$]*)$/.exec(c);
  if (inp) return { kind: "input", field: inp[1] };
  const m = pathRe.exec(c);
  if (m && steps.includes(m[1])) return { kind: "step", step: m[1], path: m[2].replace(/^\./, "") };
  if (ident.test(c) && steps.includes(c)) return { kind: "step", step: c, path: "" };
  if (/^`[^`]*`$/.test(c)) return { kind: "template", text: c.slice(1, -1) };
  if (/^(["'])(?:\\.|(?!\1).)*\1$/.test(c)) {
    try {
      return { kind: "literal", value: parseLiteral(c) as string };
    } catch {
      /* fall through */
    }
  }
  if (/^-?\d+(\.\d+)?$/.test(c)) return { kind: "literal", value: Number(c) };
  if (c === "true" || c === "false") return { kind: "literal", value: c === "true" };
  return { kind: "expr", code: c };
}

export function sourceCode(s: Source): string {
  switch (s.kind) {
    case "input":
      return `input.${s.field}`;
    case "step":
      return s.path ? `${s.step}${s.path.startsWith("[") ? "" : "."}${s.path}` : s.step;
    case "literal":
      return typeof s.value === "string" ? JSON.stringify(s.value) : String(s.value);
    case "template":
      return `\`${s.text}\``;
    case "expr":
      return s.code;
  }
}

/** Which steps a source reads from (for drawing edges). */
export function sourceSteps(s: Source): { step: string; path: string }[] {
  if (s.kind === "step") return [{ step: s.step, path: s.path }];
  if (s.kind === "template" || s.kind === "expr") {
    const text = s.kind === "template" ? s.text : s.code;
    return [...text.matchAll(/([A-Za-z_$][\w$]*)((?:\.[A-Za-z_$][\w$]*|\[\d+\])+)/g)].map((m) => ({
      step: m[1],
      path: m[2].replace(/^\./, ""),
    }));
  }
  return [];
}

export function sourceInputs(s: Source): string[] {
  if (s.kind === "input") return [s.field];
  if (s.kind === "template" || s.kind === "expr") {
    const text = s.kind === "template" ? s.text : s.code;
    return [...text.matchAll(/input\.([A-Za-z_$][\w$]*)/g)].map((m) => m[1]);
  }
  return [];
}

// ───────── Parse ─────────

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

export function parseFlow(src: string): ParseResult {
  const fail = (reason: string, line?: number): ParseResult => ({ flow: null, ok: false, reason, line });
  const lineOf = (i: number) => src.slice(0, i).split("\n").length;
  const t = /export\s+const\s+tool\s*(?::\s*[\w.]+\s*)?=\s*\{/.exec(src);
  if (!t) return fail("The file has no `export const tool = { … }`.");
  const open = t.index + t[0].length - 1;
  const close = matchBrace(src, open);
  if (close < 0) return fail("`tool` isn't closed.", lineOf(open));
  let tool: Record<string, unknown>;
  try {
    tool = parseLiteral(src.slice(open, close + 1)) as Record<string, unknown>;
  } catch (e) {
    return fail(`\`tool\` uses something other than plain values (${(e as Error).message}).`, lineOf(open));
  }
  const schema = (tool.inputSchema ?? {}) as { properties?: Record<string, any>; required?: string[] };
  const required = new Set(schema.required ?? []);
  const inputs: ToolInput[] = Object.entries(schema.properties ?? {}).map(([name, p]) => ({
    name,
    type: str(p?.type) || "string",
    description: str(p?.description),
    required: required.has(name),
  }));
  const meta: ToolMeta = {
    name: str(tool.name),
    description: str(tool.description),
    useWhen: str(tool.useWhen),
    dontUseWhen: str(tool.dontUseWhen),
  };
  const f = /export\s+default\s+async\s+function\s*\w*\s*\(\s*input\s*(?::\s*[\w.]+)?\s*\)\s*(?::\s*[^{]+)?\{/.exec(
    src,
  );
  if (!f) return fail("The file has no `export default async function (input) { … }`.");
  const bOpen = f.index + f[0].length - 1;
  const bClose = matchBrace(src, bOpen);
  if (bClose < 0) return fail("The function isn't closed.", lineOf(bOpen));
  const rest = src.slice(close + 1, f.index) + src.slice(bClose + 1);
  if (
    /\S/.test(
      rest
        .replace(/\/\/.*$/gm, "")
        .replace(/\/\*[\s\S]*?\*\//g, "")
        .replace(/;/g, ""),
    )
  ) {
    return fail("There's code outside `tool` and the function, like a helper.");
  }
  // Statements: split the body on top-level semicolons and newlines.
  const body = src.slice(bOpen + 1, bClose);
  const bodyStart = bOpen + 1;
  const stmts: { text: string; at: number }[] = [];
  {
    let depth = 0;
    let start = 0;
    for (let i = 0; i < body.length; i++) {
      const c = body[i];
      if (c === '"' || c === "'" || c === "`") {
        const q = c;
        i++;
        while (i < body.length && body[i] !== q) {
          if (body[i] === "\\") i++;
          else if (q === "`" && body[i] === "$" && body[i + 1] === "{") i = matchBrace(body, i + 1);
          i++;
        }
        continue;
      }
      if (c === "/" && body[i + 1] === "/") {
        const e = body.indexOf("\n", i);
        i = e < 0 ? body.length : e;
        if (depth === 0) {
          stmts.push({ text: body.slice(start, i), at: start });
          start = i + 1;
        }
        continue;
      }
      if ("{([".includes(c)) depth++;
      if ("})]".includes(c)) depth--;
      if (depth === 0 && (c === ";" || c === "\n")) {
        stmts.push({ text: body.slice(start, i), at: start });
        start = i + 1;
      }
    }
    stmts.push({ text: body.slice(start), at: start });
  }
  const steps: Step[] = [];
  let output: Output | null = null;
  for (const st of stmts) {
    const text = st.text.replace(/\/\/.*$/gm, "").trim();
    if (!text) continue;
    const line = lineOf(bodyStart + st.at + (st.text.length - st.text.trimStart().length));
    if (output) return fail("There's code after `return`.", line);
    const call = /^const\s+([A-Za-z_$][\w$]*)\s*=\s*await\s+([A-Za-z_$][\w$]*)\.([A-Za-z_$][\w$]*)\(([\s\S]*)\)$/.exec(
      text,
    );
    if (call) {
      const [, id, api, req, argText] = call;
      const args: Arg[] = [];
      const a = argText.trim();
      if (a) {
        if (!a.startsWith("{") || matchBrace(a, 0) !== a.length - 1) return fail(`${id}: pass one { … } object.`, line);
        for (const part of splitTop(a.slice(1, -1))) {
          const kv = /^([A-Za-z_$][\w$]*|"[^"]+"|'[^']+')\s*:\s*([\s\S]+)$/.exec(part);
          if (!kv) {
            // shorthand { appId } → appId: appId
            if (ident.test(part)) {
              args.push({
                name: part,
                source: parseSource(
                  part,
                  steps.map((s) => s.id),
                ),
              });
              continue;
            }
            return fail(`${id}: can't read "${part}".`, line);
          }
          const name = kv[1].replace(/^["']|["']$/g, "");
          const val = kv[2].trim();
          if (name === "headers" && val.startsWith("{") && matchBrace(val, 0) === val.length - 1) {
            for (const h of splitTop(val.slice(1, -1))) {
              const hk = /^([A-Za-z_$][\w$]*|"[^"]+"|'[^']+')\s*:\s*([\s\S]+)$/.exec(h);
              if (!hk) return fail(`${id}: can't read the header "${h}".`, line);
              args.push({
                name: hk[1].replace(/^["']|["']$/g, ""),
                header: true,
                source: parseSource(
                  hk[2],
                  steps.map((s) => s.id),
                ),
              });
            }
            continue;
          }
          args.push({
            name,
            source: parseSource(
              val,
              steps.map((s) => s.id),
            ),
          });
        }
      }
      if (steps.some((s) => s.id === id)) return fail(`${id} is used twice.`, line);
      steps.push({ id, api, req, args });
      continue;
    }
    const ret = /^return\s+([\s\S]+)$/.exec(text);
    if (ret) {
      const r = ret[1].trim();
      const pk = /^pick\(\s*([A-Za-z_$][\w$]*)\s*,\s*(\[[\s\S]*\])\s*\)$/.exec(r);
      if (pk && steps.some((s) => s.id === pk[1])) {
        try {
          const fields = parseLiteral(pk[2]) as unknown[];
          if (fields.every((x) => typeof x === "string")) {
            output = { step: pk[1], fields: fields as string[] };
            continue;
          }
        } catch {
          /* custom */
        }
      }
      if (ident.test(r) && steps.some((s) => s.id === r)) {
        output = { step: r, fields: [] };
        continue;
      }
      output = { code: r };
      continue;
    }
    return fail(`Line ${line} isn't a step or a return (custom logic).`, line);
  }
  return { ok: true, flow: { meta, inputs, steps, output } };
}

// ───────── Generate ─────────

function q(s: string) {
  return JSON.stringify(s);
}

function key(k: string) {
  return ident.test(k) ? k : q(k);
}

export function generateFlow(f: Flow): string {
  const lines: string[] = [];
  lines.push("// Built with the flow builder. Visual and Code edit this same file: keep one");
  lines.push("// `const step = await app.request({ … })` per line and the canvas stays in sync.");
  lines.push("export const tool = {");
  lines.push(`  name: ${q(f.meta.name)},`);
  lines.push(`  description: ${q(f.meta.description)},`);
  if (f.meta.useWhen) lines.push(`  useWhen: ${q(f.meta.useWhen)},`);
  if (f.meta.dontUseWhen) lines.push(`  dontUseWhen: ${q(f.meta.dontUseWhen)},`);
  lines.push("  inputSchema: {");
  lines.push('    type: "object",');
  if (f.inputs.length === 0) lines.push("    properties: {},");
  else {
    lines.push("    properties: {");
    for (const i of f.inputs) {
      const d = i.description ? `, description: ${q(i.description)}` : "";
      lines.push(`      ${key(i.name)}: { type: ${q(i.type || "string")}${d} },`);
    }
    lines.push("    },");
  }
  const req = f.inputs.filter((i) => i.required).map((i) => q(i.name));
  lines.push(`    required: [${req.join(", ")}],`);
  lines.push("  },");
  lines.push("};");
  lines.push("");
  lines.push("export default async function (input: Input) {");
  for (const s of f.steps) {
    const plain = s.args.filter((a) => !a.header).map((a) => `${key(a.name)}: ${sourceCode(a.source)}`);
    const hdrs = s.args.filter((a) => a.header).map((a) => `${key(a.name)}: ${sourceCode(a.source)}`);
    if (hdrs.length) plain.push(`headers: { ${hdrs.join(", ")} }`);
    lines.push(`  const ${s.id} = await ${s.api}.${s.req}(${plain.length ? `{ ${plain.join(", ")} }` : ""});`);
  }
  const o = f.output;
  if (o && "code" in o) lines.push(`  return ${o.code};`);
  else if (o?.step && o.fields.length > 0) lines.push(`  return pick(${o.step}, [${o.fields.map(q).join(", ")}]);`);
  else if (o?.step) lines.push(`  return ${o.step};`);
  else if (f.steps.length) lines.push(`  return ${f.steps[f.steps.length - 1].id};`);
  else lines.push("  return {};");
  lines.push("}");
  lines.push("");
  return lines.join("\n");
}

// ───────── Helpers for building flows from requests ─────────

export type ReqInfo = {
  api: string;
  name: string;
  method: string;
  url: string;
  query: { key: string; disabled?: boolean }[];
  bodyType: string;
  body: string;
  example?: unknown;
};

/** argsOf lists a request's parameters: path params, query keys, top-level JSON body keys. */
export function argsOf(r: ReqInfo): { name: string; where: "path" | "query" | "body" }[] {
  const out: { name: string; where: "path" | "query" | "body" }[] = [];
  // {id} but not {{baseUrl}}.
  for (const m of r.url.replace(/\{\{[^}]*\}\}/g, "").matchAll(/\{([A-Za-z0-9_]+)\}/g))
    out.push({ name: m[1], where: "path" });
  for (const m of r.url.replace(/\{\{[^}]*\}\}/g, "").matchAll(/\/:([A-Za-z_]\w*)/g))
    if (!out.some((o) => o.name === m[1])) out.push({ name: m[1], where: "path" });
  for (const k of r.query) if (k.key && !out.some((o) => o.name === k.key)) out.push({ name: k.key, where: "query" });
  if (r.bodyType === "json" && r.body.trim().startsWith("{")) {
    try {
      // Bodies are JSON (with {{vars}} in strings), so JSON.parse is enough.
      const b = JSON.parse(r.body.replace(/"\{\{([^}]+)\}\}"/g, '"__mcpdeck_var__"').replace(/\{\{([^}]+)\}\}/g, "0"));
      // Fields filled from the environment ({{password}}) stay with the request.
      for (const [k, v] of Object.entries(b))
        if (v !== "__mcpdeck_var__" && !out.some((o) => o.name === k)) out.push({ name: k, where: "body" });
    } catch {
      /* not JSON */
    }
  }
  return out;
}

export type FieldInfo = { path: string; type: string; sample?: string };

/** fieldsOf flattens an example response. mode "wire" uses [0] for arrays (one value to pass on);
 *  mode "pick" uses [] (every item), for choosing output fields. */
export function fieldsOf(example: unknown, mode: "wire" | "pick", max = 60): FieldInfo[] {
  const out: FieldInfo[] = [];
  const walk = (v: unknown, path: string, depth: number) => {
    if (out.length >= max || depth > 4) return;
    if (Array.isArray(v)) {
      if (path) out.push({ path, type: "array" });
      if (v.length > 0) walk(v[0], `${path}${mode === "wire" ? "[0]" : "[]"}`, depth + 1);
      return;
    }
    if (v && typeof v === "object") {
      if (path && mode === "pick") out.push({ path, type: "object" });
      for (const [k, x] of Object.entries(v as Record<string, unknown>)) {
        walk(x, path ? `${path}.${k}` : k, depth + 1);
      }
      return;
    }
    out.push({
      path,
      type: v === null ? "null" : typeof v,
      sample: v === null || v === undefined ? "" : String(v).slice(0, 40),
    });
  };
  walk(example, "", 0);
  return out;
}

/** stepId makes a short variable name from a request name: listApplications → applications. */
export function stepId(req: string, taken: string[]): string {
  let base = req.replace(/^(get|list|search|fetch|find|create|post|do)(?=[A-Z])/, "");
  base = base.charAt(0).toLowerCase() + base.slice(1);
  if (!ident.test(base) || ["input", "tool", "pick", "call", "vars", "await", "return", "const"].includes(base))
    base = req;
  let id = base;
  for (let n = 2; taken.includes(id); n++) id = `${base}${n}`;
  return id;
}

const tokenFields = ["access_token", "accessToken", "token", "id_token", "jwt"];

/**
 * autoWire guesses where each argument of a new step comes from (TOOL-10):
 * a field of the same name in an earlier step, `<thing>Id` from an earlier
 * list's first item, or else a new tool input. A step after one that
 * returned a token gets `Authorization: Bearer …`.
 */
export function autoWire(
  step: Step,
  info: ReqInfo,
  before: { step: Step; fields: FieldInfo[] }[],
  inputs: ToolInput[],
  appAuth: string,
): { args: Arg[]; newInputs: ToolInput[] } {
  const newInputs: ToolInput[] = [];
  const args: Arg[] = [];
  for (const a of argsOf(info)) {
    let src: Source | null = null;
    for (let i = before.length - 1; i >= 0 && !src; i--) {
      const b = before[i];
      const exact = b.fields.find((f) => f.path.split(".").pop()?.toLowerCase() === a.name.toLowerCase());
      if (exact) src = { kind: "step", step: b.step.id, path: exact.path };
      else if (/Id$|_id$/.test(a.name)) {
        const id = b.fields.find(
          (f) => /(^|\.)(items|data|results|records|\w+)\[0\]\.id$/.test(f.path) || f.path === "id",
        );
        if (id) src = { kind: "step", step: b.step.id, path: id.path };
      }
    }
    if (!src) {
      // Optional query params stay out until someone adds them on the step.
      if (a.where === "query") continue;
      const name = a.name;
      if (!inputs.some((x) => x.name === name) && !newInputs.some((x) => x.name === name))
        newInputs.push({ name, type: "string", description: "", required: a.where === "path" });
      src = { kind: "input", field: name };
    }
    args.push({ name: a.name, source: src });
  }
  if (appAuth === "none" || appAuth === "") {
    for (let i = before.length - 1; i >= 0; i--) {
      const tok = before[i].fields.find((f) => tokenFields.includes(f.path));
      if (tok && before[i].step.api === step.api) {
        args.push({
          name: "Authorization",
          header: true,
          source: { kind: "template", text: `Bearer \${${before[i].step.id}.${tok.path}}` },
        });
        break;
      }
    }
  }
  return { args, newInputs };
}
