# Detailed Usage Guide (oct)

This page covers core commands and operational configuration for `one-click-tools`.

## 1) `oct update`

Updates `oct` itself.

```bash
oct update
oct update --beta
```

## 2) `oct agent-update`

Updates all supported agents.

Supported agents:
- Claude Code (`@anthropic-ai/claude-code`)
- Command Code (`command-code`, binary: `commandcode` / `cmd`)
- OpenAI Codex (`@openai/codex`)
- Antigravity CLI (official installer, binary: `agy`)
- GitHub Copilot (`@github/copilot`)
- Cursor (`cursor-agent`)
- OpenCode (`opencode-ai`)

Default behavior:
- macOS: `brew update/upgrade`, npm-based updates, then fallback paths when needed
- Linux: npm-based updates, with `sudo` path on permission failures

Logs:
- `~/.oct/logs/agent-update-YYYYMMDD-HHMMSS.log`

## 3) `oct usage`

Collects and prints usage from configured providers.

```bash
oct usage
oct usage --compact
oct usage --json
oct usage --notify
```

Notes:
- In non-TTY environments (CI/pipes), output auto-switches to JSON.
- `--compact` prints remaining percent in one line, for example `C-88% X-45% G-72%` (`Claude=C`, `Codex=X`, `Gemini/Antigravity=G`).
- Alert logic applies when `--notify` is set or `usage_alert_enabled=true`.
- Selected providers are filtered by `enabled_tools`, and output order follows `agent_order`.
- Command Code reads 5h/7d/monthly buckets from the billing API using `COMMAND_CODE_API_KEY` or `~/.commandcode/auth.json`.
- Legacy config values `gemini` and `gemini-cli` are still accepted, but they normalize internally to `agy`.

### Key environment variables

- Common endpoint overrides:
  - `OCT_CODEX_USAGE_ENDPOINT`
  - `OCT_CLAUDE_USAGE_ENDPOINT`
  - `OCT_COPILOT_USAGE_ENDPOINT`
- Antigravity compatibility testing overrides (legacy alias only):
  - `OCT_GEMINI_USAGE_ENDPOINT`
  - `OCT_GEMINI_API_ENDPOINT`
- Cursor:
  - `OCT_CURSOR_USAGE_URL` (custom remote endpoint)
  - `CURSOR_API_KEY` (Bearer token used with `OCT_CURSOR_USAGE_URL`)
  - `OCT_CURSOR_API_USAGE_URL` (override for `https://api2.cursor.sh/auth/usage`)
- Copilot filters:
  - `OCT_COPILOT_USAGE_YEAR`, `OCT_COPILOT_USAGE_MONTH`, `OCT_COPILOT_USAGE_DAY`
  - `OCT_COPILOT_USAGE_MODEL`, `OCT_COPILOT_USAGE_PRODUCT`
- Debug:
  - `OCT_USAGE_DEBUG=1` (shows richer provider source details)

### Cursor usage fetch priority

1. `OCT_CURSOR_USAGE_URL` (custom remote)
2. Local auth token (`~/.config/cursor/auth.json`) + Cursor API
3. Local workspaceStorage fallback

### OpenCode usage source

OpenCode usage is read from local session logs first.

Primary paths:
- `~/.opencode/sessions`
- `~/.config/opencode/sessions`
- `~/.local/share/opencode/sessions`

## 4) Icon rendering

Renderer fallback order depends on terminal capability.

- Order: `native_image` -> `ansi_asset` -> `text`
- Override: `OCT_ICON_RENDERER=native_image|ansi_asset|text`
