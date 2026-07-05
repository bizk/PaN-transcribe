package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	l := New("test")
	l.SetOutput(&buf)
	l.SetLevel(DEBUG)

	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "[test]") {
		t.Errorf("Expected [test] component in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	l := New("test")
	l.SetOutput(&buf)
	l.SetLevel(DEBUG)

	l2 := l.WithField("user_id", 123)
	l2.Info("user action")

	output := buf.String()
	if !strings.Contains(output, "user_id=123") {
		t.Errorf("Expected user_id=123 in output, got: %s", output)
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	l := New("test")
	l.SetOutput(&buf)
	l.SetLevel(WARN)

	// Should not be logged (below WARN)
	l.Debug("debug message")
	l.Info("info message")

	// Should be logged
	l.Warn("warn message")
	l.Error("error message")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Errorf("DEBUG should not be logged when level is WARN")
	}
	if strings.Contains(output, "info message") {
		t.Errorf("INFO should not be logged when level is WARN")
	}
	if !strings.Contains(output, "warn message") {
		t.Errorf("WARN should be logged")
	}
	if !strings.Contains(output, "error message") {
		t.Errorf("ERROR should be logged")
	}
}

func TestLoggerWithMultipleFields(t *testing.T) {
	var buf bytes.Buffer
	l := New("test")
	l.SetOutput(&buf)

	l2 := l.WithFields(map[string]interface{}{
		"job_id":  42,
		"chat_id": 12345,
	})
	l2.Info("processing job")

	output := buf.String()
	if !strings.Contains(output, "job_id=42") {
		t.Errorf("Expected job_id=42 in output, got: %s", output)
	}
	if !strings.Contains(output, "chat_id=12345") {
		t.Errorf("Expected chat_id=12345 in output, got: %s", output)
	}
}
