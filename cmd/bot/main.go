package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/override/pan-transcribe/internal/bot"
	"github.com/override/pan-transcribe/internal/cleanup"
	"github.com/override/pan-transcribe/internal/config"
	"github.com/override/pan-transcribe/internal/logger"
	"github.com/override/pan-transcribe/internal/queue"
	"github.com/override/pan-transcribe/internal/summary"
	"github.com/override/pan-transcribe/internal/transcribe"
	"github.com/override/pan-transcribe/internal/worker"
)

func main() {
	// Parse config path from args
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Fatal("Failed to load config: %v", err)
	}

	// Determine data directory
	dataDir := "data"
	if envDir := os.Getenv("DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	// Initialize logging
	logDir := "logs"
	if envLogDir := os.Getenv("LOG_DIR"); envLogDir != "" {
		logDir = envLogDir
	}
	debug := os.Getenv("DEBUG") == "true"
	if err := logger.InitializeLogging(logDir, debug); err != nil {
		logger.Fatal("Failed to initialize logging: %v", err)
	}

	logger.Info("Starting PaN Transcribe Bot")
	logger.Info("Config loaded from: %s", configPath)

	// Initialize database
	dbPath := filepath.Join(dataDir, "jobs.db")
	db, err := queue.OpenDB(dbPath)
	if err != nil {
		logger.Fatal("Failed to open database: %v", err)
	}
	defer db.Close()

	jobStore := queue.NewJobStore(db)
	settingsStore := queue.NewSettingsStore(db)

	logger.Info("Database initialized at: %s", dbPath)

	// Initialize bot
	tgBot, err := bot.New(cfg, jobStore, settingsStore, dataDir)
	if err != nil {
		logger.Fatal("Failed to create bot: %v", err)
	}

	// Initialize transcriber
	transcriber := transcribe.NewMistralTranscriber(cfg.Mistral.APIKey, cfg.Mistral.Model)
	logger.Info("Transcriber initialized: Mistral API (%s)", cfg.Mistral.Model)

	// Initialize summary generator
	summaryGen := summary.NewGenerator(cfg.OpenAI.APIKey, cfg.OpenAI.SummaryModel)
	logger.Info("Summary generator initialized: OpenAI (%s)", cfg.OpenAI.SummaryModel)

	// Initialize worker
	w := worker.New(worker.Config{
		DataDir:       dataDir,
		DefaultPrompt: cfg.Summary.DefaultPrompt,
	})
	w.SetJobStore(jobStore)
	w.SetSettingsStore(settingsStore)
	w.SetTranscriber(transcriber)
	w.SetSummaryGenerator(summaryGen)
	w.SetNotifier(tgBot)

	// Initialize cleanup
	cleaner := cleanup.New(cleanup.Config{
		OutputDir:     filepath.Join(dataDir, "output"),
		AudioDir:      filepath.Join(dataDir, "audio"),
		RetentionDays: cfg.Processing.OutputRetentionDays,
	})

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("Shutdown signal received: %v", sig)
		cancel()
		w.Stop()
		cleaner.Stop()
	}()

	// Start cleanup scheduler
	if err := cleaner.Start(); err != nil {
		logger.Warn("Cleanup scheduler failed to start: %v", err)
	}

	// Start worker in background
	go w.Start(ctx)

	// Start bot (blocking)
	logger.Info("Bot started and ready to receive messages")
	if err := tgBot.Start(); err != nil {
		logger.Fatal("Bot error: %v", err)
	}
}
