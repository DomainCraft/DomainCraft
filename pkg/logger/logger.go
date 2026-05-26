package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Level represents a logging level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

var levelColors = map[Level]string{
	DEBUG: "\033[36m", // cyan
	INFO:  "\033[32m", // green
	WARN:  "\033[33m", // yellow
	ERROR: "\033[31m", // red
	FATAL: "\033[35m", // magenta
}

const reset = "\033[0m"
const bold = "\033[1m"

// Logger is a simple leveled logger with optional color output
type Logger struct {
	writer    io.Writer
	minLevel  Level
	useColors bool
}

// New creates a new Logger with default settings
func New() *Logger {
	return &Logger{
		writer:    os.Stdout,
		minLevel:  INFO,
		useColors: true,
	}
}

// SetMinLevel sets the minimum logging level
func (l *Logger) SetMinLevel(level Level) {
	l.minLevel = level
}

// SetWriter sets the output writer
func (l *Logger) SetWriter(w io.Writer) {
	l.writer = w
}

// SetColors enables or disables colored output
func (l *Logger) SetColors(enabled bool) {
	l.useColors = enabled
}

// log is the core logging function
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.minLevel {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	levelName := levelNames[level]

	var prefix string
	if l.useColors {
		color := levelColors[level]
		prefix = fmt.Sprintf("%s%s[%s]%s %s", color, bold, timestamp, reset, color)
		prefix = fmt.Sprintf("%s%-5s%s ", prefix, levelName, reset)
	} else {
		prefix = fmt.Sprintf("[%s] %-5s ", timestamp, levelName)
	}

	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.writer, "%s%s\n", prefix, message)

	if level == FATAL {
		os.Exit(1)
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

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}

// Success logs a success message (green checkmark)
func (l *Logger) Success(format string, args ...interface{}) {
	if l.minLevel <= INFO {
		message := fmt.Sprintf(format, args...)
		if l.useColors {
			fmt.Fprintf(l.writer, "%s✓ %s%s\n", "\033[32m", message, reset)
		} else {
			fmt.Fprintf(l.writer, "✓ %s\n", message)
		}
	}
}

// Heading logs a section heading
func (l *Logger) Heading(text string) {
	if l.minLevel <= INFO {
		if l.useColors {
			fmt.Fprintf(l.writer, "\n%s%s=== %s ===%s\n\n", bold, "\033[36m", text, reset)
		} else {
			fmt.Fprintf(l.writer, "\n=== %s ===\n\n", text)
		}
	}
}

// Verbose logs a detailed message (only when minLevel <= DEBUG)
func (l *Logger) Verbose(format string, args ...interface{}) {
	if l.minLevel <= DEBUG {
		message := fmt.Sprintf(format, args...)
		if l.useColors {
			fmt.Fprintf(l.writer, "%s  → %s%s\n", "\033[90m", message, reset)
		} else {
			fmt.Fprintf(l.writer, "  → %s\n", message)
		}
	}
}

// Table logs a table with headers and rows
func (l *Logger) Table(headers []string, rows [][]string) {
	if l.minLevel > INFO {
		return
	}

	// Compute column widths
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		fmt.Fprintf(l.writer, "%-*s  ", colWidths[i], h)
	}
	fmt.Fprintf(l.writer, "\n")

	// Print separator
	for i, w := range colWidths {
		fmt.Fprint(l.writer, strings.Repeat("-", w))
		if i < len(colWidths)-1 {
			fmt.Fprint(l.writer, "  ")
		}
	}
	fmt.Fprint(l.writer, "\n")

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			fmt.Fprintf(l.writer, "%-*s  ", colWidths[i], cell)
		}
		fmt.Fprintf(l.writer, "\n")
	}
	fmt.Fprintf(l.writer, "\n")
}
