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
	BackendStatusSuccess BackendStatus = "ok"
	BackendStatusError   BackendStatus = "error"
)

type BackendFile struct {
	ID        string
	Name      string
	Path      string
	URL       string
	MIMEType  string
	SizeBytes int64
}

type BackendMessage struct {
	Text      string
	MessageID string
	Target    Target
	File      *BackendFile
	Status    BackendStatus
	Error     string
}
