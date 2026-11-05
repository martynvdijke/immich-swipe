package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func deleteAccountRequest(t *testing.T, srv *Server, token, confirm string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"confirm": confirm})
	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func TestDeleteAccountErasesAccountSessionsCredentialsAndCounters(t *testing.T) {
	srv := NewServer(Config{})
	srv.accounts.SetPassword("http://immich.example", "Alice", "immich-api-key", "secret123")
	srv.accounts.SetEmail("http://immich.example", "Alice", "alice@example.com")
	token := srv.session.CreateAPIKey("Alice", "immich-api-key", "http://immich.example")
	// A second session for the same identity must also be revoked.
	other := srv.session.CreateAPIKey("Alice", "immich-api-key", "http://immich.example")
	srv.stats.IncrementKept("http://immich.example", "Alice", 3)
	srv.stats.IncrementDeleted("http://immich.example", "Alice", 1)

	rr := deleteAccountRequest(t, srv, token, "Alice")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	if _, ok := srv.accounts.Get("http://immich.example", "Alice"); ok {
		t.Fatal("account row survived deletion")
	}
	if _, ok := srv.session.Get(token); ok {
		t.Fatal("current session survived deletion")
	}
	if _, ok := srv.session.Get(other); ok {
		t.Fatal("sibling session survived deletion")
	}
	if _, ok := srv.stats.Get("http://immich.example", "Alice"); ok {
		t.Fatal("per-user counters survived deletion")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "deleted" {
		t.Fatalf("status = %v", resp["status"])
	}
	if resp["immichAccount"] != "retained" {
		t.Fatalf("must tell the caller the Immich account remains: %v", resp["immichAccount"])
	}
}

func TestDeleteAccountRequiresTypedConfirmation(t *testing.T) {
	srv := NewServer(Config{})
	srv.accounts.SetPassword("http://immich.example", "Alice", "immich-api-key", "secret123")
	token := srv.session.CreateAPIKey("Alice", "immich-api-key", "http://immich.example")

	rr := deleteAccountRequest(t, srv, token, "alice")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["code"] != "confirmation_mismatch" {
		t.Fatalf("code = %q", resp["code"])
	}
	if _, ok := srv.accounts.Get("http://immich.example", "Alice"); !ok {
		t.Fatal("account must survive a failed confirmation")
	}
	if _, ok := srv.session.Get(token); !ok {
		t.Fatal("session must survive a failed confirmation")
	}
}

func TestDeleteAccountRequiresAuthentication(t *testing.T) {
	srv := NewServer(Config{})
	body, _ := json.Marshal(map[string]string{"confirm": "Alice"})
	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestDeleteAccountIsCSRFProtectedForCookieSessions(t *testing.T) {
	srv := NewServer(Config{})
	srv.accounts.SetPassword("http://immich.example", "Alice", "immich-api-key", "secret123")
	token := srv.session.CreateAPIKey("Alice", "immich-api-key", "http://immich.example")
	body, _ := json.Marshal(map[string]string{"confirm": "Alice"})

	// Session cookie without the double-submit header: refused before auth.
	req := httptest.NewRequest(http.MethodDelete, "/api/auth/account", strings.NewReader(string(body)))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF header, got %d body=%s", rr.Code, rr.Body.String())
	}
	if _, ok := srv.accounts.Get("http://immich.example", "Alice"); !ok {
		t.Fatal("CSRF failure must not delete the account")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/auth/account", strings.NewReader(string(body)))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	req.Header.Set(csrfHeaderName, "csrf-token")
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with CSRF header, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDeleteAccountDropsAccessTokenSession(t *testing.T) {
	srv := NewServer(Config{})
	srv.accounts.SetPassword("http://immich.example", "OAuth Alice", "", "secret123")
	token := srv.session.CreateAccessToken("OAuth Alice", "immich-access-token", "http://immich.example", "alice@example.com", "uid-1")

	rr := deleteAccountRequest(t, srv, token, "OAuth Alice")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	// The access token is only stored inside the session row; deleting the row
	// erases the local reference (Immich-side revocation is best-effort).
	if _, ok := srv.session.Get(token); ok {
		t.Fatal("access-token session survived deletion")
	}
	if leaked := srv.session.SessionsFor("http://immich.example", "OAuth Alice"); len(leaked) != 0 {
		t.Fatalf("expected no sessions for the identity, got %d", len(leaked))
	}
}

func TestExportAccountContainsProfileAndCountersButNoSecrets(t *testing.T) {
	srv := NewServer(Config{})
	srv.accounts.SetPassword("http://immich.example", "Alice", "immich-api-key-secret", "password-secret")
	srv.accounts.SetEmail("http://immich.example", "Alice", "alice@example.com")
	token := srv.session.CreateAPIKey("Alice", "immich-api-key-secret", "http://immich.example")
	srv.stats.IncrementKept("http://immich.example", "Alice", 4)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/account/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") {
		t.Fatalf("expected attachment, got %q", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "alice@example.com") || !strings.Contains(body, "Alice") {
		t.Fatalf("export missing profile: %s", body)
	}
	if !strings.Contains(body, `"keptCount":4`) {
		t.Fatalf("export missing counters: %s", body)
	}
	for _, secret := range []string{"immich-api-key-secret", "password-secret", "pbkdf2$"} {
		if strings.Contains(body, secret) {
			t.Fatalf("export leaked %q: %s", secret, body)
		}
	}
	if strings.Contains(body, token) {
		t.Fatal("export leaked the Swipe session token")
	}
}

func TestExportAccountRequiresAuthentication(t *testing.T) {
	srv := NewServer(Config{})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/account/export", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
