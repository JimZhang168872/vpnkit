// Package log wraps log/slog with a file-backed handler used across vpnkit.
package log

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Level is a thin alias to keep slog imports out of callers.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger writes structured logs to a file. Safe for concurrent use.
type Logger struct {
	*slog.Logger
	file io.Closer
}

// New opens (creates) the log file with 0o600 and returns a Logger writing to it.
func New(path string, level Level) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
	return &Logger{Logger: slog.New(handler), file: f}, nil
}

// Close releases the underlying file handle. Safe to call once.
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
