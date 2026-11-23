# Copilot instructions

The full instructions for this repo are in [AGENTS.md](../AGENTS.md). Read it
first. The essentials:

- mcpDeck is a single Go binary with an embedded React UI. It sits between AI
  agents and APIs: versioned tools, a credential vault, per-agent access,
  sessions with time and tokens, and an audit log.
- Work test-first. Every change maps to a requirement ID in
  `docs/requirements.md`; tests carry `// REQ: <ID>`.
- All outbound HTTP goes through `internal/proxy`. Credentials are injected
  there and must never reach an agent, a log or a recording.
- Read grants block write methods at the proxy. Agent tokens can't grant,
  approve or read secrets. Tool versions are immutable.
- UI must match `docs/screenshots/` and the tokens in `docs/design.md#ui`.
- Before finishing: `make test && make lint`. For proxy/MCP/UI changes also
  `make dev-up` and exercise the `mcpdeck-dev` MCP server from `.vscode/mcp.json`.
