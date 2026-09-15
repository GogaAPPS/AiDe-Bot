package config

import (
	"testing"
	"time"
)

func TestLoadRequiresToken(t *testing.T) {
	t.Setenv("MAX_BOT_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected token validation error")
	}
}

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("MAX_BOT_TOKEN", "token")

	settings, err := Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}

	if settings.AppName != "aide-bot" {
		t.Fatalf("unexpected app name: %s", settings.AppName)
	}
	if settings.RequestTimeout != 45*time.Second {
		t.Fatalf("unexpected request timeout: %s", settings.RequestTimeout)
	}
	if settings.BackendMessagesPath != "" {
		t.Fatalf("unexpected backend messages path: %s", settings.BackendMessagesPath)
	}
	if settings.BackendClearHistoryPath != "" {
		t.Fatalf("unexpected backend clear history path: %s", settings.BackendClearHistoryPath)
	}
	if !settings.BackendStub {
		t.Fatal("expected backend stub to be enabled by default")
	}
}

func TestLoadUsesConfiguredClearHistoryPath(t *testing.T) {
	t.Setenv("MAX_BOT_TOKEN", "token")
	t.Setenv("BACKEND_CLEAR_HISTORY_PATH", "/custom/clear-history")

	settings, err := Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if settings.BackendClearHistoryPath != "/custom/clear-history" {
		t.Fatalf("unexpected backend clear history path: %s", settings.BackendClearHistoryPath)
	}
}
