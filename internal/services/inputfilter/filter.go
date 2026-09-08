package inputfilter

import (
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/GogaAPPS/AiDe-Bot/internal/domain"
)

const (
	defaultMinTextRunes = 4
	defaultMaxTextRunes = 2000
	defaultRateLimitMax = 5
	defaultRateLimitFor = 10 * time.Second
	defaultDuplicateFor = 30 * time.Second
)

type Options struct {
	MinTextRunes        int
	MaxTextRunes        int
	RateLimitMax        int
	RateLimitWindow     time.Duration
	DuplicateTextWindow time.Duration
	Now                 func() time.Time
}

type Filter struct {
	options Options

	mu    sync.Mutex
	users map[int64]*userState
}

type userState struct {
	seenAt   []time.Time
	lastText map[string]time.Time
}

type Result struct {
	Message     domain.IncomingMessage
	Accepted    bool
	Explanation string
}

func DefaultOptions() Options {
	return Options{
		MinTextRunes:        defaultMinTextRunes,
		MaxTextRunes:        defaultMaxTextRunes,
		RateLimitMax:        defaultRateLimitMax,
		RateLimitWindow:     defaultRateLimitFor,
		DuplicateTextWindow: defaultDuplicateFor,
		Now:                 time.Now,
	}
}

func New(options Options) *Filter {
	defaults := DefaultOptions()
	if options.MinTextRunes <= 0 {
		options.MinTextRunes = defaults.MinTextRunes
	}
	if options.MaxTextRunes <= 0 {
		options.MaxTextRunes = defaults.MaxTextRunes
	}
	if options.RateLimitMax <= 0 {
		options.RateLimitMax = defaults.RateLimitMax
	}
	if options.RateLimitWindow <= 0 {
		options.RateLimitWindow = defaults.RateLimitWindow
	}
	if options.DuplicateTextWindow <= 0 {
		options.DuplicateTextWindow = defaults.DuplicateTextWindow
	}
	if options.Now == nil {
		options.Now = defaults.Now
	}

	return &Filter{
		options: options,
		users:   make(map[int64]*userState),
	}
}

func (f *Filter) Check(message domain.IncomingMessage) Result {
	message.Text = Normalize(message.Text)
	if message.Text == "" {
		return reject(message, "Напишите ваш вопрос")
	}

	if isCommand(message.Text) || isShortServiceReply(message.Text) {
		return accept(message)
	}

	if utf8.RuneCountInString(message.Text) > f.options.MaxTextRunes {
		return reject(message, "Сообщение слишком длинное.")
	}
	if explanation, ok := noiseExplanation(message.Text); ok {
		return reject(message, explanation)
	}
	if countMeaningfulRunes(message.Text) < f.options.MinTextRunes {
		return reject(message, "Сообщение слишком короткое.")
	}
	if explanation, ok := f.rejectRepeatedOrFrequent(message); ok {
		return reject(message, explanation)
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
	return strings.HasPrefix(strings.TrimSpace(text), "/")
}

func isShortServiceReply(text string) bool {
	switch strings.ToLower(text) {
	case "да", "нет", "ок", "окей", "хорошо", "готово", "отмена", "назад", "стоп", "продолжить":
		return true
	default:
		return false
	}
}

func countMeaningfulRunes(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

func noiseExplanation(text string) (string, bool) {
	compact := compactLettersAndDigits(text)
	if compact == "" {
		return "В сообщении одни символы. Напишите вопрос словами, например: что нужно сделать или проверить.", true
	}
	if containsOnlyDigits(compact) {
		return "В сообщении только цифры. Добавьте словами, что означает число и какой вопрос нужно решить.", true
	}
	if isSingleRuneRepeated(compact) || hasLongRuneRepeat(compact) || hasDominantRune(compact) || isRepeatedPattern(compact) || isLowDiversity(compact) {
		return "Похоже на повторяющиеся символы. Сформулируйте запрос обычными словами.", true
	}

	return "", false
}

func compactLettersAndDigits(text string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func containsOnlyDigits(text string) bool {
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return text != ""
}

func isSingleRuneRepeated(text string) bool {
	if utf8.RuneCountInString(text) < 3 {
		return false
	}

	var first rune
	for index, r := range text {
		if index == 0 {
			first = r
			continue
		}
		if r != first {
			return false
		}
	}
	return true
}

func hasLongRuneRepeat(text string) bool {
	const maxRun = 5

	var previous rune
	run := 0
	for _, r := range text {
		if r == previous {
			run++
		} else {
			previous = r
			run = 1
		}
		if run >= maxRun {
			return true
		}
	}
	return false
}

func hasDominantRune(text string) bool {
	runes := []rune(text)
	if len(runes) < 6 {
		return false
	}

	counts := make(map[rune]int, len(runes))
	maxCount := 0
	for _, r := range runes {
		counts[r]++
		if counts[r] > maxCount {
			maxCount = counts[r]
		}
	}

	return float64(maxCount)/float64(len(runes)) >= 0.70
}

func isRepeatedPattern(text string) bool {
	runes := []rune(text)
	if len(runes) < 6 {
		return false
	}

	for patternSize := 1; patternSize <= 4 && patternSize <= len(runes)/2; patternSize++ {
		if len(runes)%patternSize != 0 {
			continue
		}

		repeated := true
		for index, r := range runes {
			if r != runes[index%patternSize] {
				repeated = false
				break
			}
		}
		if repeated {
			return true
		}
	}

	return false
}

func isLowDiversity(text string) bool {
	runes := []rune(text)
	if len(runes) < 8 {
		return false
	}

	unique := make(map[rune]struct{}, len(runes))
	for _, r := range runes {
		unique[r] = struct{}{}
	}
	return float64(len(unique))/float64(len(runes)) <= 0.25
}

func (f *Filter) rejectRepeatedOrFrequent(message domain.IncomingMessage) (string, bool) {
	userKey := message.Target.UserID
	if userKey == 0 {
		userKey = message.Target.ChatID
	}
	if userKey == 0 {
		return "", false
	}

	now := f.options.Now()
	f.mu.Lock()
	defer f.mu.Unlock()

	state := f.users[userKey]
	if state == nil {
		state = &userState{lastText: make(map[string]time.Time)}
		f.users[userKey] = state
	}

	state.seenAt = removeExpiredTimes(state.seenAt, now.Add(-f.options.RateLimitWindow))
	for text, seenAt := range state.lastText {
		if seenAt.Before(now.Add(-f.options.DuplicateTextWindow)) {
			delete(state.lastText, text)
		}
	}

	textKey := strings.ToLower(message.Text)
	if seenAt, ok := state.lastText[textKey]; ok && !seenAt.Before(now.Add(-f.options.DuplicateTextWindow)) {
		return "Такой запрос уже был недавно. Подождите ответ или уточните вопрос новыми деталями.", true
	}
	if len(state.seenAt) >= f.options.RateLimitMax {
		return "Слишком много сообщений подряд. Подождите несколько секунд и отправьте один полный запрос.", true
	}

	state.seenAt = append(state.seenAt, now)
	state.lastText[textKey] = now
	return "", false
}

func removeExpiredTimes(values []time.Time, threshold time.Time) []time.Time {
	next := values[:0]
	for _, value := range values {
		if !value.Before(threshold) {
			next = append(next, value)
		}
	}
	return next
}
