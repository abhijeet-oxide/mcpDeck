import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { Home } from "./Home";

const base = {
  apis: 2,
  requests: 9,
  toolkits: 1,
  tools: 3,
  agents: 1,
  activeAgents: 1,
  callsToday: 12,
  tokensToday: 900,
  errorsToday: 1,
  blockedToday: 0,
  recent: [],
  steps: [
    { key: "api", done: true },
    { key: "send", done: true },
    { key: "example", done: true },
    { key: "toolkit", done: true },
    { key: "tool", done: true },
    { key: "agent", done: true },
  ],
  hideOnboarding: false,
  welcomeSeen: true,
  demo: false,
  daily: [],
  health: [
    {
      name: "security",
      tools: 2,
      listTokens: 410,
      score: 70,
      must: 1,
      avoid: 1,
      top: "Generic name new_tool",
      agents: 1,
    },
  ],
  heaviest: [],
  fetched: 4000,
  returned: 400,
  calls7d: 20,
  errors7d: 0,
};

// REQ: UI-1, UI-11, GUIDE-4
describe("Home", () => {
  it("lists what needs a person, with the action inline", async () => {
    mockAPI({
      "/api/home": {
        ...base,
        needsYou: [
          {
            kind: "review",
            id: "v1",
            title: "reassign_ticket v1",
            detail: "Assign a ticket",
            toolkit: "support",
            agent: "Claude Code",
            at: new Date().toISOString(),
          },
          {
            kind: "request",
            id: "r1",
            title: "Tool request",
            detail: "Credit holds this week",
            toolkit: "support",
            agent: "Cursor",
            at: new Date().toISOString(),
          },
        ],
      },
      "/api/toolkits": [],
      "/api/agents": [],
    });
    renderApp(<Home />);
    expect(await screen.findByText("reassign_ticket v1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Review" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy prompt" })).toBeInTheDocument();
    expect(screen.getByText(/12 today/)).toBeInTheDocument();
    // Toolkit health and savings (GUIDE-4).
    expect(screen.getByText("Generic name new_tool")).toBeInTheDocument();
    expect(screen.getByText("1 must fix")).toBeInTheDocument();
    expect(screen.getByText("90%")).toBeInTheDocument();
    // All four steps done: no onboarding text anywhere.
    expect(screen.queryByText(/Getting started/)).not.toBeInTheDocument();
  });

  it("guides a new user through every step, in any order, saying what each needs", async () => {
    mockAPI({
      "/api/home": {
        ...base,
        apis: 0,
        needsYou: [],
        steps: [
          { key: "api", done: false },
          { key: "send", done: false, reason: "Add an application first" },
          { key: "example", done: false, reason: "Send a request first" },
          { key: "toolkit", done: false },
          { key: "tool", done: false, reason: "Create a toolkit first" },
          { key: "agent", done: false, reason: "Create a toolkit first" },
        ],
      },
      "/api/info": { demo: false },
      "/api/apis": [],
      "/api/toolkits": [],
    });
    renderApp(<Home />);
    expect(await screen.findByText("Getting started")).toBeInTheDocument();
    expect(screen.getByText("Add an application first")).toBeInTheDocument();
    // The next step and a step with no prerequisite can both be started.
    expect(screen.getAllByRole("button", { name: "Create application" })[0]).toBeEnabled();
    expect(screen.getByRole("button", { name: "Create toolkit" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "New tool" })).toBeDisabled();
    // The demo is always one click away.
    expect(screen.getAllByRole("button", { name: "Demo" }).length).toBeGreaterThan(0);
  });

  it("says what went wrong when the server fails", async () => {
    mockAPI({});
    renderApp(<Home />);
    expect(await screen.findByText("not found")).toBeInTheDocument();
  });
});
