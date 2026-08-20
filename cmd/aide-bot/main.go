package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	botapp "github.com/GogaAPPS/AiDe-Bot/internal/apps/bot"
	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	applogging "github.com/GogaAPPS/AiDe-Bot/internal/core/logging"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := applogging.New(settings.LogLevel)
	app, err := botapp.New(settings, logger)
	if err != nil {
		logger.Error("create app", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("run app", "error", err)
		os.Exit(1)
	}
}
