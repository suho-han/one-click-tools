package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/notify"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

var alertCmd = &cobra.Command{
	Use:     "alert",
	GroupID: "manage",
	Short:   "Usage alert configuration and testing",
}

var alertConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage usage alert config",
}

var alertConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective usage alert config",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := buildAlertConfigFromViper(viper.GetBool("usage_alert_enabled"))
		payload, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(payload))
	},
}

var alertConfigSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set usage alert config value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := strings.TrimSpace(args[0])
		val := strings.TrimSpace(args[1])
		if err := setAlertConfigValue(key, val); err != nil {
			fmt.Println(err.Error())
			return
		}
		if err := persistViperConfig(); err != nil {
			fmt.Printf("failed to write config: %v\n", err)
			return
		}
		fmt.Println("alert config updated.")
	},
}

var alertConfigSetProviderThresholdCmd = &cobra.Command{
	Use:   "set-provider-threshold <window> <value>",
	Short: "Set provider threshold with interactive provider selection",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		window := strings.TrimSpace(args[0])
		value := strings.TrimSpace(args[1])
		provider, _ := cmd.Flags().GetString("provider")
		provider = strings.TrimSpace(strings.ToLower(provider))
		if provider == "" {
			picked, err := pickProviderInteractive(providerOptions())
			if err != nil {
				fmt.Printf("provider selection failed: %v\n", err)
				return
			}
			provider = picked
		}
		if provider == "" {
			fmt.Println("provider is required")
			return
		}
		if err := setAlertConfigValue(fmt.Sprintf("provider.%s.%s", provider, strings.ToLower(window)), value); err != nil {
			fmt.Println(err.Error())
			return
		}
		if err := persistViperConfig(); err != nil {
			fmt.Printf("failed to write config: %v\n", err)
			return
		}
		fmt.Printf("alert provider threshold updated: provider=%s window=%s value=%s\n", provider, strings.ToLower(window), value)
	},
}

var alertTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test usage alert decision with synthetic input",
	Run: func(cmd *cobra.Command, args []string) {
		provider, _ := cmd.Flags().GetString("provider")
		window, _ := cmd.Flags().GetString("window")
		value, _ := cmd.Flags().GetFloat64("value")
		quietNow, _ := cmd.Flags().GetBool("quiet-now")

		if provider == "" {
			provider = "codex"
		}
		if window == "" {
			window = "5h"
		}

		cfg := buildAlertConfigFromViper(true)
		statePath := viper.GetString("usage_alert_state_path")
		if statePath == "" {
			statePath = ""
		}
		cfg.StatePath = statePath

		r := usage.UsageResult{Provider: provider, Unit: "percent", Used: fmt.Sprintf("%.1f", value), Buckets: map[string]string{window: fmt.Sprintf("%.1f", value)}}
		now := time.Now()
		if quietNow {
			viper.Set("usage_alert_quiet_hours", "00:00-23:59")
			cfg.QuietHours = "00:00-23:59"
		}
		if err := notify.MaybeSendUsageAlerts([]usage.UsageResult{r}, cfg, now); err != nil {
			fmt.Printf("test failed: %v\n", err)
			return
		}

		threshold := cfg.ThresholdPct
		if t, ok := cfg.GlobalThresholds[window]; ok && t > 0 {
			threshold = t
		}
		if pm, ok := cfg.ProviderThreshold[strings.ToLower(provider)]; ok {
			if t, ok := pm[window]; ok && t > 0 {
				threshold = t
			} else if t, ok := pm["default"]; ok && t > 0 {
				threshold = t
			}
		}

		priority := alertPriorityLabel(value, threshold, cfg.CriticalPct)
		fmt.Printf("provider=%s window=%s value=%.1f threshold=%.1f priority=%s quiet_hours=%s critical=%.1f\n", provider, window, value, threshold, priority, cfg.QuietHours, cfg.CriticalPct)
		fmt.Println("test executed (notification may be suppressed by cooldown/quiet hours/snooze).")
	},
}

var alertSnoozeCmd = &cobra.Command{Use: "snooze", Short: "Manage alert snooze"}

var alertSnoozeSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set snooze duration",
	Run: func(cmd *cobra.Command, args []string) {
		duration, _ := cmd.Flags().GetDuration("duration")
		provider, _ := cmd.Flags().GetString("provider")
		window, _ := cmd.Flags().GetString("window")
		if duration <= 0 {
			fmt.Println("duration must be > 0, e.g. --duration 2h")
			return
		}
		statePath := getAlertStatePath()
		until := time.Now().Add(duration)
		if err := notify.SetSnooze(statePath, provider, window, until); err != nil {
			fmt.Printf("failed to set snooze: %v\n", err)
			return
		}
		fmt.Printf("snooze set key=%s until=%s\n", snoozeDisplayKey(provider, window), until.Format(time.RFC3339))
	},
}

var alertSnoozeShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show active snoozes",
	Run: func(cmd *cobra.Command, args []string) {
		statePath := getAlertStatePath()
		m, err := notify.GetSnooze(statePath)
		if err != nil {
			fmt.Printf("failed to load snooze: %v\n", err)
			return
		}
		if len(m) == 0 {
			fmt.Println("no active snooze")
			return
		}
		now := time.Now()
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			until := m[k]
			status := "expired"
			if now.Before(until) {
				status = "active"
			}
			fmt.Printf("%s -> %s (%s)\n", k, until.Format(time.RFC3339), status)
		}
	},
}

var alertSnoozeClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear snooze",
	Run: func(cmd *cobra.Command, args []string) {
		provider, _ := cmd.Flags().GetString("provider")
		window, _ := cmd.Flags().GetString("window")
		statePath := getAlertStatePath()
		if err := notify.ClearSnooze(statePath, provider, window); err != nil {
			fmt.Printf("failed to clear snooze: %v\n", err)
			return
		}
		fmt.Printf("snooze cleared key=%s\n", snoozeDisplayKey(provider, window))
	},
}

func setAlertConfigValue(key, val string) error {
	key = strings.TrimSpace(strings.ToLower(key))
	val = strings.TrimSpace(val)
	switch key {
	case "enabled":
		enabled, err := parseAlertBool(val)
		if err != nil {
			return err
		}
		viper.Set("usage_alert_enabled", enabled)
		return nil
	case "cooldown_minutes":
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid cooldown_minutes %q: must be a positive integer", val)
		}
		viper.Set("usage_alert_cooldown_minutes", n)
		return nil
	case "threshold_percent":
		f, err := parseAlertPercent("threshold_percent", val)
		if err != nil {
			return err
		}
		viper.Set("usage_alert_threshold_percent", f)
		return nil
	case "critical_percent":
		f, err := parseAlertPercent("critical_percent", val)
		if err != nil {
			return err
		}
		viper.Set("usage_alert_critical_percent", f)
		return nil
	case "quiet_hours":
		if err := validateAlertQuietHours(val); err != nil {
			return err
		}
		viper.Set("usage_alert_quiet_hours", val)
		return nil
	case "timezone":
		if strings.TrimSpace(val) != "" {
			if _, err := time.LoadLocation(val); err != nil {
				return fmt.Errorf("invalid timezone %q: %v", val, err)
			}
		}
		viper.Set("usage_alert_timezone", val)
		return nil
	}

	if strings.HasPrefix(key, "threshold.") {
		window := strings.TrimSpace(strings.TrimPrefix(key, "threshold."))
		if window == "" {
			return fmt.Errorf("invalid key: threshold.<window> expected")
		}
		f, err := parseAlertPercent("threshold", val)
		if err != nil {
			return err
		}
		viper.Set("usage_alert_thresholds."+strings.ToLower(window), f)
		return nil
	}

	if strings.HasPrefix(key, "provider.") {
		rest := strings.TrimPrefix(key, "provider.")
		parts := strings.Split(rest, ".")
		if len(parts) != 2 {
			return fmt.Errorf("invalid key: provider.<name>.<window|default> expected")
		}
		provider := strings.TrimSpace(parts[0])
		window := strings.TrimSpace(parts[1])
		if provider == "" || window == "" {
			return fmt.Errorf("invalid key: provider.<name>.<window|default> expected")
		}
		f, err := parseAlertPercent("provider threshold", val)
		if err != nil {
			return err
		}
		provider = strings.ToLower(provider)
		window = strings.ToLower(window)
		viper.Set("usage_alert_provider_thresholds."+provider+"."+window, f)
		return nil
	}

	return fmt.Errorf("supported keys: enabled, cooldown_minutes, threshold_percent, critical_percent, quiet_hours, timezone, threshold.<window>, provider.<name>.<window|default>")
}

func parseAlertBool(val string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid enabled value %q: use true or false", val)
	}
}

func parseAlertPercent(name, val string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %v", name, val, err)
	}
	if f <= 0 || f > 100 {
		return 0, fmt.Errorf("invalid %s %.1f: must be > 0 and <= 100", name, f)
	}
	return f, nil
}

func validateAlertQuietHours(val string) error {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}
	parts := strings.Split(val, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid quiet_hours %q: expected HH:MM-HH:MM", val)
	}
	if _, ok := parseAlertClockMinute(parts[0]); !ok {
		return fmt.Errorf("invalid quiet_hours %q: expected HH:MM-HH:MM", val)
	}
	if _, ok := parseAlertClockMinute(parts[1]); !ok {
		return fmt.Errorf("invalid quiet_hours %q: expected HH:MM-HH:MM", val)
	}
	return nil
}

func parseAlertClockMinute(val string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(val), ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, false
	}
	hour, errH := strconv.Atoi(parts[0])
	minute, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func providerOptions() []string {
	base := []string{"antigravity", "codex", "claude-code", "commandcode", "copilot", "cursor", "opencode"}
	seen := map[string]bool{}
	out := make([]string, 0, len(base)+4)
	for _, p := range base {
		seen[p] = true
		out = append(out, p)
	}
	for _, et := range viper.GetStringSlice("enabled_tools") {
		p := update.NormalizeToolName(et)
		switch p {
		case "agy":
			p = "antigravity"
		case "cursor-agent":
			p = "cursor"
		case "claude":
			p = "claude-code"
		case "commandcode":
			p = "commandcode"
		}
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func pickProviderInteractive(options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	selector := promptui.Select{
		Label: "Select provider",
		Items: options,
		Size:  len(options),
	}

	idx, value, err := selector.Run()
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(options) {
		return "", fmt.Errorf("invalid selection")
	}
	return value, nil
}

func getAlertStatePath() string {
	p := strings.TrimSpace(viper.GetString("usage_alert_state_path"))
	if p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".oct/state/usage-alert-state.json"
	}
	return filepath.Join(home, ".oct", "state", "usage-alert-state.json")
}

func persistViperConfig() error {
	cfg := viper.ConfigFileUsed()
	if strings.TrimSpace(cfg) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cfg = filepath.Join(home, ".oct", "config.yaml")
	}
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(cfg); err == nil {
		return viper.WriteConfigAs(cfg)
	}
	return viper.SafeWriteConfigAs(cfg)
}

func snoozeDisplayKey(provider, window string) string {
	provider = strings.TrimSpace(provider)
	window = strings.TrimSpace(window)
	if provider == "" && window == "" {
		return "global"
	}
	if provider != "" && window == "" {
		return "provider:" + provider
	}
	if provider == "" {
		return "window:" + window
	}
	return "provider:" + provider + ":window:" + window
}

func alertPriorityLabel(value, threshold, critical float64) string {
	if value >= critical {
		return "CRITICAL"
	}
	if value >= threshold {
		return "HIGH"
	}
	return "NORMAL"
}

func parseThresholdMap(raw map[string]any) map[string]float64 {
	out := map[string]float64{}
	for k, v := range raw {
		switch x := v.(type) {
		case int:
			out[strings.ToLower(k)] = float64(x)
		case int64:
			out[strings.ToLower(k)] = float64(x)
		case float64:
			out[strings.ToLower(k)] = x
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
				out[strings.ToLower(k)] = f
			}
		}
	}
	return out
}

func parseProviderThresholdMap(raw map[string]any) map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	for provider, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		out[strings.ToLower(provider)] = parseThresholdMap(m)
	}
	return out
}

func buildAlertConfigFromViper(enabled bool) notify.UsageAlertConfig {
	cfg := notify.UsageAlertConfig{
		Enabled:           enabled,
		ThresholdPct:      viper.GetFloat64("usage_alert_threshold_percent"),
		CooldownMinutes:   viper.GetInt("usage_alert_cooldown_minutes"),
		StatePath:         viper.GetString("usage_alert_state_path"),
		QuietHours:        viper.GetString("usage_alert_quiet_hours"),
		Timezone:          viper.GetString("usage_alert_timezone"),
		CriticalPct:       viper.GetFloat64("usage_alert_critical_percent"),
		GlobalThresholds:  parseThresholdMap(viper.GetStringMap("usage_alert_thresholds")),
		ProviderThreshold: parseProviderThresholdMap(viper.GetStringMap("usage_alert_provider_thresholds")),
	}
	if cfg.GlobalThresholds == nil {
		cfg.GlobalThresholds = map[string]float64{}
	}
	if _, ok := cfg.GlobalThresholds["default"]; !ok && cfg.ThresholdPct > 0 {
		cfg.GlobalThresholds["default"] = cfg.ThresholdPct
	}
	return cfg
}

func init() {
	rootCmd.AddCommand(alertCmd)
	alertCmd.AddCommand(alertConfigCmd)
	alertCmd.AddCommand(alertTestCmd)
	alertCmd.AddCommand(alertSnoozeCmd)
	alertConfigCmd.AddCommand(alertConfigShowCmd)
	alertConfigCmd.AddCommand(alertConfigSetCmd)
	alertConfigCmd.AddCommand(alertConfigSetProviderThresholdCmd)
	alertSnoozeCmd.AddCommand(alertSnoozeSetCmd)
	alertSnoozeCmd.AddCommand(alertSnoozeShowCmd)
	alertSnoozeCmd.AddCommand(alertSnoozeClearCmd)

	alertTestCmd.Flags().String("provider", "codex", "provider name")
	alertTestCmd.Flags().String("window", "5h", "window key (e.g. 5h, 7d, current)")
	alertTestCmd.Flags().Float64("value", 90, "synthetic usage value percent")
	alertTestCmd.Flags().Bool("quiet-now", false, "force quiet-hours simulation")
	alertConfigSetProviderThresholdCmd.Flags().String("provider", "", "provider name (empty = interactive select)")

	alertSnoozeSetCmd.Flags().Duration("duration", 2*time.Hour, "snooze duration")
	alertSnoozeSetCmd.Flags().String("provider", "", "provider scope")
	alertSnoozeSetCmd.Flags().String("window", "", "window scope")
	alertSnoozeClearCmd.Flags().String("provider", "", "provider scope")
	alertSnoozeClearCmd.Flags().String("window", "", "window scope")
}
