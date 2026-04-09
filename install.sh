#!/usr/bin/env bash
set -euo pipefail

REPO="index-null/cmus-lyric"
BINARY="lyrics"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
}

detect_platform() {
  local os arch
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac

  case "$os" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac

  echo "${os}_${arch}"
}

main() {
  local version platform url tmpdir

  echo "Detecting platform..."
  platform=$(detect_platform)
  echo "Platform: ${platform}"

  echo "Fetching latest version..."
  version=$(get_latest_version)
  echo "Version: v${version}"

  url="https://github.com/${REPO}/releases/download/v${version}/cmus-lyric_${version}_${platform}.tar.gz"
  echo "Downloading: ${url}"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  curl -fsSL "$url" -o "${tmpdir}/release.tar.gz"
  tar -xzf "${tmpdir}/release.tar.gz" -C "$tmpdir"

  echo "Installing to ${INSTALL_DIR}/${BINARY}..."
  install -d "$INSTALL_DIR"
  install -m 755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

  echo "Done! Run 'lyrics' to start."
}

main
