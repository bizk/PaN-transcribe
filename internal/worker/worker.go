package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/override/pan-transcribe/internal/queue"
	"github.com/override/pan-transcribe/internal/summary"
	"github.com/override/pan-transcribe/internal/transcribe"
)

type Config struct {
	DataDir       string
	DefaultPrompt string
}

type ResultNotifier interface {
	SendResult(chatID int64, outputPath, summaryPath string) error
	SendError(chatID int64, errorMsg string)
	SendProgress(chatID int64, percent int, estimatedMinutes int)
}

type Worker struct {
	config           Config
	jobStore         *queue.JobStore
	settingsStore    *queue.SettingsStore
	localTranscriber transcribe.Transcriber
	cloudTranscriber transcribe.Transcriber
	summaryGenerator *summary.Generator
	notifier         ResultNotifier
	stopCh           chan struct{}
	stopOnce         sync.Once
}

func New(config Config) *Worker {
	return &Worker{
		config: config,
		stopCh: make(chan struct{}),
	}
}

func (w *Worker) SetJobStore(store *queue.JobStore) {
	w.jobStore = store
}

func (w *Worker) SetSettingsStore(store *queue.SettingsStore) {
	w.settingsStore = store
}

func (w *Worker) SetLocalTranscriber(t transcribe.Transcriber) {
	w.localTranscriber = t
}

func (w *Worker) SetCloudTranscriber(t transcribe.Transcriber) {
	w.cloudTranscriber = t
}

func (w *Worker) SetSummaryGenerator(g *summary.Generator) {
	w.summaryGenerator = g
}

func (w *Worker) SetNotifier(n ResultNotifier) {
	w.notifier = n
}

func (w *Worker) Start(ctx context.Context) {
	if w.jobStore == nil {
		log.Fatal("Worker started without jobStore")
	}
	if w.settingsStore == nil {
		log.Fatal("Worker started without settingsStore")
	}

	log.Println("Worker started")

	// Reset any jobs that were processing when we last shut down
	if err := w.jobStore.ResetProcessingJobs(); err != nil {
		log.Printf("Error resetting processing jobs: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker stopping due to context cancellation")
			return
		case <-w.stopCh:
			log.Println("Worker stopping")
			return
		case <-ticker.C:
			w.processNextJob(ctx)
		}
	}
}

func (w *Worker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

func (w *Worker) processNextJob(ctx context.Context) {
	job, err := w.jobStore.GetNextPending()
	if err != nil {
		log.Printf("Error getting next job: %v", err)
		return
	}

	if job == nil {
		return // No pending jobs
	}

	log.Printf("Processing job #%d", job.ID)

	if err := w.jobStore.UpdateStatus(job.ID, queue.StatusProcessing); err != nil {
		log.Printf("Error updating job status: %v", err)
		return
	}

	outputPath, summaryPath, err := w.processJob(ctx, job)
	if err != nil {
		log.Printf("Job #%d failed: %v", job.ID, err)
		w.jobStore.Fail(job.ID, err.Error())
		if w.notifier != nil {
			w.notifier.SendError(job.ChatID, err.Error())
		}

		// Clean up audio file for failed jobs after 1 hour (handled by cleanup task)
		return
	}

	if err := w.jobStore.Complete(job.ID, outputPath, summaryPath); err != nil {
		log.Printf("Error completing job: %v", err)
		return
	}

	// Clean up audio file immediately after successful processing
	os.Remove(job.AudioPath)

	// Notify user
	if w.notifier != nil {
		if err := w.notifier.SendResult(job.ChatID, outputPath, summaryPath); err != nil {
			log.Printf("Error notifying user: %v", err)
		}
	}

	log.Printf("Job #%d completed successfully", job.ID)
}

func (w *Worker) processJob(ctx context.Context, job *queue.Job) (outputPath, summaryPath string, err error) {
	// Select transcriber based on mode
	var t transcribe.Transcriber
	if job.Mode == "cloud" {
		t = w.cloudTranscriber
	} else {
		t = w.localTranscriber
	}

	if t == nil {
		return "", "", fmt.Errorf("transcriber not configured for mode: %s", job.Mode)
	}

	// Transcribe
	log.Printf("Transcribing with %s: %s", t.Name(), job.AudioPath)
	result, err := t.Transcribe(ctx, job.AudioPath)
	if err != nil {
		// If local transcriber fails, we could retry with cloud
		// For now, just return the error
		return "", "", fmt.Errorf("transcription failed: %w", err)
	}

	// Save transcription
	outputDir := filepath.Join(w.config.DataDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("creating output dir: %w", err)
	}

	outputPath = filepath.Join(outputDir, fmt.Sprintf("%d.txt", job.ID))
	if err := os.WriteFile(outputPath, []byte(result.Text), 0644); err != nil {
		return "", "", fmt.Errorf("saving transcription: %w", err)
	}

	// Generate summary if requested
	if job.WithSummary && w.summaryGenerator != nil {
		log.Printf("Generating summary for job #%d", job.ID)

		// Get custom prompt or use default
		prompt, err := w.settingsStore.GetCustomPrompt(job.ChatID)
		if err != nil {
			log.Printf("Error getting custom prompt: %v", err)
		}
		if prompt == "" {
			prompt = w.config.DefaultPrompt
		}

		summaryText, err := w.summaryGenerator.Generate(ctx, result.Text, prompt)
		if err != nil {
			log.Printf("Summary generation failed: %v", err)
			// Don't fail the job, just skip summary
		} else {
			summaryPath = filepath.Join(outputDir, fmt.Sprintf("%d_summary.txt", job.ID))
			if err := os.WriteFile(summaryPath, []byte(summaryText), 0644); err != nil {
				log.Printf("Error saving summary: %v", err)
			}
		}
	}

	return outputPath, summaryPath, nil
}
