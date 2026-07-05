package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Level represents the severity of a log message
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

// Logger provides structured logging with levels and context
type Logger struct {
	mu        sync.Mutex
	output    io.Writer
	level     Level
	component string
	fields    map[string]interface{}
}

// New creates a new logger for a specific component
func New(component string) *Logger {
	return &Logger{
		output:    os.Stdout,
		level:     INFO,
		component: component,
		fields:    make(map[string]interface{}),
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput sets the output destination
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// WithField returns a new logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &Logger{
		output:    l.output,
		level:     l.level,
		component: l.component,
		fields:    newFields,
	}
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		output:    l.output,
		level:     l.level,
		component: l.component,
		fields:    newFields,
	}
}

// log writes a log message with the given level
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := levelNames[level]
	message := fmt.Sprintf(format, args...)

	// Build context string from fields
	var context string
	if len(l.fields) > 0 {
		context = " ["
		first := true
		for k, v := range l.fields {
			if !first {
				context += " "
			}
			context += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		context += "]"
	}

	logLine := fmt.Sprintf("%s [%s] [%s]%s %s\n", timestamp, levelStr, l.component, context, message)

	// Write to output
	if l.output != nil {
		l.output.Write([]byte(logLine))
	}

	// Also write errors to stderr for visibility
	if level == ERROR {
		os.Stderr.Write([]byte(logLine))
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs an error message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
	os.Exit(1)
}

// Global logger instance
var std = New("main")

// SetGlobalLevel sets the level for the global logger
func SetGlobalLevel(level Level) {
	std.SetLevel(level)
}

// SetGlobalOutput sets the output for the global logger
func SetGlobalOutput(w io.Writer) {
	std.SetOutput(w)
}

// Debug logs a debug message using the global logger
func Debug(format string, args ...interface{}) {
	std.Debug(format, args...)
}

// Info logs an info message using the global logger
func Info(format string, args ...interface{}) {
	std.Info(format, args...)
}

// Warn logs a warning message using the global logger
func Warn(format string, args ...interface{}) {
	std.Warn(format, args...)
}

// Error logs an error message using the global logger
func Error(format string, args ...interface{}) {
	std.Error(format, args...)
}

// Fatal logs an error message and exits using the global logger
func Fatal(format string, args ...interface{}) {
	std.Fatal(format, args...)
}

// InitializeLogging sets up logging for the application
func InitializeLogging(logDir string, debug bool) error {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	// Create log file with date
	logFile := fmt.Sprintf("%s/bot_%s.log", logDir, time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	// Write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, f)
	std.SetOutput(multiWriter)

	// Set log level
	if debug {
		std.SetLevel(DEBUG)
	} else {
		std.SetLevel(INFO)
	}

	// Also set standard library logger to use our format
	log.SetOutput(multiWriter)
	log.SetFlags(0) // We handle formatting ourselves

	return nil
}
