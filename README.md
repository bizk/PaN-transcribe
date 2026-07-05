# PaN Transcribe

A Telegram bot for transcribing psychology class recordings using Mistral's audio API with optional GPT-4o-mini summaries.

## Features

- Transcribe audio files (mp3, wav, m4a, ogg, flac)
- Cloud transcription using Mistral audio API
- Psychology-focused class summaries using GPT-4o-mini
- Customizable summary prompts
- Job queue with persistence
- Runs on Raspberry Pi 5 (4GB RAM)

## Prerequisites

### System Dependencies

```bash
# On Raspberry Pi / Debian / Ubuntu
sudo apt update
sudo apt install -y ffmpeg sqlite3
```

### Get Your Telegram Bot Token

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot` and follow the prompts
3. Copy the bot token

### Get Your Telegram User ID

1. Message [@userinfobot](https://t.me/userinfobot) on Telegram
2. It will reply with your user ID

### Get API Keys

1. **Mistral API Key**: Go to [Mistral AI Console](https://console.mistral.ai/api-keys/) and create a new API key
2. **OpenAI API Key**: Go to [OpenAI API Keys](https://platform.openai.com/api-keys) and create a new API key

## Quick Start (Docker - Recommended)

The easiest way to run PaN Transcribe is with Docker:

```bash
# Clone the repository
git clone https://github.com/override/pan-transcribe.git
cd pan-transcribe

# Set up configuration
cp .env.example .env
cp config.yaml.example config.yaml

# Edit .env and config.yaml with your values
nano .env
nano config.yaml

# Create directories and start
make init-dirs
make docker-build
make docker-run

# View logs
make docker-logs
```

See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed deployment options including Raspberry Pi, systemd, and manual installation.

## Installation (Manual)

For manual installation without Docker:

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

mistral:
  api_key: "${MISTRAL_API_KEY}"
  model: "mistral-large-latest"

openai:
  api_key: "${OPENAI_API_KEY}"
  summary_model: "gpt-4o-mini"

processing:
  max_file_size_mb: 100
  output_retention_days: 30

summary:
  default_prompt: |
    Eres un asistente para estudiantes de psicología...
```

Set environment variables:

```bash
export TELEGRAM_BOT_TOKEN="your_token"
export MISTRAL_API_KEY="your_mistral_key"
export OPENAI_API_KEY="your_openai_key"
```

## Running

### Docker

```bash
# Start the bot
make docker-run

# View logs
make docker-logs

# Stop the bot
make docker-stop
```

### Manual

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

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for comprehensive deployment guides including:
- Docker deployment (recommended)
- Raspberry Pi deployment (Docker or native)
- Systemd service setup
- Troubleshooting

## License

MIT
