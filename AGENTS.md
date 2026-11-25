# AGENTS.md

Instructions for any coding agent working in this repo (Claude Code, Copilot,
Cursor, Codex). People: this is a good onboarding read too.

## What we're building

mcpDeck is a single-binary layer between AI agents and APIs, including legacy
ones. People import APIs the way they would in Postman, store credentials
locally, and write versioned tools in TypeScript that call one or more APIs.
Each toolkit is its own MCP endpoint that people add to Claude Code, VS Code
or Cursor in one click. Agents use the tools they're given and can build new
ones inside those limits. Every call is recorded with time and tokens, and
rule-based suggestions show how to make tools smaller and faster.

**Phase 1 (now): dev.** One binary on a laptop, Windows first-class, SQLite,
everything local. Phase 2 (prod: Postgres, SSO, external vaults, visual
builder) is not scheduled; don't build it unless asked.

Read before non-trivial work:

- [docs/vision.md](docs/vision.md): problem, mental model, principles
- [docs/requirements.md](docs/requirements.md): what to build, with IDs and milestones
- [docs/design.md](docs/design.md): architecture, stack, data model, UI tokens
- [docs/testing.md](docs/testing.md): TDD loop, test layers, mock servers

**Status:** design phase. Folders exist; code arrives milestone by milestone
(Phase 1, M1 walking skeleton first). Don't build ahead of the current milestone unless asked.

## Repo map

```
cmd/mcpdeck/      CLI entry and wiring
internal/        Go packages, one per feature (see docs/design.md#packages)
web/             React + Vite UI, embedded into the binary
skills/          optional files `mcpdeck connect` installs into Claude / Copilot / Cursor
examples/        demo world and sample imports
e2e/             Playwright + MCP client tests against a running binary
docs/            vision, requirements, design, testing, releasing, mockups
scripts/         install.sh / install.ps1, shipped with every release
tools/           release tooling: nextversion, checkdist (not in the binary)
site/            the GitHub Pages site (abhijeet-oxide.github.io/mcpDeck)
.github/         CI, release, security and Pages workflows
```

## Commands

```sh
make dev         # Go server + Vite dev server with hot reload
make test        # all unit + integration tests (Go with -race, Vitest)
make lint        # golangci-lint + Biome
make e2e         # Playwright + MCP suite against `mcpdeck dev`
make dev-up      # start mcpdeck with mock apps in the background (127.0.0.1:47800)
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

## Operating the repo

Agents own this repo end to end: code, CI, releases, security and docs. The
maintainer sets requirements; everything after that is ours, including
noticing what's broken and fixing it without being asked.

- **Releases are automatic** (NFR-8, [docs/releasing.md](docs/releasing.md)).
  Merging to `main` runs CI, then tags and publishes if any commit since the
  last tag is `feat`, `fix`, `perf` or breaking. **The commit type is the
  release decision**: use `feat`/`fix` only for changes users can see, and
  make PR titles conventional too (squash merges use them).
- **CI is the gate, and it must be green before you say done.** After pushing
  a PR, subscribe to its activity and fix failures yourself. A red check
  you didn't cause still gets fixed or reported, never ignored.
- **Workflow changes:** run `actionlint` (CI does too). Actions are pinned
  to a commit SHA with the version in a comment (`@<sha> # v7.0.1`);
  Dependabot updates both. Workflows use the
  latest Go patch (`GO_VERSION: 1.26.x`) because patches carry stdlib security
  fixes; bump the minor in go.mod and the workflows together.
- **Security:** CodeQL, govulncheck (source and the shipped binary),
  dependency review and Scorecard run in CI. A new advisory is a `fix:` PR,
  which ships a patch release on merge.
- **Dependabot PRs:** check the changelog for breaking changes, let CI run,
  and recommend merging or say what blocks it.
- Use the `ship` skill for the PR → CI → release loop.

## Invariants (never break these)

These protect users' systems. Each has a test; don't weaken it.

- **Credentials never reach an agent.** Secrets are resolved only inside
  `internal/proxy`. They must not appear in MCP responses, API responses,
  logs, recorded calls, history, examples or errors, and tool or script code
  can't read them. (CRED-4, CRED-7)
- **All outbound HTTP goes through `internal/proxy`.** No other package
  creates an `http.Client` for upstream calls. That's where allow-lists,
  grants, recording and token counting live.
- **Read grants can't write.** Write requests are blocked at the proxy,
  regardless of what a tool says. (ACC-2)
- **Connection tokens can't escalate:** no changing access, approving, making
  write tools live, or reading secrets. (ACC-6)
- **The local server is local.** `127.0.0.1` only; `Host` and `Origin` are
  checked; the UI needs its launch cookie. (RUN-5)
- **Tool versions are immutable.** Saving creates a new version. (VER-1)
- **The JS sandbox has no I/O.** The API clients and `call()` are
  the only way out. (TOOL-4)
- **The audit log is append-only.** (AUD-1)

If a task seems to need breaking one of these, stop and ask.

## Go conventions

- Windows is first-class: use `filepath`, never hard-code `/` or `~`; OS
  specifics (paths, keychain, browser open, client config locations) go
  behind an interface with a fake.

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
- Reuse familiar patterns: see the table in `docs/design.md#familiar-patterns`
  (Postman, Bruno, VS Code, DevTools). Don't invent a new interaction when one
  exists.
- Nothing fake: every control works; no "coming soon", no lorem ipsum. (UI-4)
- Styling with Tailwind and the tokens in `web/src/styles/tokens.css`. Match
  the layouts in `docs/mockups/` (spec in `docs/ui.md`) with the calmer look in
  `docs/design.md#look`: solid panels with hairline borders, neutral
  background, compact sizes, black and blue pill buttons, coloured mono chips
  for HTTP methods. No glass, glows, purple, emoji or gradients on content.
- Accessible by default: real `<button>`/`<a>`, labels on inputs, visible focus,
  AA contrast.
- Icons are Lucide (`lucide-react`). Don't mix in other icon sets.
- Navigation stays at Home, APIs, Toolkits, Agents, Activity, then About and
  Settings at the bottom. Ask
  before adding a top-level item.

## Commits and PRs

- Conventional commits: `feat(proxy): block write methods for read grants`.
  The type decides the release (see Operating the repo).
- Reference requirement IDs in the body.
- PR description: what changed, why, how it was tested (commands and results).
- Never commit secrets, `.env` files, `.mcpdeck/` data or real API responses.
- Branches: before starting new work, check whether your branch's PR has
  merged. If it has, restart the branch from the latest `origin/main`, then
  push and open a **new** PR. Never add commits to a merged PR's branch
  without rebasing first.
- README: purpose, features, install, roadmap. Every claim must be something
  the requirements make achievable. No milestones, internal status or
  invented numbers.
