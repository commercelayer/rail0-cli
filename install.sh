#!/bin/sh
# Install the rail0 CLI from a GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/commercelayer/rail0-cli/main/install.sh | sh
#
# Picks the archive for this OS and CPU (macOS or Linux, amd64 or arm64), verifies it
# against the release's checksums.txt, and installs `rail0` plus the short `r0` alias into
# ~/.local/bin — a directory the user owns, so neither this script nor a later
# `rail0 update` needs sudo. Nothing is installed unless the checksum matches.
#
# Environment (all optional):
#   RAIL0_VERSION       a release to install, e.g. 1.2.0 (default: the latest release)
#   RAIL0_INSTALL_DIR   where to put the binary (default: $HOME/.local/bin)
#   RAIL0_RELEASES_URL  the releases base URL (default: the GitHub repository's); for a
#                       mirror, or for testing against a local copy of a release
#
# While the repository is private, its releases (and this script's raw URL) answer 404
# to an anonymous curl. With the GitHub CLI signed in to an account that can read the
# repository, the script falls back to `gh release download`, and runs as
#
#   gh api repos/commercelayer/rail0-cli/contents/install.sh -H 'Accept: application/vnd.github.raw' | sh
#
# POSIX sh on purpose: it runs piped into `sh` on any macOS or Linux, including minimal
# containers without bash. Windows: download the .zip from the releases page instead.
set -eu

REPO=commercelayer/rail0-cli
RELEASES_URL="${RAIL0_RELEASES_URL:-https://github.com/$REPO/releases}"
INSTALL_DIR="${RAIL0_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() {
  printf 'rail0 install: %s\n' "$*" >&2
  exit 1
}
need() { command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but was not found"; }

need curl
need tar
need uname

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "unsupported OS '$(uname -s)'. On Windows, download the .zip from $RELEASES_URL" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported CPU '$(uname -m)' (builds exist for amd64 and arm64)" ;;
esac

# A SHA-256 tool: Linux ships sha256sum, macOS ships shasum.
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
  fail "neither sha256sum nor shasum was found; cannot verify the download"
fi

# Anonymous by default; through the GitHub CLI when the releases are not public (a
# private repository answers 404) and gh is signed in. Only for the default releases
# URL: a mirror is whatever RAIL0_RELEASES_URL says, and gh knows nothing of it.
use_gh=0
if [ -z "${RAIL0_RELEASES_URL:-}" ] && ! curl -fsSLI -o /dev/null "$RELEASES_URL/latest" 2>/dev/null &&
  command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
  use_gh=1
  say "The releases are not public; downloading through the GitHub CLI (gh)."
fi

# The latest version, read from where GitHub redirects /releases/latest (…/tag/vX.Y.Z)
# rather than from the API, which rate-limits anonymous callers — or from gh.
version="${RAIL0_VERSION:-}"
if [ -z "$version" ] && [ "$use_gh" = 1 ]; then
  version=$(gh release view -R "$REPO" --json tagName -q .tagName) ||
    fail "gh could not read the latest release of $REPO (does your account have access?)"
elif [ -z "$version" ]; then
  latest_url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$RELEASES_URL/latest") ||
    fail "could not reach $RELEASES_URL (is the repository public, and are you online? A private one needs 'gh auth login')"
  version="${latest_url##*/}"
fi
case "$version" in
  v[0-9]* | [0-9]*) ;;
  *) fail "no published release found at $RELEASES_URL" ;;
esac
version="${version#v}"

archive="rail0_${version}_${os}_${arch}.tar.gz"
base="$RELEASES_URL/download/v$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "Downloading rail0 $version ($os/$arch)…"
if [ "$use_gh" = 1 ]; then
  gh release download "v$version" -R "$REPO" -D "$tmp" -p "$archive" -p checksums.txt ||
    fail "gh could not download $archive and checksums.txt for v$version"
else
  curl -fsSL -o "$tmp/$archive" "$base/$archive" ||
    fail "download failed: $base/$archive (is $version a release with a $os/$arch build?)"
  curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" ||
    fail "could not download checksums.txt for $version; refusing to install unverified"
fi

want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || fail "checksums.txt lists no checksum for $archive; refusing to install unverified"
got=$(sha256 "$tmp/$archive")
[ "$got" = "$want" ] || fail "checksum mismatch for $archive (expected $want, got $got); nothing was installed"

tar -xzf "$tmp/$archive" -C "$tmp" rail0 || fail "the archive contains no rail0 binary"

mkdir -p "$INSTALL_DIR"
# Into place with a rename from the same directory, so an interrupted install never
# leaves a half-written binary at the path.
cp "$tmp/rail0" "$INSTALL_DIR/.rail0.new"
chmod 755 "$INSTALL_DIR/.rail0.new"
mv -f "$INSTALL_DIR/.rail0.new" "$INSTALL_DIR/rail0"
ln -sf rail0 "$INSTALL_DIR/r0"

say "Installed rail0 $version to $INSTALL_DIR/rail0 (alias: r0)"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    say ""
    say "$INSTALL_DIR is not on your PATH. Add it, e.g. for zsh:"
    say "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.zshrc && exec zsh"
    ;;
esac

say ""
say "Next: rail0 init    (guided setup: gateway, key, sign-in)"
say "Shell completions: rail0 completion --help"
