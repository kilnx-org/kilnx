# Kilnx Agent Distribution Architecture

How Kilnx ships to AI clients: MCP server, skills, plugins, and auto-update strategy.

---

## The Landscape

There is no universal marketplace for AI agent extensions. Each platform is a silo with its own packaging format, install mechanism, and update model.

| Platform | Extension Type | Install Model | Marketplace | Auto-Update |
|----------|---------------|---------------|-------------|-------------|
| **Claude Desktop** | Skills (local dir) + MCP servers | Manual copy or MCP config JSON | None (skills); MCP registries emerging | Manual / script |
| **Cursor** | VS Code extension + MCP + `.cursorrules` | VS Code marketplace + MCP config | VS Code marketplace (extensions) | Automatic (extensions) |
| **Codex (OpenAI)** | Plugins (`.codex-plugin/`) | Git clone or built-in marketplace | Curated list in `marketplace.json` | Plugin manager pulls latest |
| **Kimi CLI** | Skills (local dir) | Manual copy | None | Manual / script |
| **Generic MCP** | MCP servers (stdio/SSE) | `npm i -g`, `go install`, binary | npm, GitHub Releases, `mcp-get` | Package manager |

Kilnx ships as **four artifacts** that can evolve independently:

1. **MCP Server** (`kilnx mcp`) — universal, protocol-based
2. **Skill** (`SKILL.md`) — natural-language instructions for the model
3. **Codex Plugin** — bundles skill + MCP config + metadata
4. **Binary** (`kilnx`) — the compiler, runtime, and MCP server in one

---

## 1. MCP Server (Most Universal)

The MCP server is the most portable artifact because it speaks an open protocol (Model Context Protocol). Any client that supports MCP can use it without knowing anything about Kilnx's internals.

### Distribution

- **Primary:** GitHub Releases with pre-built binaries for macOS, Linux, Windows
- **Secondary:** `go install github.com/kilnx-org/kilnx/cmd/kilnx@latest`
- **Tertiary:** Homebrew formula (future)

### Client Configuration

Clients configure the MCP server by pointing to a command:

```json
{
  "mcpServers": {
    "kilnx": {
      "command": "/usr/local/bin/kilnx",
      "args": ["mcp"]
    }
  }
}
```

- **Claude Desktop:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Cursor:** `~/.cursor/mcp.json` or `~/.config/Cursor/mcp.json`
- **Codex Plugin:** `.mcp.json` inside the plugin directory
- **Generic:** any client that implements MCP stdio transport

### Update Strategy

Because the MCP server is embedded in the `kilnx` binary, updating the compiler also updates the MCP server:

```bash
# Option A: rebuild from source
go build -o kilnx ./cmd/kilnx/

# Option B: download release
curl -sL https://github.com/kilnx-org/kilnx/releases/latest/download/kilnx-linux-amd64 \
  -o /usr/local/bin/kilnx && chmod +x /usr/local/bin/kilnx
```

The `sync-kilnx-skill.sh` script rebuilds and copies the binary automatically.

---

## 2. Skills (Platform-Specific Directories)

A skill is a markdown file (`SKILL.md`) with YAML frontmatter that teaches the model how to work with Kilnx. It lives in different directories depending on the client.

### Directory Mapping

| Client | Skill Path |
|--------|-----------|
| Claude Desktop | `~/.claude/skills/kilnx/SKILL.md` |
| Cursor | `~/.claude/skills/kilnx/SKILL.md` (Cursor reads Claude's skill dir via symlink or shared path) |
| Kimi CLI | `~/.claude/skills/kilnx/SKILL.md` (Kimi also reads this path) |
| Codex | `~/.codex/skills/kilnx/SKILL.md` |

### Format

All platforms use the same base format:

```yaml
---
name: kilnx
description: "Write, read, debug, and refactor Kilnx code..."
---

# Kilnx
...
```

- **Claude/Kimi:** accept `user-invocable: true` in frontmatter
- **Codex:** prefers a longer `description` used for skill triggering; also wants `agents/openai.yaml` for UI metadata

### Update Strategy

Skills are plain files. The `sync-kilnx-skill.sh` script copies the canonical skill from the repo to all detected platform directories:

```bash
./sync-kilnx-skill.sh
```

This script also regenerates `llms-full.txt` (the concatenated single-file documentation used for full-context loads).

---

## 3. Codex Plugin (Bundle)

OpenAI Codex has the richest plugin architecture of the major clients. A Codex plugin is a directory with a manifest that can bundle:

- **Skills** (`skills/*/SKILL.md`)
- **MCP servers** (`.mcp.json`)
- **Apps** (`.app.json`)
- **Commands** (`commands/`)
- **Hooks** (`hooks.json`)
- **Assets** (icons, logos)

### Plugin Structure

```
kilnx-codex-plugin/
├── .codex-plugin/
│   └── plugin.json          # manifest (name, version, description, interface UI)
├── .mcp.json                # MCP server config (auto-wired by Codex)
├── skills/
│   └── kilnx/
│       ├── SKILL.md         # the skill
│       └── agents/
│           └── openai.yaml  # UI metadata
└── assets/
    ├── app-icon.png
    └── kilnx-small.svg
```

### Manifest Example

See `agents/codex-plugin/.codex-plugin/plugin.json` in this repo.

Key fields:
- `skills`: path to skills directory
- `mcpServers`: path to `.mcp.json`
- `interface`: UI metadata (display name, icon, default prompt, brand color)

### Installation

Currently, Codex plugins are installed by copying the plugin directory into the Codex plugins folder:

```bash
cp -r agents/codex-plugin ~/.codex/.tmp/plugins/plugins/kilnx
```

Then reference it in the user's personal `marketplace.json` or wait for Codex to scan the directory.

### Marketplace (Future)

OpenAI maintains a curated `marketplace.json` at:
`~/.codex/.tmp/plugins/.agents/plugins/marketplace.json`

Each entry points to a plugin source:

```json
{
  "name": "openai-curated",
  "plugins": [
    {
      "name": "kilnx",
      "source": {
        "source": "git",
        "url": "https://github.com/kilnx-org/kilnx.git",
        "path": "agents/codex-plugin"
      },
      "policy": {
        "installation": "AVAILABLE",
        "authentication": "NONE"
      },
      "category": "Coding"
    }
  ]
}
```

> **Note:** The `git` source type is speculative based on the architecture. As of today, OpenAI's curated marketplace only uses `local` paths. Third-party distribution will likely use `git` or a registry URL once the marketplace opens to external publishers.

### Update Strategy

When Codex supports git-sourced plugins, updates are automatic: Codex pulls the latest version of the plugin on startup or when the user clicks "Update".

Until then, use `sync-kilnx-skill.sh --skills` to copy the latest skill files into the plugin directory.

---

## 4. Auto-Update Architecture

Because there is no universal marketplace, Kilnx uses a **hub-and-spoke model**:

```
┌─────────────────────────────────────────┐
│         GitHub Repo (kilnx-org/kilnx)    │
│  ┌─────────┐ ┌─────────┐ ┌───────────┐ │
│  │ Source  │ │ Skills  │ │  Releases │ │
│  │  Code   │ │  (md)   │ │ (binaries)│ │
│  └────┬────┘ └────┬────┘ └─────┬─────┘ │
└───────┼───────────┼────────────┼───────┘
        │           │            │
   ┌────┴────┐ ┌────┴────┐  ┌────┴────┐
   │  go build│ │  git    │  │ curl    │
   │  (local) │ │  pull   │  │ release │
   └────┬─────┘ └────┬────┘  └────┬────┘
        │            │            │
   ┌────┴────────────┴────────────┴────┐
   │      sync-kilnx-skill.sh          │
   │  (detects platforms, copies files) │
   └─────────────┬─────────────────────┘
                 │
     ┌───────────┼───────────┐
     ▼           ▼           ▼
  ┌──────┐  ┌────────┐  ┌────────┐
  │Claude│  │ Codex  │  │  Kimi  │
  │Cursor│  │ Plugin │  │  CLI   │
  └──┬───┘  └───┬────┘  └───┬────┘
     │          │           │
     └──────┬───┘           │
            ▼               │
      ~/.claude/skills      │
            │               │
            └───────────────┘
                 ~/.codex/skills
```

### The Sync Script

`sync-kilnx-skill.sh` is the single command that updates everything:

1. `git pull` — fetch latest source
2. `go build` — rebuild the binary (includes updated MCP server)
3. Copy `SKILL.md` to `~/.claude/skills/kilnx/` and `~/.codex/skills/kilnx/`
4. Regenerate `llms-full.txt`

### Running on a Schedule

Add to your shell profile or cron for automatic daily updates:

```bash
# ~/.bashrc or ~/.zshrc
alias kilnx-update='cd ~/Dev/kilnx-org/kilnx && ./sync-kilnx-skill.sh'

# Or cron (daily at 9am)
0 9 * * * cd ~/Dev/kilnx-org/kilnx && ./sync-kilnx-skill.sh >> ~/.kilnx-update.log 2>&1
```

---

## 5. Publishing Checklist

When you release a new version of Kilnx, update these artifacts in order:

1. **Source code** — merge to `main`, tag with `vX.Y.Z`
2. **GitHub Release** — attach pre-built binaries (`kilnx-darwin-amd64`, `kilnx-linux-amd64`, etc.)
3. **Skills** — update `SKILL.md` with any new keywords, field types, or CLI commands
4. **Codex Plugin** — bump version in `agents/codex-plugin/.codex-plugin/plugin.json`
5. **llms-full.txt** — regenerate via `sync-kilnx-skill.sh` or manually concatenate PRINCIPLES + GRAMMAR + FEATURES + CHANGELOG
6. **Announce** — users run `sync-kilnx-skill.sh` or wait for their package manager to pick up the new release

---

## 6. Future Directions

| Idea | Status |
|------|--------|
| Homebrew formula | Not started |
| npm wrapper (`npx kilnx-mcp`) | Not started |
| VS Code extension for Kilnx LSP | Not started |
| Codex marketplace publishing | Waiting for OpenAI to open to third parties |
| Claude skill marketplace | Waiting for Anthropic to launch |
| `mcp-get` registry entry | Not started |
| Automatic update daemon | Not started |

---

## Quick Reference

```bash
# Install everything (binary + skills) for the first time
./install-agent.sh

# Update everything after pulling latest code
./sync-kilnx-skill.sh

# Update only skills (fast, no Go rebuild)
./sync-kilnx-skill.sh --skills

# Check if updates are available without applying
./sync-kilnx-skill.sh --check
```
