package main

import (
	"bytes"
	"encoding/json"
	"io"
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
		body map[string]string
	}{
		{"email+apiKey", map[string]string{"email": "a@b.c", "password": "x", "apiKey": "k"}},
		{"email+userName", map[string]string{"email": "a@b.c", "password": "x", "userName": "Alice"}},
		{"apiKey+userName", map[string]string{"apiKey": "k", "userName": "Alice"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			srv.loginHandler(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
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
	var sawLoginAuth bool
	var sawMeAuth string

	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/auth/login"):
			body, _ := io.ReadAll(r.Body)
			var payload map[string]string
			_ = json.Unmarshal(body, &payload)
			if payload["email"] != "user@example.com" || payload["password"] != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"Invalid credentials"}`))
				return
			}
			sawLoginAuth = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"accessToken":"immich-jwt",
				"name":"Display Name",
				"userEmail":"user@example.com",
				"userId":"uid-1"
			}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/api/users/me"):
			sawMeAuth = r.Header.Get("Authorization")
			if sawMeAuth != "Bearer immich-jwt" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"Display Name","email":"user@example.com"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})
	raw, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !sawLoginAuth {
		t.Fatal("expected Immich login call")
	}
	if sawMeAuth != "Bearer immich-jwt" {
		t.Fatalf("expected users/me Bearer validation, got %q", sawMeAuth)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected session token")
	}
	if resp["userName"] != "Display Name" {
		t.Fatalf("expected display name, got %v", resp["userName"])
	}
	if _, ok := resp["accessToken"]; ok {
		t.Fatal("response must not include accessToken")
	}
	if _, ok := resp["password"]; ok {
		t.Fatal("response must not include password")
	}

	// Session should be access-token mode
	token, _ := resp["token"].(string)
	session, ok := srv.session.Get(token)
	if !ok {
		t.Fatal("session not found")
	}
	if session.Mode != AuthModeAccessToken {
		t.Fatalf("expected accessToken mode, got %q", session.Mode)
	}
	if session.AccessToken != "immich-jwt" {
		t.Fatalf("expected stored access token")
	}
	if session.APIKey != "" {
		t.Fatalf("APIKey should be empty in access-token mode")
	}
}

func TestLoginHandler_CredentialLoginInvalid(t *testing.T) {
	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/auth/login") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Invalid credentials"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})
	raw, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": "wrong",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
	if len(srv.session.sessions) != 0 {
		t.Fatal("should not create session on failed login")
	}
}

func TestLoginHandler_APIKeyStillWorks(t *testing.T) {
	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Key User"}`))
	}))
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})
	raw, _ := json.Marshal(map[string]string{
		"apiKey":    "valid-key",
		"serverUrl": immich.URL,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	token, _ := resp["token"].(string)
	session, ok := srv.session.Get(token)
	if !ok {
		t.Fatal("session missing")
	}
	if session.Mode != AuthModeAPIKey {
		t.Fatalf("expected apiKey mode, got %q", session.Mode)
	}
	if session.APIKey != "valid-key" {
		t.Fatalf("expected API key stored")
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
	store := NewSessionStore()
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
	srv := NewServer(Config{}) // no default ServerURL
	raw, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLoginHandler_CredentialPasswordDisabled(t *testing.T) {
	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/auth/login") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"Password login has been disabled","statusCode":400}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})
	raw, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for password-disabled, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !strings.Contains(strings.ToLower(resp["error"]), "disabled") {
		t.Fatalf("expected disabled message, got %q", resp["error"])
	}
	if len(srv.session.sessions) != 0 {
		t.Fatal("should not create session when password login is disabled")
	}
}

func TestLoginHandler_EmailOnlyReturnsBadRequest(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich"})
	raw, _ := json.Marshal(map[string]string{
		"email": "user@example.com",
	})
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
	raw, _ := json.Marshal(map[string]string{
		"password": "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestLoginHandler_EnvUser(t *testing.T) {
	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "env-alice-key" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"Alice","email":"alice@test.com"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer immich.Close()

	srv := NewServer(Config{
		ServerURL: immich.URL,
		Users: []UserConfig{
			{Name: "Alice", APIKey: "env-alice-key"},
		},
	})
	raw, _ := json.Marshal(map[string]string{
		"userName": "Alice",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["userName"] != "Alice" {
		t.Fatalf("expected userName Alice, got %v", resp["userName"])
	}
	token, _ := resp["token"].(string)
	session, ok := srv.session.Get(token)
	if !ok {
		t.Fatal("session missing")
	}
	if session.Mode != AuthModeAPIKey {
		t.Fatalf("expected apiKey mode for env user, got %q", session.Mode)
	}
}

func TestLoginHandler_EnvUserUnknown(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich"})
	raw, _ := json.Marshal(map[string]string{
		"userName": "Unknown",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestSessionStore_GetDeleteCleanup(t *testing.T) {
	store := NewSessionStore()
	token := store.CreateAPIKey("u", "k", "http://s")
	s1, ok := store.Get(token)
	if !ok {
		t.Fatal("missing session")
	}
	if s1.Mode != AuthModeAPIKey {
		t.Fatalf("expected apiKey mode")
	}
	// Second get should still succeed (sliding TTL)
	if _, ok := store.Get(token); !ok {
		t.Fatal("missing session on second get")
	}
	store.Delete(token)
	if _, ok := store.Get(token); ok {
		t.Fatal("deleted session still present")
	}
	// Cleanup should not panic on empty store
	store.Cleanup()
}
