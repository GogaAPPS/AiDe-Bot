package inputfilter

import (
	"testing"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

func TestCheckNormalizesWhitespace(t *testing.T) {
	result := New(DefaultOptions()).Check(message("  Привет\n\nмир\t!  "))

	if !result.Accepted {
		t.Fatalf("expected message to be accepted: %s", result.Explanation)
	}
	if result.Message.Text != "Привет мир !" {
		t.Fatalf("unexpected normalized text: %q", result.Message.Text)
	}
}

func TestCheckRejectsEmptyMessage(t *testing.T) {
	result := New(DefaultOptions()).Check(message(" \n\t "))

	if result.Accepted {
		t.Fatal("expected empty message to be rejected")
	}
	if result.Explanation == "" {
		t.Fatal("expected explanation")
	}
}

func TestCheckRejectsTooLongMessage(t *testing.T) {
	filter := New(Options{MaxTextRunes: 5})
	result := filter.Check(message("шестой"))

	if result.Accepted {
		t.Fatal("expected long message to be rejected")
	}
}

func TestCheckAllowsCommandsAndShortServiceReplies(t *testing.T) {
	filter := New(Options{MaxTextRunes: 2})

	for _, text := range []string{"/start", "да", "ок"} {
		result := filter.Check(message(text))
		if !result.Accepted {
			t.Fatalf("expected %q to be accepted: %s", text, result.Explanation)
		}
	}
}

func message(text string) domain.IncomingMessage {
	return domain.IncomingMessage{Text: text}
}

func TestFilterAcceptsSinglePhotoWithoutText(t *testing.T) {
	filter := New(DefaultOptions())
	result := filter.Check(domain.IncomingMessage{Attachments: []domain.IncomingAttachment{{Type: "image", URL: "https://cdn.example/image"}}})
	if !result.Accepted {
		t.Fatalf("photo rejected: %s", result.Explanation)
	}
}

func TestFilterRejectsUnsupportedOrMultipleAttachments(t *testing.T) {
	filter := New(DefaultOptions())
	for _, attachments := range [][]domain.IncomingAttachment{
		{{Type: "file", URL: "https://cdn.example/document"}},
		{{Type: "image"}},
		{{Type: "image", URL: "https://cdn.example/a"}, {Type: "image", URL: "https://cdn.example/b"}},
	} {
		result := filter.Check(domain.IncomingMessage{Text: "caption", Attachments: attachments})
		if result.Accepted {
			t.Fatalf("unexpected accepted attachments: %+v", attachments)
		}
	}
}
