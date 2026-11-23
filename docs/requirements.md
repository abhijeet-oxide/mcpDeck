# Requirements

Each requirement has an ID, a milestone and acceptance criteria. Tests reference
IDs in their names or comments (`// REQ: TOOL-4`) so coverage of a requirement
can be found with `grep`.

Milestones:

| | Milestone | Outcome |
|---|---|---|
| **M1** | Applications | Import APIs, run them, save examples |
| **M2** | Access | Vault, agents, grants, proxy |
| **M3** | Sessions | Every call traced with time and tokens; audit log |
| **M4** | MCP + skill | Agents connect from Claude Code, Copilot, Cursor and use tools |
| **M5** | Tools | Builder, multi-step tools, output fields, versions, tests |
| **M6** | Agent-built tools | Agents save tools; approval rules |
| **M7** | Export | One-file export/import of a working copy |
| **Later** | | Insights, external vault plug-ins, WSDL, OIDC, Postgres, downstream MCP proxy |

MUST / SHOULD / MAY follow RFC 2119.

---

## Applications and APIs (M1)

**APP-1 Create an application.** A user MUST be able to create, rename and
delete an application with a name, icon and one base URL per environment
(dev / staging / prod).
*Accept:* CRUD via UI, CLI and HTTP API; deleting an application with tools
that use it is refused with a list of those tools.

**APP-2 Allowed hosts.** Each application MUST have an allow-list of outbound
hosts, defaulting to the hosts of its base URLs. Calls to any other host MUST
be refused.
*Accept:* a tool step or `run_api` call to a non-listed host fails with
`host_not_allowed` and is recorded.

**IMP-1 OpenAPI / Swagger import.** Import OpenAPI 3.x and Swagger 2.0 from a
file or URL.
*Accept:* every operation becomes an API with method, path, params, request
and response schema; golden-file tests over at least five public specs.

**IMP-2 Postman import.** Import Postman collection v2.1, including folders,
variables and example responses (which become examples).

**IMP-3 curl import.** Paste one or more curl commands; headers that look like
credentials are moved to a credential and removed from the API.
*Accept:* `Authorization`, `Cookie`, `X-Api-Key` and query `api_key`/`token`
are never stored on the API.

**IMP-4 HAR import.** Import a HAR file, keeping only requests to chosen hosts,
grouping identical paths with different IDs into one templated API
(`/orders/123` → `/orders/{id}`).

**IMP-5 Re-import with diff.** Re-importing a source MUST show added, changed
and removed APIs and list tool versions that would break.

**API-1 Read / write classification.** Each API is `read` (GET, HEAD, OPTIONS)
or `write` (everything else) by default. A user MAY override (e.g. a POST
search is `read`). The classification is stored and shown everywhere.

**API-2 Run an API.** The UI MUST offer a Postman-style runner: environment,
params, headers, body, credential, Send. Requests go through the proxy so
credentials are injected and the call is recorded.

**EX-1 Save as example.** A response MAY be saved as an example. Fields marked
sensitive are masked before storage.

**EX-2 Captured schema.** mcpDeck MUST infer a JSON schema from saved examples,
merge across examples, and show where it differs from the imported spec.

## Credentials (M2)

**CRED-1 Auth types.** API key (header or query), bearer, basic, OAuth2 client
credentials, custom header. Later: OAuth2 authorization code (per user), mTLS,
SOAP WS-Security.

**CRED-2 Local vault.** Secrets are encrypted at rest with AES-256-GCM. The
master key comes from `MCPDECK_MASTER_KEY` or a key file; if neither exists, one
is generated on first run with 0600 permissions.

**CRED-3 External vaults (Later).** HashiCorp Vault, AWS Secrets Manager, Azure
Key Vault, GCP Secret Manager, 1Password through one `vault.Backend` interface.

**CRED-4 Never returned.** A secret MUST NOT appear in: any MCP response, any
HTTP API response after save, logs, recorded calls, examples, exports (unless
explicitly included and passphrase-encrypted), or error messages.
*Accept:* a CI test seeds canary secrets, drives every endpoint and MCP tool,
and fails if a canary appears in any output, log line or stored row.

**CRED-5 Rotation.** Replacing a secret takes effect on the next call without
changing any tool.

**CRED-6 Per environment.** A credential is bound to an application and an
environment.

## Agents and access (M2)

**AGT-1 Register an agent.** Name, owner, client type (Claude Code, Copilot,
Cursor, other). Creating an agent issues a token shown once; only its hash is
stored.

**AGT-2 Revoke access.** One action MUST revoke an agent's token and end its open
sessions within one second. Revoked tokens are refused with `401`.

**AGT-3 Rotate token.** Issue a new token; the old one stays valid for a grace
period (default 0, configurable).

**ACC-1 Grants.** An agent is granted tools or whole projects, per environment,
with one access level: `read`, `read_write`.

**ACC-2 Enforced at the proxy.** With a `read` grant, any request with a write
method (after overrides) is refused at the proxy, whatever the tool claims.
*Accept:* table-driven test across all methods × grants.

**ACC-3 Least exposure.** MCP `tools/list` returns only tools the agent is
granted, at the versions it is pinned to.

**ACC-4 Needs approval.** A grant MAY mark specific tools or write APIs as
`needs_approval`: the call is held, shown in the UI, and runs only after a
person approves (or fails after a timeout).

**ACC-5 Rate limits.** Per agent requests per minute and optional daily token
budget. Exceeding either returns a clear, actionable MCP error.

**ACC-6 Agents can't escalate.** An agent token MUST NOT be able to: grant
access, approve anything, publish a write tool, read or change credentials,
or change its own limits. Tested per endpoint.

## Sessions and tracing (M3)

**SES-1 Session lifecycle.** A session starts when an agent authenticates
(first MCP `initialize` or first proxied call with a valid token) and stays
valid until the client disconnects, the agent is revoked, or it is idle longer
than the timeout (default 30 minutes).

**SES-2 Every call belongs to a session.** Published tools, meta tools
(`search_apis`, `run_api`, …) and raw proxy calls are all recorded against the
session with: start time, duration, tool or API, tool version, input (masked),
output size in bytes, status, error, and token count.

**SES-3 Token counting.** For every response mcpDeck returns to an agent it
MUST count tokens of that response (tiktoken-compatible encoder; the encoder
name is stored with the count). If the client reports its own usage (for
example Claude Code hooks or OpenTelemetry), mcpDeck SHOULD store it as
`client_tokens` on the session. The UI labels which is which. mcpDeck never
estimates tokens it did not see.

**SES-4 Session views.** Per agent: list of sessions (start, duration, calls,
tokens); per session: a timeline of calls with durations; per week: totals and
tokens per session.

**SES-5 Retention.** Configurable per application; default 30 days for call
bodies, 1 year for call metadata.

**SES-6 Replay.** A recorded session MAY be replayed against a newer tool
version or mock server and compared side by side.

**AUD-1 Audit log.** Append-only record of: application, API, credential,
tool, version, grant and agent changes; approvals and rejections; revocations;
exports. Each entry: actor (person or agent), action, target, time, detail.

**AUD-2 Tamper evidence.** Each audit entry stores the hash of the previous
entry; `mcpdeck audit verify` checks the chain.

**AUD-3 Export.** Audit log exportable as JSON Lines; OpenTelemetry export of
traces and metrics is optional and off by default.

## MCP server and agent skill (M4)

**MCP-1 Transport.** Streamable HTTP at `/mcp`, plus `mcpdeck stdio` for clients
that need stdio. Authenticated with the agent token.

**MCP-2 Published tools.** Granted tools appear as normal MCP tools with their
description, "use when / don't use when" context, input schema and examples.

**MCP-3 Meta tools.** Always available to agents (subject to grants):

| Tool | Purpose |
|---|---|
| `search_apis` | Find APIs by intent across granted applications |
| `describe_api` | Schema, captured schema and a masked example |
| `run_api` | Call an API through the proxy; read calls run without a person |
| `search_tools` | Find existing tools before building one |
| `save_tool` | Save a tool definition; creates a new version if it exists |
| `test_tool` | Run a tool definition against examples or a mock |
| `request_tool` | Ask for a tool the agent cannot build |

**MCP-4 Actionable errors.** Every error returned to an agent says what went
wrong and what to do next (e.g. "write not allowed: this agent has read access
to jira; ask a person to approve or use a read tool").

**SKL-1 One-command connect.** `mcpdeck connect claude|copilot|cursor`:
creates an agent if needed, writes the MCP client config for that tool, and
installs the mcpDeck skill / instructions. Idempotent.

| Client | Files written |
|---|---|
| Claude Code | `.mcp.json` (or user scope) and `.claude/skills/mcpdeck/SKILL.md` |
| VS Code + Copilot | `.vscode/mcp.json` and `.github/instructions/mcpdeck.instructions.md` |
| Cursor | `.cursor/mcp.json` and `.cursor/rules/mcpdeck.mdc` |

**SKL-2 Skill content.** The skill teaches the loop: search tools → if missing,
search APIs → describe → run → save tool → test. It contains no secrets and
never asks the agent to handle credentials.

**SKL-3 Export a tool as a skill.** Any tool or project MAY be exported as a
Claude skill / Copilot instructions / Cursor rule that calls through mcpDeck, so
access, credentials and audit still apply.

## Tools and versions (M5)

**PRJ-1 Projects.** Tools live in projects. A project MAY use APIs from any
number of applications.

**TOOL-1 Definition.** A tool has: name, description, `use_when`,
`dont_use_when`, input schema with examples, steps, output fields, error
mapping, and a computed access level (`write` if any step is write).

**TOOL-2 Steps.** A step calls one API. Steps run in order; a step MAY run in
parallel with others that don't depend on it. Values map from tool inputs or
earlier steps (`1.billingId`). Conditions and loops are Later.

**TOOL-3 Builder.** The UI builder is Postman-style: pick application and API,
map values, see a live response from examples or a real call. No code required.

**TOOL-4 Script steps.** A step MAY be a JavaScript function run in the
sandbox. The sandbox has no filesystem, network, environment, timers beyond
the step timeout, or `require`; `call(app, api, params)` is the only way out
and goes through grants and the proxy.
*Accept:* sandbox escape test suite (filesystem, network, prototype pollution,
infinite loop, memory bomb) passes.

**TOOL-5 Output fields.** The user picks which fields to return; mcpDeck can
rename, flatten and cap list length. The UI shows bytes and tokens before and
after.

**TOOL-6 Limits.** Per tool: timeout (default 10s), memory cap for scripts
(default 32 MB), retries for idempotent reads, max response size.

**TOOL-7 Local test.** "Test" runs the tool against saved examples (no network)
or the real APIs, and shows each step's request and response.

**TOOL-8 Tool tests.** A tool MAY have test cases (input → expected output
fields) built from examples or recorded calls. Saving runs them.

**VER-1 Versions are immutable.** Saving creates version N+1. A version never
changes after creation.

**VER-2 Pinning.** A grant pins an agent to a version or to `latest`. Agents
pinned to a version keep getting it after newer saves.

**VER-3 Breaking change check.** On save, mcpDeck compares input and output
schemas with the previous version and flags breaking changes; agents on
`latest` are only moved if the change is non-breaking or a person confirms.

**VER-4 Diff and roll back.** Any two versions can be diffed; "roll back"
creates a new version equal to an older one.

## Agent-built tools and approvals (M6)

**AB-1 Agents create tools.** An agent with the `build` permission MAY call
`save_tool`. The saved version records the agent, session and the calls it was
built from.

**AB-2 Auto-approve read tools.** A version that is `read` only and whose tests
pass is published automatically (setting on by default, can be turned off per
project).

**AB-3 Write tools need a person.** A version that writes is `pending` until a
person approves it in the UI or CLI. Approval with an agent token is refused.

**AB-4 Request a tool.** `request_tool` creates a request visible in the UI that
a person or another agent can fulfil.

**AB-5 Review view.** A pending version shows: diff from previous version, APIs
and methods used, test results, source session and measured savings.

## Export and import (M7)

**EXP-1 One-file export.** From Settings, export a `.mcpdeck` file. The user
picks: applications and APIs, projects and tools (all versions), examples,
agents and access, sessions (time range), audit log.

**EXP-2 Credentials.** Left out by default. MAY be included, encrypted with a
passphrase (Argon2id + AES-256-GCM).

**EXP-3 Run from export.** `mcpdeck serve file.mcpdeck` starts a working copy
from the file in an isolated data directory. Agent tokens are reissued.

**EXP-4 Tools as files.** `mcpdeck tools export <dir>` writes each tool as YAML
(+ JS for script steps) so teams can review tools in pull requests;
`mcpdeck tools import <dir>` reads them back.

## Mock servers and dev mode (M1, grows each milestone)

Captured here, designed in [testing.md](testing.md#mock-servers).

**MOCK-1 Mock from an application.** `mcpdeck mock <app>` serves a fake version
of an application from its spec and saved examples on a local port:
deterministic, seeded, schema-valid responses.

**MOCK-2 Fault injection.** Mocks MAY add latency, error rates, rate limiting
and malformed responses per API, from flags or config.

**MOCK-3 Demo world.** A built-in set of three related mock applications (CRM,
billing, ticketing) with consistent seeded data, used by `mcpdeck dev`, the
quickstart, e2e tests and demos.

**MOCK-4 Dev mode.** `mcpdeck dev` starts mcpDeck with a throwaway data directory,
the demo world, a seeded agent and printed connect commands, so a coding agent
in the same session (Claude Code, Copilot, Cursor) can connect and test end to
end without real systems.

**MOCK-5 Record → mock.** Recorded calls from a session MAY be turned into mock
responses, so a bug seen against a real system can be reproduced offline.

## UI

**UI-1 Four places.** Navigation is Home, Applications, Projects, Agents, plus
Settings. No onboarding wizard.

**UI-2 Look.** Light "liquid glass" style: frosted panels over a soft neutral
background, Inter / system font, black and blue actions, no purple, no emoji.
See [design.md](design.md#ui).

**UI-3 Accessible.** Keyboard reachable, visible focus, WCAG AA contrast,
works at 1024px and down to tablet width.

**UI-4 Dark mode.** SHOULD follow the system setting.

**UI-5 Embedded.** The UI ships inside the binary and works with no network
access.

## Non-functional

**NFR-1 One binary.** Single static binary for macOS (arm64, amd64), Linux
(arm64, amd64) and Windows (amd64). No CGO.

**NFR-2 Light.** Binary < 40 MB; idle memory < 50 MB; cold start < 200 ms on a
laptop. Measured in CI and reported on each release.

**NFR-3 Fast.** Proxy overhead p50 < 2 ms, p99 < 10 ms excluding upstream time,
at 200 requests/second on a laptop.

**NFR-4 Zero config.** `mcpdeck serve` works with no flags: SQLite in the user
data directory, port 8080, local vault.

**NFR-5 Postgres (Later).** Store behind an interface; Postgres for teams.

**NFR-6 Everything has an API.** Every UI action is an HTTP API call with an
OpenAPI description; the CLI uses the same API.

**NFR-7 Test-first.** See [testing.md](testing.md). Coverage of `internal/`
≥ 80%, and 100% of requirements tagged MUST have at least one test referencing
their ID.
