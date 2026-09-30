package maxapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
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

func TestDownloadImageAcceptsRealImageFormats(t *testing.T) {
	var jpegBuffer, pngBuffer bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := jpeg.Encode(&jpegBuffer, picture, nil); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&pngBuffer, picture); err != nil {
		t.Fatal(err)
	}
	webpBytes, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		content      []byte
		expectedMIME string
	}{
		{"jpeg", jpegBuffer.Bytes(), "image/jpeg"},
		{"png", pngBuffer.Bytes(), "image/png"},
		{"webp", webpBytes, "image/png"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// The response header and original filename are not reliable format indicators.
				w.Header().Set("Content-Type", "image/jpeg")
				_, _ = w.Write(test.content)
			}))
			defer server.Close()
			client := &Client{imageHTTPClient: server.Client()}
			result, err := client.DownloadImage(context.Background(), server.URL+"/original.jpg")
			if err != nil {
				t.Fatal(err)
			}
			if result.MIMEType != test.expectedMIME {
				t.Fatalf("unexpected MIME: %s", result.MIMEType)
			}
			if test.name == "webp" {
				decoded, err := png.Decode(bytes.NewReader(result.Content))
				if err != nil {
					t.Fatalf("not a valid PNG: %v", err)
				}
				if decoded.Bounds().Dx() != 1 || decoded.Bounds().Dy() != 1 {
					t.Fatal("wrong dimensions")
				}
			} else if !bytes.Equal(result.Content, test.content) {
				t.Fatal("JPEG/PNG should pass through unchanged")
			}
		})
	}
}

func TestDownloadImageReportsActualTypeWithoutURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>access denied</html>"))
	}))
	defer server.Close()
	client := &Client{imageHTTPClient: server.Client()}
	_, err := client.DownloadImage(context.Background(), server.URL+"/image?token=private")
	if err == nil {
		t.Fatal("expected failure")
	}
	for _, field := range []string{"detected_type=text/html", "response_type=text/html", "size_bytes="} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("missing %s in error: %v", field, err)
		}
	}
	if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "access denied") {
		t.Fatal("response or URL leaked")
	}
}

func TestNormalizeImageRejectsLargeWebPBeforeDecoding(t *testing.T) {
	// VP8L header declaring 8192 x 8192 pixels, with no pixel data.
	content := []byte("RIFF\x12\x00\x00\x00WEBPVP8L\x05\x00\x00\x00\x2f\x00\x00\x00\x00\x00")
	binary.LittleEndian.PutUint32(content[21:25], uint32(8191|(8191<<14)))
	_, err := normalizeImage(content, "image/webp")
	if err == nil || !strings.Contains(err.Error(), "разрешение") {
		t.Fatalf("expected resolution rejection: %v", err)
	}
}

func TestNormalizeImageRejectsBrokenWebP(t *testing.T) {
	if _, err := normalizeImage([]byte("RIFF0000WEBPVP8 "), "image/webp"); err == nil {
		t.Fatal("expected decoding error")
	}
}

func TestPNGOutputWriterEnforcesSizeLimit(t *testing.T) {
	var output bytes.Buffer
	writer := &limitedImageWriter{buffer: &output}
	if _, err := writer.Write(make([]byte, domain.MaxImageBytes)); err != nil {
		t.Fatal(err)
	}
	if n, err := writer.Write([]byte{1}); err == nil || n != 0 {
		t.Fatal("expected output size rejection")
	}
	if output.Len() != domain.MaxImageBytes {
		t.Fatal("oversized output was written")
	}
}
