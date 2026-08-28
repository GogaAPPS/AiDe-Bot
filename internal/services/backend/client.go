package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

type BackendClient struct {
	baseURL     string
	messagesURL string
	httpClient  *http.Client
	stub        bool

	mu     sync.RWMutex
	status domain.BackendStatus
}

func NewClient(settings config.Settings) (*BackendClient, error) {
	if !settings.BackendStub && strings.TrimSpace(settings.BackendAPIBaseURL) == "" {
		return nil, errors.New("BACKEND_API_BASE_URL is required when BACKEND_STUB=false")
	}

	return &BackendClient{
		baseURL:     strings.TrimRight(settings.BackendAPIBaseURL, "/"),
		messagesURL: strings.TrimRight(settings.BackendAPIBaseURL, "/") + "/" + strings.TrimLeft(settings.BackendMessagesPath, "/"),
		httpClient:  &http.Client{Timeout: settings.BackendRequestTimeout},
		stub:        settings.BackendStub,
		status:      domain.BackendStatusSuccess,
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
	if c.stub {
		return c.sendMessageStub(message)
	}

	return c.sendMessageHTTP(ctx, message)
}

func (c *BackendClient) sendMessageStub(message domain.IncomingMessage) (domain.BackendMessage, error) {
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

func (c *BackendClient) sendMessageHTTP(ctx context.Context, message domain.IncomingMessage) (domain.BackendMessage, error) {
	payload, err := json.Marshal(sendMessageRequest{
		MessageID: message.MessageID,
		Text:      message.Text,
		ChatID:    message.Target.ChatID,
		UserID:    message.Target.UserID,
	})
	if err != nil {
		return c.errorMessage(message, fmt.Errorf("marshal backend request: %w", err))
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL, bytes.NewReader(payload))
	if err != nil {
		return c.errorMessage(message, fmt.Errorf("create backend request: %w", err))
	}
	request.Header.Set("Content-Type", "application/json")

	httpResponse, err := c.httpClient.Do(request)
	if err != nil {
		return c.errorMessage(message, fmt.Errorf("send backend request: %w", err))
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return c.errorMessage(message, fmt.Errorf("backend returned HTTP status %d", httpResponse.StatusCode))
	}

	var responseBody sendMessageResponse
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, 1<<20))
	if err := decoder.Decode(&responseBody); err != nil {
		return c.errorMessage(message, fmt.Errorf("decode backend response: %w", err))
	}

	if responseBody.Status != string(domain.BackendStatusSuccess) {
		backendError := responseBody.Error
		if backendError == "" {
			backendError = "backend returned unsuccessful response"
		}
		return c.errorMessage(message, errors.New(backendError))
	}
	if responseBody.Text == "" && responseBody.File == nil {
		return c.errorMessage(message, errors.New("backend response contains neither text nor file"))
	}

	response := domain.BackendMessage{
		Text: responseBody.Text, MessageID: message.MessageID, Target: message.Target,
		Status: domain.BackendStatusSuccess,
	}
	if responseBody.File != nil {
		response.File = &domain.BackendFile{
			Name: responseBody.File.Name, ContentType: responseBody.File.ContentType, Data: responseBody.File.Data,
		}
	}

	c.setState(domain.BackendStatusSuccess)
	return response, nil
}

func (c *BackendClient) Consult(ctx context.Context, request ConsultationRequest) (ConsultationResponse, error) {
	message, err := c.SendMessage(ctx, request.Message)
	return ConsultationResponse{Message: message}, err
}

func (c *BackendClient) errorMessage(message domain.IncomingMessage, err error) (domain.BackendMessage, error) {
	c.setState(domain.BackendStatusError)
	return domain.BackendMessage{MessageID: message.MessageID, Target: message.Target, Status: domain.BackendStatusError, Error: err.Error()}, err
}
