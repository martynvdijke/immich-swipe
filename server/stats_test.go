package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatsStore_IncrementAndSnapshot(t *testing.T) {
	store := NewStatsStore("")
	store.IncrementKept("http://immich.example", "Alice", 3)
	store.IncrementDeleted("http://immich.example", "Alice", 2)
	store.IncrementDeleted("http://immich.example", "Alice", 1) // another delete

	kept, deleted, users, updatedAt := store.Snapshot()
	if kept != 3 {
		t.Fatalf("expected kept=3, got %d", kept)
	}
	if deleted != 3 {
		t.Fatalf("expected deleted=3, got %d", deleted)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].UserName != "Alice" || users[0].KeptCount != 3 || users[0].DeletedCount != 3 {
		t.Fatalf("unexpected user entry: %+v", users[0])
	}
	if updatedAt.IsZero() {
		t.Fatal("expected non-zero updatedAt")
	}
}

func TestStatsStore_SignedDeltas(t *testing.T) {
	store := NewStatsStore("")
	store.IncrementKept("http://immich.example", "Alice", 5)
	store.IncrementKept("http://immich.example", "Alice", -2) // undo
	store.IncrementDeleted("http://immich.example", "Alice", 4)
	store.IncrementDeleted("http://immich.example", "Alice", -1) // restore

	kept, deleted, _, _ := store.Snapshot()
	if kept != 3 {
		t.Fatalf("expected kept=3 after undo, got %d", kept)
	}
	if deleted != 3 {
		t.Fatalf("expected deleted=3 after restore, got %d", deleted)
	}
}

func TestStatsStore_PerUserIsolation(t *testing.T) {
	store := NewStatsStore("")
	store.IncrementKept("http://immich.example", "Alice", 7)
	store.IncrementKept("http://immich.example", "Bob", 2)
	store.IncrementDeleted("http://other.example", "Alice", 9)

	kept, deleted, users, _ := store.Snapshot()
	if kept != 9 {
		t.Fatalf("expected aggregated kept=9, got %d", kept)
	}
	if deleted != 9 {
		t.Fatalf("expected aggregated deleted=9, got %d", deleted)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 distinct (server,user) entries, got %d: %+v", len(users), users)
	}
	// Deterministic sort order: ServerURL asc, then UserName asc.
	if users[0].ServerURL > users[1].ServerURL {
		t.Fatalf("users not sorted by ServerURL: %+v", users)
	}
}

func TestStatsStore_Reset(t *testing.T) {
	store := NewStatsStore("")
	store.IncrementKept("http://immich.example", "Alice", 3)
	store.IncrementDeleted("http://immich.example", "Alice", 1)
	store.Reset()

	kept, deleted, users, _ := store.Snapshot()
	if kept != 0 || deleted != 0 {
		t.Fatalf("expected zero counters after reset, got kept=%d deleted=%d", kept, deleted)
	}
	if len(users) != 0 {
		t.Fatalf("expected no users after reset, got %d", len(users))
	}
}

func TestStatsStore_ZeroDeltaIsNoop(t *testing.T) {
	store := NewStatsStore("")
	store.IncrementKept("http://immich.example", "Alice", 0)
	store.IncrementDeleted("http://immich.example", "Alice", 0)

	_, _, users, _ := store.Snapshot()
	if len(users) != 0 {
		t.Fatalf("zero deltas must not create entries, got %d", len(users))
	}
}

func TestStatsStore_PersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")

	store := NewStatsStore(path)
	store.IncrementKept("http://immich.example", "Alice", 4)
	store.IncrementDeleted("http://immich.example", "Alice", 2)
	store.IncrementKept("http://immich.example", "Bob", 1)
	store.IncrementDeleted("http://other.example", "Carol", 3)
	if err := store.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	// Load into a fresh store and compare.
	fresh := NewStatsStore("")
	if err := fresh.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	kept, deleted, users, _ := fresh.Snapshot()
	if kept != 5 {
		t.Fatalf("expected kept=5 after reload, got %d", kept)
	}
	if deleted != 5 {
		t.Fatalf("expected deleted=5 after reload, got %d", deleted)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 entries after reload, got %d", len(users))
	}
}

func TestStatsStore_LoadFromFileMissingIsEmpty(t *testing.T) {
	store := NewStatsStore("")
	if err := store.LoadFromFile(filepath.Join(t.TempDir(), "does-not-exist.json")); err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	kept, deleted, users, _ := store.Snapshot()
	if kept != 0 || deleted != 0 || len(users) != 0 {
		t.Fatalf("expected empty store, got kept=%d deleted=%d users=%d", kept, deleted, len(users))
	}
}

func TestStatsStore_SaveToFileUnwritableDirDegrades(t *testing.T) {
	// A path whose parent does not exist cannot be written; SaveToFile must
	// return an error (the caller logs a warning and continues in-memory).
	store := NewStatsStore("")
	store.IncrementKept("http://immich.example", "Alice", 1)
	err := store.SaveToFile(filepath.Join(t.TempDir(), "no-such-dir", "stats.json"))
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}

func TestStatsStore_LoadFromFileCorruptDegrades(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStatsStore("")
	if err := store.LoadFromFile(path); err == nil {
		t.Fatal("expected error for corrupt stats file")
	}
}

func TestStatsStore_UpdatedAtPersisted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")

	store := NewStatsStore(path)
	store.IncrementKept("http://immich.example", "Alice", 1)
	if err := store.SaveToFile(path); err != nil {
		t.Fatal(err)
	}
	_, _, _, first := store.Snapshot()

	// Save again after another change; the persisted updatedAt must move forward.
	time.Sleep(2 * time.Millisecond)
	store.IncrementKept("http://immich.example", "Alice", 1)
	_, _, _, second := store.Snapshot()
	if !second.After(first) {
		t.Fatal("expected updatedAt to advance after a change")
	}

	fresh := NewStatsStore("")
	if err := fresh.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	_, _, _, loaded := fresh.Snapshot()
	if loaded.IsZero() {
		t.Fatal("expected loaded updatedAt to be non-zero")
	}
}
