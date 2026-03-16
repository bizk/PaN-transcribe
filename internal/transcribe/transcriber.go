package transcribe

import "context"

// Result contains the transcription output
type Result struct {
	Text     string
	Language string
	Duration float64 // audio duration in seconds
}

// Transcriber defines the interface for audio transcription
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) (*Result, error)
	Name() string
}
