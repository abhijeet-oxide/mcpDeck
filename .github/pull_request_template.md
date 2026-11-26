<!-- Title: a conventional commit, e.g. `feat(proxy): block write methods for read grants`.
     feat/fix/perf release on merge; docs/ci/test/chore/build/refactor/style don't.
     The pr-title check enforces the format. -->

## What and why

<!-- One or two sentences. -->

Requirements: <!-- e.g. ACC-2, AUTH-2 -->

## How it was tested

<!-- Commands run and what you saw. -->

- [ ] Failing test written first, tagged `// REQ: <ID>` (bug fixes: a test that reproduces the bug)
- [ ] User-visible behaviour has a scenario in `e2e/scenarios`; `make scenarios` passes
- [ ] `make test` and `make lint` pass
- [ ] For proxy, MCP, session or UI changes: exercised with `make dev-up` or `make dev`

## Docs and claims

- [ ] README changed only with a test behind each new claim; `make claims` passes
- [ ] No comparisons with other products; numbers come from committed raw output (`make bench`)
- [ ] New builder action is on the UI, the CLI (`--json`) and the admin MCP server, with a row in `docs/agents.md`
- [ ] New lint rule has a source, a hit-and-miss test, and `make lint-docs` was run
- [ ] `CHANGELOG.md` has a line under Unreleased for anything users see

## Safety

- [ ] No invariant in AGENTS.md is weakened
- [ ] No secrets, `.mcpdeck/` data or real API responses committed
- [ ] UI changes follow `docs/ui.md`, with screenshots at 1440x900 and 1280x720
