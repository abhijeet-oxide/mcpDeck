export const tool = {
  name: "submit_claim",
  description: "Creates an expense report, attaches a receipt from your receipts folder and submits it, once you confirm in mcpDeck. Returns the report id and status.",
  useWhen: "The user asks to claim an expense and list_claims shows no report for it yet.",
  confirm: true,
  timeout: 300,
  inputSchema: {
    type: "object",
    properties: {
      date: { type: "string", description: "Expense date, YYYY-MM-DD" },
      amount: { type: "string", description: "Amount, e.g. 52.10" },
      category: { type: "string", enum: ["Internet", "Mobile", "Travel"] },
      purpose: { type: "string", description: "Short purpose, e.g. Home internet November" },
      receipt: { type: "string", description: "A file name from list_receipts" },
    },
    required: ["date", "amount", "category", "purpose", "receipt"],
  },
};

type Input = { date: string; amount: string; category: string; purpose: string; receipt: string };

export default async function (input: Input) {
  const receipts = await files.list("expenses");
  if (!receipts.some((f) => f.name === input.receipt)) {
    throw new Error(`no receipt named ${input.receipt}; call list_receipts`);
  }
  // Nothing is sent before the user confirms.
  await confirm({
    title: `Submit ${input.category} claim of ${input.amount}`,
    fields: { date: input.date, amount: input.amount, purpose: input.purpose, receipt: input.receipt },
  });

  // Each form carries one-time hidden state: fetch it, then post it back.
  const form = html.forms(await expenses.getExpensesClaimsNew()).find((f) => f.name === "claimForm");
  if (!form) throw new Error("the new-report page has no form; the portal may have changed");
  const created = await expenses.postExpensesClaims.raw({
    ...form.values,
    event: "save",
    date: input.date,
    amount: input.amount,
    category: input.category,
    purpose: input.purpose,
  });
  const where = created.headers.location;
  if (created.status !== 302 || !where) {
    throw new Error("the portal didn't create the report: " + html.text(String(created.body)).slice(0, 200));
  }
  const id = where.split("/").pop() as string;

  const upload = html.forms(await expenses.getExpensesClaimsById({ id })).find((f) => f.name === "receiptForm");
  await expenses.postExpensesClaimsReceiptById({ path: { id }, body: { ...upload?.values, receipt: files.attach(input.receipt) } });

  const submit = html.forms(await expenses.getExpensesClaimsById({ id })).find((f) => f.name === "submitForm");
  if (!submit) throw new Error(`report ${id} was created but can't be submitted; open it in the portal`);
  await expenses.postExpensesClaimsSubmitById({ path: { id }, body: submit.values });

  const text = html.text(await expenses.getExpensesClaimsById({ id }));
  return { id, status: /Status (\w+)/.exec(text)?.[1] ?? "unknown", receipt: input.receipt };
}
