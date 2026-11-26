import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { APIDetail } from "../lib/types";
import { mockAPI, renderApp } from "../test/render";
import { AuthTab, TLSPanel } from "./APIWorkspace";

const base = {
  id: "a1",
  name: "vault",
  title: "Vault",
  description: "",
  source: "curl",
  allowedHosts: null,
  activeEnvId: "e1",
  createdAt: "2026-11-26T00:00:00Z",
  requests: [],
  variables: [{ id: "v1", envId: "e1", name: "clientKey", source: "secret", value: "" }],
} as unknown as APIDetail;

function patchBody(fetchFn: ReturnType<typeof mockAPI>) {
  const call = fetchFn.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "PATCH");
  return call ? JSON.parse(String((call[1] as RequestInit).body)) : undefined;
}

// REQ: API-9, API-10
describe("TLS settings", () => {
  it("keeps the client certificate when verification changes", async () => {
    const f = mockAPI({ "/api/apis/a1": {} });
    const a = {
      ...base,
      tls: { insecure: false, caCerts: "", clientCert: "-----BEGIN CERTIFICATE-----x", clientKey: "{{clientKey}}" },
    } as APIDetail;
    renderApp(<TLSPanel a={a} />);
    expect(screen.getByLabelText("Client certificate")).toHaveValue("-----BEGIN CERTIFICATE-----x");
    fireEvent.click(screen.getByRole("switch", { name: "Verify certificates" }));
    fireEvent.click(screen.getByRole("button", { name: "Save TLS settings" }));
    await waitFor(() => expect(patchBody(f)).toBeDefined());
    expect(patchBody(f).tls).toMatchObject({
      insecure: true,
      clientCert: "-----BEGIN CERTIFICATE-----x",
      clientKey: "{{clientKey}}",
    });
  });
});

// REQ: AUTH-3, AUTH-4
describe("Authentication", () => {
  it("edits OAuth 2.0 settings", async () => {
    const f = mockAPI({ "/api/apis/a1": {} });
    const a = {
      ...base,
      auth: { type: "oauth2", grant: "client_credentials", tokenUrl: "https://idp/token", clientId: "{{clientId}}" },
    } as APIDetail;
    renderApp(<AuthTab a={a} />);
    const url = screen.getByLabelText("Token URL");
    expect(url).toHaveValue("https://idp/token");
    fireEvent.change(screen.getByLabelText("Scope"), { target: { value: "billing.read" } });
    fireEvent.click(screen.getByRole("button", { name: "Save authentication" }));
    await waitFor(() => expect(patchBody(f)).toBeDefined());
    expect(patchBody(f).auth).toMatchObject({ type: "oauth2", tokenUrl: "https://idp/token", scope: "billing.read" });
  });
  it("edits HMAC signing with headers on every call", async () => {
    const f = mockAPI({ "/api/apis/a1": {} });
    const a = { ...base, auth: { type: "hmac", secret: "{{signingSecret}}", apply: [] } } as unknown as APIDetail;
    renderApp(<AuthTab a={a} />);
    fireEvent.change(screen.getByLabelText("String to sign"), { target: { value: "{{method}}\n{{path}}" } });
    fireEvent.click(screen.getByRole("button", { name: "Save authentication" }));
    await waitFor(() => expect(patchBody(f)).toBeDefined());
    expect(patchBody(f).auth).toMatchObject({
      type: "hmac",
      secret: "{{signingSecret}}",
      stringToSign: "{{method}}\n{{path}}",
    });
    expect(screen.getByText("Headers on every call")).toBeInTheDocument();
  });
});
