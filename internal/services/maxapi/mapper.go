package maxapi

import (
	"strings"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

type CallbackEvent struct {
	ID        string
	MessageID string
	Payload   string
	Target    domain.Target
}

func IncomingMessageFromUpdate(update model.Update) (domain.IncomingMessage, bool) {
	if update.UpdateType != model.UpdateMessageCreated {
		return domain.IncomingMessage{}, false
	}

	body := update.GetMessage().Body
	attachments := make([]domain.IncomingAttachment, 0, len(body.Attachments))
	for _, attachment := range body.Attachments {
		attachments = append(attachments, domain.IncomingAttachment{
			Type: string(attachment.Type), URL: attachment.Payload.URL,
		})
	}
	text := strings.TrimSpace(body.Text)
	return domain.IncomingMessage{
		Text:        text,
		Attachments: attachments,
		MessageID:   update.MessageID,
		Target: domain.Target{
			ChatID: update.ChatID,
			UserID: update.UserID,
		},
	}, true
}

func CallbackEventFromUpdate(update model.Update) (CallbackEvent, bool) {
	if update.UpdateType != model.UpdateMessageCallback || update.Callback == nil {
		return CallbackEvent{}, false
	}

	return CallbackEvent{
		ID:        update.Callback.CallbackID,
		MessageID: update.MessageID,
		Payload:   strings.TrimSpace(update.Callback.Payload),
		Target: domain.Target{
			ChatID: update.ChatID,
			UserID: update.UserID,
		},
	}, true
}
