package main

import (
	"net/http"
	"os"
	"strings"
)

// LegalConfig is the public, non-secret description of this deployment that the
// privacy, terms and deletion pages render. It is derived from environment
// configuration and the live server state so the text cannot claim a processor
// (SMTP, tracing) that this instance is not running.
type LegalConfig struct {
	AppName             string `json:"appName"`
	Controller          string `json:"controller"`
	ContactEmail        string `json:"contactEmail"`
	ContactAddress      string `json:"contactAddress"`
	HostingRegion       string `json:"hostingRegion"`
	DefaultServerURL    string `json:"defaultServerUrl"`
	SMTPConfigured      bool   `json:"smtpConfigured"`
	TelemetryConfigured bool   `json:"telemetryConfigured"`
	PrivacyURL          string `json:"privacyUrl"`
	TermsURL            string `json:"termsUrl"`
	DeletionURL         string `json:"deletionUrl"`
}

const defaultLegalController = "the operator of this Immich Swipe instance"
const defaultLegalHostingRegion = "the European Union"

// legalConfig builds the policy facts for one request. Every field is either
// explicit configuration or read from the running server; missing values fall
// back to text that does not invent an identity or address.
func (s *Server) legalConfig() LegalConfig {
	cfg := LegalConfig{
		AppName:          "Immich Swipe",
		Controller:       defaultLegalController,
		HostingRegion:    defaultLegalHostingRegion,
		DefaultServerURL: s.config.ServerURL,
		PrivacyURL:       "/privacy",
		TermsURL:         "/terms",
		DeletionURL:      "/account/delete",
	}
	if v := strings.TrimSpace(s.config.LegalControllerName); v != "" {
		cfg.Controller = v
	}
	cfg.ContactEmail = strings.TrimSpace(s.config.LegalContactEmail)
	cfg.ContactAddress = strings.TrimSpace(s.config.LegalContactAddress)
	if v := strings.TrimSpace(s.config.LegalHostingRegion); v != "" {
		cfg.HostingRegion = v
	}
	if strings.TrimSpace(s.config.SMTPHost) != "" {
		cfg.SMTPConfigured = true
	} else if es, ok := s.effectiveEmailSettings(); ok && strings.TrimSpace(es.Host) != "" {
		cfg.SMTPConfigured = true
	}
	cfg.TelemetryConfigured = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" ||
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")) != ""
	return cfg
}

// legalConfigHandler serves GET /api/legal. Public: both the store reviewer and
// a signed-out visitor must be able to read the policy.
func (s *Server) legalConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, s.legalConfig())
}
