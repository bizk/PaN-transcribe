# Multi-stage build for PaN Transcribe bot
# Stage 1: Builder
FROM golang:1.23-alpine AS builder

# Install build dependencies (CGO required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bot ./cmd/bot

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    sqlite \
    ffmpeg \
    tzdata

# Create non-root user for security
RUN addgroup -g 1000 botuser && \
    adduser -D -u 1000 -G botuser botuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bot .

# Copy example config (actual config will be mounted or provided via env)
COPY config.yaml.example ./config.yaml.example

# Create necessary directories and set permissions
RUN mkdir -p data/audio data/output logs && \
    chown -R botuser:botuser /app

# Switch to non-root user
USER botuser

# Expose health check port (if implemented)
# EXPOSE 8080

# Set default config path
ENV CONFIG_PATH=/app/config.yaml

# Run the bot
CMD ["./bot", "/app/config.yaml"]
