@AGENTS.md

## Claude Code notes

- Project skills in `.claude/skills/`:
  - `tdd`: the red → green → refactor loop with requirement IDs. Use it for any
    behaviour change.
  - `ship`: PR → CI → release. Use when a change is ready, CI or a release
    is red, or a Dependabot PR or security alert needs handling.
- `.mcp.json` defines `mcpdeck-dev`, which points at a local `mcpdeck dev`
  instance (the `dev` toolkit's use endpoint). After `make dev-up` you can call its tools
  (`customer_360`, `search_tools`, `request_tool`…) to test a change end to end. The build
  tools are only on the builder endpoint: delegate to the `mcpdeck-dev-builder` subagent
  (`.claude/agents/`), the same way mcpDeck's users build tools. Without `make dev-up` both
  show as unavailable. That's expected.
- For broad searches across the repo, prefer a subagent so the main context
  stays focused on the change.
- When a change touches an invariant in AGENTS.md, re-read the related test
  before editing and run it afterwards.
