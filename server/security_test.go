package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsDisallowedIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.5", true},
		{"192.168.1.5", true},
		{"172.16.0.5", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"fd00::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if got := isDisallowedIP(ip); got != tc.want {
			t.Errorf("isDisallowedIP(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
	if !isDisallowedIP(nil) {
		t.Error("nil IP must be disallowed")
	}
}

func TestValidateClientServerURLRejectsPrivateHost(t *testing.T) {
	s := &Server{config: Config{}}
	if _, err := s.validateClientServerURL("http://127.0.0.1:2283"); err == nil {
		t.Fatal("loopback URL must be rejected for client-supplied servers")
	}
	if _, err := s.validateClientServerURL("http://169.254.169.254/latest/meta-data"); err == nil {
		t.Fatal("metadata service must be rejected")
	}
	if _, err := s.validateClientServerURL("file:///etc/passwd"); err == nil {
		t.Fatal("non-http schemes must be rejected")
	}
	if _, err := s.validateClientServerURL("http://user:pass@example.com"); err == nil {
		t.Fatal("embedded credentials must be rejected")
	}
}

func TestValidateClientServerURLTrustsConfiguredServer(t *testing.T) {
	s := &Server{config: Config{ServerURL: "http://192.168.1.10:2283"}}
	u, err := s.validateClientServerURL("http://192.168.1.10:2283")
	if err != nil {
		t.Fatalf("configured LAN server should be trusted: %v", err)
	}
	if u.Host != "192.168.1.10:2283" {
		t.Fatalf("unexpected URL: %v", u)
	}
	// A different private host is still refused.
	if _, err := s.validateClientServerURL("http://192.168.1.11:2283"); err == nil {
		t.Fatal("unconfigured private host must be rejected")
	}
}

func TestAllowUnsafeRequestCSRF(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "http://app.example/api/auth/login", nil)
	req.Host = "app.example"

	// No cookies: legacy API/browser bootstrap requests pass.
	if !s.allowUnsafeRequest(req) {
		t.Fatal("cookie-less request should be allowed")
	}

	// Session cookie without a CSRF pair is rejected.
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	if s.allowUnsafeRequest(req) {
		t.Fatal("session request without CSRF cookie must be rejected")
	}

	// Matching double-submit pair is accepted.
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	req.Header.Set(csrfHeaderName, "csrf")
	if !s.allowUnsafeRequest(req) {
		t.Fatal("matching CSRF token should be allowed")
	}

	// Cross-origin is rejected even with a valid token.
	req.Header.Set("Origin", "https://evil.example")
	if s.allowUnsafeRequest(req) {
		t.Fatal("cross-origin request must be rejected")
	}

	// Same origin with the forwarded host matches.
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("X-Forwarded-Host", "app.example")
	if !s.allowUnsafeRequest(req) {
		t.Fatal("same-origin request with forwarded host should be allowed")
	}
}

func TestSessionCookieFlags(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://app.example/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	s.setSessionCookie(w, req, "token")
	cookie := w.Header().Get("Set-Cookie")
	for _, want := range []string{"HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(cookie, want) {
			t.Errorf("session cookie missing %s: %s", want, cookie)
		}
	}
}
