package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasswordHashRoundtrip(t *testing.T) {
	hash := hashPassword("secret123")
	if hash == "secret123" {
		t.Fatal("password hash must never equal the plaintext password")
	}
	if !strings.HasPrefix(hash, "pbkdf2$") {
		t.Fatalf("expected pbkdf2$ prefix, got %q", hash)
	}
	if !verifyPasswordHash(hash, "secret123") {
		t.Fatal("expected correct password to verify")
	}
	if verifyPasswordHash(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail verification")
	}
	if verifyPasswordHash("garbage", "secret123") {
		t.Fatal("expected malformed hash to fail closed")
	}
	if verifyPasswordHash("", "secret123") {
		t.Fatal("expected empty hash to fail closed")
	}
}

func TestNewAccountStore_MigratesEnvUsers(t *testing.T) {
	store := NewAccountStore(nil, []UserConfig{
		{Name: "Alice", APIKey: "key-alice"},
		{Name: "Bob", APIKey: "key-bob"},
	}, "http://immich.example")

	alice, ok := store.Get("http://immich.example", "Alice")
	if !ok {
		t.Fatal("expected migrated account for Alice")
	}
	if alice.APIKey != "key-alice" {
		t.Fatalf("expected env api key, got %q", alice.APIKey)
	}
	if store.HasPassword("http://immich.example", "Alice") {
		t.Fatal("migrated accounts must not have a password until set")
	}
	if _, ok := store.Get("http://immich.example", "Bob"); !ok {
		t.Fatal("expected migrated account for Bob")
	}
	// Migration must not touch a password a person already set.
	store.SetPassword("http://immich.example", "Carol", "key-carol", "secret123")
	NewAccountStore(nil, []UserConfig{{Name: "Carol", APIKey: "key-carol"}}, "http://immich.example")
	if !store.HasPassword("http://immich.example", "Carol") {
		t.Fatal("migration must not clobber existing passwords")
	}
}

func TestAccountStore_SetAndVerifyPassword(t *testing.T) {
	store := NewAccountStore(nil, nil, "")
	store.SetPassword("http://immich.example", "Alice", "key-alice", "secret123")

	if !store.HasPassword("http://immich.example", "Alice") {
		t.Fatal("expected HasPassword after SetPassword")
	}
	account, ok := store.VerifyPassword("http://immich.example", "Alice", "secret123")
	if !ok {
		t.Fatal("expected correct password to verify")
	}
	if account.APIKey != "key-alice" {
		t.Fatalf("expected api key bound to account, got %q", account.APIKey)
	}
	if _, ok := store.VerifyPassword("http://immich.example", "Alice", "nope"); ok {
		t.Fatal("expected wrong password to fail")
	}
	if _, ok := store.VerifyPassword("http://immich.example", "Unknown", "secret123"); ok {
		t.Fatal("expected unknown user to fail")
	}
	if _, ok := store.Get("http://immich.example", "Alice"); !ok {
		t.Fatal("expected account to exist")
	}
}

// newImmichStub spins up an Immich upstream that accepts any API key on
// /api/users/me and returns the caller's identity from the header.
func newImmichStub(t *testing.T) *httptest.Server {
	t.Helper()
	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/api/users/me"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"Alice","email":"alice@example.com"}`))
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/api/auth/logout"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(immich.Close)
	return immich
}

func TestLoginHandler_AccountLoginSuccess(t *testing.T) {
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
	var resp struct {
		Token     string `json:"token"`
		UserName  string `json:"userName"`
		ServerURL string `json:"serverUrl"`
		Mode      string `json:"mode"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" {
		t.Fatal("expected a session token")
	}
	if resp.Mode != "apiKey" {
		t.Fatalf("expected mode apiKey, got %q", resp.Mode)
	}

	// The session must carry the account's API key upstream.
	proxyReq := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	proxyReq.Header.Set("Authorization", "Bearer "+resp.Token)
	proxyRR := httptest.NewRecorder()
	srv.ServeHTTP(proxyRR, proxyReq)
	if proxyRR.Code != http.StatusOK {
		t.Fatalf("expected proxied users/me 200, got %d body=%s", proxyRR.Code, proxyRR.Body.String())
	}
}

func TestLoginHandler_AccountLoginFailures(t *testing.T) {
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})
	srv.accounts.SetPassword(immich.URL, "Alice", "key-alice", "secret123")

	cases := []struct {
		name       string
		body       map[string]string
		wantStatus int
		wantCode   string
	}{
		{"unknown user", map[string]string{"userName": "Nobody", "password": "secret123"}, http.StatusUnauthorized, "unknown_user"},
		{"wrong password", map[string]string{"userName": "Alice", "password": "wrong"}, http.StatusUnauthorized, "invalid_password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			srv.loginHandler(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tc.wantStatus, rr.Code, rr.Body.String())
			}
			var resp map[string]string
			_ = json.Unmarshal(rr.Body.Bytes(), &resp)
			if resp["code"] != tc.wantCode {
				t.Fatalf("expected code %q, got %q body=%s", tc.wantCode, resp["code"], rr.Body.String())
			}
		})
	}

	// A migrated account (env user, no password yet) cannot be claimed via
	// userName+password: it must first set a password from a session.
	t.Run("no password set", func(t *testing.T) {
		migrated := NewServer(Config{ServerURL: immich.URL, Users: []UserConfig{{Name: "Migrated", APIKey: "key-migrated"}}})
		raw, _ := json.Marshal(map[string]string{"userName": "Migrated", "password": "secret123"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		migrated.loginHandler(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["code"] != "password_not_set" {
			t.Fatalf("expected code password_not_set, got %q body=%s", resp["code"], rr.Body.String())
		}
	})
}

func TestLoginHandler_EnvUserPasswordRequired(t *testing.T) {
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL, Users: []UserConfig{{Name: "Alice", APIKey: "key-alice"}}})

	// Without a password: legacy auto-login keeps working.
	raw, _ := json.Marshal(map[string]string{"userName": "Alice"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected legacy env auto-login 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	// Once a password is set, userName alone must be rejected.
	srv.accounts.SetPassword(immich.URL, "Alice", "key-alice", "secret123")
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.loginHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 password_required, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["code"] != "password_required" {
		t.Fatalf("expected code password_required, got %q", resp["code"])
	}
}

func TestAccountHandler(t *testing.T) {
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})

	// Unauthenticated → 401.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/account", strings.NewReader(`{"password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rr.Code)
	}

	// Access-token session → unsupported_mode.
	token := srv.session.CreateAccessToken("Alice", "immich-token", immich.URL, "a@b.c", "uid-1")
	req = httptest.NewRequest(http.MethodPost, "/api/auth/account", strings.NewReader(`{"password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 unsupported_mode for access-token session, got %d body=%s", rr.Code, rr.Body.String())
	}

	// API-key session: set a password.
	token = srv.session.CreateAPIKey("Alice", "key-alice", immich.URL)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/account", strings.NewReader(`{"password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 setting password, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !srv.accounts.HasPassword(immich.URL, "Alice") {
		t.Fatal("expected password to be set")
	}
	if _, ok := srv.accounts.VerifyPassword(immich.URL, "Alice", "secret123"); !ok {
		t.Fatal("expected stored password to verify")
	}

	// Changing it without the current password → 401.
	req = httptest.NewRequest(http.MethodPost, "/api/auth/account", strings.NewReader(`{"password":"newsecret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 current_password_required, got %d body=%s", rr.Code, rr.Body.String())
	}

	// Changing it with the current password → 200.
	req = httptest.NewRequest(http.MethodPost, "/api/auth/account", strings.NewReader(`{"password":"newsecret","currentPassword":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 changing password, got %d body=%s", rr.Code, rr.Body.String())
	}
	if _, ok := srv.accounts.VerifyPassword(immich.URL, "Alice", "newsecret"); !ok {
		t.Fatal("expected new password to verify")
	}

	// Too short → 400 weak_password.
	req = httptest.NewRequest(http.MethodPost, "/api/auth/account", strings.NewReader(`{"password":"short"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 weak_password, got %d body=%s", rr.Code, rr.Body.String())
	}

	// The account must never store the plaintext password.
	account, _ := srv.accounts.Get(immich.URL, "Alice")
	if account.PasswordHash != nil && strings.Contains(*account.PasswordHash, "newsecret") {
		t.Fatal("plaintext password leaked into stored hash")
	}
}

// postLogin posts a raw login body and returns the recorder.
func postLogin(srv *Server, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.loginHandler(rr, req)
	return rr
}

func TestLoginHandler_AccountCreateSuccess(t *testing.T) {
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})

	rr := postLogin(srv, `{"userName":"Bob","password":"secret123","apiKey":"key-bob","serverUrl":"`+immich.URL+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token     string `json:"token"`
		UserName  string `json:"userName"`
		ServerURL string `json:"serverUrl"`
		Mode      string `json:"mode"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" || resp.Mode != "apiKey" {
		t.Fatalf("expected apiKey session token, got %+v", resp)
	}

	// Account created with hashed password and bound key.
	if !srv.accounts.HasPassword(immich.URL, "Bob") {
		t.Fatal("expected account password to be set")
	}
	account, ok := srv.accounts.VerifyPassword(immich.URL, "Bob", "secret123")
	if !ok {
		t.Fatal("expected created account password to verify")
	}
	if account.APIKey != "key-bob" {
		t.Fatalf("expected bound api key, got %q", account.APIKey)
	}

	// The session proxies with the account's API key.
	proxyReq := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	proxyReq.Header.Set("Authorization", "Bearer "+resp.Token)
	proxyRR := httptest.NewRecorder()
	srv.ServeHTTP(proxyRR, proxyReq)
	if proxyRR.Code != http.StatusOK {
		t.Fatalf("expected proxied users/me 200, got %d body=%s", proxyRR.Code, proxyRR.Body.String())
	}
}

func TestLoginHandler_AccountCreateFailures(t *testing.T) {
	immich := newImmichStub(t)
	srv := NewServer(Config{ServerURL: immich.URL})

	t.Run("weak password", func(t *testing.T) {
		rr := postLogin(srv, `{"userName":"Bob","password":"short","apiKey":"key","serverUrl":"`+immich.URL+`"}`)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["code"] != "weak_password" {
			t.Fatalf("expected weak_password, got %q", resp["code"])
		}
	})

	t.Run("invalid api key", func(t *testing.T) {
		// Immich stub that rejects unknown keys.
		strict := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-api-key") == "good-key" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"Alice","email":"a@b.c"}`))
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer strict.Close()
		strictSrv := NewServer(Config{ServerURL: strict.URL})

		rr := postLogin(strictSrv, `{"userName":"NewUser","password":"secret123","apiKey":"bad-key","serverUrl":"`+strict.URL+`"}`)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["code"] != "invalid_api_key" {
			t.Fatalf("expected invalid_api_key, got %q", resp["code"])
		}
		if srv.accounts.HasPassword(strict.URL, "NewUser") {
			t.Fatal("no account must be created for an invalid API key")
		}
	})

	t.Run("existing password cannot be re-claimed", func(t *testing.T) {
		srv.accounts.SetPassword(immich.URL, "Alice", "key-alice", "secret123")
		rr := postLogin(srv, `{"userName":"Alice","password":"anotherpass","apiKey":"key-alice","serverUrl":"`+immich.URL+`"}`)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["code"] != "account_exists" {
			t.Fatalf("expected account_exists, got %q", resp["code"])
		}
		// The existing password is untouched.
		if _, ok := srv.accounts.VerifyPassword(immich.URL, "Alice", "secret123"); !ok {
			t.Fatal("existing password must be unchanged")
		}
	})
}

func TestLoginHandler_AccountCreateClaimMigratedUser(t *testing.T) {
	immich := newImmichStub(t)

	// Env user migrates into accounts without a password, bound to its key.
	srv := NewServer(Config{ServerURL: immich.URL, Users: []UserConfig{{Name: "Martyn", APIKey: "key-martyn"}}})

	t.Run("matching key claims the account", func(t *testing.T) {
		rr := postLogin(srv, `{"userName":"Martyn","password":"secret123","apiKey":"key-martyn","serverUrl":"`+immich.URL+`"}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		if !srv.accounts.HasPassword(immich.URL, "Martyn") {
			t.Fatal("expected account to be claimed with a password")
		}
	})

	t.Run("foreign key cannot claim the account", func(t *testing.T) {
		// Fresh server so the account still has no password.
		fresh := NewServer(Config{ServerURL: immich.URL, Users: []UserConfig{{Name: "Martyn", APIKey: "key-martyn"}}})
		rr := postLogin(fresh, `{"userName":"Martyn","password":"secret123","apiKey":"evil-key","serverUrl":"`+immich.URL+`"}`)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
		}
		var resp map[string]string
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["code"] != "invalid_api_key" {
			t.Fatalf("expected invalid_api_key, got %q", resp["code"])
		}
		if fresh.accounts.HasPassword(immich.URL, "Martyn") {
			t.Fatal("account must not be claimed with a foreign key")
		}
	})
}
