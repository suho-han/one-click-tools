package usage

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func RenderCompactRemaining(w io.Writer, results []UsageResult) {
	fmt.Fprintln(w, CompactRemainingTitle(results))
}

// CompactRemainingTitle renders the "P-NN%" compact title used by
// `oct usage --compact`. It is intentionally always "remaining" regardless of
// the usage_display_mode setting -- that's the documented contract of the
// --compact flag ("Output compact remaining usage").
func CompactRemainingTitle(results []UsageResult) string {
	return CompactTitle(results, DisplayModeRemaining)
}

// CompactTitle renders the same "P-NN%" compact title for the given display
// mode. It backs both CompactRemainingTitle (always "remaining") and the
// menubar's compact title mode, which honors the user's usage_display_mode
// setting so the status-bar title never disagrees with the popover/dropdown
// body below it.
func CompactTitle(results []UsageResult, mode string) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, compactProviderLabel(result.Provider)+"-"+compactValueForMode(result, mode))
	}
	return strings.Join(parts, " ")
}

func compactProviderLabel(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch {
	case strings.Contains(p, "claude"):
		return "C"
	case strings.Contains(p, "commandcode"), strings.Contains(p, "command code"):
		return "D"
	case strings.Contains(p, "codex"), strings.Contains(p, "openai"):
		return "X"
	case strings.Contains(p, "antigravity"), strings.Contains(p, "gemini"), p == "agy":
		return "G"
	case strings.Contains(p, "cursor"):
		return "R"
	case strings.Contains(p, "copilot"), strings.Contains(p, "github"):
		return "P"
	case strings.Contains(p, "opencode"):
		return "O"
	}
	if p == "" {
		return "?"
	}
	return strings.ToUpper(string([]rune(p)[0]))
}

func compactValueForMode(r UsageResult, mode string) string {
	// "quota" (e.g. Copilot) is always a pre-computed used-percentage even
	// when r.Unit is a count unit like "AIC", not "percent" -- check it
	// before the percent-unit gate below, matching usageSummaryDisplay's
	// quota handling, or Copilot always renders "?" here while the `oct
	// usage` table and the Swift menubar show a real number for the exact
	// same data.
	if quota := strings.TrimSpace(r.Buckets["quota"]); quota != "" {
		if label, ok := compactPercentLabel(quota, mode); ok {
			return label
		}
	}
	if !strings.EqualFold(r.Unit, "percent") {
		return "?"
	}
	if raw, ok := compactUsageMetric(r); ok {
		if label, ok := compactPercentLabel(raw, mode); ok {
			return label
		}
	}
	if label, ok := compactPercentLabel(r.Used, mode); ok {
		return label
	}
	return "?"
}

func compactUsageMetric(r UsageResult) (string, bool) {
	if len(r.Buckets) == 0 {
		return "", false
	}
	provider := strings.ToLower(strings.TrimSpace(r.Provider))
	labels := []string{"5h", "7d"}
	if strings.Contains(provider, "codex") {
		labels = []string{"7d", "5h"}
	}
	for _, label := range labels {
		if value := strings.TrimSpace(r.Buckets[label]); value != "" {
			return value, true
		}
	}
	if modelParts := modelBucketDisplays(r, DisplayModeUsed); len(modelParts) > 0 {
		fields := strings.Fields(modelParts[0])
		if len(fields) > 0 {
			return fields[len(fields)-1], true
		}
	}
	return "", false
}

// compactPercentLabel formats a raw (always "used") bucket/used percentage as
// a bare "NN%" for the given mode. It is the only place that inverts used ->
// remaining for the compact title; callers must pass the raw stored value,
// never an already-formatted display string, or the inversion doubles up.
func compactPercentLabel(used string, mode string) (string, bool) {
	v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(used), "%"), 64)
	if err != nil {
		return "", false
	}
	value := v
	if mode == DisplayModeRemaining {
		value = 100 - v
	}
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return fmt.Sprintf("%.0f%%", value), true
}
