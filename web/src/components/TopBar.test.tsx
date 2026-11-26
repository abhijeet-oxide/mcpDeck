import { fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { Palette } from "./Palette";
import { TopBar } from "./Sidebar";

function prefersDark(dark: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((q: string) => ({
      matches: dark && q.includes("dark"),
      media: q,
      addEventListener: () => {},
      removeEventListener: () => {},
    })),
  );
}

afterEach(() => {
  delete document.documentElement.dataset.theme;
  localStorage.clear();
  vi.unstubAllGlobals();
});

// REQ: UI-8
describe("theme toggle", () => {
  it("follows the computer until you choose, then keeps your choice", () => {
    prefersDark(true);
    renderApp(<TopBar onSearch={() => {}} />);
    const btn = screen.getByRole("button", { name: "Switch to light theme" });
    expect(document.documentElement.dataset.theme).toBeUndefined();
    fireEvent.click(btn);
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(localStorage.getItem("mcpdeck-theme")).toBe("light");
    fireEvent.click(screen.getByRole("button", { name: "Switch to dark theme" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
  });
});

// REQ: UI-6
describe("search", () => {
  it("sits on the right of the top bar and opens the palette", () => {
    prefersDark(false);
    const onSearch = vi.fn();
    renderApp(<TopBar onSearch={onSearch} />);
    const search = screen.getByRole("button", { name: "Search or run a command" });
    const theme = screen.getByRole("button", { name: "Switch to dark theme" });
    expect(search.compareDocumentPosition(theme) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(search.previousElementSibling).toHaveClass("crumb");
    fireEvent.click(search);
    expect(onSearch).toHaveBeenCalled();
  });

  it("finds requests across applications and opens them", async () => {
    prefersDark(false);
    mockAPI({
      "/api/apis": [{ id: "a1", name: "shop", title: "Shop" }],
      "/api/apis/a1": { id: "a1", name: "shop", requests: [{ id: "r9", name: "refundOrder", method: "POST" }] },
      "/api/toolkits": [],
      "/api/tools": [],
      "/api/agents": [],
    });
    renderApp(<Palette onClose={() => {}} onConnect={() => {}} />);
    fireEvent.change(screen.getByLabelText("Search"), { target: { value: "refund" } });
    const hit = await screen.findByRole("button", { name: /refundOrder/ });
    expect(hit).toHaveTextContent("Request · POST · shop");
    fireEvent.click(hit);
    await waitFor(() => expect(window.location.pathname + window.location.search).toBe("/apps/a1?req=r9"));
  });
});
