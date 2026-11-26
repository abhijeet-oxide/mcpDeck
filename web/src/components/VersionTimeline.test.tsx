import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Version } from "../lib/types";
import { mockAPI, renderApp } from "../test/render";
import { unified, VersionTimeline } from "./VersionTimeline";

const v = (n: number, source: string, status: Version["status"] = "retired"): Version =>
  ({
    id: `v${n}`,
    n,
    source,
    status,
    createdByKind: "you",
    createdByName: "",
    createdAt: "2026-11-26T05:00:00Z",
    note: n === 2 ? "fix the limit" : undefined,
  }) as unknown as Version;

const one = Array.from({ length: 20 }, (_, i) => `line ${i}`).join("\n");
const two = one.replace("line 10", "line ten");
const three = `${two}\nline 20`;
// newest first, as the API returns them
const versions = [v(3, three, "live"), v(2, two), v(1, one)];

// REQ: VER-4
describe("unified diff", () => {
  it("numbers lines and folds unchanged runs around a change", () => {
    const d = unified(one, two);
    expect(d.added).toBe(1);
    expect(d.removed).toBe(1);
    const skipped = d.hunks.filter((h) => "skipped" in h);
    expect(skipped).toHaveLength(2);
    const shown = d.hunks.flatMap((h) => ("rows" in h ? h.rows : []));
    expect(shown.find((r) => r.kind === "add")).toMatchObject({ text: "line ten", b: 11 });
    expect(shown.find((r) => r.kind === "del")).toMatchObject({ text: "line 10", a: 11 });
  });
});

// REQ: VER-4
describe("VersionTimeline", () => {
  it("shows the selected version against the one before it", () => {
    renderApp(<VersionTimeline versions={versions} />);
    // latest selected: v3 adds one line against v2
    expect(screen.getByText("+1")).toBeInTheDocument();
    expect(screen.getByText("lines against v2")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /v2/ }));
    expect(screen.getByText("lines against v1")).toBeInTheDocument();
    expect(screen.getByText("line ten")).toBeInTheDocument();
    expect(screen.getByText("line 10")).toBeInTheDocument();
  });

  it("restores an older version only after confirming", async () => {
    const f = mockAPI({ "/api/versions/v1/rollback": { message: "Saved v4" } });
    renderApp(<VersionTimeline versions={versions} />);
    expect(screen.queryByRole("button", { name: "Restore this version" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /^v1/ }));
    fireEvent.click(screen.getByRole("button", { name: "Restore this version" }));
    expect(await screen.findByText(/as a new version, v4/)).toBeInTheDocument();
    expect(f).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Restore v1" }));
    await waitFor(() => expect(f).toHaveBeenCalled());
    expect(String(f.mock.calls[0][0])).toContain("/api/versions/v1/rollback");
  });

  it("explains the first version", () => {
    renderApp(<VersionTimeline versions={[v(1, one, "live")]} />);
    expect(screen.getByText(/The first version/)).toBeInTheDocument();
  });
});
