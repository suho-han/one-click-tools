package usage

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func baseAntigravityUsageResult() UsageResult {
	return UsageResult{
		Provider:   "antigravity",
		Plan:       "unknown",
		PlanSource: "antigravity cli does not expose tier; see app settings",
		Period:     "current",
		Used:       "n/a",
		Limit:      "100",
		Unit:       "percent",
		Source:     "agy-cli",
		Status:     "warn",
		Message:    "No data: Antigravity usage unavailable (run agy and check /usage)",
	}
}

var antigravityUsageCommandOutput = commandOutput

func FetchAntigravityUsage() UsageResult {
	result := withPlanDetection(baseAntigravityUsageResult(), detectAntigravityPlan)
	if cliResult, ok := fetchAntigravityCLIUsage(result); ok {
		return cliResult
	}
	return result
}

func FetchGeminiUsage() UsageResult {
	return FetchAntigravityUsage()
}

type antigravityCLIUsageRow struct {
	Label     string
	Window    string
	Remaining float64
	ResetTime string
}

func fetchAntigravityCLIUsage(base UsageResult) (UsageResult, bool) {
	out, err := antigravityUsageCommandOutput(20*time.Second, "agy", "--print", "/usage")
	if err != nil || strings.TrimSpace(out) == "" {
		return base, false
	}

	rows := parseAntigravityCLIUsage(out)
	if len(rows) == 0 {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "agy-cli"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}
	result.Message = "Usage parsed from agy --print /usage"

	maxUsed := 0.0
	debugParts := make([]string, 0, len(rows))
	for _, row := range rows {
		used := percentUsedFromRemaining(row.Remaining)
		if used > maxUsed {
			maxUsed = used
		}
		label := antigravityCLIUsageBucketLabel(row.Label)
		key := "model:" + label
		result.Buckets[key] = fmt.Sprintf("%.1f", used)
		if strings.TrimSpace(row.ResetTime) != "" {
			result.BucketResets[key] = strings.TrimSpace(row.ResetTime)
		}
		debugParts = append(debugParts, fmt.Sprintf("%s %s remaining=%.1f reset=%s", label, row.Window, row.Remaining, row.ResetTime))
	}
	result.Used = fmt.Sprintf("%.1f", maxUsed)
	if len(result.BucketResets) == 0 {
		result.BucketResets = nil
	}
	if osDebugEnabled() {
		result.SourceDetail = strings.Join(debugParts, ";")
	}
	return result, true
}

func parseAntigravityCLIUsage(output string) []antigravityCLIUsageRow {
	rows := []antigravityCLIUsageRow{}
	for _, line := range strings.Split(output, "\n") {
		row, ok := parseAntigravityCLIUsageLine(line)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func parseAntigravityCLIUsageLine(line string) (antigravityCLIUsageRow, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 4 {
		return antigravityCLIUsageRow{}, false
	}

	limitIdx := -1
	for i := 0; i < len(fields)-3; i++ {
		if strings.EqualFold(fields[i], "Limit") && strings.EqualFold(fields[i+1], "Remaining") && strings.HasSuffix(fields[i+2], "%") {
			limitIdx = i
			break
		}
	}
	if limitIdx <= 0 {
		return antigravityCLIUsageRow{}, false
	}

	labelEnd := limitIdx
	window := ""
	if looksLikeAntigravityUsageWindow(fields[limitIdx-1]) {
		window = fields[limitIdx-1]
		labelEnd = limitIdx - 1
	}
	if labelEnd <= 0 {
		return antigravityCLIUsageRow{}, false
	}

	remaining, err := strconv.ParseFloat(strings.TrimSuffix(fields[limitIdx+2], "%"), 64)
	if err != nil || remaining < 0 || remaining > 100 {
		return antigravityCLIUsageRow{}, false
	}

	return antigravityCLIUsageRow{
		Label:     strings.Join(fields[:labelEnd], " "),
		Window:    window,
		Remaining: remaining,
		ResetTime: fields[limitIdx+3],
	}, true
}

func looksLikeAntigravityUsageWindow(value string) bool {
	s := strings.ToLower(strings.TrimSpace(value))
	if s == "daily" || s == "weekly" || s == "monthly" || s == "hourly" {
		return true
	}
	return strings.HasSuffix(s, "h") || strings.HasSuffix(s, "d") || strings.HasSuffix(s, "w") || strings.HasSuffix(s, "m")
}

func antigravityCLIUsageBucketLabel(label string) string {
	lower := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(lower, "claude") && strings.Contains(lower, "gpt"):
		return "Claude/GPT"
	case strings.Contains(lower, "gemini") || strings.Contains(lower, "google"):
		return "Gemini"
	default:
		return strings.TrimSpace(label)
	}
}

func percentUsedFromRemaining(remaining float64) float64 {
	used := 100 - remaining
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

func osDebugEnabled() bool {
	return strings.TrimSpace(os.Getenv("OCT_USAGE_DEBUG")) == "1"
}
