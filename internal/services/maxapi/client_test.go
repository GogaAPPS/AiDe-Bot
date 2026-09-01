package maxapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
