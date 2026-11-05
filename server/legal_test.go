package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegalConfigHandlerUsesConfiguredOperator(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector.example:4317")
	srv := NewServer(Config{
		LegalControllerName: "Example Photo Cooperative",
		LegalContactEmail:   "privacy@example.com",
		LegalContactAddress: "1 Example Street",
		LegalHostingRegion:  "the Netherlands",
		SMTPHost:            "smtp.example.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/legal", nil)
	rr := httptest.NewRecorder()
	srv.legalConfigHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var got LegalConfig
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Controller != "Example Photo Cooperative" {
		t.Fatalf("controller = %q", got.Controller)
	}
	if got.ContactEmail != "privacy@example.com" || got.ContactAddress != "1 Example Street" {
		t.Fatalf("contact not published: %+v", got)
	}
	if got.HostingRegion != "the Netherlands" {
		t.Fatalf("hosting region = %q", got.HostingRegion)
	}
	if !got.SMTPConfigured || !got.TelemetryConfigured {
		t.Fatalf("processor flags not reflected: %+v", got)
	}
	if got.PrivacyURL != "/privacy" || got.TermsURL != "/terms" || got.DeletionURL != "/account/delete" {
		t.Fatalf("legal URLs = %+v", got)
	}
}

func TestLegalConfigHandlerFallsBackWithoutInventingIdentity(t *testing.T) {
	srv := NewServer(Config{})
	req := httptest.NewRequest(http.MethodGet, "/api/legal", nil)
	rr := httptest.NewRecorder()
	srv.legalConfigHandler(rr, req)

	var got LegalConfig
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Controller == "" || got.HostingRegion == "" {
		t.Fatalf("expected neutral fallbacks, got %+v", got)
	}
	if got.ContactEmail != "" || got.ContactAddress != "" {
		t.Fatalf("must not invent a contact, got %+v", got)
	}
}

func TestLegalConfigHandlerRejectsWrites(t *testing.T) {
	srv := NewServer(Config{})
	req := httptest.NewRequest(http.MethodPost, "/api/legal", nil)
	rr := httptest.NewRecorder()
	srv.legalConfigHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

// The legal pages are Vue routes inside the SPA. This pins the server contract
// they rely on: an unknown non-API path falls back to index.html without a
// session, so a hard refresh at /privacy, /terms or /account/delete works.
func TestStaticHandlerServesLegalRoutesWithoutAuth(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>Immich Swipe</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Config{StaticDir: dir})
	for _, path := range []string{"/privacy", "/terms", "/account/delete"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "<div id=\"app\">") && !strings.Contains(rr.Body.String(), "Immich Swipe") {
			t.Fatalf("%s: expected SPA shell, got %q", path, rr.Body.String())
		}
	}
}
