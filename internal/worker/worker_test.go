package worker

import (
	"testing"
)

func TestWorker_New(t *testing.T) {
	w := New(Config{
		DataDir:       "/tmp/data",
		DefaultPrompt: "Summarize this",
	})

	if w == nil {
		t.Fatal("New() returned nil")
	}

	if w.config.DataDir != "/tmp/data" {
		t.Errorf("DataDir = %q, want %q", w.config.DataDir, "/tmp/data")
	}
}
