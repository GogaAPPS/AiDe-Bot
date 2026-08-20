package maxapi

import (
	"strings"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

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
