# Telegram Transcription Bot for Psychology Students

**Date:** 2026-03-15
**Status:** Approved

## Overview

A Telegram bot written in Go that receives audio files of psychology class recordings, transcribes them using Whisper (locally via whisper.cpp or via OpenAI API), and optionally generates summaries tailored for psychology students using GPT-4o-mini.

## Requirements

### Functional Requirements

- Receive audio files via Telegram (mp3, wav, m4a, ogg, flac)
- Transcribe audio to text using whisper.cpp (local, default) or OpenAI Whisper API (cloud fallback)
- Return transcription as a .txt file
- Optionally generate psychology-focused class summaries using GPT-4o-mini
- Allow customization of the summary prompt
- Support batch processing (no real-time requirements)
- Single user (personal tool)

### Non-Functional Requirements

- Run on Raspberry Pi 5 with 4GB RAM
- Spanish language support
- Persist job queue across reboots
- Retain transcriptions for 1 month

## Architecture

```
┌─────────────────┐     ┌──────────────────────────────────────────────┐
│  Telegram User  │     │            Raspberry Pi 5                    │
│                 │     │                                              │
│  Send audio ────┼────▶│  ┌────────────────┐    ┌─────────────────┐  │
│                 │     │  │  Go Telegram   │───▶│  Job Queue      │  │
│  Receive txt ◀──┼─────│  │  Bot           │    │  (SQLite)       │  │
│  + summary      │     │  └────────────────┘    └────────┬────────┘  │
└─────────────────┘     │                                 │           │
                        │         ┌───────────────────────┘           │
                        │         ▼                                    │
                        │  ┌─────────────────┐                        │
                        │  │  Worker Process │                        │
                        │  │                 │                        │
                        │  │  ┌───────────┐  │    ┌────────────────┐  │
                        │  │  │Whisper.cpp│◀─┼───▶│ OpenAI Whisper │  │
                        │  │  │ (local)   │  │    │ API (fallback) │  │
                        │  │  └───────────┘  │    └────────────────┘  │
                        │  │        │        │                        │
                        │  │        ▼        │                        │
                        │  │  ┌───────────┐  │    ┌────────────────┐  │
                        │  │  │ OpenAI    │◀─┼───▶│ GPT-4o-mini    │  │
                        │  │  │ Summary   │  │    │ (cloud)        │  │
                        │  │  └───────────┘  │    └────────────────┘  │
                        │  └─────────────────┘                        │
                        └──────────────────────────────────────────────┘
```

### Components

| Component | Responsibility |
|-----------|----------------|
| Go Telegram Bot | Receives audio files, queues jobs, sends results |
| SQLite Job Queue | Persists transcription jobs (survives reboots) |
| Worker Process | Picks jobs from queue, runs transcription, generates summaries |
| Whisper.cpp | Local transcription (default) |
| OpenAI Whisper API | Cloud transcription (fallback/on-demand) |
| OpenAI GPT-4o-mini | Generates psychology class summaries |

## Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Welcome message with usage instructions |
| `/transcribe` | Default mode - transcribe only, return .txt file |
| `/summarize` | Transcribe + generate psychology class summary |
| `/status` | Check status of pending jobs |
| `/setprompt <text>` | Customize the summary prompt |
| `/showprompt` | Display current summary prompt |
| `/cloud` | Force next transcription to use OpenAI API (faster) |
| `/local` | Force next transcription to use Whisper.cpp (default) |

## User Interaction Flow

1. User sends an audio file (mp3, wav, m4a, ogg, flac)
2. Bot replies: "Queued for transcription (position #X). Estimated time: ~Y minutes. Use /status to check progress."
3. When complete, bot sends:
   - The transcription as a `.txt` file
   - If `/summarize` was used: a summary message + optional `.txt` file

## Default Summary Prompt

```
Eres un asistente para estudiantes de psicología. Resume esta transcripción
de clase incluyendo:
- Temas principales cubiertos
- Conceptos clave y definiciones
- Teorías o autores mencionados
- Puntos importantes para estudiar
Responde en español.
```

## Data Flow & Storage

### Directory Structure

```
/home/pi/pan-transcribe/
├── data/
│   ├── jobs.db              # SQLite database (job queue + settings)
│   ├── audio/               # Temporary audio files (deleted after processing)
│   │   └── {job_id}.wav
│   └── output/              # Transcriptions (kept for 1 month)
│       ├── {job_id}.txt
│       └── {job_id}_summary.txt
├── whisper.cpp/             # Whisper.cpp installation
│   └── models/
│       └── ggml-small.bin   # Spanish-capable model (~466MB)
└── bot                      # Go binary
```

### Job States

```
pending → processing → completed
                    → failed (with error message)
```

### File Handling

| Type | Retention |
|------|-----------|
| Audio files | Deleted immediately after transcription |
| Transcription/summary files | Kept for 1 month, then auto-deleted |
| Failed job audio | Deleted after 1 hour |

### Limits

- Max audio file size: 100MB
- Supported formats: mp3, wav, m4a, ogg, flac

## Configuration

### config.yaml

```yaml
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  allowed_users:
    - 123456789  # Your Telegram user ID

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
    Eres un asistente para estudiantes de psicología. Resume esta transcripción
    de clase incluyendo:
    - Temas principales cubiertos
    - Conceptos clave y definiciones
    - Teorías o autores mencionados
    - Puntos importantes para estudiar
    Responde en español.
```

### Environment Variables (.env)

```
TELEGRAM_BOT_TOKEN=your_bot_token_here
OPENAI_API_KEY=your_openai_key_here
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Unsupported audio format | Reply: "Formato no soportado. Usa: mp3, wav, m4a, ogg, flac" |
| File too large (>100MB) | Reply: "Archivo muy grande. Máximo: 100MB" |
| Whisper.cpp fails | Auto-retry once, then offer `/cloud` fallback |
| OpenAI API fails | Retry 3x with backoff, then report error |
| Pi runs out of disk space | Reject new jobs, alert user, auto-cleanup old files |
| Bot restarts mid-job | Job resumes from queue on startup |
| Unauthorized user | Silently ignore (no response) |

### Progress Updates

For long local transcriptions (>15 min audio): send progress update every 10 minutes.
Example: "Procesando... 45% completado. Tiempo restante estimado: ~20 min"

### Logging

- Log file: `/home/pi/pan-transcribe/logs/bot.log`
- Rotation: keep last 7 days
- Levels: errors always, info for job lifecycle, debug optional

## Technology Stack

### Go Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-telegram-bot-api/telegram-bot-api/v5` | Telegram Bot API |
| `github.com/sashabaranov/go-openai` | OpenAI API client |
| `github.com/mattn/go-sqlite3` | SQLite driver |
| `github.com/spf13/viper` | Configuration management |
| `github.com/robfig/cron/v3` | Scheduled cleanup tasks |

### System Dependencies

| Dependency | Purpose |
|------------|---------|
| `whisper.cpp` | Local speech-to-text |
| `ffmpeg` | Audio format conversion |
| `sqlite3` | Job queue database |

### Whisper Model

- Primary: `ggml-small` (~466MB) - good balance of accuracy and speed for Spanish
- Alternative: `ggml-base` (~142MB) if RAM becomes tight

## Deployment

- Cross-compile Go binary for `linux/arm64` or compile on Pi
- Run as systemd service for auto-start on boot
- Whisper.cpp compiled on Pi with ARM optimizations

## Security

- Bot restricted to configured Telegram user ID only
- API keys stored in environment variables
- No audio/transcription data leaves Pi unless `/cloud` is explicitly used
