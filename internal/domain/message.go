package domain

type Target struct {
	ChatID int64
	UserID int64
}

type IncomingMessage struct {
	Text      string
	MessageID string
	Target    Target
}

type OutgoingMessage struct {
	Text   string
	Target Target
}

type BackendStatus string

const (
	BackendStatusLoading BackendStatus = "loading"
	BackendStatusSuccess BackendStatus = "success"
	BackendStatusError   BackendStatus = "error"
)

type BackendFile struct {
	Name        string
	ContentType string
	Data        []byte
}

type BackendMessage struct {
	Text      string
	MessageID string
	Target    Target
	File      *BackendFile
	Status    BackendStatus
	Error     string
}
