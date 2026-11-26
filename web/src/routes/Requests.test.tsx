import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RequestTray, seconds } from "../components/RequestTray";
import { resetDrafts } from "../lib/drafts";
import { resetFlights } from "../lib/inflight";
import type { APIDetail, Request } from "../lib/types";
import { renderApp } from "../test/render";
import { pathNames, Requests } from "./APIWorkspace";

const req = (over: Partial<Request> = {}): Request =>
  ({
    id: "r1",
    apiId: "a1",
    folder: "",
    name: "listOrders",
    title: "",
    description: "",
    method: "GET",
    url: "{{baseUrl}}/orders",
    headers: [],
    query: [],
    bodyType: "none",
    body: "",
    access: "read",
    accessReason: "",
    accessFlag: false,
    ...over,
  }) as Request;

const api = (requests: Request[]): APIDetail =>
  ({
    id: "a1",
    name: "shop",
    title: "Shop",
    activeEnvId: "e1",
    environments: [{ id: "e1", apiId: "a1", name: "dev" }],
    variables: [{ id: "v1", envId: "", name: "baseUrl", source: "literal", value: "http://x" }],
    requests,
    auth: { type: "none" },
  }) as unknown as APIDetail;

/** A fetch whose /api/send waits until released, and honours abort. */
function slowSend() {
  let release: (body: unknown) => void = () => {};
  const signals: AbortSignal[] = [];
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      calls.push({ url, init });
      if (url.startsWith("/api/send")) {
        if (init?.signal) signals.push(init.signal);
        return new Promise<Response>((resolve, reject) => {
          init?.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
          release = (b) => resolve(new Response(JSON.stringify(b), { status: 200 }));
        });
      }
      if (url.includes("/history") || url === "/api/toolkits") return Promise.resolve(new Response("[]"));
      if (url === "/api/lint/output") return Promise.resolve(new Response(JSON.stringify({ findings: [], tokens: 3 })));
      return Promise.resolve(new Response(JSON.stringify({ id: "r1", ...{} }), { status: 200 }));
    }),
  );
  return { release: (b: unknown) => release(b), signals, calls };
}

afterEach(() => {
  resetFlights();
  resetDrafts();
});

// REQ: NET-2
describe("durations", () => {
  it("reads like Postman", () => {
    expect(seconds(850)).toBe("850 ms");
    expect(seconds(3240)).toBe("3.2 s");
    expect(seconds(65_000)).toBe("1 min 05 s");
  });
});

// REQ: NET-3
describe("sending", () => {
  it("keeps a send going when you leave the request, and shows it when you return", async () => {
    const f = slowSend();
    const a = api([req(), req({ id: "r2", name: "getOrder" })]);
    renderApp(
      <>
        <Requests a={a} onTab={() => {}} />
        <RequestTray />
      </>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Send" }));
    // Send turns into Cancel with the elapsed time.
    expect(await screen.findByRole("button", { name: /Cancel · / })).toBeInTheDocument();
    expect(screen.getByText("1 request running")).toBeInTheDocument();
    // Open another request: the first keeps running in the tray.
    fireEvent.click(screen.getByRole("button", { name: /^GET ?getOrder/ }));
    expect(screen.getByRole("button", { name: "Send" })).toBeInTheDocument();
    expect(f.signals[0].aborted).toBe(false);
    await act(async () =>
      f.release({ status: 200, statusText: "200 OK", body: '{"orders":[]}', tokens: 3, durationMs: 1200, size: 13 }),
    );
    expect(await screen.findByText("Finished while you were away")).toBeInTheDocument();
    // Back on the first request, its response is there and the tray clears.
    fireEvent.click(screen.getByRole("button", { name: /^GET ?listOrders/ }));
    expect(await screen.findByText("200 OK")).toBeInTheDocument();
    expect(screen.getByText(/1\.2 s/)).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText("Finished while you were away")).toBeNull());
  });

  it("cancels from the tray, which stops the request on the server", async () => {
    const f = slowSend();
    renderApp(
      <>
        <Requests a={api([req()])} onTab={() => {}} />
        <RequestTray />
      </>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Send" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel shop.listOrders" }));
    await waitFor(() => expect(f.signals[0].aborted).toBe(true));
    expect(await screen.findByText(/Canceled before a response came back/)).toBeInTheDocument();
  });

  // REQ: NET-2
  it("saves a request's own timeout", async () => {
    const f = slowSend();
    renderApp(<Requests a={api([req()])} onTab={() => {}} />);
    fireEvent.mouseDown(await screen.findByRole("tab", { name: "Settings" }), { button: 0 });
    fireEvent.change(screen.getByLabelText("Timeout in seconds"), { target: { value: "120" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(f.calls.some((c) => c.init?.method === "PUT")).toBe(true));
    const put = f.calls.find((c) => c.init?.method === "PUT")!;
    expect(JSON.parse(String(put.init?.body)).timeoutSeconds).toBe(120);
  });
});

// REQ: EX-3
describe("saved responses in the sidebar", () => {
  it("lists them under their request, opens one, and deletes one", async () => {
    const f = slowSend();
    const r = req({
      responses: [
        { id: "x1", name: "Two orders", status: 200, body: { orders: [1, 2] }, savedAt: "2026-11-26T05:00:00Z" },
        { id: "x2", name: "Not found", status: 404, body: { error: "nope" }, savedAt: "2026-11-26T05:00:00Z" },
      ],
    });
    renderApp(<Requests a={api([r, req({ id: "r2", name: "getOrder" })])} onTab={() => {}} />);
    // The selected request shows its saved responses as children.
    expect(await screen.findByTitle("Saved response: Two orders")).toBeInTheDocument();
    fireEvent.click(screen.getByTitle("Saved response: Not found"));
    expect(await screen.findByText(/"error": "nope"/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete saved response Two orders" }));
    await waitFor(() =>
      expect(f.calls.some((c) => c.init?.method === "DELETE" && c.url === "/api/requests/r1/responses/x1")).toBe(true),
    );
  });
});

// REQ: RNR-1
describe("path variables", () => {
  it("finds {id} and :id but not {{variables}}", () => {
    expect(pathNames("{{baseUrl}}/customers/{id}/orders/:orderId?x={y}")).toEqual(["id", "orderId", "y"]);
    expect(pathNames("{{baseUrl}}/health")).toEqual([]);
  });

  it("sends the values set under Params", async () => {
    const f = slowSend();
    renderApp(<Requests a={api([req({ url: "{{baseUrl}}/customers/{id}/summary" })])} onTab={() => {}} />);
    fireEvent.change(await screen.findByLabelText("Path variable id"), { target: { value: "C-42" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(f.calls.some((c) => c.url === "/api/send")).toBe(true));
    const body = JSON.parse(String(f.calls.find((c) => c.url === "/api/send")!.init?.body));
    expect(body.draft.pathParams).toEqual([{ key: "id", value: "C-42" }]);
  });
});

// REQ: NET-3
describe("unsaved edits", () => {
  it("stay with the request while you look at another, until reverted", async () => {
    slowSend();
    renderApp(<Requests a={api([req(), req({ id: "r2", name: "getOrder" })])} onTab={() => {}} />);
    fireEvent.change(await screen.findByLabelText("URL"), { target: { value: "{{baseUrl}}/orders?limit=5" } });
    fireEvent.click(screen.getByRole("button", { name: /^GET ?getOrder/ }));
    expect(screen.getByLabelText("URL")).toHaveValue("{{baseUrl}}/orders");
    expect(screen.getByLabelText("Unsaved changes")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^GET ?listOrders/ }));
    expect(screen.getByLabelText("URL")).toHaveValue("{{baseUrl}}/orders?limit=5");
    fireEvent.click(screen.getByRole("button", { name: "Revert" }));
    expect(screen.getByLabelText("URL")).toHaveValue("{{baseUrl}}/orders");
    await waitFor(() => expect(screen.queryByLabelText("Unsaved changes")).toBeNull());
  });
});
