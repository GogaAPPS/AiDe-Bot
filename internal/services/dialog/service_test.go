package dialog

import (
	"testing"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

func TestHandleStartCommand(t *testing.T) {
	service := NewService()

	response := service.Handle(domain.IncomingMessage{
		Text: "/start",
		Target: domain.Target{
			ChatID: 1,
		},
	})

	if response.Text == "" {
		t.Fatal("expected response text")
	}
	if response.Target.ChatID != 1 {
		t.Fatalf("unexpected target: %+v", response.Target)
	}
}
