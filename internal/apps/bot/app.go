package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	client  botClient
	backend botBackend
	filter  *inputfilter.Filter
	logger  *slog.Logger
	backoff time.Duration
}

type botClient interface {
	LogBotInfo(context.Context, *slog.Logger) error
	GetUpdates(context.Context, int64) ([]model.Update, int64, error)
	SendText(context.Context, domain.Target, string) error
	SendTextWithMenu(context.Context, domain.Target, string) error
	SendFile(context.Context, domain.Target, string, io.Reader, int64) error
	SendFileWithMenu(context.Context, domain.Target, string, io.Reader, int64) error
	SendMainMenu(context.Context, domain.Target) error
	AnswerCallback(context.Context, string, string) error
	DeleteMessage(context.Context, string) error
}

type botBackend interface {
	SendMessage(context.Context, domain.IncomingMessage) (domain.BackendMessage, error)
	DownloadFile(context.Context, domain.BackendFile) (backend.FileDownload, error)
}

const documentDeliveryError = "Не удалось прикрепить документ к сообщению. Попробуйте повторить запрос позже."

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
				if err := a.client.SendTextWithMenu(messageCtx, filtered.Message.Target, filtered.Explanation); err != nil {
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
			if response.Status != domain.BackendStatusSuccess || (response.Text == "" && response.File == nil) {
				a.logMessage(messageCtx, message, slog.LevelInfo, "bot reply skipped", "bot.reply.skipped", "bot_reply_skipped", applogging.DirectionInternal,
					slog.String(applogging.FieldStatus, string(response.Status)),
					slog.Any(applogging.FieldBody, response),
				)
				continue
			}

			a.sendBackendResponse(messageCtx, message, response)
		}
	}
}

func (a *App) sendBackendResponse(ctx context.Context, message domain.IncomingMessage, response domain.BackendMessage) {
	if response.Text != "" {
		outgoing := domain.OutgoingMessage{Text: response.Text, Target: response.Target}
		a.logMessage(ctx, message, slog.LevelInfo, "bot reply sending", "bot.reply.sending", "bot_reply_sending", applogging.DirectionOutgoing,
			slog.String(applogging.FieldStatus, "sending"),
			slog.Any(applogging.FieldBody, outgoing),
		)
		sendText := a.client.SendText
		if response.File == nil {
			sendText = a.client.SendTextWithMenu
		}
		if err := sendText(ctx, response.Target, response.Text); err != nil {
			a.logMessage(ctx, message, slog.LevelError, "send message", "bot.reply.failed", "bot_reply_failed", applogging.DirectionOutgoing,
				slog.String(applogging.FieldStatus, string(domain.BackendStatusError)),
				slog.String(applogging.FieldError, err.Error()),
				slog.Any(applogging.FieldBody, outgoing),
			)
		} else {
			a.logMessage(ctx, message, slog.LevelInfo, "bot reply sent", "bot.reply.sent", "bot_reply_sent", applogging.DirectionOutgoing,
				slog.String(applogging.FieldStatus, "ok"),
				slog.Any(applogging.FieldBody, outgoing),
			)
		}
	}

	if response.File == nil {
		return
	}

	download, err := a.backend.DownloadFile(ctx, *response.File)
	if err != nil {
		a.logDocumentFailure(ctx, message, err)
		a.sendDocumentFailure(ctx, response.Target)
		return
	}
	if download.Body == nil {
		a.logDocumentFailure(ctx, message, errors.New("document download body is nil"))
		a.sendDocumentFailure(ctx, response.Target)
		return
	}

	err = a.client.SendFileWithMenu(ctx, response.Target, response.File.Name, download.Body, download.Size)
	closeErr := download.Body.Close()
	if err != nil {
		a.logDocumentFailure(ctx, message, err)
		a.sendDocumentFailure(ctx, response.Target)
		return
	}
	if closeErr != nil {
		a.logDocumentFailure(ctx, message, fmt.Errorf("close document download: %w", closeErr))
		return
	}

	a.logMessage(ctx, message, slog.LevelInfo, "bot document sent", "bot.document.sent", "bot_document_sent", applogging.DirectionOutgoing,
		slog.String(applogging.FieldStatus, "ok"),
		slog.String("file_name", response.File.Name),
		slog.Int64("file_size", download.Size),
	)
}

func (a *App) sendDocumentFailure(ctx context.Context, target domain.Target) {
	if err := a.client.SendTextWithMenu(ctx, target, documentDeliveryError); err != nil {
		a.logger.Error("send document failure notification", "error", err, "chat_id", target.ChatID, "user_id", target.UserID)
	}
}

func (a *App) logDocumentFailure(ctx context.Context, message domain.IncomingMessage, err error) {
	a.logMessage(ctx, message, slog.LevelError, "document delivery failed", "bot.document.failed", "bot_document_failed", applogging.DirectionInternal,
		slog.String(applogging.FieldStatus, string(domain.BackendStatusError)),
		slog.String(applogging.FieldError, err.Error()),
	)
}

func (a *App) handleCallback(ctx context.Context, callback maxapi.CallbackEvent) error {
	switch callback.Payload {
	case maxapi.CallbackMainMenu:
		return a.handleMainMenu(ctx, callback)
	case maxapi.CallbackBack:
		return a.handleBack(ctx, callback)
	case maxapi.CallbackNewChat:
		return a.handleNewChat(ctx, callback)
	case maxapi.CallbackDesignDevelop:
		return a.handleDesignDevelop(ctx, callback)
	default:
		a.logger.Warn("unknown callback", "payload", callback.Payload)
		return a.client.AnswerCallback(ctx, callback.ID, "Неизвестное действие")
	}
}

func (a *App) handleMainMenu(ctx context.Context, callback maxapi.CallbackEvent) error {
	if err := a.client.AnswerCallback(ctx, callback.ID, ""); err != nil {
		return err
	}

	return a.client.SendMainMenu(ctx, callback.Target)
}

func (a *App) handleBack(ctx context.Context, callback maxapi.CallbackEvent) error {
	if err := a.client.AnswerCallback(ctx, callback.ID, ""); err != nil {
		return err
	}

	return a.client.DeleteMessage(ctx, callback.MessageID)
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
