package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ─── Config ────────────────────────────────────────────────────────────────

// Version is set at build time via -ldflags (e.g. -X main.Version=1.2.5)
var Version = "dev"

type UserConfig struct {
	Name   string
	APIKey string
}

type Config struct {
	ServerURL  string
	ListenAddr string
	StaticDir  string
	Users      []UserConfig
}

func loadConfig() Config {
	cfg := Config{
		ListenAddr: getEnv("LISTEN_ADDR", ":8080"),
		StaticDir:  getEnv("STATIC_DIR", "./dist"),
		ServerURL:  os.Getenv("IMMICH_SERVER_URL"),
	}
	for i := 1; ; i++ {
		// Primary naming: IMMICH_API_KEY_<N>_NAME / IMMICH_API_KEY_<N>_KEY
		name := os.Getenv(fmt.Sprintf("IMMICH_API_KEY_%d_NAME", i))
		key := os.Getenv(fmt.Sprintf("IMMICH_API_KEY_%d_KEY", i))

		// Fallback naming: IMMICH_USER_<N>_NAME / IMMICH_USER_<N>_API_KEY
		if name == "" {
			name = os.Getenv(fmt.Sprintf("IMMICH_USER_%d_NAME", i))
		}
		if key == "" {
			key = os.Getenv(fmt.Sprintf("IMMICH_USER_%d_API_KEY", i))
		}

		if name == "" || key == "" {
			break
		}
		cfg.Users = append(cfg.Users, UserConfig{Name: name, APIKey: key})
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ─── Session Store ─────────────────────────────────────────────────────────

type AuthMode string

const (
	AuthModeAPIKey      AuthMode = "apiKey"
	AuthModeAccessToken AuthMode = "accessToken"
)

type Session struct {
	UserName    string
	ServerURL   string
	ExpiresAt   time.Time
	Mode        AuthMode
	APIKey      string // set when Mode == AuthModeAPIKey
	AccessToken string // set when Mode == AuthModeAccessToken
	UserEmail   string
	UserID      string
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

func (s *SessionStore) CreateAPIKey(userName, apiKey, serverURL string) string {
	token := generateToken()
	s.mu.Lock()
	s.sessions[token] = &Session{
		UserName:  userName,
		APIKey:    apiKey,
		ServerURL: serverURL,
		Mode:      AuthModeAPIKey,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	s.mu.Unlock()
	return token
}

func (s *SessionStore) CreateAccessToken(userName, accessToken, serverURL, userEmail, userID string) string {
	token := generateToken()
	s.mu.Lock()
	s.sessions[token] = &Session{
		UserName:    userName,
		AccessToken: accessToken,
		ServerURL:   serverURL,
		Mode:        AuthModeAccessToken,
		UserEmail:   userEmail,
		UserID:      userID,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	s.mu.Unlock()
	return token
}

func (s *SessionStore) Get(token string) (*Session, bool) {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(session.ExpiresAt) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return nil, false
	}
	// Sliding expiration
	s.mu.Lock()
	session.ExpiresAt = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	return session, true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *SessionStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
}

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate token: %v", err))
	}
	return hex.EncodeToString(b)
}

// ─── Server ────────────────────────────────────────────────────────────────

type Server struct {
	config  Config
	session *SessionStore
}

func NewServer(cfg Config) *Server {
	return &Server{
		config:  cfg,
		session: NewSessionStore(),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/health":
		s.healthHandler(w, r)

	case path == "/api/auth/login":
		s.loginHandler(w, r)

	case path == "/api/auth/logout":
		s.logoutHandler(w, r)

	case path == "/api/auth/config":
		s.configHandler(w, r)

	case strings.HasPrefix(path, "/api/"):
		s.authMiddleware(http.HandlerFunc(s.proxyHandler)).ServeHTTP(w, r)

	default:
		s.staticHandler(w, r)
	}
}

// ─── Health ────────────────────────────────────────────────────────────────

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Config ────────────────────────────────────────────────────────────────

func (s *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	userNames := make([]string, len(s.config.Users))
	for i, u := range s.config.Users {
		userNames[i] = u.Name
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users":           userNames,
		"defaultServerUrl": s.config.ServerURL,
		"version":         Version,
	})
}

// ─── Auth ──────────────────────────────────────────────────────────────────

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read request body"})
		return
	}
	defer r.Body.Close()

	var req struct {
		UserName  string `json:"userName"`
		APIKey    string `json:"apiKey"`
		ServerURL string `json:"serverUrl"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	hasUserName := req.UserName != ""
	hasAPIKey := req.APIKey != ""
	hasEmail := req.Email != ""
	hasPassword := req.Password != ""

	// Reject ambiguous combinations
	if hasEmail && hasAPIKey {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide either email/password or apiKey, not both"})
		return
	}
	if hasEmail && hasUserName {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide either email/password or userName, not both"})
		return
	}
	if hasAPIKey && hasUserName {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide either userName or apiKey, not both"})
		return
	}
	if hasPassword && !hasEmail {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password requires email"})
		return
	}
	if hasEmail && !hasPassword {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email requires password"})
		return
	}

	// 1) Env user by name → API-key session
	if hasUserName {
		s.loginWithEnvUser(w, req.UserName)
		return
	}

	// 2) Manual API key → API-key session
	if hasAPIKey {
		s.loginWithAPIKey(w, req.APIKey, req.ServerURL)
		return
	}

	// 3) Immich email/password → access-token session
	if hasEmail {
		s.loginWithCredentials(w, req.Email, req.Password, req.ServerURL)
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide userName, apiKey, or email/password"})
}

func (s *Server) loginWithEnvUser(w http.ResponseWriter, userName string) {
	var apiKey string
	found := false
	for _, u := range s.config.Users {
		if u.Name == userName {
			apiKey = u.APIKey
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unknown user"})
		return
	}
	serverURL := s.config.ServerURL
	if serverURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no server URL configured"})
		return
	}

	valid, name, err := s.validateAPIKey(serverURL, apiKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot reach Immich server"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}
	if name != "" {
		userName = name
	}

	token := s.session.CreateAPIKey(userName, apiKey, serverURL)
	log.Printf("Login: mode=apiKey user=%q session=%s…", userName, token[:12])
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":     token,
		"userName":  userName,
		"serverUrl": serverURL,
	})
}

func (s *Server) loginWithAPIKey(w http.ResponseWriter, apiKey, serverURL string) {
	if serverURL == "" {
		serverURL = s.config.ServerURL
	}
	if serverURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no server URL configured"})
		return
	}

	valid, name, err := s.validateAPIKey(serverURL, apiKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot reach Immich server"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}

	userName := name
	if userName == "" {
		userName = "manual"
	}

	token := s.session.CreateAPIKey(userName, apiKey, serverURL)
	log.Printf("Login: mode=apiKey user=%q session=%s…", userName, token[:12])
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":     token,
		"userName":  userName,
		"serverUrl": serverURL,
	})
}

func (s *Server) loginWithCredentials(w http.ResponseWriter, email, password, serverURL string) {
	if serverURL == "" {
		serverURL = s.config.ServerURL
	}
	if serverURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no server URL configured"})
		return
	}

	loginResult, status, errMsg := s.immichPasswordLogin(serverURL, email, password)
	if errMsg != "" {
		writeJSON(w, status, map[string]string{"error": errMsg})
		return
	}

	// Validate token works via users/me
	valid, displayName, err := s.validateAccessToken(serverURL, loginResult.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot reach Immich server"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	userName := displayName
	if userName == "" {
		userName = loginResult.Name
	}
	if userName == "" {
		userName = loginResult.UserEmail
	}
	if userName == "" {
		userName = email
	}

	token := s.session.CreateAccessToken(userName, loginResult.AccessToken, serverURL, loginResult.UserEmail, loginResult.UserID)
	log.Printf("Login: mode=accessToken user=%q session=%s…", userName, token[:12])
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":     token,
		"userName":  userName,
		"serverUrl": serverURL,
	})
}

type immichLoginResponse struct {
	AccessToken string `json:"accessToken"`
	Name        string `json:"name"`
	UserEmail   string `json:"userEmail"`
	UserID      string `json:"userId"`
}

// immichPasswordLogin calls Immich POST /api/auth/login.
// Never logs email or password.
func (s *Server) immichPasswordLogin(serverURL, email, password string) (result immichLoginResponse, status int, errMsg string) {
	base := strings.TrimRight(serverURL, "/")
	targetURL := base + "/api/auth/login"

	payload, err := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	if err != nil {
		return result, http.StatusInternalServerError, "internal error"
	}

	req, err := http.NewRequest("POST", targetURL, strings.NewReader(string(payload)))
	if err != nil {
		return result, http.StatusInternalServerError, "internal error"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, http.StatusInternalServerError, "cannot reach Immich server"
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(respBody, &result); err != nil || result.AccessToken == "" {
			return result, http.StatusInternalServerError, "unexpected response from Immich"
		}
		return result, http.StatusOK, ""
	}

	// Map Immich errors without leaking details
	bodyLower := strings.ToLower(string(respBody))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if strings.Contains(bodyLower, "password") && (strings.Contains(bodyLower, "disabled") || strings.Contains(bodyLower, "not enabled") || strings.Contains(bodyLower, "unavailable")) {
			return result, http.StatusForbidden, "password login is disabled on this Immich server"
		}
		return result, http.StatusUnauthorized, "invalid email or password"
	}
	if resp.StatusCode == http.StatusBadRequest {
		if strings.Contains(bodyLower, "password") && (strings.Contains(bodyLower, "disabled") || strings.Contains(bodyLower, "not enabled") || strings.Contains(bodyLower, "unavailable")) {
			return result, http.StatusForbidden, "password login is disabled on this Immich server"
		}
		return result, http.StatusUnauthorized, "invalid email or password"
	}

	log.Printf("Immich password login failed: status=%d", resp.StatusCode)
	return result, http.StatusInternalServerError, "cannot reach Immich server"
}

func (s *Server) validateAPIKey(serverURL, apiKey string) (valid bool, userName string, err error) {
	return s.validateUserMe(serverURL, func(req *http.Request) {
		req.Header.Set("x-api-key", apiKey)
	})
}

func (s *Server) validateAccessToken(serverURL, accessToken string) (valid bool, userName string, err error) {
	return s.validateUserMe(serverURL, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	})
}

func (s *Server) validateUserMe(serverURL string, setAuth func(*http.Request)) (valid bool, userName string, err error) {
	base := strings.TrimRight(serverURL, "/")
	targetURL := base + "/api/users/me"

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return false, "", err
	}
	setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", nil
	}

	var userInfo struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if body, err := io.ReadAll(resp.Body); err == nil {
		_ = json.Unmarshal(body, &userInfo)
	}

	name := userInfo.Name
	if name == "" {
		name = userInfo.Email
	}
	return true, name, nil
}

func (s *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if session, ok := s.session.Get(token); ok {
			// Best-effort Immich logout for access-token sessions
			if session.Mode == AuthModeAccessToken && session.AccessToken != "" {
				s.immichLogout(session.ServerURL, session.AccessToken)
			}
			s.session.Delete(token)
		} else {
			// Session already gone — still succeed locally
			s.session.Delete(token)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// immichLogout best-effort calls Immich POST /api/auth/logout. Failures are ignored.
func (s *Server) immichLogout(serverURL, accessToken string) {
	base := strings.TrimRight(serverURL, "/")
	targetURL := base + "/api/auth/logout"
	req, err := http.NewRequest("POST", targetURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		session, ok := s.session.Get(token)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
			return
		}

		ctx := context.WithValue(r.Context(), sessionKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type sessionKey struct{}

func sessionFromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionKey{}).(*Session)
	return s
}

// ─── Proxy ─────────────────────────────────────────────────────────────────

func (s *Server) proxyHandler(w http.ResponseWriter, r *http.Request) {
	session := sessionFromContext(r.Context())
	if session == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session"})
		return
	}

	targetRaw := strings.TrimRight(session.ServerURL, "/")
	target, err := url.Parse(targetRaw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server URL"})
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			// Strip browser auth, then attach session Immich credentials.
			// Never forward the Swipe session Bearer to Immich.
			applySessionAuth(req, session)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
		},
	}

	proxy.ServeHTTP(w, r)
}

// ─── Static Files ──────────────────────────────────────────────────────────

func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cleanPath := filepath.Clean(r.URL.Path)
	staticDir := filepath.Clean(s.config.StaticDir)

	if cleanPath == "/" || cleanPath == "." {
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		return
	}

	fullPath := filepath.Join(staticDir, cleanPath)

	// Prevent directory traversal
	if !strings.HasPrefix(fullPath, staticDir) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		// SPA fallback
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		return
	}

	// Cache headers for static assets
	ext := strings.ToLower(filepath.Ext(fullPath))
	switch ext {
	case ".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".woff", ".woff2", ".ico":
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	}

	http.ServeFile(w, r, fullPath)
}

// stripClientAuthHeaders removes browser-supplied auth headers before the
// proxy attaches session credentials. Exported for tests via package-level use.
func stripClientAuthHeaders(req *http.Request) {
	req.Header.Del("Authorization")
	req.Header.Del("x-api-key")
	req.Header.Del("x-immich-user-token")
	req.Header.Del("x-immich-session-token")
	req.Header.Del("x-immich-share-key")
}

// applySessionAuth attaches Immich credentials from the session after stripping
// client auth headers. Used by the proxy Director and unit tests.
func applySessionAuth(req *http.Request, session *Session) {
	stripClientAuthHeaders(req)
	if session == nil {
		return
	}
	switch session.Mode {
	case AuthModeAccessToken:
		req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	default:
		req.Header.Set("x-api-key", session.APIKey)
	}
}

// ─── JSON Helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// ─── Main ──────────────────────────────────────────────────────────────────

func main() {
	cfg := loadConfig()
	srv := NewServer(cfg)

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Periodic session cleanup
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			srv.session.Cleanup()
		}
	}()

	go func() {
		log.Printf("Immich Swipe server v%s starting on %s", Version, cfg.ListenAddr)
		log.Printf("  Static dir: %s", cfg.StaticDir)
		log.Printf("  Users configured: %d", len(cfg.Users))
		if cfg.ServerURL != "" {
			log.Printf("  Default Immich URL: %s", cfg.ServerURL)
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-sigCh
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	log.Println("Server stopped")
}
