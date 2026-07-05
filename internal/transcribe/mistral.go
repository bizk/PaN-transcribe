package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const mistralAPIURL = "https://api.mistral.ai/v1/audio/transcriptions"

type MistralTranscriber struct {
	apiKey   string
	model    string
	language string
	client   *http.Client
}

type mistralResponse struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Usage    struct {
		PromptAudioSeconds float64 `json:"prompt_audio_seconds"`
	} `json:"usage"`
}

type mistralErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func NewMistralTranscriber(apiKey, model string) *MistralTranscriber {
	if model == "" {
		model = "mistral-large-latest"
	}
	return &MistralTranscriber{
		apiKey:   apiKey,
		model:    model,
		language: "es", // Spanish by default
		client:   &http.Client{},
	}
}

func (m *MistralTranscriber) Name() string {
	return "mistral"
}

func (m *MistralTranscriber) Transcribe(ctx context.Context, audioPath string) (*Result, error) {
	if m.apiKey == "" {
		return nil, fmt.Errorf("Mistral API key not configured")
	}

	// Check file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("audio file not found: %s", audioPath)
	} else if err != nil {
		return nil, fmt.Errorf("checking audio file: %w", err)
	}

	// Open file
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("opening audio file: %w", err)
	}
	defer file.Close()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copying file to form: %w", err)
	}

	// Add model field
	if err := writer.WriteField("model", m.model); err != nil {
		return nil, fmt.Errorf("writing model field: %w", err)
	}

	// Add language field
	if err := writer.WriteField("language", m.language); err != nil {
		return nil, fmt.Errorf("writing language field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", mistralAPIURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp mistralErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("Mistral API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("Mistral API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result mistralResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &Result{
		Text:     result.Text,
		Language: result.Language,
		Duration: result.Usage.PromptAudioSeconds,
	}, nil
}

// SetHTTPClient allows setting a custom HTTP client for testing
func (m *MistralTranscriber) SetHTTPClient(client *http.Client) {
	m.client = client
}

// SetLanguage allows overriding the default language
func (m *MistralTranscriber) SetLanguage(lang string) {
	m.language = lang
}
