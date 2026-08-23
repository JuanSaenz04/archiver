package main

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setAPIConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("ARCHIVES_DIR", t.TempDir())
	t.Setenv("SQLITE_DIR", "")
	t.Setenv("APP_PUBLIC_URL", "https://archiver.example.com")
	t.Setenv("REPLAY_PUBLIC_URL", "https://replay.example.com")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("TRUSTED_PROXIES", "")
}

func TestLoadConfig(t *testing.T) {
	setAPIConfigEnv(t)
	sqliteDir := t.TempDir()
	t.Setenv("SQLITE_DIR", sqliteDir)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8")

	cfg, err := loadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg.RedisOptions)

	assert.Equal(t, "localhost:6379", cfg.RedisOptions.Addr)
	assert.Equal(t, 0, cfg.RedisOptions.DB)
	assert.Equal(t, sqliteDir, cfg.SQLiteDir)
	assert.NotEmpty(t, cfg.ArchivesDir)
	assert.Equal(t, "https://archiver.example.com", cfg.AppPublicURL)
	assert.Equal(t, "https://replay.example.com", cfg.ReplayPublicURL)
	assert.Equal(t, "10.0.0.0/8", cfg.TrustedProxies)
	assert.Equal(t, ":1080", cfg.AppAddress)
	assert.Equal(t, ":1081", cfg.ReplayAddress)
	assert.Equal(t, 10*time.Second, cfg.GracefulTimeout)

	assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
}

func TestLoadConfig_SQLiteDirDefaultsToArchivesDir(t *testing.T) {
	setAPIConfigEnv(t)

	cfg, err := loadConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.ArchivesDir, cfg.SQLiteDir)
}

func TestLoadConfig_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*testing.T)
		wantError string
	}{
		{
			name: "missing redis url",
			configure: func(t *testing.T) {
				setAPIConfigEnv(t)
				t.Setenv("REDIS_URL", "")
			},
			wantError: "REDIS_URL not set",
		},
		{
			name: "invalid redis url",
			configure: func(t *testing.T) {
				setAPIConfigEnv(t)
				t.Setenv("REDIS_URL", "not a redis url")
			},
			wantError: "invalid REDIS_URL",
		},
		{
			name: "missing archives dir",
			configure: func(t *testing.T) {
				setAPIConfigEnv(t)
				t.Setenv("ARCHIVES_DIR", "")
			},
			wantError: "ARCHIVES_DIR not set",
		},
		{
			name: "missing app public url",
			configure: func(t *testing.T) {
				setAPIConfigEnv(t)
				t.Setenv("APP_PUBLIC_URL", "")
			},
			wantError: "APP_PUBLIC_URL not set",
		},
		{
			name: "missing replay public url",
			configure: func(t *testing.T) {
				setAPIConfigEnv(t)
				t.Setenv("REPLAY_PUBLIC_URL", "")
			},
			wantError: "REPLAY_PUBLIC_URL not set",
		},
		{
			name: "same public origins",
			configure: func(t *testing.T) {
				setAPIConfigEnv(t)
				t.Setenv("REPLAY_PUBLIC_URL", "https://archiver.example.com")
			},
			wantError: "must use different origins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.configure(t)
			_, err := loadConfig()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestPublicOriginFromEnv(t *testing.T) {
	const name = "TEST_PUBLIC_ORIGIN"

	t.Run("normalizes trailing slash and default port", func(t *testing.T) {
		t.Setenv(name, "https://ARCHIVER.example.com:443/")
		origin, err := publicOriginFromEnv(name)
		require.NoError(t, err)
		assert.Equal(t, "https://archiver.example.com", origin)
	})

	t.Run("preserves non-default port", func(t *testing.T) {
		t.Setenv(name, "http://ARCHIVER.example.com:8080/")
		origin, err := publicOriginFromEnv(name)
		require.NoError(t, err)
		assert.Equal(t, "http://archiver.example.com:8080", origin)
	})

	for _, value := range []string{"", "archiver.example.com", "ftp://archiver.example.com", "https://user@archiver.example.com", "https://archiver.example.com/path", "https://archiver.example.com?query=1", "https://archiver.example.com#fragment"} {
		t.Run("rejects "+value, func(t *testing.T) {
			t.Setenv(name, value)
			_, err := publicOriginFromEnv(name)
			assert.Error(t, err)
		})
	}
}
