package transcribe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMistralTranscriber_Name(t *testing.T) {
	m := NewMistralTranscriber("test-key", "mistral-large-latest")
	if m.Name() != "mistral" {
		t.Errorf("Name() = %q, want %q", m.Name(), "mistral")
	}
}

func TestMistralTranscriber_DefaultModel(t *testing.T) {
	m := NewMistralTranscriber("test-key", "")
	if m.model != "mistral-large-latest" {
		t.Errorf("default model = %q, want %q", m.model, "mistral-large-latest")
	}
}

func TestMistralTranscriber_EmptyAPIKey(t *testing.T) {
	m := NewMistralTranscriber("", "mistral-large-latest")

	_, err := m.Transcribe(context.Background(), "/tmp/test.wav")
	if err == nil {
		t.Error("Transcribe() with empty API key should return error")
	}
	if !strings.Contains(err.Error(), "API key not configured") {
		t.Errorf("expected API key error, got: %v", err)
	}
}

func TestMistralTranscriber_MissingFile(t *testing.T) {
	m := NewMistralTranscriber("test-key", "mistral-large-latest")

	_, err := m.Transcribe(context.Background(), "/nonexistent/audio.wav")
	if err == nil {
		t.Error("Transcribe() with missing file should return error")
	}
	if !strings.Contains(err.Error(), "audio file not found") {
		t.Errorf("expected file not found error, got: %v", err)
	}
}

func TestMistralTranscriber_SuccessfulTranscription(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing or invalid Authorization header")
		}

		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Error("Content-Type should be multipart/form-data")
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("failed to parse multipart form: %v", err)
		}

		// Verify fields
		if r.FormValue("model") == "" {
			t.Error("model field missing")
		}
		if r.FormValue("language") != "es" {
			t.Errorf("expected language es, got %s", r.FormValue("language"))
		}

		// Verify file
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("file field missing: %v", err)
		} else {
			file.Close()
		}

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"text": "Hola, esta es una transcripción de prueba.",
			"language": "es",
			"usage": {
				"prompt_audio_seconds": 10.5
			}
		}`))
	}))
	defer server.Close()

	// Create temp audio file
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test.wav")
	if err := os.WriteFile(audioPath, []byte("fake audio data"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Create transcriber with custom client pointing to mock server
	m := NewMistralTranscriber("test-key", "mistral-large-latest")
	m.client = &http.Client{
		Transport: &mockTransport{server: server},
	}

	result, err := m.Transcribe(context.Background(), audioPath)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}

	if result.Text != "Hola, esta es una transcripción de prueba." {
		t.Errorf("unexpected text: %s", result.Text)
	}
	if result.Language != "es" {
		t.Errorf("expected language es, got %s", result.Language)
	}
	if result.Duration != 10.5 {
		t.Errorf("expected duration 10.5, got %f", result.Duration)
	}
}

func TestMistralTranscriber_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message": "Invalid audio format", "type": "invalid_request_error"}`))
	}))
	defer server.Close()

	// Create temp audio file
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "test.wav")
	if err := os.WriteFile(audioPath, []byte("fake audio data"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	m := NewMistralTranscriber("test-key", "mistral-large-latest")
	m.client = &http.Client{
		Transport: &mockTransport{server: server},
	}

	_, err := m.Transcribe(context.Background(), audioPath)
	if err == nil {
		t.Error("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "Invalid audio format") {
		t.Errorf("expected error message to contain API error, got: %v", err)
	}
}

func TestMistralTranscriber_SetLanguage(t *testing.T) {
	m := NewMistralTranscriber("test-key", "mistral-large-latest")
	if m.language != "es" {
		t.Errorf("default language should be es, got %s", m.language)
	}

	m.SetLanguage("en")
	if m.language != "en" {
		t.Errorf("expected language en, got %s", m.language)
	}
}

// mockTransport redirects requests to the test server
type mockTransport struct {
	server *httptest.Server
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and copy the body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()

	// Create new request to test server
	newReq, err := http.NewRequest(req.Method, t.server.URL+req.URL.Path, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}

	// Copy headers
	for k, v := range req.Header {
		newReq.Header[k] = v
	}

	return http.DefaultTransport.RoundTrip(newReq)
}
