.PHONY: build test run clean build-pi

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
