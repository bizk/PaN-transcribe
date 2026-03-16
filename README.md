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
