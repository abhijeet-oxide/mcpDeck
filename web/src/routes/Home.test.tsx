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
    { key: "toolkit", done: true },
    { key: "agent", done: true },
    { key: "tool", done: true },
  ],
  hideOnboarding: false,
  demo: false,
  daily: [],
};

// REQ: UI-1, UI-11
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
    expect(screen.getByText("12")).toBeInTheDocument();
    // All four steps done: no onboarding text anywhere.
    expect(screen.queryByText(/Getting started/)).not.toBeInTheDocument();
  });

  it("shows the first-run steps when there are no APIs, each locked until the one before", async () => {
    mockAPI({
      "/api/home": {
        ...base,
        apis: 0,
        needsYou: [],
        steps: [
          { key: "api", done: false },
          { key: "toolkit", done: false, reason: "Add an API first" },
          { key: "agent", done: false, reason: "Create a toolkit first" },
          { key: "tool", done: false, reason: "Connect an agent first" },
        ],
      },
    });
    renderApp(<Home />);
    expect(await screen.findByText("Welcome to mcpDeck")).toBeInTheDocument();
    expect(screen.getByText(/Add an API first/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add API" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "New toolkit" })).not.toBeInTheDocument();
  });

  it("says what went wrong when the server fails", async () => {
    mockAPI({});
    renderApp(<Home />);
    expect(await screen.findByText("not found")).toBeInTheDocument();
  });
});
