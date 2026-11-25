# Releasing

Releases are built by [GoReleaser](https://goreleaser.com) from
`.goreleaser.yaml` when a `v*` tag is pushed (`.github/workflows/release.yml`).
One release produces:

| Channel | What users run | Needs |
|---|---|---|
| GitHub Releases | download the `.zip` (Windows) or `.tar.gz` (macOS, Linux) | nothing extra |
| Homebrew tap (macOS) | `brew install --cask abhijeet-oxide/tap/mcpdeck` | the `homebrew-tap` repo and a token |
| Scoop bucket (Windows) | `scoop bucket add mcpdeck https://github.com/abhijeet-oxide/scoop-bucket` then `scoop install mcpdeck` | the `scoop-bucket` repo and a token |

Every release also has `checksums.txt` and an SBOM per archive.

## One-time setup

Neither Homebrew nor Scoop has to approve anything: a tap and a bucket are
ordinary public GitHub repositories that GoReleaser commits to.

1. Create two **public** repositories under the same owner:
   `homebrew-tap` and `scoop-bucket`. Each needs only a README. The
   `homebrew-` prefix is required by Homebrew; it's what lets users type
   `abhijeet-oxide/tap`.
2. Create a fine-grained personal access token with **Contents: read and
   write** on those two repositories only.
3. In this repository, Settings › Secrets and variables › Actions, add the
   token twice: `HOMEBREW_TAP_TOKEN` and `SCOOP_BUCKET_TOKEN` (or two
   separate tokens, one per repository).

## Cut a release

```sh
git tag v0.1.0
git push origin v0.1.0
```

Tags with a suffix (`v0.2.0-rc.1`) are published as pre-releases, and the tap
and bucket are left alone (`skip_upload: auto`).

To try the whole pipeline without publishing anything:

```sh
goreleaser check
goreleaser release --snapshot --clean
```

## Not set up yet

- **winget.** Publishing means a pull request to `microsoft/winget-pkgs` from
  a fork, which Microsoft validates and reviews before merging. GoReleaser
  can open that PR (`winget:` section). Add it after the first release.
- **homebrew-core.** The main Homebrew repository has its own acceptance
  criteria for new software. The tap works without it.
- **Code signing.** Unsigned binaries work, with these effects:
  - Windows: SmartScreen warns when a downloaded `.exe` is run. Signing
    needs a paid code-signing certificate or Azure Trusted Signing.
  - macOS: Gatekeeper blocks unsigned binaries downloaded from a browser.
    The Homebrew cask removes the quarantine flag on install. Signing and
    notarisation need an Apple Developer Program membership; GoReleaser's
    `notarize:` section can then do it from Linux CI.
