package zamu_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirerun/zamu"
)

func expectTerminal(t *testing.T, h *harness, want error) {
	t.Helper()
	select {
	case err := <-h.done:
		h.done <- err
		if !errors.Is(err, want) {
			t.Fatalf("error=%v want=%v", err, want)
		}
	case <-h.ctx.Done():
		t.Fatal("invalid event did not terminate")
	}
}
func TestInputBoundaryFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*harness)
		event   zamu.InputEvent
		want    error
	}{
		{"wrong_binding", nil, zamu.InputEvent{Kind: zamu.Activated, Binding: "other"}, zamu.ErrUnauthorized},
		{"unmuted_capture_reset", nil, zamu.InputEvent{Kind: zamu.CaptureReset, Binding: "conversation", StreamID: "new"}, zamu.ErrUnauthorized},
		{"unknown_event", nil, zamu.InputEvent{Kind: "unexpected"}, zamu.ErrUnsupported},
		{"missing_segment", func(h *harness) { h.input(zamu.InputEvent{Kind: zamu.SpeechStarted}) }, zamu.InputEvent{Kind: zamu.TranscriptFinal, Text: "Hi", End: 1}, zamu.ErrContract},
		{"inverted_segment", func(h *harness) { h.input(zamu.InputEvent{Kind: zamu.SpeechStarted}) }, zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "a", Text: "Hi", Start: 10, End: 1}, zamu.ErrContract},
		{"stop_before_start", func(h *harness) { h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 10}) }, zamu.InputEvent{Kind: zamu.SpeechStopped, End: 1}, zamu.ErrContract},
		{"regressed_coverage", func(h *harness) { h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 10}) }, zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 9}, zamu.ErrContract},
		{"old_interval", func(h *harness) {
			h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 10})
			h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 10})
		}, zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "a", Text: "Hi", Start: 1, End: 11}, zamu.ErrContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setup(t)
			if tc.prepare != nil {
				tc.prepare(h)
			}
			h.input(tc.event)
			expectTerminal(t, h, tc.want)
		})
	}
}
func TestSessionExpiryAndIdleRequireFreshAdmission(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		h := setup(t)
		h.barrier()
		h.c.advance(2 * time.Hour)
		expectTerminal(t, h, zamu.ErrClosed)
	})
	t.Run("idle", func(t *testing.T) {
		h := setup(t)
		h.barrier()
		h.c.advance(5 * time.Minute)
		h.event("idle_timeout")
		h.utterance("Old stream cannot run", 0, 16000)
		h.barrier()
		h.c.advance(time.Second)
		h.barrier()
		select {
		case turn := <-h.a.turns:
			t.Fatalf("inactive turn %+v", turn)
		default:
		}
	})
}
func TestClosedSessionRejectsControls(t *testing.T) {
	h := setup(t)
	h.barrier()
	if err := h.s.Run(context.Background()); !errors.Is(err, zamu.ErrRunning) {
		t.Fatal(err)
	}
	if err := h.s.Close(h.ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.s.Interrupt(h.ctx); !errors.Is(err, zamu.ErrClosed) {
		t.Fatal(err)
	}
	if err := h.s.Close(h.ctx); err != nil {
		t.Fatal(err)
	}
}
func TestControlNoiseAndBackchannels(t *testing.T) {
	d := zamu.EnglishDetector{}
	for _, text := range []string{"mm-hmm", "mm hmm", "mhm", "uh-huh", "uh huh", "yeah", "yes", "okay", "ok"} {
		if d.Classify(text, false) != zamu.Backchannel {
			t.Fatal(text)
		}
		if d.Classify(text, true) != zamu.Substantive {
			t.Fatal(text)
		}
	}
	if d.Classify("", false) != zamu.Noise {
		t.Fatal("empty speech classified as turn")
	}
}
