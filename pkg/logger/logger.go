package logger

import (
	"fmt"
	"io"
	"os"
)

// Logger is a simple CLI logger with colored output for key stages, warnings, and errors.
type Logger struct {
	writer    io.Writer
	useColors bool
}

// New creates a Logger that writes to stdout with colors enabled.
func New() *Logger {
	return &Logger{
		writer:    os.Stdout,
		useColors: true,
	}
}

// SetWriter overrides the output destination (useful for testing).
func (l *Logger) SetWriter(w io.Writer) {
	l.writer = w
}

// Info logs a key stage message (e.g. "Validating schema...").
// Safe to call on a nil Logger (no-op), so renderers can accept an optional logger.
func (l *Logger) Info(format string, args ...interface{}) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.useColors {
		fmt.Fprintf(l.writer, "  \033[36m▸\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(l.writer, "  ▸ %s\n", msg)
	}
}

// Success logs a completed stage (e.g. "✓ Generated 12 files").
// Safe to call on a nil Logger (no-op).
func (l *Logger) Success(format string, args ...interface{}) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.useColors {
		fmt.Fprintf(l.writer, "  \033[32m✓\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(l.writer, "  ✓ %s\n", msg)
	}
}

// Warn logs a non-fatal warning.
// Safe to call on a nil Logger (no-op).
func (l *Logger) Warn(format string, args ...interface{}) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.useColors {
		fmt.Fprintf(l.writer, "  \033[33m⚠\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(l.writer, "  ⚠ %s\n", msg)
	}
}

// Error logs an error message (does not exit).
// Safe to call on a nil Logger (no-op).
func (l *Logger) Error(format string, args ...interface{}) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.useColors {
		fmt.Fprintf(l.writer, "  \033[31m✗\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(l.writer, "  ✗ %s\n", msg)
	}
}
