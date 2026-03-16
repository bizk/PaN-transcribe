package transcribe

import (
	"context"
	"testing"
)

func TestOpenAITranscriber_Name(t *testing.T) {
	o := NewOpenAITranscriber("test-key", "whisper-1")
	if o.Name() != "openai-whisper" {
		t.Errorf("Name() = %q, want %q", o.Name(), "openai-whisper")
	}
}

func TestOpenAITranscriber_EmptyAPIKey(t *testing.T) {
	o := NewOpenAITranscriber("", "whisper-1")

	_, err := o.Transcribe(context.Background(), "/tmp/test.wav")
	if err == nil {
		t.Error("Transcribe() with empty API key should return error")
	}
}

func TestOpenAITranscriber_MissingFile(t *testing.T) {
	o := NewOpenAITranscriber("test-key", "whisper-1")

	_, err := o.Transcribe(context.Background(), "/nonexistent/audio.wav")
	if err == nil {
		t.Error("Transcribe() with missing file should return error")
	}
}
