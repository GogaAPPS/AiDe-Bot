package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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
			if callback, ok := maxapi.CallbackEventFromUpdate(update); ok {
				if err := a.handleCallback(ctx, callback); err != nil {
					a.logger.Error("handle callback", "error", err, "payload", callback.Payload)
				}
				continue
			}

			message, ok := maxapi.IncomingMessageFromUpdate(update)
			if !ok {
				continue
			}

			if isStartCommand(message.Text) {
				if err := a.client.SendMainMenu(ctx, message.Target); err != nil {
					a.logger.Error("send main menu", "error", err, "chat_id", message.Target.ChatID, "user_id", message.Target.UserID)
				}
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

func (a *App) handleCallback(ctx context.Context, callback maxapi.CallbackEvent) error {
	switch callback.Payload {
	case maxapi.CallbackNewChat:
		return a.handleNewChat(ctx, callback)
	case maxapi.CallbackDesignDevelop:
		return a.handleDesignDevelop(ctx, callback)
	default:
		a.logger.Warn("unknown callback", "payload", callback.Payload)
		return a.client.AnswerCallback(ctx, callback.ID, "Неизвестное действие")
	}
}

func (a *App) handleNewChat(ctx context.Context, callback maxapi.CallbackEvent) error {
	a.logger.Info("new chat button pressed", "chat_id", callback.Target.ChatID, "user_id", callback.Target.UserID)
	return a.client.AnswerCallback(ctx, callback.ID, "Новый чат: скоро добавим логику")
}

func (a *App) handleDesignDevelop(ctx context.Context, callback maxapi.CallbackEvent) error {
	a.logger.Info("design develop button pressed", "chat_id", callback.Target.ChatID, "user_id", callback.Target.UserID)
	return a.client.AnswerCallback(ctx, callback.ID, "Генерация дизайна: скоро добавим логику")
}

func isStartCommand(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "/start")
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
