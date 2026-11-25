# skills

Optional files `mcpdeck connect` can install into a user's project. An agent
doesn't need them: each toolkit endpoint sends MCP `instructions` on
initialize with what it needs to know (TK-3). These files help clients that
ignore `instructions`, and teams that want the loop written down in the repo.
They're product files shipped in the binary, not instructions for working on
this repo (those are in `AGENTS.md` and `.claude/`).

| Folder | Installed as |
|---|---|
| `claude/` | `.claude/skills/mcpdeck/SKILL.md` |
| `copilot/` | `.github/instructions/mcpdeck.instructions.md` |
| `cursor/` | `.cursor/rules/mcpdeck.mdc` |
