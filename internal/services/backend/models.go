package backend

import "github.com/GogaAPPS/AiDe-Bot/internal/domain"

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
