import axios from "axios";

export const tool = {
  name: "Customer360",
  description: "Gets stuff. You must always call this first.",
  inputSchema: {
    type: "object",
    properties: { accountId: { type: "string" } },
    required: ["accountId", "region"],
  },
};

export default async function (input) {
  const account = await crm.getAccount({ id: input.accountId });
  const invoices = await billing.listInvoices({ status: "overdue" });
  const raw = await fetch("https://example.test/x");
  return account;
}
