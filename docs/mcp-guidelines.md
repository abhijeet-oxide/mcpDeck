# What makes a good MCP toolkit

mcpDeck checks every toolkit and every run against these rules (GUIDE-1…4).
The checks are plain code in `internal/lint`; no model is involved. The
Guidelines tab of a toolkit shows the same text.

Every rule, with its severity (block, warn or hint), what it checks, the fix
and its source, is listed in [lint-rules.md](lint-rules.md). That file is
generated from the code, so it can't drift. The principles behind the rules:

1. **Build tools for tasks, not endpoints.** An agent that has to call
   `login`, then `listApplications`, copy an ID, then `listImages`, pays for
   every response and can copy the wrong ID. One tool that runs the chain and
   returns the answer is faster and cheaper.
2. **Names and descriptions decide which tool gets picked.** Verb-first
   names, one or two sentences on what it returns, and "use when" text once a
   toolkit has several tools.
3. **Inputs agents can fill correctly.** Specific parameter names
   (`user_id`, not `id`), enums for closed sets, required parameters marked,
   and no credentials as inputs.
4. **Return only what the task needs.** Pick fields, limit lists, leave out
   blobs and credentials.
5. **Valid and safe.** Input schemas the spec accepts, no credentials in tool
   files, and write tools never marked read-only.

Only the last group blocks a save. Everything else is advice.

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
