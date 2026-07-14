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

type Session struct {
	UserName  string
	APIKey    string
	ServerURL string
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

func (s *SessionStore) Create(userName, apiKey, serverURL string) string {
	token := generateToken()
	s.mu.Lock()
	s.sessions[token] = &Session{
		UserName:  userName,
		APIKey:    apiKey,
		ServerURL: serverURL,
		ExpiresAt: time.Now().Add(24 * time.Hour),
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
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	var apiKey, serverURL, userName string

	if req.UserName != "" {
		found := false
		for _, u := range s.config.Users {
			if u.Name == req.UserName {
				userName = u.Name
				apiKey = u.APIKey
				found = true
				break
			}
		}
		if !found {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unknown user"})
			return
		}
		serverURL = s.config.ServerURL
	} else if req.APIKey != "" {
		apiKey = req.APIKey
		serverURL = req.ServerURL
		if serverURL == "" {
			serverURL = s.config.ServerURL
		}
		userName = "manual"
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide userName or apiKey"})
		return
	}

	if serverURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no server URL configured"})
		return
	}

	valid, name, err := s.validateCredentials(serverURL, apiKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("cannot reach Immich server: %v", err)})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}

	if name != "" && req.UserName == "" {
		userName = name
	}

	token := s.session.Create(userName, apiKey, serverURL)
	log.Printf("Login: user=%q session=%s…", userName, token[:12])

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":     token,
		"userName":  userName,
		"serverUrl": serverURL,
	})
}

func (s *Server) validateCredentials(serverURL, apiKey string) (valid bool, userName string, err error) {
	base := strings.TrimRight(serverURL, "/")
	targetURL := base + "/api/users/me"

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Set("x-api-key", apiKey)
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
		Name string `json:"name"`
	}
	if body, err := io.ReadAll(resp.Body); err == nil {
		json.Unmarshal(body, &userInfo)
	}

	return true, userInfo.Name, nil
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
			// Strip browser-originated auth headers so Immich's auth guard
			// only sees the server-side x-api-key. Immich checks
			// Authorization: Bearer before x-api-key, so forwarding the
			// browser's Go session token would cause a 401 even with a
			// valid API key.
			req.Header.Del("Authorization")
			req.Header.Del("x-immich-user-token")
			req.Header.Del("x-immich-session-token")
			req.Header.Del("x-immich-share-key")
			req.Header.Set("x-api-key", session.APIKey)
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
