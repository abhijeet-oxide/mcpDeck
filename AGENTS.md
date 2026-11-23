# AGENTS.md

Instructions for any coding agent working in this repo (Claude Code, Copilot,
Cursor, Codex). People: this is a good onboarding read too.

## What we're building

mcpDeck is a local, single-binary layer between AI agents and APIs. People import
APIs, store credentials in a vault, and build versioned tools from one or more
APIs. Agents connect over MCP, use the tools they're granted, and can build new
tools themselves inside those limits. Every call is recorded in a session with
time and tokens, and every change goes into an audit log.

Read before non-trivial work:

- [docs/vision.md](docs/vision.md): problem, mental model, principles
- [docs/requirements.md](docs/requirements.md): what to build, with IDs and milestones
- [docs/design.md](docs/design.md): architecture, stack, data model, UI tokens
- [docs/testing.md](docs/testing.md): TDD loop, test layers, mock servers

**Status:** design phase. Folders exist; code arrives milestone by milestone
(M1 first). Don't build ahead of the current milestone unless asked.

## Repo map

```
cmd/mcpdeck/      CLI entry and wiring
internal/        Go packages, one per feature (see docs/design.md#packages)
web/             React + Vite UI, embedded into the binary
skills/          files `mcpdeck connect` installs into Claude / Copilot / Cursor
examples/        demo world and sample imports
e2e/             Playwright + MCP client tests against a running binary
docs/            vision, requirements, design, testing, mockups, screenshots
```

## Commands

```sh
make dev         # Go server + Vite dev server with hot reload
make test        # all unit + integration tests (Go with -race, Vitest)
make lint        # golangci-lint + Biome
make e2e         # Playwright + MCP suite against `mcpdeck dev`
make dev-up      # start mcpdeck with mock apps in the background (127.0.0.1:8080)
make dev-down    # stop it and delete its data
make build       # ./bin/mcpdeck
```

Run `make test` and `make lint` before you say a change is done.

## How to work

1. **Start from a requirement.** Find its ID in `docs/requirements.md`. If the
   change isn't covered, add or edit the requirement first so the intent is
   written down.
2. **Test first.** Write a failing test tagged `// REQ: <ID>`, make it pass,
   then refactor. Commit test and code together.
3. **Check it for real when behaviour crosses a boundary.** For changes to the
   proxy, MCP tools, sessions or the UI, run `make dev-up` and exercise the
   change: call the `mcpdeck-dev` MCP server from your session, or drive the UI
   with Playwright. Report what you ran and what you saw.
4. **Keep diffs small and focused.** One behaviour per commit. Don't reformat
   unrelated code.
5. **Say what you didn't do.** If you skipped a test, couldn't run something, or
   left a TODO, say so plainly in your summary.

## Invariants (never break these)

These protect users' systems. Each has a test; don't weaken it.

- **Credentials never reach an agent.** Secrets are injected only inside
  `internal/proxy`. They must not appear in MCP responses, API responses,
  logs, recorded calls, examples or errors. (CRED-4)
- **All outbound HTTP goes through `internal/proxy`.** No other package
  creates an `http.Client` for upstream calls. That's where allow-lists,
  grants, recording and token counting live.
- **Read grants block write methods at the proxy**, regardless of what a tool
  says. (ACC-2)
- **Agent tokens can't escalate:** no granting, approving, publishing write
  tools, or reading secrets. (ACC-6)
- **Tool versions are immutable.** Saving creates a new version. (VER-1)
- **The JS sandbox has no I/O.** `call()` is the only way out. (TOOL-4)
- **The audit log is append-only.** (AUD-1)

If a task seems to need breaking one of these, stop and ask.

## Go conventions

- Stdlib first. A new dependency needs a one-line reason in the PR.
- Constructors take dependencies explicitly (`New(store, clock, ids)`);
  no package-level state, no `init()` side effects.
- `context.Context` first on anything that does I/O.
- Errors: wrap with `%w` and context; define sentinel errors for things callers
  branch on. Agent-facing errors must say what to do next (MCP-4).
- SQL lives in `internal/store/queries/*.sql`; regenerate with `make sqlc`.
- Tests: table-driven, `go-cmp` for diffs, `t.TempDir()` for SQLite, injected
  clock instead of `time.Sleep`.

## Web conventions

- TypeScript strict. Function components and hooks.
- Server state through TanStack Query; the API client is generated from the
  server's OpenAPI. Don't hand-write fetch calls.
- Styling with Tailwind and the tokens in `web/src/styles/tokens.css`. Match
  the look in `docs/screenshots/`: frosted glass panels, neutral background,
  black and blue pill buttons, coloured mono chips for HTTP methods. No purple,
  no emoji, no gradients on content.
- Accessible by default: real `<button>`/`<a>`, labels on inputs, visible focus,
  AA contrast.
- Navigation stays at Home, Applications, Projects, Agents and Settings. Ask
  before adding a top-level item.

## Commits and PRs

- Conventional commits: `feat(proxy): block write methods for read grants`.
- Reference requirement IDs in the body.
- PR description: what changed, why, how it was tested (commands and results).
- Never commit secrets, `.env` files, `.mcpdeck/` data or real API responses.
