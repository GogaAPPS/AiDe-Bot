package inputfilter

import (
	"testing"
	"time"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

func TestCheckNormalizesWhitespaceForAcceptedMessage(t *testing.T) {
	filter := newTestFilter()

	result := filter.Check(message("  Как\n\nсоставить\tиск?  ", 1))

	if !result.Accepted {
		t.Fatalf("expected message to be accepted: %s", result.Explanation)
	}
	if result.Message.Text != "Как составить иск?" {
		t.Fatalf("unexpected normalized text: %q", result.Message.Text)
	}
}

func TestCheckRejectsEmptyLongAndShortFreeText(t *testing.T) {
	filter := New(Options{
		MinTextRunes: 4,
		MaxTextRunes: 10,
		Now:          fixedClock(time.Now()),
	})

	tests := []struct {
		name string
		text string
	}{
		{name: "empty", text: " \n\t "},
		{name: "too long", text: "очень длинный вопрос"},
		{name: "too short", text: "я?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.Check(message(tt.text, 1))
			if result.Accepted {
				t.Fatalf("expected %q to be rejected", tt.text)
			}
			if result.Explanation == "" {
				t.Fatal("expected explanation")
			}
		})
	}
}

func TestCheckAllowsCommandsAndShortServiceReplies(t *testing.T) {
	filter := New(Options{
		MinTextRunes:    4,
		RateLimitMax:    1,
		RateLimitWindow: time.Minute,
		Now:             fixedClock(time.Now()),
	})

	first := filter.Check(message("первый вопрос", 1))
	if !first.Accepted {
		t.Fatalf("expected first message to be accepted: %s", first.Explanation)
	}

	for _, text := range []string{"/x", "да", "ок"} {
		t.Run(text, func(t *testing.T) {
			result := filter.Check(message(text, 1))
			if !result.Accepted {
				t.Fatalf("expected %q to bypass free-text limits: %s", text, result.Explanation)
			}
		})
	}
}

func TestCheckRejectsNoise(t *testing.T) {
	tests := []string{
		"!!!!!!",
		"123456",
		"аааааа",
		"аааааб",
		"абабабаб",
		"abcabcabc",
		"bcdfgh",
		"бвгджз",
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			result := newTestFilter().Check(message(text, 1))
			if result.Accepted {
				t.Fatalf("expected %q to be rejected", text)
			}
			if result.Explanation == "" {
				t.Fatal("expected explanation")
			}
		})
	}
}

func TestCheckRejectsDuplicateNormalizedText(t *testing.T) {
	now := time.Now()
	filter := New(Options{
		DuplicateTextWindow: time.Minute,
		RateLimitMax:        10,
		Now:                 fixedClock(now),
	})

	first := filter.Check(message("Как составить иск?", 1))
	if !first.Accepted {
		t.Fatalf("expected first message to be accepted: %s", first.Explanation)
	}

	duplicate := filter.Check(message(" как\nсоставить   иск? ", 1))
	if duplicate.Accepted {
		t.Fatal("expected duplicate normalized message to be rejected")
	}
}

func TestCheckAllowsDuplicateAfterWindow(t *testing.T) {
	now := time.Now()
	filter := New(Options{
		DuplicateTextWindow: time.Second,
		RateLimitMax:        10,
		Now: func() time.Time {
			return now
		},
	})

	first := filter.Check(message("Как составить иск?", 1))
	if !first.Accepted {
		t.Fatalf("expected first message to be accepted: %s", first.Explanation)
	}

	now = now.Add(2 * time.Second)
	next := filter.Check(message("Как составить иск?", 1))
	if !next.Accepted {
		t.Fatalf("expected duplicate after window to be accepted: %s", next.Explanation)
	}
}

func TestCheckRejectsTooManyMessagesFromSameUser(t *testing.T) {
	now := time.Now()
	filter := New(Options{
		RateLimitMax:    2,
		RateLimitWindow: time.Minute,
		Now: func() time.Time {
			return now
		},
	})

	for _, text := range []string{"первый вопрос", "второй вопрос"} {
		result := filter.Check(message(text, 1))
		if !result.Accepted {
			t.Fatalf("expected %q to be accepted: %s", text, result.Explanation)
		}
	}

	rejected := filter.Check(message("третий вопрос", 1))
	if rejected.Accepted {
		t.Fatal("expected third message inside rate window to be rejected")
	}

	now = now.Add(time.Minute + time.Second)
	accepted := filter.Check(message("четвертый вопрос", 1))
	if !accepted.Accepted {
		t.Fatalf("expected message after rate window to be accepted: %s", accepted.Explanation)
	}
}

func newTestFilter() *Filter {
	return New(Options{
		RateLimitMax: 10,
		Now:          fixedClock(time.Now()),
	})
}

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time {
		return now
	}
}

func message(text string, userID int64) domain.IncomingMessage {
	return domain.IncomingMessage{
		Text:      text,
		MessageID: "message-1",
		Target: domain.Target{
			ChatID: 100,
			UserID: userID,
		},
	}
}
