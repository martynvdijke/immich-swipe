package main

import (
	"net/http"
	"os"
	"strings"
)

// OTelSettings is the public, non-secret telemetry configuration the mobile app
// fetches at startup. Only the traces exporter is implemented in the app today,
// so metrics and logs are always reported as disabled.
type OTelSettings struct {
	Endpoint       string `json:"endpoint"`
	Enabled        bool   `json:"enabled"`
	TracesEnabled  bool   `json:"traces_enabled"`
	MetricsEnabled bool   `json:"metrics_enabled"`
	LogsEnabled    bool   `json:"logs_enabled"`
}

// otelSettings derives the mobile telemetry config from the same environment
// that configures the server-side exporter. With no endpoint configured the
// whole feature is reported as disabled, so clients stay silent.
func (s *Server) otelSettings() OTelSettings {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	enabled := endpoint != ""
	return OTelSettings{
		Endpoint:       endpoint,
		Enabled:        enabled,
		TracesEnabled:  enabled,
		MetricsEnabled: false,
		LogsEnabled:    false,
	}
}

// otelSettingsHandler serves GET /api/otel-settings. Public: the app needs the
// collector endpoint before the user has signed in.
func (s *Server) otelSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, s.otelSettings())
}
