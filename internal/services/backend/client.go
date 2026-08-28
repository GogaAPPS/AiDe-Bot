package backend

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

type BackendClient struct {
	baseURL    string
	httpClient *http.Client
	stub       bool

	mu     sync.RWMutex
	status domain.BackendStatus
}

func NewClient(settings config.Settings) (*BackendClient, error) {
	if !settings.BackendStub && strings.TrimSpace(settings.BackendAPIBaseURL) == "" {
		return nil, errors.New("BACKEND_API_BASE_URL is required when BACKEND_STUB=false")
	}

	return &BackendClient{
		baseURL:    strings.TrimRight(settings.BackendAPIBaseURL, "/"),
		httpClient: &http.Client{Timeout: settings.BackendRequestTimeout},
		stub:       settings.BackendStub,
		status:     domain.BackendStatusSuccess,
	}, nil
}

func (c *BackendClient) State() domain.BackendStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *BackendClient) setState(status domain.BackendStatus) {
	c.mu.Lock()
	c.status = status
	c.mu.Unlock()
}

func (c *BackendClient) SendMessage(ctx context.Context, message domain.IncomingMessage) (domain.BackendMessage, error) {
	c.setState(domain.BackendStatusLoading)
	if err := ctx.Err(); err != nil {
		return c.errorMessage(message, err)
	}
	if !c.stub {
		return c.errorMessage(message, errors.New("backend HTTP transport is not implemented"))
	}

	responseText := "Сообщение получил."
	if strings.HasPrefix(strings.TrimSpace(message.Text), "/start") {
		responseText = "Привет! Я AiDe."
	}

	c.setState(domain.BackendStatusSuccess)
	return domain.BackendMessage{
		Text: responseText, MessageID: message.MessageID, Target: message.Target,
		Status: domain.BackendStatusSuccess,
	}, nil
}

func (c *BackendClient) Consult(ctx context.Context, request ConsultationRequest) (ConsultationResponse, error) {
	message, err := c.SendMessage(ctx, request.Message)
	return ConsultationResponse{Message: message}, err
}

func (c *BackendClient) errorMessage(message domain.IncomingMessage, err error) (domain.BackendMessage, error) {
	c.setState(domain.BackendStatusError)
	return domain.BackendMessage{MessageID: message.MessageID, Target: message.Target, Status: domain.BackendStatusError, Error: err.Error()}, err
}
