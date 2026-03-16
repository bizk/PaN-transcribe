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
