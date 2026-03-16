package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanup_RemovesOldFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an "old" file (we'll mock the time check)
	oldFile := filepath.Join(tmpDir, "old.txt")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a "new" file
	newFile := filepath.Join(tmpDir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modify the old file's mtime to be 40 days ago
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	c := New(Config{
		OutputDir:     tmpDir,
		RetentionDays: 30,
	})

	removed, err := c.CleanOldFiles()
	if err != nil {
		t.Fatalf("CleanOldFiles() error: %v", err)
	}

	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// Old file should be gone
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should have been removed")
	}

	// New file should still exist
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("new file should still exist")
	}
}
