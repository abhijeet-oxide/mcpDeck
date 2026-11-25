import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockAPI, renderApp } from "../test/render";
import { NewToolkitDialog } from "./Toolkits";

// REQ: TK-1, UI-4
describe("New toolkit", () => {
  it("shows the name the server will use and refuses a taken or too-short one", async () => {
    mockAPI({ "/api/apis": [], "/api/toolkits": [{ id: "t1", name: "support" }] });
    renderApp(<NewToolkitDialog onClose={() => {}} />);
    const input = await screen.findByPlaceholderText("Support");
    const create = screen.getByRole("button", { name: "Create toolkit" });
    expect(create).toBeDisabled();
    fireEvent.change(input, { target: { value: "Finance Ops" } });
    expect(screen.getByText("finance-ops", { selector: "code" })).toBeInTheDocument();
    expect(create).toBeEnabled();
    fireEvent.change(input, { target: { value: "x" } });
    expect(screen.getByText(/at least two/)).toBeInTheDocument();
    expect(create).toBeDisabled();
    fireEvent.change(input, { target: { value: "Support" } });
    expect(await screen.findByText(/already exists/)).toBeInTheDocument();
    expect(create).toBeDisabled();
  });
});
