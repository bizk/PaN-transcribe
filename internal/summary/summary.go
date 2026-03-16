package summary

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type Generator struct {
	client *openai.Client
	apiKey string
	model  string
}

func NewGenerator(apiKey, model string) *Generator {
	client := openai.NewClient(apiKey)
	return &Generator{
		client: client,
		apiKey: apiKey,
		model:  model,
	}
}

func (g *Generator) buildPrompt(customPrompt, transcript string) string {
	return fmt.Sprintf("%s\n\n---\n\nTranscripción:\n%s", customPrompt, transcript)
}

func (g *Generator) Generate(ctx context.Context, transcript, prompt string) (string, error) {
	if g.apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	fullPrompt := g.buildPrompt(prompt, transcript)

	req := openai.ChatCompletionRequest{
		Model: g.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fullPrompt,
			},
		},
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	resp, err := g.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summary generation failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	return resp.Choices[0].Message.Content, nil
}
