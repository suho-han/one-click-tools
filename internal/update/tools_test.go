package update

import "testing"

func TestNormalizeToolNameAntigravityAliases(t *testing.T) {
	cases := []string{"agy", "antigravity", "gemini", "gemini-cli"}
	for _, in := range cases {
		if got := NormalizeToolName(in); got != "agy" {
			t.Fatalf("NormalizeToolName(%q) = %q, want agy", in, got)
		}
	}
}

func TestNormalizeToolNameCommandCodeAliases(t *testing.T) {
	cases := []string{"commandcode", "command-code", "cmd", "cmdc"}
	for _, in := range cases {
		if got := NormalizeToolName(in); got != "commandcode" {
			t.Fatalf("NormalizeToolName(%q) = %q, want commandcode", in, got)
		}
	}
}

func TestAntigravityToolDoesNotMatchAgAlias(t *testing.T) {
	for _, tool := range Tools {
		if tool.BinaryName != "agy" {
			continue
		}
		if tool.MatchesName("ag") {
			t.Fatalf("did not expect agy tool to match ag alias")
		}
		return
	}
	t.Fatal("agy tool not found")
}

func TestToolSelectionAcceptsCommaSeparatedValues(t *testing.T) {
	ordered := GetOrderedTools([]string{"codex, agy", "claude"})
	if len(ordered) < 3 {
		t.Fatalf("expected at least 3 ordered tools, got %d", len(ordered))
	}
	if ordered[0].BinaryName != "codex" || ordered[1].BinaryName != "agy" || ordered[2].BinaryName != "claude" {
		t.Fatalf("unexpected order from comma-separated values: %v", []string{ordered[0].BinaryName, ordered[1].BinaryName, ordered[2].BinaryName})
	}

	filtered := GetFilteredTools([]string{"codex, gemini", "cursor"}, ordered)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered tools, got %d", len(filtered))
	}
	if filtered[0].BinaryName != "codex" || filtered[1].BinaryName != "agy" || filtered[2].BinaryName != "cursor-agent" {
		t.Fatalf("unexpected filtered tools: %v", []string{filtered[0].BinaryName, filtered[1].BinaryName, filtered[2].BinaryName})
	}
}
