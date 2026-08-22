package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	appconfig "github.com/JuanSaenz04/archiver/internal/config"
	"github.com/redis/go-redis/v9"
)

type apiConfig struct {
	RedisOptions    *redis.Options
	ArchivesDir     string
	SQLiteDir       string
	LogLevel        slog.Level
	AppPublicURL    string
	ReplayPublicURL string
	TrustedProxies  string
	AppAddress      string
	ReplayAddress   string
	GracefulTimeout time.Duration
}

func loadConfig() (apiConfig, error) {
	redisURL, err := appconfig.Required("REDIS_URL")
	if err != nil {
		return apiConfig{}, err
	}
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		return apiConfig{}, fmt.Errorf("invalid REDIS_URL: %w", err)
	}

	archivesDir, err := appconfig.Required("ARCHIVES_DIR")
	if err != nil {
		return apiConfig{}, err
	}

	sqliteDir := appconfig.Env("SQLITE_DIR")
	if sqliteDir == "" {
		sqliteDir = archivesDir
	}

	appPublicURL, err := publicOriginFromEnv("APP_PUBLIC_URL")
	if err != nil {
		return apiConfig{}, err
	}
	replayPublicURL, err := publicOriginFromEnv("REPLAY_PUBLIC_URL")
	if err != nil {
		return apiConfig{}, err
	}
	if appPublicURL == replayPublicURL {
		return apiConfig{}, errors.New("APP_PUBLIC_URL and REPLAY_PUBLIC_URL must use different origins")
	}

	return apiConfig{
		RedisOptions:    redisOptions,
		ArchivesDir:     archivesDir,
		SQLiteDir:       sqliteDir,
		LogLevel:        appconfig.LogLevel(appconfig.Env("LOG_LEVEL")),
		AppPublicURL:    appPublicURL,
		ReplayPublicURL: replayPublicURL,
		TrustedProxies:  appconfig.Env("TRUSTED_PROXIES"),
		AppAddress:      ":1080",
		ReplayAddress:   ":1081",
		GracefulTimeout: 10 * time.Second,
	}, nil
}

func publicOriginFromEnv(name string) (string, error) {
	value := appconfig.Env(name)
	if value == "" {
		return "", fmt.Errorf("environment variable %s not set", name)
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("environment variable %s must be an HTTP(S) origin without a path", name)
	}

	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	host := hostname
	if strings.Contains(hostname, ":") || port != "" {
		host = net.JoinHostPort(hostname, port)
		if port == "" {
			host = "[" + hostname + "]"
		}
	}

	return parsed.Scheme + "://" + host, nil
}
