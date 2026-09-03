package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"golang.org/x/sync/errgroup"
)

// Display modes for the "used" vs "remaining" toggle (usage_display_mode
// config key). All UI surfaces (oct usage, oct monitor, both menubars) should
// resolve the raw config value through NormalizeDisplayMode so an invalid or
// missing value falls back the same way everywhere.
const (
	DisplayModeUsed      = "used"
	DisplayModeRemaining = "remaining"
)

// NormalizeDisplayMode canonicalizes a usage_display_mode value. Invalid or
// empty input falls back to "remaining", matching root.go's viper default,
// so a missing/corrupt config can't make one UI surface disagree with another.
func NormalizeDisplayMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode != DisplayModeUsed && mode != DisplayModeRemaining {
		return DisplayModeRemaining
	}
	return mode
}

type UsageResult struct {
	Provider     string            `json:"provider"`
	Plan         string            `json:"plan,omitempty"`
	PlanSource   string            `json:"plan_source,omitempty"`
	Period       string            `json:"period"`
	Used         string            `json:"used"`
	Limit        string            `json:"limit"`
	Unit         string            `json:"unit"`
	Source       string            `json:"source"`
	Status       string            `json:"status"`
	Message      string            `json:"message"`
	SourceDetail string            `json:"source_detail"`
	Buckets      map[string]string `json:"buckets"`                 // e.g. {"5h": "10", "7d": "20"}
	BucketResets map[string]string `json:"bucket_resets,omitempty"` // e.g. {"5h": "2026-07-22T06:09:59Z"}
}

func SelectedTools() []update.Tool {
	order := viper.GetStringSlice("agent_order")
	if len(order) == 0 {
		order = []string{"agy", "claude", "commandcode", "cursor-agent", "copilot", "opencode", "codex"}
	}
	enabledTools := viper.GetStringSlice("enabled_tools")
	orderedTools := update.GetOrderedTools(order)
	return update.GetFilteredTools(enabledTools, orderedTools)
}

func GetUsage() ([]UsageResult, error) {
	selectedTools := SelectedTools()

	fetchers := map[string]func() UsageResult{
		"agy":          FetchAntigravityUsage,
		"antigravity":  FetchAntigravityUsage,
		"gemini":       FetchAntigravityUsage,
		"claude":       FetchClaudeUsage,
		"commandcode":  FetchCommandCodeUsage,
		"cursor-agent": FetchCursorUsage,
		"copilot":      FetchCopilotUsage,
		"opencode":     FetchOpenCodeUsage,
		"codex":        FetchCodexUsage,
	}

	results := make([]UsageResult, len(selectedTools))
	g := new(errgroup.Group)

	for i, t := range selectedTools {
		i, t := i, t // Capture for goroutine
		if fetcher, ok := fetchers[strings.ToLower(t.BinaryName)]; ok {
			g.Go(func() error {
				results[i] = fetcher()
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Filter out empty results if any tools were skipped
	var filtered []UsageResult
	for _, r := range results {
		if r.Provider != "" {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

func PrintTable(results []UsageResult) {
	RenderTable(os.Stdout, results)
}

func RenderTable(w io.Writer, results []UsageResult) {
	width := terminalWidth()

	displayMode := NormalizeDisplayMode(viper.GetString("usage_display_mode"))

	cardWidth := tableCardWidth(width)
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		renderProviderCard(w, r, displayMode, cardWidth)
	}
}

func colorizeProvider(label string, provider string) string {
	p := strings.ToLower(provider)
	code := ""
	switch {
	case strings.Contains(p, "antigravity"), strings.Contains(p, "gemini"):
		code = "94"
	case strings.Contains(p, "claude"):
		code = "93"
	case strings.Contains(p, "commandcode"), strings.Contains(p, "command code"):
		code = "94"
	case strings.Contains(p, "codex"), strings.Contains(p, "openai"):
		code = "96"
	case strings.Contains(p, "copilot"), strings.Contains(p, "github"):
		code = "95"
	case strings.Contains(p, "cursor"):
		code = "94"
	case strings.Contains(p, "opencode"):
		code = "97"
	}
	if code == "" {
		return label
	}
	return "\x1b[1;" + code + "m" + label + "\x1b[0m"
}

func colorizeStatus(label string, status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "ok":
		return "\x1b[1;92m" + label + "\x1b[0m"
	case "warn":
		return "\x1b[1;93m" + label + "\x1b[0m"
	default:
		return "\x1b[1;91m" + label + "\x1b[0m"
	}
}

func colorizeMessage(message string, status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "warn":
		return "\x1b[93m" + message + "\x1b[0m"
	case "error":
		return "\x1b[91m" + message + "\x1b[0m"
	default:
		return message
	}
}

func providerDisplayLabel(provider string) string {
	if !supportsProviderIcons() {
		return provider
	}
	p := strings.ToLower(strings.TrimSpace(provider))
	switch {
	case strings.Contains(p, "antigravity"), strings.Contains(p, "gemini"):
		return "✨ " + provider
	case strings.Contains(p, "cursor"):
		return "▣ " + provider
	case strings.Contains(p, "opencode"):
		return "🧩 " + provider
	case strings.Contains(p, "commandcode"), strings.Contains(p, "command code"):
		return "⌘ " + provider
	default:
		return provider
	}
}

func supportsProviderIcons() bool {
	if isTruthy(os.Getenv("OCT_NO_ICONS")) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	locale := strings.ToUpper(strings.TrimSpace(os.Getenv("LC_ALL")))
	if locale == "" {
		locale = strings.ToUpper(strings.TrimSpace(os.Getenv("LC_CTYPE")))
	}
	if locale == "" {
		locale = strings.ToUpper(strings.TrimSpace(os.Getenv("LANG")))
	}
	return strings.Contains(locale, "UTF-8") || strings.Contains(locale, "UTF8")
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func padRightRunes(s string, width int) string {
	if utf8.RuneCountInString(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-utf8.RuneCountInString(s))
}

func terminalWidth() int {
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 40 {
			return n
		}
	}
	return 100
}

func tableMessageWidth(width int) int {
	switch {
	case width <= 80:
		return 10
	case width <= 100:
		return 20
	case width <= 120:
		return 28
	default:
		return 40
	}
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func tableCardWidth(width int) int {
	switch {
	case width < 56:
		return 56
	case width > 92:
		return 92
	default:
		return width
	}
}

func renderProviderCard(w io.Writer, r UsageResult, displayMode string, cardWidth int) {
	innerWidth := cardWidth - 2
	providerLabel := providerDisplayLabel(r.Provider)
	statusLabel := strings.ToUpper(strings.TrimSpace(r.Status))
	if statusLabel == "" {
		statusLabel = "UNKNOWN"
	}

	fmt.Fprintf(w, "╭%s╮\n", strings.Repeat("─", innerWidth))
	fmt.Fprintln(w, cardTitleLine(providerLabel, r.Provider, statusLabel, r.Status, innerWidth))
	fmt.Fprintf(w, "├%s┤\n", strings.Repeat("─", innerWidth))
	fmt.Fprintln(w, cardKeyValueLine("Plan", tablePlanLabel(r.Plan), innerWidth, ""))
	fmt.Fprintln(w, cardKeyValueLine("Quota", usageSummaryDisplay(r, displayMode), innerWidth, ""))
	fmt.Fprintln(w, cardKeyValueLine("Source", tableSourceLabel(r.Source), innerWidth, ""))
	if msg := strings.TrimSpace(r.Message); msg != "" {
		fmt.Fprintln(w, cardKeyValueLine("Note", truncateText(msg, innerWidth-10), innerWidth, r.Status))
	}
	fmt.Fprintf(w, "╰%s╯\n", strings.Repeat("─", innerWidth))
}

func cardTitleLine(providerLabel, provider, statusLabel, status string, innerWidth int) string {
	left := " " + providerLabel
	right := statusLabel + " "
	gap := innerWidth - utf8.RuneCountInString(left) - utf8.RuneCountInString(right)
	if gap < 1 {
		gap = 1
	}
	return "│" +
		colorizeProvider(left, provider) +
		strings.Repeat(" ", gap) +
		colorizeStatus(right, status) +
		"│"
}

func cardKeyValueLine(key, value string, innerWidth int, statusForValue string) string {
	keyLabel := fmt.Sprintf(" %-7s", key)
	value = strings.TrimSpace(value)
	if value == "" {
		value = "—"
	}
	maxValueWidth := innerWidth - utf8.RuneCountInString(keyLabel) - 1
	if maxValueWidth < 1 {
		maxValueWidth = 1
	}
	value = truncateText(value, maxValueWidth)
	padding := innerWidth - utf8.RuneCountInString(keyLabel) - 1 - utf8.RuneCountInString(value)
	if padding < 0 {
		padding = 0
	}
	if statusForValue != "" {
		value = colorizeMessage(value, statusForValue)
	}
	return "│" + keyLabel + " " + value + strings.Repeat(" ", padding) + "│"
}

func tableSourceLabel(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "—"
	}
	return source
}

func formatBucketDisplay(r UsageResult, rawValue, mode string) string {
	value := rawValue
	if mode == DisplayModeRemaining && strings.EqualFold(r.Unit, "percent") {
		if rem, ok := remainingFromUsed(rawValue); ok {
			value = rem
		}
	}

	if strings.EqualFold(r.Unit, "percent") {
		return value + "%"
	}
	// Antigravity alias compatibility keeps count-style numbers unmodified in table output.
	if strings.Contains(strings.ToLower(r.Provider), "antigravity") || strings.Contains(strings.ToLower(r.Provider), "gemini") {
		return value
	}
	return value
}

func remainingFromUsed(used string) (string, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(used), 64)
	if err != nil {
		return "", false
	}
	remaining := 100 - v
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("%.1f", remaining), true
}

func tablePlanLabel(plan string) string {
	plan = strings.TrimSpace(plan)
	if plan == "" || strings.EqualFold(plan, "unknown") || strings.EqualFold(plan, "n/a") {
		return "—"
	}
	return plan
}

func usageSummaryDisplay(r UsageResult, mode string) string {
	var parts []string
	if !strings.EqualFold(r.Provider, "codex") {
		if val, ok := visibleBucketValue(r, "5h", mode); ok {
			parts = append(parts, "5h "+val)
		}
	}
	if val, ok := visibleBucketValue(r, "7d", mode); ok {
		parts = append(parts, "7d "+val)
	}
	if val, ok := visibleBucketValue(r, "1m", mode); ok {
		parts = append(parts, "1m "+val)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " · ")
	}

	if modelParts := modelBucketDisplays(r, mode); len(modelParts) > 0 {
		return strings.Join(modelParts, " · ")
	}

	used := strings.TrimSpace(r.Used)
	msg := strings.ToLower(strings.TrimSpace(r.Message))
	if used == "" || strings.EqualFold(used, "n/a") || hasNoDataSignal(msg) {
		return "—"
	}
	if mode == DisplayModeRemaining && strings.EqualFold(r.Unit, "percent") {
		if rem, ok := remainingFromUsed(used); ok {
			used = rem
		}
	}
	if strings.EqualFold(r.Unit, "percent") {
		return used + "%"
	}
	unit := strings.TrimSpace(r.Unit)
	if unit == "" || strings.EqualFold(unit, "n/a") {
		return used
	}
	limit := strings.TrimSpace(r.Limit)
	if limit != "" && !strings.EqualFold(limit, "n/a") {
		if quota := strings.TrimSpace(r.Buckets["quota"]); quota != "" {
			// "quota" (e.g. Copilot) is always a pre-computed used-percentage
			// even though r.Unit is a count unit like "AIC", not "percent" --
			// it still has to honor the used/remaining toggle like every
			// other percent-based bucket, or it silently ignores the setting.
			return used + "/" + limit + " " + unit + " (" + quotaModeLabel(quota, mode) + ")"
		}
		return used + "/" + limit + " " + unit
	}
	return used + " " + unit
}

func quotaModeLabel(quota string, mode string) string {
	if mode == DisplayModeRemaining {
		if rem, ok := remainingFromUsed(quota); ok {
			return rem + "% left"
		}
	}
	return quota + "% used"
}

// BucketValue resolves a single bucket's display value for the given mode,
// applying the used/remaining inversion and "% left" labeling used by the
// `oct usage` card table. `oct monitor` and the legacy menubar's fixed-width
// surfaces intentionally use their own terser bucketVal (cmd/monitor.go)
// instead -- see its doc comment -- but both apply the identical numeric
// inversion, so the two never disagree on the underlying number, only on
// whether it's labeled.
// ok is false when there's no usable value for this bucket.
func BucketValue(r UsageResult, bucket string, mode string) (string, bool) {
	raw := ""
	if r.Buckets != nil {
		raw = strings.TrimSpace(r.Buckets[bucket])
	}
	if raw == "" || raw == "-" || strings.EqualFold(raw, "n/a") || strings.EqualFold(raw, "unavailable") {
		return "", false
	}
	value := formatBucketDisplay(r, raw, mode)
	if mode == DisplayModeRemaining && strings.EqualFold(r.Unit, "percent") {
		value += " left"
	}
	return value, true
}

func visibleBucketValue(r UsageResult, bucket string, mode string) (string, bool) {
	value, ok := BucketValue(r, bucket, mode)
	if !ok {
		return "", false
	}
	if resetLabel := bucketResetDisplay(r, bucket); resetLabel != "" {
		value += " (" + resetLabel + ")"
	}
	return value, true
}

func bucketResetDisplay(r UsageResult, bucket string) string {
	if r.BucketResets == nil {
		return ""
	}
	reset := strings.TrimSpace(r.BucketResets[bucket])
	if reset == "" {
		return ""
	}
	t, ok := parseBucketResetTime(reset)
	if !ok {
		return ""
	}
	d := time.Until(t)
	if d <= 0 {
		return "resets now"
	}
	return "resets in " + compactDuration(d)
}

func parseBucketResetTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if sec, err := strconv.ParseInt(raw, 10, 64); err == nil && sec > 0 {
		if sec > 10_000_000_000 {
			return time.UnixMilli(sec), true
		}
		return time.Unix(sec, 0), true
	}
	return time.Time{}, false
}

func compactDuration(d time.Duration) string {
	minutes := int(d.Round(time.Minute).Minutes())
	if minutes < 1 {
		return "<1m"
	}
	days := minutes / (24 * 60)
	hours := (minutes % (24 * 60)) / 60
	mins := minutes % 60
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	case hours > 0 && mins > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

func modelBucketDisplays(r UsageResult, mode string) []string {
	if len(r.Buckets) == 0 {
		return nil
	}
	keys := make([]string, 0, len(r.Buckets))
	for key := range r.Buckets {
		if strings.HasPrefix(key, "model:") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		value, ok := visibleBucketValue(r, key, mode)
		if !ok {
			continue
		}
		label := strings.TrimPrefix(key, "model:")
		if label == "" {
			label = "model"
		}
		out = append(out, label+" "+value)
	}
	return out
}

// FallbackMetricsSummary returns a display string for providers that don't
// populate the 5h/7d/1m buckets: Copilot's single pre-computed "quota"
// percentage, or Antigravity's per-model "model:*" buckets. ok is false when
// neither applies, meaning the caller has nothing to show here.
//
// This exists so every UI surface -- the `oct usage` table (via
// usageSummaryDisplay), the legacy menubar's provider line/details
// (cmd/menubar_state.go), and the Swift menubar's popover (mirrored in
// visibleMetrics) -- falls back to the same value instead of some surfaces
// showing real numbers while others show blank/dashed buckets for the exact
// same result.
func FallbackMetricsSummary(r UsageResult, mode string) (string, bool) {
	if quota := strings.TrimSpace(r.Buckets["quota"]); quota != "" {
		return "quota " + quotaModeLabel(quota, mode), true
	}
	if modelParts := modelBucketDisplays(r, mode); len(modelParts) > 0 {
		return strings.Join(modelParts, " · "), true
	}
	return "", false
}

func PrintJSON(results []UsageResult) error {
	type UsageResultCompact struct {
		Provider     string            `json:"provider"`
		Plan         string            `json:"plan,omitempty"`
		PlanSource   string            `json:"plan_source,omitempty"`
		Status       string            `json:"status"`
		Used         string            `json:"used"`
		Unit         string            `json:"unit"`
		Buckets      map[string]string `json:"buckets,omitempty"`
		BucketResets map[string]string `json:"bucket_resets,omitempty"`
		SourceDetail string            `json:"source_detail,omitempty"`
		Message      string            `json:"message,omitempty"`
	}
	type UsageSummary struct {
		Total int `json:"total"`
		OK    int `json:"ok"`
		Warn  int `json:"warn"`
		Error int `json:"error"`
	}
	payload := struct {
		Summary UsageSummary         `json:"summary"`
		Results []UsageResultCompact `json:"results"`
	}{
		Summary: UsageSummary{Total: len(results)},
		Results: make([]UsageResultCompact, 0, len(results)),
	}

	for _, r := range results {
		payload.Results = append(payload.Results, UsageResultCompact{
			Provider:     r.Provider,
			Plan:         r.Plan,
			PlanSource:   r.PlanSource,
			Status:       r.Status,
			Used:         r.Used,
			Unit:         r.Unit,
			Buckets:      r.Buckets,
			BucketResets: r.BucketResets,
			SourceDetail: r.SourceDetail,
			Message:      truncateText(r.Message, 48),
		})

		switch classifySummaryStatus(r) {
		case "ok":
			payload.Summary.OK++
		case "warn":
			payload.Summary.Warn++
		default:
			payload.Summary.Error++
		}
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func classifySummaryStatus(r UsageResult) string {
	status := strings.ToLower(strings.TrimSpace(r.Status))
	if status != "ok" {
		if status == "warn" {
			return "warn"
		}
		return "error"
	}

	used := strings.ToLower(strings.TrimSpace(r.Used))
	if used == "" || used == "n/a" {
		return "warn"
	}

	msg := strings.ToLower(strings.TrimSpace(r.Message))
	if hasNoDataSignal(msg) || hasPartialSignal(msg) {
		return "warn"
	}
	if used == "0" && hasNoDataSignal(msg) {
		return "warn"
	}

	return "ok"
}

func hasNoDataSignal(msg string) bool {
	return strings.HasPrefix(msg, "no data:") ||
		strings.HasPrefix(msg, "no ") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no configured") ||
		strings.Contains(msg, "no usage metrics")
}

func hasPartialSignal(msg string) bool {
	return strings.HasPrefix(msg, "partial:") || strings.Contains(msg, "partial data")
}
