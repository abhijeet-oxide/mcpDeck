<!-- Title: a conventional commit, e.g. `feat(proxy): block write methods for read grants`.
     feat/fix/perf release on merge; docs/ci/test/chore/build/refactor don't. -->

## What and why

<!-- One or two sentences. -->

Requirements: <!-- e.g. ACC-2, SES-3 -->

## How it was tested

- [ ] Failing test written first, tagged `// REQ: <ID>`
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] For proxy / MCP / session / UI changes: exercised with `make dev-up` (describe below)

<!-- Commands run and what you saw. -->

## Checklist

- [ ] No invariant in AGENTS.md is weakened
- [ ] No secrets, `.mcpdeck/` data or real API responses committed
- [ ] UI changes match `docs/mockups/` and `docs/ui.md` (add a screenshot)
