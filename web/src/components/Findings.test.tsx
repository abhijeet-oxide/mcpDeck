import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FindingList } from "./Findings";

// REQ: GUIDE-2
describe("FindingList", () => {
  it("puts blocking findings first and shows how to fix each", () => {
    render(
      <FindingList
        showTool
        findings={[
          { rule: "param-docs", level: "hint", title: "Parameters without a description", detail: "d", fix: "Add one" },
          {
            rule: "name",
            level: "block",
            title: "Generic name new_tool",
            detail: "d",
            fix: "Rename it",
            tool: "new_tool",
          },
        ]}
      />,
    );
    const titles = screen.getAllByText(/Generic name|Parameters without/).map((e) => e.textContent);
    expect(titles[0]).toBe("Generic name new_tool");
    expect(screen.getByText("Rename it")).toBeInTheDocument();
    expect(screen.getByText("new_tool")).toBeInTheDocument();
  });
  it("says so when there's nothing to fix", () => {
    render(<FindingList findings={[]} empty="All good." />);
    expect(screen.getByText("All good.")).toBeInTheDocument();
  });
});
