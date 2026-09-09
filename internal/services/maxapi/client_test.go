package maxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GogaAPPS/AiDe-Bot/internal/core/config"
	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

func TestSendMainMenu(t *testing.T) {
	var requestBody struct {
		Text        string `json:"text"`
		Attachments []struct {
			Type    string `json:"type"`
			Payload struct {
				Buttons [][]struct {
					Type    string `json:"type"`
					Text    string `json:"text"`
					Payload string `json:"payload"`
				} `json:"buttons"`
			} `json:"payload"`
		} `json:"attachments"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/messages" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.URL.Query().Get("user_id") != "7" {
			t.Fatalf("unexpected user_id: %s", request.URL.RawQuery)
		}
		if request.Header.Get("Authorization") != "token" {
			t.Fatalf("unexpected authorization header")
		}
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"message":{"body":{"text":"ok"}}}`))
	}))
	defer server.Close()

	client, err := NewClient(config.Settings{
		MaxBotToken:    "token",
		MaxAPIBaseURL:  server.URL,
		RequestTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	err = client.SendMainMenu(context.Background(), domain.Target{UserID: 7})
	if err != nil {
		t.Fatalf("send main menu: %v", err)
	}

	if requestBody.Text != "Выберите действие:" {
		t.Fatalf("unexpected text: %q", requestBody.Text)
	}
	if len(requestBody.Attachments) != 1 {
		t.Fatalf("unexpected attachments: %+v", requestBody.Attachments)
	}

	keyboard := requestBody.Attachments[0]
	if keyboard.Type != "inline_keyboard" {
		t.Fatalf("unexpected attachment type: %s", keyboard.Type)
	}
	if len(keyboard.Payload.Buttons) != 2 || len(keyboard.Payload.Buttons[0]) != 1 || len(keyboard.Payload.Buttons[1]) != 1 {
		t.Fatalf("unexpected buttons: %+v", keyboard.Payload.Buttons)
	}

	firstButton := keyboard.Payload.Buttons[0][0]
	if firstButton.Type != "callback" || firstButton.Text != "Новый чат" || firstButton.Payload != CallbackNewChat {
		t.Fatalf("unexpected first button: %+v", firstButton)
	}

	secondButton := keyboard.Payload.Buttons[1][0]
	if secondButton.Type != "callback" || secondButton.Text != "Генерация дизайна(Develop)" || secondButton.Payload != CallbackDesignDevelop {
		t.Fatalf("unexpected second button: %+v", secondButton)
	}
}

func TestSendFileUploadsAndSendsAttachment(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/uploads":
			if request.Method != http.MethodPost || request.URL.Query().Get("type") != "file" {
				t.Fatalf("unexpected upload URL: %s %s", request.Method, request.URL.String())
			}
			if request.Header.Get("Authorization") != "token" {
				t.Fatalf("missing authorization on upload URL request")
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"url":"` + server.URL + `/upload-file"}`))
		case "/upload-file":
			if request.Method != http.MethodPost {
				t.Fatalf("unexpected upload method: %s", request.Method)
			}
			reader, err := request.MultipartReader()
			if err != nil {
				t.Fatalf("create multipart reader: %v", err)
			}
			part, err := reader.NextPart()
			if err != nil {
				t.Fatalf("read multipart part: %v", err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read multipart data: %v", err)
			}
			if part.FormName() != "data" || part.FileName() != "report.xlsx" || string(data) != "content" {
				t.Fatalf("unexpected multipart file: form=%q name=%q data=%q", part.FormName(), part.FileName(), data)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"token":"file-token"}`))
		case "/messages":
			if request.URL.Query().Get("chat_id") != "42" {
				t.Fatalf("unexpected chat id: %s", request.URL.Query().Get("chat_id"))
			}
			var body struct {
				Attachments []struct {
					Type    string `json:"type"`
					Payload struct {
						Token string `json:"token"`
					} `json:"payload"`
				} `json:"attachments"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode message: %v", err)
			}
			if len(body.Attachments) != 1 || body.Attachments[0].Type != "file" || body.Attachments[0].Payload.Token != "file-token" {
				t.Fatalf("unexpected message attachment: %+v", body.Attachments)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"message":{"body":{"text":"ok"}}}`))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(config.Settings{
		MaxBotToken:    "token",
		MaxAPIBaseURL:  server.URL,
		RequestTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	err = client.SendFile(context.Background(), domain.Target{ChatID: 42}, "report.xlsx", bytes.NewReader([]byte("content")), 7)
	if err != nil {
		t.Fatalf("send file: %v", err)
	}
}

func TestSendFileRejectsInvalidInput(t *testing.T) {
	client, err := NewClient(config.Settings{MaxBotToken: "token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	for name, reader := range map[string]io.Reader{
		"nil reader":    nil,
		"negative size": bytes.NewReader(nil),
	} {
		t.Run(name, func(t *testing.T) {
			size := int64(1)
			if name == "negative size" {
				size = -1
			}
			if err := client.SendFile(context.Background(), domain.Target{}, "file", reader, size); err == nil || !strings.Contains(err.Error(), "file") {
				t.Fatalf("expected file input error, got %v", err)
			}
		})
	}
}

func TestSendFilePropagatesUploadAndMessageErrors(t *testing.T) {
	tests := []struct {
		name          string
		uploadStatus  int
		messageStatus int
	}{
		{name: "upload error", uploadStatus: http.StatusBadGateway},
		{name: "message error", messageStatus: http.StatusBadGateway},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/uploads":
					if testCase.uploadStatus != 0 {
						writer.WriteHeader(testCase.uploadStatus)
						return
					}
					_, _ = writer.Write([]byte(`{"url":"` + server.URL + `/upload-file"}`))
				case "/upload-file":
					_, _ = io.Copy(io.Discard, request.Body)
					_, _ = writer.Write([]byte(`{"token":"file-token"}`))
				case "/messages":
					writer.WriteHeader(testCase.messageStatus)
				default:
					t.Fatalf("unexpected request path: %s", request.URL.Path)
				}
			}))
			defer server.Close()

			client, err := NewClient(config.Settings{
				MaxBotToken:    "token",
				MaxAPIBaseURL:  server.URL,
				RequestTimeout: time.Second,
			})
			if err != nil {
				t.Fatalf("create client: %v", err)
			}

			err = client.SendFile(context.Background(), domain.Target{ChatID: 42}, "report.xlsx", bytes.NewReader([]byte("content")), 7)
			if err == nil {
				t.Fatal("expected send file error")
			}
		})
	}
}
