# What makes a good MCP toolkit

mcpDeck checks every toolkit and every run against these rules (GUIDE-1…4).
The checks are plain code in `internal/lint`; no model is involved. The
Guidelines tab of a toolkit shows the same text.

Rules have three levels:

- **Must fix**: breaks agents or clients. Fix before you connect agents.
- **Avoid**: works, but costs tokens on every turn or makes agents pick the
  wrong tool.
- **Tip**: usually helps.

## 1. Build tools for tasks, not endpoints

An agent that has to call `login`, then `listApplications`, copy an ID, then
`listImages`, then `listVulnerabilities` pays for every response and can copy
the wrong ID. One tool that runs the chain and returns the answer is faster
and cheaper.

| Rule | Level | Check |
|---|---|---|
| `endpoint-wrappers` | Avoid | 80% or more of a toolkit's tools (5+) call a single request |
| `overlap` | Avoid | two tools call the same set of requests |
| `tool-count` | Avoid > 15, Must fix > 40 | tools in one toolkit (Cursor sends at most 40) |
| `list-tokens` | Avoid > 4,000, Must fix > 10,000 | tokens of `tools/list`, paid in every conversation |

## 2. Names and descriptions decide which tool gets picked

| Rule | Level | Check |
|---|---|---|
| `name` | Must fix | generic names (`new_tool`, `get_data`, `run`…); tip for one-word names |
| `description` | Must fix | empty, or the starter text left in |
| `description` | Avoid | under 25 characters, or over 1,024 with use-when text |
| `use-when` | Tip | toolkits with 4+ tools: say which question this tool answers |
| `write-guard` | Avoid | write tools without "don't use when" |

## 3. Inputs agents can fill correctly

| Rule | Level | Check |
|---|---|---|
| `param-name` | Avoid | vague names: `id`, `name`, `q`, `data`, `value`, `type` |
| `param-docs` | Tip | parameters with no description |
| `required` | Avoid | parameters but none required |
| `params` | Avoid | more than 8 parameters; bind fixed values as toolkit variables |

## 4. Return only what the task needs

| Rule | Level | Check |
|---|---|---|
| `raw-return` | Avoid | `return res;` with no `pick(...)` |
| `output-tokens` | Avoid > 2,000, Must fix > 25,000 | Claude Code cuts tool results over 25,000 tokens |
| `field-count` | Avoid | more than 25 leaf fields |
| `unbounded-list` | Avoid | a list of more than 50 items: add a `limit` input |
| `credential-in-output` | Must fix | fields named like tokens, passwords, keys |
| `blob` | Avoid | base64 or token-like strings |
| `deep-nesting`, `empty-fields`, `long-text`, `opaque-ids` | Tip | flatten, drop empties, summarise, return names next to IDs |

The score on a toolkit starts at 100 and loses 15 per must-fix and 5 per
avoid.

## Sources

- Anthropic, [Writing effective tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents):
  choose tools around workflows, namespace them, return meaningful and
  concise context, page and filter large results (Claude Code limits tool
  results to 25,000 tokens by default), describe parameters unambiguously,
  make errors actionable.
- Anthropic, [Code execution with MCP](https://www.anthropic.com/engineering/code-execution-with-mcp):
  tool definitions and intermediate results fill the context; filtering
  data in code before it reaches the model saves most of the tokens.
- Cloudflare, [Code Mode](https://blog.cloudflare.com/code-mode/) and
  [Code Mode MCP](https://blog.cloudflare.com/code-mode-mcp/): too many or
  too complex tools make models pick and use tools badly; a small fixed
  surface costs a fraction of the tokens.
- [Model Context Protocol: tools](https://modelcontextprotocol.io/specification/2025-06-18/server/tools):
  names, descriptions, input schemas and annotations.
