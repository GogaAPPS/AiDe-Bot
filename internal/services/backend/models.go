package backend

import "github.com/GogaAPPS/AiDe-Bot/internal/domain"

type sendMessageRequest struct {
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
	ChatID    int64  `json:"chat_id"`
	UserID    int64  `json:"user_id"`
}

type sendMessageResponse struct {
	Status string       `json:"status"`
	Text   string       `json:"text"`
	File   *backendFile `json:"file"`
	Error  string       `json:"error"`
}

type backendFile struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

type ConsultationRequest struct {
	Message domain.IncomingMessage
}

type ConsultationResponse struct {
	Message domain.BackendMessage
}

type CreateDocumentRequest struct {
	Title string
}

type ListDocumentsRequest struct {
	Target domain.Target
}

type Document struct {
	ID    string
	Title string
}

type CreateDocumentResponse struct {
	Status   domain.BackendStatus
	Document Document
	Error    string
}

type ListDocumentsResponse struct {
	Status    domain.BackendStatus
	Documents []Document
	Error     string
}
