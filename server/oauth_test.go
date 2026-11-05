package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
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
	if loc := rr.Header().Get("Location"); loc != "http://example.com/login?oauthError=invalid_state" {
		t.Fatalf("expected absolute invalid_state redirect, got %q", loc)
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
	parsed, err := url.Parse(loc)
	if err != nil || parsed.Path != "/login" || parsed.Query().Get("oauthCode") == "" {
		t.Fatalf("expected absolute oauthCode redirect, got %q", loc)
	}
	if strings.Contains(loc, "immich-oauth-jwt") || strings.Contains(loc, "session") {
		t.Fatalf("redirect must not leak tokens: %q", loc)
	}
	handoff := parsed.Query().Get("oauthCode")

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

// TestOAuthCallback_ReusesStartRedirectURI guards against re-deriving the
// callback URL from the incoming callback request's headers: behind a proxy
// those can differ from the start request, making Immich's token exchange send
// a redirect_uri the IdP never saw.
func TestOAuthCallback_ReusesStartRedirectURI(t *testing.T) {
	var authorizeRedirect, callbackURL string
	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/public/config"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"oauth":{"enabled":true,"buttonText":"SSO"}}`))
		case strings.HasSuffix(r.URL.Path, "/api/oauth/authorize"):
			body, _ := io.ReadAll(r.Body)
			var payload map[string]string
			_ = json.Unmarshal(body, &payload)
			authorizeRedirect = payload["redirectUri"]
			http.SetCookie(w, &http.Cookie{Name: "immich_oauth_state", Value: "state-cookie"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"url":"https://idp.example/authorize"}`))
		case strings.HasSuffix(r.URL.Path, "/api/oauth/callback"):
			body, _ := io.ReadAll(r.Body)
			var payload map[string]string
			_ = json.Unmarshal(body, &payload)
			callbackURL = payload["url"]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"accessToken":"tok","name":"SSO User","userEmail":"sso@example.com","userId":"uid"}`))
		case strings.HasSuffix(r.URL.Path, "/api/users/me"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"SSO User","email":"sso@example.com"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL})

	startReq := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/start", strings.NewReader(`{}`))
	startReq.Header.Set("Content-Type", "application/json")
	startReq.Header.Set("X-Forwarded-Proto", "https")
	startReq.Host = "start.example"
	startRR := httptest.NewRecorder()
	srv.oauthStartHandler(startRR, startReq)
	if startRR.Code != http.StatusOK {
		t.Fatalf("start failed: %d %s", startRR.Code, startRR.Body.String())
	}
	if authorizeRedirect != "https://start.example/api/auth/oauth/callback" {
		t.Fatalf("unexpected authorize redirectUri: %q", authorizeRedirect)
	}
	var start struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(startRR.Body.Bytes(), &start)

	// The IdP redirect arrives on a different host/proto than the start request,
	// and includes the RFC 9207 `iss` param (Authelia sends it) that must be
	// forwarded verbatim or Immich's oauth4webapi rejects it as OAUTH_INVALID_RESPONSE.
	rawQuery := "code=idp-code&state=" + url.QueryEscape(start.State) + "&iss=" + url.QueryEscape("https://idp.example")
	cbReq := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/callback?"+rawQuery, nil)
	cbReq.Host = "proxy.example"
	cbRR := httptest.NewRecorder()
	srv.oauthCallbackHandler(cbRR, cbReq)
	if cbRR.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d body=%s", cbRR.Code, cbRR.Body.String())
	}
	if !strings.HasPrefix(callbackURL, authorizeRedirect+"?") {
		t.Fatalf("callback url must reuse the start redirect_uri\n got: %q\nwant prefix: %q", callbackURL, authorizeRedirect)
	}
	parsedCallback, err := url.Parse(callbackURL)
	if err != nil {
		t.Fatalf("invalid callback url %q: %v", callbackURL, err)
	}
	gotQuery := parsedCallback.Query()
	if gotQuery.Get("iss") != "https://idp.example" {
		t.Fatalf("callback url must forward the IdP iss param, got %q", callbackURL)
	}
	if gotQuery.Get("code") != "idp-code" || gotQuery.Get("state") != start.State {
		t.Fatalf("callback url must forward code+state, got %q", callbackURL)
	}
}

// startOAuth drives POST /api/auth/oauth/start and returns the state param.
func startOAuth(t *testing.T, srv *Server, body string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.oauthStartHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("start failed: %d %s", rr.Code, rr.Body.String())
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("start must be no-store, got %q", cc)
	}
	var start struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &start); err != nil || start.State == "" {
		t.Fatalf("invalid start payload: %s err=%v", rr.Body.String(), err)
	}
	return start.State
}

func TestOAuthCallback_FormPost(t *testing.T) {
	immich := fakeImmichOAuth(t, true)
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL, PublicURL: "https://swipe.example"})
	state := startOAuth(t, srv, `{}`)

	form := url.Values{
		"code":  {"idp-code"},
		"state": {state},
		"iss":   {"https://idp.example"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// A browser arriving from the IdP carries the CSRF cookie but cannot send
	// the double-submit header; the callback must be exempt from that check.
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "browser-csrf"})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after form_post, got %d body=%s", rr.Code, rr.Body.String())
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("callback must be no-store, got %q", cc)
	}
	loc := rr.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil || parsed.Path != "/login" || parsed.Query().Get("oauthCode") == "" {
		t.Fatalf("expected absolute /login?oauthCode= redirect, got %q", loc)
	}
	if !strings.HasPrefix(loc, "https://swipe.example/login?oauthCode=") {
		t.Fatalf("redirect must be absolute via PublicURL, got %q", loc)
	}

	finishReq := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/finish",
		strings.NewReader(`{"code":"`+parsed.Query().Get("oauthCode")+`"}`))
	finishRR := httptest.NewRecorder()
	srv.oauthFinishHandler(finishRR, finishReq)
	if finishRR.Code != http.StatusOK {
		t.Fatalf("expected handoff code to be valid, got %d %s", finishRR.Code, finishRR.Body.String())
	}
	if cc := finishRR.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("finish must be no-store, got %q", cc)
	}
}

func TestOAuthCallback_ForwardsIdPError(t *testing.T) {
	immich := fakeImmichOAuth(t, true)
	defer immich.Close()
	srv := NewServer(Config{ServerURL: immich.URL, PublicURL: "https://swipe.example"})
	state := startOAuth(t, srv, `{}`)

	query := url.Values{
		"error":             {"access_denied"},
		"error_description": {"User denied access"},
		"error_uri":         {"https://idp.example/errors/denied"},
		"state":             {state},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/callback?"+query.Encode(), nil)
	rr := httptest.NewRecorder()
	srv.oauthCallbackHandler(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("invalid redirect %q: %v", loc, err)
	}
	got := parsed.Query()
	if got.Get("oauthError") != "access_denied" {
		t.Fatalf("expected forwarded error, got %q", loc)
	}
	if got.Get("oauthErrorDescription") != "User denied access" {
		t.Fatalf("expected forwarded description, got %q", loc)
	}
	if got.Get("oauthErrorUri") != "https://idp.example/errors/denied" {
		t.Fatalf("expected forwarded error_uri, got %q", loc)
	}
}

func TestOAuthCallback_MobileHandoff(t *testing.T) {
	immich := fakeImmichOAuth(t, true)
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL, PublicURL: "https://swipe.example"})
	state := startOAuth(t, srv, `{"client":"mobile"}`)

	req := httptest.NewRequest(http.MethodGet,
		"/api/auth/oauth/callback?code=idp-code&state="+url.QueryEscape(state), nil)
	rr := httptest.NewRecorder()
	srv.oauthCallbackHandler(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil || parsed.Path != "/oauth/mobile" {
		t.Fatalf("expected mobile handoff, got %q", loc)
	}
	if !strings.HasPrefix(loc, "https://swipe.example/oauth/mobile?") {
		t.Fatalf("expected absolute mobile URL, got %q", loc)
	}
	handoff := parsed.Query().Get("code")
	if handoff == "" {
		t.Fatalf("expected one-time code, got %q", loc)
	}

	pageReq := httptest.NewRequest(http.MethodGet, "/oauth/mobile?code="+url.QueryEscape(handoff), nil)
	pageRR := httptest.NewRecorder()
	srv.ServeHTTP(pageRR, pageReq)
	if pageRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from mobile page, got %d", pageRR.Code)
	}
	if ct := pageRR.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected HTML page, got %q", ct)
	}
	if cc := pageRR.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("mobile page must be no-store, got %q", cc)
	}
	body := pageRR.Body.String()
	if !strings.Contains(body, handoff) {
		t.Fatalf("mobile page must display the one-time code")
	}
	if strings.Contains(body, "/api/") || strings.Contains(body, "<script") {
		t.Fatalf("mobile page must not be the SPA or run scripts")
	}

	// The code is still usable by the app (the page must not consume it).
	finishReq := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/finish",
		strings.NewReader(`{"code":"`+handoff+`"}`))
	finishRR := httptest.NewRecorder()
	srv.oauthFinishHandler(finishRR, finishReq)
	if finishRR.Code != http.StatusOK {
		t.Fatalf("expected mobile code to remain valid, got %d %s", finishRR.Code, finishRR.Body.String())
	}
}

func TestOAuthCallback_MobileErrorPage(t *testing.T) {
	srv := NewServer(Config{PublicURL: "https://swipe.example"})
	req := httptest.NewRequest(http.MethodGet,
		"/oauth/mobile?oauthError=access_denied&oauthErrorDescription=User+denied", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "access_denied") || !strings.Contains(body, "User denied") {
		t.Fatalf("expected error details on the mobile page, got %s", body)
	}
}

func TestOAuthCallback_PendingSurvivesFailedExchange(t *testing.T) {
	var callbacks int32
	immich := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/public/config"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"oauth":{"enabled":true,"buttonText":"SSO"}}`))
		case strings.HasSuffix(r.URL.Path, "/api/oauth/authorize"):
			http.SetCookie(w, &http.Cookie{Name: "immich_oauth_state", Value: "state-cookie"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"url":"https://idp.example/authorize"}`))
		case strings.HasSuffix(r.URL.Path, "/api/oauth/callback"):
			if atomic.AddInt32(&callbacks, 1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"accessToken":"tok","name":"SSO User","userEmail":"sso@example.com","userId":"uid"}`))
		case strings.HasSuffix(r.URL.Path, "/api/users/me"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"SSO User","email":"sso@example.com"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer immich.Close()

	srv := NewServer(Config{ServerURL: immich.URL, PublicURL: "https://swipe.example"})
	state := startOAuth(t, srv, `{}`)
	raw := "/api/auth/oauth/callback?code=idp-code&state=" + url.QueryEscape(state)

	firstRR := httptest.NewRecorder()
	srv.oauthCallbackHandler(firstRR, httptest.NewRequest(http.MethodGet, raw, nil))
	if firstRR.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", firstRR.Code)
	}
	firstLoc := firstRR.Header().Get("Location")
	if !strings.Contains(firstLoc, "oauthError=oauth_exchange_failed") {
		t.Fatalf("expected oauth_exchange_failed, got %q", firstLoc)
	}

	srv.oauthMu.Lock()
	_, stillPending := srv.oauthPending[state]
	srv.oauthMu.Unlock()
	if !stillPending {
		t.Fatalf("pending state must survive a transient exchange failure")
	}

	secondRR := httptest.NewRecorder()
	srv.oauthCallbackHandler(secondRR, httptest.NewRequest(http.MethodGet, raw, nil))
	if secondRR.Code != http.StatusFound {
		t.Fatalf("expected 302 on retry, got %d", secondRR.Code)
	}
	secondLoc := secondRR.Header().Get("Location")
	if !strings.Contains(secondLoc, "oauthCode=") {
		t.Fatalf("retry should succeed with a handoff code, got %q", secondLoc)
	}

	srv.oauthMu.Lock()
	_, stillPending = srv.oauthPending[state]
	srv.oauthMu.Unlock()
	if stillPending {
		t.Fatalf("pending state must be consumed after a successful exchange")
	}
}

func TestOAuthPublicBase_HonorsPathsAndForwardedPrefix(t *testing.T) {
	srv := NewServer(Config{PublicURL: "https://public.example/app/"})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/callback", nil)
	if got := srv.oauthPublicBase(req); got != "https://public.example/app" {
		t.Fatalf("expected PublicURL path preserved, got %q", got)
	}

	srv = NewServer(Config{})
	req.Host = "internal:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "swipe.example")
	req.Header.Set("X-Forwarded-Prefix", "/swipe")
	if got := srv.oauthPublicBase(req); got != "https://swipe.example/swipe" {
		t.Fatalf("expected forwarded prefix, got %q", got)
	}
}
