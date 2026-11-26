import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { mockAPI, renderApp } from "../test/render";

// Monaco needs a real browser; a small stand-in keeps the model text.
vi.mock("../lib/monaco", () => {
  const model = (text: string) => {
    let v = text;
    return {
      uri: "file:///t.ts",
      getValue: () => v,
      setValue: (x: string) => {
        v = x;
      },
      onDidChangeContent: () => ({ dispose() {} }),
      pushEditOperations: (_: unknown, ops: { text: string }[]) => {
        v = ops[0].text;
      },
      getFullModelRange: () => ({}),
      dispose() {},
    };
  };
  const monaco = {
    editor: {
      createModel: model,
      create: (_: unknown, o: { model: unknown }) => ({ getModel: () => o.model, dispose() {}, executeEdits() {} }),
      onDidChangeMarkers: () => ({ dispose() {} }),
      getModelMarkers: () => [],
    },
    MarkerSeverity: { Error: 8 },
    Uri: { parse: (s: string) => s },
    Range: class {},
  };
  return { monaco, applyTheme() {}, setTypes() {} };
});
vi.stubGlobal(
  "ResizeObserver",
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
);

const { default: ToolEditor } = await import("./ToolEditor");

const source = `export const tool = {
  name: "ping",
  description: "Checks the service is up.",
  inputSchema: { type: "object", properties: {} },
};

export default async function (input: Input) {
  const status = await svc.getStatus({});
  return status;
}
`;

// REQ: VER-4
describe("ToolEditor", () => {
  it("keeps unsaved edits when you look at Versions and come back", async () => {
    mockAPI({
      "/api/tools/ping": {
        tool: { id: "t1", name: "ping" },
        versions: [
          {
            id: "v1",
            n: 1,
            source,
            status: "live",
            tests: [],
            createdByKind: "you",
            createdAt: "2026-11-26T05:00:00Z",
          },
        ],
        fields: [],
      },
      "/api/apis": [],
      "/api/toolkits/dev/types.d.ts": "",
    });
    renderApp(<ToolEditor toolkit="dev" tool="ping" />);
    const name = await screen.findByDisplayValue("ping");
    fireEvent.change(name, { target: { value: "ping_service" } });
    fireEvent.mouseDown(screen.getByRole("tab", { name: /Versions/ }), { button: 0 });
    expect(await screen.findByText("History")).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole("tab", { name: /Build/ }), { button: 0 });
    expect(screen.getByDisplayValue("ping_service")).toBeInTheDocument();
  });
});
