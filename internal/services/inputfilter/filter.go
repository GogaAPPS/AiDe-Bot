package inputfilter

import (
	"strings"
	"unicode/utf8"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

const defaultMaxTextRunes = 2000

type Options struct {
	MaxTextRunes int
}

type Filter struct {
	maxTextRunes int
}

type Result struct {
	Message     domain.IncomingMessage
	Accepted    bool
	Explanation string
}

func DefaultOptions() Options {
	return Options{MaxTextRunes: defaultMaxTextRunes}
}

func New(options Options) *Filter {
	if options.MaxTextRunes <= 0 {
		options.MaxTextRunes = defaultMaxTextRunes
	}
	return &Filter{maxTextRunes: options.MaxTextRunes}
}

func (f *Filter) Check(message domain.IncomingMessage) Result {
	message.Text = Normalize(message.Text)
	if len(message.Attachments) > 1 {
		return reject(message, "Пришлите одну фотографию за сообщение.")
	}
	if len(message.Attachments) == 1 {
		attachment := message.Attachments[0]
		if attachment.Type != "image" || attachment.URL == "" {
			return reject(message, "Пришлите фотографию в формате JPEG или PNG.")
		}
	}
	if message.Text == "" && len(message.Attachments) == 0 {
		return reject(message, "Напишите, пожалуйста, ваш запрос.")
	}

	if isCommand(message.Text) || isShortServiceReply(message.Text) {
		return accept(message)
	}
	if utf8.RuneCountInString(message.Text) > f.maxTextRunes {
		return reject(message, "Сообщение слишком длинное. Сократите его до 2000 символов.")
	}

	return accept(message)
}

func Normalize(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func accept(message domain.IncomingMessage) Result {
	return Result{Message: message, Accepted: true}
}

func reject(message domain.IncomingMessage, explanation string) Result {
	return Result{Message: message, Explanation: explanation}
}

func isCommand(text string) bool {
	return strings.HasPrefix(text, "/")
}

func isShortServiceReply(text string) bool {
	switch strings.ToLower(text) {
	case "да", "нет", "ок", "окей", "хорошо", "готово", "отмена", "назад", "стоп", "продолжить":
		return true
	default:
		return false
	}
}
