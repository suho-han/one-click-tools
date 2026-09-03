package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/execenv"
)

type shellDoctorBinary struct {
	Name             string `json:"name"`
	RawResolvedPath  string `json:"raw_resolved_path,omitempty"`
	BootResolvedPath string `json:"boot_resolved_path,omitempty"`
}

type shellDoctorReport struct {
	RawPATH          string              `json:"raw_path"`
	BootstrappedPATH string              `json:"bootstrapped_path"`
	Binaries         []shellDoctorBinary `json:"binaries"`
}

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	GroupID: "maintenance",
	Short:   "Run environment diagnostics",
}

var doctorShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Compare raw vs bootstrapped PATH resolution",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		verbose, _ := cmd.Flags().GetBool("verbose")
		report := collectShellDoctorReport([]string{"oct", "gh", "brew", "claude", "commandcode", "codex", "copilot", "cursor-agent", "opencode", "agy"})
		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		printShellDoctorReport(cmd.OutOrStdout(), report, verbose)
		return nil
	},
}

func collectShellDoctorReport(names []string) shellDoctorReport {
	rawPath := normalizePathList(strings.TrimSpace(os.Getenv("PATH")))
	report := shellDoctorReport{
		RawPATH:          rawPath,
		BootstrappedPATH: normalizePathList(execenv.BuildPATH(rawPath)),
	}
	for _, name := range names {
		entry := shellDoctorBinary{Name: name}
		if raw, err := exec.LookPath(name); err == nil {
			entry.RawResolvedPath = raw
		}
		if boot, err := execenv.LookPath(name); err == nil {
			entry.BootResolvedPath = boot
		}
		report.Binaries = append(report.Binaries, entry)
	}
	return report
}

func printShellDoctorReport(w io.Writer, report shellDoctorReport, verbose bool) {
	okCount := 0
	missingRaw := 0
	missingBoot := 0
	changed := 0
	interesting := make([]shellDoctorBinary, 0, len(report.Binaries))
	for _, item := range report.Binaries {
		same := item.RawResolvedPath != "" && item.RawResolvedPath == item.BootResolvedPath
		switch {
		case same:
			okCount++
		case item.RawResolvedPath == "":
			missingRaw++
			interesting = append(interesting, item)
		case item.BootResolvedPath == "":
			missingBoot++
			interesting = append(interesting, item)
		default:
			changed++
			interesting = append(interesting, item)
		}
	}
	fmt.Fprintf(w, "shell doctor: ok=%d changed=%d missing_raw=%d missing_boot=%d\n", okCount, changed, missingRaw, missingBoot)
	fmt.Fprintf(w, "raw PATH : %s\n", compactPathForDisplay(report.RawPATH))
	fmt.Fprintf(w, "boot PATH: %s\n", compactPathForDisplay(report.BootstrappedPATH))
	if verbose {
		interesting = report.Binaries
	}
	if len(interesting) == 0 {
		fmt.Fprintln(w, "bins: all tracked binaries resolve identically")
		return
	}
	fmt.Fprintln(w, "bins:")
	for _, item := range interesting {
		raw := compactBinaryPath(item.RawResolvedPath)
		boot := compactBinaryPath(item.BootResolvedPath)
		if raw == boot {
			fmt.Fprintf(w, "- %s raw=%s boot=%s\n", item.Name, raw, boot)
			continue
		}
		fmt.Fprintf(w, "- %s raw=%s | boot=%s\n", item.Name, raw, boot)
	}
}

func normalizePathList(pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return ""
	}
	seen := map[string]struct{}{}
	parts := make([]string, 0)
	for _, part := range filepath.SplitList(pathValue) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cleaned := filepath.Clean(part)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		parts = append(parts, cleaned)
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func compactPathForDisplay(pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return "-"
	}
	parts := filepath.SplitList(pathValue)
	if len(parts) <= 6 {
		return pathValue
	}
	return strings.Join(parts[:4], string(os.PathListSeparator)) + string(os.PathListSeparator) + "…" + string(os.PathListSeparator) + strings.Join(parts[len(parts)-2:], string(os.PathListSeparator))
}

func compactBinaryPath(pathValue string) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return "-"
	}
	return pathValue
}

func init() {
	doctorShellCmd.Flags().Bool("json", false, "Output in JSON format")
	doctorShellCmd.Flags().Bool("verbose", false, "Show all tracked binaries, not only mismatches/missing ones")
	doctorCmd.AddCommand(doctorShellCmd)
	rootCmd.AddCommand(doctorCmd)
}
