package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/override/pan-transcribe/internal/bot"
	"github.com/override/pan-transcribe/internal/cleanup"
	"github.com/override/pan-transcribe/internal/config"
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
		log.Fatalf("Failed to load config: %v", err)
	}

	// Determine data directory
	dataDir := "data"
	if envDir := os.Getenv("DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	// Initialize database
	dbPath := filepath.Join(dataDir, "jobs.db")
	db, err := queue.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	jobStore := queue.NewJobStore(db)
	settingsStore := queue.NewSettingsStore(db)

	// Initialize bot
	tgBot, err := bot.New(cfg, jobStore, settingsStore, dataDir)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Initialize transcribers
	localTranscriber := transcribe.NewWhisperTranscriber(cfg.Whisper.ModelPath, cfg.Whisper.Threads)
	cloudTranscriber := transcribe.NewOpenAITranscriber(cfg.OpenAI.APIKey, cfg.OpenAI.WhisperModel)

	// Initialize summary generator
	summaryGen := summary.NewGenerator(cfg.OpenAI.APIKey, cfg.OpenAI.SummaryModel)

	// Initialize worker
	w := worker.New(worker.Config{
		DataDir:       dataDir,
		DefaultPrompt: cfg.Summary.DefaultPrompt,
	})
	w.SetJobStore(jobStore)
	w.SetSettingsStore(settingsStore)
	w.SetLocalTranscriber(localTranscriber)
	w.SetCloudTranscriber(cloudTranscriber)
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
		<-sigCh
		log.Println("Shutdown signal received")
		cancel()
		w.Stop()
		cleaner.Stop()
	}()

	// Start cleanup scheduler
	if err := cleaner.Start(); err != nil {
		log.Printf("Warning: cleanup scheduler failed to start: %v", err)
	}

	// Start worker in background
	go w.Start(ctx)

	// Start bot (blocking)
	log.Println("Starting bot...")
	if err := tgBot.Start(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
