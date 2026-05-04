#!/usr/bin/env bash
# install-agent.sh — Installs Kilnx MCP server + skills across all detected AI platforms
#
# Usage:
#   ./install-agent.sh              # interactive install
#   ./install-agent.sh --force      # overwrite existing skills
#   ./install-agent.sh --mcp-only   # install only the MCP binary
#   ./install-agent.sh --skills-only # install only skills (no binary build)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="${SCRIPT_DIR}"
FORCE=0
MCP_ONLY=0
SKILLS_ONLY=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Options:
  --force        Overwrite existing skill files
  --mcp-only     Install only the MCP server binary
  --skills-only  Install only skills (skip binary build)
  --help         Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force) FORCE=1; shift ;;
    --mcp-only) MCP_ONLY=1; shift ;;
    --skills-only) SKILLS_ONLY=1; shift ;;
    --help) usage; exit 0 ;;
    *) log_error "Unknown option: $1"; usage; exit 1 ;;
  esac
done

# ── Detect platforms ──────────────────────────────────────────────────────────
DETECTED=()

has_claude() { [[ -d "${HOME}/.claude/skills" ]] || [[ -d "${HOME}/Library/Application Support/Claude" ]]; }
has_cursor() { [[ -d "${HOME}/.cursor" ]] || [[ -d "${HOME}/.config/Cursor" ]]; }
has_codex()  { [[ -d "${HOME}/.codex/skills" ]]; }
has_kimi()   { command -v kimi &>/dev/null || [[ -d "${HOME}/.claude/skills" ]]; }

has_claude && DETECTED+=("claude")
has_cursor && DETECTED+=("cursor")
has_codex  && DETECTED+=("codex")
has_kimi   && DETECTED+=("kimi")

log_info "Detected platforms: ${DETECTED[*]:-(none)}"

# ── Build MCP binary ──────────────────────────────────────────────────────────
install_mcp() {
  if [[ "$SKILLS_ONLY" -eq 1 ]]; then
    log_info "Skipping MCP binary build (--skills-only)"
    return
  fi

  log_info "Building Kilnx binary (includes MCP server)..."
  cd "${REPO_DIR}"
  go build -o kilnx ./cmd/kilnx/

  # Try to install to a PATH location
  if [[ -w "/usr/local/bin" ]]; then
    cp "${REPO_DIR}/kilnx" /usr/local/bin/kilnx
    log_ok "Installed kilnx → /usr/local/bin/kilnx"
  elif command -v install &>/dev/null && sudo -n true 2>/dev/null; then
    sudo cp "${REPO_DIR}/kilnx" /usr/local/bin/kilnx
    log_ok "Installed kilnx → /usr/local/bin/kilnx (via sudo)"
  else
    mkdir -p "${HOME}/.local/bin"
    cp "${REPO_DIR}/kilnx" "${HOME}/.local/bin/kilnx"
    log_warn "Installed kilnx → ${HOME}/.local/bin/kilnx (add to PATH)"
  fi
}

# ── Install skills ────────────────────────────────────────────────────────────
install_skill() {
  local src="$1"
  local dst="$2"
  local name="$3"

  if [[ -d "$dst" && "$FORCE" -eq 0 ]]; then
    log_warn "Skill already exists at $dst (use --force to overwrite)"
    return
  fi

  mkdir -p "$(dirname "$dst")"
  rm -rf "$dst"
  cp -r "$src" "$dst"
  log_ok "Installed Kilnx skill for $name → $dst"
}

install_skills() {
  if [[ "$MCP_ONLY" -eq 1 ]]; then
    log_info "Skipping skills install (--mcp-only)"
    return
  fi

  local skill_src="${REPO_DIR}/.claude/skills/kilnx"

  # Ensure we have the skill source
  if [[ ! -d "$skill_src" ]]; then
    # Fallback: use the codex-plugin skills directory
    skill_src="${REPO_DIR}/agents/codex-plugin/skills/kilnx"
  fi

  if [[ ! -d "$skill_src" ]]; then
    log_error "Skill source not found. Expected: ${REPO_DIR}/.claude/skills/kilnx"
    exit 1
  fi

  # Claude / Cursor / Kimi (shared path)
  if has_claude || has_cursor || has_kimi; then
    install_skill "$skill_src" "${HOME}/.claude/skills/kilnx" "Claude/Cursor/Kimi"
  fi

  # Codex
  if has_codex; then
    install_skill "$skill_src" "${HOME}/.codex/skills/kilnx" "Codex"
    # Also copy agents metadata if available
    if [[ -f "${skill_src}/agents/openai.yaml" ]]; then
      mkdir -p "${HOME}/.codex/skills/kilnx/agents"
      cp "${skill_src}/agents/openai.yaml" "${HOME}/.codex/skills/kilnx/agents/openai.yaml"
    fi
  fi
}

# ── Configure MCP clients ─────────────────────────────────────────────────────
configure_mcp_clients() {
  if [[ "$SKILLS_ONLY" -eq 1 ]]; then
    return
  fi

  local kilnx_path
  kilnx_path="$(command -v kilnx 2>/dev/null || true)"
  if [[ -z "$kilnx_path" ]]; then
    kilnx_path="${HOME}/.local/bin/kilnx"
  fi

  # Claude Desktop config (macOS/Linux)
  local claude_config_dir
  if [[ "$OSTYPE" == "darwin"* ]]; then
    claude_config_dir="${HOME}/Library/Application Support/Claude"
  else
    claude_config_dir="${HOME}/.config/Claude"
  fi

  if [[ -d "$claude_config_dir" ]]; then
    log_info "Claude Desktop detected. Add this to your claude_desktop_config.json:"
    cat <<EOF
{
  "mcpServers": {
    "kilnx": {
      "command": "${kilnx_path}",
      "args": ["mcp"]
    }
  }
}
EOF
  fi

  # Cursor MCP config
  if has_cursor; then
    log_info "Cursor detected. MCP config usually lives in:"
    echo "  ~/.cursor/mcp.json   or   ~/.config/Cursor/mcp.json"
    echo "Add the same kilnx block above."
  fi
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  echo ""
  echo "╔══════════════════════════════════════════════════════════════╗"
  echo "║          Kilnx Agent Installer (MCP + Skills)                ║"
  echo "╚══════════════════════════════════════════════════════════════╝"
  echo ""

  if ! command -v go &>/dev/null; then
    log_error "Go is not installed. Required to build the MCP binary."
    exit 1
  fi

  install_mcp
  install_skills
  configure_mcp_clients

  echo ""
  log_ok "Installation complete!"
  echo ""
  echo "Next steps:"
  echo "  1. Restart your AI client (new chat) to load the updated skill"
  echo "  2. Configure MCP in Claude Desktop / Cursor if not done above"
  echo "  3. Run './sync-kilnx-skill.sh' anytime to pull updates from the repo"
  echo ""
}

main "$@"
