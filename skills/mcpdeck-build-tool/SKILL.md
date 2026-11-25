---
name: mcpdeck-build-tool
description: Use when you can build mcpDeck tools (search_apis and save_tool are available), usually as an mcpDeck builder subagent, and a task needs a tool that doesn't exist, or a tool request is open. Covers exploring APIs, writing the TypeScript tool file, checking it, testing and saving it so it passes the toolkit's approval rules.
---

# Build an mcpDeck tool

You're usually running as the toolkit's builder subagent
(`mcpdeck-<toolkit>-builder`): the main agent sent you a short brief and
waits for a two-line answer. Explore and test here, so none of it lands in
the main agent's context. When you're done, reply with the tool's name, its
status (live, or pending and which rule held it) and what it returns.

A tool is one TypeScript file that calls one or more APIs and returns only
what's needed. Once it passes the toolkit's approval rules it's published to
every agent using the toolkit, so build it for the general case, not just
this conversation.

## 1. Check it doesn't exist

- `search_tools` for the intent. Reuse or request a change instead of making
  a near copy (mcpDeck warns about duplicates and holds them).
- `list_tool_requests`: if someone asked for this, build theirs and pass its
  id as `fulfils` when saving.

## 2. Explore

- `search_apis "<intent>"` returns one line per request, like
  `crm.getAccount({ id }) → Account · GET /accounts/{id} · read`.
- `describe_api crm.getAccount` returns the TypeScript signature, the
  response type and a masked example.
- `run_api` or `run_script` to try calls. `run_script` takes the same
  TypeScript as a tool, so you can try the whole function before saving.

## 3. Write the file

```ts
export const tool = {
  name: "overdue_invoices",                       // snake_case, 3–64 chars
  description: "Overdue invoices for one account: number, due date, amount.",
  useWhen: "A question about what an account owes or what is late",
  dontUseWhen: "Totals across many accounts",
  inputSchema: {
    type: "object",
    properties: { accountId: { type: "string", examples: ["001Ab00000XyZ"] } },
    required: ["accountId"],
  },
};

export default async function (input: Input) {
  const account = await crm.getAccount({ id: input.accountId });
  const res = await billing.listInvoices({ customerId: account.billingId, status: "overdue" });
  return {
    invoices: res.items.map((i) => ({ number: i.number, dueDate: i.dueDate, amount: i.amount })),
  };
}
```

Rules that keep tools small and fast:

- **Return only what's needed.** Build the `return` from the fields the task
  used. The **mcpdeck-trim-output** script's `--keep` writes it for you.
- **Run independent calls together** with `Promise.all`.
- **Filter at the API** (query params like `status=overdue`) rather than
  fetching everything and filtering in code.
- **Cap lists** and say so in the description ("up to 50, newest first").
- **Don't ask for known values.** If the toolkit has a variable such as
  `vars.tenantId`, use it instead of adding an input.
- **No `fetch`, imports, `process` or timers.** The API clients (`crm`,
  `billing`…), `call()` and `vars` are the only globals.
- **Descriptions describe the tool.** Don't address other agents ("always
  call this"), and no links; those are held for review.

## 4. Check, test, save

```sh
node scripts/check-tool.mjs overdue_invoices.ts
```

Fix every error; fix warnings unless you have a reason. Then:

1. `test_tool` against examples, then against the real API.
2. `save_tool` with the file, at least one test, and `fulfils` if there's a
   request. The response says whether it went live or is pending, and which
   approval rule held it.

Tools that write are always pending until a person approves them. Tell the
user what you built, its status, and what it returns.
