package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOTelSettingsRouteReportsConfiguredEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://collector.example.com:4318")
	srv := NewServer(Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/otel-settings", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"endpoint", "enabled", "traces_enabled", "metrics_enabled", "logs_enabled"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("missing %q in response: %s", key, rr.Body.String())
		}
	}

	var got OTelSettings
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Endpoint != "https://collector.example.com:4318" {
		t.Fatalf("endpoint = %q", got.Endpoint)
	}
	if !got.Enabled || !got.TracesEnabled {
		t.Fatalf("expected traces enabled: %+v", got)
	}
	if got.MetricsEnabled || got.LogsEnabled {
		t.Fatalf("metrics/logs should be reported disabled: %+v", got)
	}
}

func TestOTelSettingsRouteDisabledWhenUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	srv := NewServer(Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/otel-settings", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var got OTelSettings
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Endpoint != "" || got.Enabled || got.TracesEnabled {
		t.Fatalf("expected disabled settings, got %+v", got)
	}
}

func TestOTelSettingsRouteIsPublicAndGetOnly(t *testing.T) {
	srv := NewServer(Config{})

	// Unauthenticated GET must not be rejected by the auth middleware.
	req := httptest.NewRequest(http.MethodGet, "/api/otel-settings", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("public GET status = %d", rr.Code)
	}

	// POST is a method error, not a proxied/auth failure.
	post := httptest.NewRequest(http.MethodPost, "/api/otel-settings", nil)
	postRR := httptest.NewRecorder()
	srv.otelSettingsHandler(postRR, post)
	if postRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", postRR.Code)
	}
}
