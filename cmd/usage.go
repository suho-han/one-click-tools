package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/notify"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

type usageModel struct {
	results      []usage.UsageResult
	err          error
	done         bool
	spinner      int
	orderedTools []update.Tool
	activeIdx    int
}

func (m usageModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			res, err := usageFetcher()
			if err != nil {
				return err
			}
			return res
		},
		tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
			return switchProviderMsg{}
		}),
	)
}

func (m usageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case []usage.UsageResult:
		m.results = msg
		m.done = true
		return m, tea.Quit
	case error:
		m.err = msg
		m.done = true
		return m, tea.Quit
	case spinnerMsg:
		m.spinner++
		return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return spinnerMsg{}
		})
	case switchProviderMsg:
		if len(m.orderedTools) > 0 {
			m.activeIdx = (m.activeIdx + 1) % len(m.orderedTools)
		}
		return m, tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
			return switchProviderMsg{}
		})
	}
	return m, nil
}

type spinnerMsg struct{}
type switchProviderMsg struct{}

var usageFetcher = usage.GetUsage

func shouldAutoJSONFallback(jsonMode bool, compactMode bool, isTTY bool) bool {
	return !jsonMode && !compactMode && !isTTY
}

func usageOrderedTools() []update.Tool {
	return usage.SelectedTools()
}

func maybeSendUsageAlerts(results []usage.UsageResult, force bool) {
	enabled := force || viper.GetBool("usage_alert_enabled")
	if !enabled {
		return
	}
	cfg := buildAlertConfigFromViper(true)
	_ = notify.MaybeSendUsageAlerts(results, cfg, time.Now())
}

func (m usageModel) View() string {
	if m.done {
		return ""
	}
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	s := spinners[m.spinner%len(spinners)]

	target := "AI providers"
	if len(m.orderedTools) > 0 {
		t := m.orderedTools[m.activeIdx]
		target = t.Colorize(t.Name)
	}

	return fmt.Sprintf("\n  %s Fetching usage data from %s...\n", s, target)
}

var usageCmd = &cobra.Command{
	Use:     "usage",
	GroupID: "core",
	Short:   "Show tool usage report",
	Long: `Show tool usage report for configured AI developer tools.

To properly fetch usage, ensure you are authenticated:
  - Antigravity: Run 'agy' once; usage is parsed from 'agy --print /usage'
  - Claude:  Run 'claude auth login'; fallback usage is parsed from 'claude --print /usage'
  - Command Code: Run 'commandcode login' or set COMMAND_CODE_API_KEY
  - Cursor CLI: Official CLI install is recommended; usage remains best-effort with local fallback
  - Copilot: Configure your token via 'oct config'
  - OpenCode: Reads usage from local session logs first (no API token)
  - Codex:   Automatically reads from local session logs

Legacy aliases 'gemini' and 'gemini-cli' still map to 'agy' for compatibility.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		compactMode, _ := cmd.Flags().GetBool("compact")
		notifyMode, _ := cmd.Flags().GetBool("notify")

		isTTY := false
		if fi, err := os.Stdout.Stat(); err == nil {
			isTTY = (fi.Mode() & os.ModeCharDevice) != 0
		}

		// Auto-fallback for non-TTY environments (CI, pipes, cron, tool runners)
		if shouldAutoJSONFallback(jsonMode, compactMode, isTTY) {
			jsonMode = true
			fmt.Fprintln(os.Stderr, "[oct] non-TTY detected -> switching to --json (pretty output)")
		}

		if jsonMode || compactMode {
			results, err := usageFetcher()
			if err != nil {
				return fmt.Errorf("fetch usage: %w", err)
			}
			maybeSendUsageAlerts(results, notifyMode)
			if compactMode {
				usage.RenderCompactRemaining(os.Stdout, results)
				return nil
			}
			if err := usage.PrintJSON(results); err != nil {
				return fmt.Errorf("print usage json: %w", err)
			}
			return nil
		}

		selectedTools := usageOrderedTools()

		m := usageModel{
			orderedTools: selectedTools,
		}
		p := tea.NewProgram(m)

		// Start spinner tick
		go func() {
			time.Sleep(100 * time.Millisecond)
			p.Send(spinnerMsg{})
		}()

		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("run usage ui: %w", err)
		}

		fm := finalModel.(usageModel)
		if fm.err != nil {
			return fmt.Errorf("fetch usage: %w", fm.err)
		}

		if len(fm.results) > 0 {
			maybeSendUsageAlerts(fm.results, notifyMode)
			usage.PrintTable(fm.results)
			fmt.Println("\nTip: Run 'oct usage --help' for authentication instructions.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.Flags().Bool("json", false, "Output in JSON format")
	usageCmd.Flags().Bool("compact", false, "Output compact remaining usage (C-45% X-25%)")
	usageCmd.Flags().Bool("notify", false, "Send usage alerts based on threshold/cooldown rules")
}
