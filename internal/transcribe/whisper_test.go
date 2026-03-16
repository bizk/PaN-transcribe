package transcribe

import (
	"context"
	"testing"
)

func TestWhisperTranscriber_Name(t *testing.T) {
	w := NewWhisperTranscriber("/path/to/model", 4)
	if w.Name() != "whisper.cpp" {
		t.Errorf("Name() = %q, want %q", w.Name(), "whisper.cpp")
	}
}

func TestWhisperTranscriber_buildCommand(t *testing.T) {
	w := NewWhisperTranscriber("/models/ggml-small.bin", 4)

	args := w.buildArgs("/tmp/audio.wav", "/tmp/output")

	hasModel := false
	hasThreads := false
	hasFile := false
	hasOutput := false

	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) && args[i+1] == "/models/ggml-small.bin" {
			hasModel = true
		}
		if arg == "-t" && i+1 < len(args) && args[i+1] == "4" {
			hasThreads = true
		}
		if arg == "-f" && i+1 < len(args) && args[i+1] == "/tmp/audio.wav" {
			hasFile = true
		}
		if arg == "-of" && i+1 < len(args) && args[i+1] == "/tmp/output" {
			hasOutput = true
		}
	}

	if !hasModel {
		t.Error("missing -m model argument")
	}
	if !hasThreads {
		t.Error("missing -t threads argument")
	}
	if !hasFile {
		t.Error("missing -f file argument")
	}
	if !hasOutput {
		t.Error("missing -of output argument")
	}
}

func TestWhisperTranscriber_Transcribe_MissingBinary(t *testing.T) {
	w := &WhisperTranscriber{
		modelPath:  "/nonexistent/model.bin",
		threads:    4,
		binaryPath: "/nonexistent/whisper",
	}

	_, err := w.Transcribe(context.Background(), "/tmp/audio.wav")
	if err == nil {
		t.Error("expected error for missing binary, got nil")
	}
}
