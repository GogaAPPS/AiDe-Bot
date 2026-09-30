package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

func TestSendMessageUploadsMultipartWithoutLoggingImage(t *testing.T) {
	imageBytes := []byte("private-image-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/process" || r.Method != http.MethodPost {
			t.Error("wrong endpoint")
		}
		if r.Header.Get("X-Trace-ID") != "photo-1" {
			t.Error("missing trace ID")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		defer r.MultipartForm.RemoveAll()
		if r.FormValue("message_id") != "photo-1" || r.FormValue("chat_id") != "42" || r.FormValue("user_id") != "7" || r.FormValue("text") != "caption" {
			t.Error("wrong metadata")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		content, _ := io.ReadAll(file)
		if !bytes.Equal(content, imageBytes) || header.Header.Get("Content-Type") != "image/png" {
			t.Error("wrong photo")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok","text":"reply","error":""}`)
	}))
	defer server.Close()
	var logs bytes.Buffer
	client, err := NewClient(config.Settings{BackendAPIBaseURL: server.URL, BackendMessagesPath: "/api/v1/process", BackendRequestTimeout: time.Second}, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.SendMessage(context.Background(), domain.IncomingMessage{
		MessageID: "photo-1", Text: "caption", Target: domain.Target{ChatID: 42, UserID: 7},
		Image: &domain.IncomingImage{Content: imageBytes, MIMEType: "image/png"},
	})
	if err != nil || response.Text != "reply" {
		t.Fatalf("unexpected response: %+v, %v", response, err)
	}
	if strings.Contains(logs.String(), string(imageBytes)) {
		t.Fatal("image leaked into logs")
	}
}

func TestSendMessagePreservesRecognitionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(504)
		_, _ = io.WriteString(w, `{"code":"recognition_timeout","message":"Попробуйте ещё раз."}`)
	}))
	defer server.Close()
	client, _ := NewClient(config.Settings{BackendAPIBaseURL: server.URL, BackendMessagesPath: "/api/v1/process", BackendRequestTimeout: time.Second}, testLogger())
	_, err := client.SendMessage(context.Background(), domain.IncomingMessage{Text: "question"})
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Code != "recognition_timeout" {
		t.Fatalf("unexpected error: %v", err)
	}
}
