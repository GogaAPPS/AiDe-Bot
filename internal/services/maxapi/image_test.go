package maxapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

func TestIncomingMessageKeepsPhotoAndCaption(t *testing.T) {
	update := model.Update{UpdateType: model.UpdateMessageCreated, MessageID: "photo", ChatID: 42, UserID: 7,
		Message: &model.MessageUpdate{Body: model.MessageBody{Text: " caption ", Attachments: []model.Attachment{
			{Type: model.AttachImage, Payload: model.Payload{URL: "https://cdn.example/image"}},
		}}},
	}
	message, ok := IncomingMessageFromUpdate(update)
	if !ok || message.Text != "caption" || len(message.Attachments) != 1 || message.Attachments[0].URL != "https://cdn.example/image" {
		t.Fatalf("unexpected mapped message: %+v", message)
	}
	if message.MessageID != "photo" || message.Target.ChatID != 42 || message.Target.UserID != 7 {
		t.Fatal("lost message metadata")
	}
}

func TestDownloadImage(t *testing.T) {
	photo := append([]byte("\x89PNG\r\n\x1a\n"), []byte("image-data")...)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("must not send bot token to file host")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(photo)
	}))
	defer server.Close()
	client := &Client{imageHTTPClient: server.Client()}
	image, err := client.DownloadImage(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if image.MIMEType != "image/png" || !bytes.Equal(image.Content, photo) {
		t.Fatal("wrong image")
	}
}

func TestDownloadImageRejectsInvalidContent(t *testing.T) {
	for _, test := range []struct {
		name    string
		content []byte
		status  int
	}{
		{"empty", nil, 200}, {"html", []byte("<html>bad</html>"), 200},
		{"too large", bytes.Repeat([]byte("x"), domain.MaxImageBytes+1), 200},
		{"unavailable", nil, 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write(test.content)
			}))
			defer server.Close()
			client := &Client{imageHTTPClient: server.Client()}
			if _, err := client.DownloadImage(context.Background(), server.URL); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDownloadImageRejectsUnsafeURLAndRedactsFailedURL(t *testing.T) {
	client := &Client{imageHTTPClient: &http.Client{}}
	for _, url := range []string{"http://cdn.example/image", "file:///image", "https://user:password@cdn.example/image"} {
		if _, err := client.DownloadImage(context.Background(), url); err == nil {
			t.Fatal("expected URL validation error")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.DownloadImage(ctx, "https://cdn.example/image?token=private")
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("expected safe download error")
	}
}
