# Testing

mcpDeck is built test-first. Agents and people move fast in this repo, and
the tests are how we know a change did what it claims without reading every
line.

## The loop

1. **Find the requirement.** Every change maps to an ID in
   [requirements.md](requirements.md) (`TOOL-4`, `ACC-2`…). If there isn't
   one, add it first.
2. **Red.** Write the smallest test that fails for the right reason. Name it
   after the behaviour and tag the requirement:
   ```go
   // REQ: ACC-2
   func TestDo_ReadGrantBlocksEveryWrite(t *testing.T) { … }
   ```
3. **Green.** Write the least code that passes.
4. **Refactor** with the tests green.
5. **Run the gates** below before you push.

Commit the failing test and the fix together. Don't delete or weaken a test
to get green. If a test is wrong, fix it in its own commit and say why.

## Layers

| Layer | Lives in | What it covers |
|---|---|---|
| Unit and integration | `internal/<pkg>/*_test.go` | Importers, proxy and auth, sandbox, store, MCP handlers, lint rules, against `httptest` servers and SQLite in `t.TempDir()` |
| Security | `internal/proxy`, `internal/tool`, `internal/server`, `internal/mcp` | Canary secrets, sandbox escapes, read grants, the local guard (Host, Origin, cookie, token) |
| Scenarios | `e2e/scenarios`, mocks in `e2e/mockenv` | A real `mcpdeck` binary driven like a user: CLI, the UI's HTTP API and a real MCP client, against mock JFrog, Anchore, Jira, GitHub, ServiceNow, Kubernetes, OAuth, legacy CSRF, HMAC and mTLS servers |
| Benchmark | `e2e/scenarios/bench_test.go`, `bench/results.json` | The README's token numbers, checked on every run |
| UI unit | `web/src/**/*.test.ts(x)` (Vitest, Testing Library) | Components and routes against a mocked API |
| Skill scripts | `skills/scripts.test.mjs` (`node --test`) | The agent skill scripts on Linux and Windows, Node 18 |
| Installers | `scripts/install_test.go` | `install.sh` and `install.ps1` against a fake release: platform, checksum, user install |
| Release tooling | `tools/*/main_test.go` | Next version from commits, dist checks, claims check, scenario report |

Rules:

- No network outside `httptest` and the mock servers. The quickstart
  against the public Petstore is checked by hand, not in CI.
- Prefer an injected clock (`internal/clock`) to sleeps. Scenarios that
  test retries and token expiry wait for real, for at most a few seconds.
- Table-driven tests for anything with a matrix (methods × grants, auth
  types).
- Every bug fix starts with a test that reproduces it.
- Tests pass with `-race` and in any order (`-shuffle=on`).

## Gates

| Gate | Command | Where |
|---|---|---|
| Go tests | `go test -race -shuffle=on ./...` | CI on Linux, macOS, Windows (scenarios skipped there with `MCPDECK_SKIP_SCENARIOS=1`) |
| Scenarios and benchmark | `make scenarios` | CI on every PR (about 2 minutes); nightly with `-race` on Linux and macOS; before every release |
| Coverage floor | `make cover` | CI: unit coverage of `internal/` may not drop below 45% |
| Go lint | `golangci-lint run` | CI |
| Web | `pnpm -C web exec biome check . && pnpm -C web test --run && pnpm -C web build` | CI |
| Claims | `make claims` | CI: every README claim names a test that exists ([claims.md](claims.md)) |
| Lint rule docs | `make lint-docs` | CI fails when `docs/lint-rules.md` is stale |
| Requirements | `make req-check` | Lists requirements with no `REQ:`-tagged test |

## The scenario suite

`go test ./e2e/scenarios` builds `mcpdeck`, starts the mock servers on free
ports, starts `mcpdeck serve` in a temporary folder, and runs every scenario
registered with `add(category, name, fn)`. Each system is onboarded once,
the way its documentation leads a person to: import what the vendor
publishes, store credentials, set auth.

```sh
make scenarios                                    # all, with a pass rate by category
cd e2e/scenarios && go test -run 'TestScenarios/auth' -v    # one category
cd e2e/scenarios && go test -args -report out.json          # JSON report
```

### Adding a scenario

Every feature and every bug fix that users can see adds at least one.

1. Copy [`e2e/scenarios/template_test.go.txt`](../e2e/scenarios/template_test.go.txt)
   to a `_test.go` file, or add to the file for that system.
2. If the system is new, add a mock in `e2e/mockenv` that behaves like the
   real one (same paths, status codes, error bodies, pagination), and an
   `onboard` entry in `setup_test.go`.
3. Assert what an agent would see: tool output, error text, what was sent
   upstream. Never assert on internals.
4. If the README claims the behaviour, add the scenario to
   [claims.md](claims.md).

## Dev mode

`mcpdeck dev` starts a demo in temporary data with five mock APIs (ports
9001 to 9005), toolkits, agents and a random dev agent token it prints.
`mcpdeck dev --fixed-token` uses the token `mcpdeck-dev` instead, for the
editor configs in this repo. `make dev` runs it with Vite on port 5173 for
UI work with hot reload.
