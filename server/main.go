package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
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

	_ "modernc.org/sqlite"
)

// ─── Config ────────────────────────────────────────────────────────────────

// Version is set at build time via -ldflags (e.g. -X main.Version=1.2.5)
var Version = "dev"

type UserConfig struct {
	Name   string
	APIKey string
}

type Config struct {
	ServerURL     string
	ListenAddr    string
	StaticDir     string
	StatsFile     string // optional TRMNL stats persistence path ("" = memory only)
	SessionDBFile string // optional IMMICH_SESSIONS_DB SQLite path ("" = in-memory sessions)
	Users         []UserConfig
}

func loadConfig() Config {
	cfg := Config{
		ListenAddr:    getEnv("LISTEN_ADDR", ":8080"),
		StaticDir:     getEnv("STATIC_DIR", "./dist"),
		ServerURL:     os.Getenv("IMMICH_SERVER_URL"),
		StatsFile:     os.Getenv("TRMNL_STATS_FILE"),
		SessionDBFile: os.Getenv("IMMICH_SESSIONS_DB"),
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

// sessionTTL is the sliding session lifetime: every authenticated request
// extends the session by this amount.
const sessionTTL = 24 * time.Hour

// SessionStore keeps sessions in an in-memory map (the hot read path). When a
// database file path is configured, every mutation is additionally written
// through to a SQLite database so sessions survive server restarts.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	db       *sql.DB // nil = in-memory only
	dbPath   string
}

const sessionsSchema = `
CREATE TABLE IF NOT EXISTS sessions (
	token        TEXT PRIMARY KEY,
	user_name    TEXT NOT NULL,
	server_url   TEXT NOT NULL,
	mode         TEXT NOT NULL,
	api_key      TEXT,
	access_token TEXT,
	user_email   TEXT,
	user_id      TEXT,
	expires_at   INTEGER NOT NULL
);`

// NewSessionStore returns a session store. When dbPath is non-empty it opens
// (creating if needed) a SQLite database, migrates the schema, purges expired
// rows, and restores all remaining sessions into memory. Any database error is
// logged and degrades to in-memory only — the server must keep working.
func NewSessionStore(dbPath string) *SessionStore {
	s := &SessionStore{sessions: make(map[string]*Session), dbPath: dbPath}
	if dbPath == "" {
		return s
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("Warning: cannot open session database %s: %v (continuing in-memory)", dbPath, err)
		return s
	}
	if err := db.Ping(); err != nil {
		log.Printf("Warning: cannot reach session database %s: %v (continuing in-memory)", dbPath, err)
		db.Close()
		return s
	}
	if _, err := db.Exec(sessionsSchema); err != nil {
		log.Printf("Warning: cannot migrate session database %s: %v (continuing in-memory)", dbPath, err)
		db.Close()
		return s
	}
	s.db = db

	// Startup restore + purge: load non-expired sessions, drop expired rows.
	now := time.Now().Unix()
	rows, err := db.Query(`SELECT token, user_name, server_url, mode, api_key, access_token, user_email, user_id, expires_at FROM sessions`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var (
				token, userName, serverURL, mode     string
				apiKey, accessToken, userEmail, userID sql.NullString
				expiresAt                              int64
			)
			if err := rows.Scan(&token, &userName, &serverURL, &mode, &apiKey, &accessToken, &userEmail, &userID, &expiresAt); err != nil {
				log.Printf("Warning: skipping malformed session row: %v", err)
				continue
			}
			if expiresAt <= now {
				continue // expired; purged below
			}
			session := &Session{
				UserName:    userName,
				ServerURL:   serverURL,
				Mode:        AuthMode(mode),
				APIKey:      apiKey.String,
				AccessToken: accessToken.String,
				UserEmail:   userEmail.String,
				UserID:      userID.String,
				ExpiresAt:   time.Unix(expiresAt, 0),
			}
			if session.Mode != AuthModeAPIKey && session.Mode != AuthModeAccessToken {
				log.Printf("Warning: skipping session row with unknown mode %q", mode)
				continue
			}
			s.sessions[token] = session
		}
	} else {
		log.Printf("Warning: cannot read session database %s: %v (continuing with empty store)", dbPath, err)
	}
	if _, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now); err != nil {
		log.Printf("Warning: cannot purge expired sessions from %s: %v", dbPath, err)
	}
	return s
}

// Close releases the underlying database handle, if any.
func (s *SessionStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// persistExpiry writes the session's current expiry to the database. Failures
// are logged and ignored: the session stays valid in memory for the process
// lifetime, it just may not survive the next restart.
func (s *SessionStore) persistExpiry(token string, expiresAt time.Time) {
	if s.db == nil {
		return
	}
	if _, err := s.db.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`, expiresAt.Unix(), token); err != nil {
		log.Printf("Warning: cannot persist session expiry for %s…: %v", token[:12], err)
	}
}

func (s *SessionStore) CreateAPIKey(userName, apiKey, serverURL string) string {
	token := generateToken()
	now := time.Now().Add(sessionTTL)
	s.mu.Lock()
	s.sessions[token] = &Session{
		UserName:  userName,
		APIKey:    apiKey,
		ServerURL: serverURL,
		Mode:      AuthModeAPIKey,
		ExpiresAt: now,
	}
	s.mu.Unlock()
	s.insertRow(token, &Session{
		UserName:  userName,
		APIKey:    apiKey,
		ServerURL: serverURL,
		Mode:      AuthModeAPIKey,
		ExpiresAt: now,
	})
	return token
}

func (s *SessionStore) CreateAccessToken(userName, accessToken, serverURL, userEmail, userID string) string {
	token := generateToken()
	now := time.Now().Add(sessionTTL)
	s.mu.Lock()
	s.sessions[token] = &Session{
		UserName:    userName,
		AccessToken: accessToken,
		ServerURL:   serverURL,
		Mode:        AuthModeAccessToken,
		UserEmail:   userEmail,
		UserID:      userID,
		ExpiresAt:   now,
	}
	s.mu.Unlock()
	s.insertRow(token, &Session{
		UserName:    userName,
		AccessToken: accessToken,
		ServerURL:   serverURL,
		Mode:        AuthModeAccessToken,
		UserEmail:   userEmail,
		UserID:      userID,
		ExpiresAt:   now,
	})
	return token
}

// insertRow writes a session row to the database (INSERT OR REPLACE so
// re-created tokens overwrite stale rows). Failures degrade to in-memory.
func (s *SessionStore) insertRow(token string, session *Session) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO sessions (token, user_name, server_url, mode, api_key, access_token, user_email, user_id, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		token,
		session.UserName,
		session.ServerURL,
		string(session.Mode),
		nullIfEmpty(session.APIKey),
		nullIfEmpty(session.AccessToken),
		nullIfEmpty(session.UserEmail),
		nullIfEmpty(session.UserID),
		session.ExpiresAt.Unix(),
	)
	if err != nil {
		log.Printf("Warning: cannot persist session %s…: %v", token[:12], err)
	}
}

func nullIfEmpty(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
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
		s.deleteRow(token)
		return nil, false
	}
	// Sliding expiration
	s.mu.Lock()
	session.ExpiresAt = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	s.persistExpiry(token, session.ExpiresAt)
	return session, true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	s.deleteRow(token)
}

func (s *SessionStore) deleteRow(token string) {
	if s.db == nil {
		return
	}
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token); err != nil {
		log.Printf("Warning: cannot delete session %s… from database: %v", token[:12], err)
	}
}

func (s *SessionStore) Cleanup() {
	s.mu.Lock()
	now := time.Now()
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
	if s.db == nil {
		return
	}
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
		log.Printf("Warning: cannot purge expired sessions: %v", err)
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
	config    Config
	session   *SessionStore
	accounts  *AccountStore
	stats     *StatsStore
	transport http.RoundTripper // optional instrumented transport for the reverse proxy
}

func NewServer(cfg Config) *Server {
	s := &Server{
		config:  cfg,
		stats:   NewStatsStore(cfg.StatsFile),
	}
	// Sessions and accounts share one SQLite handle (both nil when running
	// in-memory); the account store migrates env-configured users into
	// accounts so existing users keep working.
	s.session = NewSessionStore(cfg.SessionDBFile)
	s.accounts = NewAccountStore(s.session.db, cfg.Users, cfg.ServerURL)
	// Restore persisted stats on startup when a stats file is configured.
	// On any file error we log a warning and continue with in-memory only.
	if cfg.StatsFile != "" {
		if err := s.stats.LoadFromFile(cfg.StatsFile); err != nil {
			log.Printf("Warning: cannot load stats file %s: %v (continuing in-memory)", cfg.StatsFile, err)
		}
	}
	return s
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

	case path == "/api/auth/account":
		s.authMiddleware(http.HandlerFunc(s.accountHandler)).ServeHTTP(w, r)

	case path == "/api/trmnl/stats":
		// Public Trmnl e-ink polling endpoint. Registered before the /api/
		// proxy catch-all so it is served locally and never proxied to Immich.
		s.trmnlStatsHandler(w, r)

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
		"users":            userNames,
		"defaultServerUrl": s.config.ServerURL,
		"version":          Version,
	})
}

// ─── Trmnl Stats ───────────────────────────────────────────────────────────

// trmnlStatsHandler serves the public Trmnl e-ink polling endpoint. It is
// unauthenticated on purpose (Trmnl devices have no Immich session) and only
// exposes aggregate keep/delete counters, never credentials or asset data.
func (s *Server) trmnlStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	kept, deleted, users, updatedAt := s.stats.Snapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"keptCount":    kept,
		"deletedCount": deleted,
		"totalCount":   kept + deleted,
		"serverUrl":    s.config.ServerURL,
		"updatedAt":    updatedAt.Format(time.RFC3339),
		"users":        users,
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
	if hasAPIKey && hasUserName && !hasPassword {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide either userName or apiKey, not both (or add a password to create an account)"})
		return
	}
	if hasPassword && !hasEmail && !hasUserName {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password requires email or userName"})
		return
	}
	if hasEmail && !hasPassword {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email requires password"})
		return
	}

	// 0) Create/claim a local account: userName + password + API key
	//    → API-key session. New names are created; migrated env users can only
	//    be claimed with the API key bound to their account.
	if hasUserName && hasPassword && hasAPIKey {
		s.loginWithAccountCreate(w, req.UserName, req.Password, req.APIKey, req.ServerURL)
		return
	}

	// 1) Local account by name + password → API-key session
	if hasUserName && hasPassword {
		s.loginWithAccount(w, req.UserName, req.Password, req.ServerURL)
		return
	}

	// 2) Env user by name → API-key session
	if hasUserName {
		s.loginWithEnvUser(w, req.UserName)
		return
	}

	// 3) Manual API key → API-key session
	if hasAPIKey {
		s.loginWithAPIKey(w, req.APIKey, req.ServerURL)
		return
	}

	// 4) Immich email/password → access-token session
	if hasEmail {
		s.loginWithCredentials(w, req.Email, req.Password, req.ServerURL)
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provide userName, apiKey, or email/password"})
}

// loginWithAccount authenticates against the local accounts table: the
// userName/password pair must match an existing account. Migrated env users
// can only be claimed once a password was set (Settings → Account password);
// until then auto-login via userName alone keeps working. Never logs the
// password.
func (s *Server) loginWithAccount(w http.ResponseWriter, userName, password, serverURL string) {
	if serverURL == "" {
		serverURL = s.config.ServerURL
	}
	if serverURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no server URL configured"})
		return
	}

	account, ok := s.accounts.VerifyPassword(serverURL, userName, password)
	if !ok {
		if _, exists := s.accounts.Get(serverURL, userName); !exists {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unknown user", "code": "unknown_user"})
			return
		}
		if !s.accounts.HasPassword(serverURL, userName) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no password set for this account", "code": "password_not_set"})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password", "code": "invalid_password"})
		return
	}

	valid, name, err := s.validateAPIKey(serverURL, account.APIKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot reach Immich server"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}
	if name != "" {
		account.UserName = name
	}

	token := s.session.CreateAPIKey(account.UserName, account.APIKey, serverURL)
	log.Printf("Login: mode=apiKey user=%q session=%s…", account.UserName, token[:12])
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":     token,
		"userName":  account.UserName,
		"serverUrl": serverURL,
		"mode":      "apiKey",
	})
}

// loginWithAccountCreate creates or claims a local account in one step from
// the login page: userName + password + Immich API key. Claim rules:
//   - the name already has a password   → reject (password changes live in Settings)
//   - the name exists without password  → only the API key bound to the account
//     (e.g. a migrated env user) may claim it; a foreign key is rejected
//   - the name is new                    → the API key is validated against Immich
//
// Never logs the password or the API key.
func (s *Server) loginWithAccountCreate(w http.ResponseWriter, userName, password, apiKey, serverURL string) {
	if len(password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters", "code": "weak_password"})
		return
	}
	if serverURL == "" {
		serverURL = s.config.ServerURL
	}
	if serverURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no server URL configured"})
		return
	}

	if account, exists := s.accounts.Get(serverURL, userName); exists {
		if account.PasswordHash != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "this user name already has a password; sign in with it or change it in Settings",
				"code":  "account_exists",
			})
			return
		}
		if account.APIKey != apiKey {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "this user name is reserved for another API key",
				"code":  "invalid_api_key",
			})
			return
		}
		// Migrated env user claiming their own account with the bound key.
	} else {
		valid, _, err := s.validateAPIKey(serverURL, apiKey)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot reach Immich server"})
			return
		}
		if !valid {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key", "code": "invalid_api_key"})
			return
		}
	}

	s.accounts.SetPassword(serverURL, userName, apiKey, password)
	token := s.session.CreateAPIKey(userName, apiKey, serverURL)
	log.Printf("Login: mode=apiKey (account created/claimed) user=%q session=%s…", userName, token[:12])
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":     token,
		"userName":  userName,
		"serverUrl": serverURL,
		"mode":      "apiKey",
	})
}

func (s *Server) loginWithEnvUser(w http.ResponseWriter, userName string) {
	// A person who has set an account password must use it: the unauthenticated
	// userName auto-login is disabled for that account.
	if s.config.ServerURL != "" && s.accounts.HasPassword(s.config.ServerURL, userName) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "password required", "code": "password_required"})
		return
	}

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
		"mode":      "apiKey",
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
		"mode":      "apiKey",
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
		"mode":      "accessToken",
	})
}

// accountHandler lets an authenticated person set or change the password for
// their local account, binding their session's Immich API key to it. Only
// API-key sessions can do this: access tokens are ephemeral and cannot be
// re-validated on later account logins.
func (s *Server) accountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session := sessionFromContext(r.Context())
	if session == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session"})
		return
	}
	if session.Mode != AuthModeAPIKey {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "password accounts are only available for API-key sessions; log in with an Immich API key first",
			"code":  "unsupported_mode",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read request body"})
		return
	}
	defer r.Body.Close()

	var req struct {
		Password        string `json:"password"`
		CurrentPassword string `json:"currentPassword"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters", "code": "weak_password"})
		return
	}

	// Changing an existing password requires the current one.
	if s.accounts.HasPassword(session.ServerURL, session.UserName) {
		if _, ok := s.accounts.VerifyPassword(session.ServerURL, session.UserName, req.CurrentPassword); !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password required", "code": "current_password_required"})
			return
		}
	}

	s.accounts.SetPassword(session.ServerURL, session.UserName, session.APIKey, req.Password)
	log.Printf("Account: password set for user=%q server=%q", session.UserName, session.ServerURL)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

// pendingStats holds counter deltas derived from a countable request body
// before it is forwarded upstream. Deltas are applied only when the upstream
// responds 2xx and at most once per request (design D1/D4).
type pendingStats struct {
	keptDelta    int
	deletedDelta int
	applied      bool
}

type pendingStatsKey struct{}

// countRoute identifies which (method, path) pair maps to a stats counter.
type countRoute int

const (
	countNone         countRoute = iota
	countTrashDelete             // DELETE /api/assets            -> deleted += len(ids)
	countTrashRestore            // POST  /api/trash/restore/assets -> deleted -= len(ids)
	countAlbumAdd                // PUT   /api/albums/<id>/assets -> kept += len(ids)
	countFavorite                // PUT   /api/assets/<id>        -> kept += 1 (isFavorite:true)
)

func classifyCountRoute(r *http.Request) countRoute {
	switch {
	case r.Method == http.MethodDelete && r.URL.Path == "/api/assets":
		return countTrashDelete
	case r.Method == http.MethodPost && r.URL.Path == "/api/trash/restore/assets":
		return countTrashRestore
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/albums/") && strings.HasSuffix(r.URL.Path, "/assets"):
		return countAlbumAdd
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/assets/"):
		return countFavorite
	default:
		return countNone
	}
}

// countRequestAndRewind inspects countable requests (design D4). For those it
// reads the request body, parses the JSON, rewinds the body with
// io.NopCloser so the upstream receives it byte-identical, and returns the
// pending deltas. It returns nil for non-countable requests, malformed JSON,
// and requests that must not change any counter (e.g. force:true deletes);
// those are still forwarded untouched.
func countRequestAndRewind(r *http.Request) *pendingStats {
	route := classifyCountRoute(r)
	if route == countNone {
		return nil
	}

	// Read and rewind the body unconditionally so upstream behavior is
	// unchanged regardless of whether we can parse it.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	var payload struct {
		IDs        []string `json:"ids"`
		Force      *bool    `json:"force"`
		IsFavorite *bool    `json:"isFavorite"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil // malformed bodies never change counters
	}

	pending := &pendingStats{}
	switch route {
	case countTrashDelete:
		// Hard deletes (force:true) are not swipe trashes; ignore them.
		if payload.Force != nil && *payload.Force {
			return nil
		}
		pending.deletedDelta = len(payload.IDs)
	case countTrashRestore:
		pending.deletedDelta = -len(payload.IDs)
	case countAlbumAdd:
		pending.keptDelta = len(payload.IDs)
	case countFavorite:
		if payload.IsFavorite != nil && *payload.IsFavorite {
			pending.keptDelta = 1
		}
	}
	if pending.keptDelta == 0 && pending.deletedDelta == 0 {
		return nil
	}
	return pending
}

// applyPendingCounts is the proxy ModifyResponse hook: it applies pending
// counter deltas only when the upstream responded 2xx. The pending state is
// marked applied on every invocation so a single request is never counted
// twice, even if the hook were to fire more than once (design D1).
func (s *Server) applyPendingCounts(resp *http.Response) {
	if resp == nil || resp.Request == nil {
		return
	}
	ctx := resp.Request.Context()
	pending, _ := ctx.Value(pendingStatsKey{}).(*pendingStats)
	if pending == nil || pending.applied {
		return
	}
	pending.applied = true

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return
	}
	session := sessionFromContext(ctx)
	if session == nil {
		return
	}
	if pending.keptDelta != 0 {
		s.stats.IncrementKept(session.ServerURL, session.UserName, pending.keptDelta)
	}
	if pending.deletedDelta != 0 {
		s.stats.IncrementDeleted(session.ServerURL, session.UserName, pending.deletedDelta)
	}
}

func (s *Server) proxyHandler(w http.ResponseWriter, r *http.Request) {
	session := sessionFromContext(r.Context())
	if session == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session"})
		return
	}

	// Inspect countable requests (design D4) and rewind the body so the
	// upstream receives it byte-identical.
	pending := countRequestAndRewind(r)

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
		Transport: s.transport,
		ModifyResponse: func(resp *http.Response) error {
			// Success-gated stats counting (design D1).
			s.applyPendingCounts(resp)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
		},
	}

	if pending != nil {
		ctx := context.WithValue(r.Context(), pendingStatsKey{}, pending)
		r = r.WithContext(ctx)
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

	// OpenTelemetry: initializes from OTEL_* env vars and degrades to a noop
	// instance when no OTLP endpoint is configured.
	tel := initTelemetry()
	slog.SetDefault(tel.logger)
	defer tel.shutdown(context.Background())

	// Wrap the server with OTel tracing + metrics middleware when enabled.
	var handler http.Handler = srv
	if !tel.disabled {
		handler = tel.metricsMiddleware(handler)
		handler = tel.middleware(handler)
	}
	srv.transport = tel.proxyTransport

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: handler,
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
		if cfg.StatsFile != "" {
			log.Printf("  Trmnl stats persistence: %s", cfg.StatsFile)
		} else {
			log.Printf("  Trmnl stats: in-memory only (no TRMNL_STATS_FILE)")
		}
		if cfg.SessionDBFile != "" {
			log.Printf("  Session persistence: %s", cfg.SessionDBFile)
		} else {
			log.Printf("  Sessions: in-memory only (no IMMICH_SESSIONS_DB; sessions lost on restart)")
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
