package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	AppName                 string
	AppEnv                  string
	LogLevel                string
	MaxBotToken             string
	MaxAPIBaseURL           string
	BackendAPIBaseURL       string
	BackendMessagesPath     string
	BackendClearHistoryPath string
	BackendStub             bool
	BackendRequestTimeout   time.Duration
	RequestTimeout          time.Duration
	PollingPause            time.Duration
	PollingTimeout          time.Duration
}

func Load() (Settings, error) {
	settings := Settings{
		AppName:                 getEnv("APP_NAME", "aide-bot"),
		AppEnv:                  getEnv("APP_ENV", "local"),
		LogLevel:                getEnv("LOG_LEVEL", "INFO"),
		MaxBotToken:             strings.TrimSpace(os.Getenv("MAX_BOT_TOKEN")),
		MaxAPIBaseURL:           strings.TrimSpace(os.Getenv("MAX_API_BASE_URL")),
		BackendAPIBaseURL:       strings.TrimSpace(os.Getenv("BACKEND_API_BASE_URL")),
		BackendMessagesPath:     getEnv("BACKEND_MESSAGES_PATH", ""),
		BackendClearHistoryPath: getEnv("BACKEND_CLEAR_HISTORY_PATH", ""),
		BackendStub:             getBool("BACKEND_STUB", true),
		BackendRequestTimeout:   getDuration("BACKEND_REQUEST_TIMEOUT", 10*time.Second),
		RequestTimeout:          getDuration("MAX_REQUEST_TIMEOUT", 45*time.Second),
		PollingPause:            getDuration("MAX_POLLING_PAUSE", 500*time.Millisecond),
		PollingTimeout:          getDuration("MAX_POLLING_TIMEOUT", 30*time.Second),
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

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
