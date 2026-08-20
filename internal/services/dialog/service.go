package dialog

import (
	"strings"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Handle(message domain.IncomingMessage) domain.OutgoingMessage {
	text := strings.TrimSpace(message.Text)
	if text == "" {
		return domain.OutgoingMessage{Target: message.Target}
	}

	if strings.HasPrefix(text, "/start") {
		return domain.OutgoingMessage{
			Text:   "Привет! Я AiDe.",
			Target: message.Target,
		}
	}

	return domain.OutgoingMessage{
		Text:   "Сообщение получил.",
		Target: message.Target,
	}
}
