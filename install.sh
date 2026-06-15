#!/bin/sh
# agentmod installer — downloads a release binary from GitHub and installs it.
#
#   curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh
#
# Options (env vars, or a version as the first argument):
#   AGENTMOD_VERSION      version or tag to install (default: latest release)
#   AGENTMOD_INSTALL_DIR  install directory (default: /usr/local/bin if
#                         writable, otherwise ~/.local/bin)
#
# Pure POSIX sh. Needs: curl or wget, tar, and sha256sum or shasum.
set -eu

REPO="mojomoth/agentmod"
BIN="agentmod"

err() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*" >&2; }
have() { command -v "$1" >/dev/null 2>&1; }

download() { # url dest
  if have curl; then curl -fsSL "$1" -o "$2"
  elif have wget; then wget -qO "$2" "$1"
  else err "need curl or wget"; fi
}
fetch() { # url -> stdout
  if have curl; then curl -fsSL "$1"
  elif have wget; then wget -qO- "$1"
  else err "need curl or wget"; fi
}

# --- detect platform ---
os=$(uname -s)
case "$os" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) err "unsupported OS '$os' — try 'go install github.com/$REPO@latest'" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) err "unsupported architecture '$arch'" ;;
esac

# --- resolve version/tag ---
ver="${AGENTMOD_VERSION:-${1:-}}"
if [ -z "$ver" ]; then
  info "Resolving latest release of $REPO..."
  ver=$(fetch "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 \
    | sed -E 's/.*"tag_name"[ ]*:[ ]*"([^"]+)".*/\1/')
  [ -n "$ver" ] || err "could not determine latest version; set AGENTMOD_VERSION"
fi
# Release tag carries a leading 'v'; archive filename uses the bare number.
case "$ver" in v*) tag="$ver" ;; *) tag="v$ver" ;; esac
num="${tag#v}"

archive="${BIN}_${num}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading $archive ($tag)..."
download "$base/$archive" "$tmp/$archive" || err "download failed: $base/$archive"
download "$base/checksums.txt" "$tmp/checksums.txt" \
  || info "warning: checksums.txt unavailable; skipping verification"

# --- verify checksum ---
if [ -f "$tmp/checksums.txt" ]; then
  want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt" | head -1)
  if [ -n "$want" ]; then
    if have sha256sum; then got=$(sha256sum "$tmp/$archive" | awk '{print $1}')
    elif have shasum; then got=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
    else got=""; info "warning: no sha256 tool found; skipping verification"; fi
    if [ -n "$got" ]; then
      [ "$got" = "$want" ] || err "checksum mismatch for $archive"
      info "Checksum verified."
    fi
  else
    info "warning: $archive not listed in checksums.txt; skipping verification"
  fi
fi

# --- extract ---
tar -xzf "$tmp/$archive" -C "$tmp" "$BIN" || err "failed to extract $BIN from archive"
chmod +x "$tmp/$BIN"

# --- choose install dir ---
dir="${AGENTMOD_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then dir=/usr/local/bin
  else dir="$HOME/.local/bin"; fi
fi
mkdir -p "$dir" || err "cannot create $dir"
mv "$tmp/$BIN" "$dir/$BIN" \
  || err "cannot write to $dir — set AGENTMOD_INSTALL_DIR or re-run with sudo"

info "Installed $BIN to $dir/$BIN"
case ":$PATH:" in
  *":$dir:"*) ;;
  *)
    info ""
    info "Note: $dir is not on your PATH. Add it, e.g.:"
    info "  export PATH=\"$dir:\$PATH\""
    ;;
esac

"$dir/$BIN" version || true
