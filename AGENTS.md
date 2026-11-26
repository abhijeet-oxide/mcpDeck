# AGENTS.md

The rulebook for any coding agent maintaining this repo (Claude Code, Copilot,
Cursor, Codex). People: it is a good onboarding read too. Short version in
[llms.txt](llms.txt).

## What mcpDeck is

One Go binary with an embedded React UI. People and agents import APIs
(OpenAPI, Swagger 2.0, Postman, curl), store credentials locally (encrypted),
set auth once per API (login chains, OAuth 2.0, HMAC, mTLS), and write small
versioned TypeScript tools that chain calls and trim the answer. Each toolkit
is an MCP server at `/mcp/<toolkit>`; builder agents get
`/mcp/<toolkit>/build`; `mcpdeck mcp` is an admin MCP server on stdio. Every
call goes through one proxy that injects credentials, enforces read/write
access and records the call.

**Status:** Phase 1 (local, SQLite, single user) is built and released as
`0.x`. Phase 2 (team server, Postgres, SSO, external vaults) is not
scheduled; don't build it unless asked. In the UI an API is called an
*application*; code, CLI and docs say API.

## Repo map

```
cmd/mcpdeck/      CLI (cobra), admin MCP server (admin.go), serve and dev
internal/         one package per concern: proxy, tool (goja sandbox), mcp, deck
                  (use cases), store (SQLite), lint, importer, vault, audit, server
web/              React + Vite UI, embedded into the binary from web/dist
e2e/scenarios     real-world scenario suite (real binary, real MCP client)
e2e/mockenv       mock JFrog, Anchore, Jira, GitHub, OAuth, legacy, HMAC, mTLS servers
bench/            committed raw output of the token benchmark
skills/           agent skills `mcpdeck connect` can install
examples/         finished example toolkits
docs/             requirements, design, ui, agents, lint rules, claims, testing, releasing
scripts/          install.sh / install.ps1, shipped with every release
tools/            repo tooling: nextversion, checkdist, claimscheck, lintdoc,
                  scenarioreport, tokcount (not in the binary)
site/             the GitHub Pages site
```

## Commands

```sh
make test        # Go (-race, includes scenarios), Vitest, skill scripts
make lint        # golangci-lint + Biome
make scenarios   # scenario suite with a pass rate by category (about 2 minutes)
make bench       # rerun the token benchmark, rewrite bench/results.json
make claims      # every README claim names an existing test
make lint-docs   # regenerate docs/lint-rules.md from internal/lint
make dev         # mcpdeck dev (token mcpdeck-dev) + Vite hot reload on :5173
make build       # ./bin/mcpdeck with the UI embedded
```

Run `make test`, `make lint` and `make claims` before you say a change is
done, and report what you ran.

## Rules

1. **Start from a requirement.** Find its ID in
   [docs/requirements.md](docs/requirements.md). If the change isn't covered,
   add or edit the requirement first.
2. **Test first.** A failing test tagged `// REQ: <ID>`, then the fix, in one
   commit. Every bug fix starts with a test that reproduces it.
3. **Every user-visible behaviour gets a scenario** in `e2e/scenarios`
   (template: `e2e/scenarios/template_test.go.txt`, guide:
   [docs/testing.md](docs/testing.md)). If a mock is missing, build it to
   behave like the real system: same paths, status codes, error bodies and
   pagination.
4. **The README only says what a test proves.** Each claim carries
   `<!-- Cn -->` and a row in [docs/claims.md](docs/claims.md) naming the
   tests; `make claims` fails otherwise. When a feature lands, add the test,
   then the claims row, then the sentence. When one is removed, remove its
   claim in the same change. No comparisons with other products, no
   numbers without committed raw output (`make bench` writes
   `bench/results.json`), no "coming soon".
5. **Lint rules cite a source.** New rules go in `internal/lint/rules.go`
   with a severity, a fix, a source URL, and a hit-and-miss test; then
   `make lint-docs`. Block only spec violations, credential leaks and
   wrong write annotations; everything else is warn or hint.
6. **Parity.** A new builder action ships on all three surfaces: UI, CLI
   (with `--json`) and the admin MCP server, and gets a row in
   [docs/agents.md](docs/agents.md). Person-only decisions (approving write
   tools, widening access, reading secrets) stay out of agent surfaces.
7. **Agent context stays small.** Scenarios bound the use endpoint (1,000
   tokens) and the admin MCP server (2,500). Measure files with
   `go run ./tools/tokcount <files>`.
8. **UI uses established patterns** listed in [docs/ui.md](docs/ui.md); don't
   invent a widget when one exists. Every control works (UI-4). UI changes
   come with screenshots at 1440x900 and 1280x720.
9. **Keep diffs focused.** One behaviour per commit; don't reformat
   unrelated code.
10. **Say what you didn't do.** Skipped tests, things you couldn't run, open
    TODOs: say so plainly.

## Releases

Automatic ([docs/releasing.md](docs/releasing.md), NFR-8). Merging to `main`
runs the full CI suite, scenarios included, then tags and publishes with
GoReleaser (checksums, SBOM, provenance) if any commit since the last tag is
`feat`, `fix`, `perf` or breaking. **The commit type is the release
decision**, and PR titles must be conventional (the `pr-title` job checks).
The release notes are the commit subjects, so write them for users. Add a
line to [CHANGELOG.md](CHANGELOG.md) under Unreleased for anything users see.
Homebrew, Scoop and container images are added only with a release check
that installs from them.

## Operating the repo

- **CI is the gate.** After pushing, watch the checks and fix failures,
  including ones you didn't cause, or report them.
- **Workflows:** run `actionlint`. Actions are pinned to a commit SHA with
  the version in a comment. Workflows use the latest Go patch
  (`GO_VERSION: 1.26.x`).
- **Security:** CodeQL, govulncheck (source and the shipped binary),
  dependency review and Scorecard run in CI. A new advisory is a `fix:` PR.
- **Dependabot PRs:** read the changelog, let CI run, recommend merging or
  say what blocks it.
- The `ship` skill in `.claude/skills/` covers PR, CI and release.

## Invariants (never break these)

Each has a test; don't weaken it. If a task seems to need breaking one, stop
and ask.

- **Credentials never reach an agent.** Secrets are resolved only inside
  `internal/proxy`; never in MCP or API responses, logs, recorded calls,
  examples or errors; tool code can't read them. (CRED-4, CRED-7)
- **All outbound HTTP goes through `internal/proxy`.** No other package makes
  upstream calls.
- **Read grants can't write**, whatever a tool says. (ACC-2)
- **Agent tokens can't escalate:** no changing access, approving, making
  write tools live, or reading secrets. (ACC-6, PAR-2)
- **The local server is local.** `127.0.0.1` only; `Host` and `Origin`
  checked; the UI needs its launch cookie. (RUN-5)
- **Tool versions are immutable.** Saving creates a new version. (VER-1)
- **The JS sandbox has no I/O** beyond the API clients and `call()`. (TOOL-4)
- **The audit log is append-only.** (AUD-1)

## Go conventions

- Windows is first-class: `filepath`, never hard-coded `/` or `~`; OS
  specifics behind an interface with a fake.
- Stdlib first. A new dependency needs a one-line reason in the PR.
- `context.Context` first on anything that does I/O. Constructors take
  dependencies explicitly.
- Errors wrap with `%w`. Agent-facing errors say what to do next (MCP-4).
- Tests: table-driven, `t.TempDir()` for SQLite, the injected clock
  (`internal/clock`) instead of sleeps where the code allows.

## Web conventions

- TypeScript strict; function components; server state through TanStack
  Query and the small client in `web/src/lib/api.ts`.
- Tailwind with the tokens in `web/src/styles/tokens.css`; Lucide icons only.
  Calm look: solid panels, hairline borders, primary buttons on the right,
  coloured mono chips for HTTP methods. No glass, glows, purple, emoji or
  gradients on content.
- Accessible by default: real buttons and links, labels, visible focus, AA
  contrast.
- Navigation stays Home, Applications, Toolkits, Agents, Activity, then About
  and Settings. Ask before adding a top-level item.

## Commits and PRs

- Conventional commits: `feat(proxy): block write methods for read grants`;
  requirement IDs in the body.
- PR description follows `.github/pull_request_template.md`.
- Never commit secrets, `.env` files, `.mcpdeck/` data or real API responses.
- Before new work, check whether your branch's PR merged; if so, restart from
  the latest `origin/main` and open a new PR. Rebase, don't merge `main` in.
