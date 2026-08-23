package main

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	appconfig "github.com/JuanSaenz04/archiver/internal/config"
	"github.com/JuanSaenz04/archiver/internal/crawler"
	"github.com/JuanSaenz04/archiver/internal/worker"
	"github.com/redis/go-redis/v9"
)

type workerConfig struct {
	RedisOptions   *redis.Options
	ArchivesDir    string
	SQLiteDir      string
	LogLevel       slog.Level
	CrawlerTimeout int
	AnubisMode     crawler.AnubisMode
	ConsumerName   string
}

func loadConfig() (workerConfig, error) {
	redisURL, err := appconfig.Required("REDIS_URL")
	if err != nil {
		return workerConfig{}, err
	}
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		return workerConfig{}, fmt.Errorf("invalid REDIS_URL: %w", err)
	}

	archivesDir, err := appconfig.Required("ARCHIVES_DIR")
	if err != nil {
		return workerConfig{}, err
	}

	sqliteDir := appconfig.Env("SQLITE_DIR")
	if sqliteDir == "" {
		sqliteDir = archivesDir
	}

	crawlerTimeout, err := appconfig.Int("CRAWLER_TIMEOUT", 90)
	if err != nil {
		return workerConfig{}, err
	}

	anubisMode, err := parseAnubisMode(appconfig.Env("ANUBIS_MODE"))
	if err != nil {
		return workerConfig{}, err
	}

	return workerConfig{
		RedisOptions:   redisOptions,
		ArchivesDir:    archivesDir,
		SQLiteDir:      sqliteDir,
		LogLevel:       appconfig.LogLevel(appconfig.Env("LOG_LEVEL")),
		CrawlerTimeout: crawlerTimeout,
		AnubisMode:     anubisMode,
		ConsumerName:   worker.GetWorkerName(appconfig.Env("CONSUMER_NAME"), appconfig.Env("HOSTNAME")),
	}, nil
}

func parseAnubisMode(value string) (crawler.AnubisMode, error) {
	mode := crawler.AnubisMode(strings.ToLower(strings.TrimSpace(value)))
	if mode == "" {
		return crawler.AnubisModeAuto, nil
	}

	switch mode {
	case crawler.AnubisModeAuto, crawler.AnubisModeAlways, crawler.AnubisModeOff:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid ANUBIS_MODE %q: expected auto, always, or off", value)
	}
}

func databasePath(cfg workerConfig) string {
	return filepath.Join(cfg.SQLiteDir, "archive.db")
}
