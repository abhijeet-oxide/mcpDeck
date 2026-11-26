import { fireEvent, screen, waitFor } from "@testing-library/react";
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

  // REQ: NET-1, NET-2, CRED-2
  it("sets the timeout and a proxy, sending only what changed and never showing the password", async () => {
    const f = mockAPI({
      "/api/settings": {
        values: {
          requestTimeoutSeconds: "30",
          requestTimeoutMaxSeconds: "300",
          proxyMode: "manual",
          proxyHttp: "proxy.corp:3128",
          proxyHttps: "",
          noProxy: "",
          proxyUser: "me",
          proxyPasswordSet: "true",
        },
        port: 47800,
        dataDir: "/tmp/x",
        database: "/tmp/x/mcpdeck.db",
        keychain: "os keychain",
        demo: false,
        version: "dev",
      },
    });
    renderApp(<Settings section="network" />);
    expect(await screen.findByText("Set · Encrypted")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Default timeout"), { target: { value: "90" } });
    // Replace, then leave it empty: the stored password is kept.
    fireEvent.click(screen.getByRole("button", { name: "Replace Proxy password" }));
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(putBody(f)).toBeDefined());
    expect(putBody(f)).toEqual({ requestTimeoutSeconds: "90" });
  });

  it("sends a new proxy password, or an empty one to remove it", async () => {
    const f = mockAPI({
      "/api/settings": {
        values: { proxyMode: "manual", proxyPasswordSet: "true" },
        port: 47800,
        dataDir: "/tmp/x",
        database: "/tmp/x/mcpdeck.db",
        keychain: "os keychain",
        demo: false,
        version: "dev",
      },
    });
    renderApp(<Settings section="network" />);
    fireEvent.click(await screen.findByRole("button", { name: "Remove Proxy password" }));
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(putBody(f)).toEqual({ proxyPassword: "" }));
  });

  // REQ: CRED-2
  it("says where secrets and their key are stored", async () => {
    mockAPI({
      "/api/settings": {
        values: {},
        port: 47800,
        dataDir: "/tmp/x",
        database: "/tmp/x/mcpdeck.db",
        keychain: "key file",
        keyFile: "/tmp/x/mcpdeck.key",
        demo: false,
        version: "dev",
      },
    });
    renderApp(<Settings section="security" />);
    expect(await screen.findByText("/tmp/x/mcpdeck.db")).toBeInTheDocument();
    expect(screen.getByText("Key file")).toBeInTheDocument();
    expect(screen.getByText("/tmp/x/mcpdeck.key")).toBeInTheDocument();
  });
});

function putBody(f: ReturnType<typeof mockAPI>) {
  const calls = f.mock.calls as unknown as [unknown, RequestInit | undefined][];
  const c = calls.find((x) => x[1]?.method === "PUT");
  return c ? JSON.parse(String(c[1]?.body)) : undefined;
}
