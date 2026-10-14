package zamu

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var errWritePending = errors.New("internal: playback write pending")

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

type command struct {
	kind   string
	result chan error
}
type message struct {
	input      *InputEvent
	reply      *ReplyEvent
	frame      *AudioFrame
	generation uint64
	receipt    *PlaybackReceipt
	ack        chan error
	kind       string
	err        error
}

type Session struct {
	cfg        Config
	deps       Components
	mu         sync.Mutex
	running    bool
	closed     bool
	cancel     context.CancelFunc
	commands   chan command
	events     chan Event
	done       chan struct{}
	cleanupErr error
	agentSlots chan struct{}
}

func New(cfg Config, deps Components) (*Session, error) {
	if cfg.SessionID == "" || cfg.Binding == "" || cfg.Source == "" {
		return nil, fmt.Errorf("session identity, binding and source required: %w", ErrUnauthorized)
	}
	if deps.Listener == nil || deps.Agent == nil || deps.Speech == nil || deps.Player == nil {
		return nil, fmt.Errorf("all voice components required: %w", ErrUnsupported)
	}
	if cfg.Voice == "" {
		cfg.Voice = "anna"
	}
	if cfg.CaptureRate == 0 {
		cfg.CaptureRate = 16000
	}
	if cfg.EndpointSilence == 0 {
		cfg.EndpointSilence = 900 * time.Millisecond
	}
	if cfg.ThinkingSilence == 0 {
		cfg.ThinkingSilence = 2500 * time.Millisecond
	}
	if cfg.RecognitionTimeout == 0 {
		cfg.RecognitionTimeout = 3 * time.Second
	}
	if cfg.ClassificationTimeout == 0 {
		cfg.ClassificationTimeout = 1600 * time.Millisecond
	}
	if cfg.MaxUtterance == 0 {
		cfg.MaxUtterance = 120 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 5 * time.Minute
	}
	if cfg.MaxSession == 0 {
		cfg.MaxSession = 2 * time.Hour
	}
	if cfg.OperationTimeout == 0 {
		cfg.OperationTimeout = 2 * time.Second
	}
	if cfg.CloseTimeout == 0 {
		cfg.CloseTimeout = 3 * time.Second
	}
	if cfg.MaxTextBytes == 0 {
		cfg.MaxTextBytes = 16 << 10
	}
	if cfg.MaxFrameBytes == 0 {
		cfg.MaxFrameBytes = 192000
	}
	if cfg.MaxQueuedTurns == 0 {
		cfg.MaxQueuedTurns = 4
	}
	if cfg.MaxQueuedPhrases == 0 {
		cfg.MaxQueuedPhrases = 64
	}
	if cfg.EventBuffer == 0 {
		cfg.EventBuffer = 128
	}
	if cfg.Clock == nil {
		cfg.Clock = realClock{}
	}
	if cfg.CaptureRate <= 0 || cfg.MaxTextBytes <= 0 || cfg.MaxFrameBytes <= 0 || cfg.EventBuffer <= 0 || cfg.MaxQueuedTurns <= 0 || cfg.MaxQueuedPhrases <= 0 {
		return nil, fmt.Errorf("positive limits required: %w", ErrContract)
	}
	for _, d := range []time.Duration{cfg.EndpointSilence, cfg.ThinkingSilence, cfg.RecognitionTimeout, cfg.ClassificationTimeout, cfg.MaxUtterance, cfg.IdleTimeout, cfg.MaxSession, cfg.OperationTimeout, cfg.CloseTimeout} {
		if d <= 0 {
			return nil, fmt.Errorf("positive deadlines required: %w", ErrContract)
		}
	}
	if deps.Detector == nil {
		deps.Detector = EnglishDetector{}
	}
	return &Session{cfg: cfg, deps: deps, commands: make(chan command, 8), events: make(chan Event, cfg.EventBuffer), done: make(chan struct{}), agentSlots: make(chan struct{}, cfg.MaxQueuedTurns)}, nil
}

func (s *Session) Events() <-chan Event { return s.events }
func (s *Session) Interrupt(ctx context.Context) error {
	s.mu.Lock()
	running, closed := s.running, s.closed
	s.mu.Unlock()
	if !running || closed {
		return ErrClosed
	}
	c := command{kind: "interrupt", result: make(chan error, 1)}
	select {
	case s.commands <- c:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return ErrClosed
	}
	select {
	case err := <-c.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return ErrClosed
	}
}
func (s *Session) Close(ctx context.Context) error {
	s.mu.Lock()
	if !s.running && !s.closed {
		s.closed = true
		close(s.events)
		close(s.done)
	} else if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	select {
	case <-s.done:
		s.mu.Lock()
		err := s.cleanupErr
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Session) Run(ctx context.Context) (result error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	if s.running {
		s.mu.Unlock()
		return ErrRunning
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	s.mu.Unlock()
	r := runtimeState{s: s, active: s.cfg.InitiallyActive, segments: map[string]segment{}, streams: map[string]bool{}, messages: make(chan message, 64), lastActivity: s.cfg.Clock.Now(), started: s.cfg.Clock.Now()}
	var wg sync.WaitGroup
	defer func() {
		var cleanupErr error
		cancel()
		if r.outputCancel != nil {
			r.outputCancel()
		}
		joined := make(chan struct{})
		go func() { wg.Wait(); close(joined) }()
		select {
		case <-joined:
		case <-time.After(s.cfg.CloseTimeout):
			cleanupErr = fmt.Errorf("adapter cleanup exceeded deadline: %w", context.DeadlineExceeded)
			result = errors.Join(result, cleanupErr)
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), s.cfg.OperationTimeout)
		_, err := s.deps.Player.Control(cleanupCtx, PlaybackCommand{Kind: "flush", Generation: r.generation})
		cleanupCancel()
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("final playback flush: %w", err))
			result = errors.Join(result, cleanupErr)
		}
		s.mu.Lock()
		s.cleanupErr = cleanupErr
		s.closed = true
		s.running = false
		s.cancel = nil
		close(s.events)
		close(s.done)
		s.mu.Unlock()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.deps.Listener.Listen(ctx, func(in InputEvent) error { return r.send(ctx, message{input: &in}) })
		_ = r.send(ctx, message{kind: "listener_done", err: err})
	}()
	for {
		delay := r.nextDelay()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c := <-s.commands:
			err := r.interrupt(ctx, "explicit")
			c.result <- err
			if err != nil {
				return err
			}
		case msg := <-r.messages:
			if msg.input != nil {
				if err := r.input(ctx, *msg.input); err != nil {
					return err
				}
			} else {
				err := r.output(ctx, msg, &wg)
				if errors.Is(err, errWritePending) {
					break
				}
				if msg.ack != nil {
					msg.ack <- err
				}
				if err != nil {
					return err
				}
			}
		case <-s.cfg.Clock.After(delay):
		}
		if err := r.tick(ctx, &wg); err != nil {
			return err
		}
	}
}

type segment struct {
	text       string
	start, end uint64
	final      bool
}
type runtimeState struct {
	sealed                                                map[string]segment
	sealedOrder                                           []string
	textSizes                                             map[uint64]int
	drain                                                 chan struct{}
	s                                                     *Session
	active, muted, speaking, hasUtterance, awaitingAnswer bool
	stream                                                string
	streams                                               map[string]bool
	seq, watermark, start, end                            uint64
	segments                                              map[string]segment
	speechAt, stoppedAt, lastActivity, started            time.Time
	generation, turnNumber, pauseToken                    uint64
	pausedAt                                              time.Time
	receipt                                               PlaybackReceipt
	outputCancel                                          context.CancelFunc
	outputContext                                         context.Context
	chunks                                                chan SpeechChunk
	textSequence, audioSequence, audioOffset              uint64
	textBytes                                             int
	format                                                Format
	outputActive                                          bool
	messages                                              chan message
}

func (r *runtimeState) send(ctx context.Context, m message) error {
	select {
	case r.messages <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (r *runtimeState) emit(e Event) error {
	e.SessionID = r.s.cfg.SessionID
	e.Generation = r.generation
	select {
	case r.s.events <- e:
		return nil
	default:
		return fmt.Errorf("event consumer stalled: %w", ErrOverload)
	}
}
func (r *runtimeState) control(ctx context.Context, kind string) (PlaybackReceipt, error) {
	call, cancel := context.WithTimeout(ctx, r.s.cfg.OperationTimeout)
	defer cancel()
	receipt, err := r.s.deps.Player.Control(call, PlaybackCommand{Kind: kind, Generation: r.generation, Token: r.pauseToken})
	if err != nil {
		return receipt, fmt.Errorf("playback %s: %w", kind, err)
	}
	if receipt.Generation != r.generation || (kind == "pause" || kind == "resume") && receipt.Token != r.pauseToken {
		return receipt, fmt.Errorf("mismatched playback acknowledgement: %w", ErrContract)
	}
	if receipt.Played < r.receipt.Played && receipt.Generation == r.receipt.Generation {
		return receipt, fmt.Errorf("playback cursor regressed: %w", ErrContract)
	}
	return receipt, nil
}
func (r *runtimeState) interrupt(ctx context.Context, reason string) error {
	if r.outputCancel != nil {
		r.outputCancel()
		r.outputCancel = nil
	}
	r.outputActive = false
	r.chunks = nil
	r.pausedAt = time.Time{}
	receipt, err := r.control(ctx, "flush")
	if err != nil {
		return err
	}
	receipt.Interrupted = true
	r.receipt = receipt
	r.generation++
	return r.emit(Event{Kind: "interrupted", Text: reason, Receipt: receipt})
}
func (r *runtimeState) clearUtterance() {
	if r.sealed == nil {
		r.sealed = map[string]segment{}
	}
	for id, seg := range r.segments {
		if seg.final {
			r.sealed[id] = seg
			r.sealedOrder = append(r.sealedOrder, id)
		}
	}
	for len(r.sealedOrder) > 1024 {
		delete(r.sealed, r.sealedOrder[0])
		r.sealedOrder = r.sealedOrder[1:]
	}
	r.hasUtterance = false
	r.speaking = false
	r.segments = map[string]segment{}
	r.stoppedAt = time.Time{}
	r.speechAt = time.Time{}
}
func (r *runtimeState) text() string {
	list := make([]segment, 0, len(r.segments))
	for _, seg := range r.segments {
		list = append(list, seg)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].start < list[j].start })
	parts := make([]string, 0, len(list))
	for _, seg := range list {
		parts = append(parts, seg.text)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
func (r *runtimeState) covered() bool {
	if r.speaking || !r.hasUtterance || r.watermark < r.end {
		return false
	}
	for _, seg := range r.segments {
		if !seg.final || seg.end > r.end {
			return false
		}
	}
	return true
}
func (r *runtimeState) input(ctx context.Context, in InputEvent) error {
	if in.SessionID != r.s.cfg.SessionID || in.Source != r.s.cfg.Source {
		return fmt.Errorf("input binding mismatch: %w", ErrUnauthorized)
	}
	if in.Kind == Activated || in.Kind == CaptureReset {
		if in.Kind == CaptureReset && (!r.active || !r.muted) {
			return ErrUnauthorized
		}
		if in.Binding != r.s.cfg.Binding || in.StreamID == "" {
			return ErrUnauthorized
		}
		if r.stream == in.StreamID && in.Sequence <= r.seq {
			return nil
		}
		if r.streams[in.StreamID] {
			return fmt.Errorf("stream identity reused: %w", ErrContract)
		}
		if len(r.streams) >= 1024 {
			return ErrOverload
		}
		if r.outputActive && in.Kind == Activated {
			if err := r.interrupt(ctx, "new admission"); err != nil {
				return err
			}
		}
		r.clearUtterance()
		r.pausedAt = time.Time{}
		r.sealed = map[string]segment{}
		r.sealedOrder = nil
		r.stream = in.StreamID
		r.streams[in.StreamID] = true
		r.seq = in.Sequence
		r.watermark = 0
		r.active = true
		r.muted = false
		r.lastActivity = r.s.cfg.Clock.Now()
		return r.emit(Event{Kind: "listening"})
	}
	if r.stream == "" && r.active && in.StreamID != "" {
		r.stream = in.StreamID
		r.streams[in.StreamID] = true
	}
	if in.StreamID != r.stream || in.Sequence <= r.seq {
		return nil
	}
	r.seq = in.Sequence
	switch in.Kind {
	case AdmissionClosed, Stopped, Unavailable:
		r.active = false
		r.clearUtterance()
		if err := r.interrupt(ctx, string(in.Kind)); err != nil {
			return err
		}
		return r.emit(Event{Kind: string(in.Kind)})
	case Muted:
		if !r.pausedAt.IsZero() && r.outputActive {
			receipt, err := r.control(ctx, "resume")
			if err != nil {
				return err
			}
			r.receipt = receipt
			r.pausedAt = time.Time{}
		}
		r.pausedAt = time.Time{}
		r.muted = true
		r.clearUtterance()
		return r.emit(Event{Kind: "muted"})
	case Unmuted:
		r.muted = false
		return r.emit(Event{Kind: "listening"})
	case PlaybackProgress:
		if in.Receipt.Generation == r.generation && in.Receipt.Played >= r.receipt.Played {
			r.receipt = in.Receipt
			return r.emit(Event{Kind: "playback", Receipt: r.receipt})
		}
		return nil
	}
	if !r.active || r.muted {
		return nil
	}
	now := r.s.cfg.Clock.Now()
	switch in.Kind {
	case SpeechStarted:
		if r.speaking {
			return nil
		}
		if !r.hasUtterance {
			r.start = in.Start
			r.hasUtterance = true
			r.speechAt = now
		}
		r.speaking = true
		r.stoppedAt = time.Time{}
		r.lastActivity = now
		if r.outputActive && r.pausedAt.IsZero() {
			r.pauseToken++
			receipt, err := r.control(ctx, "pause")
			if err != nil {
				return err
			}
			r.receipt = receipt
			r.pausedAt = now
			return r.emit(Event{Kind: "paused", Receipt: receipt})
		}
	case SpeechStopped:
		if !r.hasUtterance {
			return nil
		}
		if in.End < r.start {
			return ErrContract
		}
		r.speaking = false
		r.end = in.End
		r.stoppedAt = now
		r.lastActivity = now
	case TranscriptUpdated, TranscriptFinal:
		if prior, ok := r.segments[in.SegmentID]; ok && prior == (segment{text: strings.TrimSpace(in.Text), start: in.Start, end: in.End, final: in.Kind == TranscriptFinal}) {
			return nil
		}
		if prior, ok := r.sealed[in.SegmentID]; ok {
			if prior == (segment{text: strings.TrimSpace(in.Text), start: in.Start, end: in.End, final: in.Kind == TranscriptFinal}) {
				return nil
			}
			return fmt.Errorf("changed disposed final segment: %w", ErrContract)
		}
		if in.Start < r.watermark {
			return fmt.Errorf("late transcript behind finalized coverage: %w", ErrContract)
		}
		if !r.hasUtterance {
			return nil
		}
		if in.SegmentID == "" || in.End < in.Start || !utf8.ValidString(in.Text) {
			return ErrContract
		}
		// An idle checkpoint can finish after new speech starts. Its text
		// (including recognizer hallucinations on silence) is not that turn.
		if in.End <= r.start {
			return nil
		}
		old, exists := r.segments[in.SegmentID]
		seg := segment{text: strings.TrimSpace(in.Text), start: in.Start, end: in.End, final: in.Kind == TranscriptFinal}
		if exists && old == seg {
			return nil
		}
		if exists && old.final || in.Start < r.watermark {
			return fmt.Errorf("revision behind finalized recognition: %w", ErrContract)
		}
		if in.Start < r.start {
			r.start = in.Start
		}
		r.segments[in.SegmentID] = seg
		if len(r.text()) > r.s.cfg.MaxTextBytes || len(r.segments) > 1024 {
			return ErrOverload
		}
	case RecognitionProgress:
		if in.End < r.watermark {
			return fmt.Errorf("recognition coverage regressed: %w", ErrContract)
		}
		for _, seg := range r.segments {
			if seg.start < in.End && !seg.final {
				return fmt.Errorf("coverage advanced across provisional segment: %w", ErrContract)
			}
		}
		r.watermark = in.End
	case Gap:
		r.active = false
		r.clearUtterance()
		if err := r.interrupt(ctx, "transcript gap"); err != nil {
			return err
		}
		return r.emit(Event{Kind: "repeat_required", NeedsAdmission: true, Text: "I missed part of that. Please start again."})
	case ClearInterruption:
		return r.interrupt(ctx, "confirmed speech")
	default:
		return fmt.Errorf("unknown input kind %q: %w", in.Kind, ErrUnsupported)
	}
	return nil
}
func (r *runtimeState) output(ctx context.Context, m message, wg *sync.WaitGroup) error {
	if m.kind == "listener_done" {
		if m.err != nil {
			return fmt.Errorf("listener ended: %w", m.err)
		}
		return ErrClosed
	}
	if m.generation != r.generation || !r.outputActive {
		return nil
	}
	if m.err != nil {

		if err := r.interrupt(ctx, "response failure"); err != nil {
			return err
		}
		return r.emit(Event{Kind: "response_failed", Error: m.err.Error()})
	}
	if m.reply != nil {
		if r.chunks == nil {
			return ErrContract
		}
		ch := m.reply.Chunk
		if ch.Sequence != r.textSequence+1 || ch.Text == "" || !utf8.ValidString(ch.Text) {
			return fmt.Errorf("invalid stable chunk: %w", ErrContract)
		}
		if r.textBytes+len(ch.Text) > r.s.cfg.MaxTextBytes {
			return ErrOverload
		}
		r.textBytes += len(ch.Text)
		r.textSizes[ch.Sequence] = len(ch.Text)
		r.textSequence = ch.Sequence
		r.awaitingAnswer = m.reply.AwaitingAnswer
		select {
		case r.chunks <- ch:
		default:
			return fmt.Errorf("synthesis text queue full: %w", ErrOverload)
		}
		return r.emit(Event{Kind: "generated", Text: ch.Text})
	}
	if m.frame != nil {
		f := *m.frame
		if f.Generation != r.generation || f.Sequence != r.audioSequence+1 || f.Offset != r.audioOffset || f.Format.Encoding != "pcm_s16le" || f.Format.Channels <= 0 || f.Format.SampleRate <= 0 {
			return fmt.Errorf("invalid audio frame: %w", ErrContract)
		}
		if f.Samples > uint64(r.s.cfg.MaxFrameBytes/2/f.Format.Channels) || len(f.PCM) != int(f.Samples)*2*f.Format.Channels || len(f.PCM) > r.s.cfg.MaxFrameBytes {
			return ErrOverload
		}
		if r.audioSequence > 0 && f.Format != r.format {
			return fmt.Errorf("format changed mid-response: %w", ErrContract)
		}
		if f.TextSequence == 0 || f.TextSequence > r.textSequence {
			return ErrContract
		}
		if m.kind != "write_done" {
			call, cancel := context.WithTimeout(r.outputContext, r.s.cfg.OperationTimeout)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer cancel()
				m.kind = "write_done"
				m.err = r.s.deps.Player.Write(call, f)
				_ = r.send(ctx, m)
			}()
			// The synthesis callback retains ownership of this frame until its
			// deferred acknowledgement. Capture/control remain responsive.
			return errWritePending
		}
		r.audioSequence = f.Sequence
		r.audioOffset += f.Samples
		r.format = f.Format
		if f.PhraseEnd {
			size, ok := r.textSizes[f.TextSequence]
			if !ok {
				return ErrContract
			}
			r.textBytes -= size
			delete(r.textSizes, f.TextSequence)
		}
		return r.emit(Event{Kind: "queued"})
	}
	switch m.kind {
	case "agent_done":
		if r.chunks != nil {
			close(r.chunks)
			r.chunks = nil
		}
	case "speech_done":
		close(r.drain)
		return r.emit(Event{Kind: "synthesized"})
	case "playback_done":
		if m.receipt == nil || m.receipt.Generation != r.generation || m.receipt.Played < r.receipt.Played {
			return ErrContract
		}
		r.receipt = *m.receipt
		r.outputActive = false
		return r.emit(Event{Kind: "completed", Receipt: r.receipt})
	}
	return nil
}
func (r *runtimeState) nextDelay() time.Duration { return 10 * time.Millisecond }
func (r *runtimeState) tick(ctx context.Context, wg *sync.WaitGroup) error {
	now := r.s.cfg.Clock.Now()
	if now.Sub(r.started) >= r.s.cfg.MaxSession {
		return fmt.Errorf("voice session renewal required: %w", ErrClosed)
	}
	if r.active && !r.hasUtterance && !r.outputActive && !r.awaitingAnswer && now.Sub(r.lastActivity) >= r.s.cfg.IdleTimeout {
		r.active = false
		return r.emit(Event{Kind: "idle_timeout"})
	}
	if !r.active || r.muted {
		return nil
	}
	if r.hasUtterance && now.Sub(r.speechAt) >= r.s.cfg.MaxUtterance {
		if r.outputActive || !r.pausedAt.IsZero() {
			if err := r.interrupt(ctx, "utterance limit"); err != nil {
				return err
			}
		}
		r.clearUtterance()
		r.active = false
		return r.emit(Event{Kind: "repeat_required", NeedsAdmission: true, Text: "Please continue in a new turn."})
	}
	if !r.pausedAt.IsZero() {
		if r.covered() {
			switch r.s.deps.Detector.Classify(r.text(), r.awaitingAnswer) {
			case Backchannel, Noise:
				if !r.outputActive {
					r.pausedAt = time.Time{}
					r.clearUtterance()
					return r.emit(Event{Kind: "backchannel", Receipt: r.receipt})
				}
				receipt, err := r.control(ctx, "resume")
				if err != nil {
					return err
				}
				r.receipt = receipt
				r.pausedAt = time.Time{}
				r.clearUtterance()
				return r.emit(Event{Kind: "resumed", Receipt: receipt})
			case Substantive:
				if err := r.interrupt(ctx, "substantive speech"); err != nil {
					return err
				}
			}
		}
		if !r.pausedAt.IsZero() && now.Sub(r.pausedAt) >= r.s.cfg.ClassificationTimeout {
			if err := r.interrupt(ctx, "classification timeout"); err != nil {
				return err
			}
		}
	}
	if !r.hasUtterance || r.speaking || r.stoppedAt.IsZero() {
		return nil
	}
	silence := r.s.cfg.EndpointSilence
	text := r.text()
	if !r.s.deps.Detector.Complete(text) {
		silence = r.s.cfg.ThinkingSilence
	}
	if now.Sub(r.stoppedAt) < silence {
		return nil
	}
	if !r.covered() {
		if now.Sub(r.stoppedAt) >= silence+r.s.cfg.RecognitionTimeout {
			r.clearUtterance()
			r.active = false
			return r.emit(Event{Kind: "repeat_required", NeedsAdmission: true, Text: "I didn't catch the complete thought. Please start again."})
		}
		return nil
	}
	if text == "" {
		r.clearUtterance()
		return nil
	}
	if !r.s.deps.Detector.Complete(text) {
		if now.Sub(r.stoppedAt) >= 5*time.Second {
			r.clearUtterance()
			return r.emit(Event{Kind: "repeat_required", Text: "Please finish your thought."})
		}
		return nil
	}
	if r.outputActive {
		if err := r.interrupt(ctx, "new turn"); err != nil {
			return err
		}
	}
	select {
	case r.s.agentSlots <- struct{}{}:
	default:
		r.clearUtterance()
		return r.emit(Event{Kind: "turn_rejected", Text: text, Error: "voice host is busy; please repeat when ready"})
	}
	r.turnNumber++
	turn := Turn{Sequence: r.turnNumber, ID: fmt.Sprintf("%s/%d", r.s.cfg.SessionID, r.turnNumber), SessionID: r.s.cfg.SessionID, Binding: r.s.cfg.Binding, Text: text, Start: r.start, End: r.end, PriorPlayback: r.receipt}
	r.clearUtterance()
	r.awaitingAnswer = false
	if err := r.emit(Event{Kind: "committed", TurnID: turn.ID, Text: turn.Text}); err != nil {
		<-r.s.agentSlots
		return err
	}
	r.begin(ctx, wg, turn)
	return nil
}
func (r *runtimeState) begin(ctx context.Context, wg *sync.WaitGroup, turn Turn) {
	r.generation++
	generation := r.generation
	child, cancel := context.WithCancel(ctx)
	r.outputCancel = cancel
	r.outputContext = child
	r.outputActive = true
	r.chunks = make(chan SpeechChunk, r.s.cfg.MaxQueuedPhrases)
	chunks := r.chunks
	r.textSequence = 0
	r.audioSequence = 0
	r.audioOffset = 0
	r.receipt = PlaybackReceipt{Generation: generation}
	r.textBytes = 0
	r.textSizes = map[uint64]int{}
	drain := make(chan struct{})
	r.drain = drain
	wg.Add(3)
	go func() {
		defer wg.Done()
		select {
		case <-child.Done():
			return
		case <-drain:
		}
		receipt, err := r.s.deps.Player.Control(child, PlaybackCommand{Kind: "drain", Generation: generation})
		_ = r.send(child, message{generation: generation, kind: "playback_done", receipt: &receipt, err: err})
	}()
	go func() {
		defer wg.Done()
		defer func() { <-r.s.agentSlots }()
		err := r.s.deps.Agent.Respond(child, turn, func(reply ReplyEvent) error {
			if len(reply.Chunk.Text) > r.s.cfg.MaxTextBytes {
				return ErrOverload
			}
			return r.deliver(child, message{generation: generation, reply: &reply})
		})
		_ = r.send(child, message{generation: generation, kind: "agent_done", err: err})
	}()
	go func() {
		defer wg.Done()
		req := SynthesisRequest{SessionID: r.s.cfg.SessionID, Generation: generation, Voice: r.s.cfg.Voice, StyleRevision: r.s.cfg.StyleRevision}
		err := r.s.deps.Speech.Synthesize(child, req, chunks, func(frame AudioFrame) error {
			if len(frame.PCM) > r.s.cfg.MaxFrameBytes {
				return ErrOverload
			}
			// Callback cancellation may return before an admitted writer finishes.
			// Own the bounded PCM so the synthesizer can immediately reuse its buffer.
			frame.PCM = append([]byte(nil), frame.PCM...)
			return r.deliver(child, message{generation: generation, frame: &frame})
		})
		_ = r.send(child, message{generation: generation, kind: "speech_done", err: err})
	}()
}

func (r *runtimeState) deliver(ctx context.Context, m message) error {
	m.ack = make(chan error, 1)
	if err := r.send(ctx, m); err != nil {
		return err
	}
	select {
	case err := <-m.ack:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
