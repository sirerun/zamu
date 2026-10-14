package zamu_test

import (
	"testing"
	"time"

	"github.com/sirerun/zamu"
)

// These are policy fixtures, not a measurement of acoustic recognition quality.
func TestEnglishThinkingPauseFixtures(t *testing.T) {
	fixtures := []struct{ prefix, rest string }{
		{"Can you help me with", "my schedule?"}, {"I want to", "plan tomorrow."},
		{"Please remind me when", "the meeting starts."}, {"Find a", "quiet place."},
		{"Could you check the", "latest reply?"}, {"I was thinking...", "we could leave early."},
		{"Let's do this if", "there is enough time."}, {"I need coffee and", "some breakfast."},
		{"I'd like tea or", "maybe water."}, {"I can do that but", "I need an hour."},
		{"Cancel it because", "the time changed."}, {"I feel tired so", "let's rest."},
		{"Let me think, um", "Tuesday works."}, {"I'd say, uh", "around ten."},
		{"The main thing is—", "I need more time."}, {"Send the note to", "my colleague."},
		{"Compare these with", "the original."}, {"We should wait because", "the result isn't ready."},
		{"Let's start when", "everyone arrives."}, {"Maybe we can...", "try again tomorrow."},
	}
	for i, f := range fixtures {
		t.Run(f.prefix, func(t *testing.T) {
			h := setup(t)
			h.utterance(f.prefix, 0, 16000)
			h.barrier()
			h.c.advance(time.Duration(1+i%3) * time.Second)
			h.barrier()
			select {
			case turn := <-h.a.turns:
				t.Fatalf("premature turn %+v", turn)
			default:
			}
			h.utterance(f.rest, 16000, 32000)
			h.barrier()
			h.c.advance(time.Second)
			got := h.turn()
			if got.Text != f.prefix+" "+f.rest {
				t.Fatalf("turn=%+v", got)
			}
		})
	}
}
func TestEnglishCompleteTurnFixtures(t *testing.T) {
	for _, text := range []string{"Hello.", "Good morning.", "What time is it?", "Tell me a story.", "How does this work?", "Please check my calendar.", "I am ready.", "Let's try again.", "That sounds good.", "I disagree.", "Could you explain?", "Make it shorter.", "Read that again.", "I need a break.", "What do you think?", "Please continue.", "Wait, change the date.", "No, next Friday.", "Thanks for helping.", "I have another question."} {
		t.Run(text, func(t *testing.T) {
			h := setup(t)
			h.utterance(text, 0, 16000)
			h.barrier()
			h.c.advance(time.Second)
			if got := h.turn(); got.Text != text || got.Sequence != 1 {
				t.Fatalf("turn=%+v", got)
			}
		})
	}
}
func TestBackchannelCannotHideInstructions(t *testing.T) {
	for _, text := range []string{"yeah, but change the date", "okay cancel that", "yes and send it", "mm-hmm wait", "uh huh delete it"} {
		if (zamu.EnglishDetector{}).Classify(text, false) != zamu.Substantive {
			t.Fatalf("instruction ignored: %s", text)
		}
	}
}

func TestLabelledBackchannelFixtures(t *testing.T) {
	type fixture struct {
		text     string
		question bool
		want     zamu.Classification
	}
	fixtures := []fixture{}
	for _, text := range []string{"mm-hmm", "mm hmm", "mhm", "uh-huh", "uh huh", "yeah", "yes", "okay", "ok", "Okay!"} {
		fixtures = append(fixtures, fixture{text, false, zamu.Backchannel}, fixture{text, true, zamu.Substantive})
	}
	for _, text := range []string{"wait", "stop", "wait a minute", "yeah, but stop the upload", "okay change the date", "yes please cancel that", "no", "please continue", "I disagree", "do not send it"} {
		fixtures = append(fixtures, fixture{text, false, zamu.Substantive})
	}
	fixtures = append(fixtures, fixture{"", false, zamu.Noise}, fixture{"...", false, zamu.Noise})
	for _, f := range fixtures {
		t.Run(f.text+map[bool]string{true: "_answer", false: "_unprompted"}[f.question], func(t *testing.T) {
			if got := (zamu.EnglishDetector{}).Classify(f.text, f.question); got != f.want {
				t.Fatalf("got %s want %s", got, f.want)
			}
		})
	}
}
