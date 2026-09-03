package update

import (
	"fmt"
	"strconv"
	"strings"
)

type Tool struct {
	Name          string
	Package       string
	BinaryName    string
	BinaryAliases []string
	BrewPackage   string
	Icon          string
	LobeIcon      string
	HexColor      string
}

func (t Tool) BrewTarget() string {
	if t.BrewPackage != "" {
		return t.BrewPackage
	}
	return t.BinaryName
}

func (t Tool) MatchesName(name string) bool {
	normalized := NormalizeToolName(name)
	if normalized == NormalizeToolName(t.BinaryName) {
		return true
	}
	for _, alias := range t.BinaryAliases {
		if normalized == NormalizeToolName(alias) {
			return true
		}
	}
	return false
}

func (t Tool) hexRGB() (uint8, uint8, uint8, bool) {
	if len(t.HexColor) != 7 || t.HexColor[0] != '#' {
		return 0, 0, 0, false
	}
	r, err1 := strconv.ParseUint(t.HexColor[1:3], 16, 8)
	g, err2 := strconv.ParseUint(t.HexColor[3:5], 16, 8)
	b, err3 := strconv.ParseUint(t.HexColor[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return uint8(r), uint8(g), uint8(b), true
}

func (t Tool) Colorize(text string) string {
	r, g, b, ok := t.hexRGB()
	if !ok {
		return text
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, text)
}

func (t Tool) ColorizeWithBackgroundBlackText(text string) string {
	r, g, b, ok := t.hexRGB()
	if !ok {
		return text
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[30m%s\x1b[0m", r, g, b, text)
}

var Tools = []Tool{
	{
		Name:          "Claude Code",
		Package:       "@anthropic-ai/claude-code",
		BinaryName:    "claude",
		BinaryAliases: []string{"claude-code"},
		BrewPackage:   "claude-code",
		Icon:          "🤖",
		LobeIcon:      "ClaudeCode",
		HexColor:      "#D97757",
	},
	{
		Name:          "Command Code",
		Package:       "command-code",
		BinaryName:    "commandcode",
		BinaryAliases: []string{"command-code", "cmd", "cmdc"},
		Icon:          "⌘",
		LobeIcon:      "CommandCode",
		HexColor:      "#60A5FA",
	},
	{
		Name:          "Cursor CLI",
		Package:       "cursor-agent",
		BinaryName:    "cursor-agent",
		BinaryAliases: []string{"cursor", "agent"},
		BrewPackage:   "cursor-agent",
		Icon:          "▣",
		LobeIcon:      "Cursor",
		HexColor:      "#E6EDF3",
	},
	{
		Name:          "OpenCode",
		Package:       "opencode-ai",
		BinaryName:    "opencode",
		BinaryAliases: []string{"opencode-ai"},
		BrewPackage:   "opencode",
		Icon:          "🧩",
		LobeIcon:      "OpenCode",
		HexColor:      "#4F46E5",
	},
	{
		Name:        "OpenAI Codex",
		Package:     "@openai/codex",
		BinaryName:  "codex",
		BrewPackage: "codex",
		Icon:        "⚛️",
		LobeIcon:    "Codex",
		HexColor:    "#00A67E",
	},
	{
		Name:          "Antigravity CLI",
		Package:       "github.com/google-antigravity/antigravity-cli",
		BinaryName:    "agy",
		BinaryAliases: []string{"antigravity", "gemini", "gemini-cli"},
		Icon:          "✨",
		LobeIcon:      "GeminiCLI",
		HexColor:      "#4285F4",
	},
	{
		Name:        "GitHub Copilot",
		Package:     "@github/copilot",
		BinaryName:  "copilot",
		BrewPackage: "copilot-cli",
		Icon:        "🐙",
		LobeIcon:    "GithubCopilot",
		HexColor:    "#BC8CF2",
	},
}

func NormalizeToolName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "none":
		return ""
	case "claude-code":
		return "claude"
	case "command-code", "cmd", "cmdc":
		return "commandcode"
	case "cursor", "agent":
		return "cursor-agent"
	case "antigravity", "gemini", "gemini-cli":
		return "agy"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}
func splitToolNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		for _, part := range strings.Split(name, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func canonicalToolMap() map[string]Tool {
	toolMap := make(map[string]Tool, len(Tools))
	for _, t := range Tools {
		toolMap[NormalizeToolName(t.BinaryName)] = t
	}
	return toolMap
}

func GetOrderedTools(order []string) []Tool {
	order = splitToolNames(order)
	if len(order) == 0 {
		return Tools
	}

	ordered := make([]Tool, 0, len(Tools))
	toolMap := canonicalToolMap()
	seen := make(map[string]bool, len(Tools))

	for _, name := range order {
		normalized := NormalizeToolName(name)
		if normalized == "" || seen[normalized] {
			continue
		}
		if t, ok := toolMap[normalized]; ok {
			ordered = append(ordered, t)
			seen[normalized] = true
		}
	}

	for _, t := range Tools {
		normalized := NormalizeToolName(t.BinaryName)
		if seen[normalized] {
			continue
		}
		ordered = append(ordered, t)
	}

	return ordered
}

func GetFilteredTools(enabled []string, ordered []Tool) []Tool {
	enabled = splitToolNames(enabled)
	if len(enabled) == 0 {
		return ordered
	}
	result := make([]Tool, 0, len(enabled))
	seen := make(map[string]bool, len(enabled))
	for _, et := range enabled {
		normalized := NormalizeToolName(et)
		if normalized == "" || seen[normalized] {
			continue
		}
		for _, t := range ordered {
			if t.MatchesName(normalized) {
				result = append(result, t)
				seen[normalized] = true
				break
			}
		}
	}
	return result
}
