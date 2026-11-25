@AGENTS.md

## Claude Code notes

- Project skills in `.claude/skills/`:
  - `tdd`: the red → green → refactor loop with requirement IDs. Use it for any
    behaviour change.
  - `ui-screen`: building or changing a UI screen to match the mockups.
- `.mcp.json` defines `mcpdeck-dev`, which points at a local `mcpdeck dev`
  instance (the `dev` toolkit). After `make dev-up` you can call its tools (`search_apis`,
  `run_api`, `run_script`, `save_tool`…) to test your own change end to end. Until the MCP
  server exists (M5) it will show as unavailable. That's expected.
- For broad searches across the repo, prefer a subagent so the main context
  stays focused on the change.
- When a change touches an invariant in AGENTS.md, re-read the related test
  before editing and run it afterwards.
