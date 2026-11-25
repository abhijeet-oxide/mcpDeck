# Testing

mcpDeck is built test-first. The point is not ceremony. Agents (and people) move
fast in this repo, and tests are how we know a change did what it claimed
without reading every line.

## The loop

1. **Find the requirement.** Every change maps to an ID in
   [requirements.md](requirements.md) (`TOOL-4`, `ACC-2`…). If there isn't one,
   add it first.
2. **Red.** Write the smallest test that fails for the right reason. Name it
   after the behaviour and tag the requirement:
   ```go
   // REQ: ACC-2
   func TestProxy_ReadGrantBlocksWriteMethods(t *testing.T) { … }
   ```
3. **Green.** Write the least code that passes.
4. **Refactor.** Clean up with the tests green.
5. **Run the whole suite** (`make test`) before committing.

Commit the failing test and the fix together. Don't delete or weaken a test to
get green. If a test is wrong, fix it in its own commit and say why.

## Layers

| Layer | Tool | Lives in | What it covers |
|---|---|---|---|
| Unit | Go `testing`, `go-cmp` | next to code, `*_test.go` | Pure logic: importers, variable resolution, access calc, schema merge, typegen, suggestion rules |
| Golden | Go + `testdata/` | `internal/<pkg>/testdata/` | Importers, OpenAPI → requests, `.d.ts` generation, client config writers, tool files. `go test ./... -update` rewrites |
| Integration | Go + `httptest` + real SQLite in `t.TempDir()` | `internal/<pkg>/*_test.go` | Proxy, sessions, MCP handlers, store |
| Security | Go | `internal/proxy`, `internal/tool`, `internal/server`, `e2e/` | Canary secrets (CRED-4, CRED-7), sandbox escapes (TOOL-4), escalation (ACC-6), SSRF (APP-2), local guard: Host × Origin × cookie × token (RUN-5) |
| UI unit | Vitest + Testing Library + MSW | `web/src/**/*.test.tsx` | Components and routes against mocked API |
| End to end | Playwright + real binary + mock apps | `e2e/` | Real flows through UI and MCP |
| MCP client | Go MCP client from the SDK | `e2e/mcp_test.go` | An agent's view: list tools, call, errors, build a tool with only `instructions` (TK-3) |
| Skill scripts | `node --test` | `skills/scripts.test.mjs` | `field-sizes.mjs` and `check-tool.mjs` on Linux and Windows, Node 18 (SKL-1) |
| Windows, macOS | Same Go suite on `windows-latest` and `macos-latest` | CI | Paths, keychain, client config locations, browser open (NFR-4) |
| Installers | Go runs `install.sh` / `install.ps1` against an `httptest` fake release | `scripts/install_test.go` | Platform detection, checksum check, user install, on each OS (NFR-7) |
| Release | GoReleaser snapshot + `tools/checkdist` + `govulncheck -mode=binary` | CI `release-dry-run` | Every artifact, checksum, size limit, version string, known CVEs in what ships (NFR-1, NFR-5) |
| Release tooling | Table tests | `tools/*/main_test.go` | Version rules from commits (NFR-8), dist checks |

Rules:

- No sleeps. Inject a clock (`internal/clock`) and ID generator.
- No network in unit or integration tests. Upstreams are `httptest` servers
  or `internal/mock`.
- Table-driven tests for anything with a matrix (methods × grants, auth types).
- Every bug fix starts with a test that reproduces it.
- Tests must pass with `-race` and in any order (`-shuffle=on`), on Linux and Windows.
- OS specifics (keychain, paths, config locations, browser) are tested through
  fakes everywhere and against the real OS in the Windows and Linux CI jobs.

## Coverage gates (CI)

- `internal/` line coverage ≥ 80%.
- Every MUST requirement has at least one test with `REQ: <ID>`.
  `make req-check` lists requirements without tests.
- The canary test (CRED-4) and sandbox suite (TOOL-4) can never be skipped.

## Mock servers

> Requirement only. Not built yet. See MOCK-1 … MOCK-5.

We need realistic upstream APIs in three places: tests, local development, and
a coding agent's session (Claude Code, Copilot, Cursor) where the agent
builds a feature and then checks it end to end itself.

### What we'll build

**`internal/mock`**: a Go package that serves an API as a fake server.

- Input: an API's requests (from any importer) plus its saved examples.
- Response choice: exact example match → closest example → generated from the
  response schema with a seeded faker. Same seed, same responses.
- Stateful mode (optional): `POST` / `PUT` / `DELETE` change an in-memory store
  so a write followed by a read behaves sensibly.
- Faults per API: latency (`300ms±100`), error rate (`5%:503`), rate limit
  (`10/min`), malformed body, timeout.
- Usable in Go tests as `mock.Start(t, app, opts...)` returning an
  `httptest.Server`.

**Demo world** (`examples/demo/`): four related mock APIs with
consistent seeded data. The same customer IDs appear across all three, so
cross-API tools make sense.

| API | Style | Shape |
|---|---|---|
| `acme-crm` | REST, OpenAPI 3 | accounts, contacts, owners |
| `acme-billing` | REST, Swagger 2, API-key auth | customers, invoices (large responses on purpose) |
| `acme-support` | Jira-like, Postman collection, bearer auth | issues, comments, search via POST |
| `acme-legacy` | SOAP-style XML over POST, cookie login form | customer notes, credit holds |

`acme-billing` returns deliberately bloated responses so output-field savings
are visible. `acme-support` uses `POST /search` so read overrides get exercised.
`acme-legacy` exercises login-request auth (CRED-1), XML bodies and the
"check access" flag (API-1).

**CLI**

```sh
mcpdeck mock acme-crm --port 9001 --seed 42 --faults "GET /accounts/*:latency=200ms"
mcpdeck dev                    # throwaway data dir + demo world + dev toolkit
```

`mcpdeck dev` prints:

```
mcpDeck  http://127.0.0.1:47800
mocks   acme-crm :9001 · acme-billing :9002 · acme-support :9003 · acme-legacy :9004
toolkit dev   http://127.0.0.1:47800/mcp/dev   token: mcpdeck-dev
```

Dev mode uses a fixed token, `mcpdeck-dev`, so the editor configs in this repo can
hard-code it. It's only accepted when mcpDeck runs with `dev` on `127.0.0.1` with
a throwaway data directory and mock APIs. Normal `serve` refuses it.

### Testing inside a coding-agent session

The goal: an agent working on this repo can build mcpDeck, start it with mocks,
connect to it as an MCP client, and check its own change. No real systems, no
human in the loop.

1. `make dev-up`: build, start `mcpdeck dev` in the background, wait for
   `/healthz`.
2. The repo's `.mcp.json` (Claude Code), `.vscode/mcp.json` (VS Code / Copilot)
   and `.cursor/mcp.json` (Cursor) define an `mcpdeck-dev` server at
   `http://127.0.0.1:47800/mcp/dev` with the dev token. After `make dev-up` the agent
   can call mcpDeck's own tools. (Until M5 ships, that server just shows as
   unavailable.)
3. The agent exercises the change: e.g. `search_apis "invoices"` → `run_api` →
   `run_script` → `save_tool` → call the new tool.
4. `make e2e` runs Playwright and the MCP client suite against the same
   instance.
5. `make dev-down` stops everything and deletes the data dir.

Because the mocks are deterministic, the agent can assert on exact responses.
Because faults are configurable, it can test retries and error messages.

**Record → mock (MOCK-5).** `mcpdeck mock --from-session <id>` replays the
responses recorded in a session. A bug seen against a real system can be
exported (`.mcpdeck` with that session) and reproduced offline.

## Commands

```sh
make test          # go test -race -shuffle=on ./... && pnpm -C web test
make test-go       # Go only
make test-web      # Vitest only
make test-skills   # agent skill scripts
make e2e           # Playwright + MCP client suite (starts mcpdeck dev)
make cover         # coverage report, fails under the gate
make req-check     # MUST requirements without a REQ-tagged test
make dev-up / dev-down
```
