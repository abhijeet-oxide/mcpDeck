import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { Settings } from "./Settings";

// REQ: UI-4, UI-8
describe("Settings", () => {
  it("offers Save only once something changed", async () => {
    mockAPI({
      "/api/settings": {
        values: {
          toolTimeoutSeconds: "10",
          retentionBodiesDays: "30",
          retentionCallsDays: "365",
          hideOnboarding: "false",
        },
        port: 47800,
        dataDir: "/tmp/x",
        keychain: "os keychain",
        demo: false,
        version: "dev",
      },
    });
    renderApp(<Settings />);
    await screen.findByText("Tool timeout");
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
    fireEvent.change(screen.getByDisplayValue("10"), { target: { value: "20" } });
    expect(screen.getByRole("button", { name: "Save changes" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
    expect(screen.queryByText(/demo/i)).not.toBeInTheDocument();
  });

  // REQ: UI-8
  it("shows one section per page and sets the accent colour", async () => {
    mockAPI({
      "/api/settings": {
        values: { toolTimeoutSeconds: "10" },
        port: 47800,
        dataDir: "/tmp/x",
        keychain: "os keychain",
        demo: false,
        version: "dev",
      },
    });
    renderApp(<Settings section="appearance" />);
    await screen.findByText("Accent colour");
    expect(screen.queryByText("Tool timeout")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Appearance/ })).toHaveAttribute("aria-current", "page");
    fireEvent.click(screen.getByRole("button", { name: "Teal" }));
    expect(document.documentElement.dataset.accent).toBe("teal");
    fireEvent.click(screen.getByRole("button", { name: "Blue" }));
    expect(document.documentElement.dataset.accent).toBeUndefined();
  });
});
