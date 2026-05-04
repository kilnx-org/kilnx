#!/usr/bin/env bash
# sync-kilnx-skill.sh — Syncs the latest Kilnx skill from the repo to all detected platforms
#
# Usage:
#   ./sync-kilnx-skill.sh           # sync skills + rebuild MCP binary
#   ./sync-kilnx-skill.sh --skills  # sync skills only (no rebuild)
#   ./sync-kilnx-skill.sh --check   # check if updates are available

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="${SCRIPT_DIR}"
SKILLS_ONLY=0
CHECK_ONLY=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skills) SKILLS_ONLY=1; shift ;;
    --check)  CHECK_ONLY=1; shift ;;
    --help)
      echo "Usage: $(basename "$0") [--skills] [--check]"
      exit 0
      ;;
    *) log_error "Unknown option: $1"; exit 1 ;;
  esac
done

# ── Check for updates ─────────────────────────────────────────────────────────
check_updates() {
  log_info "Checking for updates..."

  cd "${REPO_DIR}"

  # If it's a git repo, check if we're behind origin
  if git rev-parse --git-dir &>/dev/null; then
    git fetch --quiet origin main 2>/dev/null || true
    LOCAL=$(git rev-parse HEAD)
    REMOTE=$(git rev-parse origin/main 2>/dev/null || echo "$LOCAL")
    if [[ "$LOCAL" != "$REMOTE" ]]; then
      log_warn "Updates available on origin/main"
      echo "  Local:  ${LOCAL:0:8}"
      echo "  Remote: ${REMOTE:0:8}"
      return 1
    else
      log_ok "Already up to date (${LOCAL:0:8})"
      return 0
    fi
  else
    log_warn "Not a git repo — cannot check for remote updates"
    return 0
  fi
}

if [[ "$CHECK_ONLY" -eq 1 ]]; then
  check_updates
  exit $?
fi

# ── Pull latest (optional) ────────────────────────────────────────────────────
if git rev-parse --git-dir &>/dev/null; then
  log_info "Pulling latest changes..."
  git pull --ff-only origin main 2>/dev/null || git pull --ff-only 2>/dev/null || log_warn "Could not auto-pull (check manually)"
fi

# ── Rebuild MCP binary ────────────────────────────────────────────────────────
if [[ "$SKILLS_ONLY" -eq 0 ]]; then
  log_info "Rebuilding Kilnx binary..."
  cd "${REPO_DIR}"
  go build -o kilnx ./cmd/kilnx/

  # Copy to installed locations
  if [[ -f /usr/local/bin/kilnx ]]; then
    cp "${REPO_DIR}/kilnx" /usr/local/bin/kilnx
    log_ok "Updated /usr/local/bin/kilnx"
  elif [[ -f "${HOME}/.local/bin/kilnx" ]]; then
    cp "${REPO_DIR}/kilnx" "${HOME}/.local/bin/kilnx"
    log_ok "Updated ${HOME}/.local/bin/kilnx"
  fi
fi

# ── Sync skills ───────────────────────────────────────────────────────────────
sync_skill() {
  local src="$1"
  local dst="$2"
  local name="$3"

  if [[ ! -d "$dst" ]]; then
    log_warn "Skill not installed for $name (run install-agent.sh first)"
    return
  fi

  rm -rf "$dst"
  cp -r "$src" "$dst"
  log_ok "Synced skill for $name → $dst"
}

log_info "Syncing skills..."

local skill_src="${REPO_DIR}/.claude/skills/kilnx"
if [[ ! -d "$skill_src" ]]; then
  skill_src="${REPO_DIR}/agents/codex-plugin/skills/kilnx"
fi

# Claude / Cursor / Kimi
if [[ -d "${HOME}/.claude/skills" ]]; then
  sync_skill "$skill_src" "${HOME}/.claude/skills/kilnx" "Claude/Cursor/Kimi"
fi

# Codex
if [[ -d "${HOME}/.codex/skills" ]]; then
  sync_skill "$skill_src" "${HOME}/.codex/skills/kilnx" "Codex"
  if [[ -f "${skill_src}/agents/openai.yaml" ]]; then
    mkdir -p "${HOME}/.codex/skills/kilnx/agents"
    cp "${skill_src}/agents/openai.yaml" "${HOME}/.codex/skills/kilnx/agents/openai.yaml"
  fi
fi

# ── Sync Codex plugin (if user has it linked) ─────────────────────────────────
if [[ -d "${HOME}/.codex/.tmp/plugins/plugins/kilnx" ]]; then
  log_info "Syncing Codex plugin..."
  rm -rf "${HOME}/.codex/.tmp/plugins/plugins/kilnx"
  cp -r "${REPO_DIR}/agents/codex-plugin" "${HOME}/.codex/.tmp/plugins/plugins/kilnx"
  log_ok "Synced Codex plugin"
fi

# ── Regenerate llms-full.txt ──────────────────────────────────────────────────
if [[ -f "${REPO_DIR}/llms-full.txt" ]]; then
  log_info "Regenerating llms-full.txt..."
  cd "${REPO_DIR}"
  {
    echo "# Kilnx — Full Documentation for LLMs"
    echo ""
    echo "Concatenated reference: PRINCIPLES, GRAMMAR, FEATURES, CHANGELOG."
    echo ""
    echo "---"
    echo ""
    cat PRINCIPLES.md
    echo -e "\n---\n"
    cat GRAMMAR.md
    echo -e "\n---\n"
    cat FEATURES.md
    echo -e "\n---\n"
    cat CHANGELOG.md
  } > llms-full.txt
  log_ok "Regenerated llms-full.txt"
fi

echo ""
log_ok "Sync complete! Restart your AI clients to load updates."
