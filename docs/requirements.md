# Requirements

Each requirement has an ID, a milestone and acceptance criteria. Tests reference
IDs in their names or comments (`// REQ: TOOL-4`) so coverage of a requirement
can be found with `grep`.

MUST / SHOULD / MAY follow RFC 2119.

## Phases

**Phase 1: Dev.** One binary on a developer's laptop, Windows first-class. It
does everything on its own: UI, SQLite, credential store, tool runtime, MCP
endpoints, call log and suggestions. This is the current focus.

**Phase 2: Prod.** The same binary deployed for a team: Postgres, SSO, external
vaults, approvals, the visual tool builder and promotion from git.

Don't build Phase 2 items during Phase 1 unless asked. Design Phase 1 so they
fit later (interfaces for the store and vault; tools stored as plain files).

## Phase 1 milestones

M1 is a thin slice through everything, so the whole loop works end to end
early. Later milestones make each part complete.

| | Milestone | Outcome |
|---|---|---|
| **M1** | Walking skeleton | `mcpdeck.exe` starts, opens the UI, stores in SQLite; add one request by hand, send it through the proxy, write a one-step tool, put it in a toolkit, add it to Claude Code, call it, see the call |
| **M2** | Applications | Postman-style collections: imports, environments, variables, auth, runner, history, examples, scripts |
| **M3** | Credentials | Local encrypted store with the key in the OS keychain |
| **M4** | Code tools | TypeScript editor with autocomplete, sandbox, tests, versions, tools as files |
| **M5** | Toolkits and MCP | One endpoint per toolkit, connections, one-click add to clients, access rules, agent-built tools |
| **M6** | Calls and usage | Who called what and when, time, bytes, tokens; usage views; audit log |
| **M7** | Suggestions | Rule-based suggestions from usage, no LLM, with before/after |

---

## Foundation (M1)

**RUN-1 One binary.** A single static binary per platform: Windows (amd64,
arm64), macOS (arm64, amd64), Linux (amd64, arm64). No CGO, no installer
required, no runtime to install.
*Accept:* CI builds all targets; the Windows binary passes the e2e suite on a
Windows runner.

**RUN-2 Start and open.** Running `mcpdeck` with no arguments (or double-
clicking `mcpdeck.exe`) starts the server and opens the UI in the default
browser. A second launch opens the browser on the running instance instead of
starting another. `mcpdeck serve --no-open` starts without a browser.
*Accept:* second launch exits 0 within 1 s and the first instance keeps running.

**RUN-3 Local data.** All settings, applications, tools, connections and calls
live in one SQLite database in the user data folder: `%LOCALAPPDATA%\mcpDeck`
on Windows, `~/Library/Application Support/mcpDeck` on macOS,
`$XDG_DATA_HOME/mcpdeck` on Linux. `--data <dir>` overrides it. No other state
files except the WAL and, where no OS keychain exists, the key file (CRED-2).
*Accept:* deleting the data folder resets mcpDeck completely.

**RUN-4 Port.** Default `127.0.0.1:47800`, stored in settings on first run so
client configs stay valid. If the port is taken by something else, mcpDeck
refuses to start and says which port and how to change it. It never silently
picks another port.

**RUN-5 Local security.** The local server MUST NOT be usable by web pages or
other machines:
- binds to `127.0.0.1` only (`--listen` elsewhere is Phase 2 and needs auth);
- rejects requests whose `Host` is not `127.0.0.1:<port>` or `localhost:<port>`
  (DNS rebinding);
- the UI API requires a session cookie (`HttpOnly`, `SameSite=Strict`) issued
  through a one-time launch URL opened by RUN-2, and rejects cross-origin
  `Origin` headers; no CORS headers are ever sent;
- MCP endpoints require a connection token and reject browser `Origin` headers
  that aren't local.
*Accept:* table test of Host × Origin × cookie × token for every route group;
a page on another origin can't read or change anything.

**RUN-6 Everything has an API.** Every UI action is an HTTP API call described
in the server's OpenAPI document. The CLI uses the same API. Agents can do
anything a person can, except what ACC-6 forbids.

**RUN-7 Settings.** Settings has: port, data folder (read-only), theme,
default timeouts, retention, keychain status, and a "Reveal data folder"
button. Every setting is stored in SQLite and takes effect without a restart,
except the port.

## Applications (M2)

Applications behave like Postman or Bruno collections. Someone who has used
either should not need to read anything.

**APP-1 Application.** Create, rename, duplicate and delete an application,
with folders inside it. Deleting one that tools use is refused with a list of
those tools.

**APP-2 Allowed hosts.** Each application has an allow-list of outbound hosts,
defaulting to the hosts of its environments' base URLs. Calls to any other host
are refused with `host_not_allowed` and recorded.

**ENV-1 Environments.** An application has environments (e.g. Local, Dev,
Staging), each a set of variables. One is active in the UI; each toolkit
connection picks one (CON-1).

**VAR-1 Variables.** `{{name}}` works in URL, params, headers, body and auth,
with Postman's scopes and order: request-local → environment → application →
global. Unresolved variables are highlighted in red in the editor and listed
before sending.

**VAR-2 Variable sources.** A variable's value can be: a literal; an OS
environment variable of the mcpDeck process; a key from a local `.env` file
the user picks; or a secret (stored per CRED-2). Secret variables resolve only
inside the proxy (CRED-7).
*Accept:* changing the `.env` file is picked up on the next call.

**AUTH-1 Auth inheritance.** Auth is set on the application, a folder or a
request, with "Inherit from parent" as the default, like Postman. Types:
none, API key (header or query), bearer, basic, OAuth 2.0 client credentials,
digest, and **login request** (CRED-1).

**HTTP-1 Requests.** A request has method (any verb, including custom),
URL, params, headers, body and auth. Bodies: none, JSON, XML, text,
`x-www-form-urlencoded`, multipart, GraphQL, binary file. Cookies are kept in
a per-application cookie jar that can be viewed and cleared.

**IMP-1 OpenAPI / Swagger.** Import OpenAPI 3.x and Swagger 2.0 from a file or
URL. Each operation becomes a request named from its `operationId` (or method
and path), with params, body example, and request and response schemas.
*Accept:* golden-file tests over at least five public specs, including one
Swagger 2.0.

**IMP-2 Swagger page.** Given the URL of a Swagger UI or Redoc page, find and
import the spec it loads.
*Accept:* works for the default Swagger UI and Redoc setups; otherwise says
"no spec found on that page; paste the spec URL instead".

**IMP-3 Postman.** Import Postman collection v2.1 and Postman environments:
folders, variables, auth, example responses (become examples) and scripts
(imported untrusted, RNR-6).

**IMP-4 curl.** Paste one or more curl commands. Headers that look like
credentials (`Authorization`, `Cookie`, `X-Api-Key`, query `api_key` /
`token` / `key`) are moved into a credential, never stored on the request.

**RNR-1 Runner.** A Postman-style request editor: tabs for open requests,
method and URL bar, Params / Auth / Headers / Body / Scripts tabs, Send
(`Ctrl+Enter` / `Cmd+Enter`), save (`Ctrl+S`). The response pane shows status,
time, size, and Pretty / Raw / Preview / Headers / Cookies / Timing tabs. All
sends go through the proxy and are recorded as the user's own calls.

**RNR-2 History.** Every send is in History, newest first, searchable, and can
be reopened as a request.

**RNR-3 Copy as code.** Copy any request as curl, `fetch`, and as a tool call
(`crm.getAccount({ id })`) for the code editor.

**RNR-4 Scripts.** Requests, folders and applications MAY have pre-request and
post-response scripts in JavaScript with a Postman-compatible subset of `pm`:
`pm.environment`, `pm.variables`, `pm.collectionVariables`, `pm.request`,
`pm.response` (`json()`, `text()`, `code`, `headers`), `pm.test`, `pm.expect`
(Chai). Scripts run in the same sandbox as tools (TOOL-4).
*Accept:* a fixture Postman collection with scripts runs with the same test
results as in Postman.

**RNR-5 Tests.** `pm.test` results show in a Test Results tab with pass/fail
per test.

**RNR-6 Script trust.** Imported scripts don't run until the user trusts that
application ("Run scripts in this collection"), like Bruno's safe mode.

**API-1 Read / write.** Each request is `read` (GET, HEAD, OPTIONS) or `write`
(everything else) by default. The user can override (a POST search is `read`).
Requests that look non-REST (SOAP envelopes, XML-RPC, GraphQL mutations, or a
GET whose path contains `delete`, `remove`, `update`, `create`, `set`, `reset`)
are flagged "check access" until someone confirms them.
*Accept:* the flag shows in the list and the runner; a tool using a flagged
request is treated as `write` until confirmed.

**EX-1 Examples.** Save a response as an example (`Ctrl+Shift+S`). Values of
fields named like secrets or personal data (`password`, `token`, `secret`,
`authorization`, `ssn`, `card`, `iban` and a configurable list) are masked
before storage, and the user can mask more by clicking a field.

**EX-2 Captured schema.** mcpDeck infers a JSON schema from saved examples,
merges across examples and shows where it differs from the imported spec. The
code editor's types come from this (TOOL-3).

## Credentials (M3)

**CRED-1 Auth types.** As AUTH-1. **Login request** is for legacy systems:
pick a request (e.g. `POST /login` with form fields), say what to capture
(cookie, header or a JSON path) and how long it lasts; mcpDeck runs it when
needed and adds the captured value to later calls.
*Accept:* a mock app with cookie login works through a tool without the tool
code handling the login.

**CRED-2 Local store.** Secrets are encrypted at rest with AES-256-GCM, one
random nonce per value, in SQLite. The data key is stored in the OS keychain:
Windows Credential Manager (DPAPI), macOS Keychain, Linux Secret Service. Only
when no keychain is available, a key file with owner-only permissions is used,
and Settings shows a warning saying so. The key is never stored in SQLite.
*Accept:* the database file alone decrypts nothing; tests cover each backend
through an interface with a fake.

**CRED-4 Never returned.** A secret MUST NOT appear in any MCP response, any
HTTP API response after save, logs, recorded calls, history, examples, exports
or error messages.
*Accept:* a CI test seeds canary secrets, drives every endpoint, MCP tool and
script path, and fails if a canary appears in any output, log line or stored
row.

**CRED-5 Write-only.** In the UI a saved secret shows as masked with Replace and
Delete. There's no Reveal. Replacing takes effect on the next call without
changing any tool.

**CRED-6 Per environment.** A credential belongs to an application and an
environment.

**CRED-7 Not readable by code.** Scripts and tools can't read secret values.
`pm.environment.get` on a secret variable returns `"[secret]"`. Secrets are
resolved only in the proxy when the request is sent.

**CRED-8 Honest threat model.** Settings › Security states plainly what
mcpDeck protects (agents over MCP never get keys; the database alone is
useless) and what it doesn't (a program running as you on the same machine,
including an agent with a shell, can ask your keychain or call the API
directly). The same text is in the README.

## Code tools (M4)

A tool is a short TypeScript file. It looks like a Postman script and a plain
async function, so people and agents can write it without learning anything.

**TOOL-1 Tool file.** A tool is one TypeScript module:

```ts
export const tool = {
  name: "customer_360",
  description: "One view of a customer: account, tier, balance, overdue invoices and open tickets.",
  useWhen: "A question is about one customer or account",
  dontUseWhen: "Searching across many customers",
  inputSchema: {
    type: "object",
    properties: { accountId: { type: "string", examples: ["001Ab00000XyZ"] } },
    required: ["accountId"],
  },
};

export default async function (input: Input) {
  const account = await crm.getAccount({ id: input.accountId });
  const [summary, tickets] = await Promise.all([
    billing.getCustomerSummary({ id: account.billingId }),
    support.searchIssues({ body: { jql: `org = "${account.jiraOrg}" AND status != Done` } }),
  ]);
  return {
    name: account.name,
    tier: account.tier,
    balance: summary.balance,
    overdueInvoices: summary.invoices.filter((i) => i.overdue),
    openTickets: tickets.issues.map((t) => ({ key: t.key, title: t.fields.summary })),
  };
}
```

`tool` has the same shape as an MCP tool definition plus `useWhen` /
`dontUseWhen`. `Input` is generated from `inputSchema`. The return value is
exactly what the agent gets.
*Accept:* the example above runs against the demo world (MOCK-3).

**TOOL-2 Runtime.** Globals available to tool code, and nothing else:
- one client per application, named after it (`crm`, `billing`), with one
  method per request, named after the request (`getAccount`). Arguments:
  `{ path, query, headers, body }` or flat params when unambiguous. Returns
  the parsed body; non-2xx throws an `ApiError` with status and body.
- `call(app, "METHOD /path", args)` for anything without a named request.
- `vars.<name>`: toolkit and connection variables (TOOL-9). Secrets read as
  `"[secret]"`.
- `console.log`, `console.error`: shown in test output and recorded.
- `pm.test`, `pm.expect` in tool test files (TOOL-8).
All network goes through the proxy with the connection's grants.

**TOOL-3 Code editor.** The editor is Monaco (the VS Code editor) with the
TypeScript language service:
- autocomplete for application clients, request methods, their params and
  response fields, with types from the spec or captured schema (EX-2);
- inline errors, hover docs, go-to-definition into the generated types,
  format on save, `Ctrl+S` to save, `Ctrl+Enter` to test;
- snippets for common shapes (one call, chained calls, parallel calls,
  paginate) that insert real code;
- a side panel listing applications and requests; clicking one inserts the
  call.
*Accept:* typing `crm.` lists every request in `crm`; typing `.` on a response
lists its fields from a saved example.

**TOOL-4 Sandbox.** Tool code runs in an embedded JS engine with no
filesystem, network, environment, timers beyond the step timeout, `eval` of
strings, or `require`/`import` of anything but the runtime. The clients and
`call()` are the only way out and go through the proxy.
*Accept:* sandbox escape suite (filesystem, network, prototype pollution,
infinite loop, memory bomb, deep recursion, ReDoS-style regex, huge strings)
passes.

**TOOL-5 Output.** The return value is the output. The test panel shows the
size of each upstream response and of the tool's output, in bytes and tokens,
so the saving is visible while editing.

**TOOL-6 Limits.** Per tool: timeout (default 10 s), memory cap (default
32 MB), max output size (default 64 KB), retries for idempotent reads.

**TOOL-7 Test run.** "Test" (`Ctrl+Enter`) runs the tool with an input form
generated from `inputSchema`, against real APIs or against saved examples
(no network). The result panel shows the output, console, and every upstream
call as a timeline with request and response, like the Network tab in browser
dev tools.

**TOOL-8 Tool tests.** A tool MAY have test cases: an input plus a script using
`pm.test` / `pm.expect` on the output. They can be created from a test run or
a recorded call ("Save as test"). Saving runs them against examples.

**TOOL-9 Bound inputs.** A toolkit and a connection MAY define variables
(VAR-2 sources) that tools read as `vars.<name>`. They are not part of the
input schema, so the agent never sees or supplies them.
*Accept:* `vars.tenantId` set on a connection is used in the call and does
not appear in `tools/list`.

**TOOL-10 Create from requests.** In the runner or History, select one or more
requests and choose "Create tool". mcpDeck generates the tool file: one call
per request, values that flow from one response into a later request wired up
(found by matching values), independent calls in `Promise.all`, and a
`return` of the fields the user ticks.

**VER-1 Versions are immutable.** Saving creates version N+1. A version never
changes after creation.

**VER-2 Pinning.** A connection gets each tool at `latest` or a pinned
version. Pinned connections keep getting it after newer saves.

**VER-3 Breaking change check.** On save, compare input schema and output shape
(from tests and recent calls) with the previous version and flag breaking
changes; connections on `latest` move only when non-breaking or confirmed.

**VER-4 Diff and roll back.** Any two versions can be diffed side by side, like
a code review. "Roll back" creates a new version equal to an older one.

**FILE-1 Tools as files.** `mcpdeck pull <toolkit> <dir>` writes each tool as
`<name>.ts`, plus `mcpdeck.d.ts` (all client types) and a `tsconfig.json`, so
any editor or coding agent can edit and type-check them with its normal
tools. `mcpdeck push <dir>` saves changed files as new versions, runs tests,
and prints a diff summary. Same file format as TOOL-1.
*Accept:* pull → edit → `tsc --noEmit` passes → push creates a version.

## Toolkits and MCP (M5)

**TK-1 Toolkits.** A toolkit is a named set of tools. Its tools can use
requests from any application. A toolkit has variables (TOOL-9), a default
environment per application, and a "can build tools" switch for its
connections (MCP-3).

**TK-2 One endpoint per toolkit.** Each toolkit is its own MCP server at
`/mcp/<toolkit>`, Streamable HTTP. `mcpdeck stdio --toolkit <name>` bridges
it for clients that only speak stdio. An agent connected to one toolkit
can't see or call another's tools.

**TK-3 Server instructions.** Each endpoint sends MCP `instructions` on
initialize: what the toolkit is for, and, when building is allowed, the
loop in five lines (search tools → search APIs → describe → run → save). No
skill file is needed for an agent to use it.

**CON-1 Connections.** Adding a toolkit to a client creates a connection: a
name ("Claude Code on this laptop"), a token (stored hashed), an access level
(`read` or `read_write`), an environment per application, variables, and
version pins. Shown on the Agents page.

**CON-2 Add to client.** The toolkit page has "Add to…" for Claude Code,
VS Code (Copilot), Cursor and Claude Desktop. For each it offers, in this
order:
1. the client's own command, ready to copy (e.g.
   `claude mcp add --transport http <name> http://127.0.0.1:47800/mcp/<name> --header "Authorization: Bearer …"`);
2. "Write config for me": updates the client's config file (user or project
   scope, chosen), after showing the exact change, keeping a backup;
3. the JSON snippet to paste.
Paths are correct for Windows, macOS and Linux.
*Accept:* on each OS in CI, the written file parses and lists the server;
existing entries in the file are kept.

**CON-3 Revoke and rotate.** Revoking a connection refuses its token
immediately and ends its open sessions within one second. Rotating issues a
new token and offers to rewrite the client config.

**ACC-2 Writes blocked at the proxy.** With `read` access, any request whose
classification is `write` (after overrides and API-1 flags) is refused at
the proxy, whatever the tool says. Write tools are also left out of
`tools/list` for read connections.
*Accept:* table-driven test across all methods × access levels.

**ACC-3 Least exposure.** `tools/list` returns only the toolkit's tools that the
connection may use, at its pinned versions. If a toolkit has more than 25
tools (configurable), the list holds the 15 most used plus `find_tool` and
`use_tool`, so the context stays small.

**ACC-6 Agents can't escalate.** A connection token MUST NOT be able to:
change access, approve anything, make a write tool live, read or change
credentials or variables, or change its own limits. Tested per endpoint.

**MCP-2 Published tools.** Tools appear as normal MCP tools. `description`
is built from description, "Use when" and "Don't use when". `readOnlyHint`
and `destructiveHint` annotations follow the tool's access.

**MCP-3 Build tools.** When the toolkit allows building, connections also get:

| Tool | Purpose |
|---|---|
| `search_apis` | Find requests by intent across the toolkit's applications. Returns one line each: `crm.getAccount({ id }) → Account · GET /accounts/{id} · read` |
| `describe_api` | The TypeScript signature and response type, and one masked example |
| `run_api` | Send one request through the proxy, with a compact response (large arrays trimmed, with a count) |
| `run_script` | Run TypeScript in the tool sandbox with the same runtime as TOOL-2, for exploring |
| `save_tool` | Save a tool file (TOOL-1); returns version, access and test results |
| `test_tool` | Run a tool file against examples or real APIs |
| `request_tool` | Ask a person for a tool the agent can't build (missing API, write access) |

Responses are short by default; an agent asks for more with `detail: true`.

**MCP-4 Actionable errors.** Every error says what went wrong and what to do
next. E.g. "write blocked: this connection is read-only. Use a read tool,
or ask the user to switch this connection to read+write in mcpDeck."

**AB-1 Agents save tools.** A version saved by an agent records the
connection, session and the calls it was built from.

**AB-2 Read tools go live.** A version that only reads, whose tests pass and
whose description passes AB-6, goes live automatically (can be turned off per
toolkit).

**AB-3 Write tools wait.** A version that writes is `pending` until the user
approves it in the UI or CLI. The review shows the diff, the requests and
methods it uses, test results and the session it came from.

**AB-4 Requests.** `request_tool` creates an item on Home with the agent's
description and the calls it tried.

**AB-5 Duplicate check.** `save_tool` warns, and the UI shows, when a new tool
uses the same requests as an existing one, suggesting the existing tool.

**AB-6 Description check.** Agent-written descriptions are checked for length
and for text that addresses other agents ("ignore", "always call", "you
must", URLs). Matches keep the version pending for review.

## Calls and usage (M6)

**SES-1 Sessions.** A session is an MCP session (`Mcp-Session-Id`) on a
connection. It records the client name and version from `initialize`, start,
last activity and how it ended (closed, idle 30 min, revoked).

**SES-2 Every call recorded.** Tool calls, build tool calls, `tools/list`, and
every upstream request (from tools, the runner and scripts) are recorded
with: who (connection or "you"), session, time, duration, tool and version or
request, input (masked), output size, status, error, tokens, and the parent
call for upstream requests.

**SES-3 Tokens.** mcpDeck counts tokens of everything it returns to an agent,
including the `tools/list` payload (the standing cost of a toolkit), with a
named offline encoder. The UI labels these "tokens returned by mcpDeck" and
never estimates tokens it didn't see.

**SES-4 Call views.** Agents page: connections with last seen, calls today,
errors. A connection: its sessions. A session: a table of calls like the
browser Network tab (name, status, type, duration, size, tokens, waterfall),
with filters and a detail pane (Input, Output, Upstream calls, Timing).

**USE-1 Usage.** Per tool, per toolkit and per connection, over 24 h / 7 d /
30 d: calls, error rate, p50 and p95 duration, average output tokens,
upstream calls per call, and `tools/list` cost.

**SES-5 Retention.** Default 30 days for call bodies and 1 year for call
metadata, set in Settings.

**AUD-1 Audit log.** Append-only record of changes to applications, requests,
credentials (that they changed, never values), tools, versions, toolkits,
connections and access; approvals; revocations. Each entry: actor (you or a
connection), action, target, time, detail.

**AUD-2 Tamper evidence.** Each audit entry stores the hash of the previous
entry; `mcpdeck audit verify` checks the chain.

## Suggestions (M7)

**INS-1 Rules, not models.** Suggestions come from deterministic rules over
recorded calls. Each one shows the evidence (the sessions and calls),
the expected saving, and an action that opens a prefilled change. It can be
dismissed. Once acted on, the before/after numbers are shown.

**INS-2 Make a tool.** The same sequence of `run_api` or `run_script` calls
appears in 3+ sessions → suggest a tool, generated as in TOOL-10.

**INS-3 Extend a tool.** A tool is followed by the same upstream request
(using its output) in most sessions → suggest adding that call to the tool.

**INS-4 Trim output.** A tool's output is large (default > 2,000 tokens) →
show which fields take the most bytes and suggest dropping or shortening
them.

**INS-5 Parallel calls.** Two upstream calls in a tool ran one after the other
with no data flowing between them → suggest `Promise.all`, with the time it
would save.

**INS-6 Paginate in the tool.** An agent calls the same request with
increasing page or offset values → suggest a tool that paginates.

**INS-7 Input mistakes.** Calls to a tool fail and are retried with a changed
input → show the common error and the fix, and suggest a better schema or
description.

**INS-8 Known values.** The same literal value is passed for an input in most
calls of a connection → suggest a bound variable (TOOL-9).

**INS-9 Unused tools.** A tool in a toolkit is not called for 14 days →
suggest removing it, showing the `tools/list` tokens saved.

**INS-10 Repeated calls.** The same call with the same input repeats inside a
session → flag it, so the description or output can be fixed.

**INS-11 Blocked writes.** Write attempts blocked by ACC-2 → list them, so a
person can decide on access or a read tool.

## Mock servers and dev mode (M1, grows each milestone)

Designed in [testing.md](testing.md#mock-servers).

**MOCK-1 Mock from an application.** `mcpdeck mock <app>` serves a fake version
of an application from its spec and saved examples on a local port:
deterministic, seeded, schema-valid responses.

**MOCK-2 Fault injection.** Mocks MAY add latency, error rates, rate limiting
and malformed responses per request, from flags or config.

**MOCK-3 Demo world.** A built-in set of related mock applications (CRM,
billing, ticketing, and a legacy SOAP-style service with cookie login) with
consistent seeded data, used by `mcpdeck dev`, e2e tests and demos.

**MOCK-4 Dev mode.** `mcpdeck dev` starts mcpDeck with a throwaway data
directory, the demo world, a `dev` toolkit, and a connection with a fixed
token, so a coding agent working on this repo can test end to end.

**MOCK-5 Record → mock.** Recorded calls from a session MAY be turned into mock
responses, so a bug seen against a real system can be reproduced offline.

## UI

**UI-1 Five places.** Navigation is Home, Applications, Toolkits, Agents,
Settings. No onboarding wizard. Home shows: suggestions, tool requests,
pending versions, and recent activity; when empty, it shows three real
actions (Import an API, Create a toolkit, Try the demo).

**UI-2 Look.** Clean and calm, Apple-like: frosted panels over a soft neutral
background, system font, black and blue actions, coloured mono chips for HTTP
methods. No purple, no emoji, no gradients on content. See
[design.md](design.md#ui).

**UI-3 Familiar patterns.** Each screen follows the pattern listed in
[design.md](design.md#familiar-patterns). Shortcuts match Postman and VS Code
and show `Ctrl` on Windows and Linux, `⌘` on macOS.

**UI-4 Nothing fake.** Every button, field and menu item works and is
persisted. No "coming soon", no sample data outside the demo, no lorem ipsum.
Input placeholders show a real example value, never instructions. Every list
has a real empty state and every action a real error state.
*Accept:* an e2e test clicks every enabled control on every screen with no
console error and no unhandled request.

**UI-5 Words.** Short, plain, specific. Banned in UI text and docs:
"seamless", "effortless", "powerful", "unlock", "supercharge", "leverage",
"revolutionize", "AI-powered", "elevate". Checked in CI.

**UI-6 Command palette.** `Ctrl+K` / `⌘K` finds any application, request,
toolkit, tool, connection or setting, and runs actions ("New request",
"Add toolkit to Claude Code").

**UI-7 Accessible.** Keyboard reachable, visible focus, WCAG AA contrast,
works at 1024 px and down to tablet width.

**UI-8 Dark mode.** Follows the system setting, with a switch in Settings.

**UI-9 Offline.** The UI ships inside the binary and works with no network.
Fonts are bundled.

## Non-functional

**NFR-1 Light.** Binary < 60 MB (the code editor is most of the UI);
idle memory < 80 MB; cold start < 300 ms. Measured in CI per release.

**NFR-2 Fast.** Proxy overhead p50 < 2 ms, p99 < 10 ms excluding upstream time,
at 200 requests/second on a laptop. A one-call tool adds < 5 ms over a raw
request.

**NFR-3 Zero config.** First run needs no flags, files or account.

**NFR-4 Windows first-class.** Tests run on Windows in CI. Paths, line
endings, file locking and the keychain are handled per OS.

**NFR-5 Trustworthy release.** Release binaries are signed (Authenticode on
Windows, notarised on macOS), with checksums and an SBOM.

**NFR-6 Test-first.** See [testing.md](testing.md). Coverage of `internal/`
≥ 80%, and every MUST requirement has at least one test referencing its ID.

---

## Phase 2: Prod

Listed so Phase 1 design leaves room. Not scheduled.

**BLD-1 Visual tool builder.** Build a tool without code: pick requests as
steps, map values by clicking fields in an example response, tick output
fields, add simple conditions. The builder writes the same TypeScript file
(TOOL-1). A file it can't represent opens in the code editor.

**BLD-2 Output field picker.** Pick, rename, flatten and cap fields by
clicking in a response, with bytes and tokens before and after; generates the
`return` statement.

**CRED-3 External vaults.** HashiCorp Vault, AWS Secrets Manager, Azure Key
Vault, GCP Secret Manager, 1Password through one `vault.Backend` interface.

**TEAM-1 Server mode.** `mcpdeck serve --db postgres://…` with the same
binary; listens beyond localhost only with auth configured.

**TEAM-2 People.** OIDC sign-in, roles (admin, author, viewer), approvals by
another person.

**TEAM-3 Promote from git.** `mcpdeck apply <dir>` applies applications,
toolkits and tools from files (FILE-1) and shows a plan first, like
`terraform plan`.

**TEAM-4 Per-user auth.** OAuth 2.0 authorization code per user, so calls run
as the person behind the agent.

**ACC-4 Hold for approval.** Mark write requests or tools as needing approval
per call; the call waits until a person approves or a timeout.

**ACC-5 Rate limits and budgets.** Per connection requests per minute and a
daily token budget, with actionable errors.

**USE-2 Client usage.** Receive token usage reported by clients (e.g.
OpenTelemetry from Claude Code) and show it next to mcpDeck's own counts,
labelled separately.

**SES-6 Replay.** Replay a recorded session against a newer tool version or a
mock and compare side by side.

**IMP-5 Re-import with diff.** Re-importing a source shows added, changed and
removed requests and the tool versions that would break.

**IMP-6 More formats.** Bruno, Insomnia, HAR (grouping `/orders/123` into
`/orders/{id}`), WSDL / SOAP.

**DRIFT-1 Drift.** Flag when upstream responses stop matching the captured
schema, with the tools affected.

**EXP-1 One-file export.** Export a `.mcpdeck` file with chosen parts;
credentials left out unless included with a passphrase (Argon2id +
AES-256-GCM). `mcpdeck serve file.mcpdeck` starts a working copy.

**AUD-3 Audit export.** JSON Lines and OpenTelemetry export, off by default.

**MCP-5 Downstream MCP.** Put existing MCP servers behind mcpDeck so their
tools get the same access rules and call log.

**SKL-3 Export as skill.** Export a toolkit as a Claude skill / Copilot
instructions / Cursor rule that calls through mcpDeck.
