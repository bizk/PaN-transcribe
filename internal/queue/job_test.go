package queue

import (
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *JobStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB() error: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return NewJobStore(db)
}

func TestJobStore_CreateAndGet(t *testing.T) {
	store := setupTestDB(t)

	job := &Job{
		ChatID:      123456,
		MessageID:   789,
		AudioPath:   "/tmp/audio.wav",
		Mode:        "local",
		WithSummary: true,
	}

	id, err := store.Create(job)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if got.ChatID != job.ChatID {
		t.Errorf("ChatID = %d, want %d", got.ChatID, job.ChatID)
	}
	if got.Status != StatusPending {
		t.Errorf("Status = %q, want %q", got.Status, StatusPending)
	}
}

func TestJobStore_GetNextPending(t *testing.T) {
	store := setupTestDB(t)

	job1 := &Job{ChatID: 1, MessageID: 1, AudioPath: "/tmp/1.wav", Mode: "local"}
	job2 := &Job{ChatID: 2, MessageID: 2, AudioPath: "/tmp/2.wav", Mode: "local"}

	store.Create(job1)
	store.Create(job2)

	next, err := store.GetNextPending()
	if err != nil {
		t.Fatalf("GetNextPending() error: %v", err)
	}
	if next.ChatID != 1 {
		t.Errorf("ChatID = %d, want 1", next.ChatID)
	}
}

func TestJobStore_UpdateStatus(t *testing.T) {
	store := setupTestDB(t)

	job := &Job{ChatID: 1, MessageID: 1, AudioPath: "/tmp/test.wav", Mode: "local"}
	id, _ := store.Create(job)

	err := store.UpdateStatus(id, StatusProcessing)
	if err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	got, _ := store.Get(id)
	if got.Status != StatusProcessing {
		t.Errorf("Status = %q, want %q", got.Status, StatusProcessing)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt should be set when status changes to processing")
	}
}

func TestJobStore_Complete(t *testing.T) {
	store := setupTestDB(t)

	job := &Job{ChatID: 1, MessageID: 1, AudioPath: "/tmp/test.wav", Mode: "local"}
	id, _ := store.Create(job)

	err := store.Complete(id, "/tmp/output.txt", "/tmp/summary.txt")
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	got, _ := store.Get(id)
	if got.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, StatusCompleted)
	}
	if got.OutputPath != "/tmp/output.txt" {
		t.Errorf("OutputPath = %q, want %q", got.OutputPath, "/tmp/output.txt")
	}
}

func TestJobStore_Fail(t *testing.T) {
	store := setupTestDB(t)

	job := &Job{ChatID: 1, MessageID: 1, AudioPath: "/tmp/test.wav", Mode: "local"}
	id, _ := store.Create(job)

	err := store.Fail(id, "transcription failed")
	if err != nil {
		t.Fatalf("Fail() error: %v", err)
	}

	got, _ := store.Get(id)
	if got.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, StatusFailed)
	}
	if got.ErrorMessage != "transcription failed" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "transcription failed")
	}
}

func TestJobStore_CountPending(t *testing.T) {
	store := setupTestDB(t)

	for i := 0; i < 3; i++ {
		job := &Job{ChatID: int64(i), MessageID: i, AudioPath: "/tmp/test.wav", Mode: "local"}
		store.Create(job)
	}

	count, err := store.CountPending()
	if err != nil {
		t.Fatalf("CountPending() error: %v", err)
	}
	if count != 3 {
		t.Errorf("CountPending() = %d, want 3", count)
	}
}

func TestJobStore_GetPendingBefore(t *testing.T) {
	store := setupTestDB(t)

	job := &Job{ChatID: 1, MessageID: 1, AudioPath: "/tmp/test.wav", Mode: "local"}
	id, _ := store.Create(job)

	count, err := store.GetPendingBefore(id)
	if err != nil {
		t.Fatalf("GetPendingBefore() error: %v", err)
	}
	if count != 0 {
		t.Errorf("GetPendingBefore() = %d, want 0", count)
	}

	job2 := &Job{ChatID: 2, MessageID: 2, AudioPath: "/tmp/test2.wav", Mode: "local"}
	id2, _ := store.Create(job2)

	count, _ = store.GetPendingBefore(id2)
	if count != 1 {
		t.Errorf("GetPendingBefore() = %d, want 1", count)
	}
}

func TestJobStore_ResetProcessingJobs(t *testing.T) {
	store := setupTestDB(t)

	// Create a job and set to processing
	job := &Job{ChatID: 1, MessageID: 1, AudioPath: "/tmp/test.wav", Mode: "local"}
	id, _ := store.Create(job)
	store.UpdateStatus(id, StatusProcessing)

	// Reset processing jobs
	err := store.ResetProcessingJobs()
	if err != nil {
		t.Fatalf("ResetProcessingJobs() error: %v", err)
	}

	// Verify it's back to pending
	got, _ := store.Get(id)
	if got.Status != StatusPending {
		t.Errorf("Status = %q, want %q", got.Status, StatusPending)
	}
}

func TestJobStore_GetJobsForUser(t *testing.T) {
	store := setupTestDB(t)

	// Create jobs for user 1
	for i := 0; i < 3; i++ {
		job := &Job{ChatID: 1, MessageID: i, AudioPath: "/tmp/test.wav", Mode: "local"}
		store.Create(job)
	}

	// Create job for user 2
	job := &Job{ChatID: 2, MessageID: 1, AudioPath: "/tmp/test.wav", Mode: "local"}
	store.Create(job)

	// Get jobs for user 1
	jobs, err := store.GetJobsForUser(1)
	if err != nil {
		t.Fatalf("GetJobsForUser() error: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("len(jobs) = %d, want 3", len(jobs))
	}
}
