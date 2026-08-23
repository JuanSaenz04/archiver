package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvTrimsWhitespace(t *testing.T) {
	t.Setenv("TEST_CONFIG_ENV", "  value  ")
	assert.Equal(t, "value", Env("TEST_CONFIG_ENV"))
}

func TestRequired(t *testing.T) {
	t.Setenv("TEST_CONFIG_REQUIRED", "configured")
	value, err := Required("TEST_CONFIG_REQUIRED")
	require.NoError(t, err)
	assert.Equal(t, "configured", value)

	t.Setenv("TEST_CONFIG_REQUIRED", "   ")
	_, err = Required("TEST_CONFIG_REQUIRED")
	require.EqualError(t, err, "environment variable TEST_CONFIG_REQUIRED not set")
}

func TestInt(t *testing.T) {
	t.Setenv("TEST_CONFIG_INT", "42")
	value, err := Int("TEST_CONFIG_INT", 90)
	require.NoError(t, err)
	assert.Equal(t, 42, value)

	t.Setenv("TEST_CONFIG_INT", "")
	value, err = Int("TEST_CONFIG_INT", 90)
	require.NoError(t, err)
	assert.Equal(t, 90, value)

	t.Setenv("TEST_CONFIG_INT", "invalid")
	_, err = Int("TEST_CONFIG_INT", 90)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TEST_CONFIG_INT")
}

func TestLogLevel(t *testing.T) {
	assert.Equal(t, slog.LevelDebug, LogLevel(" DEBUG "))
	assert.Equal(t, slog.LevelWarn, LogLevel("warning"))
	assert.Equal(t, slog.LevelError, LogLevel("error"))
	assert.Equal(t, slog.LevelInfo, LogLevel(""))
	assert.Equal(t, slog.LevelInfo, LogLevel("invalid"))
}
