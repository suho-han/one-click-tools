# 상세 사용 가이드 (oct)

`one-click-tools`의 핵심 명령어와 운영 시 참고할 설정을 정리합니다.

## 1) `oct update`

`oct` 자체를 최신 버전으로 업데이트합니다.

```bash
oct update
oct update --beta
```

## 2) `oct agent-update`

지원 에이전트를 일괄 업데이트합니다.

지원 대상:
- Claude Code (`@anthropic-ai/claude-code`)
- Command Code (`command-code`, binary: `commandcode` / `cmd`)
- OpenAI Codex (`@openai/codex`)
- Antigravity CLI (official installer, binary: `agy`)
- GitHub Copilot (`@github/copilot`)
- Cursor (`cursor-agent`)
- OpenCode (`opencode-ai`)

기본 동작:
- macOS: `brew update/upgrade` 후 npm 기반 업데이트 + fallback 경로 시도
- Linux: npm 기반 업데이트, 권한 오류 시 `sudo` 경로 시도

로그:
- `~/.oct/logs/agent-update-YYYYMMDD-HHMMSS.log`

## 3) `oct usage`

에이전트 사용량을 수집/출력합니다.

```bash
oct usage
oct usage --compact
oct usage --json
oct usage --notify
```

참고:
- 비 TTY 환경(CI/파이프)에서는 자동으로 JSON 출력으로 전환됩니다.
- `--compact`는 잔여 percent를 한 줄로 출력합니다. 예: `C-88% X-45% G-72%` (`Claude=C`, `Codex=X`, `Gemini/Antigravity=G`).
- `--notify` 또는 `usage_alert_enabled=true`일 때 알림 규칙이 적용됩니다.
- 조회 대상 provider는 `enabled_tools` 기준이며, 출력 순서는 `agent_order`를 따릅니다.
- Command Code는 `COMMAND_CODE_API_KEY` 또는 `~/.commandcode/auth.json`의 로그인 정보를 사용해 billing API에서 5h/7d/monthly bucket을 조회합니다.
- legacy config 값 `gemini`, `gemini-cli`는 계속 허용되지만 내부적으로 `agy`로 normalize 됩니다.

### 주요 환경 변수

- 공통 endpoint override:
  - `OCT_CODEX_USAGE_ENDPOINT`
  - `OCT_CLAUDE_USAGE_ENDPOINT`
  - `OCT_COPILOT_USAGE_ENDPOINT`
- Antigravity compatibility testing override (legacy alias only):
  - `OCT_GEMINI_USAGE_ENDPOINT`
  - `OCT_GEMINI_API_ENDPOINT`
- Cursor:
  - `OCT_CURSOR_USAGE_URL` (커스텀 원격 endpoint)
  - `CURSOR_API_KEY` (`OCT_CURSOR_USAGE_URL` 호출 시 Bearer)
  - `OCT_CURSOR_API_USAGE_URL` (`https://api2.cursor.sh/auth/usage` 대체)
- Copilot 필터:
  - `OCT_COPILOT_USAGE_YEAR`, `OCT_COPILOT_USAGE_MONTH`, `OCT_COPILOT_USAGE_DAY`
  - `OCT_COPILOT_USAGE_MODEL`, `OCT_COPILOT_USAGE_PRODUCT`
- 디버그:
  - `OCT_USAGE_DEBUG=1` (provider별 source detail 확장)

### Cursor 사용량 수집 우선순위

1. `OCT_CURSOR_USAGE_URL` (커스텀 원격)
2. 로컬 auth token(`~/.config/cursor/auth.json`) + Cursor API
3. 로컬 workspaceStorage 기반 fallback

### OpenCode 사용량 소스

- 우선 로컬 세션 로그를 읽습니다.
- 주요 경로:
  - `~/.opencode/sessions`
  - `~/.config/opencode/sessions`
  - `~/.local/share/opencode/sessions`

## 4) 아이콘 렌더링

터미널 아이콘 렌더러는 환경에 따라 fallback 됩니다.

- 우선순위: `native_image` -> `ansi_asset` -> `text`
- 강제 지정: `OCT_ICON_RENDERER=native_image|ansi_asset|text`
