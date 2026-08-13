package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// openRawDB opens the sessions DB directly to seed or inspect rows.
func openRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("cannot open raw db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionStore_PersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")

	store := NewSessionStore(path)
	defer store.Close()
	tokenKey := store.CreateAPIKey("Alice", "key-1", "http://immich:2283")
	tokenTok := store.CreateAccessToken("Bob", "jwt-1", "http://immich:2283", "bob@test.com", "uid-2")

	// Reopen a fresh store on the same file (simulated restart).
	fresh := NewSessionStore(path)
	defer fresh.Close()

	s, ok := fresh.Get(tokenKey)
	if !ok {
		t.Fatal("API-key session not restored after reopen")
	}
	if s.UserName != "Alice" || s.APIKey != "key-1" || s.ServerURL != "http://immich:2283" {
		t.Fatalf("API-key session fields mismatch: %+v", s)
	}
	if s.Mode != AuthModeAPIKey {
		t.Fatalf("expected apiKey mode, got %q", s.Mode)
	}
	if s.ExpiresAt.IsZero() {
		t.Fatal("expiry not restored")
	}

	s2, ok := fresh.Get(tokenTok)
	if !ok {
		t.Fatal("access-token session not restored after reopen")
	}
	if s2.UserName != "Bob" || s2.AccessToken != "jwt-1" || s2.UserEmail != "bob@test.com" || s2.UserID != "uid-2" {
		t.Fatalf("access-token session fields mismatch: %+v", s2)
	}
	if s2.Mode != AuthModeAccessToken {
		t.Fatalf("expected accessToken mode, got %q", s2.Mode)
	}
}

func TestSessionStore_DeletePersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")

	store := NewSessionStore(path)
	token := store.CreateAPIKey("Alice", "key-1", "http://immich")
	store.Delete(token)
	store.Close()

	fresh := NewSessionStore(path)
	defer fresh.Close()
	if _, ok := fresh.Get(token); ok {
		t.Fatal("deleted session was restored")
	}
}

func TestSessionStore_ExpiredSessionsPurgedOnStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")

	store := NewSessionStore(path)
	alive := store.CreateAPIKey("Alice", "key-1", "http://immich")
	store.Close()

	// Seed an expired row directly into the DB.
	db := openRawDB(t, path)
	expired := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	if _, err := db.Exec(
		`INSERT INTO sessions (token, user_name, server_url, mode, api_key, access_token, user_email, user_id, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		expired, "Old", "http://immich", string(AuthModeAPIKey), "k", nil, nil, nil, time.Now().Add(-time.Hour).Unix(),
	); err != nil {
		t.Fatalf("cannot seed expired row: %v", err)
	}

	fresh := NewSessionStore(path)
	defer fresh.Close()

	if _, ok := fresh.Get(alive); !ok {
		t.Fatal("non-expired session should survive restart")
	}
	if _, ok := fresh.Get(expired); ok {
		t.Fatal("expired session must not be restored")
	}

	// The expired row must be gone from the DB file too.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token = ?`, expired).Scan(&count); err != nil {
		t.Fatalf("cannot count: %v", err)
	}
	if count != 0 {
		t.Fatal("expired row still present in database after startup purge")
	}
}

func TestSessionStore_SlidingExpiryPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")

	store := NewSessionStore(path)
	token := store.CreateAPIKey("Alice", "key-1", "http://immich")

	// Use the session so the sliding window extends, then reopen.
	if _, ok := store.Get(token); !ok {
		t.Fatal("session missing before restart")
	}
	store.Close()

	fresh := NewSessionStore(path)
	defer fresh.Close()
	if _, ok := fresh.Get(token); !ok {
		t.Fatal("session should survive restart after being used")
	}
}

func TestSessionStore_CleanupRemovesExpiredRowsFromDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")

	store := NewSessionStore(path)
	token := store.CreateAPIKey("Alice", "key-1", "http://immich")
	store.Close()

	// Backdate the row in the DB to simulate a long-idle session.
	db := openRawDB(t, path)
	if _, err := db.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`, time.Now().Add(-time.Hour).Unix(), token); err != nil {
		t.Fatalf("cannot backdate: %v", err)
	}

	// Cleanup is done on a store pointing at the same file.
	cleaner := NewSessionStore(path)
	cleaner.Cleanup()
	cleaner.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token = ?`, token).Scan(&count); err != nil {
		t.Fatalf("cannot count: %v", err)
	}
	if count != 0 {
		t.Fatal("cleanup did not remove expired row from the database")
	}
}

func TestSessionStore_UnwritablePathDegrades(t *testing.T) {
	// Point at a path whose parent directory does not exist.
	store := NewSessionStore(filepath.Join(t.TempDir(), "missing", "nested", "sessions.db"))
	defer store.Close()

	// The store must still work in-memory.
	token := store.CreateAPIKey("Alice", "k", "http://immich")
	if _, ok := store.Get(token); !ok {
		t.Fatal("store must keep working in-memory when DB path is unusable")
	}
}

func TestSessionStore_CorruptDBDegrades(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.db")
	if err := os.WriteFile(path, []byte("this is not a sqlite database at all, definitely not a valid header"), 0o600); err != nil {
		t.Fatalf("cannot write corrupt db: %v", err)
	}

	store := NewSessionStore(path)
	defer store.Close()

	token := store.CreateAPIKey("Alice", "k", "http://immich")
	if _, ok := store.Get(token); !ok {
		t.Fatal("store must keep working in-memory on a corrupt DB file")
	}
}

func TestSessionStore_UnconfiguredCreatesNoFile(t *testing.T) {
	store := NewSessionStore("")
	defer store.Close()
	token := store.CreateAPIKey("Alice", "k", "http://immich")
	if _, ok := store.Get(token); !ok {
		t.Fatal("in-memory store must work")
	}
	// No file could have been created; nothing to assert on disk, but the
	// store must not have a DB handle.
	if store.db != nil {
		t.Fatal("in-memory store must not hold a database handle")
	}
}
