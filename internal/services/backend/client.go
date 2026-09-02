package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	applogging "github.com/GogaAPPS/AiDe-Bot/internal/core/logging"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

const responseBodyLimit = 1 << 20

type BackendClient struct {
	baseURL     string
	messagesURL string
	httpClient  *http.Client
	logger      *slog.Logger
	clientName  string
	stub        bool

	mu     sync.RWMutex
	status domain.BackendStatus
}

func NewClient(settings config.Settings, logger *slog.Logger) (*BackendClient, error) {
	if !settings.BackendStub && strings.TrimSpace(settings.BackendAPIBaseURL) == "" {
		return nil, errors.New("BACKEND_API_BASE_URL is required when BACKEND_STUB=false")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &BackendClient{
		baseURL:     strings.TrimRight(settings.BackendAPIBaseURL, "/"),
		messagesURL: strings.TrimRight(settings.BackendAPIBaseURL, "/") + "/" + strings.TrimLeft(settings.BackendMessagesPath, "/"),
		httpClient: &http.Client{
			Timeout:       settings.BackendRequestTimeout,
			CheckRedirect: rejectBackendRedirect,
		},
		logger: logger,
		clientName: func() string {
			if strings.TrimSpace(settings.AppName) != "" {
				return settings.AppName
			}
			return "aide-bot"
		}(),
		stub:   settings.BackendStub,
		status: domain.BackendStatusSuccess,
	}, nil
}

func rejectBackendRedirect(request *http.Request, previous []*http.Request) error {
	if len(previous) == 0 {
		return nil
	}

	return fmt.Errorf(
		"backend redirected %s %s to %s; check BACKEND_API_BASE_URL",
		previous[0].Method,
		previous[0].URL.String(),
		request.URL.String(),
	)
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
	traceID := applogging.TraceID(ctx, message.MessageID, message.Target.ChatID, message.Target.UserID)
	ctx = applogging.WithTraceID(ctx, traceID)

	c.setState(domain.BackendStatusLoading)
	if err := ctx.Err(); err != nil {
		c.logFailure(ctx, message, err)
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
	requestBody := sendMessageRequest{
		MessageID: message.MessageID,
		Text:      message.Text,
		ChatID:    message.Target.ChatID,
		UserID:    message.Target.UserID,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		c.logFailure(ctx, message, fmt.Errorf("marshal backend request: %w", err))
		return c.errorMessage(message, fmt.Errorf("marshal backend request: %w", err))
	}

	c.logInfo(ctx, message, "backend request prepared",
		"backend.request.prepared",
		"backend_request_prepared",
		applogging.DirectionOutgoing,
		slog.String(applogging.FieldRoute, "backend.process"),
		slog.String(applogging.FieldMethod, http.MethodPost),
		slog.String(applogging.FieldURL, c.messagesURL),
		slog.String(applogging.FieldBody, string(payload)),
	)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL, bytes.NewReader(payload))
	if err != nil {
		c.logFailure(ctx, message, fmt.Errorf("create backend request: %w", err))
		return c.errorMessage(message, fmt.Errorf("create backend request: %w", err))
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Trace-ID", applogging.TraceID(ctx, message.MessageID, message.Target.ChatID, message.Target.UserID))
	request.Header.Set("X-Client-Name", c.clientName)

	c.logInfo(ctx, message, "backend request sent",
		"backend.request.sent",
		"backend_request_sent",
		applogging.DirectionOutgoing,
		slog.String(applogging.FieldRoute, "backend.process"),
		slog.String(applogging.FieldMethod, request.Method),
		slog.String(applogging.FieldURL, request.URL.String()),
		slog.Int64("timeout_ms", applogging.DurationMS(c.httpClient.Timeout)),
	)

	startedAt := time.Now()
	httpResponse, err := c.httpClient.Do(request)
	duration := time.Since(startedAt)
	if err != nil {
		c.logFailure(ctx, message, fmt.Errorf("send backend request: %w", err),
			slog.String(applogging.FieldRoute, "backend.process"),
			slog.String(applogging.FieldMethod, request.Method),
			slog.String(applogging.FieldURL, request.URL.String()),
			slog.Int64(applogging.FieldDurationMS, applogging.DurationMS(duration)),
		)
		return c.errorMessage(message, fmt.Errorf("send backend request: %w", err))
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, responseBodyLimit))
	if err != nil {
		c.logFailure(ctx, message, fmt.Errorf("read backend response: %w", err),
			slog.Int("http_status", httpResponse.StatusCode),
			slog.Int64(applogging.FieldDurationMS, applogging.DurationMS(duration)),
		)
		return c.errorMessage(message, fmt.Errorf("read backend response: %w", err))
	}

	c.logInfo(ctx, message, "backend response received",
		"backend.response.received",
		"backend_response_received",
		applogging.DirectionIncoming,
		slog.String(applogging.FieldRoute, "backend.process"),
		slog.String(applogging.FieldMethod, request.Method),
		slog.String(applogging.FieldURL, request.URL.String()),
		slog.Int(applogging.FieldStatus, httpResponse.StatusCode),
		slog.Int64(applogging.FieldDurationMS, applogging.DurationMS(duration)),
		slog.String(applogging.FieldBody, string(body)),
	)

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		c.logFailure(ctx, message, fmt.Errorf("backend returned HTTP status %d", httpResponse.StatusCode),
			slog.Int("http_status", httpResponse.StatusCode),
			slog.String(applogging.FieldBody, string(body)),
		)
		return c.errorMessage(message, fmt.Errorf("backend returned HTTP status %d", httpResponse.StatusCode))
	}

	var responseBody sendMessageResponse
	if err := json.Unmarshal(body, &responseBody); err != nil {
		c.logFailure(ctx, message, fmt.Errorf("decode backend response: %w", err),
			slog.String(applogging.FieldBody, string(body)),
		)
		return c.errorMessage(message, fmt.Errorf("decode backend response: %w", err))
	}

	if responseBody.Status != string(domain.BackendStatusSuccess) {
		backendError := responseBody.Error
		if backendError == "" {
			backendError = "backend returned unsuccessful response"
		}
		c.logFailure(ctx, message, errors.New(backendError),
			slog.String("backend_status", responseBody.Status),
			slog.Any(applogging.FieldBody, responseBody),
		)
		return c.errorMessage(message, errors.New(backendError))
	}
	if responseBody.Text == "" && responseBody.File == nil {
		c.logFailure(ctx, message, errors.New("backend response contains neither text nor file"),
			slog.String("backend_status", responseBody.Status),
			slog.Any(applogging.FieldBody, responseBody),
		)
		return c.errorMessage(message, errors.New("backend response contains neither text nor file"))
	}

	response := domain.BackendMessage{
		Text: responseBody.Text, MessageID: message.MessageID, Target: message.Target,
		Status: domain.BackendStatusSuccess,
	}
	if responseBody.File != nil {
		response.File = &domain.BackendFile{
			ID:        responseBody.File.ID,
			Name:      responseBody.File.Name,
			Path:      responseBody.File.Path,
			URL:       responseBody.File.URL,
			MIMEType:  responseBody.File.MIMEType,
			SizeBytes: responseBody.File.SizeBytes,
		}
	}

	c.setState(domain.BackendStatusSuccess)
	c.logInfo(ctx, message, "backend response parsed",
		"backend.response.parsed",
		"backend_response_parsed",
		applogging.DirectionInternal,
		slog.String(applogging.FieldRoute, "backend.process"),
		slog.String(applogging.FieldStatus, string(response.Status)),
		slog.Any(applogging.FieldBody, response),
	)
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

func (c *BackendClient) logInfo(
	ctx context.Context,
	message domain.IncomingMessage,
	logMessage string,
	event string,
	stage string,
	direction string,
	extra ...slog.Attr,
) {
	c.log(ctx, message, slog.LevelInfo, logMessage, event, stage, direction, extra...)
}

func (c *BackendClient) logFailure(ctx context.Context, message domain.IncomingMessage, err error, extra ...slog.Attr) {
	c.log(ctx, message, slog.LevelError, "backend request failed", "backend.request.failed", "backend_request_failed", applogging.DirectionInternal, append(
		extra,
		slog.String(applogging.FieldStatus, string(domain.BackendStatusError)),
		slog.String(applogging.FieldError, err.Error()),
	)...)
}

func (c *BackendClient) log(
	ctx context.Context,
	message domain.IncomingMessage,
	level slog.Level,
	logMessage string,
	event string,
	stage string,
	direction string,
	extra ...slog.Attr,
) {
	traceID := applogging.TraceID(ctx, message.MessageID, message.Target.ChatID, message.Target.UserID)
	attrs := applogging.TemplateAttrs(event, stage, direction, traceID, message.MessageID, message.Target.ChatID, message.Target.UserID)
	attrs = append(attrs, extra...)
	c.logger.LogAttrs(ctx, level, logMessage, attrs...)
}
