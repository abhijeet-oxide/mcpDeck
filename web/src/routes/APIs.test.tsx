import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { APIs } from "./APIs";

// REQ: UI-4, APP-1
describe("APIs", () => {
  it("has a real empty state", async () => {
    mockAPI({ "/api/apis": [] });
    renderApp(<APIs />);
    expect(await screen.findByText("No APIs yet")).toBeInTheDocument();
  });
  it("shows each API with what needs attention", async () => {
    mockAPI({
      "/api/apis": [
        {
          id: "a1",
          name: "crm",
          title: "Acme CRM",
          source: "openapi",
          requests: 5,
          environments: ["Default"],
          tools: ["customer_360"],
          needsSecret: true,
          flagged: 1,
        },
      ],
    });
    renderApp(<APIs />);
    expect(await screen.findByText("crm")).toBeInTheDocument();
    expect(screen.getByText("add credential")).toBeInTheDocument();
    expect(screen.getByText("1 to check")).toBeInTheDocument();
  });
});
