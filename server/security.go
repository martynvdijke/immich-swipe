package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	sessionCookieName = "swipe_session"
	csrfCookieName    = "swipe_csrf"
	csrfHeaderName    = "X-CSRF-Token"
)

const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' https:; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: http: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self' http: https:; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// setSecurityHeaders adds baseline browser hardening headers to every
// response (HTML, JSON, and proxied assets alike).
func (s *Server) setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", contentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}

// requestIsSecure reports whether the browser reached this server over TLS,
// including when TLS terminates at a reverse proxy.
func requestIsSecure(r *http.Request) bool {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("SWIPE_COOKIE_SECURE"))); v != "" {
		return v == "1" || v == "true" || v == "yes"
	}
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		first := strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
		return first == "https"
	}
	return false
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ensureCSRFCookie sets the double-submit CSRF cookie when absent. The cookie
// is intentionally readable by the SPA (not HttpOnly): the client must echo it
// in the X-CSRF-Token header for state-changing requests.
func (s *Server) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: false,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
	return token
}

// sameOriginRequest rejects unsafe requests whose Origin (when present) does
// not match the host that served the app. This complements the double-submit
// token and catches misconfigured clients early.
func sameOriginRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	expected := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		expected = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return strings.EqualFold(u.Host, expected)
}

// allowUnsafeRequest implements CSRF protection for cookie-authenticated
// state-changing requests using the double-submit pattern.
//
// Requests that carry neither a session cookie nor a CSRF cookie are treated
// as non-browser/API clients (for example legacy Bearer-token callers and Go
// tests) and are allowed through; browsers always carry the CSRF cookie set by
// GET /api/auth/config.
func (s *Server) allowUnsafeRequest(r *http.Request) bool {
	if !sameOriginRequest(r) {
		return false
	}
	csrf, err := r.Cookie(csrfCookieName)
	_, sessionErr := r.Cookie(sessionCookieName)
	if err != nil && sessionErr != nil {
		return true
	}
	if err != nil || csrf.Value == "" {
		return false
	}
	header := r.Header.Get(csrfHeaderName)
	if header == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(csrf.Value)) == 1
}

// sessionTokenFromRequest resolves the session token from the HttpOnly cookie
// first, falling back to the legacy Authorization Bearer header so existing API
// clients and mobile builds keep working.
func sessionTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
