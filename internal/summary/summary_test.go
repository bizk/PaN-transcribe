package summary

import (
	"testing"
)

func TestGenerator_Name(t *testing.T) {
	g := NewGenerator("test-key", "gpt-4o-mini")
	if g.model != "gpt-4o-mini" {
		t.Errorf("model = %q, want %q", g.model, "gpt-4o-mini")
	}
}

func TestGenerator_BuildPrompt(t *testing.T) {
	g := NewGenerator("test-key", "gpt-4o-mini")

	customPrompt := "Summarize this:"
	transcript := "This is the transcript text."

	result := g.buildPrompt(customPrompt, transcript)

	if result == "" {
		t.Error("buildPrompt returned empty string")
	}

	// Should contain both prompt and transcript
	if len(result) < len(customPrompt)+len(transcript) {
		t.Error("buildPrompt result too short")
	}
}
