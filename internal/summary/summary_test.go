package summary

import (
	"context"
	"strings"
	"testing"
)

func TestGenerator_Model(t *testing.T) {
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

	// Should contain the custom prompt
	if !strings.Contains(result, customPrompt) {
		t.Error("buildPrompt result should contain custom prompt")
	}

	// Should contain the transcript
	if !strings.Contains(result, transcript) {
		t.Error("buildPrompt result should contain transcript")
	}

	// Should contain the separator
	if !strings.Contains(result, "---") {
		t.Error("buildPrompt result should contain separator")
	}
}

func TestGenerator_EmptyAPIKey(t *testing.T) {
	g := NewGenerator("", "gpt-4o-mini")

	_, err := g.Generate(context.Background(), "test transcript", "test prompt")
	if err == nil {
		t.Error("Generate() with empty API key should return error")
	}
}
