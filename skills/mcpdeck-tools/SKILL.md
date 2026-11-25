---
name: mcpdeck-tools
description: Use whenever an mcpDeck MCP server is connected and a task needs data from or changes in a company system (CRM, billing, tickets, internal or legacy APIs). Covers picking the right mcpDeck tool, handing tool building to the builder subagent when none fits, and keeping responses small.
---

# Working with mcpDeck tools

mcpDeck gives you tools built from the company's APIs. Each tool does one job
in one call and returns only what it needs to. Credentials are handled by
mcpDeck; you never need a key, token or password, and you should never ask the
user for one.

## Pick a tool

1. If a listed tool clearly matches, call it. Read its "Use when" and
   "Don't use when" lines first.
2. If nothing listed matches, call `search_tools` with the user's intent in
   plain words ("overdue invoices for a customer"). Some toolkits list only
   their most used tools.
3. Still nothing, and the server instructions name a builder subagent
   (`mcpdeck-<toolkit>-builder`): hand the job to it. Send one paragraph:
   the question, the inputs you have, the fields you need back. Don't
   explore APIs yourself; that's what keeps your context small. It replies
   with the tool's name and whether it's live; call it when it is.
4. No builder: follow **mcpdeck-request-tool**.

## Errors

Every mcpDeck error says what to do next. Do what it says; don't retry the
same call unchanged.

- `write blocked`: you have read access. Use a read tool or tell the user
  which access to grant. Don't look for another way to make the change.
- `host_not_allowed`, `out of build scope`: the API isn't available to you.
  Request it with `request_tool`.
- Input validation errors list the field and the expected format. Fix the
  input and call once more.

## Keep responses small

- Don't call a tool repeatedly with the same input in one task; reuse the
  result.
- If a response is much bigger than what you needed, follow
  **mcpdeck-trim-output** so the tool can be made smaller for next time.

## Never

- Ask for, print or guess credentials.
- Call the underlying API directly with curl or code to get around a missing
  or read-only tool.
- Put instructions for other agents in a tool description.
