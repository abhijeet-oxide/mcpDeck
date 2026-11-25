// Tests for the skill scripts. Run: node --test skills/
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { check, mask } from "./mcpdeck-build-tool/scripts/check-tool.mjs";
import { analyse, buildTrie, generate, project } from "./mcpdeck-trim-output/scripts/field-sizes.mjs";

const fixture = (name) => readFileSync(new URL(`./testdata/${name}`, import.meta.url), "utf8");

test("field-sizes: lists merge items under path[] and keep object depth", () => {
  const stats = analyse({ a: [{ b: 1 }, { b: 2 }] });
  const b = stats.find((s) => s.path === "a[].b");
  assert.equal(b.seen, 2);
  assert.equal(b.depth, 2);
});

test("field-sizes: --keep projects nested lists and drops everything else", () => {
  const data = JSON.parse(fixture("search-issues.json"));
  const trie = buildTrie(["total", "issues[].key", "issues[].fields.status.name"]);
  const out = project(data, trie);
  assert.deepEqual(Object.keys(out), ["total", "issues"]);
  assert.deepEqual(out.issues[0], { key: "SUP-1", fields: { status: { name: "Open" } } });
});

test("field-sizes: generated return statement produces the same projection", () => {
  const data = JSON.parse(fixture("search-issues.json"));
  const trie = buildTrie(["total", "issues[].key", "issues[].fields.summary", "odd key"]);
  const code = generate("res", trie);
  const fromCode = new Function("res", `return ${code};`)({ ...data, "odd key": 1 });
  assert.deepEqual(fromCode, project({ ...data, "odd key": 1 }, trie));
});

test("field-sizes: rejects malformed paths", () => {
  assert.throws(() => buildTrie(["a..b"]), /bad path/);
});

test("check-tool: a good tool has no errors or warnings", () => {
  assert.deepEqual(check(fixture("good-tool.ts")), { errors: [], warnings: [] });
});

test("check-tool: flags sandbox misuse, bad schema and agent-directed text", () => {
  const { errors, warnings } = check(fixture("bad-tool.ts"));
  const text = [...errors, ...warnings].map((e) => e.msg).join("\n");
  assert.match(text, /fetch\(\) isn't available/);
  assert.match(text, /imports aren't available/);
  assert.match(text, /name must be/);
  assert.match(text, /"region", which isn't in properties/);
  assert.match(text, /always call/);
  assert.match(text, /Promise\.all/);
  assert.match(text, /returns all of account/);
  assert.equal(errors.length, 4);
});

test("check-tool: comments and strings don't trigger sandbox errors", () => {
  const src = `// fetch(x)\nconst s = "require(y)";\n`;
  assert.doesNotMatch(mask(src, { strings: true }), /fetch\(|require\(/);
});
