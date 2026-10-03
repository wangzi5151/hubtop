#!/bin/sh
# Installs the latest hubtop release for your platform.
set -eu

REPO="wangzi5151/hubtop"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux) bin="hubtop-linux-$arch" ;;
  darwin) bin="hubtop-darwin-$arch" ;;
  *) echo "unsupported os: $os (download a .exe from Releases on Windows)" >&2; exit 1 ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
url="https://github.com/$REPO/releases/latest/download/$bin"
echo "downloading $url"
curl -fsSL "$url" -o "$tmp/hubtop"
chmod +x "$tmp/hubtop"

dest="${INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$dest"
mv "$tmp/hubtop" "$dest/hubtop"
echo "installed to $dest/hubtop"
"$dest/hubtop" --version
