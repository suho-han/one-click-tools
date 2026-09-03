package usage

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func withMockAntigravityUsageCommand(t *testing.T, output string, err error) {
	t.Helper()
	old := antigravityUsageCommandOutput
	t.Cleanup(func() { antigravityUsageCommandOutput = old })
	antigravityUsageCommandOutput = func(timeout time.Duration, name string, args ...string) (string, error) {
		if timeout != 20*time.Second {
			t.Fatalf("timeout = %v, want 20s", timeout)
		}
		if name != "agy" {
			t.Fatalf("command = %q, want agy", name)
		}
		if len(args) != 2 || args[0] != "--print" || args[1] != "/usage" {
			t.Fatalf("args = %v, want [--print /usage]", args)
		}
		return output, err
	}
}

func TestParseAntigravityCLIUsage(t *testing.T) {
	rows := parseAntigravityCLIUsage("Gemini Models\tWeekly Limit Remaining\t100%\t2026-09-10T17:41:25Z\nClaude and GPT models\tWeekly Limit Remaining\t87.5%\t2026-09-10T17:41:25Z\n")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Label != "Gemini Models" || rows[0].Window != "Weekly" || rows[0].Remaining != 100 || rows[0].ResetTime != "2026-09-10T17:41:25Z" {
		t.Fatalf("unexpected first row: %#v", rows[0])
	}
	if rows[1].Label != "Claude and GPT models" || rows[1].Remaining != 87.5 {
		t.Fatalf("unexpected second row: %#v", rows[1])
	}
}

func TestFetchAntigravityUsageUsesAgyPrintUsage(t *testing.T) {
	withMockAntigravityUsageCommand(t, "Gemini Models\tWeekly Limit Remaining\t100%\t2026-09-10T17:41:25Z\nClaude and GPT models\tWeekly Limit Remaining\t87.5%\t2026-09-10T17:41:25Z\n", nil)

	result := FetchAntigravityUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q", result.Status)
	}
	if result.Source != "agy-cli" {
		t.Fatalf("expected agy-cli source, got %q", result.Source)
	}
	if result.Used != "12.5" {
		t.Fatalf("expected max used percent 12.5, got %q", result.Used)
	}
	if result.Buckets["model:Gemini"] != "0.0" {
		t.Fatalf("expected Gemini used 0.0, got %q", result.Buckets["model:Gemini"])
	}
	if result.Buckets["model:Claude/GPT"] != "12.5" {
		t.Fatalf("expected Claude/GPT used 12.5, got %q", result.Buckets["model:Claude/GPT"])
	}
	if result.BucketResets["model:Claude/GPT"] != "2026-09-10T17:41:25Z" {
		t.Fatalf("expected reset time, got %q", result.BucketResets["model:Claude/GPT"])
	}
}

func TestFetchAntigravityUsageNoCLIUsage(t *testing.T) {
	withMockAntigravityUsageCommand(t, "", errors.New("agy failed"))

	result := FetchAntigravityUsage()
	if result.Status != "warn" {
		t.Fatalf("expected warn status, got %q", result.Status)
	}
	if result.Used != "n/a" {
		t.Fatalf("expected n/a usage, got %q", result.Used)
	}
	if result.Unit != "percent" {
		t.Fatalf("expected percent unit, got %q", result.Unit)
	}
	if strings.EqualFold(result.Unit, "sessions") || strings.Contains(strings.ToLower(result.Message), "session") {
		t.Fatalf("local session fallback leaked into result: %#v", result)
	}
}

func TestFetchGeminiUsageDelegatesToAntigravity(t *testing.T) {
	withMockAntigravityUsageCommand(t, "Gemini Models\tWeekly Limit Remaining\t99%\t2026-09-10T17:41:25Z\n", nil)

	result := FetchGeminiUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if !strings.EqualFold(result.Source, "agy-cli") {
		t.Fatalf("expected agy-cli source, got %q", result.Source)
	}
}
