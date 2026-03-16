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
