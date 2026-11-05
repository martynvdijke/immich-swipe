package main

import "log"

// ─── Demo account ──────────────────────────────────────────────────────────
//
// The demo account is an ordinary local account whose credentials are
// intentionally published in GET /api/auth/config so app-store reviewers can
// try the app without an Immich server. It is only published when a complete,
// valid configuration is present; the Immich API key is never exposed.

// demoCredentials validates the demo configuration and returns the resolved
// values. ok is false when demo mode is disabled or the configuration is
// incomplete (in which case nothing may be published or created).
func (c Config) demoCredentials() (serverURL, userName, password string, ok bool) {
	if !c.DemoUserEnabled {
		return "", "", "", false
	}
	serverURL = c.DemoUserServerURL
	if serverURL == "" {
		serverURL = c.ServerURL
	}
	userName = c.DemoUserName
	password = c.DemoUserPassword
	if serverURL == "" || userName == "" || password == "" || c.DemoUserAPIKey == "" {
		return "", "", "", false
	}
	return serverURL, userName, password, true
}

// ensureDemoAccount creates or repairs the demo account on every boot. It is
// idempotent (SetPassword upserts and refreshes the bound Immich API key) and
// does nothing when demo mode is disabled or misconfigured.
func (s *Server) ensureDemoAccount() {
	if !s.config.DemoUserEnabled {
		return
	}
	serverURL, userName, password, ok := s.config.demoCredentials()
	if !ok {
		log.Printf("Warning: DEMO_USER_ENABLED is set but the demo account is incomplete (DEMO_USER_API_KEY and a server URL are required); demo login disabled")
		return
	}
	s.accounts.SetPassword(serverURL, userName, s.config.DemoUserAPIKey, password)
	if s.config.DemoUserEmail != "" {
		s.accounts.SetEmail(serverURL, userName, s.config.DemoUserEmail)
	}
	log.Printf("Demo account ensured: user=%q server=%q (published in /api/auth/config)", userName, serverURL)
}
