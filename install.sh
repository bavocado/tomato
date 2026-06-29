#!/usr/bin/env bash
set -euo pipefail

REPO="bavocado/tomato"
INSTALL_DIR="${INSTALL_DIR:-}"
VERSION="${VERSION:-latest}"
INSTALL_ADAPTER="${INSTALL_ADAPTER:-1}"

info() { printf '\033[1;34m[info]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*"; }
err()  { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; }

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    err "Missing required command: $1"
    exit 1
  fi
}

normalize_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux)  echo "linux" ;;
    *) err "Unsupported OS: $(uname -s)"; exit 1 ;;
  esac
}

normalize_arch() {
  case "$(uname -m)" in
    arm64|aarch64) echo "arm64" ;;
    x86_64|amd64)  echo "amd64" ;;
    *) err "Unsupported architecture: $(uname -m)"; exit 1 ;;
  esac
}

pick_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    echo "$INSTALL_DIR"
    return
  fi

  if [ -d /opt/homebrew/bin ] && [ -w /opt/homebrew/bin ]; then
    echo "/opt/homebrew/bin"
    return
  fi

  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    echo "/usr/local/bin"
    return
  fi

  mkdir -p "$HOME/.local/bin"
  echo "$HOME/.local/bin"
}

latest_tag() {
  if command -v gh >/dev/null 2>&1; then
    gh release view --repo "$REPO" --json tagName --jq .tagName
    return
  fi

  need_cmd curl
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | head -n 1
}

download_asset() {
  local tag="$1"
  local asset="$2"
  local out="$3"

  local url="https://github.com/${REPO}/releases/download/${tag}/${asset}"
  info "Downloading $url"

  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$out" "$url"
  else
    err "Need curl or wget to download release assets"
    exit 1
  fi
}

extract_archive() {
  local archive="$1"
  local dest="$2"
  case "$archive" in
    *.zip)
      need_cmd unzip
      unzip -q "$archive" -d "$dest"
      ;;
    *.tar.gz)
      tar xzf "$archive" -C "$dest"
      ;;
    *)
      err "Unsupported archive format: $archive"
      exit 1
      ;;
  esac
}

install_binary() {
  local binary="$1"
  local install_dir="$2"
  chmod +x "$binary"
  install -m 0755 "$binary" "$install_dir/$(basename "$binary" | sed 's/_darwin_arm64$//;s/_darwin_amd64$//;s/_linux_arm64$//;s/_linux_amd64$//')"
}

main() {
  local os arch ext tag install_dir tmpdir tomato_asset adapter_asset

  os="$(normalize_os)"
  arch="$(normalize_arch)"
  ext="tar.gz"
  [ "$os" = "darwin" ] && ext="zip"

  if [ "$VERSION" = "latest" ]; then
    tag="$(latest_tag)"
  else
    tag="$VERSION"
  fi

  if [ -z "$tag" ]; then
    err "Could not determine release tag"
    exit 1
  fi

  install_dir="$(pick_install_dir)"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  tomato_asset="tomato_${os}_${arch}.${ext}"
  adapter_asset="github-tomato-adapter_${os}_${arch}.${ext}"

  info "Installing tomato $tag for ${os}/${arch}"
  info "Install directory: $install_dir"

  download_asset "$tag" "$tomato_asset" "$tmpdir/$tomato_asset"
  extract_archive "$tmpdir/$tomato_asset" "$tmpdir"
  install_binary "$tmpdir/tomato_${os}_${arch}" "$install_dir"

  if [ "$INSTALL_ADAPTER" = "1" ]; then
    download_asset "$tag" "$adapter_asset" "$tmpdir/$adapter_asset"
    extract_archive "$tmpdir/$adapter_asset" "$tmpdir"
    install_binary "$tmpdir/github-tomato-adapter_${os}_${arch}" "$install_dir"
  fi

  info "Installed: $(command -v tomato || echo "$install_dir/tomato")"
  if command -v tomato >/dev/null 2>&1; then
    tomato --version || true
  else
    warn "$install_dir is not on PATH. Add it to your shell profile."
  fi

  if [ "$INSTALL_ADAPTER" = "1" ]; then
    info "Installed adapter: $install_dir/github-tomato-adapter"
  fi
}

main "$@"
