# one-click-ai-tools (oct)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

[한국어](README.md)

**one-click-ai-tools (oct)** is a high-performance CLI for installing, updating, and inspecting popular AI developer tools from one command.

## 🚀 Quick Start

### Installation

#### GitHub Releases installer

```bash
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | sh
```

The installer downloads the matching GitHub Release binary, verifies it when the release checksum entry is present, installs `oct` to `~/.local/bin` by default, and opens `oct config` immediately when a terminal is available.

```bash
# Install a specific version
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_VERSION=v0.1.1 sh

# Install somewhere else
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_DIR=/usr/local/bin sh

# Skip post-install configuration
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_RUN_CONFIG=0 sh
```

### Core flows

```bash
# Update all AI agents
oct agent-update

# Show update plan without executing installs
oct agent-update --dry-run --explain

# Probe local auth/session state without sending prompts
oct session-refresh --dry-run

# Check usage/quota
oct usage

# Check release preflight
oct release-doctor

# Compare raw vs bootstrapped PATH resolution
oct doctor shell
```

## 🍎 Menubar helper (macOS)

The Swift menubar helper can be built and installed separately.

```bash
# Inspect helper resolution / launch mode
oct menubar doctor

# Build the Swift helper
oct menubar build-helper

# Install to ~/.local/bin/OctMenubarApp
oct menubar install-helper
```

## ✅ Common Commands

### Update agents

```bash
oct agent-update
```

### Token-free session refresh

```bash
oct session-refresh --dry-run
oct schedule config --enabled --interval daily --hour 9
```

### Check usage

```bash
oct usage
```

### Interactive `oct config`

```bash
oct config
```

- `↑/↓`: move cursor
- `Enter`: toggle current row
- `Enter` on `Choose all / Choose none`: toggle all rows
- `Enter` on final `Confirm` row: save and exit
- `Ctrl+C` / `Ctrl+Q` / `q`: cancel

Environment-variable overrides require the `OCT_` prefix.
- Example: `OCT_ENABLED_TOOLS=codex,agy`
- Non-prefixed vars like `ENABLED_TOOLS` are ignored.
- Legacy config values `gemini` and `gemini-cli` are still accepted and normalized to `agy`.

### Always-on monitoring

```bash
oct monitor --interval 10s
oct monitor --once
oct monitor --once --sort-by used --desc --top 5 --compact
```

### Usage alert configuration

```bash
oct alert config show
oct alert config set enabled true
oct alert config set cooldown_minutes 120
oct alert config set threshold_percent 85
oct alert config set critical_percent 98
oct alert config set quiet_hours 00:00-08:00
oct alert config set timezone Asia/Seoul
```

## 🛠 Supported Agents

- **Claude Code** (`@anthropic-ai/claude-code`)
- **Command Code** (`command-code`, binary: `commandcode` / `cmd`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (official installer, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)

## 🧭 Manager Support Matrix

| Manager | Detection strategy | Install path | Built-in use |
| --- | --- | --- | --- |
| `brew` | binary under `brew --prefix` / `brew list` | `brew upgrade <formula>` | Claude, Cursor, OpenCode, Codex when Homebrew-owned |
| `npm` | `npm prefix -g` / `npm list -g` | `npm install -g <package>` | default fallback for Claude, Command Code, OpenCode, Codex, Copilot |
| `pnpm` | `pnpm bin -g` / `pnpm list -g` | `pnpm add -g <package>` | provenance-based detection only |
| `yarn` | `yarn global bin` / `yarn global list` | `yarn global add <package>` | provenance-based detection only |
| `cargo` | `cargo:` package prefix / cargo bin path | `cargo install <crate> --locked` | explicit package override |
| `go-install` | `go:` package prefix / `go env GOPATH` bin path | `go install <package>@latest` | explicit package override |
| `pip` | `pip:` package prefix / `python3 -m site --user-base` bin path | `python3 -m pip install --upgrade <package>` | explicit package override |
| `cursor-agent` | tool identity (`cursor-agent` / `cursor` / `agent`) | `curl https://cursor.com/install -fsS \| bash` | Cursor CLI |
| `antigravity-installer` | tool identity (`agy` / `antigravity`, legacy `gemini*`) | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` | Antigravity CLI |

The built-in support matrix is regression-tested in `internal/update/manager_test.go` so manager fallback changes stay explicit.

## Requirements

- **Runtime users**
  - **macOS**: Homebrew for agent update support
- **Developers (build/test from source)**
  - **Go >= 1.25**

## Release

- Primary binary distribution: GitHub Releases + `scripts/install.sh`
- Local release wrapper: `bash scripts/release-package.sh vX.Y.Z`

## License

MIT © Suho Han
