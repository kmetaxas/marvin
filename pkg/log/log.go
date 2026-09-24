package log

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	globalLogger *slog.Logger
	setupOnce    sync.Once
)

// Configure sets up the global slog logger with the given level.
// Safe to call multiple times; only the first call takes effect.
func Configure(level string) {
	setupOnce.Do(func() {
		lvl := ParseLevel(level)
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
		globalLogger = slog.New(handler)
		slog.SetDefault(globalLogger)
	})
}

// Logger returns the configured global logger, falling back to info level if
// Configure has not been called.
func Logger() *slog.Logger {
	if globalLogger == nil {
		Configure("info")
	}
	return globalLogger
}

// ParseLevel converts a string like "debug", "info", "warn", or "error" into a
// slog.Level. Unrecognised values fall back to info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ValidateLevel returns an error if the level string is not one of the recognised
// values. An empty string is accepted (defaults to info at runtime).
func ValidateLevel(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "info", "warn", "warning", "error":
		return nil
	default:
		return fmt.Errorf("invalid log level %q (expected debug, info, warn, or error)", s)
	}
}
