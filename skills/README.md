# skills

Agent skills that ship with mcpDeck and get installed into Claude Code,
VS Code (Copilot) or Cursor when a toolkit is added (SKL-1, SKL-2). They're
product files, not instructions for working on this repo (those are in
`AGENTS.md` and `.claude/`).

| Skill | Used when | Scripts |
|---|---|---|
| [`mcpdeck-tools`](mcpdeck-tools/SKILL.md) | Any task that needs a company system through mcpDeck | |
| [`mcpdeck-request-tool`](mcpdeck-request-tool/SKILL.md) | No tool fits, 3+ calls for one answer, or write access is missing | |
| [`mcpdeck-trim-output`](mcpdeck-trim-output/SKILL.md) | A response was much bigger than needed | `field-sizes.mjs` |
| [`mcpdeck-build-tool`](mcpdeck-build-tool/SKILL.md) | The agent can build and a tool is missing or requested | `check-tool.mjs` |

Agents never change a shared tool's output from feedback. They report what
they used (`report_field_usage`), mcpDeck combines reports across agents, and
a person approves any change (AB-8).

## Scripts

Node 18+, no dependencies, safe to run on any JSON or tool file:

```sh
node mcpdeck-trim-output/scripts/field-sizes.mjs testdata/search-issues.json
node mcpdeck-trim-output/scripts/field-sizes.mjs testdata/search-issues.json \
  --keep "total,issues[].key,issues[].fields.summary"
node mcpdeck-build-tool/scripts/check-tool.mjs testdata/bad-tool.ts
```

Tests: `make test-skills` (`node --test skills/scripts.test.mjs`).
`testdata/` holds synthetic fixtures only, never real API responses.
