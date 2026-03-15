# Telegram Transcription Bot Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Telegram bot in Go that transcribes psychology class audio files using whisper.cpp (local) or OpenAI Whisper API (cloud), with optional GPT-4o-mini summaries.

**Architecture:** Single Go binary with embedded SQLite job queue. Bot receives audio via Telegram, queues jobs, and a worker goroutine processes them sequentially. Transcription uses whisper.cpp CLI by default, with OpenAI API as fallback. Summaries generated via OpenAI GPT-4o-mini.

**Tech Stack:** Go 1.21+, SQLite, telegram-bot-api/v5, go-openai, viper, cron/v3, whisper.cpp, ffmpeg

---

## File Structure

```
pan-transcribe/
├── cmd/
│   └── bot/
│       └── main.go              # Entry point, wires everything together
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration loading with viper
│   ├── bot/
│   │   ├── bot.go               # Telegram bot initialization
│   │   ├── handlers.go          # Command and message handlers
│   │   └── auth.go              # User authorization
│   ├── queue/
│   │   ├── db.go                # SQLite connection and migrations
│   │   ├── job.go               # Job model and CRUD operations
│   │   └── settings.go          # User settings (custom prompt)
│   ├── worker/
│   │   └── worker.go            # Job processing worker
│   ├── transcribe/
│   │   ├── transcriber.go       # Transcriber interface
│   │   ├── whisper.go           # Whisper.cpp CLI integration
│   │   └── openai.go            # OpenAI Whisper API integration
│   ├── summary/
│   │   └── summary.go           # GPT-4o-mini summary generation
│   └── cleanup/
│       └── cleanup.go           # Scheduled file cleanup
├── config.yaml.example          # Example configuration
├── go.mod
├── go.sum
├── Makefile                     # Build and deployment commands
└── README.md                    # Setup instructions (created at end)
```

---

## Chunk 1: Project Setup & Configuration

### Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `go.sum`

- [ ] **Step 1: Initialize Go module**

```bash
go mod init github.com/override/pan-transcribe
```

Expected: `go.mod` created with module path

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/go-telegram-bot-api/telegram-bot-api/v5@latest
go get github.com/sashabaranov/go-openai@latest
go get github.com/mattn/go-sqlite3@latest
go get github.com/spf13/viper@latest
go get github.com/robfig/cron/v3@latest
```

Expected: `go.sum` populated with dependencies

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "Initialize Go module with dependencies"
```

---

### Task 2: Create Configuration Module

**Files:**
- Create: `internal/config/config.go`
- Create: `config.yaml.example`

- [ ] **Step 1: Write config test file**

Create: `internal/config/config_test.go`

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  allowed_users:
    - 123456789

openai:
  api_key: "${OPENAI_API_KEY}"
  whisper_model: "whisper-1"
  summary_model: "gpt-4o-mini"

whisper:
  model_path: "./whisper.cpp/models/ggml-small.bin"
  threads: 4

processing:
  default_mode: "local"
  max_file_size_mb: 100
  output_retention_days: 30

summary:
  default_prompt: "Summarize this text"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set env vars
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	os.Setenv("OPENAI_API_KEY", "test-api-key")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Telegram.BotToken != "test-token" {
		t.Errorf("BotToken = %q, want %q", cfg.Telegram.BotToken, "test-token")
	}
	if cfg.OpenAI.APIKey != "test-api-key" {
		t.Errorf("APIKey = %q, want %q", cfg.OpenAI.APIKey, "test-api-key")
	}
	if len(cfg.Telegram.AllowedUsers) != 1 || cfg.Telegram.AllowedUsers[0] != 123456789 {
		t.Errorf("AllowedUsers = %v, want [123456789]", cfg.Telegram.AllowedUsers)
	}
	if cfg.Processing.MaxFileSizeMB != 100 {
		t.Errorf("MaxFileSizeMB = %d, want 100", cfg.Processing.MaxFileSizeMB)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Implement config module**

Create: `internal/config/config.go`

```go
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	OpenAI     OpenAIConfig     `mapstructure:"openai"`
	Whisper    WhisperConfig    `mapstructure:"whisper"`
	Processing ProcessingConfig `mapstructure:"processing"`
	Summary    SummaryConfig    `mapstructure:"summary"`
}

type TelegramConfig struct {
	BotToken     string  `mapstructure:"bot_token"`
	AllowedUsers []int64 `mapstructure:"allowed_users"`
}

type OpenAIConfig struct {
	APIKey       string `mapstructure:"api_key"`
	WhisperModel string `mapstructure:"whisper_model"`
	SummaryModel string `mapstructure:"summary_model"`
}

type WhisperConfig struct {
	ModelPath string `mapstructure:"model_path"`
	Threads   int    `mapstructure:"threads"`
}

type ProcessingConfig struct {
	DefaultMode         string `mapstructure:"default_mode"`
	MaxFileSizeMB       int    `mapstructure:"max_file_size_mb"`
	OutputRetentionDays int    `mapstructure:"output_retention_days"`
}

type SummaryConfig struct {
	DefaultPrompt string `mapstructure:"default_prompt"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	// Expand environment variables in string values
	for _, key := range v.AllKeys() {
		val := v.GetString(key)
		if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
			envVar := val[2 : len(val)-1]
			v.Set(key, os.Getenv(envVar))
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/config/... -v
```

Expected: PASS

- [ ] **Step 5: Create example config file**

Create: `config.yaml.example`

```yaml
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  allowed_users:
    - 123456789  # Your Telegram user ID (get it from @userinfobot)

openai:
  api_key: "${OPENAI_API_KEY}"
  whisper_model: "whisper-1"
  summary_model: "gpt-4o-mini"

whisper:
  model_path: "./whisper.cpp/models/ggml-small.bin"
  threads: 4

processing:
  default_mode: "local"  # "local" for whisper.cpp, "cloud" for OpenAI API
  max_file_size_mb: 100
  output_retention_days: 30

summary:
  default_prompt: |
    Eres un asistente para estudiantes de psicología. Resume esta transcripción
    de clase incluyendo:
    - Temas principales cubiertos
    - Conceptos clave y definiciones
    - Teorías o autores mencionados
    - Puntos importantes para estudiar
    Responde en español.
```

- [ ] **Step 6: Commit**

```bash
git add internal/config/ config.yaml.example
git commit -m "Add configuration module with viper"
```

---

### Task 3: Create Directory Structure

**Files:**
- Create: `cmd/bot/main.go` (placeholder)
- Create: `Makefile`
- Create: `data/.gitkeep`

- [ ] **Step 1: Create placeholder main.go**

Create: `cmd/bot/main.go`

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/override/pan-transcribe/internal/config"
)

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Bot configured for %d allowed users\n", len(cfg.Telegram.AllowedUsers))
	fmt.Println("Bot placeholder - implementation pending")
}
```

- [ ] **Step 2: Create Makefile**

Create: `Makefile`

```makefile
.PHONY: build test run clean build-pi

# Build for current platform
build:
	go build -o bin/bot ./cmd/bot

# Build for Raspberry Pi 5 (ARM64)
build-pi:
	GOOS=linux GOARCH=arm64 go build -o bin/bot-arm64 ./cmd/bot

# Run tests
test:
	go test ./... -v

# Run locally
run: build
	./bin/bot config.yaml

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf data/audio/*
	rm -rf data/output/*

# Create data directories
init-dirs:
	mkdir -p data/audio data/output logs
```

- [ ] **Step 3: Create data directory structure**

```bash
mkdir -p data/audio data/output logs
touch data/.gitkeep logs/.gitkeep
echo "data/audio/*" >> .gitignore
echo "data/output/*" >> .gitignore
echo "logs/*.log" >> .gitignore
echo "!data/.gitkeep" >> .gitignore
echo "!logs/.gitkeep" >> .gitignore
echo "config.yaml" >> .gitignore
echo "bin/" >> .gitignore
echo "*.db" >> .gitignore
```

- [ ] **Step 4: Verify build works**

```bash
make build
```

Expected: `bin/bot` created successfully

- [ ] **Step 5: Commit**

```bash
git add cmd/bot/main.go Makefile data/.gitkeep logs/.gitkeep .gitignore
git commit -m "Add project structure with Makefile"
```

---

## Chunk 2: SQLite Job Queue

### Task 4: Create Database Connection Module

**Files:**
- Create: `internal/queue/db.go`
- Create: `internal/queue/db_test.go`

- [ ] **Step 1: Write database test**

Create: `internal/queue/db_test.go`

```go
package queue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB() error: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='jobs'").Scan(&tableName)
	if err != nil {
		t.Errorf("jobs table not created: %v", err)
	}

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='settings'").Scan(&tableName)
	if err != nil {
		t.Errorf("settings table not created: %v", err)
	}
}

func TestOpenDB_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "test.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB() error: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/queue/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Implement database module**

Create: `internal/queue/db.go`

```go
package queue

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func OpenDB(dbPath string) (*sql.DB, error) {
	// Create directory if needed
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		message_id INTEGER NOT NULL,
		audio_path TEXT,
		output_path TEXT,
		summary_path TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		mode TEXT NOT NULL DEFAULT 'local',
		with_summary INTEGER NOT NULL DEFAULT 0,
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS settings (
		user_id INTEGER PRIMARY KEY,
		custom_prompt TEXT,
		next_mode TEXT,
		next_with_summary INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
	`

	_, err := db.Exec(schema)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/queue/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/queue/db.go internal/queue/db_test.go
git commit -m "Add SQLite database module with migrations"
```

---

### Task 5: Create Job Model and Operations

**Files:**
- Create: `internal/queue/job.go`
- Create: `internal/queue/job_test.go`

- [ ] **Step 1: Write job operations test**

Create: `internal/queue/job_test.go`

```go
package queue

import (
	"path/filepath"
	"testing"
	"time"
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

	// Create two jobs
	job1 := &Job{ChatID: 1, MessageID: 1, AudioPath: "/tmp/1.wav", Mode: "local"}
	job2 := &Job{ChatID: 2, MessageID: 2, AudioPath: "/tmp/2.wav", Mode: "local"}

	store.Create(job1)
	store.Create(job2)

	// Get next pending - should be first job
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

	// Create 3 pending jobs
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

	// Get pending before job - should be 0
	count, err := store.GetPendingBefore(id)
	if err != nil {
		t.Fatalf("GetPendingBefore() error: %v", err)
	}
	if count != 0 {
		t.Errorf("GetPendingBefore() = %d, want 0", count)
	}

	// Create another job
	job2 := &Job{ChatID: 2, MessageID: 2, AudioPath: "/tmp/test2.wav", Mode: "local"}
	id2, _ := store.Create(job2)

	// Get pending before second job - should be 1
	count, _ = store.GetPendingBefore(id2)
	if count != 1 {
		t.Errorf("GetPendingBefore() = %d, want 1", count)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/queue/... -v -run TestJobStore
```

Expected: FAIL - Job type not defined

- [ ] **Step 3: Implement job module**

Create: `internal/queue/job.go`

```go
package queue

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type Job struct {
	ID           int64
	ChatID       int64
	MessageID    int
	AudioPath    string
	OutputPath   string
	SummaryPath  string
	Status       string
	Mode         string
	WithSummary  bool
	ErrorMessage string
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
}

type JobStore struct {
	db *sql.DB
}

func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{db: db}
}

func (s *JobStore) Create(job *Job) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO jobs (chat_id, message_id, audio_path, mode, with_summary, status)
		VALUES (?, ?, ?, ?, ?, ?)`,
		job.ChatID, job.MessageID, job.AudioPath, job.Mode, job.WithSummary, StatusPending,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting job: %w", err)
	}
	return result.LastInsertId()
}

func (s *JobStore) Get(id int64) (*Job, error) {
	job := &Job{}
	var withSummary int
	err := s.db.QueryRow(`
		SELECT id, chat_id, message_id, audio_path, COALESCE(output_path, ''),
		       COALESCE(summary_path, ''), status, mode, with_summary,
		       COALESCE(error_message, ''), created_at, started_at, completed_at
		FROM jobs WHERE id = ?`, id,
	).Scan(
		&job.ID, &job.ChatID, &job.MessageID, &job.AudioPath, &job.OutputPath,
		&job.SummaryPath, &job.Status, &job.Mode, &withSummary,
		&job.ErrorMessage, &job.CreatedAt, &job.StartedAt, &job.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting job: %w", err)
	}
	job.WithSummary = withSummary == 1
	return job, nil
}

func (s *JobStore) GetNextPending() (*Job, error) {
	job := &Job{}
	var withSummary int
	err := s.db.QueryRow(`
		SELECT id, chat_id, message_id, audio_path, COALESCE(output_path, ''),
		       COALESCE(summary_path, ''), status, mode, with_summary,
		       COALESCE(error_message, ''), created_at, started_at, completed_at
		FROM jobs
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT 1`, StatusPending,
	).Scan(
		&job.ID, &job.ChatID, &job.MessageID, &job.AudioPath, &job.OutputPath,
		&job.SummaryPath, &job.Status, &job.Mode, &withSummary,
		&job.ErrorMessage, &job.CreatedAt, &job.StartedAt, &job.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting next pending job: %w", err)
	}
	job.WithSummary = withSummary == 1
	return job, nil
}

func (s *JobStore) UpdateStatus(id int64, status string) error {
	var query string
	switch status {
	case StatusProcessing:
		query = `UPDATE jobs SET status = ?, started_at = CURRENT_TIMESTAMP WHERE id = ?`
	case StatusCompleted:
		query = `UPDATE jobs SET status = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`
	default:
		query = `UPDATE jobs SET status = ? WHERE id = ?`
	}

	_, err := s.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("updating job status: %w", err)
	}
	return nil
}

func (s *JobStore) Complete(id int64, outputPath, summaryPath string) error {
	_, err := s.db.Exec(`
		UPDATE jobs
		SET status = ?, output_path = ?, summary_path = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		StatusCompleted, outputPath, summaryPath, id,
	)
	if err != nil {
		return fmt.Errorf("completing job: %w", err)
	}
	return nil
}

func (s *JobStore) Fail(id int64, errorMessage string) error {
	_, err := s.db.Exec(`
		UPDATE jobs
		SET status = ?, error_message = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		StatusFailed, errorMessage, id,
	)
	if err != nil {
		return fmt.Errorf("failing job: %w", err)
	}
	return nil
}

func (s *JobStore) CountPending() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status = ?`, StatusPending).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending jobs: %w", err)
	}
	return count, nil
}

func (s *JobStore) GetPendingBefore(id int64) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM jobs
		WHERE status = ? AND id < ?`, StatusPending, id,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending jobs before: %w", err)
	}
	return count, nil
}

func (s *JobStore) ResetProcessingJobs() error {
	_, err := s.db.Exec(`
		UPDATE jobs SET status = ?, started_at = NULL
		WHERE status = ?`,
		StatusPending, StatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("resetting processing jobs: %w", err)
	}
	return nil
}

func (s *JobStore) GetJobsForUser(chatID int64) ([]*Job, error) {
	rows, err := s.db.Query(`
		SELECT id, chat_id, message_id, audio_path, COALESCE(output_path, ''),
		       COALESCE(summary_path, ''), status, mode, with_summary,
		       COALESCE(error_message, ''), created_at, started_at, completed_at
		FROM jobs
		WHERE chat_id = ? AND status IN (?, ?)
		ORDER BY created_at DESC
		LIMIT 10`, chatID, StatusPending, StatusProcessing,
	)
	if err != nil {
		return nil, fmt.Errorf("getting jobs for user: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job := &Job{}
		var withSummary int
		err := rows.Scan(
			&job.ID, &job.ChatID, &job.MessageID, &job.AudioPath, &job.OutputPath,
			&job.SummaryPath, &job.Status, &job.Mode, &withSummary,
			&job.ErrorMessage, &job.CreatedAt, &job.StartedAt, &job.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning job: %w", err)
		}
		job.WithSummary = withSummary == 1
		jobs = append(jobs, job)
	}
	return jobs, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/queue/... -v
```

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/queue/job.go internal/queue/job_test.go
git commit -m "Add job model with CRUD operations"
```

---

### Task 6: Create Settings Store

**Files:**
- Create: `internal/queue/settings.go`
- Create: `internal/queue/settings_test.go`

- [ ] **Step 1: Write settings test**

Create: `internal/queue/settings_test.go`

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/queue/... -v -run TestSettingsStore
```

Expected: FAIL - SettingsStore not defined

- [ ] **Step 3: Implement settings store**

Create: `internal/queue/settings.go`

```go
package queue

import (
	"database/sql"
	"fmt"
)

type SettingsStore struct {
	db *sql.DB
}

func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

func (s *SettingsStore) GetCustomPrompt(userID int64) (string, error) {
	var prompt sql.NullString
	err := s.db.QueryRow(`SELECT custom_prompt FROM settings WHERE user_id = ?`, userID).Scan(&prompt)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting custom prompt: %w", err)
	}
	return prompt.String, nil
}

func (s *SettingsStore) SetCustomPrompt(userID int64, prompt string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (user_id, custom_prompt) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET custom_prompt = excluded.custom_prompt`,
		userID, prompt,
	)
	if err != nil {
		return fmt.Errorf("setting custom prompt: %w", err)
	}
	return nil
}

func (s *SettingsStore) GetNextMode(userID int64) (string, error) {
	var mode sql.NullString
	err := s.db.QueryRow(`SELECT next_mode FROM settings WHERE user_id = ?`, userID).Scan(&mode)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting next mode: %w", err)
	}
	return mode.String, nil
}

func (s *SettingsStore) SetNextMode(userID int64, mode string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (user_id, next_mode) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET next_mode = excluded.next_mode`,
		userID, mode,
	)
	if err != nil {
		return fmt.Errorf("setting next mode: %w", err)
	}
	return nil
}

func (s *SettingsStore) GetAndClearNextMode(userID int64) (string, error) {
	mode, err := s.GetNextMode(userID)
	if err != nil {
		return "", err
	}

	if mode != "" {
		_, err = s.db.Exec(`UPDATE settings SET next_mode = NULL WHERE user_id = ?`, userID)
		if err != nil {
			return "", fmt.Errorf("clearing next mode: %w", err)
		}
	}

	return mode, nil
}

func (s *SettingsStore) SetNextWithSummary(userID int64, withSummary bool) error {
	val := 0
	if withSummary {
		val = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO settings (user_id, next_with_summary) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET next_with_summary = excluded.next_with_summary`,
		userID, val,
	)
	if err != nil {
		return fmt.Errorf("setting next with summary: %w", err)
	}
	return nil
}

func (s *SettingsStore) GetAndClearNextWithSummary(userID int64) (bool, error) {
	var val sql.NullInt64
	err := s.db.QueryRow(`SELECT next_with_summary FROM settings WHERE user_id = ?`, userID).Scan(&val)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("getting next with summary: %w", err)
	}

	// Clear the value
	_, err = s.db.Exec(`UPDATE settings SET next_with_summary = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return false, fmt.Errorf("clearing next with summary: %w", err)
	}

	return val.Int64 == 1, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/queue/... -v
```

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/queue/settings.go internal/queue/settings_test.go
git commit -m "Add settings store for user preferences"
```

---

## Chunk 3: Telegram Bot Core

### Task 7: Create Authorization Module

**Files:**
- Create: `internal/bot/auth.go`
- Create: `internal/bot/auth_test.go`

- [ ] **Step 1: Write auth test**

Create: `internal/bot/auth_test.go`

```go
package bot

import (
	"testing"
)

func TestAuthorizer_IsAllowed(t *testing.T) {
	auth := NewAuthorizer([]int64{123, 456, 789})

	tests := []struct {
		userID int64
		want   bool
	}{
		{123, true},
		{456, true},
		{789, true},
		{999, false},
		{0, false},
	}

	for _, tt := range tests {
		got := auth.IsAllowed(tt.userID)
		if got != tt.want {
			t.Errorf("IsAllowed(%d) = %v, want %v", tt.userID, got, tt.want)
		}
	}
}

func TestAuthorizer_EmptyList(t *testing.T) {
	auth := NewAuthorizer([]int64{})

	if auth.IsAllowed(123) {
		t.Error("IsAllowed() = true for empty list, want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/bot/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Implement auth module**

Create: `internal/bot/auth.go`

```go
package bot

type Authorizer struct {
	allowedUsers map[int64]bool
}

func NewAuthorizer(allowedUsers []int64) *Authorizer {
	m := make(map[int64]bool)
	for _, id := range allowedUsers {
		m[id] = true
	}
	return &Authorizer{allowedUsers: m}
}

func (a *Authorizer) IsAllowed(userID int64) bool {
	return a.allowedUsers[userID]
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/bot/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bot/auth.go internal/bot/auth_test.go
git commit -m "Add user authorization module"
```

---

### Task 8: Create Bot Initialization

**Files:**
- Create: `internal/bot/bot.go`

- [ ] **Step 1: Implement bot initialization**

Create: `internal/bot/bot.go`

```go
package bot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/override/pan-transcribe/internal/config"
	"github.com/override/pan-transcribe/internal/queue"
)

type Bot struct {
	api          *tgbotapi.BotAPI
	auth         *Authorizer
	jobStore     *queue.JobStore
	settingsStore *queue.SettingsStore
	config       *config.Config
	dataDir      string
}

func New(cfg *config.Config, jobStore *queue.JobStore, settingsStore *queue.SettingsStore, dataDir string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("creating bot API: %w", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:          api,
		auth:         NewAuthorizer(cfg.Telegram.AllowedUsers),
		jobStore:     jobStore,
		settingsStore: settingsStore,
		config:       cfg,
		dataDir:      dataDir,
	}, nil
}

func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check authorization
		if !b.auth.IsAllowed(update.Message.From.ID) {
			log.Printf("Unauthorized access attempt from user %d", update.Message.From.ID)
			continue
		}

		go b.handleMessage(update.Message)
	}

	return nil
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		b.handleCommand(msg)
		return
	}

	// Check for audio file
	if msg.Audio != nil || msg.Voice != nil || msg.Document != nil {
		b.handleAudioUpload(msg)
		return
	}
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		b.handleStart(msg)
	case "transcribe":
		b.handleTranscribe(msg)
	case "summarize":
		b.handleSummarize(msg)
	case "status":
		b.handleStatus(msg)
	case "setprompt":
		b.handleSetPrompt(msg)
	case "showprompt":
		b.handleShowPrompt(msg)
	case "cloud":
		b.handleCloud(msg)
	case "local":
		b.handleLocal(msg)
	default:
		b.reply(msg.Chat.ID, "Comando desconocido. Usa /start para ver los comandos disponibles.")
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (b *Bot) replyWithFile(chatID int64, filePath string, caption string) error {
	file := tgbotapi.FilePath(filePath)
	doc := tgbotapi.NewDocument(chatID, file)
	doc.Caption = caption

	_, err := b.api.Send(doc)
	return err
}

func (b *Bot) SendResult(chatID int64, outputPath, summaryPath string) error {
	// Send transcription file
	if err := b.replyWithFile(chatID, outputPath, "Transcripción completada"); err != nil {
		return fmt.Errorf("sending transcription: %w", err)
	}

	// Send summary file if exists
	if summaryPath != "" {
		if err := b.replyWithFile(chatID, summaryPath, "Resumen de la clase"); err != nil {
			return fmt.Errorf("sending summary: %w", err)
		}
	}

	return nil
}

func (b *Bot) SendError(chatID int64, errorMsg string) {
	b.reply(chatID, fmt.Sprintf("Error: %s", errorMsg))
}

func (b *Bot) SendProgress(chatID int64, percent int, estimatedMinutes int) {
	text := fmt.Sprintf("Procesando... %d%% completado. Tiempo restante estimado: ~%d min", percent, estimatedMinutes)
	b.reply(chatID, text)
}
```

- [ ] **Step 2: Verify module compiles**

```bash
go build ./internal/bot/...
```

Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add internal/bot/bot.go
git commit -m "Add bot initialization and message routing"
```

---

### Task 9: Implement Command Handlers

**Files:**
- Create: `internal/bot/handlers.go`

- [ ] **Step 1: Implement command handlers**

Create: `internal/bot/handlers.go`

```go
package bot

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/override/pan-transcribe/internal/queue"
)

var supportedFormats = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".m4a":  true,
	".ogg":  true,
	".flac": true,
	".oga":  true,
}

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	text := `Hola! Soy tu bot de transcripción para clases de psicología.

Comandos disponibles:
/transcribe - Solo transcribir audio
/summarize - Transcribir y generar resumen
/status - Ver estado de trabajos pendientes
/setprompt <texto> - Personalizar prompt de resumen
/showprompt - Ver prompt actual
/cloud - Usar API de OpenAI (más rápido)
/local - Usar Whisper local (por defecto)

Envíame un archivo de audio (mp3, wav, m4a, ogg, flac) después de seleccionar el modo con /transcribe o /summarize.`

	b.reply(msg.Chat.ID, text)
}

func (b *Bot) handleTranscribe(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextWithSummary(msg.From.ID, false); err != nil {
		log.Printf("Error setting mode: %v", err)
	}
	b.reply(msg.Chat.ID, "Modo transcripción activado. Envíame un archivo de audio.")
}

func (b *Bot) handleSummarize(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextWithSummary(msg.From.ID, true); err != nil {
		log.Printf("Error setting mode: %v", err)
	}
	b.reply(msg.Chat.ID, "Modo transcripción + resumen activado. Envíame un archivo de audio.")
}

func (b *Bot) handleStatus(msg *tgbotapi.Message) {
	jobs, err := b.jobStore.GetJobsForUser(msg.Chat.ID)
	if err != nil {
		log.Printf("Error getting jobs: %v", err)
		b.reply(msg.Chat.ID, "Error al obtener estado de trabajos.")
		return
	}

	if len(jobs) == 0 {
		b.reply(msg.Chat.ID, "No tienes trabajos pendientes o en proceso.")
		return
	}

	var sb strings.Builder
	sb.WriteString("Tus trabajos:\n\n")

	for _, job := range jobs {
		status := "Pendiente"
		if job.Status == queue.StatusProcessing {
			status = "Procesando"
		}
		sb.WriteString(fmt.Sprintf("- #%d: %s\n", job.ID, status))
	}

	b.reply(msg.Chat.ID, sb.String())
}

func (b *Bot) handleSetPrompt(msg *tgbotapi.Message) {
	prompt := strings.TrimPrefix(msg.Text, "/setprompt ")
	prompt = strings.TrimSpace(prompt)

	if prompt == "" || prompt == "/setprompt" {
		b.reply(msg.Chat.ID, "Uso: /setprompt <tu prompt personalizado>")
		return
	}

	if err := b.settingsStore.SetCustomPrompt(msg.From.ID, prompt); err != nil {
		log.Printf("Error setting prompt: %v", err)
		b.reply(msg.Chat.ID, "Error al guardar el prompt.")
		return
	}

	b.reply(msg.Chat.ID, "Prompt personalizado guardado.")
}

func (b *Bot) handleShowPrompt(msg *tgbotapi.Message) {
	prompt, err := b.settingsStore.GetCustomPrompt(msg.From.ID)
	if err != nil {
		log.Printf("Error getting prompt: %v", err)
		b.reply(msg.Chat.ID, "Error al obtener el prompt.")
		return
	}

	if prompt == "" {
		prompt = b.config.Summary.DefaultPrompt
		b.reply(msg.Chat.ID, fmt.Sprintf("Usando prompt por defecto:\n\n%s", prompt))
	} else {
		b.reply(msg.Chat.ID, fmt.Sprintf("Tu prompt personalizado:\n\n%s", prompt))
	}
}

func (b *Bot) handleCloud(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextMode(msg.From.ID, "cloud"); err != nil {
		log.Printf("Error setting mode: %v", err)
		b.reply(msg.Chat.ID, "Error al configurar modo.")
		return
	}
	b.reply(msg.Chat.ID, "El próximo audio se procesará con la API de OpenAI (más rápido).")
}

func (b *Bot) handleLocal(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextMode(msg.From.ID, "local"); err != nil {
		log.Printf("Error setting mode: %v", err)
		b.reply(msg.Chat.ID, "Error al configurar modo.")
		return
	}
	b.reply(msg.Chat.ID, "El próximo audio se procesará con Whisper local.")
}

func (b *Bot) handleAudioUpload(msg *tgbotapi.Message) {
	var fileID string
	var fileName string
	var fileSize int

	if msg.Audio != nil {
		fileID = msg.Audio.FileID
		fileName = msg.Audio.FileName
		fileSize = msg.Audio.FileSize
	} else if msg.Voice != nil {
		fileID = msg.Voice.FileID
		fileName = "voice.ogg"
		fileSize = msg.Voice.FileSize
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
		fileName = msg.Document.FileName
		fileSize = msg.Document.FileSize
	}

	// Check file size
	maxSize := b.config.Processing.MaxFileSizeMB * 1024 * 1024
	if fileSize > maxSize {
		b.reply(msg.Chat.ID, fmt.Sprintf("Archivo muy grande. Máximo: %dMB", b.config.Processing.MaxFileSizeMB))
		return
	}

	// Check format
	ext := strings.ToLower(filepath.Ext(fileName))
	if !supportedFormats[ext] {
		b.reply(msg.Chat.ID, "Formato no soportado. Usa: mp3, wav, m4a, ogg, flac")
		return
	}

	// Download file
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Printf("Error getting file: %v", err)
		b.reply(msg.Chat.ID, "Error al obtener el archivo.")
		return
	}

	// Create job first to get ID for filename
	mode, _ := b.settingsStore.GetAndClearNextMode(msg.From.ID)
	if mode == "" {
		mode = b.config.Processing.DefaultMode
	}

	// Get withSummary setting from user preferences (set by /transcribe or /summarize commands)
	withSummary, _ := b.settingsStore.GetAndClearNextWithSummary(msg.From.ID)

	// Download to temp file
	audioDir := filepath.Join(b.dataDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		log.Printf("Error creating audio dir: %v", err)
		b.reply(msg.Chat.ID, "Error interno al preparar directorio.")
		return
	}

	// Create job
	job := &queue.Job{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.MessageID,
		Mode:        mode,
		WithSummary: withSummary,
	}

	jobID, err := b.jobStore.Create(job)
	if err != nil {
		log.Printf("Error creating job: %v", err)
		b.reply(msg.Chat.ID, "Error al crear el trabajo.")
		return
	}

	// Download file
	audioPath := filepath.Join(audioDir, fmt.Sprintf("%d%s", jobID, ext))
	if err := b.downloadFile(file.Link(b.api.Token), audioPath); err != nil {
		log.Printf("Error downloading file: %v", err)
		b.jobStore.Fail(jobID, "Error al descargar archivo")
		b.reply(msg.Chat.ID, "Error al descargar el archivo.")
		return
	}

	// Update job with audio path
	// Note: This is a simplification - ideally we'd have an Update method
	// For now, we'll delete and recreate with the path
	// Actually, let's add a SetAudioPath method
	if err := b.setJobAudioPath(jobID, audioPath); err != nil {
		log.Printf("Error setting audio path: %v", err)
		b.reply(msg.Chat.ID, "Error interno.")
		return
	}

	// Get queue position
	position, _ := b.jobStore.GetPendingBefore(jobID)
	position++ // 1-indexed for user

	// Estimate time (rough: 1 min audio = 2 min processing for local)
	estimatedMinutes := 5 // Base estimate
	if mode == "cloud" {
		estimatedMinutes = 2
	}

	b.reply(msg.Chat.ID, fmt.Sprintf(
		"Audio recibido. Posición en cola: #%d. Tiempo estimado: ~%d minutos. Usa /status para ver el progreso.",
		position, estimatedMinutes,
	))
}

func (b *Bot) downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (b *Bot) setJobAudioPath(jobID int64, audioPath string) error {
	// This requires adding a method to JobStore - we'll add it
	return b.jobStore.SetAudioPath(jobID, audioPath)
}
```

- [ ] **Step 2: Add SetAudioPath method to job store**

Edit: `internal/queue/job.go` - Add this method after `GetJobsForUser`:

```go
func (s *JobStore) SetAudioPath(id int64, audioPath string) error {
	_, err := s.db.Exec(`UPDATE jobs SET audio_path = ? WHERE id = ?`, audioPath, id)
	if err != nil {
		return fmt.Errorf("setting audio path: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Verify module compiles**

```bash
go build ./internal/bot/...
```

Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add internal/bot/handlers.go internal/queue/job.go internal/queue/settings.go
git commit -m "Add command handlers and audio upload handling"
```

---

## Chunk 4: Transcription Services

### Task 10: Create Transcriber Interface

**Files:**
- Create: `internal/transcribe/transcriber.go`

- [ ] **Step 1: Define transcriber interface**

Create: `internal/transcribe/transcriber.go`

```go
package transcribe

import "context"

// Result contains the transcription output
type Result struct {
	Text     string
	Language string
	Duration float64 // audio duration in seconds
}

// Transcriber defines the interface for audio transcription
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) (*Result, error)
	Name() string
}
```

- [ ] **Step 2: Verify module compiles**

```bash
go build ./internal/transcribe/...
```

Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add internal/transcribe/transcriber.go
git commit -m "Add transcriber interface"
```

---

### Task 11: Implement Whisper.cpp Transcriber

**Files:**
- Create: `internal/transcribe/whisper.go`
- Create: `internal/transcribe/whisper_test.go`

- [ ] **Step 1: Write whisper test**

Create: `internal/transcribe/whisper_test.go`

```go
package transcribe

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func TestWhisperTranscriber_Name(t *testing.T) {
	w := NewWhisperTranscriber("/path/to/model", 4)
	if w.Name() != "whisper.cpp" {
		t.Errorf("Name() = %q, want %q", w.Name(), "whisper.cpp")
	}
}

func TestWhisperTranscriber_buildCommand(t *testing.T) {
	w := NewWhisperTranscriber("/models/ggml-small.bin", 4)

	args := w.buildArgs("/tmp/audio.wav", "/tmp/output")

	// Verify essential arguments are present
	hasModel := false
	hasThreads := false
	hasFile := false
	hasOutput := false

	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) && args[i+1] == "/models/ggml-small.bin" {
			hasModel = true
		}
		if arg == "-t" && i+1 < len(args) && args[i+1] == "4" {
			hasThreads = true
		}
		if arg == "-f" && i+1 < len(args) && args[i+1] == "/tmp/audio.wav" {
			hasFile = true
		}
		if arg == "-of" && i+1 < len(args) && args[i+1] == "/tmp/output" {
			hasOutput = true
		}
	}

	if !hasModel {
		t.Error("missing -m model argument")
	}
	if !hasThreads {
		t.Error("missing -t threads argument")
	}
	if !hasFile {
		t.Error("missing -f file argument")
	}
	if !hasOutput {
		t.Error("missing -of output argument")
	}
}

func TestWhisperTranscriber_Transcribe_MissingBinary(t *testing.T) {
	// Test with non-existent whisper binary
	w := &WhisperTranscriber{
		modelPath:  "/nonexistent/model.bin",
		threads:    4,
		binaryPath: "/nonexistent/whisper",
	}

	_, err := w.Transcribe(context.Background(), "/tmp/audio.wav")
	if err == nil {
		t.Error("expected error for missing binary, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/transcribe/... -v
```

Expected: FAIL - types not defined

- [ ] **Step 3: Implement whisper transcriber**

Create: `internal/transcribe/whisper.go`

```go
package transcribe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type WhisperTranscriber struct {
	modelPath  string
	threads    int
	binaryPath string
}

func NewWhisperTranscriber(modelPath string, threads int) *WhisperTranscriber {
	return &WhisperTranscriber{
		modelPath:  modelPath,
		threads:    threads,
		binaryPath: "whisper", // assumes whisper is in PATH
	}
}

func (w *WhisperTranscriber) Name() string {
	return "whisper.cpp"
}

func (w *WhisperTranscriber) SetBinaryPath(path string) {
	w.binaryPath = path
}

func (w *WhisperTranscriber) buildArgs(audioPath, outputBase string) []string {
	return []string{
		"-m", w.modelPath,
		"-t", strconv.Itoa(w.threads),
		"-l", "es",        // Spanish language
		"-otxt",           // Output as text
		"-of", outputBase, // Output file base name
		"-f", audioPath,   // Input file
	}
}

func (w *WhisperTranscriber) Transcribe(ctx context.Context, audioPath string) (*Result, error) {
	// Check if binary exists
	if _, err := exec.LookPath(w.binaryPath); err != nil {
		return nil, fmt.Errorf("whisper binary not found: %w", err)
	}

	// Check if model exists
	if _, err := os.Stat(w.modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper model not found: %s", w.modelPath)
	}

	// Create output path (whisper adds .txt extension)
	outputDir := filepath.Dir(audioPath)
	outputBase := filepath.Join(outputDir, "transcript")

	args := w.buildArgs(audioPath, outputBase)
	cmd := exec.CommandContext(ctx, w.binaryPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("whisper failed: %w\noutput: %s", err, string(output))
	}

	// Read the output file
	outputPath := outputBase + ".txt"
	text, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("reading transcription: %w", err)
	}

	// Clean up temporary output file
	os.Remove(outputPath)

	return &Result{
		Text:     strings.TrimSpace(string(text)),
		Language: "es",
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/transcribe/... -v
```

Expected: PASS (the MissingBinary test should pass since it expects an error)

- [ ] **Step 5: Commit**

```bash
git add internal/transcribe/whisper.go internal/transcribe/whisper_test.go
git commit -m "Add whisper.cpp transcriber implementation"
```

---

### Task 12: Implement OpenAI Whisper Transcriber

**Files:**
- Create: `internal/transcribe/openai.go`
- Create: `internal/transcribe/openai_test.go`

- [ ] **Step 1: Write OpenAI transcriber test**

Create: `internal/transcribe/openai_test.go`

```go
package transcribe

import (
	"testing"
)

func TestOpenAITranscriber_Name(t *testing.T) {
	o := NewOpenAITranscriber("test-key", "whisper-1")
	if o.Name() != "openai-whisper" {
		t.Errorf("Name() = %q, want %q", o.Name(), "openai-whisper")
	}
}

func TestOpenAITranscriber_InvalidAPIKey(t *testing.T) {
	// This is a unit test that doesn't make actual API calls
	o := NewOpenAITranscriber("", "whisper-1")
	if o.apiKey != "" {
		t.Errorf("apiKey = %q, want empty", o.apiKey)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/transcribe/... -v -run TestOpenAI
```

Expected: FAIL - types not defined

- [ ] **Step 3: Implement OpenAI transcriber**

Create: `internal/transcribe/openai.go`

```go
package transcribe

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

type OpenAITranscriber struct {
	client *openai.Client
	apiKey string
	model  string
}

func NewOpenAITranscriber(apiKey, model string) *OpenAITranscriber {
	client := openai.NewClient(apiKey)
	return &OpenAITranscriber{
		client: client,
		apiKey: apiKey,
		model:  model,
	}
}

func (o *OpenAITranscriber) Name() string {
	return "openai-whisper"
}

func (o *OpenAITranscriber) Transcribe(ctx context.Context, audioPath string) (*Result, error) {
	if o.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Check file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("audio file not found: %s", audioPath)
	}

	req := openai.AudioRequest{
		Model:    o.model,
		FilePath: audioPath,
		Language: "es", // Spanish
	}

	resp, err := o.client.CreateTranscription(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI transcription failed: %w", err)
	}

	return &Result{
		Text:     resp.Text,
		Language: "es",
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/transcribe/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/transcribe/openai.go internal/transcribe/openai_test.go
git commit -m "Add OpenAI Whisper transcriber implementation"
```

---

## Chunk 5: Summary Generation

### Task 13: Implement Summary Generator

**Files:**
- Create: `internal/summary/summary.go`
- Create: `internal/summary/summary_test.go`

- [ ] **Step 1: Write summary test**

Create: `internal/summary/summary_test.go`

```go
package summary

import (
	"testing"
)

func TestGenerator_Name(t *testing.T) {
	g := NewGenerator("test-key", "gpt-4o-mini")
	if g.model != "gpt-4o-mini" {
		t.Errorf("model = %q, want %q", g.model, "gpt-4o-mini")
	}
}

func TestGenerator_BuildPrompt(t *testing.T) {
	g := NewGenerator("test-key", "gpt-4o-mini")

	customPrompt := "Summarize this:"
	transcript := "This is the transcript text."

	result := g.buildPrompt(customPrompt, transcript)

	if result == "" {
		t.Error("buildPrompt returned empty string")
	}

	// Should contain both prompt and transcript
	if len(result) < len(customPrompt)+len(transcript) {
		t.Error("buildPrompt result too short")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/summary/... -v
```

Expected: FAIL - types not defined

- [ ] **Step 3: Implement summary generator**

Create: `internal/summary/summary.go`

```go
package summary

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type Generator struct {
	client *openai.Client
	apiKey string
	model  string
}

func NewGenerator(apiKey, model string) *Generator {
	client := openai.NewClient(apiKey)
	return &Generator{
		client: client,
		apiKey: apiKey,
		model:  model,
	}
}

func (g *Generator) buildPrompt(customPrompt, transcript string) string {
	return fmt.Sprintf("%s\n\n---\n\nTranscripción:\n%s", customPrompt, transcript)
}

func (g *Generator) Generate(ctx context.Context, transcript, prompt string) (string, error) {
	if g.apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	fullPrompt := g.buildPrompt(prompt, transcript)

	req := openai.ChatCompletionRequest{
		Model: g.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fullPrompt,
			},
		},
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	resp, err := g.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summary generation failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	return resp.Choices[0].Message.Content, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/summary/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/summary/summary.go internal/summary/summary_test.go
git commit -m "Add GPT-4o-mini summary generator"
```

---

## Chunk 6: Worker Process

### Task 14: Implement Job Worker

**Files:**
- Create: `internal/worker/worker.go`
- Create: `internal/worker/worker_test.go`

- [ ] **Step 1: Write worker test**

Create: `internal/worker/worker_test.go`

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/worker/... -v
```

Expected: FAIL - types not defined

- [ ] **Step 3: Implement worker**

Create: `internal/worker/worker.go`

```go
package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/override/pan-transcribe/internal/queue"
	"github.com/override/pan-transcribe/internal/summary"
	"github.com/override/pan-transcribe/internal/transcribe"
)

type Config struct {
	DataDir       string
	DefaultPrompt string
}

type ResultNotifier interface {
	SendResult(chatID int64, outputPath, summaryPath string) error
	SendError(chatID int64, errorMsg string)
	SendProgress(chatID int64, percent int, estimatedMinutes int)
}

type Worker struct {
	config         Config
	jobStore       *queue.JobStore
	settingsStore  *queue.SettingsStore
	localTranscriber  transcribe.Transcriber
	cloudTranscriber  transcribe.Transcriber
	summaryGenerator  *summary.Generator
	notifier       ResultNotifier
	stopCh         chan struct{}
}

func New(config Config) *Worker {
	return &Worker{
		config: config,
		stopCh: make(chan struct{}),
	}
}

func (w *Worker) SetJobStore(store *queue.JobStore) {
	w.jobStore = store
}

func (w *Worker) SetSettingsStore(store *queue.SettingsStore) {
	w.settingsStore = store
}

func (w *Worker) SetLocalTranscriber(t transcribe.Transcriber) {
	w.localTranscriber = t
}

func (w *Worker) SetCloudTranscriber(t transcribe.Transcriber) {
	w.cloudTranscriber = t
}

func (w *Worker) SetSummaryGenerator(g *summary.Generator) {
	w.summaryGenerator = g
}

func (w *Worker) SetNotifier(n ResultNotifier) {
	w.notifier = n
}

func (w *Worker) Start(ctx context.Context) {
	log.Println("Worker started")

	// Reset any jobs that were processing when we last shut down
	if err := w.jobStore.ResetProcessingJobs(); err != nil {
		log.Printf("Error resetting processing jobs: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker stopping due to context cancellation")
			return
		case <-w.stopCh:
			log.Println("Worker stopping")
			return
		case <-ticker.C:
			w.processNextJob(ctx)
		}
	}
}

func (w *Worker) Stop() {
	close(w.stopCh)
}

func (w *Worker) processNextJob(ctx context.Context) {
	job, err := w.jobStore.GetNextPending()
	if err != nil {
		log.Printf("Error getting next job: %v", err)
		return
	}

	if job == nil {
		return // No pending jobs
	}

	log.Printf("Processing job #%d", job.ID)

	if err := w.jobStore.UpdateStatus(job.ID, queue.StatusProcessing); err != nil {
		log.Printf("Error updating job status: %v", err)
		return
	}

	outputPath, summaryPath, err := w.processJob(ctx, job)
	if err != nil {
		log.Printf("Job #%d failed: %v", job.ID, err)
		w.jobStore.Fail(job.ID, err.Error())
		w.notifier.SendError(job.ChatID, err.Error())

		// Clean up audio file for failed jobs after 1 hour (handled by cleanup task)
		return
	}

	if err := w.jobStore.Complete(job.ID, outputPath, summaryPath); err != nil {
		log.Printf("Error completing job: %v", err)
		return
	}

	// Clean up audio file immediately after successful processing
	os.Remove(job.AudioPath)

	// Notify user
	if err := w.notifier.SendResult(job.ChatID, outputPath, summaryPath); err != nil {
		log.Printf("Error notifying user: %v", err)
	}

	log.Printf("Job #%d completed successfully", job.ID)
}

func (w *Worker) processJob(ctx context.Context, job *queue.Job) (outputPath, summaryPath string, err error) {
	// Select transcriber based on mode
	var t transcribe.Transcriber
	if job.Mode == "cloud" {
		t = w.cloudTranscriber
	} else {
		t = w.localTranscriber
	}

	if t == nil {
		return "", "", fmt.Errorf("transcriber not configured for mode: %s", job.Mode)
	}

	// Transcribe
	log.Printf("Transcribing with %s: %s", t.Name(), job.AudioPath)
	result, err := t.Transcribe(ctx, job.AudioPath)
	if err != nil {
		// If local transcriber fails, we could retry with cloud
		// For now, just return the error
		return "", "", fmt.Errorf("transcription failed: %w", err)
	}

	// Save transcription
	outputDir := filepath.Join(w.config.DataDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("creating output dir: %w", err)
	}

	outputPath = filepath.Join(outputDir, fmt.Sprintf("%d.txt", job.ID))
	if err := os.WriteFile(outputPath, []byte(result.Text), 0644); err != nil {
		return "", "", fmt.Errorf("saving transcription: %w", err)
	}

	// Generate summary if requested
	if job.WithSummary && w.summaryGenerator != nil {
		log.Printf("Generating summary for job #%d", job.ID)

		// Get custom prompt or use default
		prompt, err := w.settingsStore.GetCustomPrompt(job.ChatID)
		if err != nil {
			log.Printf("Error getting custom prompt: %v", err)
		}
		if prompt == "" {
			prompt = w.config.DefaultPrompt
		}

		summaryText, err := w.summaryGenerator.Generate(ctx, result.Text, prompt)
		if err != nil {
			log.Printf("Summary generation failed: %v", err)
			// Don't fail the job, just skip summary
		} else {
			summaryPath = filepath.Join(outputDir, fmt.Sprintf("%d_summary.txt", job.ID))
			if err := os.WriteFile(summaryPath, []byte(summaryText), 0644); err != nil {
				log.Printf("Error saving summary: %v", err)
			}
		}
	}

	return outputPath, summaryPath, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/worker/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/worker/worker.go internal/worker/worker_test.go
git commit -m "Add job processing worker"
```

---

### Task 15: Implement Cleanup Task

**Files:**
- Create: `internal/cleanup/cleanup.go`
- Create: `internal/cleanup/cleanup_test.go`

- [ ] **Step 1: Write cleanup test**

Create: `internal/cleanup/cleanup_test.go`

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cleanup/... -v
```

Expected: FAIL - types not defined

- [ ] **Step 3: Implement cleanup task**

Create: `internal/cleanup/cleanup.go`

```go
package cleanup

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
)

type Config struct {
	OutputDir     string
	AudioDir      string
	RetentionDays int
}

type Cleanup struct {
	config Config
	cron   *cron.Cron
}

func New(config Config) *Cleanup {
	return &Cleanup{
		config: config,
	}
}

func (c *Cleanup) Start() error {
	c.cron = cron.New()

	// Run cleanup daily at 3 AM
	_, err := c.cron.AddFunc("0 3 * * *", func() {
		removed, err := c.CleanOldFiles()
		if err != nil {
			log.Printf("Cleanup error: %v", err)
		} else if removed > 0 {
			log.Printf("Cleanup removed %d old files", removed)
		}
	})
	if err != nil {
		return err
	}

	c.cron.Start()
	log.Println("Cleanup scheduler started")
	return nil
}

func (c *Cleanup) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

func (c *Cleanup) CleanOldFiles() (int, error) {
	cutoff := time.Now().Add(-time.Duration(c.config.RetentionDays) * 24 * time.Hour)
	removed := 0

	// Clean output directory
	if c.config.OutputDir != "" {
		n, err := c.cleanDirectory(c.config.OutputDir, cutoff)
		if err != nil {
			return removed, err
		}
		removed += n
	}

	// Clean audio directory (for failed jobs)
	if c.config.AudioDir != "" {
		// Audio files for failed jobs older than 1 hour
		audioCutoff := time.Now().Add(-1 * time.Hour)
		n, err := c.cleanDirectory(c.config.AudioDir, audioCutoff)
		if err != nil {
			return removed, err
		}
		removed += n
	}

	return removed, nil
}

func (c *Cleanup) cleanDirectory(dir string, cutoff time.Time) (int, error) {
	removed := 0

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("Failed to remove %s: %v", path, err)
			} else {
				removed++
			}
		}
	}

	return removed, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/cleanup/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup/cleanup.go internal/cleanup/cleanup_test.go
git commit -m "Add scheduled file cleanup"
```

---

## Chunk 7: Main Entry Point & Integration

### Task 16: Wire Everything in main.go

**Files:**
- Modify: `cmd/bot/main.go`

- [ ] **Step 1: Update main.go with full implementation**

Replace contents of: `cmd/bot/main.go`

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/override/pan-transcribe/internal/bot"
	"github.com/override/pan-transcribe/internal/cleanup"
	"github.com/override/pan-transcribe/internal/config"
	"github.com/override/pan-transcribe/internal/queue"
	"github.com/override/pan-transcribe/internal/summary"
	"github.com/override/pan-transcribe/internal/transcribe"
	"github.com/override/pan-transcribe/internal/worker"
)

func main() {
	// Parse config path from args
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Determine data directory
	dataDir := "data"
	if envDir := os.Getenv("DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	// Initialize database
	dbPath := filepath.Join(dataDir, "jobs.db")
	db, err := queue.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	jobStore := queue.NewJobStore(db)
	settingsStore := queue.NewSettingsStore(db)

	// Initialize bot
	tgBot, err := bot.New(cfg, jobStore, settingsStore, dataDir)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Initialize transcribers
	localTranscriber := transcribe.NewWhisperTranscriber(cfg.Whisper.ModelPath, cfg.Whisper.Threads)
	cloudTranscriber := transcribe.NewOpenAITranscriber(cfg.OpenAI.APIKey, cfg.OpenAI.WhisperModel)

	// Initialize summary generator
	summaryGen := summary.NewGenerator(cfg.OpenAI.APIKey, cfg.OpenAI.SummaryModel)

	// Initialize worker
	w := worker.New(worker.Config{
		DataDir:       dataDir,
		DefaultPrompt: cfg.Summary.DefaultPrompt,
	})
	w.SetJobStore(jobStore)
	w.SetSettingsStore(settingsStore)
	w.SetLocalTranscriber(localTranscriber)
	w.SetCloudTranscriber(cloudTranscriber)
	w.SetSummaryGenerator(summaryGen)
	w.SetNotifier(tgBot)

	// Initialize cleanup
	cleaner := cleanup.New(cleanup.Config{
		OutputDir:     filepath.Join(dataDir, "output"),
		AudioDir:      filepath.Join(dataDir, "audio"),
		RetentionDays: cfg.Processing.OutputRetentionDays,
	})

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutdown signal received")
		cancel()
		w.Stop()
		cleaner.Stop()
	}()

	// Start cleanup scheduler
	if err := cleaner.Start(); err != nil {
		log.Printf("Warning: cleanup scheduler failed to start: %v", err)
	}

	// Start worker in background
	go w.Start(ctx)

	// Start bot (blocking)
	log.Println("Starting bot...")
	if err := tgBot.Start(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
```

- [ ] **Step 2: Verify everything compiles**

```bash
go build ./cmd/bot/...
```

Expected: Build succeeds

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v
```

Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/bot/main.go
git commit -m "Wire up all components in main"
```

---

### Task 17: Create README with Setup Instructions

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create README**

Create: `README.md`

```markdown
# PaN Transcribe

A Telegram bot for transcribing psychology class recordings using Whisper (local or cloud) with optional GPT-4o-mini summaries.

## Features

- Transcribe audio files (mp3, wav, m4a, ogg, flac)
- Local transcription using whisper.cpp (default)
- Cloud transcription using OpenAI Whisper API (faster)
- Psychology-focused class summaries using GPT-4o-mini
- Customizable summary prompts
- Job queue with persistence
- Runs on Raspberry Pi 5 (4GB RAM)

## Prerequisites

### System Dependencies

```bash
# On Raspberry Pi / Debian / Ubuntu
sudo apt update
sudo apt install -y ffmpeg sqlite3 build-essential

# Install whisper.cpp
git clone https://github.com/ggerganov/whisper.cpp.git
cd whisper.cpp
make

# Download Spanish model
./models/download-ggml-model.sh small

# Move to project location
mv whisper.cpp ../pan-transcribe/
```

### Get Your Telegram Bot Token

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot` and follow the prompts
3. Copy the bot token

### Get Your Telegram User ID

1. Message [@userinfobot](https://t.me/userinfobot) on Telegram
2. It will reply with your user ID

### Get OpenAI API Key

1. Go to [OpenAI API Keys](https://platform.openai.com/api-keys)
2. Create a new API key

## Installation

```bash
# Clone the repository
git clone https://github.com/override/pan-transcribe.git
cd pan-transcribe

# Build for your platform
make build

# Or cross-compile for Raspberry Pi
make build-pi

# Create config from example
cp config.yaml.example config.yaml

# Edit config with your values
nano config.yaml

# Create data directories
make init-dirs
```

## Configuration

Edit `config.yaml`:

```yaml
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  allowed_users:
    - YOUR_USER_ID  # Replace with your Telegram user ID

openai:
  api_key: "${OPENAI_API_KEY}"
  whisper_model: "whisper-1"
  summary_model: "gpt-4o-mini"

whisper:
  model_path: "./whisper.cpp/models/ggml-small.bin"
  threads: 4

processing:
  default_mode: "local"
  max_file_size_mb: 100
  output_retention_days: 30

summary:
  default_prompt: |
    Eres un asistente para estudiantes de psicología...
```

Set environment variables:

```bash
export TELEGRAM_BOT_TOKEN="your_token"
export OPENAI_API_KEY="your_key"
```

## Running

```bash
# Run directly
./bin/bot config.yaml

# Or use make
make run
```

## Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Show help |
| `/transcribe` | Transcribe only mode |
| `/summarize` | Transcribe + summary mode |
| `/status` | Check job status |
| `/setprompt <text>` | Set custom summary prompt |
| `/showprompt` | Show current prompt |
| `/cloud` | Use OpenAI API for next job |
| `/local` | Use local Whisper for next job |

## Systemd Service (Raspberry Pi)

Create `/etc/systemd/system/pan-transcribe.service`:

```ini
[Unit]
Description=PaN Transcribe Bot
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/pan-transcribe
ExecStart=/home/pi/pan-transcribe/bin/bot /home/pi/pan-transcribe/config.yaml
Restart=always
RestartSec=10
Environment=TELEGRAM_BOT_TOKEN=your_token
Environment=OPENAI_API_KEY=your_key

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable pan-transcribe
sudo systemctl start pan-transcribe
sudo systemctl status pan-transcribe
```

## License

MIT
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Add README with setup instructions"
```

---

### Task 18: Final Integration Test

- [ ] **Step 1: Run full test suite**

```bash
make test
```

Expected: All tests PASS

- [ ] **Step 2: Build for all platforms**

```bash
make build
make build-pi
```

Expected: Both builds succeed

- [ ] **Step 3: Verify binary runs**

```bash
./bin/bot --help 2>&1 || ./bin/bot config.yaml.example 2>&1 | head -5
```

Expected: Shows config loading attempt (will fail without real config, which is expected)

- [ ] **Step 4: Final commit**

```bash
git add -A
git status
# If any untracked files, add them
git commit -m "Complete Telegram transcription bot implementation" --allow-empty
```

---

## Summary

This plan implements a complete Telegram transcription bot with:

1. **Configuration** (Task 1-3): Viper-based config, env var expansion
2. **Job Queue** (Task 4-6): SQLite persistence, job/settings stores
3. **Telegram Bot** (Task 7-9): Auth, commands, file upload handling
4. **Transcription** (Task 10-12): Interface, whisper.cpp, OpenAI implementations
5. **Summary** (Task 13): GPT-4o-mini integration
6. **Worker** (Task 14-15): Background processing, cleanup scheduling
7. **Integration** (Task 16-18): Main wiring, README, final tests

Total: 18 tasks, ~70 steps, estimated implementation time varies by developer.
