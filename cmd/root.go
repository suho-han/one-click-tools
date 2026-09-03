package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/config"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "oct",
	Short:   "One-Click Tools for AI Engineers",
	Long:    `A high-performance CLI tool to manage and update AI-related command-line tools across different platforms.`,
	Version: "0.1.5",
}

func Execute() {
	reorderRootCommands()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.EnableCommandSorting = false
	cobra.OnInitialize(initConfig)

	rootCmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands (frequently used)"},
		&cobra.Group{ID: "manage", Title: "Configuration & Scheduling"},
		&cobra.Group{ID: "maintenance", Title: "Update & Maintenance"},
	)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.oct/config.yaml)")
}

func reorderRootCommands() {
	preferred := []string{"usage", "monitor", "menubar", "config", "alert", "schedule", "agent-update", "session-refresh", "release-doctor", "doctor", "update", "help", "completion"}
	current := rootCmd.Commands()
	if len(current) == 0 {
		return
	}

	byName := make(map[string]*cobra.Command, len(current))
	for _, c := range current {
		byName[c.Name()] = c
	}

	for _, c := range current {
		rootCmd.RemoveCommand(c)
	}

	added := make(map[string]bool, len(current))
	for _, name := range preferred {
		if c, ok := byName[name]; ok {
			rootCmd.AddCommand(c)
			added[name] = true
		}
	}

	for _, c := range current {
		if !added[c.Name()] {
			rootCmd.AddCommand(c)
		}
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home + "/.oct")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetDefault("icon_style", "braille")
	viper.SetDefault("usage_display_mode", "remaining")
	viper.SetDefault("usage_alert_enabled", false)
	viper.SetDefault("usage_alert_threshold_percent", 80.0)
	viper.SetDefault("usage_alert_critical_percent", 98.0)
	viper.SetDefault("usage_alert_cooldown_minutes", 360)
	viper.SetDefault("usage_alert_quiet_hours", "")
	viper.SetDefault("usage_alert_timezone", "")
	viper.SetDefault("usage_alert_thresholds", map[string]float64{"default": 80})
	viper.SetDefault("session_refresh_enabled", false)
	viper.SetDefault("session_refresh_interval", "daily")
	viper.SetDefault("session_refresh_hour", 9)
	viper.SetDefault("menubar_refresh_interval", "1m")
	viper.SetDefault("menubar_title_mode", "oct")
	// Avoid accidental overrides from generic env vars like ENABLED_TOOLS.
	// Require explicit OCT_* variables (e.g., OCT_ENABLED_TOOLS) for env-based overrides.
	viper.SetEnvPrefix("OCT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		return
	} else {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			if cfgFile != "" {
				fmt.Fprintf(os.Stderr, "failed to read config file %q: %v\n", cfgFile, err)
				os.Exit(1)
			}
			if migrateErr := config.MigrateLegacyConfig(); migrateErr != nil {
				fmt.Fprintf(os.Stderr, "failed to migrate legacy config: %v\n", migrateErr)
				os.Exit(1)
			}
			return
		}

		configPath := cfgFile
		if configPath == "" {
			configPath = viper.ConfigFileUsed()
		}
		if configPath == "" {
			configPath = "$HOME/.oct/config.yaml"
		}
		fmt.Fprintf(os.Stderr, "failed to load config %s: %v\n", configPath, err)
		os.Exit(1)
	}
}
