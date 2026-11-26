import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { type Flow, parseFlow } from "../lib/flow";
import type { APIDetail } from "../lib/types";
import { renderApp } from "../test/render";
import { FlowBuilder, layout } from "./FlowBuilder";

// xyflow measures nodes; jsdom has no layout engine.
class RO {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal("ResizeObserver", RO);
vi.stubGlobal(
  "DOMMatrixReadOnly",
  class {
    m22 = 1;
  },
);

const src = `export const tool = {
  name: "summary",
  description: "Account summary",
  inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
};

export default async function (input: Input) {
  const account = await billing.getAccount({ id: input.id });
  const [invoices, tickets] = await Promise.all([
    billing.listInvoices({ accountId: account.id }),
    billing.listTickets({ accountId: account.id }),
  ]);
  const total = invoices.length + tickets.length;
  return { account, total };
}
`;

const apis = [
  {
    id: "a",
    name: "billing",
    title: "Billing",
    requests: ["getAccount", "listInvoices", "listTickets"].map((n) => ({
      id: n,
      name: n,
      method: "GET",
      url: `https://x/${n}`,
      query: [],
      headers: [],
      bodyType: "none",
      body: "",
      responses: [],
    })),
  },
] as unknown as APIDetail[];

function flow(): Flow {
  const r = parseFlow(src);
  if (!r.flow) throw new Error(r.reason);
  return r.flow;
}

// REQ: FLOW-6
describe("flow layout", () => {
  it("puts steps in run order and stacks parallel steps in one column", () => {
    const p = layout(flow(), () => 100);
    expect(p["s:account"].x).toBeLessThan(p["s:invoices"].x);
    expect(p["s:invoices"].x).toBe(p["s:tickets"].x);
    expect(p["s:tickets"].y).toBeGreaterThan(p["s:invoices"].y);
    expect(p.output.x).toBeGreaterThan(p["s:invoices"].x);
  });
});

// REQ: FLOW-5, FLOW-6, FLOW-7
describe("FlowBuilder", () => {
  it("lists steps in run order and refuses a move that breaks a dependency", () => {
    const onChange = vi.fn();
    renderApp(<FlowBuilder flow={flow()} onChange={onChange} apis={apis} />);
    const items = screen.getAllByRole("button", { name: /^Move / });
    expect(items.map((b) => b.getAttribute("aria-label"))).toEqual([
      "Move account",
      "Move invoices",
      "Move tickets",
      expect.stringMatching(/^Move /),
    ]);
    // Select the first step and try to move it after the steps that read it.
    fireEvent.click(screen.getByRole("button", { name: /billing\.getAccount/ }));
    fireEvent.click(screen.getByRole("button", { name: "Move later" }));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText(/has to run after it/)).toBeInTheDocument();
  });

  it("moves a step and changes the code order", () => {
    const onChange = vi.fn();
    renderApp(<FlowBuilder flow={flow()} onChange={onChange} apis={apis} />);
    fireEvent.click(screen.getByRole("button", { name: /billing\.listTickets/ }));
    fireEvent.click(screen.getByRole("button", { name: "Move earlier" }));
    const next = onChange.mock.calls[0][0] as Flow;
    expect(next.steps.map((s) => s.id).slice(0, 3)).toEqual(["account", "tickets", "invoices"]);
  });

  it("shows a legend and goes full screen", () => {
    const { container } = renderApp(<FlowBuilder flow={flow()} onChange={() => {}} apis={apis} />);
    fireEvent.click(screen.getByRole("button", { name: "Legend" }));
    expect(screen.getByText(/Dashed line: used inside text/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Full screen" }));
    expect(container.querySelector(".flow-shell.full")).not.toBeNull();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(container.querySelector(".flow-shell.full")).toBeNull();
  });

  it("lets you edit an agent input", () => {
    const onChange = vi.fn();
    renderApp(<FlowBuilder flow={flow()} onChange={onChange} apis={apis} />);
    expect(screen.getByText("Agent inputs")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Type" })).toHaveTextContent("string");
    fireEvent.change(screen.getByLabelText("Input name"), { target: { value: "account id" } });
    expect((onChange.mock.calls[0][0] as Flow).inputs[0].name).toBe("account_id");
    fireEvent.change(screen.getByLabelText("Input description"), { target: { value: "The account, like 42" } });
    expect((onChange.mock.calls[1][0] as Flow).inputs[0].description).toBe("The account, like 42");
  });
});
