import { describe, expect, it } from "vitest";
import { appName, appNameError, detectImport, toolkitNameError, toolkitSlug } from "./names";

// REQ: UI-14, TK-1
describe("names", () => {
  it("turns what people type into a toolkit name the server accepts", () => {
    expect(toolkitSlug("  Finance Ops ")).toBe("finance-ops");
    expect(toolkitSlug("Support_Desk!")).toBe("support-desk");
    expect(toolkitSlug("--x--")).toBe("x");
    expect(toolkitNameError("a")).toMatch(/two/);
    expect(toolkitNameError("Support", ["support"])).toMatch(/already exists/);
    expect(toolkitNameError("")).toBe("");
  });

  it("suggests a code name for an application and says what's wrong with a bad one", () => {
    expect(appName("Acme CRM")).toBe("acmeCRM");
    expect(appName("billing-api v2")).toBe("billingApiV2");
    expect(appNameError("crm")).toBe("");
    expect(appNameError("Crm")).toMatch(/lowercase/);
    expect(appNameError("crm-api")).toMatch(/Letters and digits/);
    expect(appNameError("crm", ["crm"])).toMatch(/already exists/);
  });

  it("detects dropped files", () => {
    expect(detectImport('{"info":{"_postman_id":"1"},"item":[]}')).toBe("postman");
    expect(detectImport('{"name":"Dev","values":[],"_postman_variable_scope":"environment"}')).toBe("postman-env");
    expect(detectImport('{"openapi":"3.0.3"}')).toBe("spec");
    expect(detectImport("openapi: 3.0.0\ninfo: {}")).toBe("spec");
    expect(detectImport("curl https://x.example")).toBe("curl");
    expect(detectImport("hello")).toBe("unknown");
  });
});
