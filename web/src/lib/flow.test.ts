import { describe, expect, it } from "vitest";
import { autoWire, type Flow, fieldsOf, generateFlow, parseFlow, parseLiteral, type ReqInfo } from "./flow";

const flow: Flow = {
  meta: {
    name: "find_image_vulnerabilities",
    description: "Lists fixable vulnerabilities of an app's newest image.",
    useWhen: "A question is about security issues in one app",
    dontUseWhen: "",
  },
  inputs: [{ name: "app_name", type: "string", description: "Application name", required: true }],
  steps: [
    { id: "login", api: "scanner", req: "login", args: [] },
    {
      id: "apps",
      api: "scanner",
      req: "listApplications",
      args: [
        { name: "name", source: { kind: "input", field: "app_name" } },
        { name: "Authorization", header: true, source: { kind: "template", text: "Bearer ${login.access_token}" } },
      ],
    },
    {
      id: "images",
      api: "scanner",
      req: "listImages",
      args: [{ name: "appId", source: { kind: "step", step: "apps", path: "items[0].id" } }],
    },
  ],
  output: { step: "images", fields: ["items[].tag", "total"] },
};

// REQ: FLOW-2
describe("flow engine", () => {
  it("round-trips a flow through code", () => {
    const code = generateFlow(flow);
    expect(code).toContain(
      "const apps = await scanner.listApplications({ name: input.app_name, headers: { Authorization: `Bearer ${login.access_token}` } });",
    );
    expect(code).toContain('return pick(images, ["items[].tag", "total"]);');
    const back = parseFlow(code);
    expect(back.ok).toBe(true);
    expect(back.flow).toEqual(flow);
    expect(generateFlow(back.flow!)).toBe(code);
  });

  it("reads hand-written code in the same shape", () => {
    const r = parseFlow(`export const tool = {
  name: 'get_x', description: "Gets x", // comment
  inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"], },
};
export default async function (input: Input) {
  // first the account
  const account = await crm.getAccount({ id: input.id });
  const owner = await crm.getOwner({ id: account.ownerId, verbose: true })
  return { name: account.name, owner: owner.name };
}`);
    expect(r.ok).toBe(true);
    expect(r.flow!.steps[1].args).toEqual([
      { name: "id", source: { kind: "step", step: "account", path: "ownerId" } },
      { name: "verbose", source: { kind: "literal", value: true } },
    ]);
    expect(r.flow!.output).toEqual({ code: "{ name: account.name, owner: owner.name }" });
  });

  it("says why custom logic can't be shown", () => {
    const r = parseFlow(`export const tool = { name: "a_b", description: "x", inputSchema: { type: "object" } };
export default async function (input: Input) {
  const a = await crm.getAccount({ id: input.id });
  if (!a) throw new Error("none");
  return a;
}`);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.line).toBe(4);
  });

  it("never evaluates code in tool", () => {
    expect(() => parseLiteral("{ a: alert(1) }")).toThrow();
    expect(() => parseLiteral("{ a: `${x}` }")).toThrow();
    expect(parseLiteral("{ a: [1, 'b', true, null], 'c-d': { e: -2.5 } }")).toEqual({
      a: [1, "b", true, null],
      "c-d": { e: -2.5 },
    });
  });

  // REQ: TOOL-10, FLOW-1
  it("wires a new step from earlier outputs, and adds a bearer header after a login", () => {
    const login = { id: "login", api: "scanner", req: "login", args: [] };
    const apps = { id: "apps", api: "scanner", req: "listApplications", args: [] };
    const info: ReqInfo = {
      api: "scanner",
      name: "listImages",
      method: "GET",
      url: "{{baseUrl}}/api/v1/applications/{appId}/images",
      query: [],
      bodyType: "none",
      body: "",
    };
    const before = [
      { step: login, fields: fieldsOf({ access_token: "t", user: { id: "u" } }, "wire") },
      { step: apps, fields: fieldsOf({ items: [{ id: "a1", name: "checkout" }], total: 1 }, "wire") },
    ];
    const w = autoWire({ id: "images", api: "scanner", req: "listImages", args: [] }, info, before, [], "none");
    expect(w.args).toEqual([
      { name: "appId", source: { kind: "step", step: "apps", path: "items[0].id" } },
      { name: "Authorization", header: true, source: { kind: "template", text: "Bearer ${login.access_token}" } },
    ]);
    expect(w.newInputs).toEqual([]);
  });

  it("doesn't mistake {{variables}} for parameters", () => {
    const info: ReqInfo = {
      api: "ncd",
      name: "login",
      method: "POST",
      url: "{{baseurl}}/login/{tenant}",
      query: [],
      bodyType: "json",
      body: '{"user":"{{u}}","pw":"{{p}}","remember":true}',
    };
    const w = autoWire({ id: "login", api: "ncd", req: "login", args: [] }, info, [], [], "none");
    expect(w.newInputs.map((i) => i.name)).toEqual(["tenant", "remember"]);
  });

  it("wires getOwner({ id }) from the account's ownerId, not its id", () => {
    const info: ReqInfo = {
      api: "crm",
      name: "getOwner",
      method: "GET",
      url: "{{baseUrl}}/owners/{id}",
      query: [],
      bodyType: "none",
      body: "",
    };
    const account = { id: "account", api: "crm", req: "getAccount", args: [] };
    const w = autoWire(
      { id: "owner", api: "crm", req: "getOwner", args: [] },
      info,
      [{ step: account, fields: fieldsOf({ id: "001", ownerId: "005", name: "N" }, "wire") }],
      [],
      "bearer",
    );
    expect(w.args).toEqual([{ name: "id", source: { kind: "step", step: "account", path: "ownerId" } }]);
  });
});
