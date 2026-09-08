package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fakeImmichOAuth serves the Immich OAuth endpoints plus users/me and
// public/config. oauthEnabled toggles the public config response.
func fakeImmichOAuth(t *testing.T, oauthEnabled bool) *httptest.Server {
	t.Helper()
	var authorizeBody map[string]string
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/api/public/config"):
			w.Header().Set("Content-Type", "application/json")
			if oauthEnabled {
				_, _ = w.Write([]byte(`{"oauth":{"enabled":true,"buttonText":"Login with SSO"}}`))
			} else {
				_, _ = w.Write([]byte(`{"oauth":{"enabled":false,"buttonText":""}}`))
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/oauth/authorize"):
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &authorizeBody)
			if authorizeBody["redirectUri"] == "" || authorizeBody["state"] == "" || authorizeBody["codeChallenge"] == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "immich_oauth_state", Value: "state-cookie"})
			http.SetCookie(w, &http.Cookie{Name: "immich_oauth_code_verifier", Value: "verifier-cookie"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"url":"https://idp.example/authorize?client_id=x"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/oauth/callback"):
			body, _ := io.ReadAll(r.Body)
			var payload map[string]string
			_ = json.Unmarshal(body, &payload)
			if payload["url"] == "" || payload["state"] == "" || payload["codeVerifier"] == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if r.Header.Get("Cookie") == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"missing oauth cookies"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"accessToken":"immich-oauth-jwt",
				"name":"SSO User",
				"userEmail":"sso@example.com",
				"userId":"uid-oauth"
			}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/api/users/me"):
			if r.Header.Get("Authorization") != "Bearer immich-oauth-jwt" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"SSO User","email":"sso@example.com"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestOAuthStart_HappyPath(t *testing.T) {
	immich := fakeImmichOAuth(t, true)
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/start", strings.NewReader(`{"serverUrl":""}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.oauthStartHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var data struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil || data.URL == "" || data.State == "" {
		t.Fatalf("expected url+state, got %s err=%v", rr.Body.String(), err)
	}
}

func TestOAuthStart_Disabled(t *testing.T) {
	immich := fakeImmichOAuth(t, false)
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/start", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	srv.oauthStartHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var data map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &data)
	if data["code"] != "oauth_not_enabled" {
		t.Fatalf("expected oauth_not_enabled, got %s", rr.Body.String())
	}
}

func TestOAuthCallback_BadState(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich.example"})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/callback?code=abc&state=unknown", nil)
	rr := httptest.NewRecorder()
	srv.oauthCallbackHandler(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login?oauthError=invalid_state" {
		t.Fatalf("expected invalid_state redirect, got %q", loc)
	}
}

func oauthFullFlow(t *testing.T, srv *Server) (callbackLocation string) {
	t.Helper()
	startReq := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/start", strings.NewReader(`{}`))
	startReq.Header.Set("Content-Type", "application/json")
	startRR := httptest.NewRecorder()
	srv.oauthStartHandler(startRR, startReq)
	if startRR.Code != http.StatusOK {
		t.Fatalf("start failed: %d %s", startRR.Code, startRR.Body.String())
	}
	var start struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(startRR.Body.Bytes(), &start)

	cbReq := httptest.NewRequest(http.MethodGet,
		"/api/auth/oauth/callback?code=idp-code&state="+url.QueryEscape(start.State), nil)
	cbRR := httptest.NewRecorder()
	srv.oauthCallbackHandler(cbRR, cbReq)
	if cbRR.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d body=%s", cbRR.Code, cbRR.Body.String())
	}
	return cbRR.Header().Get("Location")
}

func TestOAuthCallback_HappyPathAndFinish(t *testing.T) {
	immich := fakeImmichOAuth(t, true)
	defer immich.Close()
	srv := NewServer(Config{ServerURL: immich.URL})

	loc := oauthFullFlow(t, srv)
	if !strings.HasPrefix(loc, "/login?oauthCode=") {
		t.Fatalf("expected oauthCode redirect, got %q", loc)
	}
	if strings.Contains(loc, "immich-oauth-jwt") || strings.Contains(loc, "session") {
		t.Fatalf("redirect must not leak tokens: %q", loc)
	}
	handoff := strings.TrimPrefix(loc, "/login?oauthCode=")

	finishReq := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/finish",
		strings.NewReader(`{"code":"`+handoff+`"}`))
	finishReq.Header.Set("Content-Type", "application/json")
	finishRR := httptest.NewRecorder()
	srv.oauthFinishHandler(finishRR, finishReq)

	if finishRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", finishRR.Code, finishRR.Body.String())
	}
	var data struct {
		Token     string `json:"token"`
		UserName  string `json:"userName"`
		ServerURL string `json:"serverUrl"`
		Mode      string `json:"mode"`
	}
	if err := json.Unmarshal(finishRR.Body.Bytes(), &data); err != nil {
		t.Fatalf("invalid finish payload: %v", err)
	}
	if data.Mode != "accessToken" || data.UserName != "SSO User" || data.ServerURL != immich.URL || data.Token == "" {
		t.Fatalf("unexpected finish payload: %s", finishRR.Body.String())
	}
	if _, ok := srv.session.Get(data.Token); !ok {
		t.Fatalf("expected swipe session to exist")
	}

	// Single-use: second exchange fails.
	reuseReq := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/finish",
		strings.NewReader(`{"code":"`+handoff+`"}`))
	reuseRR := httptest.NewRecorder()
	srv.oauthFinishHandler(reuseRR, reuseReq)
	if reuseRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on reuse, got %d", reuseRR.Code)
	}
	var errData map[string]string
	_ = json.Unmarshal(reuseRR.Body.Bytes(), &errData)
	if errData["code"] != "invalid_code" {
		t.Fatalf("expected invalid_code, got %s", reuseRR.Body.String())
	}
}

func TestOAuthFinish_UnknownCode(t *testing.T) {
	srv := NewServer(Config{ServerURL: "http://immich.example"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/finish", strings.NewReader(`{"code":"nope"}`))
	rr := httptest.NewRecorder()
	srv.oauthFinishHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestConfigHandler_OAuthFields(t *testing.T) {
	immich := fakeImmichOAuth(t, true)
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rr := httptest.NewRecorder()
	srv.configHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var data struct {
		OAuthEnabled    bool   `json:"oauthEnabled"`
		OAuthButtonText string `json:"oauthButtonText"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatalf("invalid config payload: %v", err)
	}
	if !data.OAuthEnabled || data.OAuthButtonText != "Login with SSO" {
		t.Fatalf("unexpected oauth config: %s", rr.Body.String())
	}
}
