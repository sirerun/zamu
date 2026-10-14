package zamu_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirerun/zamu"
)

type clock struct {
	mu    sync.Mutex
	now   time.Time
	ticks chan time.Time
}

func newClock() *clock                                { return &clock{now: time.Unix(1, 0), ticks: make(chan time.Time, 128)} }
func (c *clock) Now() time.Time                       { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) After(time.Duration) <-chan time.Time { return c.ticks }
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	n := c.now
	c.mu.Unlock()
	c.ticks <- n
}

type listener struct{ input chan zamu.InputEvent }

func (l *listener) Listen(ctx context.Context, emit func(zamu.InputEvent) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-l.input:
			if err := emit(e); err != nil {
				return err
			}
		}
	}
}

type agent struct {
	turns   chan zamu.Turn
	replies chan zamu.ReplyEvent
	ends    chan struct{}
}

func (a *agent) Respond(ctx context.Context, t zamu.Turn, emit func(zamu.ReplyEvent) error) error {
	select {
	case a.turns <- t:
	case <-ctx.Done():
		return ctx.Err()
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.ends:
			return nil
		case e := <-a.replies:
			if err := emit(e); err != nil {
				return err
			}
		}
	}
}

type synth struct{}

func (synth) Synthesize(ctx context.Context, r zamu.SynthesisRequest, ch <-chan zamu.SpeechChunk, emit func(zamu.AudioFrame) error) error {
	var seq, offset uint64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c, ok := <-ch:
			if !ok {
				return nil
			}
			seq++
			if err := emit(zamu.AudioFrame{Generation: r.Generation, Sequence: seq, TextSequence: c.Sequence, Offset: offset, Samples: 160, PCM: make([]byte, 320), Format: zamu.Format{Encoding: "pcm_s16le", SampleRate: 16000, Channels: 1}, PhraseEnd: true}); err != nil {
				return err
			}
			offset += 160
		}
	}
}

type player struct {
	blockedWrite chan struct{}
	writeStarted chan struct{}
	failFlush    bool
	mu           sync.Mutex
	commands     []zamu.PlaybackCommand
	frames       []zamu.AudioFrame
	played       map[uint64]uint64
	drained      chan struct{}
}

func (p *player) Write(ctx context.Context, f zamu.AudioFrame) error {
	if p.writeStarted != nil {
		select {
		case p.writeStarted <- struct{}{}:
		default:
		}
	}
	if p.blockedWrite != nil {
		select {
		case <-p.blockedWrite:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.frames = append(p.frames, f)
	return nil
}
func (p *player) Control(ctx context.Context, c zamu.PlaybackCommand) (zamu.PlaybackReceipt, error) {
	if c.Kind == "drain" {
		select {
		case <-ctx.Done():
			return zamu.PlaybackReceipt{}, ctx.Err()
		case <-p.drained:
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.commands = append(p.commands, c)
	if c.Kind == "flush" && p.failFlush {
		return zamu.PlaybackReceipt{}, errors.New("flush failed")
	}
	return zamu.PlaybackReceipt{Generation: c.Generation, Token: c.Token, Played: p.played[c.Generation], SampleRate: 16000}, nil
}

type harness struct {
	generation uint64
	t          *testing.T
	s          *zamu.Session
	l          *listener
	a          *agent
	p          *player
	c          *clock
	done       chan error
	ctx        context.Context
	seq        uint64
	stream     string
}

func setup(t *testing.T) *harness { return setupWithSpeech(t, synth{}) }
func setupWithSpeech(t *testing.T, speech zamu.Synthesizer) *harness {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	h := &harness{t: t, ctx: ctx, l: &listener{make(chan zamu.InputEvent, 64)}, a: &agent{make(chan zamu.Turn, 8), make(chan zamu.ReplyEvent, 8), make(chan struct{}, 8)}, p: &player{played: map[uint64]uint64{}, drained: make(chan struct{})}, c: newClock(), done: make(chan error, 1), stream: "stream-1"}
	var err error
	h.s, err = zamu.New(zamu.Config{SessionID: "session", Binding: "conversation", Source: "microphone", InitiallyActive: true, Clock: h.c}, zamu.Components{Listener: h.l, Agent: h.a, Speech: speech, Player: h.p})
	if err != nil {
		t.Fatal(err)
	}
	go func() { h.done <- h.s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.done:
		case <-time.After(time.Second):
			t.Error("Run failed to terminate")
		}
	})
	return h
}
func (h *harness) input(e zamu.InputEvent) {
	h.seq++
	e.Sequence = h.seq
	e.SessionID = "session"
	e.Source = "microphone"
	if e.StreamID == "" {
		e.StreamID = h.stream
	}
	select {
	case h.l.input <- e:
	case <-h.ctx.Done():
		h.t.Fatal("input timed out")
	}
}
func (h *harness) event(kind string) zamu.Event {
	h.t.Helper()
	for {
		select {
		case e, ok := <-h.s.Events():
			if !ok {
				h.t.Fatalf("closed waiting for %s", kind)
			}
			h.generation = e.Generation
			if e.Kind == kind {
				return e
			}
		case <-h.ctx.Done():
			h.t.Fatalf("timeout waiting for %s", kind)
		}
	}
}
func (h *harness) turn() zamu.Turn {
	h.t.Helper()
	select {
	case t := <-h.a.turns:
		return t
	case <-h.ctx.Done():
		h.t.Fatal("turn timed out")
		return zamu.Turn{}
	}
}
func (h *harness) utterance(text string, start, end uint64) {
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: start})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: text, Text: text, Start: start, End: end})
	h.input(zamu.InputEvent{Kind: zamu.SpeechStopped, End: end})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: end})
}
func (h *harness) barrier() {
	h.input(zamu.InputEvent{Kind: zamu.PlaybackProgress, Receipt: zamu.PlaybackReceipt{Generation: h.generation}})
	h.event("playback")
}
func (h *harness) startAnswer() {
	h.utterance("Tell me a story.", 0, 16000)
	h.barrier()
	h.c.advance(time.Second)
	h.turn()
	h.a.replies <- zamu.ReplyEvent{Chunk: zamu.SpeechChunk{Sequence: 1, Text: "Once upon a time."}}
	h.event("queued")
}

func TestWaitsForFinalCoverage(t *testing.T) {
	h := setup(t)
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptUpdated, SegmentID: "a", Text: "Turn on", End: 16000})
	h.input(zamu.InputEvent{Kind: zamu.SpeechStopped, End: 16000})
	h.barrier()
	h.c.advance(time.Second)
	h.barrier()
	select {
	case turn := <-h.a.turns:
		t.Fatalf("premature turn: %+v", turn)
	default:
	}
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "a", Text: "Turn off the light", End: 16000})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 16000})
	if got := h.turn().Text; got != "Turn off the light" {
		t.Fatalf("turn=%q", got)
	}
}
func TestBackchannelResumesWithoutTurn(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.utterance("mm-hmm", 16000, 20000)
	h.event("resumed")
	select {
	case turn := <-h.a.turns:
		t.Fatalf("backchannel submitted: %+v", turn)
	default:
	}
	h.p.mu.Lock()
	defer h.p.mu.Unlock()
	var pause, resume zamu.PlaybackCommand
	for _, c := range h.p.commands {
		if c.Kind == "pause" {
			pause = c
		}
		if c.Kind == "resume" {
			resume = c
		}
	}
	if pause.Generation != resume.Generation || pause.Token != resume.Token || resume.Kind == "" {
		t.Fatalf("invalid resume: %+v %+v", pause, resume)
	}
}
func TestMixedAcknowledgementIsWholeTurn(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.utterance("yeah, but stop the upload", 16000, 30000)
	h.event("interrupted")
	h.c.advance(time.Second)
	got := h.turn()
	if got.Text != "yeah, but stop the upload" {
		t.Fatalf("lost instruction: %q", got.Text)
	}
	if !got.PriorPlayback.Interrupted {
		t.Fatal("missing interrupted receipt")
	}
}
func TestUnfinishedBackchannelDoesNotResume(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 16000})
	h.event("paused")
	h.input(zamu.InputEvent{Kind: zamu.TranscriptUpdated, SegmentID: "b", Text: "yeah", Start: 16000, End: 20000})
	h.c.advance(2 * time.Second)
	h.event("interrupted")
	h.p.mu.Lock()
	defer h.p.mu.Unlock()
	for _, c := range h.p.commands {
		if c.Kind == "resume" {
			t.Fatal("resumed on partial acknowledgement")
		}
	}
}
func TestGapRequiresFreshAdmission(t *testing.T) {
	h := setup(t)
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "old", Text: "Do", End: 1000})
	h.input(zamu.InputEvent{Kind: zamu.Gap})
	h.event("repeat_required")
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "tail", Text: "delete it", Start: 1000, End: 2000})
	h.c.advance(5 * time.Second)
	h.stream = "stream-2"
	h.seq = 0
	h.input(zamu.InputEvent{Kind: zamu.Activated, Binding: "conversation"})
	h.event("listening")
	h.utterance("Do not delete it", 0, 16000)
	h.barrier()
	h.c.advance(time.Second)
	if got := h.turn().Text; got != "Do not delete it" {
		t.Fatal(got)
	}
}
func TestThinkingPauseRevokesCandidate(t *testing.T) {
	h := setup(t)
	h.utterance("Please send the", 0, 10000)
	h.barrier()
	h.c.advance(2 * time.Second)
	h.barrier()
	select {
	case <-h.a.turns:
		t.Fatal("submitted unfinished thought")
	default:
	}
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 11000})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "continuation", Text: "report tomorrow", Start: 11000, End: 20000})
	h.input(zamu.InputEvent{Kind: zamu.SpeechStopped, End: 20000})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 20000})
	h.barrier()
	h.c.advance(time.Second)
	if got := h.turn().Text; got != "Please send the report tomorrow" {
		t.Fatal(got)
	}
}
func TestRecognitionTimeoutDiscardsTurn(t *testing.T) {
	h := setup(t)
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptUpdated, SegmentID: "a", Text: "delete", End: 100})
	h.input(zamu.InputEvent{Kind: zamu.SpeechStopped, End: 100})
	h.barrier()
	h.c.advance(5 * time.Second)
	h.event("repeat_required")
	select {
	case <-h.a.turns:
		t.Fatal("submitted partial turn")
	default:
	}
}
func TestCoverageCannotFinalizePartial(t *testing.T) {
	h := setup(t)
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptUpdated, SegmentID: "a", Text: "go", End: 100})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 100})
	select {
	case err := <-h.done:
		if !errors.Is(err, zamu.ErrContract) {
			t.Fatal(err)
		}
		h.done <- err
	case <-h.ctx.Done():
		t.Fatal("did not reject false completeness")
	}
}
func TestCompletedPlaybackUsesActualReceipt(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.p.mu.Lock()
	h.p.played[1] = 160
	h.p.mu.Unlock()
	h.a.ends <- struct{}{}
	close(h.p.drained)
	e := h.event("completed")
	if e.Receipt.Played != 160 {
		t.Fatalf("receipt=%+v", e.Receipt)
	}
}
func TestExplicitInterruptPreservesPlayedBoundary(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.p.mu.Lock()
	h.p.played[1] = 80
	h.p.mu.Unlock()
	if err := h.s.Interrupt(h.ctx); err != nil {
		t.Fatal(err)
	}
	e := h.event("interrupted")
	if e.Receipt.Played != 80 || !e.Receipt.Interrupted {
		t.Fatalf("receipt=%+v", e.Receipt)
	}
}
func TestConstructionAndCloseBeforeRun(t *testing.T) {
	if _, err := zamu.New(zamu.Config{}, zamu.Components{}); !errors.Is(err, zamu.ErrUnauthorized) {
		t.Fatal(err)
	}
	l := &listener{make(chan zamu.InputEvent)}
	a := &agent{}
	p := &player{played: map[uint64]uint64{}}
	s, err := zamu.New(zamu.Config{SessionID: "s", Binding: "b", Source: "microphone"}, zamu.Components{Listener: l, Agent: a, Speech: synth{}, Player: p})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background()); !errors.Is(err, zamu.ErrClosed) {
		t.Fatal(err)
	}
}

func TestPunctuationOnlyIsNotComplete(t *testing.T) {
	for _, text := range []string{"...", "?", ",", "!!!", "   "} {
		t.Run(text, func(t *testing.T) {
			if (zamu.EnglishDetector{}).Complete(text) {
				t.Fatal("punctuation is not a thought")
			}
		})
	}
}
func TestCommittedTranscriptCannotChange(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "Tell me a story.", Text: "Delete the upload", Start: 0, End: 16000})
	select {
	case err := <-h.done:
		if !errors.Is(err, zamu.ErrContract) {
			t.Fatal(err)
		}
		h.done <- err
	case <-h.ctx.Done():
		t.Fatal("late revision accepted")
	}
}
func TestPlaybackCompletionDoesNotSubmitPendingBackchannel(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 16000})
	h.event("paused")
	h.a.ends <- struct{}{}
	close(h.p.drained)
	h.event("completed")
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "ack", Text: "mm-hmm", Start: 16000, End: 20000})
	h.input(zamu.InputEvent{Kind: zamu.SpeechStopped, End: 20000})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 20000})
	h.event("backchannel")
	h.c.advance(time.Second)
	select {
	case turn := <-h.a.turns:
		t.Fatalf("submitted acknowledgement: %+v", turn)
	default:
	}
}
func TestUtteranceLimitFlushesPendingPause(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 16000})
	h.event("paused")
	h.c.advance(121 * time.Second)
	h.event("repeat_required")
	h.p.mu.Lock()
	defer h.p.mu.Unlock()
	if h.p.commands[len(h.p.commands)-1].Kind != "flush" {
		t.Fatalf("stranded pause: %+v", h.p.commands)
	}
}

type stalledSpeech struct{}

func (stalledSpeech) Synthesize(ctx context.Context, _ zamu.SynthesisRequest, _ <-chan zamu.SpeechChunk, _ func(zamu.AudioFrame) error) error {
	<-ctx.Done()
	return ctx.Err()
}
func TestAggregateTextBudget(t *testing.T) {
	h := setupWithSpeech(t, stalledSpeech{})
	h.utterance("Tell me a story.", 0, 16000)
	h.barrier()
	h.c.advance(time.Second)
	h.turn()
	h.a.replies <- zamu.ReplyEvent{Chunk: zamu.SpeechChunk{Sequence: 1, Text: strings.Repeat("x", 10000)}}
	h.event("generated")
	h.a.replies <- zamu.ReplyEvent{Chunk: zamu.SpeechChunk{Sequence: 2, Text: strings.Repeat("y", 10000)}}
	select {
	case err := <-h.done:
		if !errors.Is(err, zamu.ErrOverload) {
			t.Fatal(err)
		}
		h.done <- err
	case <-h.ctx.Done():
		t.Fatal("aggregate budget unenforced")
	}
}
func TestCloseReportsCleanupFailure(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.p.mu.Lock()
	h.p.failFlush = true
	h.p.mu.Unlock()
	if err := h.s.Close(h.ctx); err == nil || !strings.Contains(err.Error(), "flush failed") {
		t.Fatalf("close error=%v", err)
	}
}

func TestMuteAfterCompletedPauseDoesNotResume(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 16000})
	h.event("paused")
	h.a.ends <- struct{}{}
	close(h.p.drained)
	h.event("completed")
	h.input(zamu.InputEvent{Kind: zamu.Muted})
	h.event("muted")
	h.p.mu.Lock()
	defer h.p.mu.Unlock()
	for _, c := range h.p.commands {
		if c.Kind == "resume" {
			t.Fatal("resumed completed generation")
		}
	}
}
func TestFreshAdmissionClearsCompletedPause(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 16000})
	h.event("paused")
	h.a.ends <- struct{}{}
	close(h.p.drained)
	h.event("completed")
	h.stream = "fresh"
	h.seq = 0
	h.input(zamu.InputEvent{Kind: zamu.Activated, Binding: "conversation"})
	h.event("listening")
	h.utterance("yeah", 0, 16000)
	h.barrier()
	h.c.advance(time.Second)
	if got := h.turn().Text; got != "yeah" {
		t.Fatal(got)
	}
}
func TestDuplicateFinalAfterCoverageIsIdempotent(t *testing.T) {
	h := setup(t)
	h.utterance("Hello", 0, 16000)
	h.barrier()
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "Hello", Text: "Hello", End: 16000})
	h.barrier()
	h.c.advance(time.Second)
	if got := h.turn().Text; got != "Hello" {
		t.Fatal(got)
	}
}

func TestCaptureResetAfterMutePreservesResponse(t *testing.T) {
	h := setup(t)
	h.startAnswer()
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 16000})
	h.event("paused")
	h.input(zamu.InputEvent{Kind: zamu.Muted})
	h.event("muted")
	h.stream = "capture-2"
	h.seq = 0
	h.input(zamu.InputEvent{Kind: zamu.CaptureReset, Binding: "conversation"})
	e := h.event("listening")
	if e.Generation != h.generation {
		t.Fatalf("generation changed: %+v", e)
	}
	h.a.replies <- zamu.ReplyEvent{Chunk: zamu.SpeechChunk{Sequence: 2, Text: "Still here."}}
	h.event("generated")
	h.p.mu.Lock()
	defer h.p.mu.Unlock()
	for _, c := range h.p.commands {
		if c.Kind == "flush" {
			t.Fatal("capture reset flushed output")
		}
	}
}

type blockingHost struct {
	entered chan struct{}
	release chan struct{}
}

func (a blockingHost) Respond(context.Context, zamu.Turn, func(zamu.ReplyEvent) error) error {
	close(a.entered)
	<-a.release
	return nil
}
func TestOutstandingHostCallsRejectFurtherTurnExplicitly(t *testing.T) {
	// A host that ignores cancellation must not accumulate unbounded turns.
	a := blockingHost{make(chan struct{}), make(chan struct{})}
	l := &listener{make(chan zamu.InputEvent, 16)}
	c := newClock()
	p := &player{played: map[uint64]uint64{}, drained: make(chan struct{})}
	s, err := zamu.New(zamu.Config{SessionID: "session", Binding: "conversation", Source: "microphone", InitiallyActive: true, Clock: c, MaxQueuedTurns: 1}, zamu.Components{Listener: l, Agent: a, Speech: synth{}, Player: p})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	defer func() {
		close(a.release)
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("session did not stop")
		}
	}()
	run := &harness{t: t, s: s, l: l, c: c, ctx: ctx, stream: "s"}
	run.utterance("Hello", 0, 16000)
	run.barrier()
	c.advance(time.Second)
	run.event("committed")
	<-a.entered
	run.utterance("Another request", 16000, 32000)
	run.event("interrupted")
	run.barrier()
	c.advance(time.Second)
	e := run.event("turn_rejected")
	if e.Text != "Another request" || e.Error == "" {
		t.Fatalf("missing disposition: %+v", e)
	}
}

func TestPlaybackBackpressureDoesNotBlockInterruption(t *testing.T) {
	h := setup(t)
	h.p.blockedWrite = make(chan struct{})
	h.p.writeStarted = make(chan struct{}, 1)
	h.utterance("Tell me a story.", 0, 16000)
	h.barrier()
	h.c.advance(time.Second)
	h.turn()
	h.a.replies <- zamu.ReplyEvent{Chunk: zamu.SpeechChunk{Sequence: 1, Text: "Once upon a time."}}
	select {
	case <-h.p.writeStarted:
	case <-h.ctx.Done():
		t.Fatal("write never started")
	}
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 16000})
	h.event("paused")
	h.c.advance(1700 * time.Millisecond)
	h.event("interrupted")
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "new", Text: "Actually tell me the weather.", Start: 16000, End: 32000})
	h.input(zamu.InputEvent{Kind: zamu.SpeechStopped, End: 32000})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 32000})
	h.barrier()
	h.c.advance(time.Second)
	if got := h.turn().Text; got != "Actually tell me the weather." {
		t.Fatal(got)
	}
}

func TestLateIdleRecognitionCannotPrependNewSpeech(t *testing.T) {
	h := setup(t)
	h.input(zamu.InputEvent{Kind: zamu.SpeechStarted, Start: 80000})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "idle", Text: "Thank you for watching", Start: 0, End: 80000})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 80000})
	h.input(zamu.InputEvent{Kind: zamu.TranscriptFinal, SegmentID: "speech", Text: "Do not send it", Start: 80000, End: 96000})
	h.input(zamu.InputEvent{Kind: zamu.SpeechStopped, End: 96000})
	h.input(zamu.InputEvent{Kind: zamu.RecognitionProgress, End: 96000})
	h.barrier()
	h.c.advance(time.Second)
	if got := h.turn().Text; got != "Do not send it" {
		t.Fatalf("idle contamination: %q", got)
	}
}
