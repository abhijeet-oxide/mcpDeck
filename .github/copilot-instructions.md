# Copilot instructions

The full instructions for this repo are in [AGENTS.md](../AGENTS.md). Read it
first. The essentials:

- mcpDeck is a single Go binary with an embedded React UI. It sits between AI
  agents and APIs, including legacy ones: Postman-style APIs,
  TypeScript tools, one MCP endpoint per toolkit, a local credential store,
  per-agent access, calls with time and tokens, and rule-based
  suggestions. Phase 1 is dev mode on a laptop, Windows first-class.
- Work test-first. Every change maps to a requirement ID in
  `docs/requirements.md`; tests carry `// REQ: <ID>`.
- All outbound HTTP goes through `internal/proxy`. Credentials are injected
  there and must never reach an agent, a log or a recording.
- Read grants can't send write requests (blocked at the proxy).
  Connection tokens can't change access, approve or read secrets. Tool
  versions are immutable. The local server checks `Host` and `Origin`.
- UI uses the established patterns in `docs/ui.md`; no invented widgets.
- Every user-visible behaviour gets a scenario in `e2e/scenarios`; every
  README claim names its tests in `docs/claims.md`.
- Before finishing: `make test && make lint && make claims`. For proxy/MCP/UI
  changes also `make dev-up` and exercise the `mcpdeck-dev` MCP server from
  `.vscode/mcp.json`.
- Commit and PR titles are conventional commits. `feat`, `fix` and `perf`
  release automatically on merge to main (see `docs/releasing.md`).
