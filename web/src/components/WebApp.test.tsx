import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { BuildWithAgent, ConfirmBanner, FilesPanel, RecordDialog } from "./WebApp";

const calls = (fn: ReturnType<typeof mockAPI>) =>
  (fn.mock.calls as unknown as [unknown, RequestInit | undefined][]).map(
    ([u, init]) => `${init?.method ?? "GET"} ${String(u)} ${init?.body ?? ""}`,
  );

// REQ: CONF-1
describe("Confirm each call", () => {
  it("shows what waits and confirms it", async () => {
    const fn = mockAPI({
      "/api/confirmations": [
        {
          id: "c1",
          title: "Submit Internet claim of 48.00",
          fields: { amount: "48.00", receipt: "december.pdf" },
          agent: "Copilot",
          at: "",
        },
      ],
      "/api/confirmations/c1": {},
    });
    renderApp(<ConfirmBanner />);
    await screen.findByText("Submit Internet claim of 48.00");
    expect(screen.getByText("december.pdf")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    await waitFor(() => expect(calls(fn)).toContain('POST /api/confirmations/c1 {"confirm":true}'));
  });
});

// REQ: REC-1
describe("Record a task", () => {
  it("starts recording at the start page", async () => {
    const fn = mockAPI({ "/api/record": { api: "expenses", signedIn: true, requests: [] } });
    renderApp(<RecordDialog api="expenses" startUrl="https://exp.corp/claims" onClose={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: /Open browser and record/ }));
    await waitFor(() =>
      expect(calls(fn)).toContain('POST /api/record {"api":"expenses","url":"https://exp.corp/claims"}'),
    );
    await screen.findByText(/Added 0 requests/);
  });
});

// REQ: HTTP-3
describe("Files folder", () => {
  it("saves the folder", async () => {
    const fn = mockAPI({ "/api/apis/a1/files-folder": {} });
    renderApp(<FilesPanel apiId="a1" />);
    fireEvent.change(screen.getByLabelText("Files folder"), { target: { value: "/home/me/Receipts" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls(fn)).toContain('PUT /api/apis/a1/files-folder {"dir":"/home/me/Receipts"}'));
  });
});

// REQ: CON-1
describe("Build with an agent", () => {
  it("offers a one-click link for VS Code", async () => {
    mockAPI({
      "/api/connect-builder": {
        plan: { link: "vscode:mcp/install?x", command: "code --add-mcp x", firstPrompt: "Use the mcpdeck tools" },
        written: [],
      },
    });
    renderApp(<BuildWithAgent />);
    const link = await screen.findByRole("link", { name: /Add to VS Code/ });
    expect(link).toHaveAttribute("href", "vscode:mcp/install?x");
  });
});
