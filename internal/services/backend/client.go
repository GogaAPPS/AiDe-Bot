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
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	applogging "github.com/GogaAPPS/AiDe-Bot/internal/core/logging"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

const responseBodyLimit = 1 << 20

type BackendClient struct {
	baseURL          string
	messagesURL      string
	messagesPath     string
	clearHistoryURL  string
	clearHistoryPath string
	httpClient       *http.Client
	logger           *slog.Logger
	clientName       string
	stub             bool

	mu     sync.RWMutex
	status domain.BackendStatus
}

type FileDownload struct {
	Body io.ReadCloser
	Size int64
}

func NewClient(settings config.Settings, logger *slog.Logger) (*BackendClient, error) {
	if !settings.BackendStub && strings.TrimSpace(settings.BackendAPIBaseURL) == "" {
		return nil, errors.New("BACKEND_API_BASE_URL is required when BACKEND_STUB=false")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &BackendClient{
		baseURL:          strings.TrimRight(settings.BackendAPIBaseURL, "/"),
		messagesURL:      strings.TrimRight(settings.BackendAPIBaseURL, "/") + "/" + strings.TrimLeft(settings.BackendMessagesPath, "/"),
		messagesPath:     strings.TrimSpace(settings.BackendMessagesPath),
		clearHistoryURL:  strings.TrimRight(settings.BackendAPIBaseURL, "/") + "/" + strings.TrimLeft(settings.BackendClearHistoryPath, "/"),
		clearHistoryPath: strings.TrimSpace(settings.BackendClearHistoryPath),
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

func (c *BackendClient) ClearHistory(ctx context.Context, target domain.Target) error {
	traceID := applogging.TraceID(ctx, "", target.ChatID, target.UserID)
	ctx = applogging.WithTraceID(ctx, traceID)
	if err := ctx.Err(); err != nil {
		c.logClearHistoryFailure(ctx, target, err)
		c.setState(domain.BackendStatusError)
		return err
	}
	if c.stub {
		c.logTarget(ctx, target, slog.LevelInfo, "backend clear history skipped in stub mode", "backend.clear_history.skipped", "backend_clear_history_skipped", applogging.DirectionInternal,
			slog.String(applogging.FieldRoute, "backend.clear_history"),
			slog.String(applogging.FieldStatus, string(domain.BackendStatusSuccess)),
		)
		c.setState(domain.BackendStatusSuccess)
		return nil
	}
	if c.clearHistoryPath == "" {
		err := errors.New("BACKEND_CLEAR_HISTORY_PATH is required when BACKEND_STUB=false")
		c.logClearHistoryFailure(ctx, target, err)
		c.setState(domain.BackendStatusError)
		return err
	}

	payload, err := json.Marshal(clearHistoryRequest{
		ConversationID: fmt.Sprintf("chat_%d_user_%d", target.ChatID, target.UserID),
	})
	if err != nil {
		c.logClearHistoryFailure(ctx, target, err)
		c.setState(domain.BackendStatusError)
		return fmt.Errorf("marshal clear history request: %w", err)
	}
	c.logTarget(ctx, target, slog.LevelInfo, "backend clear history request prepared", "backend.clear_history.request.prepared", "backend_clear_history_request_prepared", applogging.DirectionOutgoing,
		slog.String(applogging.FieldRoute, "backend.clear_history"),
		slog.String(applogging.FieldMethod, http.MethodPost),
		slog.String(applogging.FieldURL, c.clearHistoryURL),
		slog.String(applogging.FieldBody, string(payload)),
	)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.clearHistoryURL, bytes.NewReader(payload))
	if err != nil {
		c.logClearHistoryFailure(ctx, target, err)
		c.setState(domain.BackendStatusError)
		return fmt.Errorf("create clear history request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Trace-ID", traceID)
	request.Header.Set("X-Client-Name", c.clientName)
	c.logTarget(ctx, target, slog.LevelInfo, "backend clear history request sent", "backend.clear_history.request.sent", "backend_clear_history_request_sent", applogging.DirectionOutgoing,
		slog.String(applogging.FieldRoute, "backend.clear_history"),
		slog.String(applogging.FieldMethod, request.Method),
		slog.String(applogging.FieldURL, request.URL.String()),
		slog.Int64("timeout_ms", applogging.DurationMS(c.httpClient.Timeout)),
	)

	startedAt := time.Now()
	response, err := c.httpClient.Do(request)
	duration := time.Since(startedAt)
	if err != nil {
		wrappedErr := fmt.Errorf("send clear history request: %w", err)
		c.logClearHistoryFailure(ctx, target, wrappedErr,
			slog.String(applogging.FieldRoute, "backend.clear_history"),
			slog.String(applogging.FieldMethod, request.Method),
			slog.String(applogging.FieldURL, request.URL.String()),
			slog.Int64(applogging.FieldDurationMS, applogging.DurationMS(duration)),
		)
		c.setState(domain.BackendStatusError)
		return wrappedErr
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent {
		c.logTarget(ctx, target, slog.LevelInfo, "backend clear history response received", "backend.clear_history.response.received", "backend_clear_history_response_received", applogging.DirectionIncoming,
			slog.String(applogging.FieldRoute, "backend.clear_history"),
			slog.String(applogging.FieldMethod, request.Method),
			slog.String(applogging.FieldURL, request.URL.String()),
			slog.Int(applogging.FieldStatus, response.StatusCode),
			slog.Int64(applogging.FieldDurationMS, applogging.DurationMS(duration)),
			slog.String(applogging.FieldBody, ""),
		)
		c.logTarget(ctx, target, slog.LevelInfo, "backend clear history completed", "backend.clear_history.completed", "backend_clear_history_completed", applogging.DirectionInternal,
			slog.String(applogging.FieldRoute, "backend.clear_history"),
			slog.String(applogging.FieldStatus, string(domain.BackendStatusSuccess)),
		)
		c.setState(domain.BackendStatusSuccess)
		return nil
	}

	body, readErr := io.ReadAll(io.LimitReader(response.Body, responseBodyLimit))
	if readErr != nil {
		wrappedErr := fmt.Errorf("read clear history response: %w", readErr)
		c.logClearHistoryFailure(ctx, target, wrappedErr,
			slog.Int("http_status", response.StatusCode),
			slog.Int64(applogging.FieldDurationMS, applogging.DurationMS(duration)),
		)
		c.setState(domain.BackendStatusError)
		return wrappedErr
	}

	c.logTarget(ctx, target, slog.LevelInfo, "backend clear history response received", "backend.clear_history.response.received", "backend_clear_history_response_received", applogging.DirectionIncoming,
		slog.String(applogging.FieldRoute, "backend.clear_history"),
		slog.String(applogging.FieldMethod, request.Method),
		slog.String(applogging.FieldURL, request.URL.String()),
		slog.Int(applogging.FieldStatus, response.StatusCode),
		slog.Int64(applogging.FieldDurationMS, applogging.DurationMS(duration)),
		slog.String(applogging.FieldBody, string(body)),
	)

	err = fmt.Errorf("clear history returned HTTP status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	c.logClearHistoryFailure(ctx, target, err,
		slog.Int("http_status", response.StatusCode),
		slog.String(applogging.FieldBody, string(body)),
	)
	c.setState(domain.BackendStatusError)
	return err
}

func (c *BackendClient) logClearHistoryFailure(ctx context.Context, target domain.Target, err error, extra ...slog.Attr) {
	c.logTarget(ctx, target, slog.LevelError, "backend clear history failed", "backend.clear_history.failed", "backend_clear_history_failed", applogging.DirectionInternal,
		append(extra,
			slog.String(applogging.FieldStatus, string(domain.BackendStatusError)),
			slog.String(applogging.FieldError, err.Error()),
		)...,
	)
}

func (c *BackendClient) logTarget(
	ctx context.Context,
	target domain.Target,
	level slog.Level,
	logMessage string,
	event string,
	stage string,
	direction string,
	extra ...slog.Attr,
) {
	traceID := applogging.TraceID(ctx, "", target.ChatID, target.UserID)
	attrs := applogging.TemplateAttrs(event, stage, direction, traceID, "", target.ChatID, target.UserID)
	attrs = append(attrs, extra...)
	c.logger.LogAttrs(ctx, level, logMessage, attrs...)
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
	if c.messagesPath == "" {
		return c.errorMessage(message, errors.New("BACKEND_MESSAGES_PATH is required when BACKEND_STUB=false"))
	}

	return c.sendMessageHTTP(ctx, message)
}

func (c *BackendClient) DownloadFile(ctx context.Context, file domain.BackendFile) (FileDownload, error) {
	downloadURL, err := resolveBackendURL(c.baseURL, file.URL)
	if err != nil {
		return FileDownload{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return FileDownload{}, fmt.Errorf("create document download request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return FileDownload{}, fmt.Errorf("download document: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = response.Body.Close()
		return FileDownload{}, fmt.Errorf("download document returned HTTP status %d", response.StatusCode)
	}

	size := response.ContentLength
	if size < 0 {
		size = file.SizeBytes
	}
	if size < 0 {
		_ = response.Body.Close()
		return FileDownload{}, errors.New("document download size is unknown")
	}

	return FileDownload{Body: response.Body, Size: size}, nil
}

func resolveBackendURL(baseURL, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("document URL is empty")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse document URL: %w", err)
	}

	if !parsedURL.IsAbs() {
		base, err := url.Parse(baseURL)
		if err != nil {
			return "", fmt.Errorf("parse backend base URL: %w", err)
		}
		parsedURL = base.ResolveReference(parsedURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("unsupported document URL scheme %q", parsedURL.Scheme)
	}

	return parsedURL.String(), nil
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
	payload, contentType, err := encodeMessage(message)
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
		slog.String("content_type", contentType),
		slog.Int("request_bytes", len(payload)),
	)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL, bytes.NewReader(payload))
	if err != nil {
		c.logFailure(ctx, message, fmt.Errorf("create backend request: %w", err))
		return c.errorMessage(message, fmt.Errorf("create backend request: %w", err))
	}
	request.Header.Set("Content-Type", contentType)
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
		var apiError APIError
		if json.Unmarshal(body, &apiError) == nil && apiError.Code != "" && apiError.Message != "" {
			return c.errorMessage(message, &apiError)
		}
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
