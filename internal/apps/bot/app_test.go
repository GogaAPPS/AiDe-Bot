package bot

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/backend"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

type fakeBotClient struct {
	texts     []string
	files     []string
	fileNames []string
	textErr   error
	fileErr   error
}

func (f *fakeBotClient) LogBotInfo(context.Context, *slog.Logger) error { return nil }

func (f *fakeBotClient) GetUpdates(context.Context, int64) ([]model.Update, int64, error) {
	return nil, 0, nil
}

func (f *fakeBotClient) SendText(_ context.Context, _ domain.Target, text string) error {
	f.texts = append(f.texts, text)
	return f.textErr
}

func (f *fakeBotClient) SendFile(_ context.Context, _ domain.Target, name string, reader io.Reader, _ int64) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.fileNames = append(f.fileNames, name)
	f.files = append(f.files, string(data))
	return f.fileErr
}

func (f *fakeBotClient) SendMainMenu(context.Context, domain.Target) error { return nil }

func (f *fakeBotClient) AnswerCallback(context.Context, string, string) error { return nil }

type fakeBotBackend struct {
	download    backend.FileDownload
	downloadErr error
}

func (f *fakeBotBackend) SendMessage(context.Context, domain.IncomingMessage) (domain.BackendMessage, error) {
	return domain.BackendMessage{}, nil
}

func (f *fakeBotBackend) DownloadFile(context.Context, domain.BackendFile) (backend.FileDownload, error) {
	return f.download, f.downloadErr
}

func newTestApp(client *fakeBotClient, backendClient *fakeBotBackend) *App {
	return &App{
		client:  client,
		backend: backendClient,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestSendBackendResponseSendsTextAndFileSeparately(t *testing.T) {
	client := &fakeBotClient{}
	backendClient := &fakeBotBackend{
		download: backend.FileDownload{Body: io.NopCloser(bytes.NewReader([]byte("report"))), Size: 6},
	}
	app := newTestApp(client, backendClient)

	app.sendBackendResponse(context.Background(), domain.IncomingMessage{}, domain.BackendMessage{
		Text:   "Документ готов",
		Target: domain.Target{ChatID: 42},
		File:   &domain.BackendFile{Name: "report.xlsx", URL: "/download", Path: "/private/report.xlsx"},
	})

	if len(client.texts) != 1 || client.texts[0] != "Документ готов" {
		t.Fatalf("unexpected text messages: %v", client.texts)
	}
	if len(client.files) != 1 || client.files[0] != "report" || client.fileNames[0] != "report.xlsx" {
		t.Fatalf("unexpected file messages: names=%v files=%v", client.fileNames, client.files)
	}
}

func TestSendBackendResponseSendsFileWithoutText(t *testing.T) {
	client := &fakeBotClient{}
	backendClient := &fakeBotBackend{
		download: backend.FileDownload{Body: io.NopCloser(bytes.NewReader([]byte("report"))), Size: 6},
	}
	app := newTestApp(client, backendClient)

	app.sendBackendResponse(context.Background(), domain.IncomingMessage{}, domain.BackendMessage{
		Target: domain.Target{ChatID: 42},
		File:   &domain.BackendFile{Name: "report.xlsx", URL: "/download"},
	})

	if len(client.texts) != 0 || len(client.files) != 1 {
		t.Fatalf("unexpected messages: texts=%v files=%v", client.texts, client.files)
	}
}

func TestSendBackendResponseNotifiesWhenDownloadFails(t *testing.T) {
	client := &fakeBotClient{}
	backendClient := &fakeBotBackend{downloadErr: io.ErrUnexpectedEOF}
	app := newTestApp(client, backendClient)

	app.sendBackendResponse(context.Background(), domain.IncomingMessage{}, domain.BackendMessage{
		Text:   "Документ готов",
		Target: domain.Target{ChatID: 42},
		File:   &domain.BackendFile{URL: "/download", Path: "/private/report.xlsx"},
	})

	if len(client.texts) != 2 || client.texts[0] != "Документ готов" || client.texts[1] != documentDeliveryError {
		t.Fatalf("unexpected text messages: %v", client.texts)
	}
	if len(client.files) != 0 {
		t.Fatalf("file should not be sent: %v", client.files)
	}
}

func TestSendBackendResponseNotifiesWhenFileSendFails(t *testing.T) {
	client := &fakeBotClient{fileErr: io.ErrClosedPipe}
	backendClient := &fakeBotBackend{
		download: backend.FileDownload{Body: io.NopCloser(bytes.NewReader([]byte("report"))), Size: 6},
	}
	app := newTestApp(client, backendClient)

	app.sendBackendResponse(context.Background(), domain.IncomingMessage{}, domain.BackendMessage{
		Text:   "Документ готов",
		Target: domain.Target{ChatID: 42},
		File:   &domain.BackendFile{Name: "report.xlsx", URL: "/download"},
	})

	if len(client.texts) != 2 || client.texts[1] != documentDeliveryError {
		t.Fatalf("unexpected text messages: %v", client.texts)
	}
	if len(client.files) != 1 {
		t.Fatalf("expected file send attempt: %v", client.files)
	}
}

func TestSendBackendResponseSendsFileWhenTextSendFails(t *testing.T) {
	client := &fakeBotClient{textErr: io.ErrClosedPipe}
	backendClient := &fakeBotBackend{
		download: backend.FileDownload{Body: io.NopCloser(bytes.NewReader([]byte("report"))), Size: 6},
	}
	app := newTestApp(client, backendClient)

	app.sendBackendResponse(context.Background(), domain.IncomingMessage{}, domain.BackendMessage{
		Text:   "Документ готов",
		Target: domain.Target{ChatID: 42},
		File:   &domain.BackendFile{Name: "report.xlsx", URL: "/download"},
	})

	if len(client.files) != 1 || len(client.texts) != 1 {
		t.Fatalf("file delivery should be independent: texts=%v files=%v", client.texts, client.files)
	}
}
