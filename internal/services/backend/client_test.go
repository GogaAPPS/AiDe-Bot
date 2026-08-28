package backend

import (
	"context"
	"testing"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

func TestSendMessageStub(t *testing.T) {
	client, err := NewClient(config.Settings{BackendStub: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	response, err := client.SendMessage(context.Background(), domain.IncomingMessage{Text: "/start"})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if response.Text != "Привет! Я AiDe." {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Status != domain.BackendStatusSuccess || client.State() != domain.BackendStatusSuccess {
		t.Fatalf("unexpected status: response=%s client=%s", response.Status, client.State())
	}
}

func TestSendMessageStubHasOptionalDocument(t *testing.T) {
	client, err := NewClient(config.Settings{BackendStub: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	response, err := client.SendMessage(context.Background(), domain.IncomingMessage{Text: "подготовь ответ"})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if response.File != nil {
		t.Fatalf("expected no document in this response: %+v", response.File)
	}
}

func TestSendMessageContextError(t *testing.T) {
	client, err := NewClient(config.Settings{BackendStub: true})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response, err := client.SendMessage(ctx, domain.IncomingMessage{})
	if err == nil || response.Status != domain.BackendStatusError || client.State() != domain.BackendStatusError {
		t.Fatalf("expected backend error: response=%+v error=%v status=%s", response, err, client.State())
	}
}
