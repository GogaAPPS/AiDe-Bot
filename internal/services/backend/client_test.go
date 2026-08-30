package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestSendMessageHTTPWithoutFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/process" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"status": "ok",
			"text":   "Ответ backend",
		})
	}))
	defer server.Close()

	client, err := NewClient(config.Settings{
		BackendAPIBaseURL:     server.URL,
		BackendMessagesPath:   "/api/v1/process",
		BackendRequestTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	response, err := client.SendMessage(context.Background(), domain.IncomingMessage{Text: "вопрос"})
	if err != nil || response.Text != "Ответ backend" || response.File != nil {
		t.Fatalf("unexpected response: %+v, error: %v", response, err)
	}
}

func TestSendMessageHTTPWithFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": "ok",
			"file": map[string]any{
				"id":         "file-1",
				"name":       "report.txt",
				"path":       "/tmp/report.txt",
				"url":        "https://example.com/report.txt",
				"mime_type":  "text/plain",
				"size_bytes": 6,
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(config.Settings{BackendAPIBaseURL: server.URL, BackendMessagesPath: "/api/v1/process"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	response, err := client.SendMessage(context.Background(), domain.IncomingMessage{Text: "создай отчет"})
	if err != nil || response.File == nil || response.File.ID != "file-1" || response.File.MIMEType != "text/plain" || response.File.SizeBytes != 6 {
		t.Fatalf("unexpected response: %+v, error: %v", response, err)
	}
}

func TestSendMessageHTTPRejectsRedirect(t *testing.T) {
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Fatalf("redirect target should not be called: %s %s", request.Method, request.URL.Path)
	}))
	defer redirectTarget.Close()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, redirectTarget.URL+"/api/v1/process", http.StatusFound)
	}))
	defer server.Close()

	client, err := NewClient(config.Settings{BackendAPIBaseURL: server.URL, BackendMessagesPath: "/api/v1/process"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	response, err := client.SendMessage(context.Background(), domain.IncomingMessage{Text: "вопрос"})
	if err == nil || response.Status != domain.BackendStatusError {
		t.Fatalf("expected redirect error: response=%+v error=%v", response, err)
	}
	if !strings.Contains(err.Error(), "backend redirected POST") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendMessageHTTPRejectsInvalidResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "http error", statusCode: http.StatusBadGateway, body: `{"status":"ok","text":"ok"}`},
		{name: "invalid json", statusCode: http.StatusOK, body: `{`},
		{name: "empty result", statusCode: http.StatusOK, body: `{"status":"ok"}`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(testCase.statusCode)
				_, _ = writer.Write([]byte(testCase.body))
			}))
			defer server.Close()

			client, err := NewClient(config.Settings{BackendAPIBaseURL: server.URL, BackendMessagesPath: "/api/v1/process"})
			if err != nil {
				t.Fatalf("create client: %v", err)
			}

			response, err := client.SendMessage(context.Background(), domain.IncomingMessage{})
			if err == nil || response.Status != domain.BackendStatusError || client.State() != domain.BackendStatusError {
				t.Fatalf("expected error: response=%+v error=%v state=%s", response, err, client.State())
			}
		})
	}
}
