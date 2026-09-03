package usage

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFetchAntigravityUsageUsesQuotaAPI(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	credsDir := tmp + "/.gemini"
	if err := os.MkdirAll(credsDir, 0o755); err != nil {
		t.Fatalf("mkdir creds failed: %v", err)
	}
	creds := fmt.Sprintf(`{"access_token":"test-token","refresh_token":"refresh","expiry_date":%d}`, time.Now().Add(time.Hour).UnixMilli())
	if err := os.WriteFile(credsDir+"/oauth_creds.json", []byte(creds), 0o600); err != nil {
		t.Fatalf("write creds failed: %v", err)
	}

	projectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("project Authorization header did not contain test bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"cloudaicompanionProject":"project-1"}`)
	}))
	defer projectServer.Close()
	quotaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("quota Authorization header did not contain test bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"buckets":[{"remainingFraction":0.6,"resetTime":"2026-07-22T00:00:00Z","modelId":"gemini-2.5-pro"}]}`)
	}))
	defer quotaServer.Close()
	t.Setenv("OCT_GEMINI_API_ENDPOINT", projectServer.URL)
	t.Setenv("OCT_GEMINI_USAGE_ENDPOINT", quotaServer.URL)

	result := FetchAntigravityUsage()
	if result.Source != "quota" {
		t.Fatalf("expected quota source, got %q", result.Source)
	}
	if result.Used != "40.0" {
		t.Fatalf("expected 40.0 used percent, got %q", result.Used)
	}
	if result.Buckets["model:Pro"] != "40.0" {
		t.Fatalf("expected Pro model bucket 40.0, got %q", result.Buckets["model:Pro"])
	}
}

func TestReadAntigravityOAuthCredentialsUsesKeychainBase64(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	oldAvailable := antigravityKeychainAvailable
	oldOutput := antigravitySecurityOutput
	t.Cleanup(func() {
		antigravityKeychainAvailable = oldAvailable
		antigravitySecurityOutput = oldOutput
	})
	antigravityKeychainAvailable = func() bool { return true }
	antigravitySecurityOutput = func(args ...string) ([]byte, error) {
		want := []string{"find-generic-password", "-s", "gemini", "-a", "antigravity", "-w"}
		if len(args) != len(want) {
			t.Fatalf("security args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("security args = %v, want %v", args, want)
			}
		}
		payload := fmt.Sprintf(
			`{"token":{"access_token":"keychain-token","refresh_token":"refresh","expiry":%q},"auth_method":"consumer"}`,
			time.Now().Add(time.Hour).Format(time.RFC3339Nano),
		)
		return []byte("go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(payload))), nil
	}

	creds, ok := readAntigravityOAuthCredentials()
	if !ok {
		t.Fatal("expected keychain credentials")
	}
	if creds.AccessToken != "keychain-token" {
		t.Fatalf("access token = %q, want keychain-token", creds.AccessToken)
	}
	if creds.AuthSource != "keychain:gemini/antigravity" {
		t.Fatalf("auth source = %q, want keychain key", creds.AuthSource)
	}
}

func TestFetchAntigravityUsageDoesNotUseLocalSessionFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	result := FetchAntigravityUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if result.Status != "warn" {
		t.Fatalf("expected warn status, got %q", result.Status)
	}
	if result.Unit != "percent" {
		t.Fatalf("expected percent unit, got %q", result.Unit)
	}
	if strings.EqualFold(result.Unit, "sessions") || strings.Contains(strings.ToLower(result.Message), "session") {
		t.Fatalf("local session fallback leaked into result: %#v", result)
	}
}

func TestFetchGeminiUsageDelegatesToAntigravity(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	result := FetchGeminiUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if !strings.EqualFold(result.Source, "quota") {
		t.Fatalf("expected quota source, got %q", result.Source)
	}
}
