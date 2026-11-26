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
/**
 * A step is one statement of the function, in order.
 *  - "call" (the default): `const id = await api.req({ … })`. `discard` drops
 *    the `const id =` (a write whose result isn't needed); steps with the same
 *    `group` run together in one `Promise.all`.
 *  - "code": any other statement, kept as written (loops, ifs, helpers,
 *    `call("app", "GET /path")`). The canvas shows it as a code block, so the
 *    visual builder can show every tool.
 */
export type Step = {
  id: string;
  api: string;
  req: string;
  args: Arg[];
  kind?: "call" | "code";
  code?: string;
  discard?: boolean;
  group?: number;
  comment?: string;
};
export type ToolInput = {
  name: string;
  type: string;
  description: string;
  required: boolean;
  /** Other JSON Schema keywords of this property (enum, examples, default…), kept as written. */
  extra?: Record<string, unknown>;
};
export type ToolMeta = {
  name: string;
  description: string;
  useWhen: string;
  dontUseWhen: string;
  /** Other keys of `tool` (annotations, outputSchema…), kept as written. */
  extra?: Record<string, unknown>;
  /** Other keys of inputSchema (additionalProperties…). */
  schemaExtra?: Record<string, unknown>;
  /** The order of inputSchema.required as written. */
  requiredOrder?: string[];
  /** Text before `export const tool`, between it and the function, and after the function. */
  header?: string;
  between?: string;
  trailer?: string;
  /** The function's first line as written, like `export default async function (input) {`. */
  signature?: string;
};
export type Output = { step: string; fields: string[] } | { code: string };
export type Flow = {
  meta: ToolMeta;
  inputs: ToolInput[];
  steps: Step[];
  output: Output | null;
  returnComment?: string;
};
export type ParseResult = { flow: Flow; ok: true } | { flow: null; ok: false; reason: string; line?: number };

export const isCode = (s: Step) => s.kind === "code";

/** codeDefines lists the names a code step declares (const a = …, const [a, b] = …, const { a } = …). */
export function codeDefines(code: string): string[] {
  const out: string[] = [];
  for (const m of code.matchAll(/(?:^|[;\n])\s*(?:const|let|var)\s+([^=]+?)\s*=/g)) {
    const lhs = m[1].trim();
    if (/^[A-Za-z_$][\w$]*$/.test(lhs)) out.push(lhs);
    else if (/^[[{]/.test(lhs))
      for (const n of lhs.slice(1, -1).split(",")) {
        const name =
          n
            .split(":")
            .pop()
            ?.split("=")[0]
            .trim()
            .replace(/^\.\.\./, "") ?? "";
        if (/^[A-Za-z_$][\w$]*$/.test(name)) out.push(name);
      }
  }
  return out;
}

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

function omit(o: Record<string, unknown>, keys: string[]): Record<string, unknown> | undefined {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(o)) if (!keys.includes(k)) out[k] = v;
  return Object.keys(out).length ? out : undefined;
}

type Stmt = { text: string; start: number; end: number };

/** Statements of a function body: split on top-level `;` and newlines, then
 *  joined back where a statement plainly continues on the next line. */
function statements(body: string): Stmt[] {
  const raw: Stmt[] = [];
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
      i = (e < 0 ? body.length : e) - 1;
      continue;
    }
    if (c === "/" && body[i + 1] === "*") {
      const e = body.indexOf("*/", i + 2);
      i = e < 0 ? body.length : e + 1;
      continue;
    }
    if ("{([".includes(c)) depth++;
    if ("})]".includes(c)) depth--;
    if (depth === 0 && (c === ";" || c === "\n")) {
      raw.push({ text: body.slice(start, i + 1), start, end: i + 1 });
      start = i + 1;
    }
  }
  raw.push({ text: body.slice(start), start, end: body.length });
  const code = (t: string) =>
    t
      .replace(/\/\/.*$/gm, "")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .trim()
      .replace(/;$/, "")
      .trim();
  const out: Stmt[] = [];
  for (const st of raw) {
    const prev = out[out.length - 1];
    const now = code(st.text);
    const before = prev ? code(prev.text) : "";
    const continues =
      prev &&
      before &&
      !prev.text.trimEnd().endsWith(";") &&
      (/^(\.|\?\.|\?|:|&&|\|\||\?\?|[+\-*/%]|=>|\)|\]|\}|else\b|catch\b|finally\b)/.test(now) ||
        /(=|\+|-|\*|\/|&&|\|\||\?\?|\?|:|,|\(|\[|\{|=>)$/.test(before) ||
        (/^(if|for|while)\b/.test(before) && !/[};]$/.test(before)) ||
        (/^(else)\b/.test(before) && !/[};]$/.test(before)));
    if (prev && continues) {
      prev.text += st.text;
      prev.end = st.end;
    } else out.push({ ...st });
  }
  return out;
}

const callRe = /^([A-Za-z_$][\w$]*)\.([A-Za-z_$][\w$]*)\(([\s\S]*)\)$/;

/** parseArgs reads the `{ … }` argument of a request call, or returns null. */
function parseArgs(argText: string, steps: string[]): Arg[] | null {
  const args: Arg[] = [];
  const a = argText.trim();
  if (!a) return args;
  if (!a.startsWith("{") || matchBrace(a, 0) !== a.length - 1) return null;
  for (const part of splitTop(a.slice(1, -1))) {
    const kv = /^([A-Za-z_$][\w$]*|"[^"]+"|'[^']+')\s*:\s*([\s\S]+)$/.exec(part);
    if (!kv) {
      if (ident.test(part)) {
        args.push({ name: part, source: parseSource(part, steps) });
        continue;
      }
      return null;
    }
    const name = kv[1].replace(/^["']|["']$/g, "");
    const val = kv[2].trim();
    if (name === "headers" && val.startsWith("{") && matchBrace(val, 0) === val.length - 1) {
      for (const h of splitTop(val.slice(1, -1))) {
        const hk = /^([A-Za-z_$][\w$]*|"[^"]+"|'[^']+')\s*:\s*([\s\S]+)$/.exec(h);
        if (!hk) return null;
        args.push({ name: hk[1].replace(/^["']|["']$/g, ""), header: true, source: parseSource(hk[2], steps) });
      }
      continue;
    }
    args.push({ name, source: parseSource(val, steps) });
  }
  return args;
}

/** dedent removes the common leading indentation of a block. */
function dedent(s: string): string {
  const lines = s.replace(/^\n+|\s+$/g, "").split("\n");
  const ind = Math.min(...lines.filter((l) => l.trim()).map((l) => /^ */.exec(l)![0].length));
  return lines.map((l) => l.slice(Number.isFinite(ind) ? ind : 0)).join("\n");
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
  const schema = (tool.inputSchema ?? {}) as Record<string, unknown> & {
    properties?: Record<string, any>;
    required?: string[];
  };
  const required = new Set(schema.required ?? []);
  const inputs: ToolInput[] = Object.entries(schema.properties ?? {}).map(([name, p]) => {
    const prop = (p && typeof p === "object" ? p : {}) as Record<string, unknown>;
    const extra = omit(prop, ["type", "description"]);
    return {
      name,
      type: str(prop.type),
      description: str(prop.description),
      required: required.has(name),
      ...(extra ? { extra } : {}),
    };
  });
  const f = /export\s+default\s+async\s+function\s*\w*\s*\(\s*input\s*(?::\s*[\w.]+)?\s*\)\s*(?::\s*[^{]+)?\{/.exec(
    src,
  );
  if (!f) return fail("The file has no `export default async function (input) { … }`.");
  if (f.index < close) return fail("The function comes before `tool`.");
  const bOpen = f.index + f[0].length - 1;
  const bClose = matchBrace(src, bOpen);
  if (bClose < 0) return fail("The function isn't closed.", lineOf(bOpen));
  // `export const tool = {…};` — the `;` belongs to the tool.
  let afterTool = close + 1;
  if (src[afterTool] === ";") afterTool++;
  const meta: ToolMeta = {
    name: str(tool.name),
    description: str(tool.description),
    useWhen: str(tool.useWhen),
    dontUseWhen: str(tool.dontUseWhen),
  };
  const extra = omit(tool, ["name", "description", "useWhen", "dontUseWhen", "inputSchema"]);
  if (extra) meta.extra = extra;
  const schemaExtra = omit(schema, ["type", "properties", "required"]);
  if (schemaExtra) meta.schemaExtra = schemaExtra;
  if (schema.required) meta.requiredOrder = [...schema.required];
  meta.header = src.slice(0, t.index);
  meta.between = src.slice(afterTool, f.index);
  meta.trailer = src.slice(bClose + 1);
  meta.signature = f[0];

  const body = src.slice(bOpen + 1, bClose);
  const bodyStart = bOpen + 1;
  const stmts = statements(body);
  const steps: Step[] = [];
  let output: Output | null = null;
  let returnComment: string | undefined;
  let comment: string[] = [];
  let group = 0;
  const ids = () => steps.flatMap((s) => (s.kind === "code" ? codeDefines(s.code ?? "") : s.discard ? [] : [s.id]));
  const taken = () => [...steps.map((s) => s.id), ...ids()];
  const freshId = (base: string) => {
    let id = base;
    for (let n = 2; taken().includes(id); n++) id = `${base}${n}`;
    return id;
  };
  // The last statement is the output when it's a `return`.
  let lastIdx = -1;
  stmts.forEach((st, i) => {
    if (/^\s*(\/\/[^\n]*\n?\s*|\/\*[\s\S]*?\*\/\s*)*return\b/.test(st.text) && st.text.replace(/\/\/.*$/gm, "").trim())
      lastIdx = i;
    else if (
      st.text
        .replace(/\/\/.*$/gm, "")
        .replace(/\/\*[\s\S]*?\*\//g, "")
        .trim()
    )
      lastIdx = -2 - i;
  });
  for (let si = 0; si < stmts.length; si++) {
    const st = stmts[si];
    const raw = st.text;
    // Leading comment lines belong to this statement.
    const lines = raw.replace(/^\n+/, "").split("\n");
    const lead: string[] = [];
    while (lines.length && /^\s*\/\//.test(lines[0]))
      lead.push(
        lines
          .shift()!
          .trim()
          .replace(/^\/\/\s?/, ""),
      );
    const rest = lines.join("\n");
    const text = rest
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/^\s*\/\/.*$/gm, "")
      .trim()
      .replace(/;$/, "")
      .trim();
    comment.push(...lead);
    if (!text) continue;
    const takeComment = () => {
      const c = comment.length ? comment.join("\n") : undefined;
      comment = [];
      return c;
    };
    const line = lineOf(bodyStart + st.start + (raw.length - raw.trimStart().length));
    const known = ids();
    // const x = await api.req({ … })  /  await api.req({ … })
    const one = /^(?:const\s+([A-Za-z_$][\w$]*)\s*=\s*)?await\s+([\s\S]+)$/.exec(text);
    if (one && !/[\n]\s*\/\//.test(rest)) {
      const c = callRe.exec(one[2].trim());
      if (c && c[1] !== "Promise" && c[1] !== "call" && !known.includes(c[1]) && c[1] !== "input") {
        const args = parseArgs(c[3], known);
        if (args) {
          const id = one[1] ?? freshId(`${c[2]}Result`);
          if (!taken().includes(id)) {
            steps.push({
              id,
              api: c[1],
              req: c[2],
              args,
              ...(one[1] ? {} : { discard: true }),
              comment: takeComment(),
            });
            if (!steps[steps.length - 1].comment) delete steps[steps.length - 1].comment;
            continue;
          }
        }
      }
    }
    // const [a, b] = await Promise.all([api.x({…}), api.y({…})])
    const par = /^const\s+\[([^\]]+)\]\s*=\s*await\s+Promise\.all\(\s*\[([\s\S]*)\]\s*\)$/.exec(text);
    if (par) {
      const names = par[1].split(",").map((x) => x.trim());
      const calls = splitTop(par[2]);
      const parsed =
        names.length === calls.length &&
        names.every((n) => ident.test(n) && !taken().includes(n)) &&
        calls.map((cl) => {
          const c = callRe.exec(cl.trim());
          if (!c || c[1] === "Promise" || c[1] === "call" || c[1] === "input" || known.includes(c[1])) return null;
          const args = parseArgs(c[3], known);
          return args ? { api: c[1], req: c[2], args } : null;
        });
      if (parsed && parsed.every(Boolean)) {
        group++;
        const c = takeComment();
        parsed.forEach((p, i) => {
          steps.push({ id: names[i], ...p!, group, ...(i === 0 && c ? { comment: c } : {}) });
        });
        continue;
      }
    }
    // return … as the last statement is the output.
    const ret = /^return\s+([\s\S]+)$/.exec(text);
    if (ret && si === lastIdx) {
      returnComment = takeComment();
      const r = ret[1].trim();
      const stepIds = steps.filter((s) => s.kind !== "code" && !s.discard).map((s) => s.id);
      const pk = /^pick\(\s*([A-Za-z_$][\w$]*)\s*,\s*(\[[\s\S]*\])\s*\)$/.exec(r);
      if (pk && stepIds.includes(pk[1])) {
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
      if (ident.test(r) && stepIds.includes(r)) output = { step: r, fields: [] };
      else output = { code: r };
      continue;
    }
    // Anything else is kept as a code step, exactly as written.
    const codeText = dedent(rest.replace(/^\s*\/\/.*\n/gm, (m) => m)).replace(/\s+$/, "");
    const defs = codeDefines(codeText);
    const id = freshId(defs[0] && !taken().includes(defs[0]) ? `${defs[0]}_code` : "code");
    steps.push({ id, api: "", req: "", args: [], kind: "code", code: codeText, comment: takeComment() });
    if (!steps[steps.length - 1].comment) delete steps[steps.length - 1].comment;
    void line;
  }
  if (comment.length) {
    // Comments after the last statement stay at the end of the body.
    steps.push({
      id: freshId("code"),
      api: "",
      req: "",
      args: [],
      kind: "code",
      code: comment.map((c) => `// ${c}`).join("\n"),
    });
  }
  const flow: Flow = { meta, inputs, steps, output };
  if (returnComment) flow.returnComment = returnComment;
  return { ok: true, flow };
}

// ───────── Generate ─────────

function q(s: string) {
  return JSON.stringify(s);
}

function key(k: string) {
  return ident.test(k) ? k : q(k);
}

/** lit writes a plain value as a JS literal: { a: 1, "b-c": ["x"] }. */
export function lit(v: unknown): string {
  if (v === undefined) return "undefined";
  if (v === null || typeof v !== "object") return JSON.stringify(v);
  if (Array.isArray(v)) return `[${v.map(lit).join(", ")}]`;
  const parts = Object.entries(v as Record<string, unknown>).map(([k, x]) => `${key(k)}: ${lit(x)}`);
  return parts.length ? `{ ${parts.join(", ")} }` : "{}";
}

const builderHeader =
  "// Built with the flow builder. Visual and Code edit this same file: keep one\n// `const step = await app.request({ … })` per line and the canvas stays in sync.\n";

function callText(s: Step): string {
  const plain = s.args.filter((a) => !a.header).map((a) => `${key(a.name)}: ${sourceCode(a.source)}`);
  const hdrs = s.args.filter((a) => a.header).map((a) => `${key(a.name)}: ${sourceCode(a.source)}`);
  if (hdrs.length) plain.push(`headers: { ${hdrs.join(", ")} }`);
  return `${s.api}.${s.req}(${plain.length ? `{ ${plain.join(", ")} }` : ""})`;
}

const commentLines = (c: string | undefined, ind: string) =>
  c ? c.split("\n").map((l) => `${ind}//${l ? ` ${l}` : ""}`) : [];

export function generateFlow(f: Flow): string {
  const lines: string[] = [];
  const m = f.meta;
  const head = m.header ?? builderHeader;
  lines.push(`${head}export const tool = {`.replace(/\n$/, ""));
  lines.push(`  name: ${q(m.name)},`);
  lines.push(`  description: ${q(m.description)},`);
  if (m.useWhen) lines.push(`  useWhen: ${q(m.useWhen)},`);
  if (m.dontUseWhen) lines.push(`  dontUseWhen: ${q(m.dontUseWhen)},`);
  lines.push("  inputSchema: {");
  lines.push('    type: "object",');
  if (f.inputs.length === 0) lines.push("    properties: {},");
  else {
    lines.push("    properties: {");
    for (const i of f.inputs) {
      const parts: string[] = [];
      if (i.type) parts.push(`type: ${q(i.type)}`);
      if (i.description) parts.push(`description: ${q(i.description)}`);
      for (const [k, v] of Object.entries(i.extra ?? {})) parts.push(`${key(k)}: ${lit(v)}`);
      lines.push(`      ${key(i.name)}: ${parts.length ? `{ ${parts.join(", ")} }` : "{}"},`);
    }
    lines.push("    },");
  }
  const req = f.inputs.filter((i) => i.required).map((i) => i.name);
  // As written; a required name with no property stays (lint reports it).
  const names = f.inputs.map((i) => i.name);
  const ordered = [
    ...(m.requiredOrder ?? []).filter((n) => req.includes(n) || !names.includes(n)),
    ...req.filter((n) => !(m.requiredOrder ?? []).includes(n)),
  ];
  if (ordered.length || m.requiredOrder) lines.push(`    required: [${ordered.map(q).join(", ")}],`);
  for (const [k, v] of Object.entries(m.schemaExtra ?? {})) lines.push(`    ${key(k)}: ${lit(v)},`);
  lines.push("  },");
  for (const [k, v] of Object.entries(m.extra ?? {})) lines.push(`  ${key(k)}: ${lit(v)},`);
  lines.push("};");
  // The `;` of `tool` and the text up to the function, as written.
  lines.push(
    `${(m.between ?? "\n\n").replace(/^\n/, "")}${m.signature ?? "export default async function (input: Input) {"}`,
  );
  for (let i = 0; i < f.steps.length; i++) {
    const s = f.steps[i];
    lines.push(...commentLines(s.comment, "  "));
    if (s.kind === "code") {
      for (const l of (s.code ?? "").split("\n")) lines.push(l ? `  ${l}` : "");
      continue;
    }
    if (s.group !== undefined) {
      const g: Step[] = [];
      while (i < f.steps.length && f.steps[i].group === s.group && f.steps[i].kind !== "code") g.push(f.steps[i++]);
      i--;
      if (g.length > 1) {
        lines.push(`  const [${g.map((x) => x.id).join(", ")}] = await Promise.all([`);
        for (const x of g) lines.push(`    ${callText(x)},`);
        lines.push("  ]);");
        continue;
      }
    }
    lines.push(s.discard ? `  await ${callText(s)};` : `  const ${s.id} = await ${callText(s)};`);
  }
  lines.push(...commentLines(f.returnComment, "  "));
  const o = f.output;
  const calls = f.steps.filter((s) => s.kind !== "code" && !s.discard);
  if (o && "code" in o) lines.push(`  return ${o.code};`);
  else if (o?.step && o.fields.length > 0) lines.push(`  return pick(${o.step}, [${o.fields.map(q).join(", ")}]);`);
  else if (o?.step) lines.push(`  return ${o.step};`);
  else if (calls.length) lines.push(`  return ${calls[calls.length - 1].id};`);
  else lines.push("  return {};");
  lines.push("}");
  return lines.join("\n") + (m.trailer ?? "\n");
}

/** moveStep moves a step and says why not when a step would run before what it reads. */
export function moveStep(f: Flow, from: number, to: number): { flow: Flow } | { error: string } {
  if (from === to) return { flow: f };
  const steps = [...f.steps];
  const [s] = steps.splice(from, 1);
  steps.splice(to, 0, { ...s, group: undefined });
  for (let i = 0; i < steps.length; i++) {
    const later = steps.slice(i + 1);
    const needs = stepReads(steps[i]);
    const bad = later.find(
      (x) => needs.includes(x.id) || (x.kind === "code" && codeDefines(x.code ?? "").some((d) => needs.includes(d))),
    );
    if (bad) return { error: `${steps[i].id} reads ${bad.id}, so it has to run after it.` };
  }
  // A Promise.all group stays together only while its members are next to each other.
  const fixed = steps.map((x, i) => {
    if (x.group === undefined) return x;
    const near = steps[i - 1]?.group === x.group || steps[i + 1]?.group === x.group;
    return near ? x : { ...x, group: undefined };
  });
  return { flow: { ...f, steps: fixed } };
}

/** stepReads lists the names a step reads: earlier steps' results. */
export function stepReads(s: Step): string[] {
  if (s.kind === "code") return [...(s.code ?? "").matchAll(/[A-Za-z_$][\w$]*/g)].map((m) => m[0]);
  return s.args.flatMap((a) => [
    ...sourceSteps(a.source).map((r) => r.step),
    ...(a.source.kind === "expr" || a.source.kind === "template"
      ? [...(a.source.kind === "expr" ? a.source.code : a.source.text).matchAll(/[A-Za-z_$][\w$]*/g)].map((m) => m[0])
      : []),
  ]);
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
    // getOwner({ id }) wants an owner's id: prefer ownerId / owner_id over another resource's id.
    if (a.name === "id" || a.name === "Id") {
      const noun = step.req
        .replace(/^(get|list|search|fetch|find|update|delete)/, "")
        .replace(/s$/, "")
        .toLowerCase();
      for (let i = before.length - 1; i >= 0 && !src && noun; i--) {
        const f = before[i].fields.find((x) => {
          const last = x.path.split(".").pop()?.toLowerCase() ?? "";
          return last === `${noun}id` || last === `${noun}_id`;
        });
        if (f) src = { kind: "step", step: before[i].step.id, path: f.path };
      }
    }
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
