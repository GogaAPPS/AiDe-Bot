package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	AppName        string
	AppEnv         string
	LogLevel       string
	MaxBotToken    string
	MaxAPIBaseURL  string
	RequestTimeout time.Duration
	PollingPause   time.Duration
	PollingTimeout time.Duration
}

func Load() (Settings, error) {
	settings := Settings{
		AppName:        getEnv("APP_NAME", "aide-bot"),
		AppEnv:         getEnv("APP_ENV", "local"),
		LogLevel:       getEnv("LOG_LEVEL", "INFO"),
		MaxBotToken:    strings.TrimSpace(os.Getenv("MAX_BOT_TOKEN")),
		MaxAPIBaseURL:  strings.TrimSpace(os.Getenv("MAX_API_BASE_URL")),
		RequestTimeout: getDuration("MAX_REQUEST_TIMEOUT", 10*time.Second),
		PollingPause:   getDuration("MAX_POLLING_PAUSE", 500*time.Millisecond),
		PollingTimeout: getDuration("MAX_POLLING_TIMEOUT", 30*time.Second),
	}

	if settings.MaxBotToken == "" {
		return Settings{}, errors.New("MAX_BOT_TOKEN is required")
	}

	return settings, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return time.Duration(seconds) * time.Second
}
