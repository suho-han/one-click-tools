package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commandBase(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".exe")
}

func TestDetectManagerCursorAgent(t *testing.T) {
	manager := DetectManager(Tool{
		Name:          "Cursor",
		Package:       "cursor-agent",
		BinaryName:    "cursor-agent",
		BinaryAliases: []string{"cursor", "agent"},
	})

	if manager != CursorAgent {
		t.Fatalf("DetectManager(cursor-agent) = %q, want %q", manager, CursorAgent)
	}
}

func TestDetectManagerClaudeNative(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	versionDir := filepath.Join(home, ".local", "share", "claude", "versions")
	target := filepath.Join(versionDir, "2.1.214")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(binDir) error = %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(versionDir) error = %v", err)
	}
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	linkPath := filepath.Join(binDir, "claude")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	binaryLookup = func(name string) (string, error) {
		if name == "claude" {
			return linkPath, nil
		}
		return "", errExecutableNotFound
	}

	manager := DetectManager(Tool{
		Name:       "Claude Code",
		Package:    "@anthropic-ai/claude-code",
		BinaryName: "claude",
	})

	if manager != ClaudeNative {
		t.Fatalf("DetectManager(claude native) = %q, want %q", manager, ClaudeNative)
	}
}

func TestDetectManagerAntigravityInstaller(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	manager := DetectManager(Tool{
		Name:          "Antigravity CLI",
		Package:       "github.com/google-antigravity/antigravity-cli",
		BinaryName:    "agy",
		BinaryAliases: []string{"antigravity", "gemini", "gemini-cli"},
	})

	if manager != AntigravityInstaller {
		t.Fatalf("DetectManager(agy missing) = %q, want %q", manager, AntigravityInstaller)
	}
}

func TestDetectManagerAntigravityUpdater(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "agy" {
			return "/Users/test/.local/bin/agy", nil
		}
		return "", errExecutableNotFound
	}

	manager := DetectManager(Tool{
		Name:          "Antigravity CLI",
		Package:       "github.com/google-antigravity/antigravity-cli",
		BinaryName:    "agy",
		BinaryAliases: []string{"antigravity", "gemini", "gemini-cli"},
	})

	if manager != AntigravityUpdater {
		t.Fatalf("DetectManager(agy installed) = %q, want %q", manager, AntigravityUpdater)
	}
}

func TestDetectManagerOpenCodeNative(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "opencode" {
			return "/Users/test/.opencode/bin/opencode", nil
		}
		return "", errExecutableNotFound
	}

	manager := DetectManager(Tool{
		Name:       "OpenCode",
		Package:    "opencode-ai",
		BinaryName: "opencode",
	})

	if manager != OpenCodeNative {
		t.Fatalf("DetectManager(opencode installed) = %q, want %q", manager, OpenCodeNative)
	}
}

func TestDetectManagerCopilotNative(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "copilot" {
			return "/opt/homebrew/bin/copilot", nil
		}
		return "", errExecutableNotFound
	}

	manager := DetectManager(Tool{
		Name:        "GitHub Copilot",
		Package:     "@github/copilot",
		BinaryName:  "copilot",
		BrewPackage: "copilot-cli",
	})

	if manager != CopilotNative {
		t.Fatalf("DetectManager(copilot installed) = %q, want %q", manager, CopilotNative)
	}
}

func TestCursorAgentInstallCommand(t *testing.T) {
	cmd := CursorAgent.InstallCommand(Tool{BinaryName: "cursor-agent", BinaryAliases: []string{"cursor", "agent"}})
	if len(cmd.Args) != 3 {
		t.Fatalf("unexpected args length: %v", cmd.Args)
	}
	if commandBase(cmd.Args[0]) != "bash" || cmd.Args[1] != "-lc" || cmd.Args[2] != "curl https://cursor.com/install -fsS | bash" {
		t.Fatalf("CursorAgent.InstallCommand args = %v, want [bash -lc 'curl https://cursor.com/install -fsS | bash']", cmd.Args)
	}
}

func TestAntigravityInstallCommand(t *testing.T) {
	cmd := AntigravityInstaller.InstallCommand(Tool{BinaryName: "agy", BinaryAliases: []string{"antigravity", "gemini", "gemini-cli"}})
	if len(cmd.Args) != 3 {
		t.Fatalf("unexpected args length: %v", cmd.Args)
	}
	if commandBase(cmd.Args[0]) != "bash" || cmd.Args[1] != "-lc" || cmd.Args[2] != "curl -fsSL https://antigravity.google/cli/install.sh | bash" {
		t.Fatalf("AntigravityInstaller.InstallCommand args = %v, want [bash -lc 'curl -fsSL https://antigravity.google/cli/install.sh | bash']", cmd.Args)
	}
}

func TestClaudeNativeInstallCommand(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "claude" {
			return "/Users/test/.local/bin/claude", nil
		}
		return "", errExecutableNotFound
	}

	cmd := ClaudeNative.InstallCommand(Tool{BinaryName: "claude", BinaryAliases: []string{"claude-code"}})
	if len(cmd.Args) != 2 || commandBase(cmd.Args[0]) != "claude" || cmd.Args[1] != "update" {
		t.Fatalf("ClaudeNative.InstallCommand args = %v, want [claude update]", cmd.Args)
	}
}

func TestAntigravityUpdaterInstallCommand(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "agy" {
			return "/Users/test/.local/bin/agy", nil
		}
		return "", errExecutableNotFound
	}

	cmd := AntigravityUpdater.InstallCommand(Tool{BinaryName: "agy", BinaryAliases: []string{"antigravity", "gemini", "gemini-cli"}})
	if len(cmd.Args) != 2 || commandBase(cmd.Args[0]) != "agy" || cmd.Args[1] != "update" {
		t.Fatalf("AntigravityUpdater.InstallCommand args = %v, want [agy update]", cmd.Args)
	}
}

func TestOpenCodeNativeInstallCommand(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "opencode" {
			return "/Users/test/.opencode/bin/opencode", nil
		}
		return "", errExecutableNotFound
	}

	cmd := OpenCodeNative.InstallCommand(Tool{BinaryName: "opencode", BinaryAliases: []string{"opencode-ai"}})
	if len(cmd.Args) != 2 || commandBase(cmd.Args[0]) != "opencode" || cmd.Args[1] != "upgrade" {
		t.Fatalf("OpenCodeNative.InstallCommand args = %v, want [opencode upgrade]", cmd.Args)
	}
}

func TestCopilotNativeInstallCommand(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "copilot" {
			return "/opt/homebrew/bin/copilot", nil
		}
		return "", errExecutableNotFound
	}

	cmd := CopilotNative.InstallCommand(Tool{BinaryName: "copilot"})
	if len(cmd.Args) != 2 || commandBase(cmd.Args[0]) != "copilot" || cmd.Args[1] != "update" {
		t.Fatalf("CopilotNative.InstallCommand args = %v, want [copilot update]", cmd.Args)
	}
}

func TestCursorAgentNoChangeOutput(t *testing.T) {
	if !CursorAgent.IsNoChangeOutput("Already on the latest version") {
		t.Fatal("expected latest-version message to be treated as no change")
	}
}

func TestFirstNonEmptyLine(t *testing.T) {
	got := firstNonEmptyLine("\nGitHub Copilot CLI 1.0.65.\nRun 'copilot update' to check for updates.\n")
	if got != "GitHub Copilot CLI 1.0.65." {
		t.Fatalf("firstNonEmptyLine() = %q", got)
	}
}

func TestDetectManagerByPackagePrefix(t *testing.T) {
	tests := []struct {
		pkg  string
		want Manager
	}{
		{pkg: "cargo:uv", want: Cargo},
		{pkg: "go:github.com/foo/bar/cmd/baz", want: GoInstall},
		{pkg: "pip:llm", want: Pip},
	}

	for _, tc := range tests {
		got := DetectManager(Tool{Package: tc.pkg, BinaryName: "dummy"})
		if got != tc.want {
			t.Fatalf("DetectManager(%q) = %q, want %q", tc.pkg, got, tc.want)
		}
	}
}

func TestInstallCommandForExpandedManagers(t *testing.T) {
	cargoCmd := Cargo.InstallCommand(Tool{Package: "cargo:uv"})
	if got := cargoCmd.Args; len(got) != 4 || commandBase(got[0]) != "cargo" || got[1] != "install" || got[2] != "uv" || got[3] != "--locked" {
		t.Fatalf("unexpected cargo command: %v", got)
	}

	goCmd := GoInstall.InstallCommand(Tool{Package: "go:github.com/some/tool/cmd/tool"})
	if got := goCmd.Args; len(got) != 3 || commandBase(got[0]) != "go" || got[1] != "install" || got[2] != "github.com/some/tool/cmd/tool@latest" {
		t.Fatalf("unexpected go install command: %v", got)
	}

	pipCmd := Pip.InstallCommand(Tool{Package: "pip:llm"})
	if got := pipCmd.Args; len(got) != 6 || (commandBase(got[0]) != "python3" && commandBase(got[0]) != "python") || got[1] != "-m" || got[2] != "pip" || got[3] != "install" || got[4] != "--upgrade" || got[5] != "llm" {
		t.Fatalf("unexpected pip command: %v", got)
	}
}

func TestPreferredBinariesIncludesAliases(t *testing.T) {
	got := preferredBinaries(Tool{BinaryName: "cursor-agent", BinaryAliases: []string{"cursor", "agent", "cursor"}})
	want := []string{"cursor-agent", "cursor", "agent"}
	if len(got) != len(want) {
		t.Fatalf("preferredBinaries len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("preferredBinaries[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestDetectManagerPrefersBinaryProvenanceOverPackageLists(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "codex" {
			return "/opt/homebrew/bin/codex", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		switch name {
		case "brew":
			if len(args) >= 1 && args[0] == "--prefix" {
				return []byte("/opt/homebrew\n"), nil
			}
		case "npm":
			if len(args) >= 2 && args[0] == "prefix" && args[1] == "-g" {
				return []byte("/Users/test/.npm-global\n"), nil
			}
			if len(args) >= 3 && args[0] == "list" {
				return []byte("@openai/codex@1.2.3\n"), nil
			}
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "@openai/codex", BinaryName: "codex", BrewPackage: "codex"})
	if got != Brew {
		t.Fatalf("DetectManager() = %q, want %q", got, Brew)
	}
}

func TestDetectManagerUsesPnpmPrefixOwnership(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "claude" {
			return "/Users/test/Library/pnpm/claude", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		switch name {
		case "pnpm":
			if len(args) >= 1 && args[0] == "bin" {
				return []byte("/Users/test/Library/pnpm\n"), nil
			}
		case "npm":
			if len(args) >= 2 && args[0] == "prefix" && args[1] == "-g" {
				return []byte("/Users/test/.npm-global\n"), nil
			}
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "@anthropic-ai/claude-code", BinaryName: "claude"})
	if got != Pnpm {
		t.Fatalf("DetectManager() = %q, want %q", got, Pnpm)
	}
}

func TestDetectManagerUsesNpmNodeModulesSymlinkOwnership(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	targetDir := filepath.Join(home, ".local", "lib", "node_modules", "@openai", "codex", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(binDir) error = %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "codex.js"), []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	linkPath := filepath.Join(binDir, "codex")
	if err := os.Symlink(filepath.Join("..", "lib", "node_modules", "@openai", "codex", "bin", "codex.js"), linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	binaryLookup = func(name string) (string, error) {
		if name == "codex" {
			return linkPath, nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "python3" && len(args) >= 3 && args[0] == "-m" && args[1] == "site" && args[2] == "--user-base" {
			return []byte(filepath.Join(home, ".local") + "\n"), nil
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "@openai/codex", BinaryName: "codex"})
	if got != Npm {
		t.Fatalf("DetectManager() = %q, want %q", got, Npm)
	}
}

func TestDetectManagerUsesGoInstallBinaryOwnership(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "mytool" {
			return filepath.Clean("/home/test/go/bin/mytool"), nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "go" && len(args) >= 2 && args[0] == "env" && args[1] == "GOPATH" {
			return []byte("/home/test/go\n"), nil
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "go:github.com/example/mytool", BinaryName: "mytool"})
	if got != GoInstall {
		t.Fatalf("DetectManager() = %q, want %q", got, GoInstall)
	}
}

func TestDetectManagerReturnsUnknownWhenPackageAndProvenanceAreAmbiguous(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "mystery" {
			return "/tmp/mystery", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "mystery-tool", BinaryName: "mystery"})
	if got != Unknown {
		t.Fatalf("DetectManager() = %q, want %q", got, Unknown)
	}
}

func TestResolveManagerForInstallFallsBackToDefaultManager(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	got := ResolveManagerForInstall(Tool{Package: "mystery-tool", BinaryName: "mystery"})
	if got != Npm {
		t.Fatalf("ResolveManagerForInstall() = %q, want %q", got, Npm)
	}
}

func TestBuiltInToolManagerSupportMatrix(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	wantByBinary := map[string]Manager{
		"claude":       Npm,
		"commandcode":  Npm,
		"cursor-agent": CursorAgent,
		"opencode":     Npm,
		"codex":        Npm,
		"agy":          AntigravityInstaller,
		"copilot":      Npm,
	}

	for _, tool := range Tools {
		want, ok := wantByBinary[tool.BinaryName]
		if !ok {
			t.Fatalf("missing support-matrix expectation for %q", tool.BinaryName)
		}
		if got := ResolveManagerForInstall(tool); got != want {
			t.Fatalf("ResolveManagerForInstall(%s) = %q, want %q", tool.BinaryName, got, want)
		}
	}
}

func stubManagerDetection(t *testing.T) func() {
	t.Helper()
	origLookup := binaryLookup
	origOutput := commandOutput
	binaryLookup = func(string) (string, error) { return "", errExecutableNotFound }
	commandOutput = func(string, ...string) ([]byte, error) { return nil, errExecutableNotFound }
	return func() {
		binaryLookup = origLookup
		commandOutput = origOutput
	}
}
