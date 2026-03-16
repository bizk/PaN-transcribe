package transcribe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type WhisperTranscriber struct {
	modelPath  string
	threads    int
	binaryPath string
}

func NewWhisperTranscriber(modelPath string, threads int) *WhisperTranscriber {
	return &WhisperTranscriber{
		modelPath:  modelPath,
		threads:    threads,
		binaryPath: "whisper",
	}
}

func (w *WhisperTranscriber) Name() string {
	return "whisper.cpp"
}

func (w *WhisperTranscriber) SetBinaryPath(path string) {
	w.binaryPath = path
}

func (w *WhisperTranscriber) buildArgs(audioPath, outputBase string) []string {
	return []string{
		"-m", w.modelPath,
		"-t", strconv.Itoa(w.threads),
		"-l", "es",
		"-otxt",
		"-of", outputBase,
		"-f", audioPath,
	}
}

func (w *WhisperTranscriber) Transcribe(ctx context.Context, audioPath string) (*Result, error) {
	if _, err := exec.LookPath(w.binaryPath); err != nil {
		return nil, fmt.Errorf("whisper binary not found: %w", err)
	}

	if _, err := os.Stat(w.modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper model not found: %s", w.modelPath)
	}

	outputDir := filepath.Dir(audioPath)
	outputBase := filepath.Join(outputDir, "transcript")

	args := w.buildArgs(audioPath, outputBase)
	cmd := exec.CommandContext(ctx, w.binaryPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("whisper failed: %w\noutput: %s", err, string(output))
	}

	outputPath := outputBase + ".txt"
	text, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("reading transcription: %w", err)
	}

	os.Remove(outputPath)

	return &Result{
		Text:     strings.TrimSpace(string(text)),
		Language: "es",
	}, nil
}
