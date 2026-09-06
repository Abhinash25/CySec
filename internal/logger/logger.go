// Package logger provides structured logging for CySec.env with automatic
// secret redaction. Sensitive values like API keys, passwords, tokens, and
// private keys are masked in all log output.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the human-readable level name.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel converts a string to a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger is the CySec.env structured logger.
type Logger struct {
	mu       sync.Mutex
	level    Level
	writers  []io.Writer
	redact   bool
	patterns []*regexp.Regexp
}

// Predefined redaction patterns for common secret formats.
var defaultRedactionPatterns = []*regexp.Regexp{
	// API keys and tokens (generic hex/base64 strings after key= or token=)
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|passwd|pwd|auth)\s*[=:]\s*\S+`),
	// Bearer tokens
	regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]+`),
	// Private keys
	regexp.MustCompile(`(?i)-----BEGIN\s+\w+\s+PRIVATE\s+KEY-----`),
	// Connection strings with passwords
	regexp.MustCompile(`://[^:]+:[^@]+@`),
}

// New creates a new Logger.
func New(level Level, redact bool) *Logger {
	return &Logger{
		level:    level,
		writers:  []io.Writer{os.Stderr},
		redact:   redact,
		patterns: defaultRedactionPatterns,
	}
}

// AddFileOutput adds a log file writer. The log directory is created if needed.
func (l *Logger) AddFileOutput(logDir string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(logDir, "cysec.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.mu.Lock()
	l.writers = append(l.writers, f)
	l.mu.Unlock()
	return nil
}

// SetLevel changes the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
}

// redactMessage replaces sensitive patterns with [REDACTED].
func (l *Logger) redactMessage(msg string) string {
	if !l.redact {
		return msg
	}
	for _, pat := range l.patterns {
		msg = pat.ReplaceAllStringFunc(msg, func(match string) string {
			// Keep the key name but redact the value
			if idx := strings.IndexAny(match, "=:"); idx >= 0 {
				return match[:idx+1] + " [REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return msg
}

// log writes a formatted log entry at the given level.
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	msg := fmt.Sprintf(format, args...)
	msg = l.redactMessage(msg)

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] %s  %s\n", timestamp, level.String(), msg)

	for _, w := range l.writers {
		_, _ = io.WriteString(w, entry)
	}
}

// Debug logs a debug-level message.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Info logs an info-level message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warn logs a warning-level message.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Error logs an error-level message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Fatal logs an error-level message and exits.
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
	os.Exit(1)
}

// StdLogger returns a standard library *log.Logger that writes through this logger.
func (l *Logger) StdLogger() *log.Logger {
	return log.New(&logWriter{logger: l, level: LevelInfo}, "", 0)
}

// logWriter adapts Logger to io.Writer for use with standard library log.
type logWriter struct {
	logger *Logger
	level  Level
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.log(w.level, "%s", strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
