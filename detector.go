package zamu

import "strings"

// EnglishDetector only accepts a closed, finalized utterance as an
// acknowledgement; the session enforces those boundaries before calling it.
type EnglishDetector struct{}

func (EnglishDetector) Classify(text string, awaitingAnswer bool) Classification {
	text = strings.ToLower(strings.Trim(strings.TrimSpace(text), ".!?,"))
	if awaitingAnswer {
		return Substantive
	}
	switch text {
	case "mm-hmm", "mm hmm", "mhm", "uh-huh", "uh huh", "yeah", "yes", "okay", "ok":
		return Backchannel
	case "":
		return Noise
	default:
		return Substantive
	}
}

func (EnglishDetector) Complete(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimRight(text, ".!?,"))
	words := strings.Fields(lower)
	if len(words) == 0 {
		return false
	}
	switch words[len(words)-1] {
	case "and", "or", "but", "because", "if", "when", "so", "the", "a", "to", "with", "um", "uh":
		return false
	}
	return !strings.HasSuffix(text, "...") && !strings.HasSuffix(text, "—")
}
