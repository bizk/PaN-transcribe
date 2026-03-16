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
			envVal, exists := os.LookupEnv(envVar)
			if !exists {
				return nil, fmt.Errorf("environment variable %s not set (referenced in config key %s)", envVar, key)
			}
			v.Set(key, envVal)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}
