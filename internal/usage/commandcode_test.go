package usage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCommandCodeAPIKeyEnv(t *testing.T) {
	t.Setenv("COMMAND_CODE_API_KEY", "env-key")
	key, source := resolveCommandCodeAPIKey()
	if key != "env-key" {
		t.Fatalf("key = %q, want env-key", key)
	}
	if source != "env:COMMAND_CODE_API_KEY" {
		t.Fatalf("source = %q, want env:COMMAND_CODE_API_KEY", source)
	}
}

func TestResolveCommandCodeAPIKeyAuthJSON(t *testing.T) {
	t.Setenv("COMMAND_CODE_API_KEY", "")
	t.Setenv("COMMANDCODE_API_ENV", "")
	tmp := t.TempDir()
	origUserHomeDir := userHomeDir
	userHomeDir = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { userHomeDir = origUserHomeDir })

	authDir := filepath.Join(tmp, ".commandcode")
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "auth.json"), []byte(`{"apiKey":"file-key","userName":"tester"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	key, source := resolveCommandCodeAPIKey()
	if key != "file-key" {
		t.Fatalf("key = %q, want file-key", key)
	}
	if source != "auth.json" {
		t.Fatalf("source = %q, want auth.json", source)
	}
}

func TestFetchCommandCodeUsageSuccess(t *testing.T) {
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer test-key" {
			sawAuth = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/alpha/whoami":
			json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]any{"id": "u1", "userName": "tester"},
				"org":  map[string]any{"id": "org1", "login": "team"},
			})
		case "/alpha/billing/credits":
			if got := r.URL.Query().Get("orgId"); got != "org1" {
				t.Fatalf("credits orgId = %q, want org1", got)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"credits": map[string]any{
					"planId":           "individual-pro-v1",
					"monthlyCredits":   60.0,
					"purchasedCredits": 5.0,
					"freeCredits":      0.0,
				},
				"windowLimits": map[string]any{
					"limited":  true,
					"fiveHour": map[string]any{"used": 4.0, "cap": 16.0, "resetAt": 1790000000000},
					"weekly":   map[string]any{"used": 20.0, "cap": 40.0, "resetAt": 1790500000000},
				},
			})
		case "/alpha/billing/subscriptions":
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"planId":             "individual-pro-v1",
					"status":             "active",
					"currentPeriodStart": "2026-09-01T00:00:00Z",
					"currentPeriodEnd":   "2026-10-01T00:00:00Z",
				},
			})
		case "/alpha/usage/summary":
			if got := r.URL.Query().Get("since"); got != "2026-09-01T00:00:00Z" {
				t.Fatalf("summary since = %q, want current period start", got)
			}
			json.NewEncoder(w).Encode(map[string]any{"totalCost": 20.0, "totalCount": 42})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	originalBaseURL := commandCodeAPIBaseURL
	commandCodeAPIBaseURL = server.URL
	t.Cleanup(func() { commandCodeAPIBaseURL = originalBaseURL })
	t.Setenv("COMMAND_CODE_API_KEY", "test-key")
	t.Setenv("OCT_COMMANDCODE_API_BASE_URL", "")
	t.Setenv("COMMANDCODE_API_ENV", "")

	result := FetchCommandCodeUsage()
	if !sawAuth {
		t.Fatal("expected Authorization header")
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok; message=%s", result.Status, result.Message)
	}
	if result.Provider != "commandcode" {
		t.Fatalf("provider = %q, want commandcode", result.Provider)
	}
	if result.Plan != "Pro" {
		t.Fatalf("plan = %q, want Pro", result.Plan)
	}
	if result.Used != "25.0" || result.Buckets["5h"] != "25.0" || result.Buckets["7d"] != "50.0" {
		t.Fatalf("unexpected buckets used=%s buckets=%v", result.Used, result.Buckets)
	}
	if result.Buckets["1m"] != "23.5" {
		t.Fatalf("monthly bucket = %q, want 23.5", result.Buckets["1m"])
	}
	if result.BucketResets["5h"] != "1790000000000" {
		t.Fatalf("5h reset = %q", result.BucketResets["5h"])
	}
	if !strings.Contains(result.Message, "42 requests") {
		t.Fatalf("expected request count in message, got %q", result.Message)
	}
}

func TestFetchCommandCodeUsageNoAPIKey(t *testing.T) {
	t.Setenv("COMMAND_CODE_API_KEY", "")
	tmp := t.TempDir()
	origUserHomeDir := userHomeDir
	userHomeDir = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { userHomeDir = origUserHomeDir })

	result := FetchCommandCodeUsage()
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "Command Code API key not found") {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestCommandCodeHelpers(t *testing.T) {
	if got := commandCodeAuthFileName(); got != "auth.json" {
		t.Fatalf("default auth file = %q, want auth.json", got)
	}
	t.Setenv("COMMANDCODE_API_ENV", "staging")
	if got := commandCodeAuthFileName(); got != "auth.staging.json" {
		t.Fatalf("staging auth file = %q", got)
	}
	if got := commandCodePlanLabel("individual_goat_monthly"); got != "GOAT" {
		t.Fatalf("plan label = %q, want GOAT", got)
	}
	if got := commandCodePlanCredits("individual-ultra"); got != 300 {
		t.Fatalf("plan credits = %v, want 300", got)
	}
}
