package main

import (
	"log/slog"
	"testing"

	"github.com/JuanSaenz04/archiver/internal/crawler"
	"github.com/stretchr/testify/require"
)

func setWorkerConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("ARCHIVES_DIR", t.TempDir())
	t.Setenv("SQLITE_DIR", "")
	t.Setenv("CRAWLER_TIMEOUT", "")
	t.Setenv("JOB_TIMEOUT", "")
	t.Setenv("ANUBIS_MODE", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("CONSUMER_NAME", "")
	t.Setenv("HOSTNAME", "")
}

func TestLoadConfig(t *testing.T) {
	setWorkerConfigEnv(t)
	sqliteDir := t.TempDir()
	t.Setenv("SQLITE_DIR", sqliteDir)
	t.Setenv("CRAWLER_TIMEOUT", "45")
	t.Setenv("ANUBIS_MODE", "always")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("CONSUMER_NAME", "configured-worker")
	t.Setenv("HOSTNAME", "ignored-host")

	cfg, err := loadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg.RedisOptions)

	require.Equal(t, "localhost:6379", cfg.RedisOptions.Addr)
	require.Equal(t, sqliteDir, cfg.SQLiteDir)
	require.Equal(t, 45, cfg.CrawlerTimeout)
	require.Equal(t, crawler.AnubisModeAlways, cfg.AnubisMode)
	require.Equal(t, "configured-worker", cfg.ConsumerName)
	require.Equal(t, slog.LevelWarn, cfg.LogLevel)
	require.Equal(t, sqliteDir+"/archive.db", databasePath(cfg))
}

func TestLoadConfig_DefaultCrawlerTimeout(t *testing.T) {
	setWorkerConfigEnv(t)

	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, 90, cfg.CrawlerTimeout)
	require.Equal(t, cfg.ArchivesDir, cfg.SQLiteDir)
	require.Equal(t, crawler.AnubisModeAuto, cfg.AnubisMode)
}

func TestLoadConfig_ConsumerNameUsesHostnameFallback(t *testing.T) {
	setWorkerConfigEnv(t)
	t.Setenv("HOSTNAME", "worker-host")

	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, "worker-worker-host", cfg.ConsumerName)
}

func TestParseAnubisMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    crawler.AnubisMode
		wantErr bool
	}{
		{name: "empty defaults to auto", want: crawler.AnubisModeAuto},
		{name: "auto", value: "auto", want: crawler.AnubisModeAuto},
		{name: "always case insensitive", value: " ALWAYS ", want: crawler.AnubisModeAlways},
		{name: "off", value: "off", want: crawler.AnubisModeOff},
		{name: "invalid", value: "enabled", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAnubisMode(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
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
				setWorkerConfigEnv(t)
				t.Setenv("REDIS_URL", "")
			},
			wantError: "REDIS_URL not set",
		},
		{
			name: "invalid redis url",
			configure: func(t *testing.T) {
				setWorkerConfigEnv(t)
				t.Setenv("REDIS_URL", "not a redis url")
			},
			wantError: "invalid REDIS_URL",
		},
		{
			name: "missing archives dir",
			configure: func(t *testing.T) {
				setWorkerConfigEnv(t)
				t.Setenv("ARCHIVES_DIR", "")
			},
			wantError: "ARCHIVES_DIR not set",
		},
		{
			name: "invalid crawler timeout",
			configure: func(t *testing.T) {
				setWorkerConfigEnv(t)
				t.Setenv("CRAWLER_TIMEOUT", "not-an-integer")
			},
			wantError: "CRAWLER_TIMEOUT",
		},
		{
			name: "invalid anubis mode",
			configure: func(t *testing.T) {
				setWorkerConfigEnv(t)
				t.Setenv("ANUBIS_MODE", "sometimes")
			},
			wantError: "invalid ANUBIS_MODE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.configure(t)
			_, err := loadConfig()
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestLoadConfig_AcceptsNegativeIntegerCrawlerTimeout(t *testing.T) {
	setWorkerConfigEnv(t)
	t.Setenv("CRAWLER_TIMEOUT", "-1")

	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, -1, cfg.CrawlerTimeout)
}

func TestLoadConfig_JobTimeout(t *testing.T) {
	for _, value := range []string{"0", "-1", "invalid"} {
		t.Run(value, func(t *testing.T) {
			setWorkerConfigEnv(t)
			t.Setenv("JOB_TIMEOUT", value)
			_, err := loadConfig()
			require.ErrorContains(t, err, "JOB_TIMEOUT")
		})
	}
	setWorkerConfigEnv(t)
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, 3600, cfg.JobTimeout)
}
