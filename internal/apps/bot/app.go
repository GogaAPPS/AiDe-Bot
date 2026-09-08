package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	applogging "github.com/GogaAPPS/AiDe-Bot/internal/core/logging"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/backend"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/inputfilter"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/maxapi"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

type App struct {
	client  *maxapi.Client
	backend *backend.BackendClient
	filter  *inputfilter.Filter
	logger  *slog.Logger
	backoff time.Duration
}

func New(settings config.Settings, logger *slog.Logger) (*App, error) {
	client, err := maxapi.NewClient(settings)
	if err != nil {
		return nil, err
	}
	backendClient, err := backend.NewClient(settings, logger)
	if err != nil {
		return nil, err
	}

	return &App{
		client:  client,
		backend: backendClient,
		filter:  inputfilter.New(inputfilter.DefaultOptions()),
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
			updateTraceID := applogging.NewTraceID(update.MessageID, update.ChatID, update.UserID)
			updateCtx := applogging.WithTraceID(ctx, updateTraceID)
			a.logUpdate(updateCtx, update, "bot.update.received", "bot_update_received", applogging.DirectionIncoming, slog.Any(applogging.FieldBody, update))

			message, ok := maxapi.IncomingMessageFromUpdate(update)
			if !ok {
				a.logUpdate(updateCtx, update, "bot.update.skipped", "bot_update_skipped", applogging.DirectionInternal,
					slog.String(applogging.FieldStatus, "skipped"),
					slog.String("reason", "unsupported_update_type"),
					slog.Any(applogging.FieldBody, update),
				)
				continue
			}

			messageCtx := applogging.WithTraceID(ctx, updateTraceID)
			a.logMessage(messageCtx, message, slog.LevelInfo, "bot message mapped", "bot.message.mapped", "bot_message_mapped", applogging.DirectionInternal,
				slog.String(applogging.FieldStatus, "ok"),
				slog.Any(applogging.FieldBody, message),
			)

			if isStartCommand(message.Text) {
				if err := a.client.SendMainMenu(messageCtx, message.Target); err != nil {
					a.logger.Error("send main menu", "error", err, "chat_id", message.Target.ChatID, "user_id", message.Target.UserID)
				}
				continue
			}

			filtered := a.filter.Check(message)
			if !filtered.Accepted {
				a.logMessage(messageCtx, message, slog.LevelInfo, "bot message rejected", "bot.message.rejected", "bot_message_rejected", applogging.DirectionInternal,
					slog.String(applogging.FieldStatus, "rejected"),
					slog.String("reason", filtered.Explanation),
				)
				if err := a.client.SendText(messageCtx, filtered.Message.Target, filtered.Explanation); err != nil {
					a.logger.Error("send filter explanation", "error", err, "chat_id", message.Target.ChatID, "user_id", message.Target.UserID)
				}
				continue
			}
			message = filtered.Message

			a.logMessage(messageCtx, message, slog.LevelInfo, "bot backend route selected", "bot.backend.route", "bot_backend_route", applogging.DirectionOutgoing,
				slog.String(applogging.FieldStatus, "selected"),
				slog.String(applogging.FieldRoute, "backend.process"),
			)

			response, err := a.backend.SendMessage(messageCtx, message)
			if err != nil {
				a.logMessage(messageCtx, message, slog.LevelError, "backend message", "bot.backend.failed", "bot_backend_failed", applogging.DirectionInternal,
					slog.String(applogging.FieldStatus, string(domain.BackendStatusError)),
					slog.String(applogging.FieldError, err.Error()),
				)
				continue
			}
			if response.Status != domain.BackendStatusSuccess || response.Text == "" {
				a.logMessage(messageCtx, message, slog.LevelInfo, "bot reply skipped", "bot.reply.skipped", "bot_reply_skipped", applogging.DirectionInternal,
					slog.String(applogging.FieldStatus, string(response.Status)),
					slog.Any(applogging.FieldBody, response),
				)
				continue
			}

			a.logMessage(messageCtx, message, slog.LevelInfo, "bot reply sending", "bot.reply.sending", "bot_reply_sending", applogging.DirectionOutgoing,
				slog.String(applogging.FieldStatus, "sending"),
				slog.Any(applogging.FieldBody, domain.OutgoingMessage{Text: response.Text, Target: response.Target}),
			)
			if err := a.client.SendText(messageCtx, response.Target, response.Text); err != nil {
				a.logMessage(messageCtx, message, slog.LevelError, "send message", "bot.reply.failed", "bot_reply_failed", applogging.DirectionOutgoing,
					slog.String(applogging.FieldStatus, string(domain.BackendStatusError)),
					slog.String(applogging.FieldError, err.Error()),
					slog.Any(applogging.FieldBody, domain.OutgoingMessage{Text: response.Text, Target: response.Target}),
				)
				continue
			}
			a.logMessage(messageCtx, message, slog.LevelInfo, "bot reply sent", "bot.reply.sent", "bot_reply_sent", applogging.DirectionOutgoing,
				slog.String(applogging.FieldStatus, "ok"),
				slog.Any(applogging.FieldBody, domain.OutgoingMessage{Text: response.Text, Target: response.Target}),
			)
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

// Заглушки для базовых обработчиков колбэков

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

func (a *App) logUpdate(ctx context.Context, update model.Update, event, stage, direction string, extra ...slog.Attr) {
	traceID := applogging.TraceID(ctx, update.MessageID, update.ChatID, update.UserID)
	attrs := applogging.TemplateAttrs(event, stage, direction, traceID, update.MessageID, update.ChatID, update.UserID)
	attrs = append(attrs, extra...)
	a.logger.LogAttrs(ctx, slog.LevelInfo, "bot update path", attrs...)
}

func (a *App) logMessage(
	ctx context.Context,
	message domain.IncomingMessage,
	level slog.Level,
	logMessage string,
	event string,
	stage string,
	direction string,
	extra ...slog.Attr,
) {
	traceID := applogging.TraceID(ctx, message.MessageID, message.Target.ChatID, message.Target.UserID)
	attrs := applogging.TemplateAttrs(event, stage, direction, traceID, message.MessageID, message.Target.ChatID, message.Target.UserID)
	attrs = append(attrs, extra...)
	a.logger.LogAttrs(ctx, level, logMessage, attrs...)
}
