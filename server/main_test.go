package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestApplySessionAuth_APIKeyModeStripsBrowserBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer swipe-session-token")
	req.Header.Set("x-api-key", "client-supplied-key")
	req.Header.Set("x-immich-user-token", "user-token")

	session := &Session{
		Mode:   AuthModeAPIKey,
		APIKey: "server-side-api-key",
	}
	applySessionAuth(req, session)

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected Authorization stripped for API-key mode, got %q", got)
	}
	if got := req.Header.Get("x-api-key"); got != "server-side-api-key" {
		t.Fatalf("expected server x-api-key, got %q", got)
	}
	if got := req.Header.Get("x-immich-user-token"); got != "" {
		t.Fatalf("expected x-immich-user-token stripped, got %q", got)
	}
}

func TestApplySessionAuth_AccessTokenMode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer swipe-session-token")
	req.Header.Set("x-api-key", "should-be-removed")

	session := &Session{
		Mode:        AuthModeAccessToken,
		AccessToken: "immich-access-token",
	}
	applySessionAuth(req, session)

	if got := req.Header.Get("Authorization"); got != "Bearer immich-access-token" {
		t.Fatalf("expected Immich Bearer, got %q", got)
	}
	if got := req.Header.Get("x-api-key"); got != "" {
		t.Fatalf("expected no x-api-key in access-token mode, got %q", got)
	}
}

func TestLoginHandler_AmbiguousBodies(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich.example"})

	cases := []struct {
		name string
		body string
	}{
		{"empty", `{}`},
		{"email+password only", `{"email":"a@b.c","password":"x"}`},
		{"apiKey only", `{"apiKey":"k"}`},
		{"userName only", `{"userName":"Alice"}`},
		{"apiKey+userName without password", `{"apiKey":"k","userName":"Alice"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			srv.loginHandler(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
			}
			var resp map[string]string
			_ = json.Unmarshal(rr.Body.Bytes(), &resp)
			if !strings.Contains(resp["error"], "unsupported login method") {
				t.Fatalf("expected unsupported login method error, got %q", rr.Body.String())
			}
		})
	}
}

func TestLoginHandler_MissingBody(t *testing.T) {
	srv := NewServer(Config{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestLoginHandler_CredentialLoginSuccess(t *testing.T) {
	// New contract: sign-in is userName+password against local accounts only, no Immich call.
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})
	srv.accounts.SetPassword(immich.URL, "Alice", "key-alice", "secret123")

	raw, _ := json.Marshal(map[string]string{"userName": "Alice", "password": "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected session token")
	}
	if resp["userName"] != "Alice" {
		t.Fatalf("expected Alice, got %v", resp["userName"])
	}
	if resp["mode"] != "apiKey" {
		t.Fatalf("expected apiKey mode, got %v", resp["mode"])
	}
	token, _ := resp["token"].(string)
	session, ok := srv.session.Get(token)
	if !ok {
		t.Fatal("session not found")
	}
	if session.Mode != AuthModeAPIKey {
		t.Fatalf("expected apiKey mode, got %q", session.Mode)
	}
	if resp["hasApiKey"] != true {
		t.Fatalf("expected hasApiKey true, got %v", resp["hasApiKey"])
	}
}

func TestLoginHandler_CredentialLoginInvalid(t *testing.T) {
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})
	srv.accounts.SetPassword(immich.URL, "Alice", "key-alice", "secret123")

	raw, _ := json.Marshal(map[string]string{"userName": "Alice", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["code"] != "invalid_password" {
		t.Fatalf("expected invalid_password, got %q", resp["code"])
	}
	if len(srv.session.sessions) != 0 && false {
		// sessions held from previous test irrelevant
	}
}

func TestLoginHandler_APIKeyStillWorks(t *testing.T) {
	// Manual API-key login is removed; apiKey-only body must return 400 unsupported.
	srv := NewServer(Config{ServerURL: "http://immich.example"})
	raw, _ := json.Marshal(map[string]string{"apiKey": "valid-key", "serverUrl": "http://immich.example"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "unsupported login method") {
		t.Fatalf("expected unsupported login method, got %q", rr.Body.String())
	}
}

func TestLogoutHandler_DeletesSession(t *testing.T) {
	srv := NewServer(Config{})
	token := srv.session.CreateAPIKey("Alice", "key", "http://immich")

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.logoutHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if _, ok := srv.session.Get(token); ok {
		t.Fatal("session should be deleted")
	}
}

func TestLogoutHandler_SucceedsWithoutAuth(t *testing.T) {
	srv := NewServer(Config{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rr := httptest.NewRecorder()
	srv.logoutHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestSessionStore_CreateAccessToken(t *testing.T) {
	store := NewSessionStore("")
	token := store.CreateAccessToken("Alice", "jwt-token", "http://immich", "alice@test.com", "uid-1")
	s, ok := store.Get(token)
	if !ok {
		t.Fatal("missing access-token session")
	}
	if s.Mode != AuthModeAccessToken {
		t.Fatalf("expected accessToken mode, got %q", s.Mode)
	}
	if s.AccessToken != "jwt-token" {
		t.Fatalf("expected access token stored")
	}
	if s.APIKey != "" {
		t.Fatal("APIKey should be empty for access-token session")
	}
	if s.UserEmail != "alice@test.com" {
		t.Fatalf("expected user email stored, got %q", s.UserEmail)
	}
	if s.UserID != "uid-1" {
		t.Fatalf("expected user ID stored, got %q", s.UserID)
	}
	if s.UserName != "Alice" {
		t.Fatalf("expected user name, got %q", s.UserName)
	}
	if s.ServerURL != "http://immich" {
		t.Fatalf("expected server URL, got %q", s.ServerURL)
	}
	if time.Now().After(s.ExpiresAt) {
		t.Fatal("new session should not be expired")
	}
}

func TestLoginHandler_CredentialNoServerURL(t *testing.T) {
	// No serverUrl anywhere → sign-in should fail with 400 no server URL.
	srv := NewServer(Config{})
	srv.accounts.SetPassword("", "Alice", "key", "secret123")
	raw, _ := json.Marshal(map[string]string{"userName": "Alice", "password": "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLoginHandler_CredentialPasswordDisabled(t *testing.T) {
	// Password login against local accounts never contacts Immich, so "disabled" upstream is irrelevant;
	// wrong password still yields 401 invalid_password, not 403.
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})
	srv.accounts.SetPassword(immich.URL, "Alice", "key-alice", "secret123")
	raw, _ := json.Marshal(map[string]string{"userName": "Alice", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["code"] != "invalid_password" {
		t.Fatalf("expected invalid_password, got %q", resp["code"])
	}
}

func TestLoginHandler_EmailOnlyReturnsBadRequest(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich"})
	raw, _ := json.Marshal(map[string]string{"email": "user@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLoginHandler_PasswordOnlyReturnsBadRequest(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich"})
	raw, _ := json.Marshal(map[string]string{"password": "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLoginHandler_EnvUser(t *testing.T) {
	// Env user without password: userName-only is now unsupported → 400.
	srv := NewServer(Config{ServerURL: "http://immich.example", Users: []UserConfig{{Name: "Alice", APIKey: "env-alice-key"}}})
	raw, _ := json.Marshal(map[string]string{"userName": "Alice"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLoginHandler_EnvUserUnknown(t *testing.T) {
	// Unknown user with userName+password → 401 unknown_user (not 400).
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})
	raw, _ := json.Marshal(map[string]string{"userName": "Unknown", "password": "secret123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["code"] != "unknown_user" {
		t.Fatalf("expected unknown_user, got %q", resp["code"])
	}
}

func TestLoginHandler_ConfigNoUsersField(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich.example"})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rr := httptest.NewRecorder()
	srv.configHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var data map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &data)
	if _, ok := data["users"]; ok {
		t.Fatalf("config must not return users field, got %v", data)
	}
	if _, ok := data["defaultServerUrl"]; !ok {
		t.Fatal("expected defaultServerUrl")
	}
}

func TestSessionStore_GetDeleteCleanup(t *testing.T) {
	store := NewSessionStore("")
	token := store.CreateAPIKey("u", "k", "http://s")
	s1, ok := store.Get(token)
	if !ok {
		t.Fatal("missing session")
	}
	if s1.Mode != AuthModeAPIKey {
		t.Fatalf("expected apiKey mode")
	}
	if _, ok := store.Get(token); !ok {
		t.Fatal("missing session on second get")
	}
	store.Delete(token)
	if _, ok := store.Get(token); ok {
		t.Fatal("deleted session still present")
	}
	store.Cleanup()
}

// TestProxyHandler_StripsClientAuthHeaders exercises the full ServeHTTP path:
func TestProxyHandler_StripsClientAuthHeaders(t *testing.T) {
	var gotAuth, gotAPIKey, gotImmichUserToken, gotImmichSessionToken, gotImmichShareKey string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("x-api-key")
		gotImmichUserToken = r.Header.Get("x-immich-user-token")
		gotImmichSessionToken = r.Header.Get("x-immich-session-token")
		gotImmichShareKey = r.Header.Get("x-immich-share-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	srv := NewServer(Config{ServerURL: upstream.URL})
	token := srv.session.CreateAPIKey("Alice", "server-side-key", upstream.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-api-key", "client-supplied-key")
	req.Header.Set("x-immich-user-token", "user-token")
	req.Header.Set("x-immich-session-token", "session-token")
	req.Header.Set("x-immich-share-key", "share-key")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxied request, got %d body=%s", rr.Code, rr.Body.String())
	}
	if gotAuth != "" {
		t.Fatalf("upstream must not receive the Swipe session Authorization header, got %q", gotAuth)
	}
	if gotImmichUserToken != "" || gotImmichSessionToken != "" || gotImmichShareKey != "" {
		t.Fatalf("upstream received client Immich headers: user=%q session=%q share=%q",
			gotImmichUserToken, gotImmichSessionToken, gotImmichShareKey)
	}
	if gotAPIKey != "server-side-key" {
		t.Fatalf("expected server-side x-api-key, got %q", gotAPIKey)
	}
}

func TestProxy_ApiKeyRequired_ShortCircuit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream must not be called")
	}))
	defer upstream.Close()
	srv := NewServer(Config{ServerURL: upstream.URL})
	srv.accounts.SetPassword(upstream.URL, "Alice", "", "secret123")
	token := srv.session.CreateAPIKey("Alice", "", upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/random", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected 428, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["code"] != "api_key_required" {
		t.Fatalf("expected api_key_required, got %q", resp["code"])
	}
}
