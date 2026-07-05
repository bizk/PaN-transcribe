# Deployment Guide

This document covers different deployment methods for PaN Transcribe.

## Table of Contents

- [Docker Deployment (Recommended)](#docker-deployment-recommended)
- [Raspberry Pi Deployment](#raspberry-pi-deployment)
- [Manual Deployment](#manual-deployment)

## Docker Deployment (Recommended)

Docker provides the easiest and most portable deployment method.

### Prerequisites

- Docker and Docker Compose installed
- Environment variables configured

### Quick Start

1. **Clone the repository and navigate to the project directory**

2. **Create configuration files**

   ```bash
   # Create environment file from example
   cp .env.example .env

   # Edit .env with your actual API keys
   nano .env

   # Create config.yaml from example
   cp config.yaml.example config.yaml

   # Edit config.yaml to add your Telegram user ID
   nano config.yaml
   ```

3. **Set up required environment variables in `.env`**

   ```env
   TELEGRAM_BOT_TOKEN=your_bot_token_here
   MISTRAL_API_KEY=your_mistral_api_key_here
   OPENAI_API_KEY=your_openai_api_key_here
   TZ=America/Argentina/Buenos_Aires
   ```

4. **Create data directories**

   ```bash
   make init-dirs
   ```

5. **Build and run with Docker Compose**

   ```bash
   # Build the Docker image
   make docker-build

   # Start the bot
   make docker-run

   # View logs
   make docker-logs
   ```

### Docker Commands

```bash
make docker-build      # Build Docker image
make docker-run        # Start bot with docker-compose
make docker-stop       # Stop the bot
make docker-logs       # View logs (follow mode)
make docker-clean      # Stop and remove containers/volumes
```

### Manual Docker Commands

If you prefer not to use make:

```bash
# Build image
docker build -t pan-transcribe:latest .

# Run with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### Data Persistence

The following directories are mounted as volumes:
- `./data` - SQLite database and processed files
- `./logs` - Application logs
- `./config.yaml` - Configuration (read-only)

These directories will persist even if the container is removed.

## Raspberry Pi Deployment

### Option 1: Docker on Raspberry Pi (Recommended)

Docker works on Raspberry Pi OS and provides the same benefits as on other platforms.

1. **Install Docker on Raspberry Pi**

   ```bash
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   sudo usermod -aG docker $USER

   # Install Docker Compose
   sudo apt-get install docker-compose-plugin

   # Log out and back in for group changes to take effect
   ```

2. **Follow the Docker Deployment steps above**

   The Dockerfile uses multi-arch support and will automatically build for ARM64.

### Option 2: Build ARM64 Docker Image on Another Machine

If you want to build the image on your development machine:

```bash
# Build for ARM64/Raspberry Pi
make docker-build-pi

# Save image to file
docker save pan-transcribe:arm64 | gzip > pan-transcribe-arm64.tar.gz

# Transfer to Raspberry Pi
scp pan-transcribe-arm64.tar.gz pi@raspberry-pi:/home/pi/

# On Raspberry Pi, load the image
docker load < pan-transcribe-arm64.tar.gz
```

### Option 3: Native Binary Deployment

For running without Docker:

```bash
# On your development machine
make build-pi

# Transfer binary and config to Raspberry Pi
scp bin/bot-arm64 config.yaml pi@raspberry-pi:/home/pi/pan-transcribe/

# On Raspberry Pi
cd /home/pi/pan-transcribe
./bot-arm64 config.yaml
```

## Manual Deployment

### Prerequisites

- Go 1.23 or later
- SQLite3 development libraries
- ffmpeg (optional, for audio format conversion)

### Installation

1. **Install dependencies**

   Ubuntu/Debian:
   ```bash
   sudo apt-get update
   sudo apt-get install -y golang sqlite3 libsqlite3-dev ffmpeg
   ```

   macOS:
   ```bash
   brew install go sqlite ffmpeg
   ```

2. **Clone and build**

   ```bash
   git clone <repository-url>
   cd PaN-transcribe

   # Download dependencies
   go mod download

   # Build
   make build
   ```

3. **Configure**

   ```bash
   cp config.yaml.example config.yaml
   # Edit config.yaml with your settings

   # Create required directories
   make init-dirs
   ```

4. **Set environment variables**

   ```bash
   export TELEGRAM_BOT_TOKEN="your_token"
   export MISTRAL_API_KEY="your_key"
   export OPENAI_API_KEY="your_key"
   ```

5. **Run**

   ```bash
   make run
   # or
   ./bin/bot config.yaml
   ```

### Running as a Service (systemd)

Create `/etc/systemd/system/pan-transcribe.service`:

```ini
[Unit]
Description=PaN Transcribe Bot
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/PaN-transcribe
Environment="TELEGRAM_BOT_TOKEN=your_token"
Environment="MISTRAL_API_KEY=your_key"
Environment="OPENAI_API_KEY=your_key"
ExecStart=/path/to/PaN-transcribe/bin/bot /path/to/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable pan-transcribe
sudo systemctl start pan-transcribe
sudo systemctl status pan-transcribe
```

## Troubleshooting

### Docker Issues

**Container exits immediately:**
```bash
# Check logs
docker-compose logs

# Common issues:
# - Missing environment variables
# - Invalid API keys
# - Permission issues with mounted volumes
```

**Permission denied errors:**
```bash
# Fix data directory permissions
sudo chown -R 1000:1000 data/ logs/
```

### Database Issues

**Database locked errors:**
```bash
# Stop all instances
make docker-stop  # or kill the process

# Check for lock files
rm -f data/jobs.db-*
```

### Network Issues

**Bot can't connect to Telegram:**
- Check firewall settings
- Verify internet connectivity
- Ensure TELEGRAM_BOT_TOKEN is correct

## Resource Usage

Expected resource usage on Raspberry Pi 5 (4GB RAM):
- **Idle**: ~50MB RAM
- **Processing**: 200-500MB RAM (depends on file size)
- **Storage**: Varies by retention policy (default: 30 days)

Adjust Docker resource limits in `docker-compose.yml` if needed.
