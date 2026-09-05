// Package logger provides a structured logger with configurable log levels.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog.Logger with additional context.
type Logger struct {
	*slog.Logger
	verbose bool
}

var globalLogger *Logger

// Level constants.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Init initializes the global logger with the given level and format.
func Init(level, format string) *Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case LevelDebug:
		logLevel = slog.LevelDebug
	case LevelWarn:
		logLevel = slog.LevelWarn
	case LevelError:
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := &Logger{
		Logger:  slog.New(handler),
		verbose: logLevel <= slog.LevelDebug,
	}

	globalLogger = logger
	return logger
}

// Get returns the global logger instance.
func Get() *Logger {
	if globalLogger == nil {
		globalLogger = Init(LevelInfo, "text")
	}
	return globalLogger
}

// With returns a new logger with the given attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger:  l.Logger.With(args...),
		verbose: l.verbose,
	}
}

// IsVerbose returns true if debug level is enabled.
func (l *Logger) IsVerbose() bool {
	return l.verbose
}

// Log verbose debug message only if verbose mode is enabled.
func (l *Logger) DebugVerbose(msg string, args ...any) {
	if l.verbose {
		l.Debug(msg, args...)
	}
}
