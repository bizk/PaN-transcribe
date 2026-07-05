package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/override/pan-transcribe/internal/logger"
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
	transcriber      transcribe.Transcriber
	summaryGenerator *summary.Generator
	notifier         ResultNotifier
	stopCh           chan struct{}
	stopOnce         sync.Once
	log              *logger.Logger
}

func New(config Config) *Worker {
	return &Worker{
		config: config,
		stopCh: make(chan struct{}),
		log:    logger.New("worker"),
	}
}

func (w *Worker) SetJobStore(store *queue.JobStore) {
	w.jobStore = store
}

func (w *Worker) SetSettingsStore(store *queue.SettingsStore) {
	w.settingsStore = store
}

func (w *Worker) SetTranscriber(t transcribe.Transcriber) {
	w.transcriber = t
}

func (w *Worker) SetSummaryGenerator(g *summary.Generator) {
	w.summaryGenerator = g
}

func (w *Worker) SetNotifier(n ResultNotifier) {
	w.notifier = n
}

func (w *Worker) Start(ctx context.Context) {
	if w.jobStore == nil {
		w.log.Fatal("Worker started without jobStore")
	}
	if w.settingsStore == nil {
		w.log.Fatal("Worker started without settingsStore")
	}

	w.log.Info("Worker started, polling every 5 seconds")

	// Reset any jobs that were processing when we last shut down
	if err := w.jobStore.ResetProcessingJobs(); err != nil {
		w.log.Warn("Failed to reset processing jobs: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Worker stopping due to context cancellation")
			return
		case <-w.stopCh:
			w.log.Info("Worker stopping")
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
		w.log.Error("Failed to get next job: %v", err)
		return
	}

	if job == nil {
		return // No pending jobs
	}

	jobLog := w.log.WithFields(map[string]interface{}{
		"job_id":  job.ID,
		"chat_id": job.ChatID,
	})

	jobLog.Info("Processing job")

	if err := w.jobStore.UpdateStatus(job.ID, queue.StatusProcessing); err != nil {
		jobLog.Error("Failed to update job status: %v", err)
		return
	}

	outputPath, summaryPath, err := w.processJob(ctx, job)
	if err != nil {
		jobLog.Error("Job failed: %v", err)
		w.jobStore.Fail(job.ID, err.Error())
		if w.notifier != nil {
			w.notifier.SendError(job.ChatID, err.Error())
		}

		// Clean up audio file for failed jobs after 1 hour (handled by cleanup task)
		return
	}

	if err := w.jobStore.Complete(job.ID, outputPath, summaryPath); err != nil {
		jobLog.Error("Failed to complete job: %v", err)
		return
	}

	// Clean up audio file immediately after successful processing
	os.Remove(job.AudioPath)

	// Notify user
	if w.notifier != nil {
		if err := w.notifier.SendResult(job.ChatID, outputPath, summaryPath); err != nil {
			jobLog.Error("Failed to notify user: %v", err)
		}
	}

	jobLog.Info("Job completed successfully")
}

func (w *Worker) processJob(ctx context.Context, job *queue.Job) (outputPath, summaryPath string, err error) {
	if w.transcriber == nil {
		return "", "", fmt.Errorf("transcriber not configured")
	}

	jobLog := w.log.WithField("job_id", job.ID)

	// Transcribe
	jobLog.Info("Starting transcription with %s: %s", w.transcriber.Name(), job.AudioPath)
	startTime := time.Now()
	result, err := w.transcriber.Transcribe(ctx, job.AudioPath)
	if err != nil {
		return "", "", fmt.Errorf("transcription failed: %w", err)
	}
	jobLog.Info("Transcription completed in %v", time.Since(startTime))

	// Save transcription
	outputDir := filepath.Join(w.config.DataDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("creating output dir: %w", err)
	}

	outputPath = filepath.Join(outputDir, fmt.Sprintf("%d.txt", job.ID))
	if err := os.WriteFile(outputPath, []byte(result.Text), 0644); err != nil {
		return "", "", fmt.Errorf("saving transcription: %w", err)
	}
	jobLog.Info("Transcription saved to: %s", outputPath)

	// Generate summary if requested
	if job.WithSummary && w.summaryGenerator != nil {
		jobLog.Info("Starting summary generation")

		// Get custom prompt or use default
		prompt, err := w.settingsStore.GetCustomPrompt(job.ChatID)
		if err != nil {
			jobLog.Warn("Failed to get custom prompt: %v", err)
		}
		if prompt == "" {
			prompt = w.config.DefaultPrompt
		}

		summaryStartTime := time.Now()
		summaryText, err := w.summaryGenerator.Generate(ctx, result.Text, prompt)
		if err != nil {
			jobLog.Warn("Summary generation failed: %v", err)
			// Don't fail the job, just skip summary
		} else {
			jobLog.Info("Summary generated in %v", time.Since(summaryStartTime))
			summaryPath = filepath.Join(outputDir, fmt.Sprintf("%d_summary.txt", job.ID))
			if err := os.WriteFile(summaryPath, []byte(summaryText), 0644); err != nil {
				jobLog.Error("Failed to save summary: %v", err)
			} else {
				jobLog.Info("Summary saved to: %s", summaryPath)
			}
		}
	}

	return outputPath, summaryPath, nil
}
