import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { resetFlights } from "../lib/inflight";
import type { APIDetail, Request } from "../lib/types";
import { AuthTab } from "../routes/APIWorkspace";
import { mockAPI, renderApp } from "../test/render";
import { byLogin, fieldsOf } from "./CapturePicker";

afterEach(() => resetFlights());

const req = (p: Partial<Request>): Request =>
  ({
    apiId: "a1",
    folder: "",
    title: "",
    description: "",
    method: "GET",
    url: "{{baseUrl}}/x",
    headers: [],
    query: [],
    bodyType: "none",
    body: "",
    access: "read",
    accessReason: "",
    accessFlag: false,
    ...p,
  }) as Request;

const reqs = [
  req({ id: "r1", name: "listOrders", url: "{{baseUrl}}/orders" }),
  req({ id: "r2", name: "createOrder", method: "POST", url: "{{baseUrl}}/orders" }),
  req({
    id: "r3",
    name: "signIn",
    method: "POST",
    url: "{{baseUrl}}/auth/login",
    bodyType: "json",
    body: '{"user":"{{user}}","password":"{{password}}"}',
    responses: [
      {
        id: "s1",
        name: "ok",
        status: 200,
        savedAt: "2026-11-26T00:00:00Z",
        body: { data: { access_token: "eyJabc", expires_in: 3600 }, user: { id: 7 } },
      },
    ],
  }),
];

const api = (auth: APIDetail["auth"]) =>
  ({
    id: "a1",
    name: "shop",
    title: "Shop",
    activeEnvId: "e1",
    requests: reqs,
    variables: [],
    environments: [],
    auth,
  }) as unknown as APIDetail;

function patchBody(f: ReturnType<typeof mockAPI>) {
  const calls = f.mock.calls as unknown as [unknown, RequestInit | undefined][];
  const call = calls.find((c) => c[1]?.method === "PATCH");
  return call ? JSON.parse(String(call[1]?.body)) : undefined;
}

function open(name: string) {
  const box = screen.getByRole("combobox", { name });
  fireEvent.pointerDown(box, { button: 0, ctrlKey: false, pointerType: "mouse" });
  return screen.getByRole("listbox");
}

// REQ: AUTH-2
describe("login suggestions", () => {
  it("puts requests that look like a sign-in first", () => {
    expect(byLogin(reqs).map((r) => r.name)).toEqual(["signIn", "createOrder", "listOrders"]);
  });

  it("lists JSON paths with sample values, headers and cookies", () => {
    const f = fieldsOf({
      body: '{"data":{"access_token":"eyJabc","roles":["admin"]}}',
      headers: { "X-Request-Id": ["42"], "Set-Cookie": ["sid=abc123; Path=/; HttpOnly"] },
    });
    expect(f.json).toEqual([
      { value: "data.access_token", sample: "eyJabc" },
      { value: "data.roles.0", sample: "admin" },
    ]);
    expect(f.header).toEqual([{ value: "X-Request-Id", sample: "42" }]);
    expect(f.cookie).toEqual([{ value: "sid", sample: "abc123" }]);
  });
});

// REQ: AUTH-2
describe("login capture picker", () => {
  it("picks the request and a field from its last response, and sends it as a bearer token", async () => {
    const f = mockAPI({ "/api/apis/a1": { auth: { type: "login" } } });
    renderApp(<AuthTab a={api({ type: "login" })} />);
    const list = open("Step 1 request");
    const options = within(list).getAllByRole("option");
    expect(options[0]).toHaveTextContent(/signIn/);
    expect(options[0]).toHaveTextContent("Looks like a sign-in request");
    fireEvent.click(options[0]);

    fireEvent.click(screen.getByRole("button", { name: "Add a value to keep" }));
    expect(screen.getByText('Fields from the saved response "ok"')).toBeInTheDocument();
    const fields = open("Step 1 capture 1 field");
    expect(within(fields).getByRole("option", { name: /data\.expires_in/ })).toHaveTextContent("3600");
    fireEvent.click(within(fields).getByRole("option", { name: /data\.access_token/ }));
    expect(screen.getByLabelText("Step 1 capture 1 name")).toHaveValue("access_token");

    fireEvent.click(screen.getByRole("button", { name: "Use as bearer" }));
    fireEvent.click(screen.getByRole("button", { name: "Save authentication" }));
    await waitFor(() => expect(patchBody(f)).toBeDefined());
    expect(patchBody(f).auth).toMatchObject({
      type: "login",
      steps: [{ requestId: "r3", captures: [{ name: "access_token", from: "json:data.access_token" }] }],
      apply: [{ key: "Authorization", value: "Bearer {{login.access_token}}" }],
    });
  });

  it("still takes a custom expression", async () => {
    const f = mockAPI({ "/api/apis/a1": {} });
    renderApp(
      <AuthTab a={api({ type: "login", steps: [{ requestId: "r1", captures: [{ name: "csrf", from: "json:" }] }] })} />,
    );
    fireEvent.click(within(open("Step 1 capture 1 from")).getByRole("option", { name: /Custom/ }));
    fireEvent.change(screen.getByLabelText("Step 1 capture 1 field"), { target: { value: "regex:csrf=([a-z0-9]+)" } });
    fireEvent.click(screen.getByRole("button", { name: "Save authentication" }));
    await waitFor(() => expect(patchBody(f)).toBeDefined());
    expect(patchBody(f).auth.steps[0].captures).toEqual([{ name: "csrf", from: "regex:csrf=([a-z0-9]+)" }]);
  });

  it("offers to send the request when there is no response to pick from", () => {
    mockAPI({});
    renderApp(<AuthTab a={api({ type: "login", steps: [{ requestId: "r1", captures: [] }] })} />);
    expect(screen.getByRole("button", { name: "Send listOrders" })).toBeInTheDocument();
  });

  it("does not keep a token from an earlier auth type", async () => {
    const f = mockAPI({ "/api/apis/a1": {} });
    renderApp(<AuthTab a={api({ type: "bearer", token: "{{token}}" })} />);
    fireEvent.click(within(open("Auth type")).getByRole("option", { name: /Login request/ }));
    fireEvent.click(within(open("Step 1 request")).getByRole("option", { name: /signIn/ }));
    fireEvent.click(screen.getByRole("button", { name: "Save authentication" }));
    await waitFor(() => expect(patchBody(f)).toBeDefined());
    expect(patchBody(f).auth.token).toBeUndefined();
  });
});
