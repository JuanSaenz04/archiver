package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

func Env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func Required(name string) (string, error) {
	value := Env(name)
	if value == "" {
		return "", fmt.Errorf("environment variable %s not set", name)
	}
	return value, nil
}

func Int(name string, defaultValue int) (int, error) {
	value := Env(name)
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s must be an integer: %w", name, err)
	}
	return parsed, nil
}

func LogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
