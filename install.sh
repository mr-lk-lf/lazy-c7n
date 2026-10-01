#!/bin/sh
# Install lazyc7n from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/vstrofago/lazy-c7n/main/install.sh | sh
#
# Options (environment variables):
#   LAZYC7N_VERSION       version to install, e.g. 0.1.0 (default: latest release)
#   LAZYC7N_INSTALL_DIR   where to put the binary (default: ~/.local/bin)
#   LAZYC7N_BASE_URL      download from here instead of the GitHub release
#                         (mirrors, testing); needs LAZYC7N_VERSION
#
# The download is checked against the release's checksums.txt before
# anything is installed. Linux and macOS (amd64, arm64). On Windows use the
# .zip from the releases page.
set -eu

REPO="vstrofago/lazy-c7n"
BIN="lazyc7n"
INSTALL_DIR="${LAZYC7N_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'lazyc7n install: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "needs '$1'"; }

need curl
need tar
need uname

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported OS $(uname -s); download a release from https://github.com/$REPO/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m)" ;;
esac

version="${LAZYC7N_VERSION:-}"
if [ -z "$version" ]; then
  # The latest release URL redirects to .../releases/tag/vX.Y.Z
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
    fail "cannot reach GitHub"
  version="${url##*/v}"
  [ -n "$version" ] && [ "$version" != "$url" ] || fail "no release found"
fi
version="${version#v}"

archive="${BIN}_${version}_${os}_${arch}.tar.gz"
base="${LAZYC7N_BASE_URL:-https://github.com/$REPO/releases/download/v$version}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "downloading $BIN $version ($os/$arch)..."
curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "download failed: $base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "checksums.txt not found"

expected=$(grep "  $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$expected" ] || fail "$archive is not in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
else
  need shasum
  actual=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
fi
[ "$actual" = "$expected" ] || fail "checksum mismatch for $archive (expected $expected, got $actual)"

tar -xzf "$tmp/$archive" -C "$tmp" "$BIN"
mkdir -p "$INSTALL_DIR"
mv "$tmp/$BIN" "$INSTALL_DIR/$BIN"
chmod 755 "$INSTALL_DIR/$BIN"

say "installed $INSTALL_DIR/$BIN"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) say "note: $INSTALL_DIR is not in your PATH; add it to your shell profile" ;;
esac
say "lazyc7n needs the Cloud Custodian CLI: pip install c7n (or set runner.kind = \"docker\")"
"$INSTALL_DIR/$BIN" -version || true
