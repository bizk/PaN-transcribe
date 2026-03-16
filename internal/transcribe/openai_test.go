package transcribe

import (
	"testing"
)

func TestOpenAITranscriber_Name(t *testing.T) {
	o := NewOpenAITranscriber("test-key", "whisper-1")
	if o.Name() != "openai-whisper" {
		t.Errorf("Name() = %q, want %q", o.Name(), "openai-whisper")
	}
}

func TestOpenAITranscriber_InvalidAPIKey(t *testing.T) {
	// This is a unit test that doesn't make actual API calls
	o := NewOpenAITranscriber("", "whisper-1")
	if o.apiKey != "" {
		t.Errorf("apiKey = %q, want empty", o.apiKey)
	}
}
