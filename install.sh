#!/bin/sh
# flowcmd installer for Linux and macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/flowcmd/cli/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/flowcmd/cli/main/install.sh | FLOWCMD_VERSION=v0.1.0 sh
#
# Environment:
#   FLOWCMD_VERSION      — pin a specific version (e.g. v0.1.0); default: latest release
#   FLOWCMD_INSTALL_DIR  — install destination; default: /usr/local/bin if writable, else $HOME/.local/bin

set -eu

OWNER="flowcmd"
REPO="cli"
BIN="flowcmd"

# --- helpers ----------------------------------------------------------------

err() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }

require() {
  command -v "$1" >/dev/null 2>&1 || err "required tool not found: $1"
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    err "neither sha256sum nor shasum found; cannot verify download"
  fi
}

# --- detect platform --------------------------------------------------------

uname_s=$(uname -s)
case "$uname_s" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) err "unsupported OS: $uname_s (install.sh supports Linux and macOS; Windows users run install.ps1)" ;;
esac

uname_m=$(uname -m)
case "$uname_m" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) err "unsupported arch: $uname_m (supported: amd64, arm64)" ;;
esac

# --- resolve version --------------------------------------------------------

require curl
require tar

VERSION="${FLOWCMD_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "resolving latest release..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" \
    | grep -E '"tag_name"[[:space:]]*:' \
    | head -n1 \
    | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
  [ -n "$VERSION" ] || err "could not resolve latest release tag from GitHub API"
fi

# Strip leading v for artifact name (goreleaser uses bare semver in {{ .Version }})
VERSION_BARE=$(printf '%s' "$VERSION" | sed -E 's/^v//')

ARCHIVE="${BIN}_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}"

# --- pick install dir -------------------------------------------------------

if [ -n "${FLOWCMD_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$FLOWCMD_INSTALL_DIR"
elif [ -w /usr/local/bin ]; then
  INSTALL_DIR=/usr/local/bin
else
  INSTALL_DIR="$HOME/.local/bin"
fi
mkdir -p "$INSTALL_DIR" || err "cannot create $INSTALL_DIR"

# --- download + verify ------------------------------------------------------

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t flowcmd)
trap 'rm -rf "$TMP"' EXIT

info "downloading $ARCHIVE..."
curl -fsSL "$BASE_URL/$ARCHIVE" -o "$TMP/$ARCHIVE" \
  || err "failed to download $BASE_URL/$ARCHIVE"

info "verifying checksum..."
curl -fsSL "$BASE_URL/checksums.txt" -o "$TMP/checksums.txt" \
  || err "failed to download checksums.txt"

expected=$(grep "  $ARCHIVE$" "$TMP/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || err "no checksum listed for $ARCHIVE"

actual=$(sha256 "$TMP/$ARCHIVE")
if [ "$expected" != "$actual" ]; then
  err "checksum mismatch for $ARCHIVE
  expected: $expected
  actual:   $actual"
fi

# --- extract + install ------------------------------------------------------

tar -xzf "$TMP/$ARCHIVE" -C "$TMP" \
  || err "failed to extract $ARCHIVE"

[ -f "$TMP/$BIN" ] || err "binary $BIN not found in archive"

DEST="$INSTALL_DIR/$BIN"
if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "$TMP/$BIN" "$DEST" \
    || err "failed to install to $DEST"
else
  info "need elevated permissions to write to $INSTALL_DIR..."
  sudo install -m 0755 "$TMP/$BIN" "$DEST" \
    || err "failed to install to $DEST"
fi

info "✓ installed $BIN to $DEST"

# --- PATH hint --------------------------------------------------------------

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    info ""
    info "note: $INSTALL_DIR is not on your PATH. Add it:"
    info "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

# --- sanity check -----------------------------------------------------------

info ""
"$DEST" --version || err "installed binary failed to run"
