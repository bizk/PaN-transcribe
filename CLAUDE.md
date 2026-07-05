# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PaN Transcribe is a Go Telegram bot that transcribes audio recordings using Mistral's audio API, with optional GPT-4o-mini summaries. Designed to run on Raspberry Pi 5 (4GB RAM).

## Commands

```bash
make build          # Build for current platform → bin/bot
make build-pi       # Cross-compile for ARM64 → bin/bot-arm64
make test           # Run all tests (go test ./... -v)
make run            # Build and run with config.yaml
make init-dirs      # Create data/audio, data/output, logs directories
make clean          # Remove bin/ and data/* contents
```

Run a single test:
```bash
go test ./internal/queue -v -run TestJobStatusTransitions
```

Run the bot:
```bash
./bin/bot config.yaml
```

## Architecture

The system uses a background job queue pattern with SQLite persistence:

```
Telegram → Bot → JobStore (SQLite) → Worker → Transcriber + Summary → User
                      ↓
                 Cleanup (cron)
```

### Core Components (`internal/`)

| Package | Purpose |
|---------|---------|
| `bot/` | Telegram message handling, user auth via allowed_users whitelist |
| `queue/` | SQLite-backed job persistence with `jobs` and `settings` tables |
| `worker/` | Background processor: polls queue, orchestrates transcription + summary |
| `transcribe/` | `Transcriber` interface with `MistralTranscriber` implementation |
| `summary/` | GPT-4o-mini summary generation with customizable prompts |
| `cleanup/` | Cron-scheduled retention policy for output files |
| `config/` | YAML config loader with `${ENV_VAR}` expansion |
| `logger/` | Structured logging with levels (DEBUG, INFO, WARN, ERROR) and contextual fields |

### Entry Point

`cmd/bot/main.go` initializes all components and starts:
1. Cleanup scheduler (background cron)
2. Worker (background polling every 5s)
3. Bot (blocking - listens for Telegram updates)

Graceful shutdown via SIGINT/SIGTERM stops all components.

### Database Schema

SQLite database at `data/jobs.db`:
- `jobs`: id, chat_id, message_id, audio_path, output_path, summary_path, status, mode, with_summary, error_message, timestamps
- `settings`: user_id, custom_prompt, next_with_summary

Job statuses: `pending` → `processing` → `completed`/`failed`

## Configuration

Config file: `config.yaml` (copy from `config.yaml.example`)

Required environment variables:
- `TELEGRAM_BOT_TOKEN` - from @BotFather
- `MISTRAL_API_KEY` - for audio transcription
- `OPENAI_API_KEY` - for summaries

Key settings:
- `telegram.allowed_users` - list of authorized Telegram user IDs
- `mistral.model` - Mistral audio model (default: mistral-large-latest)

## Logging

The application uses a structured logging system (`internal/logger`):

- **Log Levels**: DEBUG, INFO, WARN, ERROR
- **Contextual Fields**: Use `WithField()` or `WithFields()` to add context (user_id, job_id, chat_id, etc.)
- **Output**: Logs to both stdout and file (`logs/bot_YYYY-MM-DD.log`)
- **Debug Mode**: Set `DEBUG=true` environment variable to enable DEBUG level logs

Example usage:
```go
log := logger.New("component-name")
log.Info("Processing started")
log.WithField("user_id", 123).Warn("Potential issue")
log.WithFields(map[string]interface{}{
    "job_id": 42,
    "chat_id": 12345,
}).Error("Processing failed: %v", err)
```

## External Dependencies

- `ffmpeg` - audio format conversion (optional, for format conversion)
