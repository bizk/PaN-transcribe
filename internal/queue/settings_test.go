package queue

import (
	"path/filepath"
	"testing"
)

func setupSettingsStore(t *testing.T) *SettingsStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB() error: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return NewSettingsStore(db)
}

func TestSettingsStore_CustomPrompt(t *testing.T) {
	store := setupSettingsStore(t)
	userID := int64(123456)

	// Initially should return empty
	prompt, err := store.GetCustomPrompt(userID)
	if err != nil {
		t.Fatalf("GetCustomPrompt() error: %v", err)
	}
	if prompt != "" {
		t.Errorf("prompt = %q, want empty", prompt)
	}

	// Set custom prompt
	err = store.SetCustomPrompt(userID, "My custom prompt")
	if err != nil {
		t.Fatalf("SetCustomPrompt() error: %v", err)
	}

	// Verify it was saved
	prompt, err = store.GetCustomPrompt(userID)
	if err != nil {
		t.Fatalf("GetCustomPrompt() error: %v", err)
	}
	if prompt != "My custom prompt" {
		t.Errorf("prompt = %q, want %q", prompt, "My custom prompt")
	}
}

func TestSettingsStore_NextMode(t *testing.T) {
	store := setupSettingsStore(t)
	userID := int64(123456)

	// Initially should return empty
	mode, err := store.GetNextMode(userID)
	if err != nil {
		t.Fatalf("GetNextMode() error: %v", err)
	}
	if mode != "" {
		t.Errorf("mode = %q, want empty", mode)
	}

	// Set next mode
	err = store.SetNextMode(userID, "cloud")
	if err != nil {
		t.Fatalf("SetNextMode() error: %v", err)
	}

	// Verify and clear
	mode, err = store.GetAndClearNextMode(userID)
	if err != nil {
		t.Fatalf("GetAndClearNextMode() error: %v", err)
	}
	if mode != "cloud" {
		t.Errorf("mode = %q, want %q", mode, "cloud")
	}

	// Should be cleared now
	mode, _ = store.GetNextMode(userID)
	if mode != "" {
		t.Errorf("mode = %q, want empty after clear", mode)
	}
}

func TestSettingsStore_NextWithSummary(t *testing.T) {
	store := setupSettingsStore(t)
	userID := int64(123456)

	// Initially should return false
	withSummary, err := store.GetAndClearNextWithSummary(userID)
	if err != nil {
		t.Fatalf("GetAndClearNextWithSummary() error: %v", err)
	}
	if withSummary {
		t.Error("withSummary = true, want false initially")
	}

	// Set next with summary
	err = store.SetNextWithSummary(userID, true)
	if err != nil {
		t.Fatalf("SetNextWithSummary() error: %v", err)
	}

	// Verify and clear
	withSummary, err = store.GetAndClearNextWithSummary(userID)
	if err != nil {
		t.Fatalf("GetAndClearNextWithSummary() error: %v", err)
	}
	if !withSummary {
		t.Error("withSummary = false, want true")
	}

	// Should be cleared now
	withSummary, _ = store.GetAndClearNextWithSummary(userID)
	if withSummary {
		t.Error("withSummary = true, want false after clear")
	}
}
