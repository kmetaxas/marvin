package log

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
		{"  info  ", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ParseLevel(tt.input))
		})
	}
}

func TestValidateLevel(t *testing.T) {
	t.Parallel()

	assert.NoError(t, ValidateLevel(""))
	assert.NoError(t, ValidateLevel("debug"))
	assert.NoError(t, ValidateLevel("info"))
	assert.NoError(t, ValidateLevel("warn"))
	assert.NoError(t, ValidateLevel("warning"))
	assert.NoError(t, ValidateLevel("error"))

	assert.Error(t, ValidateLevel("trace"))
	assert.Error(t, ValidateLevel("fatal"))
	assert.ErrorContains(t, ValidateLevel("foo"), "invalid log level")
}

func TestLoggerReturnsNonNil(t *testing.T) {
	t.Parallel()

	l := Logger()
	assert.NotNil(t, l)
}

func TestConfigureDefaultsToInfo(t *testing.T) {
	t.Parallel()

	// Cannot re-run Configure in the same process because of sync.Once, but we
	// can verify ParseLevel falls back to info.
	assert.Equal(t, slog.LevelInfo, ParseLevel(""))
}
