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
