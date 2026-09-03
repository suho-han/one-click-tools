package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// execCommand is a variable to allow testing with mocked commands
var execCommand = exec.Command
var claudeUsageCommandOutput = commandOutput

type claudeOAuthToken struct {
	AccessToken           string   `json:"accessToken"`
	RefreshToken          string   `json:"refreshToken"`
	ExpiresAt             int64    `json:"expiresAt"`
	RefreshTokenExpiresAt int64    `json:"refreshTokenExpiresAt"`
	Scopes                []string `json:"scopes"`
	SubscriptionType      string   `json:"subscriptionType"`
	RateLimitTier         string   `json:"rateLimitTier"`
}

func FetchClaudeUsage() UsageResult {
	home, _ := os.UserHomeDir()
	credsFile := filepath.Join(home, ".claude", ".credentials.json")

	result := UsageResult{
		Provider:   "claude-code",
		Plan:       "unknown",
		PlanSource: "claude plan not exposed",
		Period:     "current",
		Used:       "n/a",
		Limit:      "100",
		Unit:       "percent",
		Source:     "cli",
		Status:     "error",
	}

	var token string
	var keychainExpired bool
	var keychainCanRefresh bool
	var keychainSubscription string

	// Try macOS Keychain first for Claude Code-credentials
	cmd := execCommand("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		var keychainCreds struct {
			ClaudeAiOauth claudeOAuthToken `json:"claudeAiOauth"`
		}
		if json.Unmarshal(out, &keychainCreds) == nil {
			oauth := keychainCreds.ClaudeAiOauth
			keychainSubscription = oauth.SubscriptionType

			if oauth.AccessToken != "" {
				// Check if token is expired
				if oauth.ExpiresAt > 0 && time.Now().UnixMilli() > oauth.ExpiresAt {
					keychainExpired = true
					if oauth.RefreshTokenExpiresAt > 0 && time.Now().UnixMilli() < oauth.RefreshTokenExpiresAt {
						keychainCanRefresh = true
					}
				} else if oauth.ExpiresAt == 0 {
					// expiresAt=0 means token is invalid/expired
					keychainExpired = true
				} else {
					token = oauth.AccessToken
				}
			} else {
				// accessToken is empty but entry exists
				keychainExpired = true
				if oauth.RefreshTokenExpiresAt > 0 && time.Now().UnixMilli() < oauth.RefreshTokenExpiresAt {
					keychainCanRefresh = true
				}
			}
		}
	}

	// Fallback to credentials file if no valid token yet
	if token == "" {
		if _, err := os.Stat(credsFile); err == nil {
			data, _ := os.ReadFile(credsFile)
			var creds struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.Unmarshal(data, &creds); err == nil && creds.AccessToken != "" {
				token = creds.AccessToken
			}
		}
	}

	// Fallback to environment variable
	if token == "" {
		token = os.Getenv("CLAUDE_API_TOKEN")
	}

	// Extract plan info from Keychain even if token is expired
	if keychainSubscription != "" {
		result.Plan = keychainSubscription
		result.PlanSource = "keychain subscriptionType"
	}

	// Handle expired/missing token cases
	if token == "" {
		if keychainExpired {
			if keychainCanRefresh {
				result.Status = "warn"
				result.Used = "0"
				result.Message = "Claude OAuth token expired. Run 'claude auth login' to refresh"
				result.SourceDetail = "token_expired=true;can_refresh=true"
			} else {
				result.Status = "error"
				result.Used = "n/a"
				result.Message = "Claude OAuth token expired and cannot be refreshed. Run 'claude auth login'"
				result.SourceDetail = "token_expired=true;can_refresh=false"
			}
		} else {
			// No token found at all
			result.Status = "ok"
			result.Used = "0"
			result.Message = "No Claude OAuth token found (check ~/.claude/.credentials.json or CLAUDE_API_TOKEN)"
		}
		if cliResult, ok := fetchClaudeCLIUsage(result, "oauth token unavailable"); ok {
			return cliResult
		}
		return result
	}

	plan, source := detectClaudePlan(token)
	result = withPlan(result, plan, source)

	endpoint := "https://api.anthropic.com/api/oauth/usage"
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		result.Message = netclient.FormatError(resp, err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			if cached, ok := fetchClaudeCachedUsage(result, home, "API rate limited"); ok {
				return cached
			}
			result.Status = "warn"
			result.Used = "n/a"
			result.Message = "Claude usage API rate limited and no local cached usage found"
			if os.Getenv("OCT_USAGE_DEBUG") == "1" {
				result.SourceDetail = "http_status=429;cache=missing"
			}
			return result
		}
		if resp.StatusCode == http.StatusUnauthorized {
			if cliResult, ok := fetchClaudeCLIUsage(result, "OAuth API unauthorized"); ok {
				return cliResult
			}
			result.Status = "error"
			result.Message = "Claude OAuth token unauthorized. Run 'claude auth login'"
			result.SourceDetail = fmt.Sprintf("http_status=%d", resp.StatusCode)
			return result
		}
		result.Message = netclient.FormatError(resp, nil)
		return result
	}

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    string  `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    string  `json:"resets_at"`
		} `json:"seven_day"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		result.Message = "Failed to parse API response"
		return result
	}

	result.Buckets = make(map[string]string)
	result.BucketResets = make(map[string]string)
	if data.FiveHour.Utilization > 0 {
		result.Buckets["5h"] = fmt.Sprintf("%.1f", data.FiveHour.Utilization)
		if data.FiveHour.ResetsAt != "" {
			result.BucketResets["5h"] = data.FiveHour.ResetsAt
		}
	}
	if data.SevenDay.Utilization > 0 {
		result.Buckets["7d"] = fmt.Sprintf("%.1f", data.SevenDay.Utilization)
		if data.SevenDay.ResetsAt != "" {
			result.BucketResets["7d"] = data.SevenDay.ResetsAt
		}
	}
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = fmt.Sprintf("five_hour=%.1f;seven_day=%.1f", data.FiveHour.Utilization, data.SevenDay.Utilization)
	}

	if data.FiveHour.Utilization > 0 {
		result.Used = fmt.Sprintf("%.1f", data.FiveHour.Utilization)
		result.Message = "Usage fetched from Anthropic OAuth API (5h bucket)"
	} else if data.SevenDay.Utilization > 0 {
		result.Used = fmt.Sprintf("%.1f", data.SevenDay.Utilization)
		result.Message = "Usage fetched from Anthropic OAuth API (7d bucket)"
	} else {
		if cliResult, ok := fetchClaudeCLIUsage(result, "API reported no utilization"); ok {
			return cliResult
		}
		result.Used = "0"
		result.Message = "No utilization reported by API"
	}

	result.Status = "ok"
	result.Source = "oauth"
	return result
}

type claudeCLIUsageWindow struct {
	Bucket  string
	Used    float64
	ResetAt string
}

func fetchClaudeCLIUsage(base UsageResult, reason string) (UsageResult, bool) {
	out, err := claudeUsageCommandOutput(20*time.Second, "claude", "--print", "/usage", "--output-format", "json")
	if err != nil || strings.TrimSpace(out) == "" {
		return base, false
	}
	windows := parseClaudeCLIUsage(out)
	if len(windows) == 0 {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "claude-cli"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}
	maxUsed := 0.0
	for _, window := range windows {
		result.Buckets[window.Bucket] = fmt.Sprintf("%.1f", window.Used)
		if window.ResetAt != "" {
			result.BucketResets[window.Bucket] = window.ResetAt
		}
		if window.Used > maxUsed {
			maxUsed = window.Used
		}
	}
	if len(result.BucketResets) == 0 {
		result.BucketResets = nil
	}
	result.Used = fmt.Sprintf("%.1f", maxUsed)
	result.Message = "Usage parsed from claude --print /usage"
	if strings.TrimSpace(reason) != "" {
		result.Message += " (" + reason + ")"
	}
	return result, true
}

func parseClaudeCLIUsage(output string) []claudeCLIUsageWindow {
	text := claudeCLIUsageText(output)
	windows := []claudeCLIUsageWindow{}
	for _, line := range strings.Split(text, "\n") {
		if window, ok := parseClaudeCLIUsageLine(line); ok {
			windows = append(windows, window)
		}
	}
	return windows
}

func claudeCLIUsageText(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var payload struct {
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &payload); err == nil && strings.TrimSpace(payload.Result) != "" {
			return payload.Result
		}
	}
	return output
}

func parseClaudeCLIUsageLine(line string) (claudeCLIUsageWindow, bool) {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	bucket := ""
	switch {
	case strings.HasPrefix(lower, "current session:"):
		bucket = "5h"
	case strings.HasPrefix(lower, "current week"):
		bucket = "7d"
	default:
		return claudeCLIUsageWindow{}, false
	}

	usedMarker := "% used"
	usedIdx := strings.Index(lower, usedMarker)
	if usedIdx < 0 {
		return claudeCLIUsageWindow{}, false
	}
	start := usedIdx - 1
	for start >= 0 {
		ch := trimmed[start]
		if (ch < '0' || ch > '9') && ch != '.' {
			break
		}
		start--
	}
	usedRaw := strings.TrimSpace(trimmed[start+1 : usedIdx])
	used, err := strconv.ParseFloat(usedRaw, 64)
	if err != nil || used < 0 || used > 100 {
		return claudeCLIUsageWindow{}, false
	}

	resetAt := ""
	if resetIdx := strings.Index(lower[usedIdx+len(usedMarker):], "resets "); resetIdx >= 0 {
		resetRaw := strings.TrimSpace(trimmed[usedIdx+len(usedMarker)+resetIdx+len("resets "):])
		if parsed := parseClaudeCLIResetTime(resetRaw); !parsed.IsZero() {
			resetAt = parsed.Format(time.RFC3339)
		}
	}

	return claudeCLIUsageWindow{
		Bucket:  bucket,
		Used:    used,
		ResetAt: resetAt,
	}, true
}

func parseClaudeCLIResetTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}

	loc := time.Local
	if open := strings.LastIndex(raw, "("); open >= 0 && strings.HasSuffix(raw, ")") {
		name := strings.TrimSpace(strings.TrimSuffix(raw[open+1:], ")"))
		if loaded, err := time.LoadLocation(name); err == nil {
			loc = loaded
		}
		raw = strings.TrimSpace(raw[:open])
	}

	now := time.Now().In(loc)
	withYear := fmt.Sprintf("%d %s", now.Year(), raw)
	for _, layout := range []string{"2006 Jan 2 at 3:04pm", "2006 Jan 2 at 3pm"} {
		parsed, err := time.ParseInLocation(layout, withYear, loc)
		if err != nil {
			continue
		}
		if parsed.Before(now.AddDate(0, -6, 0)) {
			parsed = parsed.AddDate(1, 0, 0)
		}
		return parsed
	}
	return time.Time{}
}

type claudeCachedUsageFile struct {
	CachedUsageUtilization struct {
		FetchedAtMs int64 `json:"fetchedAtMs"`
		Utilization struct {
			FiveHour *claudeCachedUsageWindow `json:"five_hour"`
			SevenDay *claudeCachedUsageWindow `json:"seven_day"`
		} `json:"utilization"`
	} `json:"cachedUsageUtilization"`
}

type claudeCachedUsageWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

func fetchClaudeCachedUsage(base UsageResult, home string, reason string) (UsageResult, bool) {
	data, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return base, false
	}
	var cached claudeCachedUsageFile
	if err := json.Unmarshal(data, &cached); err != nil {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "cache"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}

	fiveHour := cached.CachedUsageUtilization.Utilization.FiveHour
	sevenDay := cached.CachedUsageUtilization.Utilization.SevenDay
	if fiveHour != nil && fiveHour.Utilization != nil {
		result.Buckets["5h"] = fmt.Sprintf("%.1f", *fiveHour.Utilization)
		if fiveHour.ResetsAt != "" {
			result.BucketResets["5h"] = fiveHour.ResetsAt
		}
	}
	if sevenDay != nil && sevenDay.Utilization != nil {
		result.Buckets["7d"] = fmt.Sprintf("%.1f", *sevenDay.Utilization)
		if sevenDay.ResetsAt != "" {
			result.BucketResets["7d"] = sevenDay.ResetsAt
		}
	}

	if result.Buckets["5h"] != "" {
		result.Used = result.Buckets["5h"]
	} else if result.Buckets["7d"] != "" {
		result.Used = result.Buckets["7d"]
	} else {
		return base, false
	}

	result.Message = "Cached Claude usage from ~/.claude.json"
	if strings.TrimSpace(reason) != "" {
		result.Message += " (" + reason + ")"
	}
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		details := []string{
			"cache_fetched_at_ms=" + fmt.Sprintf("%d", cached.CachedUsageUtilization.FetchedAtMs),
		}
		if fiveHour != nil && fiveHour.ResetsAt != "" {
			details = append(details, "5h_resets_at="+fiveHour.ResetsAt)
		}
		if sevenDay != nil && sevenDay.ResetsAt != "" {
			details = append(details, "7d_resets_at="+sevenDay.ResetsAt)
		}
		result.SourceDetail = strings.Join(details, ";")
	}
	return result, true
}
