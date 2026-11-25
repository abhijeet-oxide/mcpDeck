import { describe, expect, it } from "vitest";
import { diffLines } from "../components/Diff";
import { bytes } from "./format";
import { match } from "./router";

describe("router.match", () => {
  it("extracts params", () => {
    expect(match("/toolkits/:tk/tools/:tool", "/toolkits/support/tools/customer_360")).toEqual({
      tk: "support",
      tool: "customer_360",
    });
    expect(match("/apis/:id", "/apis")).toBeNull();
  });
});

// REQ: VER-4
describe("diffLines", () => {
  it("marks added and removed lines", () => {
    const d = diffLines("a\nb\nc", "a\nc\nd");
    expect(d.map((l) => l.kind)).toEqual(["same", "del", "same", "add"]);
  });
});

describe("bytes", () => {
  it("formats sizes", () => {
    expect(bytes(512)).toBe("512 B");
    expect(bytes(4300)).toBe("4.2 KB");
  });
});
