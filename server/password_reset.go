package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const passwordResetsSchema = `
CREATE TABLE IF NOT EXISTS password_resets (
	token_hash TEXT PRIMARY KEY,
	server_url TEXT NOT NULL,
	user_name TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	used_at INTEGER,
	created_at INTEGER NOT NULL
);`

func ensurePasswordResetsTable(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec(passwordResetsSchema); err != nil {
		log.Printf("Warning: cannot migrate password_resets: %v", err)
	}
}

// rateLimiter: 5/min per IP
type rateLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

var forgotRateLimiter = &rateLimiter{hits: make(map[string][]time.Time)}

func (r *rateLimiter) allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	list := r.hits[ip]
	filtered := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	list = filtered
	if len(list) >= 5 {
		r.hits[ip] = list
		return false
	}
	list = append(list, now)
	r.hits[ip] = list
	return true
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateResetToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Server) forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	if !forgotRateLimiter.allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read body"})
		return
	}
	defer r.Body.Close()
	var req struct {
		ServerURL string `json:"serverUrl"`
		UserName  string `json:"userName"`
		Email     string `json:"email"`
	}
	_ = json.Unmarshal(body, &req)
	// Accept email in userName field as well
	identifier := strings.TrimSpace(req.UserName)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	serverURL := strings.TrimSpace(req.ServerURL)
	if serverURL == "" {
		serverURL = s.config.ServerURL
	}
	genericOK := func() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	if identifier == "" || serverURL == "" {
		genericOK()
		return
	}
	// Find account: by username first, then by email
	var account *Account
	if acc, ok := s.accounts.Get(serverURL, identifier); ok {
		a := acc
		account = &a
	} else if strings.Contains(identifier, "@") {
		if acc, ok := s.accounts.GetByEmail(serverURL, identifier); ok {
			a := acc
			account = &a
		}
	} else {
		// Also try email lookup case where identifier looks like username but email matches
		// Already handled above; check all accounts email fallback
		for _, acc := range s.accounts.All() {
			if acc.ServerURL == serverURL && acc.Email != nil && strings.EqualFold(*acc.Email, identifier) {
				a := acc
				account = &a
				break
			}
		}
	}
	if account == nil || account.Email == nil || strings.TrimSpace(*account.Email) == "" {
		// Check if identifier is email and matches account via email without username match
		genericOK()
		return
	}
	es, ok := s.effectiveEmailSettings()
	if !ok {
		// Still return generic OK to avoid enumeration, but log
		log.Printf("Password reset requested but email not configured")
		genericOK()
		return
	}
	token := generateResetToken()
	tokenHash := hashToken(token)
	if s.session != nil && s.session.db != nil {
		ensurePasswordResetsTable(s.session.db)
		now := time.Now().Unix()
		expires := time.Now().Add(time.Hour).Unix()
		_, err := s.session.db.Exec(`INSERT INTO password_resets (token_hash,server_url,user_name,expires_at,created_at) VALUES (?,?,?,?,?)`, tokenHash, serverURL, account.UserName, expires, now)
		if err != nil {
			log.Printf("Warning: cannot store password reset token: %v", err)
			genericOK()
			return
		}
	} else {
		// In-memory fallback: store in map
		passwordResetMemStore.Lock()
		if passwordResetMem == nil {
			passwordResetMem = make(map[string]passwordResetEntry)
		}
		passwordResetMem[tokenHash] = passwordResetEntry{ServerURL: serverURL, UserName: account.UserName, ExpiresAt: time.Now().Add(time.Hour).Unix()}
		passwordResetMemStore.Unlock()
	}
	publicURL := s.config.PublicURL
	if publicURL == "" {
		publicURL = s.oauthPublicBase(r)
	}
	link := strings.TrimRight(publicURL, "/") + "/reset-password?token=" + token
	subject := "Reset your Immich Swipe password"
	bodyText := "You requested a password reset for your Immich Swipe account (" + account.UserName + ").\n\nReset link (valid for 1 hour):\n" + link + "\n\nIf you did not request this, you can ignore this email.\n"
	if err := sendMailFunc(es, *account.Email, subject, bodyText); err != nil {
		log.Printf("Warning: failed to send password reset email: %v", err)
	}
	genericOK()
}

type passwordResetEntry struct {
	ServerURL string
	UserName  string
	ExpiresAt int64
	UsedAt    *int64
}

var (
	passwordResetMem      map[string]passwordResetEntry
	passwordResetMemStore sync.Mutex
)

func (s *Server) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read body"})
		return
	}
	defer r.Body.Close()
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
		Password    string `json:"password"`
	}
	_ = json.Unmarshal(body, &req)
	token := strings.TrimSpace(req.Token)
	newPass := req.NewPassword
	if newPass == "" {
		newPass = req.Password
	}
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
		return
	}
	if len(newPass) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters", "code": "weak_password"})
		return
	}
	tokenHash := hashToken(token)
	var serverURL, userName string
	var expiresAt int64
	var usedAt sql.NullInt64
	found := false
	if s.session != nil && s.session.db != nil {
		ensurePasswordResetsTable(s.session.db)
		err := s.session.db.QueryRow(`SELECT server_url,user_name,expires_at,used_at FROM password_resets WHERE token_hash=?`, tokenHash).Scan(&serverURL, &userName, &expiresAt, &usedAt)
		if err == nil {
			found = true
		}
	} else {
		passwordResetMemStore.Lock()
		e, ok := passwordResetMem[tokenHash]
		if ok {
			found = true
			serverURL = e.ServerURL
			userName = e.UserName
			expiresAt = e.ExpiresAt
			if e.UsedAt != nil {
				usedAt = sql.NullInt64{Int64: *e.UsedAt, Valid: true}
			}
		}
		passwordResetMemStore.Unlock()
	}
	if !found {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or expired token"})
		return
	}
	if usedAt.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token already used"})
		return
	}
	if time.Now().Unix() > expiresAt {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token expired"})
		return
	}
	// Set password - preserve api key
	s.accounts.SetPassword(serverURL, userName, "", newPass)
	// Mark used
	if s.session != nil && s.session.db != nil {
		_, _ = s.session.db.Exec(`UPDATE password_resets SET used_at=? WHERE token_hash=?`, time.Now().Unix(), tokenHash)
	} else {
		passwordResetMemStore.Lock()
		if e, ok := passwordResetMem[tokenHash]; ok {
			now := time.Now().Unix()
			e.UsedAt = &now
			passwordResetMem[tokenHash] = e
		}
		passwordResetMemStore.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Email settings handlers

func (s *Server) emailSettingsGetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	es, ok := s.effectiveEmailSettings()
	fromEnv := s.config.SMTPHost != ""
	if !ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"smtp_host": "", "smtp_port": 0, "smtp_user": "", "smtp_from": "", "smtp_tls": "", "has_password": false, "from_env": false, "configured": false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"smtp_host": es.Host, "smtp_port": es.Port, "smtp_user": es.User, "smtp_from": es.From, "smtp_tls": es.TLS, "has_password": es.Pass != "", "from_env": fromEnv, "configured": true,
	})
}

func (s *Server) emailSettingsPutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.config.SMTPHost != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email settings are managed via environment variables"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	var req struct {
		Host     string `json:"smtp_host"`
		Port     int    `json:"smtp_port"`
		User     string `json:"smtp_user"`
		Pass     string `json:"smtp_pass"`
		From     string `json:"smtp_from"`
		TLS      string `json:"smtp_tls"`
		// also accept camelCase
		SMTPHost string `json:"smtpHost"`
		SMTPPort int    `json:"smtpPort"`
		SMTPUser string `json:"smtpUser"`
		SMTPPass string `json:"smtpPass"`
		SMTPFrom string `json:"smtpFrom"`
		SMTPTLS  string `json:"smtpTls"`
	}
	_ = json.Unmarshal(body, &req)
	// Merge alternative keys
	if req.Host == "" {
		req.Host = req.SMTPHost
	}
	if req.Port == 0 {
		req.Port = req.SMTPPort
	}
	if req.User == "" {
		req.User = req.SMTPUser
	}
	if req.Pass == "" {
		req.Pass = req.SMTPPass
	}
	if req.From == "" {
		req.From = req.SMTPFrom
	}
	if req.TLS == "" {
		req.TLS = req.SMTPTLS
	}
	req.Host = strings.TrimSpace(req.Host)
	if req.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "smtp_host is required"})
		return
	}
	tlsMode := strings.ToLower(strings.TrimSpace(req.TLS))
	if tlsMode == "" {
		tlsMode = "starttls"
	}
	if tlsMode != "starttls" && tlsMode != "ssl" && tlsMode != "none" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "smtp_tls must be starttls, ssl, or none"})
		return
	}
	es := EmailSettings{Host: req.Host, Port: req.Port, User: strings.TrimSpace(req.User), Pass: req.Pass, From: strings.TrimSpace(req.From), TLS: tlsMode}
	if err := s.saveEmailSettings(es); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot save email settings"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) emailTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	var req struct {
		To string `json:"to"`
	}
	_ = json.Unmarshal(body, &req)
	to := strings.TrimSpace(req.To)
	if to == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to is required"})
		return
	}
	es, ok := s.effectiveEmailSettings()
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email not configured"})
		return
	}
	if err := sendMailFunc(es, to, "Immich Swipe test email", "This is a test email from Immich Swipe. Your SMTP settings are working."); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send email: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) accountEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	session := sessionFromContext(r.Context())
	if session == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	var req struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(body, &req)
	email := strings.TrimSpace(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid email is required"})
		return
	}
	s.accounts.SetEmail(session.ServerURL, session.UserName, email)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
