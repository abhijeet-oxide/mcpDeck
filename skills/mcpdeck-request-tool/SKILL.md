---
name: mcpdeck-request-tool
description: Use when an mcpDeck toolkit has no tool for what the task needs, a tool keeps failing for a reason you can't fix, you needed three or more calls to answer one question, or you need write access you don't have. Files a precise tool request that a person or a building agent can fulfil.
---

# Request a tool from mcpDeck

A good request gets built once and then every agent using the toolkit gets
the tool. A vague one gets ignored. Spend a minute on it.

## When to request

Request a tool when any of these is true:

- `search_tools` found nothing that does the job.
- You answered one question with three or more calls (tool calls or
  `run_api`). That pattern should be one tool.
- A tool's output is missing a field you needed, so you called something
  else to fill it in.
- The task needs a change (create, update, delete) and you only have read
  access.

Don't request a tool for a one-off question you've already answered with one
call.

## Write the request

Call `request_tool` with:

| Field | What to put |
|---|---|
| `title` | Verb and object, like a tool name: "List overdue invoices for an account" |
| `why` | The user's question in one sentence, and what you had to do instead |
| `input` | The inputs you'd pass, with one real example value each |
| `output` | Only the fields you needed, with one example value each. Leave out fields you didn't use |
| `tried` | The calls you made, in order (mcpDeck attaches the recorded calls from this session; name them so a person can find them) |
| `access` | `read` or `write`, and for write, exactly what changes |

Example:

```json
{
  "title": "List overdue invoices for an account",
  "why": "User asked which invoices are overdue for Acme. I called billing.listInvoices for all invoices (412 rows) and filtered them myself.",
  "input": { "accountId": "001Ab00000XyZ" },
  "output": { "invoices": [{ "number": "INV-1042", "dueDate": "2026-10-01", "amount": 1200.5 }] },
  "tried": ["crm.getAccount", "billing.listInvoices"],
  "access": "read"
}
```

## After requesting

- Tell the user you've requested the tool and what it will do. Finish the
  current task with what you have if you can.
- If you can build tools here, call `list_tool_requests` first. Someone may
  already have asked; build that one instead of filing a duplicate (see
  **mcpdeck-build-tool**, and pass `fulfils` when you save).
