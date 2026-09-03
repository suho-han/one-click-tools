# one-click-ai-tools (oct)

**one-click-ai-tools (oct)** is a CLI utility to bootstrap, update, and inspect popular AI developer tools from one command.

## Supported AI Agents
- **Claude Code** (`@anthropic-ai/claude-code`)
- **Command Code** (`command-code`, binary: `commandcode` / `cmd`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (official installer, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)

## Installation

### GitHub Releases installer

```bash
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | sh
```

스크립트는 현재 OS/CPU에 맞는 GitHub Release 바이너리를 내려받고, 릴리스 checksum 항목이 있으면 검증한 뒤 기본적으로 `~/.local/bin/oct`에 설치합니다. 터미널에서 실행하면 설치 직후 `oct config`가 자동으로 열려 provider/usage 설정까지 이어서 진행합니다.

```bash
# 특정 버전 설치
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_VERSION=v0.1.1 sh

# 설치 경로 변경
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_DIR=/usr/local/bin sh

# 설치 후 config 단계 건너뛰기
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_RUN_CONFIG=0 sh
```

## Quick Start

자주 쓰는 흐름만 먼저 보면:

```bash
# 전체 agent 업데이트
oct agent-update

# 실행 없이 update plan만 확인
oct agent-update --dry-run --explain

# 토큰 없이 세션 상태 probe
oct session-refresh --dry-run

# usage 확인
oct usage

# release 전 점검
oct release-doctor

# shell PATH/bootstrap 점검
oct doctor shell
```

## Menubar helper (macOS)

Swift menubar helper를 따로 빌드/설치할 수 있습니다.

```bash
# helper 탐색/launch 상태 점검
oct menubar doctor

# Swift helper build
oct menubar build-helper

# ~/.local/bin/OctMenubarApp 로 설치
oct menubar install-helper
```

## Manager Support Matrix

| Manager | 감지 기준 | 설치 경로 | built-in 사용처 |
| --- | --- | --- | --- |
| `brew` | `brew --prefix` 하위 binary 또는 `brew list` | `brew upgrade <formula>` | Homebrew로 설치된 Claude/Cursor/OpenCode/Codex |
| `npm` | `npm prefix -g` 또는 `npm list -g` | `npm install -g <package>` | Claude/Command Code/OpenCode/Codex/Copilot 기본 fallback |
| `pnpm` | `pnpm bin -g` 또는 `pnpm list -g` | `pnpm add -g <package>` | provenance 기반 감지만 지원 |
| `yarn` | `yarn global bin` 또는 `yarn global list` | `yarn global add <package>` | provenance 기반 감지만 지원 |
| `cargo` | `cargo:` package prefix 또는 cargo bin path | `cargo install <crate> --locked` | explicit package override |
| `go-install` | `go:` package prefix 또는 `go env GOPATH` bin path | `go install <package>@latest` | explicit package override |
| `pip` | `pip:` package prefix 또는 `python3 -m site --user-base` bin path | `python3 -m pip install --upgrade <package>` | explicit package override |
| `cursor-agent` | tool identity (`cursor-agent` / `cursor` / `agent`) | `curl https://cursor.com/install -fsS \| bash` | Cursor CLI |
| `antigravity-installer` | tool identity (`agy` / `antigravity`, legacy `gemini*`) | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` | Antigravity CLI |

이 built-in support matrix는 `internal/update/manager_test.go`에서 회귀 테스트로 고정합니다.

## Requirements
- **Homebrew** (macOS, agent update support)
- **Go >= 1.25** (source build/test)

## Release
- primary binary distribution: GitHub Releases + `scripts/install.sh`
- local release wrapper: `bash scripts/release-package.sh vX.Y.Z`

### Release preflight
```bash
# local CLI version
go run main.go --version

# release integrity + Go validation
bash scripts/verify-release-integrity.sh
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go build ./...
```

### Publish lanes
1. Direct binary release path
   - GitHub Actions `goreleaser` publishes Linux/Windows assets.
   - macOS assets are built/uploaded by the `darwin-assets` job and appended to `checksums.txt`.
   - Users install/update through `scripts/install.sh` or `oct update`.
2. Manual CI rerun
   - GitHub Actions → `goreleaser` → `Run workflow`
   - `release_mode=release`
   - `git_ref=vX.Y.Z`
