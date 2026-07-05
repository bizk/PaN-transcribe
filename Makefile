.PHONY: build test run clean build-pi init-dirs docker-build docker-run docker-stop docker-logs docker-clean

# Build for current platform
build:
	go build -o bin/bot ./cmd/bot

# Build for Raspberry Pi 5 (ARM64)
build-pi:
	GOOS=linux GOARCH=arm64 go build -o bin/bot-arm64 ./cmd/bot

# Run tests
test:
	go test ./... -v

# Run locally
run: build
	./bin/bot config.yaml

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf data/audio/*
	rm -rf data/output/*

# Create data directories
init-dirs:
	mkdir -p data/audio data/output logs

# Docker commands
docker-build:
	docker build -t pan-transcribe:latest .

docker-build-pi:
	docker buildx build --platform linux/arm64 -t pan-transcribe:arm64 .

docker-run: init-dirs
	docker-compose up -d

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-clean:
	docker-compose down -v
	docker rmi pan-transcribe:latest 2>/dev/null || true
