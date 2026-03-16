# PaN Transcribe - Raspberry Pi Setup Checklist

## 1. Get API Keys & Tokens (do this from any computer)

- [ ] **Telegram Bot Token**
  1. Open Telegram, message [@BotFather](https://t.me/botfather)
  2. Send `/newbot`
  3. Choose a name (e.g., "PaN Transcribe")
  4. Choose a username (must end in `bot`, e.g., `pan_transcribe_bot`)
  5. Save the token: `xxxxxxxxxx:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

- [ ] **Your Telegram User ID**
  1. Message [@userinfobot](https://t.me/userinfobot) on Telegram
  2. It replies with your user ID (a number like `123456789`)
  3. Save this - it restricts who can use your bot

- [ ] **OpenAI API Key**
  1. Go to https://platform.openai.com/api-keys
  2. Create new secret key
  3. Save it: `sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

---

## 2. Raspberry Pi System Setup

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install dependencies
sudo apt install -y ffmpeg sqlite3 build-essential git

# Verify SQLite is installed (no server needed - it's just a file!)
sqlite3 --version
```

**Note:** SQLite doesn't need any server setup. The database is just a file (`data/jobs.db`) that gets created automatically when the bot starts.

---

## 3. Install whisper.cpp

```bash
# Clone and build whisper.cpp
cd ~
git clone https://github.com/ggerganov/whisper.cpp.git
cd whisper.cpp

# Build (uses ARM optimizations automatically on Pi)
make

# Download Spanish model (~466MB)
./models/download-ggml-model.sh small

# Verify it works
./main -m models/ggml-small.bin -l es -f samples/jfk.wav

# Add to PATH (or note the full path for config)
echo 'export PATH="$HOME/whisper.cpp:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

---

## 4. Deploy the Bot

```bash
# Create project directory
mkdir -p ~/pan-transcribe
cd ~/pan-transcribe

# Copy the ARM64 binary from your dev machine
# From your dev machine run:
# scp bin/bot-arm64 pi@raspberrypi:~/pan-transcribe/bot

# Or build on Pi directly:
# git clone <your-repo> .
# go build -o bot ./cmd/bot

# Create directories
mkdir -p data/audio data/output logs

# Create config file
cat > config.yaml << 'EOF'
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  allowed_users:
    - YOUR_USER_ID_HERE

openai:
  api_key: "${OPENAI_API_KEY}"
  whisper_model: "whisper-1"
  summary_model: "gpt-4o-mini"

whisper:
  model_path: "/home/pi/whisper.cpp/models/ggml-small.bin"
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
EOF

# Edit config.yaml and replace YOUR_USER_ID_HERE with your actual user ID
nano config.yaml
```

---

## 5. Set Up Environment Variables

```bash
# Create env file
cat > ~/.pan-transcribe-env << 'EOF'
export TELEGRAM_BOT_TOKEN="your-bot-token-here"
export OPENAI_API_KEY="sk-your-openai-key-here"
EOF

# Edit with your actual keys
nano ~/.pan-transcribe-env

# Secure it
chmod 600 ~/.pan-transcribe-env
```

---

## 6. Test Run

```bash
# Load environment
source ~/.pan-transcribe-env

# Run bot (Ctrl+C to stop)
cd ~/pan-transcribe
./bot config.yaml

# Test it:
# 1. Open Telegram
# 2. Find your bot by username
# 3. Send /start
# 4. Send an audio file
```

---

## 7. Set Up Systemd Service (auto-start on boot)

```bash
sudo tee /etc/systemd/system/pan-transcribe.service << 'EOF'
[Unit]
Description=PaN Transcribe Telegram Bot
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/pan-transcribe
ExecStart=/home/pi/pan-transcribe/bot /home/pi/pan-transcribe/config.yaml
Restart=always
RestartSec=10
EnvironmentFile=/home/pi/.pan-transcribe-env

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable pan-transcribe
sudo systemctl start pan-transcribe

# Check status
sudo systemctl status pan-transcribe

# View logs
journalctl -u pan-transcribe -f
```

---

## Quick Reference

| Item | Where to get it |
|------|-----------------|
| Bot token | @BotFather on Telegram |
| User ID | @userinfobot on Telegram |
| OpenAI key | platform.openai.com/api-keys |

| Command | What it does |
|---------|--------------|
| `sudo systemctl start pan-transcribe` | Start bot |
| `sudo systemctl stop pan-transcribe` | Stop bot |
| `sudo systemctl restart pan-transcribe` | Restart bot |
| `journalctl -u pan-transcribe -f` | View live logs |
| `sqlite3 data/jobs.db "SELECT * FROM jobs;"` | Check job queue |

---

## Troubleshooting

**Bot not responding?**
```bash
# Check if running
sudo systemctl status pan-transcribe

# Check logs
journalctl -u pan-transcribe --since "10 minutes ago"
```

**Transcription failing?**
```bash
# Test whisper directly
~/whisper.cpp/main -m ~/whisper.cpp/models/ggml-small.bin -l es -f test.wav
```

**Database issues?**
```bash
# Check database
sqlite3 ~/pan-transcribe/data/jobs.db ".tables"
sqlite3 ~/pan-transcribe/data/jobs.db "SELECT id, status, created_at FROM jobs ORDER BY id DESC LIMIT 5;"
```
