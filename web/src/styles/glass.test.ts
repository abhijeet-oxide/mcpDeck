import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(join(__dirname, "glass.css"), "utf8");

// REQ: UI-15
describe("glass", () => {
  it("is solid when the user asks for less transparency or forced colours", () => {
    const i = css.indexOf("@media (prefers-reduced-transparency: reduce), (forced-colors: active)");
    expect(i).toBeGreaterThan(0);
    const block = css.slice(i);
    for (const v of ["--glass-chrome: var(--surface)", "--glass-panel: var(--surface)", "--backdrop: none"])
      expect(block).toContain(v);
    expect(block).toMatch(/backdrop-filter: none/);
  });

  it("only blurs behind a support check, and never blurs panels", () => {
    expect(css).toContain("@supports ((backdrop-filter: blur(1px))");
    const panel = css.match(/\n {2}\.panel \{[^}]*\}/)?.[0] ?? "";
    expect(panel).not.toContain("backdrop-filter");
  });

  it("keeps surfaces at least 70% opaque", () => {
    for (const m of css.matchAll(/var\(--(?:surface|sidebar)\) (\d+)%/g))
      expect(Number(m[1])).toBeGreaterThanOrEqual(70);
  });
});
