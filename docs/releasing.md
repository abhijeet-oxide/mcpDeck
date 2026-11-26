# Releasing

Nobody cuts releases by hand. Every push to `main` runs
[`release.yml`](../.github/workflows/release.yml) (NFR-8):

```
push to main
  └─ ci        the full CI suite (ci.yml): Go on Linux/macOS/Windows, web,
  │            skills, installers, govulncheck, actionlint, release dry run
  └─ plan      tools/nextversion: next version from conventional commits
  └─ publish   tag, GoReleaser, provenance attestation   (only if plan says so)
  └─ verify    install the published release on Linux, macOS and Windows
               with install.sh / install.ps1 and check `mcpdeck version`
```

## Which commits release

| Commits since the last tag | Next version (from v0.4.2) |
|---|---|
| only `docs:` `ci:` `test:` `chore:` `build:` `refactor:` `style:` | no release |
| at least one `fix:` or `perf:` | v0.4.3 |
| at least one `feat:` | v0.5.0 |
| a breaking change (`feat!:` or a `BREAKING CHANGE:` footer) | v0.5.0 while 0.x, then a major bump |

So the commit type is a release decision. A change users can see is `feat`
or `fix`; a change they can't is one of the others. Dependabot's Go and web
bumps are `fix(deps)` (they end up in the binary) and its workflow bumps are
`ci(deps)`. With squash merges the PR title becomes the commit, so PR titles
must be conventional too.

The release notes are the `feat`, `fix` and `perf` commit subjects, grouped,
under install instructions for that exact version. Write subjects for users.

## What a release contains

| File | For |
|---|---|
| `install.ps1`, `install.sh` | one-line installs (verify checksums, user install, PATH) |
| `mcpdeck_<os>_<arch>.zip` / `.tar.gz` | archives with LICENSE and README |
| `mcpdeck_<os>_<arch>[.exe]` | the bare binary, download and run |
| `checksums.txt` | SHA-256 of every file above |
| `*.sbom.json` | SBOM per archive |
| provenance attestation | `gh attestation verify <file> --repo abhijeet-oxide/mcpDeck` |

Names have no version so `releases/latest/download/<name>` is stable. Pin a
version with `releases/download/v0.2.0/<name>` or `MCPDECK_VERSION=v0.2.0`.

## By hand

- **Force a version or a pre-release:** Actions › release › Run workflow,
  version `v0.3.0-rc.1`. Suffixed versions are published as pre-releases and
  don't move `latest`.
- **Check the pipeline locally** (needs `goreleaser`):
  `make release-check` builds every target, checks names, checksums, sizes
  and the version string with `tools/checkdist`, the same as CI.
- **See what would ship:** `make next-version`.

## If a release fails

- `ci` red: nothing was tagged. Fix on a branch, merge, it runs again.
- A job cancelled with no steps (GitHub never gave it a runner, seen on
  macOS): `retry.yml` re-runs it automatically, up to twice per run, and the
  release carries on. It never retries a job that ran and failed. If it's
  still stuck after that, re-run the failed jobs by hand once Actions is
  healthy.
- `publish` red after the tag was pushed: delete the tag and the draft or
  partial release (`git push --delete origin vX.Y.Z`), fix, and either merge
  the fix (the next version is recomputed) or run the workflow by hand with
  the same version.
- `verify` red: the release is public but doesn't install. Fix forward with a
  `fix:` commit; the next patch release supersedes it. Mark the broken one as
  a pre-release so `latest` moves back.

## Not set up yet

- **Homebrew and Scoop.** No tap or bucket repository exists, so the release
  doesn't publish to them and the README doesn't offer them. To add one:
  create the public repository, add a fine-grained token with Contents
  read/write on it as a secret, add GoReleaser's `homebrew_casks:` or
  `scoops:` section, and add an install check for it to `verify` in
  `release.yml`. Only then list it in the README.
- **Docker image.** Not built. mcpDeck keeps secrets in the OS keychain and
  opens a browser, which a container doesn't have.
- **winget.** A pull request to `microsoft/winget-pkgs` per release, which
  Microsoft validates. GoReleaser's `winget:` section can open it from a fork
  with a token. Worth adding once releases settle; until then `install.ps1`
  covers Windows without admin rights.
- **Code signing.** Unsigned binaries work, with these effects:
  - Windows: SmartScreen warns when a downloaded `.exe` is run. Signing
    needs a code-signing certificate or Azure Trusted Signing.
  - macOS: Gatekeeper blocks unsigned binaries downloaded from a browser.
    `install.sh` and `curl` downloads aren't quarantined. Signing and notarisation need an Apple Developer
    Program membership; GoReleaser's `notarize:` section can do it from CI.
