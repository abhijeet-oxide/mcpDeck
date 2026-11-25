<div align="center">

<img src="docs/assets/logo.svg" width="72" height="72" alt="">

# mcpDeck

**Turn any API, including the legacy ones, into fast, controlled tools for AI agents.**

One binary. Import what you already have, write a tool in a few lines of
TypeScript, and add it to Claude Code, VS Code or Cursor in one click.<br>
Change it in seconds. Agents get only what they need and never see a key.

[![License](https://img.shields.io/badge/license-Apache_2.0-1d1d1f?style=flat-square)](LICENSE)
![Status](https://img.shields.io/badge/status-design_phase-0071e3?style=flat-square)
![Go](https://img.shields.io/badge/Go-single_binary-1d1d1f?style=flat-square&logo=go&logoColor=white)
![Windows](https://img.shields.io/badge/Windows_·_macOS_·_Linux-1d1d1f?style=flat-square)
![MCP](https://img.shields.io/badge/MCP-Streamable_HTTP-1d1d1f?style=flat-square)

[Why](#why-this-exists) · [Why mcpDeck](#why-mcpdeck) · [How it works](#how-it-works) · [Quickstart](#quickstart) · [Docs](docs/) · [Status](#status)

<br>

<img src="docs/screenshots/tool-builder.png" width="920" alt="A customer_360 tool in mcpDeck that reads Salesforce, Billing and Jira, with its versions">

</div>

<br>

## Why this exists

**Most systems will never get an MCP server.** The internal services, vendor
APIs and SOAP endpoints where real work happens have a Swagger page, a Postman
collection or a curl command at best. Nobody will write and maintain a server
for each.

**MCP servers are slow to change.** Every tweak is code, a build and a
redeploy, so tools fall behind the APIs they wrap.

**Agents see too much and waste too much.** One server exposes every endpoint
to every agent. Responses are huge, one job takes five calls, and the model is
asked for values that are already known.

**Code execution works, then disappears.** The glue an agent writes is gone
when the session ends, different next time, and nobody reviewed it.

## Why mcpDeck

<table>
<tr>
<td width="50%" valign="top">

**1. Any system, including legacy**<br>
If you can call it from Postman, Bruno or curl, agents can use it. Point at a
Swagger page, import OpenAPI or Postman, or paste curl. XML, form posts,
cookies and login-request auth work.

</td>
<td width="50%" valign="top">

**2. Change in seconds**<br>
Edit, test against a real or saved response, save. It's live straight away as
a new version, and agents pinned to an older one keep working.

</td>
</tr>
<tr>
<td valign="top">

**3. Nothing new to learn**<br>
Applications work like Postman collections. Tools are TypeScript with
autocomplete, like Postman scripts. Inputs are JSON Schema; tools are MCP
tools. Agents already know all of it.

</td>
<td valign="top">

**4. Small tools, fewer tokens**<br>
Chain calls into one tool, run them in parallel, return only what matters, and
fill known values (tenant, region, user) from variables instead of asking the
model.

</td>
</tr>
<tr>
<td valign="top">

**5. Each agent sees only what it needs**<br>
Every toolkit is its own MCP endpoint. Read-only connections can't send POST or
DELETE: the proxy blocks it. Keys stay encrypted in mcpDeck. Revoke in one
click.

</td>
<td valign="top">

**6. Every call recorded, and it gets better**<br>
Who called what, when, how long, how many tokens. mcpDeck turns that into
suggestions (merge these calls, drop these fields, run these in parallel) with
plain rules, no LLM.

</td>
</tr>
<tr>
<td colspan="2" valign="top">

**7. Agents can build what's missing.** Inside your limits an agent can search
your APIs, try them and save a tool. Read-only tools with passing tests go
live; tools that write wait for you.

</td>
</tr>
</table>

## How it works

```
Application ─▶ Requests ─▶ Environments · Variables · Credentials ─▶ Examples
 (like a        (Swagger,     ({{baseUrl}}, {{tenantId}},            (saved
  Postman        OpenAPI,      encrypted keys)                        responses)
  collection)    Postman, curl)
                     │
                     ▼
Toolkit ─▶ Tools                 TypeScript, one or more requests, versioned
   │
   ▼
MCP endpoint /mcp/<toolkit> ─▶ Connections   Claude Code · VS Code · Cursor · Claude Desktop
                                   │          read or read+write
                                   ▼
                           Calls · time · tokens ─▶ suggestions
```

A tool is a short TypeScript file. If you've written a Postman script or an
async function, you can write one, and so can your agent:

```ts
export const tool = {
  name: "customer_360",
  description: "One view of a customer: account, balance, overdue invoices and open tickets.",
  inputSchema: { type: "object", properties: { accountId: { type: "string" } }, required: ["accountId"] },
};

export default async function (input: Input) {
  const account = await crm.getAccount({ id: input.accountId });
  const [summary, tickets] = await Promise.all([
    billing.getCustomerSummary({ id: account.billingId }),
    support.searchIssues({ body: { jql: `org = "${account.jiraOrg}" AND status != Done` } }),
  ]);
  return {
    name: account.name,
    balance: summary.balance,
    overdueInvoices: summary.invoices.filter((i) => i.overdue),
    openTickets: tickets.issues.map((t) => t.key),
  };
}
```

`crm`, `billing` and `support` are your applications, with autocomplete for
every request and response field. Credentials are added on the way out; the
code never sees them.

## Two ways to run it

| | **Dev**: the focus now | **Prod**: later |
|---|---|---|
| Runs as | One binary on your laptop. Double-click, the UI opens | The same binary as a service |
| Data | SQLite | Postgres |
| Secrets | Encrypted, key in your OS keychain | HashiCorp, AWS, Azure, GCP, 1Password |
| People | You | SSO, roles, approvals |

## Quickstart

> mcpDeck is in the design phase. This is how it will work.

Download `mcpdeck.exe` (or the macOS / Linux binary) and run it. The UI opens
at `http://127.0.0.1:47800`.

1. **Applications › Import**: paste a Swagger page URL, or drop an OpenAPI
   file or Postman collection.
2. Send a few requests and save good responses.
3. **Toolkits › New toolkit**, then **New tool**, or select requests in
   History and choose **Create tool**.
4. **Add to… › Claude Code**. Then ask your agent:

> what's the balance and open tickets for account 001Ab00000XyZ?

Want to try it without real APIs? `mcpdeck dev` starts with mock CRM, billing,
ticketing and a legacy cookie-login service already set up.

## What mcpDeck protects, and what it doesn't

Agents connected over MCP never receive your keys, and the database file alone
decrypts nothing because the key is in your OS keychain. mcpDeck can't stop a
program running as you on the same machine, including an agent with a shell,
from using your keychain or calling an API directly. Use it to control what
agents get through MCP, not as a sandbox for the agent itself.

## Status

Design phase. Phase 1 (dev) is built in this order:

1. Walking skeleton: binary, UI, SQLite, one request, one tool, one toolkit, add to Claude Code, see the call
2. Applications: imports, environments, variables, auth, runner, scripts, examples
3. Credentials: local encrypted store
4. Code tools: editor with autocomplete, sandbox, tests, versions, tools as files
5. Toolkits and MCP: endpoints, connections, one-click add, access, agent-built tools
6. Calls and usage
7. Suggestions

Phase 2 adds the visual tool builder, external vaults and team server mode.
Read [`docs/requirements.md`](docs/requirements.md) for the full list and
[`docs/design.md`](docs/design.md) for the architecture.

## Contributing

We work test-first. Start with [`AGENTS.md`](AGENTS.md) (also what Claude Code,
Copilot and Cursor read) and [`docs/testing.md`](docs/testing.md).

```sh
make dev     # Go backend + React UI with hot reload
make test    # everything
make build   # one binary: ./bin/mcpdeck
```

## License

[Apache 2.0](LICENSE)
