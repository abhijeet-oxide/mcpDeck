# Agents

mcpDeck is agent-first in two directions:

1. **Building.** An agent can do everything a person does in the UI, through
   the CLI (with `--json`) or the admin MCP server. The only exceptions are
   the decisions a person keeps.
2. **Using.** Agents that use a toolkit's tools find them cheaply, ask for
   missing ones, and get results that fit in their context.

## Surfaces

| Surface | Who uses it | How |
|---|---|---|
| UI | People | `mcpdeck` opens it at `http://127.0.0.1:47800` |
| CLI | People, scripts, coding agents with a shell | `mcpdeck <command> --json`; `mcpdeck help --json [group]` describes every command |
| Admin MCP server | Agents setting mcpDeck up | `mcpdeck mcp --agent "<name>"` on stdio; actions are audited under that name |
| Builder endpoint | Agents building tools for one toolkit | `/mcp/<toolkit>/build`, with a token that has build access |
| Use endpoint | Agents using tools | `/mcp/<toolkit>`, with a read or read-write token |

## Builder parity

Every row is covered by a scenario or a test; "person" marks decisions that
stay with a person on purpose (PAR-2).

| Action | UI | CLI | Admin MCP |
|---|---|---|---|
| Import OpenAPI, Swagger 2.0, Postman, curl | Applications › New | `import <url-or-file> --api <name>` | `import_api` |
| List and inspect APIs | Applications | `api list`, `api show <api>` | `overview`, `get_api` |
| Set a plain variable | Application › Environments | `var set` | `set_variable` |
| Store a secret | Application › Environments | `secret set` (stdin) | `secret_from_env`, `ask_for_secret` (values never pass through the chat) |
| Set auth (login chain, OAuth 2.0, HMAC, keys) | Application › Authentication | `auth set` | `set_auth` |
| TLS: company CA, client certificate, skip verification | Application › Settings | `import --ca-file`, `api tls` | `import_api ca_pem`, `set_tls` |
| Change a request's body or read/write access | Request editor | `request set-body`, `request access` | `update_request` |
| Send a request | Send | `request send` | `send_request` |
| Create and list toolkits | Toolkits | `toolkit create`, `toolkit list` | `create_toolkit`, `list_toolkits` |
| Make a starter tool from a request | Make a tool | `tool generate` | `generate_tool` |
| Run an unsaved tool | Tool editor › Test | `tool test` | `test_tool` |
| Save a tool (tests and lint run) | Tool editor › Save | `tool save`, `push` | `save_tool` |
| Edit tools as files | | `pull`, `push` | |
| Lint a toolkit or draft | Toolkit › Guidelines, editor | `lint [--file] [--json] [--strict]` | `lint` |
| List tools | Toolkit | `tool list` | `list_tools` |
| See what waits for review | Review | `tool list` (status) | `pending_reviews` |
| Approve or reject a write tool | Review | `tool approve`, `tool reject` | person |
| Connect a client | Agents › Connect | `connect <client>` | `connect_client` (read access only) |
| Revoke an agent | Agents | `agents revoke` | person |
| Change an agent's access or approval rules | Agents, Toolkit › Settings | person | person |
| Verify the audit log | Activity | `audit verify` | |
| Add the admin MCP server to a client (CON-1) | Home › Build tools with your agent | `mcp --agent <name>` (what the link runs) | person |
| Record a task in a web app (REC-1) | Application › Record a task in the browser | `record <api> <url>` | `record_workflow` |
| Browser sign-in (AUTH-5) | Application › Authentication › Browser sign-in, Sign in now | `auth set --type browser`, `auth signin` | `set_auth` {type: browser}; signing in is the person's |
| Pick the files folder tools upload from (HTTP-3) | Application › Settings › Files folder | `api files <api> --folder` | person |
| Confirm a write from a confirm-each-call tool (CONF-1) | Home, or the page mcpDeck opens | | person |
| Export a toolkit file (EXP-1) | Toolkit › Export | `toolkit export` | `export_toolkit` |
| Import a toolkit file (EXP-1) | Toolkits › Import | `toolkit import` | person |

Not on agent surfaces yet: deleting an API or toolkit, editing
environments other than the active one, and browsing the call log. These
are in the UI only.

## Web apps without an API

Some systems have no API an agent can use: server-rendered pages behind
company single sign-on, like an expense portal. The flow is the same as for
any API, with two person-only steps:

1. **Connect.** Home › *Build tools with your agent* adds the admin MCP
   server to VS Code (GitHub Copilot), Claude Code, Cursor or Claude Desktop.
2. **Describe.** Tell the agent what to automate: *"Build tools for
   https://expenses.corp.example: list my claims, list my receipts, submit a
   claim with its receipt."*
3. **Record (person).** The agent calls `record_workflow`. mcpDeck opens
   Edge or Chrome with its own profile; the person signs in (SSO, MFA) and
   does the task once, then closes the window or clicks Done. The agent gets
   each page and form post as a request, with form fields by kind
   (`(hidden: take it from the page)`, `date`, `file`) and no cookies or
   values. The API is created with browser sign-in.
4. **Build.** The agent writes tools that fetch a page, read it with
   `html.forms` / `html.tables`, and post `{ ...form.values, ...input }`.
   Uploads use `files.attach(name)`; tools that submit set `confirm: true`
   and call `confirm()` first. It tests and saves them; write tools wait.
5. **Approve (person)** in Review, pick the files folder (Application ›
   Settings) and run the tools from the editor to check them.
6. **Share.** `export_toolkit` or Toolkit › Export writes a `.mcpdeck` file.
   A colleague imports it (Toolkits › Import), approves the write tools,
   picks their own files folder and connects an agent with read+write.

When a tool runs and the session has ended, mcpDeck opens the browser for
the person to sign in again; the agent is told to call again once they
have. A write is never resent after a session ends. Each submit waits for
the person to confirm it in mcpDeck. See [the example](../examples/expenses/).

## Agents that use tools

- **Discovery.** `tools/list` returns finished tools with `readOnlyHint` and
  `destructiveHint` set from the requests each tool calls. A toolkit with
  more than 25 tools lists the 15 most used, and `search_tools` and
  `use_tool` reach the rest.
- **Requests.** `request_tool` records what the agent needed and why, for a
  person or a builder agent to act on. It grants nothing; a read agent can't
  get a write tool this way.
- **Feedback.** `report_field_usage` tells the builder which output fields
  the agent used, so tools can be trimmed.
- **Output.** A result over 20,000 tokens is cut to valid JSON: the longest
  list keeps what fits, and `_mcpdeck` gives the shown and total counts with
  a hint to narrow the input.
- **Errors** say what to do next (MCP-4), for example which input was wrong.

## Context sizes

Measured with `cl100k_base`; the scenarios fail above the bounds.

| What an agent loads | Tokens | Bound | Source |
|---|---:|---:|---|
| Use endpoint, one-tool toolkit: instructions + `tools/list` | about 470 | 1,000 | scenario `consumer/server instructions and tools/list stay small` |
| Builder endpoint: instructions + `tools/list` | 910 | | `bench/results.json` |
| Admin MCP server: instructions + `tools/list` (21 tools) | about 2,300 | 2,500 | scenario `parity/admin MCP context an agent reads stays small` |
| `mcpdeck help --json` (every command) | about 3,500 | | `go run ./tools/tokcount` |
| `mcpdeck help --json auth` (one group) | about 600 | | `go run ./tools/tokcount` |
| [`llms.txt`](../llms.txt) | about 750 | | `go run ./tools/tokcount llms.txt` |

## Lint in CI

`mcpdeck lint <toolkit> --strict` exits 1 on any block or warning, and
`--json` gives machine-readable findings. A pipeline that keeps tools as
files can run `mcpdeck serve --no-open --data ./ci-data &`, recreate the
APIs and toolkit, `push` the files and lint them. The rules are in
[lint-rules.md](lint-rules.md).
