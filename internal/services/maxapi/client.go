package maxapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	maxbot "github.com/max-messenger/max-bot-api-client-go/v2"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

type Client struct {
	api *maxbot.Api
}

func NewClient(settings config.Settings) (*Client, error) {
	options := []maxbot.Opt{
		maxbot.WithHTTPClient(&http.Client{Timeout: settings.RequestTimeout}),
		maxbot.WithPollingPause(settings.PollingPause),
		maxbot.WithPollingTimeout(settings.PollingTimeout),
	}

	if settings.MaxAPIBaseURL != "" {
		options = append(options, maxbot.WithBaseURL(settings.MaxAPIBaseURL))
	}

	api, err := maxbot.NewApi(settings.MaxBotToken, options...)
	if err != nil {
		return nil, err
	}

	return &Client{api: api}, nil
}

func (c *Client) LogBotInfo(ctx context.Context, logger *slog.Logger) error {
	info, err := c.api.Bots.GetMyInfo(ctx)
	if err != nil {
		return err
	}

	logger.Info("bot info loaded", "bot_id", info.UserID, "first_name", info.FirstName, "username", info.Username)
	return nil
}

func (c *Client) GetUpdates(ctx context.Context, marker int64) ([]model.Update, int64, error) {
	return c.api.Subscriptions.GetUpdates(ctx, marker)
}

func (c *Client) SendText(ctx context.Context, target domain.Target, text string) error {
	message := maxbot.NewMessage().SetText(text)
	if target.ChatID != 0 {
		message.SetChat(target.ChatID)
	}
	if target.UserID != 0 {
		message.SetUser(target.UserID)
	}

	_, err := c.api.Messages.Send(ctx, message)
	return err
}

func (c *Client) SendMainMenu(ctx context.Context, target domain.Target) error {
	keyboard := model.NewKeyboard()
	keyboard.
		AddRow().
		AddCallback("Новый чат", model.IntentDefault, CallbackNewChat)
	keyboard.
		AddRow().
		AddCallback("Генерация дизайна(Develop)", model.IntentDefault, CallbackDesignDevelop)

	message := maxbot.NewMessage().
		SetText("Выберите действие:").
		AddKeyboard(keyboard)
	if target.ChatID != 0 {
		message.SetChat(target.ChatID)
	}
	if target.UserID != 0 {
		message.SetUser(target.UserID)
	}

	_, err := c.api.Messages.Send(ctx, message)
	return err
}

func (c *Client) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	notification := text
	_, err := c.api.Messages.AnswerOnCallback(ctx, callbackID, model.CallbackAnswer{
		Notification: &notification,
	})
	return err
}
