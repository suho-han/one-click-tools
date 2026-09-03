package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// mockSecurityCommand replaces the security command with a test function
func mockSecurityCommand(t *testing.T, mockOutput string) func() {
	t.Helper()
	origCommand := exec.Command
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "security" {
			cs := []string{"-test.run=TestSecurityHelper", "--"}
			cs = append(cs, args...)
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = []string{"GO_WANT_SECURITY_OUTPUT=" + mockOutput}
			return cmd
		}
		return origCommand(name, args...)
	}
	return func() { execCommand = origCommand }
}

// TestSecurityHelper is used by mockSecurityCommand
func TestSecurityHelper(t *testing.T) {
	if mockOutput := os.Getenv("GO_WANT_SECURITY_OUTPUT"); mockOutput != "" {
		fmt.Print(mockOutput)
		os.Exit(0)
	}
}

func mockClaudeUsageCommand(t *testing.T, output string, err error) func() {
	t.Helper()
	orig := claudeUsageCommandOutput
	claudeUsageCommandOutput = func(timeout time.Duration, name string, args ...string) (string, error) {
		if timeout != 20*time.Second {
			t.Fatalf("timeout = %v, want 20s", timeout)
		}
		if name != "claude" {
			t.Fatalf("command = %q, want claude", name)
		}
		want := []string{"--print", "/usage", "--output-format", "json"}
		if len(args) != len(want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("args = %v, want %v", args, want)
			}
		}
		return output, err
	}
	return func() { claudeUsageCommandOutput = orig }
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchClaudeUsageBuckets(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries

	t.Setenv("CLAUDE_API_TOKEN", "dummy-token")

	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.anthropic.com/api/oauth/usage" {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			if got := req.Header.Get("Authorization"); got == "" || !strings.HasPrefix(got, "Bearer ") {
				t.Fatalf("missing or invalid auth header: %s", got)
			}

			body := `{"five_hour":{"utilization":42.5},"seven_day":{"utilization":77.7}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	result := FetchClaudeUsage()

	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %s (message=%s)", result.Status, result.Message)
	}
	if result.Used != "42.5" {
		t.Fatalf("expected used=42.5 from 5h bucket, got %s", result.Used)
	}
	if result.Buckets == nil {
		t.Fatalf("expected buckets map to be populated")
	}
	if got := result.Buckets["5h"]; got != "42.5" {
		t.Fatalf("expected 5h bucket 42.5, got %s", got)
	}
	if got := result.Buckets["7d"]; got != "77.7" {
		t.Fatalf("expected 7d bucket 77.7, got %s", got)
	}
}

func TestFetchClaudeUsageFallsBackToCLIWhenAPINoUtilization(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	t.Setenv("CLAUDE_API_TOKEN", "dummy-token")
	cliOutput := `{"result":"You are currently using your subscription to power your Claude Code usage\n\nCurrent session: 12% used · resets Sep 4 at 7:40am (Asia/Seoul)\nCurrent week (all models): 34.5% used · resets Sep 5 at 8am (Asia/Seoul)"}`
	restoreCLI := mockClaudeUsageCommand(t, cliOutput, nil)
	defer restoreCLI()

	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"five_hour":{"utilization":0},"seven_day":{"utilization":0}}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0

	result := FetchClaudeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %s", result.Status)
	}
	if result.Source != "claude-cli" {
		t.Fatalf("expected claude-cli source, got %s", result.Source)
	}
	if result.Used != "34.5" {
		t.Fatalf("expected used=34.5 from max CLI bucket, got %s", result.Used)
	}
	if got := result.Buckets["5h"]; got != "12.0" {
		t.Fatalf("expected 5h bucket 12.0, got %s", got)
	}
	if got := result.Buckets["7d"]; got != "34.5" {
		t.Fatalf("expected 7d bucket 34.5, got %s", got)
	}
}

func TestFetchClaudeUsageRateLimitedBuckets(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries

	t.Setenv("CLAUDE_API_TOKEN", "dummy-token")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	cache := fmt.Sprintf(`{
		"cachedUsageUtilization": {
			"fetchedAtMs": 1784687082111,
			"utilization": {
				"five_hour": {"utilization": 12, "resets_at": "2026-07-22T06:09:59Z"},
				"seven_day": {"utilization": 41, "resets_at": "2026-07-24T22:59:59Z"}
			}
		}
	}`)
	if err := os.WriteFile(filepath.Join(tmp, ".claude.json"), []byte(cache), 0o600); err != nil {
		t.Fatalf("write cache failed: %v", err)
	}

	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	result := FetchClaudeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %s", result.Status)
	}
	if result.Source != "cache" {
		t.Fatalf("expected cache source, got %s", result.Source)
	}
	if result.Used != "12.0" {
		t.Fatalf("expected used=12.0 from cache, got %s", result.Used)
	}
	if got := result.Buckets["5h"]; got != "12.0" {
		t.Fatalf("expected 5h bucket 12.0, got %s", got)
	}
	if got := result.Buckets["7d"]; got != "41.0" {
		t.Fatalf("expected 7d bucket 41.0, got %s", got)
	}
}

func TestFetchClaudeUsage_ExpiredTokenWithRefreshAvailable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Mock Keychain with expired token but valid refresh token
	expiredCreds := map[string]interface{}{
		"claudeAiOauth": map[string]interface{}{
			"accessToken":           "",
			"refreshToken":          "valid-refresh-token",
			"expiresAt":             time.Now().Add(-24 * time.Hour).UnixMilli(),
			"refreshTokenExpiresAt": time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
			"subscriptionType":      "pro",
		},
	}
	credsJSON, _ := json.Marshal(expiredCreds)

	// Mock the security command
	restore := mockSecurityCommand(t, string(credsJSON))
	defer restore()
	restoreCLI := mockClaudeUsageCommand(t, "", errors.New("claude unavailable"))
	defer restoreCLI()

	result := FetchClaudeUsage()

	if result.Status != "warn" {
		t.Fatalf("expected status warn for expired token, got %s", result.Status)
	}
	if result.Used != "0" {
		t.Fatalf("expected used=0 for expired token, got %s", result.Used)
	}
	if !strings.Contains(result.Message, "expired") {
		t.Fatalf("expected message to contain 'expired', got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "claude auth login") {
		t.Fatalf("expected message to suggest 'claude auth login', got: %s", result.Message)
	}
	// Should still detect subscription type
	if result.Plan != "pro" {
		t.Fatalf("expected plan=pro from keychain, got %s", result.Plan)
	}
	if !strings.Contains(result.PlanSource, "keychain") {
		t.Fatalf("expected planSource to indicate keychain, got %s", result.PlanSource)
	}
}

func TestFetchClaudeUsage_ExpiredTokenWithoutRefresh(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Mock Keychain with expired token and expired refresh token
	expiredCreds := map[string]interface{}{
		"claudeAiOauth": map[string]interface{}{
			"accessToken":           "",
			"refreshToken":          "expired-refresh-token",
			"expiresAt":             time.Now().Add(-24 * time.Hour).UnixMilli(),
			"refreshTokenExpiresAt": time.Now().Add(-1 * time.Hour).UnixMilli(),
		},
	}
	credsJSON, _ := json.Marshal(expiredCreds)

	restore := mockSecurityCommand(t, string(credsJSON))
	defer restore()
	restoreCLI := mockClaudeUsageCommand(t, "", errors.New("claude unavailable"))
	defer restoreCLI()

	result := FetchClaudeUsage()

	if result.Status != "error" {
		t.Fatalf("expected status error for fully expired credentials, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "expired") {
		t.Fatalf("expected message to contain 'expired', got: %s", result.Message)
	}
}

func TestFetchClaudeUsage_EmptyAccessToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Mock Keychain with empty access token but valid refresh
	emptyTokenCreds := map[string]interface{}{
		"claudeAiOauth": map[string]interface{}{
			"accessToken":           "",
			"refreshToken":          "valid-refresh-token",
			"expiresAt":             0,
			"refreshTokenExpiresAt": time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
			"subscriptionType":      "max",
		},
	}
	credsJSON, _ := json.Marshal(emptyTokenCreds)

	restore := mockSecurityCommand(t, string(credsJSON))
	defer restore()
	restoreCLI := mockClaudeUsageCommand(t, "", errors.New("claude unavailable"))
	defer restoreCLI()

	result := FetchClaudeUsage()

	if result.Status != "warn" {
		t.Fatalf("expected status warn for empty token, got %s", result.Status)
	}
	if result.Plan != "max" {
		t.Fatalf("expected plan=max from keychain, got %s", result.Plan)
	}
	if !strings.Contains(result.Message, "expired") {
		t.Fatalf("expected message about expired credentials, got: %s", result.Message)
	}
}

func TestFetchClaudeUsage_FallbackToCredentialsFile(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	// Create .claude directory and credentials file
	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0755)
	credsFile := filepath.Join(claudeDir, ".credentials.json")
	credsContent := map[string]interface{}{
		"access_token": "file-based-token",
	}
	credsJSON, _ := json.Marshal(credsContent)
	os.WriteFile(credsFile, credsJSON, 0600)

	// Mock API response
	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Authorization") != "Bearer file-based-token" {
				t.Fatalf("expected token from credentials file, got: %s", req.Header.Get("Authorization"))
			}
			body := `{"five_hour":{"utilization":25.5}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0

	result := FetchClaudeUsage()

	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %s (message=%s)", result.Status, result.Message)
	}
	if result.Used != "25.5" {
		t.Fatalf("expected used=25.5, got %s", result.Used)
	}
}
