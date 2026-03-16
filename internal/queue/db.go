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
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
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
	if err != nil {
		return fmt.Errorf("executing schema: %w", err)
	}
	return nil
}
