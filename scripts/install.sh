#!/bin/sh
# Install mcpdeck on macOS or Linux (NFR-7).
#
#   curl -fsSL https://github.com/abhijeet-oxide/mcpDeck/releases/latest/download/install.sh | sh
#
# Environment:
#   MCPDECK_VERSION      a tag such as v0.2.0 (default: the latest release)
#   MCPDECK_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#   MCPDECK_DOWNLOAD_URL base URL of the release files (for mirrors and tests)
#
# Downloads the archive for this OS and CPU, checks it against checksums.txt,
# and installs one file. No root needed; nothing else on the system changes.
set -eu

repo="abhijeet-oxide/mcpDeck"
version="${MCPDECK_VERSION:-latest}"
dir="${MCPDECK_INSTALL_DIR:-$HOME/.local/bin}"

fail() { printf 'mcpdeck install: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  MINGW* | MSYS* | CYGWIN*) fail "on Windows, run in PowerShell: irm https://github.com/$repo/releases/latest/download/install.ps1 | iex" ;;
  *) fail "no build for $(uname -s); download one from https://github.com/$repo/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "no build for CPU $(uname -m); download one from https://github.com/$repo/releases" ;;
esac

if [ -n "${MCPDECK_DOWNLOAD_URL:-}" ]; then
  base="$MCPDECK_DOWNLOAD_URL"
elif [ "$version" = latest ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  case "$version" in v*) ;; *) version="v$version" ;; esac
  base="https://github.com/$repo/releases/download/$version"
fi

if command -v curl >/dev/null 2>&1; then
  get() { curl -fsSL --retry 3 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  get() { wget -q -O "$2" "$1"; }
else
  fail "needs curl or wget"
fi

if command -v sha256sum >/dev/null 2>&1; then
  sha() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
  fail "needs sha256sum or shasum to verify the download"
fi

archive="mcpdeck_${os}_${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

printf 'Downloading %s\n' "$base/$archive"
get "$base/$archive" "$tmp/$archive" || fail "download failed: $base/$archive"
get "$base/checksums.txt" "$tmp/checksums.txt" || fail "download failed: $base/checksums.txt"

want="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "$archive is not listed in checksums.txt"
got="$(sha "$tmp/$archive")"
[ "$want" = "$got" ] || fail "checksum mismatch for $archive (want $want, got $got); nothing was installed"

tar -xzf "$tmp/$archive" -C "$tmp" mcpdeck || fail "could not unpack $archive"
mkdir -p "$dir"
# Write next to the target, then rename, so a running mcpdeck is never half-written.
cp "$tmp/mcpdeck" "$dir/.mcpdeck.new"
chmod 755 "$dir/.mcpdeck.new"
mv -f "$dir/.mcpdeck.new" "$dir/mcpdeck"

printf 'Installed %s\n' "$("$dir/mcpdeck" version 2>/dev/null || echo "$dir/mcpdeck")"
case ":$PATH:" in
  *":$dir:"*) printf 'Run: mcpdeck\n' ;;
  *)
    printf '\n%s is not on your PATH. Add it, for example:\n' "$dir"
    printf '  echo '\''export PATH="%s:$PATH"'\'' >> ~/.profile\n' "$dir"
    printf 'or run it directly: %s/mcpdeck\n' "$dir"
    ;;
esac
