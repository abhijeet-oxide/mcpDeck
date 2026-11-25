import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { APIs } from "./APIs";

// REQ: UI-4, APP-1
describe("Applications", () => {
  it("has a real empty state", async () => {
    mockAPI({ "/api/apis": [] });
    renderApp(<APIs />);
    expect(await screen.findByText("Add your first application")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create application" })).toBeInTheDocument();
  });
  it("shows each application with what needs attention", async () => {
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
    expect(await screen.findByText("Acme CRM")).toBeInTheDocument();
    expect(screen.getByText("crm")).toBeInTheDocument();
    expect(screen.getByText(/Needs a credential/)).toBeInTheDocument();
    expect(screen.getByText(/1 to check/)).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /New application/ }).length).toBeGreaterThan(0);
  });
});
