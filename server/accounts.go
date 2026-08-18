package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Local Accounts ────────────────────────────────────────────────────────

// Account is an app-local username/password protected identity. It binds a
// (server URL, user name) pair to an Immich API key so that a person can log
// into Swipe with their own credentials instead of (or in addition to) the
// env-configured API keys. Accounts live in the same SQLite database as the
// sessions (IMMICH_SESSIONS_DB) and degrade to in-memory when no database is
// configured.
type Account struct {
	ServerURL    string
	UserName     string
	PasswordHash *string // nil = migrated account, no password set yet
	APIKey       string
	CreatedAt    time.Time
}

// AccountStore keeps accounts in an in-memory map (the hot read path). When a
// database file path is configured, every mutation is additionally written
// through to the accounts table so accounts survive server restarts. It
// mirrors the SessionStore pattern: any database error degrades to in-memory.
type AccountStore struct {
	mu  sync.RWMutex
	db  *sql.DB // nil = in-memory only
	mem map[string]*Account
}

const accountsSchema = `
CREATE TABLE IF NOT EXISTS accounts (
	server_url    TEXT NOT NULL,
	user_name     TEXT NOT NULL,
	password_hash TEXT,
	api_key       TEXT NOT NULL,
	created_at    INTEGER NOT NULL,
	PRIMARY KEY (server_url, user_name)
);`

// pbkdf2Iterations is the PBKDF2-HMAC-SHA256 work factor for password hashing.
const pbkdf2Iterations = 600000

func accountKey(serverURL, userName string) string {
	return serverURL + "|" + userName
}

// NewAccountStore creates an account store, migrates the schema, loads
// existing accounts into memory, and migrates env-configured users into the
// accounts table (so existing users keep working without reconfiguration).
// Any database error is logged and degrades to in-memory only.
func NewAccountStore(db *sql.DB, envUsers []UserConfig, defaultServerURL string) *AccountStore {
	s := &AccountStore{db: db, mem: make(map[string]*Account)}

	if db != nil {
		if _, err := db.Exec(accountsSchema); err != nil {
			log.Printf("Warning: cannot migrate account database: %v (continuing in-memory)", err)
			s.db = nil
		}
	}

	if s.db != nil {
		rows, err := db.Query(`SELECT server_url, user_name, password_hash, api_key, created_at FROM accounts`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var (
					serverURL, userName, apiKey string
					passwordHash                 sql.NullString
					createdAt                    int64
				)
				if err := rows.Scan(&serverURL, &userName, &passwordHash, &apiKey, &createdAt); err != nil {
					log.Printf("Warning: skipping malformed account row: %v", err)
					continue
				}
				account := &Account{
					ServerURL: serverURL,
					UserName:  userName,
					APIKey:    apiKey,
					CreatedAt: time.Unix(createdAt, 0),
				}
				if passwordHash.Valid {
					hash := passwordHash.String
					account.PasswordHash = &hash
				}
				s.mem[accountKey(serverURL, userName)] = account
			}
		} else {
			log.Printf("Warning: cannot read account database: %v (continuing with empty store)", err)
		}
	}

	// Migration: existing env-configured users become accounts bound to their
	// env API key. INSERT ... ON CONFLICT DO UPDATE keeps the env key fresh on
	// restarts but never overwrites a password a person has already set.
	migrated := 0
	for _, u := range envUsers {
		if defaultServerURL == "" || u.APIKey == "" {
			continue
		}
		account := &Account{
			ServerURL: defaultServerURL,
			UserName:  u.Name,
			APIKey:    u.APIKey,
			CreatedAt: time.Now(),
		}
		key := accountKey(defaultServerURL, u.Name)
		s.mu.Lock()
		if existing, ok := s.mem[key]; ok {
			existing.APIKey = u.APIKey
		} else {
			s.mem[key] = account
		}
		s.mu.Unlock()
		if s.db != nil {
			if _, err := db.Exec(
				`INSERT INTO accounts (server_url, user_name, password_hash, api_key, created_at) VALUES (?, ?, NULL, ?, ?)
				 ON CONFLICT(server_url, user_name) DO UPDATE SET api_key = excluded.api_key`,
				defaultServerURL, u.Name, u.APIKey, account.CreatedAt.Unix(),
			); err != nil {
				log.Printf("Warning: cannot migrate env user %q into accounts: %v", u.Name, err)
				continue
			}
		}
		migrated++
	}
	if migrated > 0 {
		log.Printf("Migrated %d env-configured user(s) to local accounts", migrated)
	}
	return s
}

// Get returns the account for (serverURL, userName), if present.
func (s *AccountStore) Get(serverURL, userName string) (Account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.mem[accountKey(serverURL, userName)]
	if !ok {
		return Account{}, false
	}
	return *account, true
}

// HasPassword reports whether the account exists and has a password set.
func (s *AccountStore) HasPassword(serverURL, userName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.mem[accountKey(serverURL, userName)]
	return ok && account.PasswordHash != nil
}

// VerifyPassword checks the password against the stored hash. It returns the
// account and true on success; (zero Account, false) otherwise.
func (s *AccountStore) VerifyPassword(serverURL, userName, password string) (Account, bool) {
	s.mu.RLock()
	account, ok := s.mem[accountKey(serverURL, userName)]
	if !ok || account.PasswordHash == nil {
		s.mu.RUnlock()
		return Account{}, false
	}
	accountCopy := *account
	hash := *account.PasswordHash
	s.mu.RUnlock()
	if !verifyPasswordHash(hash, password) {
		return Account{}, false
	}
	return accountCopy, true
}

// SetPassword creates or updates the account password for (serverURL,
// userName), binding the given Immich API key. Failures writing to the
// database are logged and ignored: the account stays valid in memory.
func (s *AccountStore) SetPassword(serverURL, userName, apiKey, password string) {
	hash := hashPassword(password)
	s.mu.Lock()
	if existing, ok := s.mem[accountKey(serverURL, userName)]; ok {
		existing.PasswordHash = &hash
		existing.APIKey = apiKey
	} else {
		s.mem[accountKey(serverURL, userName)] = &Account{
			ServerURL:    serverURL,
			UserName:     userName,
			PasswordHash: &hash,
			APIKey:       apiKey,
			CreatedAt:    time.Now(),
		}
	}
	s.mu.Unlock()
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(
		`INSERT INTO accounts (server_url, user_name, password_hash, api_key, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(server_url, user_name) DO UPDATE SET password_hash = excluded.password_hash, api_key = excluded.api_key`,
		serverURL, userName, hash, apiKey, time.Now().Unix(),
	)
	if err != nil {
		log.Printf("Warning: cannot persist account %q: %v", userName, err)
	}
}

// hashPassword derives a PBKDF2-HMAC-SHA256 hash in the format
// `pbkdf2$<iterations>$<saltHex>$<keyHex>`. The salt is random per password.
func hashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(fmt.Sprintf("failed to generate password salt: %v", err))
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, 32)
	if err != nil {
		panic(fmt.Sprintf("failed to derive password key: %v", err))
	}
	return fmt.Sprintf("pbkdf2$%d$%s$%s", pbkdf2Iterations, hex.EncodeToString(salt), hex.EncodeToString(key))
}

// verifyPasswordHash parses a hashPassword output and compares it in
// constant time. Malformed hashes always fail closed.
func verifyPasswordHash(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 || iterations > 10000000 {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false
	}
	expected, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
