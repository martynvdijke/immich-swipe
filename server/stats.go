package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// StatsEntry holds the keep/delete counters for one (server URL, user name)
// combination. Served to the Trmnl e-ink display via GET /api/trmnl/stats.
type StatsEntry struct {
	UserName     string `json:"userName"`
	ServerURL    string `json:"serverUrl"`
	KeptCount    int    `json:"keptCount"`
	DeletedCount int    `json:"deletedCount"`
}

// statsFilePayload is the on-disk JSON layout used by LoadFromFile/SaveToFile.
type statsFilePayload struct {
	UpdatedAt time.Time    `json:"updatedAt"`
	Entries   []StatsEntry `json:"entries"`
}

// StatsStore is a mutex-guarded map of per-(server, user) counters. It is
// populated passively by the reverse proxy (see proxyHandler) and read by the
// public Trmnl polling endpoint.
type StatsStore struct {
	mu        sync.RWMutex
	entries   map[string]*StatsEntry // key: serverURL|userName
	updatedAt time.Time
	filePath  string // optional persistence target (TRMNL_STATS_FILE); "" = memory only
}

func NewStatsStore(filePath string) *StatsStore {
	return &StatsStore{
		entries:   make(map[string]*StatsEntry),
		updatedAt: time.Now(),
		filePath:  filePath,
	}
}

func statsKey(serverURL, userName string) string {
	return serverURL + "|" + userName
}

// addDelta applies signed deltas to the entry for (serverURL, userName),
// creating it on first touch, and records the last-change timestamp. It then
// best-effort persists the store when a stats file is configured.
func (s *StatsStore) addDelta(serverURL, userName string, keptDelta, deletedDelta int) {
	if keptDelta == 0 && deletedDelta == 0 {
		return
	}
	s.mu.Lock()
	key := statsKey(serverURL, userName)
	entry, ok := s.entries[key]
	if !ok {
		entry = &StatsEntry{UserName: userName, ServerURL: serverURL}
		s.entries[key] = entry
	}
	entry.KeptCount += keptDelta
	entry.DeletedCount += deletedDelta
	s.updatedAt = time.Now()
	s.mu.Unlock()

	if s.filePath != "" {
		if err := s.SaveToFile(s.filePath); err != nil {
			log.Printf("Warning: failed to persist stats to %s: %v", s.filePath, err)
		}
	}
}

// IncrementKept applies a signed delta to the kept counter for a server/user.
func (s *StatsStore) IncrementKept(serverURL, userName string, delta int) {
	s.addDelta(serverURL, userName, delta, 0)
}

// IncrementDeleted applies a signed delta to the deleted counter for a
// server/user (restores use a negative delta).
func (s *StatsStore) IncrementDeleted(serverURL, userName string, delta int) {
	s.addDelta(serverURL, userName, 0, delta)
}

// Snapshot returns the aggregated counters across all server/user entries,
// a deterministic per-user breakdown, and the timestamp of the last change
// (or store creation when nothing has been recorded yet).
func (s *StatsStore) Snapshot() (kept, deleted int, users []StatsEntry, updatedAt time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users = make([]StatsEntry, 0, len(s.entries))
	for _, e := range s.entries {
		kept += e.KeptCount
		deleted += e.DeletedCount
		users = append(users, *e)
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].ServerURL != users[j].ServerURL {
			return users[i].ServerURL < users[j].ServerURL
		}
		return users[i].UserName < users[j].UserName
	})
	return kept, deleted, users, s.updatedAt
}

// Reset clears all counters and restores the last-change timestamp to now.
func (s *StatsStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]*StatsEntry)
	s.updatedAt = time.Now()
}

// SaveToFile writes the full store atomically (temp file + rename) to path.
func (s *StatsStore) SaveToFile(path string) error {
	s.mu.RLock()
	payload := statsFilePayload{
		UpdatedAt: s.updatedAt,
		Entries:   make([]StatsEntry, 0, len(s.entries)),
	}
	for _, e := range s.entries {
		payload.Entries = append(payload.Entries, *e)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".trmnl-stats-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp stats file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp stats file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp stats file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename stats file: %w", err)
	}
	return nil
}

// LoadFromFile reads a stats file previously written by SaveToFile and
// replaces the in-memory contents. A missing file is treated as an empty
// store (no error).
func (s *StatsStore) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read stats file: %w", err)
	}

	var payload statsFilePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse stats file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]*StatsEntry, len(payload.Entries))
	for i := range payload.Entries {
		e := payload.Entries[i]
		s.entries[statsKey(e.ServerURL, e.UserName)] = &e
	}
	if !payload.UpdatedAt.IsZero() {
		s.updatedAt = payload.UpdatedAt
	} else {
		s.updatedAt = time.Now()
	}
	return nil
}
