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
