package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const (
	FieldEvent      = "event"
	FieldStage      = "stage"
	FieldDirection  = "direction"
	FieldTraceID    = "trace_id"
	FieldMessageID  = "message_id"
	FieldChatID     = "chat_id"
	FieldUserID     = "user_id"
	FieldStatus     = "status"
	FieldRoute      = "route"
	FieldMethod     = "method"
	FieldURL        = "url"
	FieldBody       = "body"
	FieldError      = "error"
	FieldDurationMS = "duration_ms"
)

const (
	DirectionIncoming = "incoming"
	DirectionInternal = "internal"
	DirectionOutgoing = "outgoing"
)

type traceIDKey struct{}

func New(level string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	}))
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}

	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func TraceID(ctx context.Context, messageID string, chatID, userID int64) string {
	if traceID, ok := ctx.Value(traceIDKey{}).(string); ok && traceID != "" {
		return traceID
	}

	return NewTraceID(messageID, chatID, userID)
}

func NewTraceID(messageID string, chatID, userID int64) string {
	if strings.TrimSpace(messageID) != "" {
		return messageID
	}

	return fmt.Sprintf("chat_%d_user_%d_%d", chatID, userID, time.Now().UnixNano())
}

func TemplateAttrs(event, stage, direction, traceID, messageID string, chatID, userID int64) []slog.Attr {
	return []slog.Attr{
		slog.String(FieldEvent, event),
		slog.String(FieldStage, stage),
		slog.String(FieldDirection, direction),
		slog.String(FieldTraceID, traceID),
		slog.String(FieldMessageID, messageID),
		slog.Int64(FieldChatID, chatID),
		slog.Int64(FieldUserID, userID),
	}
}

func DurationMS(duration time.Duration) int64 {
	return duration.Milliseconds()
}

func parseLevel(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
