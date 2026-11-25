<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/banner-dark.svg">
  <img src="docs/assets/banner-light.svg" width="100%" alt="mcpDeck: build MCP tools from the APIs you already have. Control what every agent can do.">
</picture>

<br>

[![License](https://img.shields.io/badge/license-Apache_2.0-1d1d1f?style=for-the-badge)](LICENSE)
![Pre-release](https://img.shields.io/badge/status-pre--release-0071e3?style=for-the-badge)
![Windows · macOS · Linux](https://img.shields.io/badge/Windows_·_macOS_·_Linux-1d1d1f?style=for-the-badge)
![MCP](https://img.shields.io/badge/MCP-HTTP_·_stdio-1d1d1f?style=for-the-badge)

**[Why](#why)** · **[Features](#features)** · **[How it works](#how-it-works)** · **[Skills](#agent-skills)** · **[Compare](#how-it-compares)** · **[Install](#install)** · **[Roadmap](#roadmap)**

</div>

<br>

> [!NOTE]
> mcpDeck is pre-release. This page describes what the first release does; follow the repo for it.

## Why

- **Building MCP servers is slow.** Every tool is code. A changed description,
  a new field or a bug fix means another build and another deploy.
- **Legacy systems don't have MCP servers.** Internal services, vendor APIs and
  SOAP endpoints have a spec at best, often only a Postman collection or a
  saved curl command.
- **Generated tools mirror the API, not the job.** Converting a spec usually
  gives one tool per endpoint and full responses, so the agent makes several
  calls and reads thousands of tokens to answer one question.
- **Agent-written glue code doesn't stick.** It works, then it's gone. Next
  session it's written again, differently, unreviewed, with the keys in its
  environment.
- **Nobody controls what agents can do.** They share the same keys and every
  tool, including the ones that delete things, and nobody can say which agent
  did what.

**mcpDeck** makes MCP tools easy to build from any HTTP API and keeps them
small and fast based on how agents actually use them. It holds the
credentials and decides, per agent, which tools it gets and whether it can
read or also write. Every call is recorded.

## Features

<table>
<tr>
<td width="50%" valign="top">
<img src="docs/assets/icons/api.svg" width="36" height="36" alt=""><br>
<b>Any HTTP API, old ones too</b><br>
Paste a Swagger page URL, import OpenAPI or a Postman collection, paste curl,
or add requests by hand. Environments, <code>{{variables}}</code>, auth and a
request runner, like Postman. XML bodies, form posts and cookie logins work.
</td>
<td width="50%" valign="top">
<img src="docs/assets/icons/code.svg" width="36" height="36" alt=""><br>
<b>Tools in a few lines of TypeScript</b><br>
Chain requests across systems, run them in parallel and return only what's
needed, with autocomplete for every API you added. Or select requests in
History and let mcpDeck write the first version.
</td>
</tr>
<tr>
<td valign="top">
<img src="docs/assets/icons/bolt.svg" width="36" height="36" alt=""><br>
<b>Change it in seconds</b><br>
Test against a real or saved response and save. It's live as a new version
straight away. Agents pinned to an older version keep working.
</td>
<td valign="top">
<img src="docs/assets/icons/layers.svg" width="36" height="36" alt=""><br>
<b>One MCP server per toolkit</b><br>
Group tools into toolkits. Each is its own MCP server you add to Claude Code,
VS Code, Cursor or Claude Desktop in a click. An agent sees only the toolkits
it's given.
</td>
</tr>
<tr>
<td valign="top">
<img src="docs/assets/icons/key.svg" width="36" height="36" alt=""><br>
<b>One set of keys, separate agents</b><br>
Credentials are stored once, encrypted, and added to requests on the way out.
Agents use them without ever seeing them, and each agent has its own access
and its own call history. Revoke one in a click.
</td>
<td valign="top">
<img src="docs/assets/icons/shield.svg" width="36" height="36" alt=""><br>
<b>Read-only means read-only</b><br>
Give an agent read access and the proxy refuses POST, PUT, PATCH and DELETE,
whatever a tool tries. Tools that write aren't even listed for it.
</td>
</tr>
<tr>
<td valign="top">
<img src="docs/assets/icons/share.svg" width="36" height="36" alt=""><br>
<b>Agents ask, build and share</b><br>
An agent missing a tool files a request, or builds it if you allow that. New
read-only tools go live when they pass your rules (tested, small output, no
duplicates) and every agent on the toolkit gets them. Tools that write wait
for you.
</td>
<td valign="top">
<img src="docs/assets/icons/activity.svg" width="36" height="36" alt=""><br>
<b>Every call recorded, then improved</b><br>
Which agent called what, when, how long it took and how many tokens came
back. mcpDeck turns that into suggestions: merge calls that always happen
together, trim large outputs, run calls in parallel. You approve each one.
</td>
</tr>
</table>

## How it works

<table>
<tr>
<td width="33%" valign="top">

**1 · Add an API**

APIs › **Import**. Paste a Swagger page URL or drop a file. Send a few
requests and save the good responses.

</td>
<td width="33%" valign="top">

**2 · Build a tool**

Select requests in History › **Create tool**, or write it in the editor.
Test, save.

</td>
<td width="33%" valign="top">

**3 · Give it to an agent**

Toolkit › **Add to** › Claude Code. Read-only by default; allow writes or
building on the Agents page.

</td>
</tr>
</table>

A tool is one file. `crm`, `billing` and `jira` are APIs you've added:

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

Three systems, one call for the agent. Keys are added by mcpDeck; this code
never sees them.

<details>
<summary><b>Prefer the terminal?</b></summary>
<br>

Everything above can also be done from the CLI, which is handy in scripts:

```sh
mcpdeck import https://petstore3.swagger.io/api/v3/openapi.json --api petstore
mcpdeck connect claude --toolkit support        # or: vscode, cursor, claude-desktop
mcpdeck agents                                  # who has access to what
```

</details>

<br>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/how-it-works-dark.svg">
  <img src="docs/assets/how-it-works-light.svg" width="100%" alt="Your systems are imported into mcpDeck as APIs. Tools are TypeScript functions over those APIs. Toolkits group tools, and each toolkit is an MCP server that Claude Code, VS Code, Cursor or Claude Desktop connects to. Every call goes out through one proxy that blocks writes for read-only agents, adds keys and records the call. Recorded calls feed suggestions that improve the tools.">
</picture>

## Agent skills

Adding a toolkit to a client can also install skills that teach the agent to
use mcpDeck well. They come with scripts where a script beats instructions.

| Skill | The agent uses it when | Includes |
|---|---|---|
| [`mcpdeck-tools`](skills/mcpdeck-tools/SKILL.md) | A task needs a company system | How to pick tools and handle errors |
| [`mcpdeck-request-tool`](skills/mcpdeck-request-tool/SKILL.md) | No tool fits, or one answer took 3+ calls | A request format people can act on |
| [`mcpdeck-trim-output`](skills/mcpdeck-trim-output/SKILL.md) | A response was much bigger than needed | `field-sizes.mjs`: bytes per field and the code for a smaller output |
| [`mcpdeck-build-tool`](skills/mcpdeck-build-tool/SKILL.md) | It may build, and a tool is missing | `check-tool.mjs`: catches what save would reject |

Agents report which fields they used; they don't change shared tools. If one
agent needs a field another doesn't, mcpDeck suggests a smaller output for
just that agent, and a person approves it.

## How it compares

Other products also turn APIs into MCP tools, mostly as cloud or gateway
services: IBM ContextForge, Speakeasy Gram, Azure API Management, Amazon
Bedrock AgentCore Gateway. mcpDeck is a single local binary focused on
building tools that combine several calls, enforcing per-agent read and write
access, and improving tools from real usage. See the
**[full comparison](docs/comparison.md)**, sourced from each product's docs.

## Install

```sh
# macOS
brew install --cask abhijeet-oxide/tap/mcpdeck

# Windows
scoop bucket add mcpdeck https://github.com/abhijeet-oxide/scoop-bucket
scoop install mcpdeck
```

Linux, or no package manager: download the archive for your platform from
[Releases](https://github.com/abhijeet-oxide/mcpDeck/releases). Then run
`mcpdeck`; the UI opens at `http://127.0.0.1:47800`. Try `mcpdeck dev` for
demo APIs to play with.

## Security

- Agents connected over MCP never receive credentials, and tool code can't
  read them.
- Secrets are encrypted with AES-256-GCM. The key lives in your OS keychain,
  not in the database.
- The server listens on `127.0.0.1` only and refuses requests from web pages.
- Every change and approval goes into an append-only audit log.

mcpDeck controls what agents can do through MCP. It doesn't sandbox the agent
itself: a program running as you on the same machine can still use your
keychain or call an API directly.

## Roadmap

**First release: on your machine**

- [ ] APIs from Swagger pages, OpenAPI, Postman and curl, with a request runner
- [ ] Encrypted credential store, including login-request auth for older systems
- [ ] TypeScript tools with autocomplete, tests and versions
- [ ] Toolkits as MCP servers, one-click setup for Claude Code, VS Code, Cursor, Claude Desktop
- [ ] Per-agent access: read or read+write, may build, revoke
- [ ] Tool requests, approval rules, publishing to every agent on a toolkit
- [ ] Call log, usage, field reports and suggestions
- [ ] Agent skills

**Then: for teams**

- [ ] Server mode with Postgres, SSO and roles
- [ ] External vaults: HashiCorp Vault, AWS, Azure, GCP, 1Password
- [ ] Visual tool builder
- [ ] Per-person credentials, so calls run as the person behind the agent
- [ ] WSDL, HAR, Bruno and Insomnia import
- [ ] Existing MCP servers behind the same access rules and call log

## Docs

[Vision](docs/vision.md) · [Requirements](docs/requirements.md) · [Design](docs/design.md) ·
[Testing](docs/testing.md) · [Comparison](docs/comparison.md) · [Releasing](docs/releasing.md)

## Contributing

We work test-first. Start with [`AGENTS.md`](AGENTS.md), which Claude Code,
Copilot and Cursor also read.

```sh
make dev     # Go backend + React UI with hot reload
make test    # all tests
make build   # one binary: ./bin/mcpdeck
```

## License

[Apache 2.0](LICENSE)
