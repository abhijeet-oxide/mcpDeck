export const tool = {
  name: "customer_360",
  description: "One customer: account, balance, overdue invoices and open tickets.",
  useWhen: "A question is about one customer or account",
  inputSchema: {
    type: "object",
    properties: { accountId: { type: "string", examples: ["001Ab00000XyZ"] } },
    required: ["accountId"],
  },
};

// Comments mentioning fetch( are fine.
export default async function (input: Input) {
  const account = await crm.getAccount({ id: input.accountId });
  const [summary, tickets] = await Promise.all([
    billing.getCustomerSummary({ id: account.billingId }),
    jira.searchIssues({ body: { jql: `org = "${account.jiraOrg}"` } }),
  ]);
  return { name: account.name, balance: summary.balance, openTickets: tickets.issues.map((t) => t.key) };
}
