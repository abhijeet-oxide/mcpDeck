# Design

How mcpDeck is built. For *what* it must do, see [requirements.md](requirements.md).

## Architecture

```
  People: Web UI · CLI · IDE            Agents: Claude Code · Copilot · Cursor
              │ HTTP API                    │ MCP (/mcp)        │ HTTP (/proxy/<app>/…)
              ▼                             ▼                   ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ server     auth (people + agent tokens) · sessions · rate limits · embedded UI│
├──────────────┬───────────────┬──────────────────┬────────────────────────────┤
│ app          │ tool          │ mcp              │ proxy                      │
│ importer     │ steps · goja  │ meta tools       │ allow-list · grants ·      │
│ examples     │ output fields │ published tools  │ credential injection ·     │
│ schema       │ versions·tests│                  │ recording · token count    │
├──────────────┴───────────────┴──────────────────┴────────────────────────────┤
│ agent (tokens, grants) · session (calls) · audit (hash chain) · insight      │
├──────────────────────────────────────────────────────────────────────────────┤
│ vault: local AES-GCM │ plug-ins later      store: SQLite (Postgres later)    │
└──────────────────────────────────────────────────────────────────────────────┘
                                   │
                    Your APIs · legacy systems · mocks (internal/mock)
```

Every outbound request, whether from a tool step, `run_api`, the UI runner or a
raw `/proxy` call, goes through **one** code path in `internal/proxy`. That's
where allow-listing, grant checks, credential injection, recording and token
counting happen. Nothing else is allowed to make an outbound HTTP call.

## Tech stack

Choose boring, small, pure-Go dependencies. Every new dependency needs a reason
in the PR description.

### Backend: Go, one binary

| Concern | Choice | Why |
|---|---|---|
| Language | Go (latest stable, pinned in `go.mod`) | Static single binary, fast start, low memory |
| HTTP | `net/http` with method + pattern routing | No framework needed |
| MCP | `github.com/modelcontextprotocol/go-sdk` | Official SDK; Streamable HTTP and stdio |
| CLI | `github.com/spf13/cobra` | Standard, good help output |
| Database | `modernc.org/sqlite` | Pure Go, so no CGO and easy cross-compiling |
| Queries | `sqlc` (generated, checked in) | Typed Go from plain SQL, no ORM |
| Migrations | Embedded `.sql` files + a ~50-line runner | No extra dependency |
| JS sandbox | `github.com/dop251/goja` | Pure Go ES engine; no I/O unless we give it |
| OpenAPI | `github.com/getkin/kin-openapi` | OpenAPI 3 + Swagger 2 conversion |
| Tokens | `github.com/pkoukk/tiktoken-go` with embedded BPE | Offline token counts |
| Crypto | stdlib `crypto/aes`, `crypto/cipher`; `golang.org/x/crypto/argon2` | No KMS needed |
| Logging | `log/slog` | Stdlib, structured |
| Telemetry | OpenTelemetry SDK, off by default | Optional export |
| UI embed | `embed.FS` | Ships the React build in the binary |

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
| Code editor | CodeMirror 6 (script steps, JSON) |
| Charts | uPlot |
| API client | Generated from the server's OpenAPI document |

### Tooling

| Concern | Choice |
|---|---|
| Package manager | pnpm |
| Lint / format (web) | Biome |
| Lint (Go) | golangci-lint, gofumpt |
| Tests | Go `testing` + `go-cmp`; Vitest + Testing Library + MSW; Playwright |
| Release | GoReleaser: binaries, Homebrew tap, Docker image |
| CI | GitHub Actions |

## Packages

```
cmd/mcpdeck/        main: wires dependencies, cobra commands
internal/
  server/          HTTP API, auth middleware, sessions, embedded UI
  app/             applications, APIs, examples, schema capture
  importer/        openapi, postman, curl, har → []app.API (pure functions)
  vault/           Backend interface; local AES-GCM implementation
  project/         projects
  tool/            definition, step runner, goja sandbox, output fields, versions, tests
  agent/           agents, tokens, grants, approvals
  session/         session lifecycle, call recording, token counting
  proxy/           the only outbound HTTP path
  mcp/             MCP server, meta tools, published tools
  insight/         patterns across sessions (Later)
  bundle/          .mcpdeck export / import
  audit/           append-only hash-chained log
  mock/            mock servers from specs + examples; demo world
  store/           SQLite, migrations/, queries/ (sqlc)
web/               React app; embed.go exposes dist/
skills/            files `mcpdeck connect` installs for each client
```

Rules:

- `internal/<pkg>` exposes a small interface and a constructor. Packages depend
  on interfaces, not on each other's internals. `cmd/mcpdeck` does the wiring.
- `importer` and output-field logic are pure functions: easy to test with
  golden files.
- No package-level state. Pass `context.Context`, a clock and an ID generator so
  tests are deterministic.

## Data model

```
application(id, name, icon, envs{name→base_url}, allowed_hosts[])
source(id, application_id, kind, location, hash, imported_at)
api(id, application_id, method, path, params, request_schema, response_schema,
    captured_schema, access: read|write, access_override)
credential(id, application_id, env, kind, backend, ref, created_at, rotated_at)
example(id, api_id, request, response_masked, created_by, created_at)

project(id, name)
tool(id, project_id, name, created_by)
tool_version(id, tool_id, n, definition_json, access, status: pending|live|retired,
             breaking, created_by_actor, source_session_id, approved_by, created_at)
tool_test(id, tool_id, input, expect)

agent(id, name, client, owner, token_hash, revoked_at, rate_limit, token_budget)
grant(id, agent_id, project_id|tool_id, env, access: read|read_write,
      pin: latest|version_n, needs_approval[])
approval(id, call_id|tool_version_id, state, decided_by, decided_at)

session(id, agent_id, client, started_at, last_seen_at, ended_at, end_reason,
        client_tokens)
call(id, session_id, kind: tool|meta|api, tool_version_id|api_id, started_at,
     duration_ms, status, error, input_masked, output_bytes, tokens, token_encoder)

tool_request(id, agent_id, session_id, description, state)
audit_event(id, prev_hash, hash, actor_kind, actor_id, action, target, at, detail)
user(id, name, role: admin|author|viewer)
```

IDs are ULIDs (sortable, generated in Go). Times are UTC.

## Key flows

**Agent call through a tool**

1. MCP request with bearer token → `server` resolves the agent, finds or opens
   the session (SES-1), checks rate limits.
2. `mcp` resolves the tool at the agent's pinned version and validates input.
3. `tool` runs steps. Each outbound call goes to `proxy.Do(ctx, req)`.
4. `proxy` checks the allow-list, grant and method → injects the credential →
   sends → masks → records the call.
5. `tool` applies output fields; `session` counts tokens of the final response
   and records the tool call.

**Agent builds a tool**: `search_apis` → `describe_api` → `run_api` (read runs
straight through) → `save_tool` → `tool` validates, computes access, runs tests
→ `live` if read-only and tests pass (AB-2), otherwise `pending` (AB-3) →
audit event.

**Revoke**: set `revoked_at` → `server` drops the token from the cache → open
sessions end with `end_reason=revoked` → audit event.

## Security model

1. Agents never receive credentials. Injection happens in `proxy` only.
2. Agent tokens cannot approve, grant, publish write tools or read secrets.
3. Write methods are blocked at the proxy for read grants.
4. The sandbox has no filesystem, network, env or `require`; `call()` is the
   only exit and goes through `proxy`.
5. Outbound hosts are allow-listed per application (SSRF protection). Private
   and link-local IP ranges are blocked unless the application's base URL is
   in them.
6. Recorded inputs, outputs and examples mask sensitive fields.
7. Audit log is append-only and hash-chained.
8. Text from imported specs and downstream MCP servers is untrusted (prompt
   injection) and is shown to reviewers, never executed.
9. The UI binds to `127.0.0.1` by default. Binding elsewhere requires
   `--listen` and a configured admin password.

## UI

Five reference screens: [mockups](mockups/) · [screenshots](screenshots/).

| Screen | File |
|---|---|
| Home | [home.png](screenshots/home.png) |
| Application: APIs, run, examples | [application.png](screenshots/application.png) |
| Project: build a tool | [tool-builder.png](screenshots/tool-builder.png) |
| Agent: sessions | [agent-sessions.png](screenshots/agent-sessions.png) |
| Settings › Export | [export.png](screenshots/export.png) |

### Look

Light, calm, Apple-like "liquid glass". Frosted panels over a soft neutral
background. Plenty of white space, few colours, no purple, no emoji, no
gradients on content.

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
  --font: Inter, -apple-system, system-ui, sans-serif;
  --mono: "JetBrains Mono", ui-monospace, monospace;
}
```

Rules: panels are glass; cards inside panels are `rgba(255,255,255,.62)`;
buttons are pills (dark for primary page action, blue for confirm, light for
secondary); HTTP methods are always shown as coloured mono chips; numbers use
tabular figures.

## Decisions

| Decision | Why | Revisit when |
|---|---|---|
| Pure-Go SQLite | One binary, no CGO, cross-compile | Team deployments need Postgres |
| goja over V8/QuickJS | Pure Go, easy to sandbox, fast enough for glue | Scripts need modern JS features goja lacks |
| Proxy as the only outbound path | One place for security, recording and tokens | Never |
| Count tokens we return; store client-reported totals separately | MCP doesn't carry usage; never guess | Protocol adds usage reporting |
| Versions are immutable | Agents never break on save | Never |
| No-code steps first, scripts second | Most tools are 1–3 API calls with mapping | — |
