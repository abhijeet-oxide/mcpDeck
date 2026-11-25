---
name: ship
description: Take a finished change in mcpDeck from branch to a verified release - PR, CI watch and fixes, release check. Also for triaging a red CI or release run, a Dependabot PR, or a security alert. Use after the tdd loop is done, or whenever CI, releases or repo health come up.
---

# Ship a change

The maintainer gives requirements; we own everything after that. "Done"
means merged-ready with green CI, not "tests pass on my machine".

## 1. Before the PR

- `make test && make lint`. Also `actionlint` if `.github/` changed, and
  `make release-check` if `.goreleaser.yaml`, `scripts/`, `tools/` or
  `cmd/mcpdeck` build flags changed.
- Pick the commit type deliberately. It's the release decision:
  - `feat`: users can do something new → minor release.
  - `fix` / `perf`: users see a bug fixed or something faster → patch.
  - `docs` `ci` `test` `chore` `build` `refactor` `style`: no release.
  - `!` / `BREAKING CHANGE:` only for a real break of CLI, API, config or
    data. Say what users must do in the footer.
- Write the subject for users: it becomes a release-notes line.
  `fix(proxy): stop dropping query strings on redirects`, not
  `fix: handle edge case`.
- `make next-version` shows what merging would release.

## 2. Open the PR

- Title: a conventional commit (squash merges use it). Body: follow
  `.github/pull_request_template.md`: what, why, requirement IDs, commands
  run and what you saw.
- Subscribe to the PR's activity and end the turn. Don't poll.

## 3. CI is red

Read the failing job's log before changing anything. Common causes:

| Symptom | Cause | Fix |
|---|---|---|
| Run is red with **no jobs** | The workflow file is invalid | `actionlint`; e.g. `hashFiles()` isn't allowed in a job-level `if:` |
| `go (windows-latest)` only | Paths, line endings, file locking, `sh` assumptions | `filepath`, no hard-coded `/`; skip shell-only tests on Windows with a reason |
| Installer test on Windows: "term 'X' is not recognized" | Windows PowerShell 5.1 started from PowerShell 7 can't load script cmdlets (`Get-FileHash`, `Expand-Archive`) | Use .NET in `install.ps1`; the test runs both `powershell` and `pwsh` |
| Passes locally, "permission denied" in CI | The cloud container runs as **root**, CI runners don't (e.g. binding ports < 1024) | Reproduce the condition in a test that fails as root too |
| scorecard: "workflow contains global env vars" | Scorecard won't publish from a workflow with top-level `env:`/`defaults:` | Keep `security.yml` free of them |
| `release-dry-run` / checkdist | An artifact name, size or the version string changed | Fix `.goreleaser.yaml`; ldflags must target `internal/version` |
| govulncheck on the binary | A Go stdlib advisory | CI already uses the latest 1.26 patch; if the fix is in a newer minor, bump go.mod and `GO_VERSION` together |
| dependency-review | A new dependency with a high-severity advisory | Pick another version or package |

This container has no PowerShell and runs as root, so Windows and
permission behaviour is only proven in CI: wait for every OS before calling
a change done, and don't merge-ready a PR whose Windows job hasn't finished.

Never skip, delete or weaken a test to get green. A flaky test is a bug:
fix it with an injected clock, not with retries or sleeps.

## 4. After merge

`release.yml` runs on main: CI → plan → publish → verify. When asked to
watch it, check the run. If `publish` or `verify` fails, follow "If a release
fails" in `docs/releasing.md`. Always fix forward with a `fix:` commit; never
move or reuse a published tag.

## Dependabot PRs

Read the changelog between the versions for breaking changes. Go and npm
bumps are `fix(deps)` and will release; workflow bumps are `ci(deps)` and
won't. If CI is green and nothing breaks, recommend merging; otherwise push
the needed code change to a new branch and say why.

## Report

Say what shipped (or will on merge), which version, what you ran, and
anything you skipped or couldn't verify.
