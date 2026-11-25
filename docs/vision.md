# Vision

## One line

mcpDeck turns any API, including the legacy ones nobody will write an MCP
server for, into small, fast, controlled tools for AI agents. You change a tool
in seconds, agents get only what they need, and every call is measured.

## The problem

1. **Most systems will never get an MCP server.** Legacy and internal systems
   have REST, SOAP, a Swagger page, a Postman collection or just a curl command
   someone saved. Nobody is going to write and maintain a server for each one.
2. **MCP servers are slow to change.** Every tweak to a description, a field or
   a step is code, a build and a redeploy. So tools fall behind the APIs they
   wrap.
3. **Most MCP servers waste the agent's time.** One tool per endpoint, huge
   responses, five calls to do one job, and the model asked for values that
   are already known (tenant, region, "me").
4. **Agents see too much.** One server exposes everything to every agent. A
   read-only task still gets delete tools, and keys leak through env vars,
   prompts and generated code.
5. **Code execution throws work away.** Agents write glue code that works, then
   it disappears. The next session pays again in tokens, time and risk, and
   the code is different every run, so nobody can review or audit it.
6. **Nothing gets better.** Nobody measures which tools are slow, expensive or
   misused, so nothing improves.

## The core idea

> Agents explore once. mcpDeck makes it repeatable.

An agent (or a person) explores an API the way they already do: send requests,
look at responses, write a bit of glue code. mcpDeck captures what worked as a
tool: a short TypeScript function that calls one or more APIs and returns only
what is needed. It's versioned, put behind access rules, and measured on every
call. mcpDeck then reads those measurements and suggests how to make each tool
smaller and faster, with plain rules and no LLM.

## Why mcpDeck (in order)

1. **Any system, including legacy.** If you can call it from Postman, Bruno or
   curl, agents can use it. Point at a Swagger page, drop in an OpenAPI file or
   Postman collection, or paste curl. XML bodies, form posts, cookies and odd
   auth all work, because underneath it's a plain HTTP client.
2. **Change in seconds.** Edit, test against a real or saved response, save.
   The new version is live immediately. Nothing to build or redeploy, and
   agents pinned to an older version keep working.
3. **Nothing new to learn, for people or agents.** APIs work like
   Postman collections. Tools are TypeScript with autocomplete, like Postman
   scripts. Inputs are JSON Schema, tools are MCP tools, config is the JSON
   your editor already reads. An agent can build a tool with no extra
   instructions because it has seen all of this before.
4. **Small tools, fewer tokens.** Chain several calls into one tool, run them in
   parallel, return only the fields that matter, and fill known values (tenant,
   region, user) from variables instead of asking the model.
5. **Each agent sees only what it needs.** Every toolkit is its own MCP
   endpoint. A read-only agent can't send POST or DELETE: the proxy blocks
   it whatever the tool says. Keys never leave mcpDeck. Revoking takes one
   click.
6. **Every call is recorded, and it gets better.** Who called what, when, for
   how long, how many bytes and tokens. mcpDeck turns that into concrete
   suggestions: merge these three calls, drop these fields, run these steps in
   parallel, remove this unused tool.
7. **Agents can build what's missing.** Inside the limits you set, an agent can
   search APIs, try them and save a tool. Tools that write wait for a person.

## Two ways to run it

| | **Dev (Phase 1, the focus now)** | **Prod (Phase 2)** |
|---|---|---|
| Who | One developer and their coding agents | A team, many agents, a security owner |
| Runs as | One binary on a laptop (Windows, macOS, Linux). Double-click, UI opens | The same binary, deployed as a service |
| Data | SQLite in the user's data folder | Postgres |
| Secrets | Local encrypted store, key in the OS keychain | External vaults (HashiCorp, AWS, Azure, GCP, 1Password) |
| People | Just you | SSO, roles, approvals |
| Moving tools | Tools as files you can commit | `mcpdeck apply` from git, like `kubectl apply` |

Dev mode has to work completely on its own. Nothing in Phase 1 depends on a
server, an account or the network beyond the APIs you call.

## Words we use

Each word is the one people already use for that thing elsewhere. Use these
words in the UI, docs, CLI, errors and code.

| Word | Means | Why this word |
|---|---|---|
| **API** | One system you call, like "the Jira API": base URLs per environment, auth, credentials, and its requests. | It's what people call it. Postman says "collection" and Zapier says "app", but "Application" clashes with OAuth apps in Okta and Azure and with the user's own app. |
| **Request** | One saved call inside an API: method, URL, params, body. | Postman, Bruno and Insomnia all say request. |
| **Environment**, **variable** | Named sets of `{{values}}` (Local, Staging) and the values themselves. | Same as Postman. |
| **Credential** | A secret mcpDeck adds to requests. Never shown to agents. | Plain word; "secret" is also fine in the UI. |
| **Tool** | One thing an agent can call: a TypeScript function over one or more requests, with a name, description and input schema. Tools live in one library and can be in several toolkits. | It's an MCP tool, exactly. |
| **Toolkit** | A named set of tools that an agent adds as one MCP server. | LangChain, the Stripe agent toolkit, Composio and Arcade all use "toolkit" for a set of tools given to an agent. "Project" says nothing about what it does. "Server" would clash with the mcpDeck server itself. |
| **Agent** | A client you added a toolkit to, like "Claude Code on this laptop", with its own token and access. Its access to each toolkit is a *grant*. | It's what people see and say. |
| **Call** | One recorded use of a tool or request: who, when, how long, tokens. | Plain word, matches "API call". |
| **Suggestion** | A rule-based change mcpDeck proposes from recorded calls. | Says what it is; "insight" sounds like a dashboard. |

## Mental model

```
API → Requests → Environments + Variables → Credentials → Examples
Tool library → Tools (TypeScript, calling one or more APIs)
Toolkit → some of those tools → one MCP endpoint
Agent (Claude Code, VS Code, Cursor…) → grants to toolkits (read or read+write, may build or not)
Agent → every call, with time, bytes and tokens → suggestions
```

1. Create an **API**, like a Postman collection: one per system.
2. Add **requests**: import a Swagger page, OpenAPI, Postman or curl, or add one
   by hand.
3. Set **environments** and **variables** (`{{baseUrl}}`, `{{tenantId}}`) and
   **credentials**, inherited by every request like collection auth in
   Postman.
4. **Send** requests and save good responses as **examples**.
5. Create a **toolkit** and write **tools** in TypeScript with autocomplete for
   every request you've added. Test, save; each save is a new version.
6. Click **Add to Claude Code** (or VS Code, Cursor, Claude Desktop). The
   toolkit becomes an MCP server in that client, with read access unless you
   choose otherwise.
7. Open **Agents** to see every call: who, when, how long, what it returned.
   Follow the suggestions to make tools smaller and faster.

## Principles

1. **Reuse, don't invent.** Every screen and format copies a pattern people
   (and models) already know: Postman, Bruno, VS Code, Chrome DevTools, JSON
   Schema, MCP. If we're designing a new interaction, we've probably gone
   wrong.
2. **Built for agents as much as people.** Everything in the UI is also an
   HTTP API, a CLI command and an MCP tool. Errors say what to do next.
3. **Lightweight.** One binary, SQLite, running in under a minute.
4. **People set the limits, agents work inside them.** An agent can't widen its
   own access, approve a write tool or read a secret.
5. **Agents never see credentials.** Secrets are added to requests at the proxy
   and are never readable from tool code.
6. **Least exposure.** Each toolkit endpoint lists only its tools; each
   agent gets only the access its grant allows.
7. **Never break a caller.** Saving a tool creates a new version.
8. **Measured, not claimed.** We show what we counted and say what we didn't.
9. **Nothing fake in the UI.** Every button works. If it isn't built, it isn't
   shown.

## Users

| User | Wants |
|---|---|
| Developer using an agent | An agent that can do real work across internal and legacy systems, today, on their laptop |
| Platform / AI engineer | Expose company systems to agents safely without writing an MCP server for each |
| The agent itself | Small tools with clear descriptions, small responses, and a way to build what's missing |
| Security / platform owner (Phase 2) | Central control of credentials, access and audit |
| Team lead (Phase 2) | Tools that are shared, reviewed and improve over time |

## Non-goals

- Not a hosted SaaS. Self-hosted, local first.
- Not a workflow automation product (no cron, no triggers).
- Not an agent framework or chat UI.
- Not an API gateway for human traffic.
- Not a model host. Running tools, and making suggestions, never needs an LLM.
- Not a replacement for Postman. We copy its patterns so people feel at home,
  and stop where agent tooling starts.

## Success measures

- Download to first agent tool call on Windows: under 5 minutes.
- Demo task: calls, tokens and time cut by at least 70% once a tool replaces
  raw calls, shown on the before/after view.
- A coding agent with only the mcpDeck MCP server and no skill or extra prompt
  can build and save a working two-API tool.
- Zero credentials ever returned to an agent (enforced by a CI test).

## Name

**mcpDeck.** A deck is a set of cards you deal from. Here the cards are tools:
each agent is dealt only the tools it needs, and a deck is also the control
surface where people manage agents, access and calls. Keep the name in one
place (`internal/version`, the CLI root command, the UI title) so changing it
later stays cheap.
