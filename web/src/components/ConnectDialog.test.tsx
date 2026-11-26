import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { ConnectDialog } from "./ConnectDialog";

// REQ: AGT-3
describe("Connect agent", () => {
  it("lists VS Code first and picks it by default", async () => {
    mockAPI({ "/api/toolkits": [], "/api/agents": [] });
    renderApp(<ConnectDialog onClose={() => {}} />);
    await screen.findByText("Install link or mcp.json + custom agent");
    const order = Array.from(document.querySelectorAll("button.choice"));
    const names = ["VS Code", "Claude Code", "Cursor", "Claude Desktop"];
    names.forEach((n, i) => expect(order[i].textContent).toContain(n));
    expect(order[0]).toHaveAttribute("aria-pressed", "true");
  });
});
