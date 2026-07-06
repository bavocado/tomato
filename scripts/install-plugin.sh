#!/usr/bin/env bash
set -euo pipefail

# tomato plugin & codegraph installer
# curl -fsSL https://raw.githubusercontent.com/bavocado/tomato/main/scripts/install-plugin.sh | bash

SKILL_REPO="bavocado/tomato"
SKILL_BRANCH="${SKILL_BRANCH:-main}"
SKILL_DIR="${HOME}/.claude/skills/tomato"

info()  { printf '\033[1;34m[info]\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m[warn]\033[0m %s\n' "$*"; }
err()   { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; }

# ── tomato skill ──────────────────────────────────────────────

install_skill() {
  info "Installing tomato skill to ${SKILL_DIR}"

  mkdir -p "${SKILL_DIR}"

  local url="https://raw.githubusercontent.com/${SKILL_REPO}/${SKILL_BRANCH}/.claude/skills/tomato/SKILL.md"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}" -o "${SKILL_DIR}/SKILL.md"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "${SKILL_DIR}/SKILL.md" "${url}"
  else
    err "Need curl or wget"
    exit 1
  fi

  info "tomato skill installed: ${SKILL_DIR}/SKILL.md"
}

# ── codegraph ─────────────────────────────────────────────────

install_codegraph() {
  if command -v codegraph >/dev/null 2>&1; then
    info "codegraph already installed: $(command -v codegraph)"
    return
  fi

  info "Installing codegraph CLI..."
  curl -fsSL https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh | sh

  if command -v codegraph >/dev/null 2>&1; then
    info "codegraph installed: $(command -v codegraph)"
  elif [ -x "${HOME}/.local/bin/codegraph" ]; then
    info "codegraph installed: ${HOME}/.local/bin/codegraph"
    warn "~/.local/bin is not on PATH. Add it to your shell profile:"
    warn "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
}

# ── shell profile ─────────────────────────────────────────────

shell_profile() {
  case "${SHELL:-}" in
    */zsh)  echo "${HOME}/.zshrc" ;;
    */bash) echo "${HOME}/.bashrc" ;;
    *)      [ -f "${HOME}/.zshrc" ] && echo "${HOME}/.zshrc" || echo "${HOME}/.profile" ;;
  esac
}

ensure_path() {
  local profile
  profile="$(shell_profile)"

  if echo "$PATH" | tr ':' '\n' | grep -qF "${HOME}/.local/bin"; then
    return
  fi

  local begin="# >>> tomato-plugin >>>"
  local end="# <<< tomato-plugin <<<"
  local block="${begin}
# Added by tomato plugin installer
export PATH=\"\${HOME}/.local/bin:\$PATH\"
${end}"

  if grep -qF "$begin" "$profile" 2>/dev/null; then
    return
  fi

  {
    printf '\n%s\n' "$block"
  } >> "$profile"
  info "Added ~/.local/bin to PATH in $profile"
}

# ── main ──────────────────────────────────────────────────────

main() {
  echo ""
  info "tomato plugin + codegraph installer"
  echo ""

  install_skill
  echo ""
  install_codegraph
  echo ""
  ensure_path
  echo ""

  info "Done!"
  info "  Skill:  ${SKILL_DIR}/SKILL.md"
  info "  Usage:  just type 'tomato run' or describe a feature in Claude Code"
}

main "$@"
