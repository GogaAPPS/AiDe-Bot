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
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	URL       string `json:"url"`
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
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
