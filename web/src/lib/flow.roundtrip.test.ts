// Every tool mcpDeck ships or writes opens in Visual and comes back as the
// same code (FLOW-1). The fixture is written by
// internal/mock/shipped_test.go from the demo, the examples, the skill's
// sample tools and the generator's starter for every demo request.
import { describe, expect, it } from "vitest";
import shipped from "../test/fixtures/shipped-tools.json";
import { generateFlow, matchBrace, parseFlow, parseLiteral } from "./flow";
import { starterTemplate } from "./starter";

const tools: { origin: string; source: string }[] = [
  ...shipped,
  { origin: "editor/starter", source: starterTemplate() },
];

/** split cuts a tool file into the `tool` literal and the code around it. */
function split(src: string) {
  const t = /export\s+const\s+tool\s*=\s*\{/.exec(src)!;
  const open = t.index + t[0].length - 1;
  const close = matchBrace(src, open);
  return { tool: parseLiteral(src.slice(open, close + 1)), code: src.slice(0, open) + src.slice(close + 1) };
}

/** same ignores layout: whitespace, semicolons and trailing commas. */
const same = (s: string) =>
  s
    .replace(/\s+/g, "")
    .replace(/;/g, "")
    .replace(/,([)\]}])/g, "$1");

describe("visual ⇄ code round trip", () => {
  it("covers the demo, examples, skill samples and generated tools", () => {
    expect(tools.filter((t) => t.origin.startsWith("demo/")).length).toBeGreaterThanOrEqual(4);
    expect(tools.filter((t) => t.origin.startsWith("generated/")).length).toBeGreaterThanOrEqual(10);
    expect(tools.some((t) => t.origin.startsWith("examples/"))).toBe(true);
  });
  for (const t of tools) {
    it(`${t.origin} opens in Visual and keeps its code`, () => {
      const p = parseFlow(t.source);
      if (!p.ok) throw new Error(`${t.origin}: ${p.reason}`);
      const out = generateFlow(p.flow);
      const a = split(t.source);
      const b = split(out);
      expect(b.tool).toEqual(a.tool);
      expect(same(b.code)).toBe(same(a.code));
      // Generating again changes nothing.
      const again = parseFlow(out);
      expect(again.ok).toBe(true);
      if (again.ok) expect(generateFlow(again.flow)).toBe(out);
    });
  }
  it("shows request calls as steps, not code, in the demo and generated tools", () => {
    for (const t of tools.filter((x) => x.origin.startsWith("generated/") || x.origin === "demo/customer_360")) {
      const p = parseFlow(t.source);
      if (!p.ok) throw new Error(p.reason);
      expect(p.flow.steps.filter((s) => s.kind !== "code").length, t.origin).toBeGreaterThan(0);
    }
    const c = parseFlow(tools.find((t) => t.origin === "demo/customer_360")!.source);
    if (!c.ok) throw new Error(c.reason);
    expect(c.flow.steps.map((s) => [s.id, s.kind ?? "call", s.group ?? null])).toEqual([
      ["account", "call", null],
      ["summary", "call", 1],
      ["tickets", "call", 1],
    ]);
  });
});
