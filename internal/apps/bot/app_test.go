package bot

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/backend"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/inputfilter"
	"github.com/GogaAPPS/AiDe-Bot/internal/services/maxapi"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

type fakeBotClient struct {
	texts         []string
	textsWithMenu []string
	files         []string
	filesWithMenu []string
	fileNames     []string
	menuTargets   []domain.Target
	callbackIDs   []string
	callbackTexts []string
	deletedIDs    []string
	updates       []model.Update
	textErr       error
	fileErr       error
}

func (f *fakeBotClient) LogBotInfo(context.Context, *slog.Logger) error { return nil }

func (f *fakeBotClient) GetUpdates(context.Context, int64) ([]model.Update, int64, error) {
	if len(f.updates) == 0 {
		return nil, 0, context.Canceled
	}
	update := f.updates[0]
	f.updates = f.updates[1:]
	return []model.Update{update}, 1, nil
}

func (f *fakeBotClient) SendText(_ context.Context, _ domain.Target, text string) error {
	f.texts = append(f.texts, text)
	return f.textErr
}

func (f *fakeBotClient) SendTextWithMenu(_ context.Context, _ domain.Target, text string) error {
	f.texts = append(f.texts, text)
	f.textsWithMenu = append(f.textsWithMenu, text)
	return f.textErr
}

func (f *fakeBotClient) SendFile(_ context.Context, _ domain.Target, name string, reader io.Reader, _ int64) error {
	return f.sendFile(name, reader, false)
}

func (f *fakeBotClient) SendFileWithMenu(_ context.Context, _ domain.Target, name string, reader io.Reader, _ int64) error {
	return f.sendFile(name, reader, true)
}

func (f *fakeBotClient) sendFile(name string, reader io.Reader, withMenu bool) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.fileNames = append(f.fileNames, name)
	f.files = append(f.files, string(data))
	if withMenu {
		f.filesWithMenu = append(f.filesWithMenu, name)
	}
	return f.fileErr
}

func (f *fakeBotClient) SendMainMenu(_ context.Context, target domain.Target) error {
	f.menuTargets = append(f.menuTargets, target)
	return nil
}

func (f *fakeBotClient) AnswerCallback(_ context.Context, callbackID string, text string) error {
	f.callbackIDs = append(f.callbackIDs, callbackID)
	f.callbackTexts = append(f.callbackTexts, text)
	return nil
}

func (f *fakeBotClient) DeleteMessage(_ context.Context, messageID string) error {
	f.deletedIDs = append(f.deletedIDs, messageID)
	return nil

}

type fakeBotBackend struct {
	download        backend.FileDownload
	downloadErr     error
	clearTargets    []domain.Target
	clearHistoryErr error
}

func (f *fakeBotBackend) SendMessage(context.Context, domain.IncomingMessage) (domain.BackendMessage, error) {
	return domain.BackendMessage{}, nil
}

func (f *fakeBotBackend) ClearHistory(_ context.Context, target domain.Target) error {
	f.clearTargets = append(f.clearTargets, target)
	return f.clearHistoryErr
}

func (f *fakeBotBackend) DownloadFile(context.Context, domain.BackendFile) (backend.FileDownload, error) {
	return f.download, f.downloadErr
}

func newTestApp(client *fakeBotClient, backendClient *fakeBotBackend) *App {
	return &App{
		client:  client,
		backend: backendClient,
		filter:  inputfilter.New(inputfilter.DefaultOptions()),
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
	if len(client.textsWithMenu) != 0 || len(client.filesWithMenu) != 1 {
		t.Fatalf("menu should be attached only to the last message: texts=%v files=%v", client.textsWithMenu, client.filesWithMenu)
	}
}

func TestSendBackendResponseSendsTextWithMenu(t *testing.T) {
	client := &fakeBotClient{}
	app := newTestApp(client, &fakeBotBackend{})

	app.sendBackendResponse(context.Background(), domain.IncomingMessage{}, domain.BackendMessage{
		Text:   "Ответ агента",
		Target: domain.Target{ChatID: 42},
	})

	if len(client.textsWithMenu) != 1 || client.textsWithMenu[0] != "Ответ агента" {
		t.Fatalf("expected text response with menu: %v", client.textsWithMenu)
	}
}

func TestHandleNewChatClearsHistoryAndAcknowledgesSuccess(t *testing.T) {
	client := &fakeBotClient{}
	backendClient := &fakeBotBackend{}
	app := newTestApp(client, backendClient)

	err := app.handleNewChat(context.Background(), maxapi.CallbackEvent{
		ID:     "callback-1",
		Target: domain.Target{ChatID: 77, UserID: 42},
	})
	if err != nil {
		t.Fatalf("handle new chat: %v", err)
	}
	if len(backendClient.clearTargets) != 1 || backendClient.clearTargets[0] != (domain.Target{ChatID: 77, UserID: 42}) {
		t.Fatalf("unexpected clear history targets: %+v", backendClient.clearTargets)
	}
	if len(client.texts) != 1 || client.texts[0] != newChatSuccessMessage {
		t.Fatalf("unexpected chat messages: %v", client.texts)
	}
	if len(client.callbackTexts) != 1 || client.callbackIDs[0] != "callback-1" || client.callbackTexts[0] != "История очищена" {
		t.Fatalf("unexpected callback response: ids=%v texts=%v", client.callbackIDs, client.callbackTexts)
	}
}

func TestHandleNewChatAcknowledgesFailure(t *testing.T) {
	client := &fakeBotClient{}
	backendClient := &fakeBotBackend{clearHistoryErr: io.ErrUnexpectedEOF}
	app := newTestApp(client, backendClient)

	err := app.handleNewChat(context.Background(), maxapi.CallbackEvent{ID: "callback-2"})
	if err != nil {
		t.Fatalf("handle new chat: %v", err)
	}
	if len(client.texts) != 1 || client.texts[0] != serviceUnavailableMessage {
		t.Fatalf("unexpected chat messages: %v", client.texts)
	}
	if len(client.callbackTexts) != 1 || client.callbackTexts[0] != serviceUnavailableMessage {
		t.Fatalf("unexpected callback response: %v", client.callbackTexts)
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
	if len(client.filesWithMenu) != 1 {
		t.Fatalf("file response should include menu: %v", client.filesWithMenu)
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
	if len(client.textsWithMenu) != 1 {
		t.Fatalf("document failure should include menu: %v", client.textsWithMenu)
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
	if len(client.filesWithMenu) != 1 {
		t.Fatalf("file send attempt should include menu: %v", client.filesWithMenu)
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
	if len(client.filesWithMenu) != 1 {
		t.Fatalf("file delivery should include menu: %v", client.filesWithMenu)
	}
}

func TestHandleMainMenuAnswersCallbackAndSendsMenu(t *testing.T) {
	client := &fakeBotClient{}
	app := newTestApp(client, &fakeBotBackend{})

	err := app.handleCallback(context.Background(), maxapi.CallbackEvent{
		ID:      "callback-1",
		Payload: maxapi.CallbackMainMenu,
		Target:  domain.Target{ChatID: 42, UserID: 7},
	})
	if err != nil {
		t.Fatalf("handle menu callback: %v", err)
	}

	if len(client.callbackIDs) != 1 || client.callbackIDs[0] != "callback-1" {
		t.Fatalf("unexpected callback answers: %v", client.callbackIDs)
	}
	if len(client.menuTargets) != 1 || client.menuTargets[0] != (domain.Target{ChatID: 42, UserID: 7}) {
		t.Fatalf("unexpected main menu targets: %v", client.menuTargets)
	}
}

func TestHandleBackAnswersCallbackAndDeletesMenu(t *testing.T) {
	client := &fakeBotClient{}
	app := newTestApp(client, &fakeBotBackend{})

	err := app.handleCallback(context.Background(), maxapi.CallbackEvent{
		ID:        "callback-1",
		MessageID: "message-1",
		Payload:   maxapi.CallbackBack,
	})
	if err != nil {
		t.Fatalf("handle back callback: %v", err)
	}

	if len(client.callbackIDs) != 1 || client.callbackIDs[0] != "callback-1" {
		t.Fatalf("unexpected callback answers: %v", client.callbackIDs)
	}
	if len(client.deletedIDs) != 1 || client.deletedIDs[0] != "message-1" {
		t.Fatalf("unexpected deleted messages: %v", client.deletedIDs)
	}
}

func TestRunRejectedMessageSendsMenu(t *testing.T) {
	client := &fakeBotClient{updates: []model.Update{{
		UpdateType: model.UpdateMessageCreated,
		MessageID:  "message-1",
		ChatID:     42,
		UserID:     7,
		Message: &model.MessageUpdate{
			Body: model.MessageBody{Text: "   "},
		},
	}}}
	app := newTestApp(client, &fakeBotBackend{})

	err := app.Run(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run should stop after fake updates are exhausted: %v", err)
	}
	if len(client.textsWithMenu) != 1 || client.textsWithMenu[0] != "Напишите, пожалуйста, ваш запрос." {
		t.Fatalf("rejected response should include menu: %v", client.textsWithMenu)
	}
}
