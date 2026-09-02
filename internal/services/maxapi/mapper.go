package maxapi

import (
	"strings"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

type CallbackEvent struct {
	ID      string
	Payload string
	Target  domain.Target
}

func IncomingMessageFromUpdate(update model.Update) (domain.IncomingMessage, bool) {
	if update.UpdateType != model.UpdateMessageCreated {
		return domain.IncomingMessage{}, false
	}

	text := strings.TrimSpace(update.GetMessage().Body.Text)
	return domain.IncomingMessage{
		Text:      text,
		MessageID: update.MessageID,
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
		ID:      update.Callback.CallbackID,
		Payload: strings.TrimSpace(update.Callback.Payload),
		Target: domain.Target{
			ChatID: update.ChatID,
			UserID: update.UserID,
		},
	}, true
}
