package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// accountExportSession is the non-secret metadata of one live session. Tokens
// (Swipe session token, Immich API key, Immich access token) are deliberately
// never included: an export is data for the person, not another credential copy.
type accountExportSession struct {
	Mode      string    `json:"mode"`
	ExpiresAt time.Time `json:"expiresAt"`
	UserEmail string    `json:"userEmail,omitempty"`
	UserID    string    `json:"userID,omitempty"`
}

type accountExportPayload struct {
	ExportedAt       time.Time              `json:"exportedAt"`
	UserName         string                 `json:"userName"`
	ServerURL        string                 `json:"serverUrl"`
	Email            string                 `json:"email,omitempty"`
	AccountCreatedAt *time.Time             `json:"accountCreatedAt,omitempty"`
	HasPassword      bool                   `json:"hasPassword"`
	HasImmichAPIKey  bool                   `json:"hasImmichApiKey"`
	Counters         *StatsEntry            `json:"counters,omitempty"`
	Sessions         []accountExportSession `json:"sessions"`
	ImmichAccount    string                 `json:"immichAccount"`
	Note             string                 `json:"note"`
}

// exportAccountHandler serves GET /api/auth/account/export: a JSON copy of the
// app-local account data plus per-user counters and session metadata. It
// contains no secrets and no Immich photo data — photos live on the user's own
// Immich server, which this app only proxies.
func (s *Server) exportAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	session := sessionFromContext(r.Context())
	if session == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session"})
		return
	}

	payload := accountExportPayload{
		ExportedAt:    time.Now().UTC(),
		UserName:      session.UserName,
		ServerURL:     session.ServerURL,
		Email:         session.UserEmail,
		Sessions:      make([]accountExportSession, 0),
		ImmichAccount: "retained on the Immich server selected by the user",
		Note: "This export contains Immich Swipe account data only. Photos, albums and the Immich account itself live on the Immich server at " +
			session.ServerURL + " and are not controlled or exported by this app.",
	}
	if account, ok := s.accounts.Get(session.ServerURL, session.UserName); ok {
		if account.Email != nil {
			payload.Email = *account.Email
		}
		created := account.CreatedAt
		payload.AccountCreatedAt = &created
		payload.HasPassword = account.PasswordHash != nil
		payload.HasImmichAPIKey = account.APIKey != ""
	}
	if entry, ok := s.stats.Get(session.ServerURL, session.UserName); ok {
		payload.Counters = &entry
	}
	for _, sess := range s.session.SessionsFor(session.ServerURL, session.UserName) {
		payload.Sessions = append(payload.Sessions, accountExportSession{
			Mode:      string(sess.Mode),
			ExpiresAt: sess.ExpiresAt,
			UserEmail: sess.UserEmail,
			UserID:    sess.UserID,
		})
	}

	w.Header().Set("Content-Disposition", `attachment; filename="immich-swipe-account.json"`)
	writeJSON(w, http.StatusOK, payload)
}

// deleteAccountHandler serves DELETE /api/auth/account. The endpoint is
// cookie/Bearer authenticated and CSRF-protected by ServeHTTP. The caller must
// type their user name as an explicit confirmation.
//
// Deletion erases the app-local account row, every session (and therefore every
// stored Immich API key or access token), and the per-user counters. It cannot
// delete the Immich account or photos: this app has no administrative access to
// the user's Immich server, and that is stated on the public deletion page.
func (s *Server) deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	session := sessionFromContext(r.Context())
	if session == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no session"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read request body"})
		return
	}
	defer r.Body.Close()

	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	confirm := strings.TrimSpace(req.Confirm)
	if confirm == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "type your user name to confirm account deletion",
			"code":  "confirmation_required",
		})
		return
	}
	if confirm != session.UserName {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "confirmation does not match the signed-in user name",
			"code":  "confirmation_mismatch",
		})
		return
	}

	// Best-effort revoke of Immich access tokens before the local rows go away.
	// API keys cannot be revoked without their Immich-side id, so the local
	// reference is erased and the deletion page tells the user how to revoke the
	// key in Immich itself.
	for _, sess := range s.session.SessionsFor(session.ServerURL, session.UserName) {
		if sess.Mode == AuthModeAccessToken && sess.AccessToken != "" {
			s.immichLogout(sess.ServerURL, sess.AccessToken)
		}
	}

	deletedSessions := s.session.DeleteAllFor(session.ServerURL, session.UserName)
	s.accounts.Delete(session.ServerURL, session.UserName)
	s.stats.Remove(session.ServerURL, session.UserName)
	s.clearSessionCookie(w, r)
	logAccountDeletion(session)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":              "deleted",
		"deletedSessions":     deletedSessions,
		"immichAccount":       "retained",
		"immichAccountDetail": "The Immich account and its photos live on the Immich server and are not deleted by this app.",
	})
}

// logAccountDeletion records the fact of deletion without logging the raw user
// name or any credential, so server logs do not become a residual PII store.
func logAccountDeletion(session *Session) {
	log.Printf("Account deleted: server=%q", redactServerLog(session.ServerURL))
}

// redactServerLog keeps only the host of a server URL for operational logs.
func redactServerLog(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexAny(raw, "/?#"); i >= 0 {
		raw = raw[:i]
	}
	return raw
}
