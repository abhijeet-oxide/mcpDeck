# Design

How mcpDeck is built. For *what* it must do, see [requirements.md](requirements.md).

## Modes

| | Dev (Phase 1) | Prod (Phase 2) |
|---|---|---|
| Start | `mcpdeck` / double-click `mcpdeck.exe` | `mcpdeck serve --db postgres://… --listen :443` |
| Store | SQLite (`modernc.org/sqlite`) | Postgres, same `store` interface |
| Secrets | `vault/local`: AES-GCM, key in OS keychain | `vault/<backend>` plug-ins |
| People | One local user, cookie from the launch URL | OIDC, roles |
| Bind | `127.0.0.1` only | Configured, auth required |

Phase 1 code must not assume Phase 2 exists, and must not block it: the store
and vault are interfaces, and tools are plain files (TOOL-1) that can move
between instances.

## Architecture

```
  You: Web UI · CLI · editor             Agents: Claude Code · VS Code · Cursor · Claude Desktop
              │ HTTP API (/api)              │ MCP (/mcp/<toolkit>, or stdio bridge)
              ▼                              ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ server   local guard (Host, Origin, cookie, token) · OpenAPI · embedded UI   │
├──────────────┬──────────────────┬──────────────────┬─────────────────────────┤
│ api          │ tool             │ mcp              │ proxy                   │
│ importer     │ runtime (JS)     │ toolkit endpoints│ allow-list · access ·   │
│ variables    │ typegen (.d.ts)  │ build tools      │ auth + secrets ·        │
│ examples     │ versions · tests │ instructions     │ cookies · recording ·   │
│ schema       │ files            │                  │ token count             │
├──────────────┴──────────────────┴──────────────────┴─────────────────────────┤
│ toolkit · agent + grant · session (calls) · usage · insight (rules) · audit  │
├──────────────────────────────────────────────────────────────────────────────┤
│ vault: local AES-GCM + OS keychain        store: SQLite (Postgres in Phase 2)│
└──────────────────────────────────────────────────────────────────────────────┘
                                   │
               Your APIs · legacy systems · mocks (internal/mock)
```

Every outbound request, whether from the runner, a script, a tool or
`run_api`, goes through **one** code path in `internal/proxy`. That's where
the allow-list, access check, variable and secret resolution, auth, cookies,
recording and token counting happen. Nothing else makes an outbound HTTP call.

## Familiar patterns

We copy, we don't invent. When a screen or format is designed, start from the
row below and change as little as possible.

| mcpDeck | Copies | What we keep |
|---|---|---|
| APIs, folders, requests | Postman / Bruno collections | Tree on the left, tabs on top, request editor with Params / Auth / Headers / Body / Scripts, response pane with Pretty / Raw / Headers / Cookies / Timing |
| Environments and variables | Postman environments | `{{var}}` syntax, scope order, environment picker top right, unresolved vars in red |
| Auth | Postman auth tab | Type dropdown, "Inherit auth from parent" default |
| Scripts and tests | Postman scripts | `pm.*` subset, Chai `pm.expect`, Test Results tab |
| Script trust | Bruno safe mode | Imported scripts off until trusted |
| Tool code | Postman scripts + VS Code | Monaco, IntelliSense, `Ctrl+S`, problems inline |
| Tool definition | MCP tool + JSON Schema | `name`, `description`, `inputSchema` exactly as MCP |
| Tool versions | GitHub | Side-by-side diff, "Roll back" as a new version |
| Call log | Chrome DevTools Network tab | Table with waterfall, detail pane with tabs |
| Add to client | Each client's own CLI and config | `claude mcp add …`, `.vscode/mcp.json`, `~/.cursor/mcp.json` |
| Tokens shown once | GitHub / Vercel tokens | Copy now, can't be shown again |
| Command palette | VS Code / Linear / Raycast | `Ctrl+K`, fuzzy search, actions |
| Settings | macOS System Settings | Grouped list on the left, plain forms on the right |
| Tools as files | `kubectl` / `git` | `pull` / `push`, plain files, diff before apply |

The same goes for agents. An agent should succeed with mcpDeck using only
things it already knows: MCP, JSON Schema, TypeScript, `async`/`await`,
OpenAPI names, Postman `pm`. The build tools return TypeScript signatures, not
a custom format.

## Tool runtime

```
tool.ts ──esbuild (strip types)──▶ JS ──▶ engine (sandbox) ──call()──▶ proxy
                                            ▲
                       runtime: clients, call, vars, console, pm (tests)
```

- **Types.** `internal/tool/typegen` turns each API's requests and
  schemas (imported or captured, EX-2) into a `.d.ts`: one namespace per
  API, one method per request, typed args and response. Served at
  `/api/toolkits/{id}/types.d.ts` for Monaco, written by `mcpdeck pull`
  (FILE-1), and returned piecewise by `describe_api`.
- **Type checking** happens where TypeScript already runs: Monaco in the
  browser and `tsc` in the user's editor. The server only strips types and
  validates inputs and outputs at run time; it doesn't type-check.
- **Engine.** `goja` (pure Go, ES2015+ with `async`/`await` and promises). A
  spike before M4 confirms the syntax agents typically write (optional
  chaining, spread, `Promise.all`, destructuring) runs. If it doesn't, the
  fallback is QuickJS compiled to WebAssembly and run with `wazero`, still pure
  Go.
- **Isolation.** A fresh runtime per run, no globals but the runtime, an
  interrupt on timeout, a memory cap, and no `eval` or `Function` from strings.
- **Clients.** `crm.getAccount(args)` is a thin wrapper over
  `call("crm", "GET /accounts/{id}", args)`. `call` resolves variables, then
  hands the request to `proxy.Do` with the agent's grant.

## MCP

- One `mcp.Server` per toolkit, mounted at `/mcp/<toolkit>`, built with the
  official Go SDK. `mcpdeck stdio --toolkit x` is a small bridge to the same
  server for stdio-only clients.
- `tools/list`: the toolkit's live tools at the grant's pins, minus write
  tools for read grants. Over the size limit (ACC-3), the top tools plus
  `find_tool` / `use_tool`.
- `instructions` on initialize carries the toolkit's purpose and, when building
  is allowed, the five-line build loop (TK-3).
- Build tools (MCP-3) return compact text by default and full JSON with
  `detail: true`.
- Tool changes send `notifications/tools/list_changed`; clients that ignore it
  pick changes up on their next session.

## Tech stack

Choose boring, small, pure-Go dependencies. Every new dependency needs a reason
in the PR description.

### Backend: Go, one binary

| Concern | Choice | Why |
|---|---|---|
| Language | Go (latest stable, pinned in `go.mod`) | Static single binary, fast start, low memory, easy Windows builds |
| HTTP | `net/http` with method + pattern routing | No framework needed |
| MCP | `github.com/modelcontextprotocol/go-sdk` | Official SDK; Streamable HTTP and stdio |
| CLI | `github.com/spf13/cobra` | Standard, good help output |
| Database | `modernc.org/sqlite` | Pure Go, so no CGO and easy cross-compiling |
| Queries | `sqlc` (generated, checked in) | Typed Go from plain SQL, no ORM |
| Migrations | Embedded `.sql` files + a ~50-line runner | No extra dependency |
| JS engine | `github.com/dop251/goja` | Pure Go; no I/O unless we give it |
| TS → JS | `github.com/evanw/esbuild/pkg/api` | Pure Go, strips types in microseconds |
| OpenAPI | `github.com/getkin/kin-openapi` | OpenAPI 3 + Swagger 2 conversion |
| Keychain | `github.com/zalando/go-keyring` | Windows Credential Manager, macOS Keychain, Secret Service, no CGO |
| Tokens | `github.com/pkoukk/tiktoken-go` with embedded BPE | Offline counts, labelled with encoder name |
| Crypto | stdlib `crypto/aes`, `crypto/cipher` | No KMS needed |
| Logging | `log/slog` | Stdlib, structured |
| UI embed | `embed.FS` | Ships the React build in the binary |

Token counts are from an open encoder, not Claude's or another vendor's own
tokenizer. They're good for comparing before and after, and are labelled as
such (SES-3).

### Frontend: React

| Concern | Choice |
|---|---|
| Build | Vite + TypeScript (strict) |
| UI | React 19 |
| Routing, data | TanStack Router, TanStack Query |
| Styling | Tailwind CSS v4 + a small glass layer (`web/src/styles/glass.css`) |
| Accessible primitives | Radix UI (unstyled) |
| Icons | Lucide |
| Motion | Motion, springs only, respects `prefers-reduced-motion` |
| Code editor | Monaco, bundled with its TypeScript worker (offline) |
| Charts | uPlot |
| Font | Inter and JetBrains Mono, bundled |
| API client | Generated from the server's OpenAPI document |

### Tooling

| Concern | Choice |
|---|---|
| Package manager | pnpm |
| Lint / format (web) | Biome |
| Lint (Go) | golangci-lint, gofumpt |
| Tests | Go `testing` + `go-cmp`; Vitest + Testing Library + MSW; Playwright |
| Release | GoReleaser: signed binaries for Windows, macOS, Linux; checksums; SBOM |
| CI | GitHub Actions on Linux and Windows |

## Packages

```
cmd/mcpdeck/        main: wires dependencies, cobra commands
internal/
  server/          HTTP API, local guard, launch cookie, embedded UI
  api/             APIs, folders, requests, environments, variables, examples, schema capture
  importer/        openapi, swagger page, postman, curl → api types (pure functions)
  vault/           Backend interface; local AES-GCM + keychain
  toolkit/         tool library membership, toolkits, build scope, approval rules
  agent/           agents, tokens, grants, pins
  tool/            tool files, runtime, sandbox, typegen, versions, tests
  proxy/           the only outbound HTTP path
  mcp/             per-toolkit MCP servers, build tools, stdio bridge
  session/         sessions, call recording, token counting
  usage/           aggregates for usage views
  insight/         suggestion rules over recorded calls
  connect/         client config writers (Claude Code, VS Code, Cursor, Claude Desktop)
  audit/           append-only hash-chained log
  mock/            mock servers from specs + examples; demo world
  store/           SQLite, migrations/, queries/ (sqlc)
  clock/, version/ injected clock and IDs; build info
web/               React app; embed.go exposes dist/
skills/            optional files `mcpdeck connect` can install for each client
```

Rules:

- `internal/<pkg>` exposes a small interface and a constructor. Packages depend
  on interfaces, not on each other's internals. `cmd/mcpdeck` does the wiring.
- `importer`, `typegen` and `insight` rules are pure functions: easy to test
  with golden files.
- No package-level state. Pass `context.Context`, a clock and an ID generator so
  tests are deterministic.
- OS-specific code (paths, keychain, browser open, config locations) lives
  behind small interfaces with a fake for tests.

## Data model

```
setting(key, value)
api(id, name, icon, allowed_hosts[], auth, scripts_trusted)
folder(id, api_id, parent_id, name, auth, scripts)
request(id, api_id, folder_id, name, method, url, params, headers, body,
        auth, scripts, request_schema, response_schema, captured_schema,
        access: read|write, access_override, access_flag)
environment(id, api_id, name)
variable(id, scope: global|api|environment|toolkit|grant, scope_id,
         name, source: literal|os_env|dotenv|secret, value, secret_id)
secret(id, api_id, environment_id, kind, ciphertext, nonce, rotated_at)
login(id, api_id, request_id, capture, ttl)
example(id, request_id, request, response_masked, created_at)
history(id, request_snapshot, call_id, created_at)

toolkit(id, name, slug, build_scope_api_ids[], approval_rules_json)
tool(id, name, created_by)                          -- the library
toolkit_tool(toolkit_id, tool_id)
tool_version(id, tool_id, n, source_ts, definition_json, access,
             status: pending|live|retired, breaking, created_by_actor,
             source_session_id, approved_at, created_at)
tool_test(id, tool_id, input, script)

agent(id, name, client, token_hash, revoked_at, created_at)
grant(id, agent_id, toolkit_id, access: read|read_write, can_build, env_by_api,
      created_at)
pin(grant_id, tool_id, version: latest|n)
output_drop(grant_id, tool_id, paths[], approved_by, created_at)   -- OUT-1

session(id, agent_id, toolkit_id, mcp_session_id, client_name, client_version,
        started_at, last_seen_at, ended_at, end_reason)
call(id, session_id, parent_call_id, actor: you|agent, kind: tool|build|list|upstream,
     tool_version_id|request_id, started_at, duration_ms, status, error,
     input_masked, output_masked, output_bytes, tokens, token_encoder)

suggestion(id, rule, target, evidence_json, saving_json, state, created_at)
field_report(id, agent_id, session_id, tool_version_id, used[], unused[], note,
             created_at)                                         -- FB-1
tool_request(id, agent_id, toolkit_id, session_id, description, tried_calls,
             state, fulfilled_by_version_id)
audit_event(id, prev_hash, hash, actor_kind, actor_id, action, target, at, detail)
```

IDs are ULIDs (sortable, generated in Go). Times are UTC.

## Key flows

**Agent calls a tool**

1. MCP request to `/mcp/<toolkit>` with bearer token → `server` checks the local
   guard, resolves the agent and its grant for that toolkit, finds or opens the session (SES-1).
2. `mcp` resolves the tool at the grant's pin and validates input against
   `inputSchema`.
3. `tool` runs the code. Each client call goes to `proxy.Do(ctx, req)`.
4. `proxy` checks allow-list and access → resolves variables and secrets →
   adds auth and cookies (running the login request if needed) → sends →
   masks → records the upstream call.
5. `tool` checks the output size; `session` counts tokens of the response and
   records the tool call.

**Agent builds a tool**: `search_apis` → `describe_api` (TS signature) →
`run_api` / `run_script` → `save_tool` → `tool` strips types, runs tests,
checks the toolkit's approval rules (AB-2) → `live` if they all pass, else
`pending` (AB-3) → audit event → on live, `tools/list_changed` to every
connected agent of every toolkit containing the tool (AB-7).

**Agent asks for a tool**: `request_tool` → request on Home and in
`list_tool_requests` for building agents of that toolkit → a person or a
building agent saves a tool with `fulfils` → it goes live by the rules →
every agent of the toolkit gets it.

**Person builds a tool**: select requests in History → "Create tool" →
generated file in the editor (TOOL-10) → Test → Save.

**Add to Claude Code**: toolkit page → "Add to…" → Claude Code → mcpDeck
reuses or creates the agent, adds a `read` grant → shows `claude mcp add …` and offers to write
the config after showing the change (AGT-3) → audit event.

**Field feedback**: an agent calls `report_field_usage` (often after an MCP-6
hint) → `field_report` row → `insight` combines reports and observed use per
field (FB-2) → a suggestion: remove the field (new version) if no agent uses
it, or a per-agent output (OUT-1) if only some do → a person approves (AB-8).

**Suggestion**: after each session ends, and hourly, `insight` runs its rules
over new calls → new `suggestion` rows → Home. Acting on one opens the editor
with the change prefilled; the next week's numbers show before and after.

## Security model

1. Agents never receive credentials. Secrets resolve only in `proxy`; tool and
   script code sees `"[secret]"`.
2. Connection tokens can't approve, change access, make write tools live or
   read secrets (ACC-6).
3. Write requests are blocked at the proxy for read grants; requests
   flagged "check access" count as write until confirmed.
4. The sandbox has no filesystem, network, env, `require` or string `eval`;
   the clients and `call()` are the only exit and go through `proxy`.
5. Outbound hosts are allow-listed per API (SSRF). Private and
   link-local ranges are blocked unless an environment's base URL is in them.
6. Local server: `127.0.0.1` only, `Host` and `Origin` checked, UI cookie from a
   one-time launch URL, no CORS (RUN-5).
7. Secrets: AES-256-GCM, key in the OS keychain, never in SQLite; write-only
   in the UI.
8. Recorded inputs, outputs, history and examples mask sensitive fields.
9. Audit log is append-only and hash-chained.
10. Text from imported specs and agent-written descriptions is untrusted (prompt
    injection): shown to people for review, never executed, flagged by AB-6.
11. What we don't protect against is stated in the product (CRED-8): a process
    running as the same OS user can use the same keychain and network.

## UI

Reference screens: [mockups](mockups/) · [screenshots](screenshots/). The
screenshots predate the move from Projects to Toolkits and from the step list
to the code editor; the look and tokens still apply.

| Screen | File |
|---|---|
| Home | [home.png](screenshots/home.png) |
| API: requests, run, examples | [application.png](screenshots/application.png) |
| Toolkit: tool editor | [tool-builder.png](screenshots/tool-builder.png) (to be redrawn with the code editor) |
| Agents: sessions and calls | [agent-sessions.png](screenshots/agent-sessions.png) |
| Settings | [export.png](screenshots/export.png) (Export is Phase 2) |

### Icon

[`docs/assets/logo.svg`](assets/logo.svg): two cards from a deck, the front one
showing `< >` because tools are code. Ink tile, blue back card, white front
card. Favicons are in `web/public/` (`favicon.svg`, 32 px and 180 px PNGs,
512 px for app icons); regenerate them from the SVG if it changes.

### Diagrams

Diagrams are hand-made SVG, generated in light and dark versions by
[`docs/assets/how-it-works.mjs`](assets/how-it-works.mjs) (`node
docs/assets/how-it-works.mjs`). Edit the script, not the SVGs. README uses
`<picture>` so GitHub shows the right one for the reader's theme.

### Look

Light, calm, Apple-like. Frosted panels over a soft neutral background. Plenty
of white space, few colours, no purple, no emoji, no gradients on content.
Dense where work happens (request editor, code, call table), airy everywhere
else.

### Tokens

```css
:root {
  --bg:            #ECEFF5;                 /* page */
  --glow-1:        #BFD4FF;                 /* background glows, blurred 110px */
  --glow-2:        #D5E3EC;
  --glow-3:        #FFDCC8;
  --glass:         rgba(255,255,255,.56);   /* panels */
  --glass-strong:  rgba(255,255,255,.9);    /* selected rows, light buttons */
  --glass-border:  rgba(255,255,255,.8);
  --glass-blur:    blur(30px) saturate(180%);
  --shadow:        inset 0 1px 0 rgba(255,255,255,.95),
                   0 1px 2px rgba(20,30,60,.04),
                   0 14px 44px rgba(20,30,60,.08);
  --text:          #1D1D1F;
  --text-2:        #3A3A3C;
  --text-muted:    #6E6E73;                 /* ≥ 4.5:1 on glass */
  --accent:        #0071E3;                 /* primary action, links #0066CC */
  --ink:           #1D1D1F;                 /* dark buttons */
  --ok:   #1F7A35 on rgba(52,199,89,.16)    /* GET, live, passed */
  --info: #0058B0 on rgba(0,113,227,.12)    /* POST, read, version */
  --warn: #A04100 on rgba(255,149,0,.18)    /* PUT, needs review */
  --bad:  #B3261E on rgba(255,59,48,.13)    /* DELETE, blocked, revoke */
  --radius-panel: 24px; --radius-card: 16px; --radius-pill: 999px;
  --font: Inter, -apple-system, "Segoe UI Variable", system-ui, sans-serif;
  --mono: "JetBrains Mono", ui-monospace, Consolas, monospace;
}
```

Rules: panels are glass; cards inside panels are `rgba(255,255,255,.62)`;
buttons are pills (dark for primary page action, blue for confirm, light for
secondary); HTTP methods are always shown as coloured mono chips; numbers use
tabular figures. The code editor and call table use a solid surface, not
glass, so text stays crisp.

## Decisions

| Decision | Why | Revisit when |
|---|---|---|
| Tools are TypeScript files, not a step DSL | Agents and people already write it; one format for code, files, git and the Phase 2 visual builder | — |
| Monaco over CodeMirror | Real TypeScript IntelliSense, same editor as VS Code and Postman | Binary size becomes a problem |
| goja, QuickJS-on-wazero as fallback | Pure Go, easy to sandbox | Spike shows goja misses syntax agents write |
| Server strips types, doesn't type-check | No TypeScript compiler in Go; editors already check | — |
| One MCP endpoint per toolkit | Least exposure and small `tools/list` | — |
| Key in OS keychain | The database file alone is useless | — |
| Pure-Go SQLite | One binary, no CGO, cross-compile | Phase 2 Postgres |
| Proxy as the only outbound path | One place for security, recording and tokens | Never |
| Suggestions are rules, not an LLM | Deterministic, explainable, offline, free | — |
| Count tokens we return, with an open encoder | MCP doesn't carry usage; never guess | Clients report usage (USE-2) |
| Versions are immutable | Agents never break on save | Never |
