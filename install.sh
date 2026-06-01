#!/bin/sh
# vcf2tf installer.
#   curl -fsSL https://raw.githubusercontent.com/warroyo/VCF-to-TF/main/install.sh | sh
#
# Env overrides:
#   VERSION   tag to install (default: latest release, e.g. v0.1.0)
#   BIN_DIR   install location (default: /usr/local/bin, falls back to ~/.local/bin)
set -eu

REPO="warroyo/VCF-to-TF"
BINARY="vcf2tf"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"

err() { echo "install: $*" >&2; exit 1; }

# --- detect platform -------------------------------------------------------
os=$(uname -s)
case "$os" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *)      err "unsupported OS: $os (use 'go install' instead)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *)             err "unsupported architecture: $arch" ;;
esac

# --- resolve version -------------------------------------------------------
VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -n1 | cut -d '"' -f4)
  [ -n "$VERSION" ] || err "could not determine latest release"
fi
num="${VERSION#v}" # GoReleaser archive names drop the leading 'v'

# --- download & extract ----------------------------------------------------
file="${BINARY}_${num}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${file}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading ${BINARY} ${VERSION} (${os}/${arch})..."
curl -fsSL "$url" -o "$tmp/$file" || err "download failed: $url"
tar -xzf "$tmp/$file" -C "$tmp" || err "extract failed"
[ -f "$tmp/$BINARY" ] || err "binary not found in archive"

# --- install ---------------------------------------------------------------
if [ -w "$BIN_DIR" ] 2>/dev/null; then
  mv "$tmp/$BINARY" "$BIN_DIR/$BINARY"
elif command -v sudo >/dev/null 2>&1 && [ -d "$BIN_DIR" ]; then
  echo "Installing to $BIN_DIR (needs sudo)..."
  sudo mv "$tmp/$BINARY" "$BIN_DIR/$BINARY"
else
  BIN_DIR="$HOME/.local/bin"
  mkdir -p "$BIN_DIR"
  mv "$tmp/$BINARY" "$BIN_DIR/$BINARY"
fi
chmod +x "$BIN_DIR/$BINARY"

echo "Installed $BINARY to $BIN_DIR/$BINARY"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "Note: $BIN_DIR is not on your PATH. Add it:"; echo "  export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac
