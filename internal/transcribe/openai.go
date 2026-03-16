package transcribe

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

type OpenAITranscriber struct {
	client *openai.Client
	apiKey string
	model  string
}

func NewOpenAITranscriber(apiKey, model string) *OpenAITranscriber {
	client := openai.NewClient(apiKey)
	return &OpenAITranscriber{
		client: client,
		apiKey: apiKey,
		model:  model,
	}
}

func (o *OpenAITranscriber) Name() string {
	return "openai-whisper"
}

func (o *OpenAITranscriber) Transcribe(ctx context.Context, audioPath string) (*Result, error) {
	if o.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Check file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("audio file not found: %s", audioPath)
	}

	req := openai.AudioRequest{
		Model:    o.model,
		FilePath: audioPath,
		Language: "es", // Spanish
	}

	resp, err := o.client.CreateTranscription(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI transcription failed: %w", err)
	}

	return &Result{
		Text:     resp.Text,
		Language: "es",
	}, nil
}
