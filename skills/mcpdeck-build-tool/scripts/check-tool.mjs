#!/usr/bin/env node
// check-tool: catch problems in an mcpDeck tool file before save_tool does.
//
//   node check-tool.mjs customer_360.ts
//   node check-tool.mjs customer_360.ts --json
//
// Errors (exit 1) are things save_tool will reject. Warnings are things that
// cost the agent tokens or time. This is a static check: it doesn't run the
// tool or type-check it. No dependencies. Node 18 or newer.

import { readFileSync } from "node:fs";
import vm from "node:vm";

// Replace comments (and, if asked, string contents) with spaces, keeping
// offsets so line numbers stay right.
export function mask(src, { strings = false } = {}) {
  let out = "";
  let i = 0;
  const blank = (s) => s.replace(/[^\n]/g, " ");
  while (i < src.length) {
    const c = src[i];
    const n = src[i + 1];
    if (c === "/" && n === "/") {
      const end = src.indexOf("\n", i);
      const stop = end === -1 ? src.length : end;
      out += blank(src.slice(i, stop));
      i = stop;
    } else if (c === "/" && n === "*") {
      const end = src.indexOf("*/", i + 2);
      const stop = end === -1 ? src.length : end + 2;
      out += blank(src.slice(i, stop));
      i = stop;
    } else if (c === '"' || c === "'" || c === "`") {
      let j = i + 1;
      while (j < src.length && src[j] !== c) j += src[j] === "\\" ? 2 : 1;
      const lit = src.slice(i, j + 1);
      out += strings ? c + blank(lit.slice(1, -1)) + (j < src.length ? c : "") : lit;
      i = j + 1;
    } else {
      out += c;
      i++;
    }
  }
  return out;
}

const lineAt = (src, idx) => src.slice(0, idx).split("\n").length;

// Find `export const tool = { ... }` and return the object literal text.
function toolLiteral(src) {
  const code = mask(src, { strings: true });
  const m = /export\s+const\s+tool\s*(?::[^=]+)?=\s*\{/.exec(code);
  if (!m) return null;
  const start = m.index + m[0].length - 1;
  let depth = 0;
  for (let i = start; i < code.length; i++) {
    if (code[i] === "{") depth++;
    else if (code[i] === "}" && --depth === 0) return { text: src.slice(start, i + 1), line: lineAt(src, start) };
  }
  return null;
}

const FORBIDDEN = [
  [/\bfetch\s*\(/, "fetch() isn't available in the sandbox; call an API client (crm.getAccount(...)) or call()"],
  [/\bXMLHttpRequest\b/, "XMLHttpRequest isn't available; use an API client or call()"],
  [/\brequire\s*\(/, "require() isn't available; the API clients, call() and vars are the only globals"],
  [/^\s*import\s+(?!type\b)/m, "imports aren't available; the API clients, call() and vars are globals"],
  [/\bprocess\./, "process isn't available; use vars.<name> for configuration"],
  [/\b(?:Deno|Bun)\./, "runtime globals aren't available in the sandbox"],
  [/\beval\s*\(/, "eval isn't allowed in the sandbox"],
  [/\bnew\s+Function\s*\(/, "new Function isn't allowed in the sandbox"],
  [/\bset(?:Timeout|Interval)\s*\(/, "timers aren't available; tools run once and return"],
];

const AGENT_DIRECTED = [/\bignore\b/i, /\balways (?:call|use)\b/i, /\byou must\b/i, /\bdo not tell\b/i, /https?:\/\//i];

export function check(src) {
  const errors = [];
  const warnings = [];
  const err = (line, msg) => errors.push({ line, msg });
  const warn = (line, msg) => warnings.push({ line, msg });
  const code = mask(src, { strings: true });

  for (const [re, msg] of FORBIDDEN) {
    const m = re.exec(code);
    if (m) err(lineAt(src, m.index), msg);
  }

  const fn = /export\s+default\s+async\s+function\b/.exec(code);
  if (!fn) err(1, "missing `export default async function (input) { ... }`");

  const lit = toolLiteral(src);
  let tool = null;
  if (!lit) {
    err(1, "missing `export const tool = { name, description, inputSchema }`");
  } else {
    try {
      tool = vm.runInNewContext(`(${lit.text})`, Object.create(null), { timeout: 100 });
    } catch (e) {
      err(lit.line, `tool definition must be a plain object literal (no variables or calls): ${e.message}`);
    }
  }

  if (tool) {
    const at = lit.line;
    if (typeof tool.name !== "string" || !/^[a-z][a-z0-9_]{2,63}$/.test(tool.name)) {
      err(at, "name must be 3–64 characters of lowercase letters, digits and _ (e.g. customer_360)");
    }
    const d = typeof tool.description === "string" ? tool.description.trim() : "";
    if (d.length < 20) err(at, "description must say what the tool returns, in at least 20 characters");
    if (d.length > 600) warn(at, `description is ${d.length} characters; it's sent in every tools/list, keep it under 600`);
    for (const re of AGENT_DIRECTED) {
      if (re.test(d)) {
        warn(at, `description contains "${d.match(re)[0]}"; text that instructs agents or links out is held for review`);
        break;
      }
    }
    if (!tool.useWhen) warn(at, "add useWhen: one line on when an agent should pick this tool");

    const s = tool.inputSchema;
    if (!s || typeof s !== "object") {
      err(at, "inputSchema is required (JSON Schema, type: \"object\")");
    } else {
      if (s.type !== "object") err(at, 'inputSchema.type must be "object"');
      const props = s.properties && typeof s.properties === "object" ? s.properties : {};
      if (s.properties !== undefined && (typeof s.properties !== "object" || Array.isArray(s.properties))) {
        err(at, "inputSchema.properties must be an object");
      }
      for (const r of Array.isArray(s.required) ? s.required : []) {
        if (!(r in props)) err(at, `inputSchema.required lists "${r}", which isn't in properties`);
      }
      for (const [k, v] of Object.entries(props)) {
        if (v && typeof v === "object" && !v.examples && !v.description && !v.enum) {
          warn(at, `input "${k}" has no description or examples; agents guess formats wrong without them`);
        }
      }
    }
  }

  // Sequential awaits that don't depend on each other could run in parallel.
  const awaits = [...code.matchAll(/(?:const|let)\s+(\w+)\s*=\s*await\s+([\w.]+)\s*\(([^;]*?)\)\s*;?\s*\n/g)];
  for (let i = 1; i < awaits.length; i++) {
    const [prevName, prevVar] = [awaits[i - 1][2], awaits[i - 1][1]];
    const [, , callee, args] = awaits[i];
    if (!new RegExp(`\\b${prevVar}\\b`).test(args)) {
      warn(
        lineAt(src, awaits[i].index),
        `${callee}() doesn't use ${prevVar} from ${prevName}(); run them together with Promise.all`,
      );
    }
  }

  const ret = /\breturn\s+([A-Za-z_$][\w$]*)\s*;/.exec(code);
  if (ret) {
    warn(lineAt(src, ret.index), `returns all of ${ret[1]}; return only the fields the agent needs (see the mcpdeck-trim-output skill)`);
  }

  return { errors, warnings };
}

function main() {
  const args = process.argv.slice(2);
  const json = args.includes("--json");
  const file = args.find((a) => !a.startsWith("--"));
  if (!file || args.includes("-h") || args.includes("--help")) {
    console.log("usage: check-tool <tool.ts> [--json]");
    process.exit(file ? 0 : 2);
  }
  let src;
  try {
    src = readFileSync(file, "utf8");
  } catch (e) {
    console.error(`check-tool: can't read ${file}: ${e.message}`);
    process.exit(1);
  }
  const res = check(src);
  if (json) {
    console.log(JSON.stringify(res, null, 2));
  } else {
    for (const e of res.errors) console.log(`${file}:${e.line}  error    ${e.msg}`);
    for (const w of res.warnings) console.log(`${file}:${w.line}  warning  ${w.msg}`);
    if (!res.errors.length && !res.warnings.length) console.log(`${file}: ok`);
    else console.log(`\n${res.errors.length} error(s), ${res.warnings.length} warning(s)`);
  }
  process.exit(res.errors.length ? 1 : 0);
}

if (process.argv[1]?.endsWith("check-tool.mjs")) main();
