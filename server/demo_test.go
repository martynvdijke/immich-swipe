package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadConfig_DemoUser(t *testing.T) {
	t.Setenv("DEMO_USER_ENABLED", "true")
	t.Setenv("DEMO_USER_NAME", "reviewer")
	t.Setenv("DEMO_USER_PASSWORD", "hunter22")
	t.Setenv("DEMO_USER_API_KEY", "demo-api-key")
	t.Setenv("DEMO_USER_SERVER_URL", "https://photos.example")
	t.Setenv("DEMO_USER_EMAIL", "demo@example.com")

	cfg := loadConfig()
	if !cfg.DemoUserEnabled || cfg.DemoUserName != "reviewer" || cfg.DemoUserPassword != "hunter22" ||
		cfg.DemoUserAPIKey != "demo-api-key" || cfg.DemoUserServerURL != "https://photos.example" ||
		cfg.DemoUserEmail != "demo@example.com" {
		t.Fatalf("unexpected demo config: %+v", cfg)
	}

	serverURL, userName, password, ok := cfg.demoCredentials()
	if !ok || serverURL != "https://photos.example" || userName != "reviewer" || password != "hunter22" {
		t.Fatalf("expected valid demo credentials, got ok=%v url=%q user=%q", ok, serverURL, userName)
	}
}

func TestLoadConfig_DemoUserDefaults(t *testing.T) {
	t.Setenv("DEMO_USER_ENABLED", "1")
	t.Setenv("DEMO_USER_API_KEY", "demo-api-key")
	t.Setenv("IMMICH_SERVER_URL", "https://immich.example")
	t.Setenv("DEMO_USER_NAME", "")
	t.Setenv("DEMO_USER_PASSWORD", "")
	t.Setenv("DEMO_USER_SERVER_URL", "")
	t.Setenv("DEMO_USER_EMAIL", "")

	cfg := loadConfig()
	if cfg.DemoUserName != "demo" || cfg.DemoUserPassword != "demo" {
		t.Fatalf("expected demo/demo defaults, got %q/%q", cfg.DemoUserName, cfg.DemoUserPassword)
	}
	serverURL, _, _, ok := cfg.demoCredentials()
	if !ok || serverURL != "https://immich.example" {
		t.Fatalf("expected fallback to IMMICH_SERVER_URL, got ok=%v url=%q", ok, serverURL)
	}

	t.Setenv("DEMO_USER_ENABLED", "false")
	if loadConfig().DemoUserEnabled {
		t.Fatal("expected DEMO_USER_ENABLED=false to disable demo")
	}
}

func TestDemoCredentials_IncompleteConfigInvalid(t *testing.T) {
	// Missing API key.
	cfg := Config{DemoUserEnabled: true, DemoUserName: "demo", DemoUserPassword: "demo", ServerURL: "https://immich.example"}
	if _, _, _, ok := cfg.demoCredentials(); ok {
		t.Fatal("demo without an API key must be invalid")
	}

	// Missing any server URL.
	cfg = Config{DemoUserEnabled: true, DemoUserName: "demo", DemoUserPassword: "demo", DemoUserAPIKey: "k"}
	if _, _, _, ok := cfg.demoCredentials(); ok {
		t.Fatal("demo without a server URL must be invalid")
	}

	// Disabled but otherwise complete.
	cfg = Config{DemoUserEnabled: false, DemoUserName: "demo", DemoUserPassword: "demo", DemoUserAPIKey: "k", ServerURL: "https://immich.example"}
	if _, _, _, ok := cfg.demoCredentials(); ok {
		t.Fatal("disabled demo must not validate")
	}
}

func TestEnsureDemoAccount_CreatesAndRepairs(t *testing.T) {
	cfg := Config{
		ServerURL:         "https://immich.example",
		DemoUserEnabled:   true,
		DemoUserName:      "demo",
		DemoUserPassword:  "demo",
		DemoUserAPIKey:    "demo-api-key",
		DemoUserServerURL: "https://photos.example",
		DemoUserEmail:     "demo@example.com",
	}

	srv := NewServer(cfg)
	account, ok := srv.accounts.Get("https://photos.example", "demo")
	if !ok {
		t.Fatal("expected demo account to be created")
	}
	if account.APIKey != "demo-api-key" {
		t.Fatalf("expected demo API key bound, got %q", account.APIKey)
	}
	if account.Email == nil || *account.Email != "demo@example.com" {
		t.Fatalf("expected demo email set, got %v", account.Email)
	}
	if _, ok := srv.accounts.VerifyPassword("https://photos.example", "demo", "demo"); !ok {
		t.Fatal("expected demo password to verify")
	}

	// Simulate drift: account key/password changed, then reboot repairs it.
	srv.accounts.SetPassword("https://photos.example", "demo", "stale-key", "stale-password")
	if _, ok := srv.accounts.VerifyPassword("https://photos.example", "demo", "stale-password"); !ok {
		t.Fatal("setup failed: expected stale password")
	}

	repaired := NewServer(cfg)
	if _, ok := repaired.accounts.VerifyPassword("https://photos.example", "demo", "demo"); !ok {
		t.Fatal("expected demo password to be repaired on boot")
	}
	account, ok = repaired.accounts.Get("https://photos.example", "demo")
	if !ok || account.APIKey != "demo-api-key" {
		t.Fatalf("expected demo API key to be repaired, got %+v", account)
	}

	// Calling it twice is safe.
	repaired.ensureDemoAccount()
	if _, ok := repaired.accounts.VerifyPassword("https://photos.example", "demo", "demo"); !ok {
		t.Fatal("expected idempotent ensureDemoAccount")
	}
}

func TestEnsureDemoAccount_IncompleteDoesNothing(t *testing.T) {
	srv := NewServer(Config{
		ServerURL:        "https://immich.example",
		DemoUserEnabled:  true,
		DemoUserName:     "demo",
		DemoUserPassword: "demo",
		// API key missing.
	})
	if _, ok := srv.accounts.Get("https://immich.example", "demo"); ok {
		t.Fatal("incomplete demo config must not create an account")
	}
}

func TestConfigHandler_DemoFields(t *testing.T) {
	srv := NewServer(Config{
		DemoUserEnabled:   true,
		DemoUserName:      "demo",
		DemoUserPassword:  "demo-pass",
		DemoUserAPIKey:    "super-secret-key",
		DemoUserServerURL: "https://photos.example",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rr := httptest.NewRecorder()
	srv.configHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatalf("invalid config payload: %v", err)
	}
	if data["demoEnabled"] != true || data["demoUserName"] != "demo" ||
		data["demoPassword"] != "demo-pass" || data["demoServerUrl"] != "https://photos.example" {
		t.Fatalf("unexpected demo config payload: %s", rr.Body.String())
	}
	if _, leaked := data["demoApiKey"]; leaked {
		t.Fatalf("demo payload must not include an API key field: %s", rr.Body.String())
	}
	if bytesContainsJSONValue(data, "super-secret-key") {
		t.Fatalf("demo payload leaked the API key: %s", rr.Body.String())
	}

	disabled := NewServer(Config{DemoUserAPIKey: "super-secret-key"})
	disabledRR := httptest.NewRecorder()
	disabled.configHandler(disabledRR, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
	var disabledData map[string]interface{}
	_ = json.Unmarshal(disabledRR.Body.Bytes(), &disabledData)
	for _, field := range []string{"demoEnabled", "demoUserName", "demoPassword", "demoServerUrl"} {
		if _, present := disabledData[field]; present {
			t.Fatalf("disabled demo must not expose %s: %s", field, disabledRR.Body.String())
		}
	}
}

// bytesContainsJSONValue reports whether any string value in the decoded JSON
// equals the given value (used to assert a secret is not exposed).
func bytesContainsJSONValue(data map[string]interface{}, value string) bool {
	for _, v := range data {
		if s, ok := v.(string); ok && s == value {
			return true
		}
	}
	return false
}
