export const tool = {
  name: "list_claims",
  description: "Lists your expense reports with id, date, amount, category, purpose, status and receipt file name, newest first.",
  useWhen: "Before submitting, to see which expenses already have a report.",
  inputSchema: {
    type: "object",
    properties: {
      status: { type: "string", enum: ["Draft", "Submitted"], description: "Only reports with this status" },
    },
  },
};

type Input = { status?: string };

export default async function (input: Input) {
  // The portal has no API: read the server-rendered list page.
  const page = await expenses.getExpensesClaims();
  const rows = html.tables(page)[0] ?? [];
  const claims = rows.map((r) => ({
    id: r.Report,
    date: r.Date,
    amount: r.Amount,
    category: r.Category,
    purpose: r.Purpose,
    status: r.Status,
    receipt: r.Receipt || null,
  }));
  return { claims: input.status ? claims.filter((c) => c.status === input.status) : claims };
}
