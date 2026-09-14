package maxapi

import (
	"context"
	"fmt"
	"io"
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

func (c *Client) SendTextWithMenu(ctx context.Context, target domain.Target, text string) error {
	message := maxbot.NewMessage().SetText(text).AddKeyboard(newMenuKeyboard())
	if target.ChatID != 0 {
		message.SetChat(target.ChatID)
	}
	if target.UserID != 0 {
		message.SetUser(target.UserID)
	}

	_, err := c.api.Messages.Send(ctx, message)
	return err
}

func (c *Client) SendFile(ctx context.Context, target domain.Target, name string, reader io.Reader, size int64) error {
	return c.sendFile(ctx, target, name, reader, size, false)
}

func (c *Client) SendFileWithMenu(ctx context.Context, target domain.Target, name string, reader io.Reader, size int64) error {
	return c.sendFile(ctx, target, name, reader, size, true)
}

func (c *Client) sendFile(ctx context.Context, target domain.Target, name string, reader io.Reader, size int64, withMenu bool) error {
	if reader == nil {
		return fmt.Errorf("file reader is nil")
	}
	if size < 0 {
		return fmt.Errorf("file size is negative: %d", size)
	}

	token, err := c.api.Upload.Upload(ctx, model.UploadFile, reader, name, size)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	if token == "" {
		return fmt.Errorf("upload file returned an empty token")
	}

	message := maxbot.NewMessage().AddAttachByToken(token, model.AttachFile)
	if withMenu {
		message.AddKeyboard(newMenuKeyboard())
	}
	if target.ChatID != 0 {
		message.SetChat(target.ChatID)
	}
	if target.UserID != 0 {
		message.SetUser(target.UserID)
	}

	_, err = c.api.Messages.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("send file message: %w", err)
	}

	return nil
}

func (c *Client) SendMainMenu(ctx context.Context, target domain.Target) error {
	message := maxbot.NewMessage().
		SetText("Выберите действие:").
		AddKeyboard(newMainMenuKeyboard())
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

func (c *Client) DeleteMessage(ctx context.Context, messageID string) error {
	if messageID == "" {
		return fmt.Errorf("message id is empty")
	}

	_, err := c.api.Messages.DeleteMessage(ctx, messageID)
	return err
}
