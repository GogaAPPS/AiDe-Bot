package bot

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/backend"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/maxapi"
)

type App struct {
	client  *maxapi.Client
	backend *backend.BackendClient
	logger  *slog.Logger
	backoff time.Duration
}

func New(settings config.Settings, logger *slog.Logger) (*App, error) {
	client, err := maxapi.NewClient(settings)
	if err != nil {
		return nil, err
	}
	backendClient, err := backend.NewClient(settings)
	if err != nil {
		return nil, err
	}

	return &App{
		client:  client,
		backend: backendClient,
		logger:  logger,
		backoff: time.Second,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.client.LogBotInfo(ctx, a.logger); err != nil {
		a.logger.Warn("bot info is unavailable", "error", err)
	}

	a.logger.Info("bot started")
	defer a.logger.Info("bot stopped")

	var marker int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, nextMarker, err := a.client.GetUpdates(ctx, marker)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}

			a.logger.Error("get updates", "error", err)
			if err := sleep(ctx, a.backoff); err != nil {
				return err
			}
			continue
		}

		marker = nextMarker
		for _, update := range updates {
			message, ok := maxapi.IncomingMessageFromUpdate(update)
			if !ok {
				continue
			}

			response, err := a.backend.SendMessage(ctx, message)
			if err != nil {
				a.logger.Error("backend message", "error", err)
				continue
			}
			if response.Status != domain.BackendStatusSuccess || response.Text == "" {
				continue
			}

			if err := a.client.SendText(ctx, response.Target, response.Text); err != nil {
				a.logger.Error("send message", "error", err, "chat_id", response.Target.ChatID, "user_id", response.Target.UserID)
			}
		}
	}
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
