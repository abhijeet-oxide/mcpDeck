<div align="center">

<img src="docs/assets/logo.svg" width="72" height="72" alt="">

# mcpDeck

**Turn any API, old ones included, into small, safe tools for your AI agents.**

One program on your laptop. Bring the APIs you already call from Postman or curl,<br>
write a tool in a few lines of TypeScript, and add it to Claude Code, VS Code or Cursor.

[![License](https://img.shields.io/badge/license-Apache_2.0-1d1d1f?style=flat-square)](LICENSE)
![Status](https://img.shields.io/badge/status-design_phase-0071e3?style=flat-square)
![Single binary](https://img.shields.io/badge/single_binary-Windows_·_macOS_·_Linux-1d1d1f?style=flat-square)
![MCP](https://img.shields.io/badge/MCP-Streamable_HTTP-1d1d1f?style=flat-square)

[The problem](#the-problem) · [What it does](#what-mcpdeck-does) · [How it fits together](#how-it-fits-together) · [Status](#status) · [Docs](docs/)

</div>

<br>

## The problem

You ask your agent a simple question about a customer. The answer is spread
across Salesforce, a billing service someone wrote in 2014, and Jira.

So the agent reads docs, guesses endpoints, and asks you for an API key, which
now sits in an env var it can print. It makes a dozen calls, scrolls through
thousands of lines of JSON to find four fields, and gets there in the end.
Tomorrow it does all of that again, a little differently. Nobody can say
exactly what it ran.

The usual fix is an MCP server. Someone writes one for Salesforce, maybe Jira.
Nobody writes one for the billing service. Each change to a description or a
field means a code change and a redeploy. And whichever agent connects gets
every tool, including the ones that delete things.

## What mcpDeck does

You do the part you already know: send requests, look at responses. mcpDeck
turns what works into a tool your agents can call in one step, gives each
agent only the tools you choose, and keeps your keys to itself.

**1. Bring your APIs.** Paste a Swagger page URL, drop in an OpenAPI file or a
Postman collection, or paste curl. It looks and works like Postman:
collections, environments, `{{variables}}`, auth, Send. Old systems are fine:
XML, form posts, and logins that hand you a cookie.

**2. Write the tool.** A tool is a short TypeScript function. Every API you
added shows up with autocomplete, down to the response fields.

```ts
export const tool = {
  name: "customer_360",
  description: "One customer: account, balance, overdue invoices and open tickets.",
  inputSchema: { type: "object", properties: { accountId: { type: "string" } }, required: ["accountId"] },
};

export default async function (input: Input) {
  const account = await crm.getAccount({ id: input.accountId });
  const [summary, tickets] = await Promise.all([
    billing.getCustomerSummary({ id: account.billingId }),
    jira.searchIssues({ body: { jql: `org = "${account.jiraOrg}" AND status != Done` } }),
  ]);
  return {
    name: account.name,
    balance: summary.balance,
    overdueInvoices: summary.invoices.filter((i) => i.overdue),
    openTickets: tickets.issues.map((t) => t.key),
  };
}
```

Three systems, one call for the agent, and only the fields it needs. The key
for each system is added on the way out; this code never sees it.

**3. Add it to your agent.** Put tools in a toolkit and click **Add to Claude
Code** (or VS Code, Cursor, Claude Desktop). Each toolkit is its own MCP
server, so a support agent sees support tools and nothing else.

Then watch it work. Every call is recorded: which agent, which tool, when, how
long, how many tokens. mcpDeck reads those calls and tells you what to fix,
with plain rules and no LLM: *these three calls always happen together, make
them one tool. This tool returns fields nobody uses. These two calls could run
in parallel.*

## Why it's built this way

|  |  |
|---|---|
| **Any system, including legacy** | If Postman or curl can call it, an agent can use it. No MCP server to write per system. |
| **Change in seconds** | Edit, test against a real or saved response, save. It's live now as a new version, and agents pinned to the old one keep working. |
| **Nothing new to learn** | Postman-style collections, TypeScript, JSON Schema, MCP. People know them, and so do agents, so an agent can build a tool here without extra instructions. |
| **Fewer calls, fewer tokens** | Chain calls, run them in parallel, return only what matters, and fill known values like tenant or region from variables instead of asking the model. |
| **Each agent gets only what it needs** | One MCP server per toolkit. Read-only agents can't send POST or DELETE: the proxy blocks it, whatever the tool says. Revoke with one click. |
| **Keys stay put** | Credentials are encrypted, with the key in your OS keychain. Agents and tool code never see them. |
| **Agents can fill the gaps** | Inside your limits an agent can try your APIs and save a tool. Read-only tools with passing tests go live; anything that writes waits for you. |

## How it fits together

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/how-it-works-dark.svg">
  <img src="docs/assets/how-it-works-light.svg" width="100%" alt="Your systems are imported into mcpDeck as APIs. Tools are TypeScript functions over those APIs. Toolkits group tools, and each toolkit is an MCP server that Claude Code, VS Code, Cursor or Claude Desktop connects to. Every call goes out through one proxy that blocks writes for read-only agents, adds keys and records the call. Recorded calls feed suggestions that improve the tools.">
</picture>

**API** is one system you call, like the Jira API. A **request** is one saved
call to it. A **tool** is what an agent calls. A **toolkit** is a set of tools
an agent adds as one MCP server. More in [docs/vision.md](docs/vision.md#words-we-use).

## Dev now, prod later

The same binary does both.

| | **Dev** (what we're building now) | **Prod** (later) |
|---|---|---|
| Runs as | One file on your laptop. Double-click and the UI opens. | A service for your team |
| Data | SQLite | Postgres |
| Secrets | Encrypted, key in your OS keychain | HashiCorp Vault, AWS, Azure, GCP, 1Password |
| People | You | SSO, roles, approvals |
| Building tools | Code editor | Code editor and a visual builder |

## What it protects, and what it doesn't

Agents connected over MCP never get your keys, and the database file is useless
without the key in your keychain. The local server only answers on
`127.0.0.1` and refuses requests from web pages.

It can't stop a program running as you on the same machine, including an agent
with a shell, from using your keychain or calling an API directly. mcpDeck
controls what agents get through MCP. It isn't a sandbox for the agent itself.

## Status

**Design phase. There's nothing to download yet.** The plan, requirements and
architecture are written down, and code lands milestone by milestone:

1. Walking skeleton: one binary, the UI, one request, one tool, one toolkit, added to Claude Code, the call recorded
2. APIs: imports, environments, variables, auth, runner, scripts, examples
3. Credentials: the local encrypted store
4. Tools: editor with autocomplete, sandbox, tests, versions, tools as files
5. Toolkits and MCP: one server per toolkit, one-click add, access, agent-built tools
6. Calls and usage
7. Suggestions

<details>
<summary>Early design mockups (some labels have changed since)</summary>
<br>

| | |
|---|---|
| <img src="docs/screenshots/home.png" alt="Home"> | <img src="docs/screenshots/application.png" alt="An API with its requests"> |
| <img src="docs/screenshots/tool-builder.png" alt="A tool with versions"> | <img src="docs/screenshots/agent-sessions.png" alt="An agent's sessions and calls"> |

</details>

Read [requirements](docs/requirements.md) for what's being built and
[design](docs/design.md) for how.

## Contributing

We work test-first. Start with [`AGENTS.md`](AGENTS.md), which Claude Code,
Copilot and Cursor also read, and [`docs/testing.md`](docs/testing.md).

```sh
make dev     # Go backend + React UI with hot reload
make test    # everything
make build   # one binary: ./bin/mcpdeck
```

## License

[Apache 2.0](LICENSE)
