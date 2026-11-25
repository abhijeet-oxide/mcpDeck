#!/usr/bin/env node
// field-sizes: where the bytes in a JSON response go, and what to keep.
//
//   node field-sizes.mjs response.json
//   node field-sizes.mjs response.json --keep "name,balance,issues[].key"
//   cat response.json | node field-sizes.mjs - --depth 4 --top 40
//   node field-sizes.mjs response.json --json
//
// Paths use dots for fields and [] for list items: issues[].fields.summary
// Token numbers are estimates (bytes / 4). mcpDeck shows exact counts per call.
// No dependencies. Node 18 or newer.

import { readFileSync } from "node:fs";

const HELP = `usage: field-sizes <file|-> [--keep paths] [--depth n] [--top n] [--json]

  <file|->      JSON file to read, or - for stdin
  --keep a,b    comma-separated paths to keep; prints the size after
                keeping only these and a return statement that does it
  --depth n     deepest path level to list (default 3)
  --top n       how many paths to list (default 25)
  --json        machine-readable output`;

function parseArgs(argv) {
  const opts = { file: null, keep: null, depth: 3, top: 25, json: false };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "-h" || a === "--help") return { help: true };
    else if (a === "--keep") opts.keep = argv[++i];
    else if (a === "--depth") opts.depth = Number(argv[++i]);
    else if (a === "--top") opts.top = Number(argv[++i]);
    else if (a === "--json") opts.json = true;
    else if (opts.file === null) opts.file = a;
    else throw new Error(`unexpected argument: ${a}`);
  }
  if (opts.file === null) return { help: true };
  if (!Number.isInteger(opts.depth) || opts.depth < 1) throw new Error("--depth must be a positive integer");
  if (!Number.isInteger(opts.top) || opts.top < 1) throw new Error("--top must be a positive integer");
  return opts;
}

const bytesOf = (v) => (v === undefined ? 0 : Buffer.byteLength(JSON.stringify(v), "utf8"));
const approxTokens = (bytes) => Math.ceil(bytes / 4);
const isEmpty = (v) =>
  v === null || v === "" || (Array.isArray(v) && v.length === 0) ||
  (typeof v === "object" && v !== null && !Array.isArray(v) && Object.keys(v).length === 0);
const typeOf = (v) => (v === null ? "null" : Array.isArray(v) ? "list" : typeof v);
const URL_RE = /^https?:\/\/\S+$/;

// Walk the value and collect stats per path. Lists merge their items under path[].
export function analyse(root) {
  const stats = new Map();
  const visit = (value, path, depth) => {
    let s = stats.get(path);
    if (!s) {
      s = { path, depth, bytes: 0, seen: 0, empty: 0, types: new Set(), strLen: 0, strings: 0, urls: 0, maxItems: 0 };
      stats.set(path, s);
    }
    s.bytes += bytesOf(value);
    s.seen += 1;
    s.types.add(typeOf(value));
    if (isEmpty(value)) s.empty += 1;
    if (typeof value === "string") {
      s.strings += 1;
      s.strLen += value.length;
      if (URL_RE.test(value)) s.urls += 1;
    }
    if (Array.isArray(value)) {
      s.maxItems = Math.max(s.maxItems, value.length);
      for (const item of value) visit(item, `${path}[]`, depth);
    } else if (value && typeof value === "object") {
      for (const [k, v] of Object.entries(value)) visit(v, path === "" ? k : `${path}.${k}`, depth + 1);
    }
  };
  visit(root, "", 0);
  stats.delete("");
  return [...stats.values()];
}

function flagsFor(s) {
  const flags = [];
  if (s.empty === s.seen) flags.push("always empty: drop it");
  if (s.strings > 0 && s.urls === s.strings) flags.push("links only: agents rarely need these");
  if (s.strings > 0 && s.strLen / s.strings > 300) flags.push("long text: shorten or drop");
  if (s.maxItems > 20) flags.push(`up to ${s.maxItems} items: cap the list or summarise`);
  return flags;
}

// --keep: parse paths into a trie, project the value, and generate code.
function parsePath(p) {
  const tokens = [];
  for (const part of p.trim().split(".")) {
    if (part === "") throw new Error(`bad path: "${p}"`);
    const m = part.match(/^([^[\]]*)((?:\[\])*)$/);
    if (!m) throw new Error(`bad path segment "${part}" in "${p}"`);
    if (m[1] !== "") tokens.push({ key: m[1] });
    for (let i = 0; i < m[2].length / 2; i++) tokens.push({ list: true });
  }
  return tokens;
}

export function buildTrie(paths) {
  const root = { children: new Map(), leaf: false };
  for (const p of paths) {
    let node = root;
    for (const t of parsePath(p)) {
      const id = t.list ? "[]" : `.${t.key}`;
      if (!node.children.has(id)) node.children.set(id, { children: new Map(), leaf: false, key: t.key });
      node = node.children.get(id);
    }
    node.leaf = true;
  }
  return root;
}

export function project(value, node) {
  if (node.leaf || value === null || value === undefined) return value;
  const list = node.children.get("[]");
  if (list) return Array.isArray(value) ? value.map((v) => project(v, list)) : undefined;
  if (typeof value !== "object" || Array.isArray(value)) return undefined;
  const out = {};
  for (const child of node.children.values()) {
    if (child.key in value) out[child.key] = project(value[child.key], child);
  }
  return out;
}

const IDENT = /^[A-Za-z_$][\w$]*$/;
const access = (expr, key) => (IDENT.test(key) ? `${expr}?.${key}` : `${expr}?.[${JSON.stringify(key)}]`);
const prop = (key) => (IDENT.test(key) ? key : JSON.stringify(key));

// level = indentation of the line the generated expression starts on.
export function generate(expr, node, level = 0, names = { n: 0 }) {
  if (node.leaf) return expr;
  const list = node.children.get("[]");
  if (list) {
    const v = names.n++ === 0 ? "item" : `item${names.n}`;
    const body = generate(v, list, level, names);
    return `${expr}?.map((${v}) => ${body.startsWith("{") ? `(${body})` : body})`;
  }
  const pad = "  ".repeat(level + 1);
  const lines = [...node.children.values()].map(
    (c) => `${pad}${prop(c.key)}: ${generate(access(expr, c.key), c, level + 1, names)},`,
  );
  return `{\n${lines.join("\n")}\n${"  ".repeat(level)}}`;
}

function fmt(n) {
  return n.toLocaleString("en-US");
}

function main() {
  let opts;
  try {
    opts = parseArgs(process.argv.slice(2));
  } catch (e) {
    console.error(`field-sizes: ${e.message}\n\n${HELP}`);
    process.exit(2);
  }
  if (opts.help) {
    console.log(HELP);
    return;
  }
  let data;
  try {
    const text = readFileSync(opts.file === "-" ? 0 : opts.file, "utf8");
    data = JSON.parse(text);
  } catch (e) {
    console.error(`field-sizes: can't read JSON from ${opts.file}: ${e.message}`);
    process.exit(1);
  }

  const total = bytesOf(data);
  const rows = analyse(data)
    .filter((s) => s.depth <= opts.depth)
    .sort((a, b) => b.bytes - a.bytes)
    .slice(0, opts.top)
    .map((s) => ({
      path: s.path,
      bytes: s.bytes,
      share: total ? s.bytes / total : 0,
      type: [...s.types].join("|"),
      seen: s.seen,
      flags: flagsFor(s),
    }));

  let keep = null;
  if (opts.keep) {
    const paths = opts.keep.split(",").map((p) => p.trim()).filter(Boolean);
    let trie;
    try {
      trie = buildTrie(paths);
    } catch (e) {
      console.error(`field-sizes: ${e.message}`);
      process.exit(2);
    }
    const kept = project(data, trie);
    const keptBytes = bytesOf(kept);
    keep = { paths, bytes: keptBytes, saved: total ? 1 - keptBytes / total : 0, code: `return ${generate("res", trie)};` };
  }

  if (opts.json) {
    console.log(JSON.stringify({ totalBytes: total, approxTokens: approxTokens(total), fields: rows, keep }, null, 2));
    return;
  }

  console.log(`total  ${fmt(total)} bytes  ≈ ${fmt(approxTokens(total))} tokens (bytes ÷ 4)\n`);
  const w = Math.min(60, Math.max(4, ...rows.map((r) => r.path.length)));
  console.log(`${"path".padEnd(w)}  ${"bytes".padStart(9)}  ${"share".padStart(6)}  type`);
  for (const r of rows) {
    const path = r.path.length > w ? `${r.path.slice(0, w - 1)}…` : r.path;
    console.log(
      `${path.padEnd(w)}  ${fmt(r.bytes).padStart(9)}  ${`${(r.share * 100).toFixed(1)}%`.padStart(6)}  ${r.type}` +
        (r.flags.length ? `   ← ${r.flags.join("; ")}` : ""),
    );
  }
  if (keep) {
    console.log(
      `\nkeeping ${keep.paths.length} path(s): ${fmt(keep.bytes)} bytes ≈ ${fmt(approxTokens(keep.bytes))} tokens` +
        `  (${(keep.saved * 100).toFixed(0)}% smaller)\n`,
    );
    console.log("// replace `res` with the variable holding the response");
    console.log(keep.code);
  } else {
    console.log('\nnext: --keep "path,path[].field" to see the size and code for a smaller output');
  }
}

if (import.meta.url === `file://${process.argv[1]}` || process.argv[1]?.endsWith("field-sizes.mjs")) main();
