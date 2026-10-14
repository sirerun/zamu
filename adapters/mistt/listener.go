// Package mistt adapts MISTT's additive microphone conversation stream. It owns
// no microphone, logs no audio, and never reconnects or replays input implicitly.
package mistt

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"sync"

	"github.com/sirerun/MISTT/conversation"
	misttv1 "github.com/sirerun/MISTT/gen/mistt/v1"
	"github.com/sirerun/zamu"
)

type Config struct {
	Client    *conversation.Client
	SessionID string
	Binding   string
	Source    string
}
type control struct {
	event  zamu.InputEvent
	result chan error
}
type Listener struct {
	cfg      Config
	mu       sync.Mutex
	stream   *conversation.Stream
	started  bool
	ready    chan struct{}
	done     chan struct{}
	controls chan control
	sendSlot chan struct{}
}

func New(cfg Config) (*Listener, error) {
	if cfg.Client == nil || cfg.SessionID == "" || cfg.Binding == "" {
		return nil, zamu.ErrUnauthorized
	}
	if cfg.Source == "" {
		cfg.Source = "microphone"
	}
	if cfg.Source != "microphone" {
		return nil, zamu.ErrUnsupported
	}
	return &Listener{cfg: cfg, ready: make(chan struct{}), done: make(chan struct{}), controls: make(chan control, 16), sendSlot: make(chan struct{}, 1)}, nil
}
func (l *Listener) Listen(ctx context.Context, emit func(zamu.InputEvent) error) error {
	if emit == nil {
		return zamu.ErrContract
	}
	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return zamu.ErrRunning
	}
	l.started = true
	l.mu.Unlock()
	defer close(l.done)
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return err
	}
	id[6] = (id[6] & 15) | 64
	id[8] = (id[8] & 63) | 128
	streamID := fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
	stream, err := l.cfg.Client.Open(ctx, streamID)
	if err != nil {
		return err
	}
	defer stream.Close()
	l.mu.Lock()
	l.stream = stream
	l.mu.Unlock()
	type result struct {
		event *misttv1.StreamResponse
		err   error
	}
	received := make(chan result, 1)
	readerDone := make(chan struct{})
	child, cancel := context.WithCancel(ctx)
	defer func() { cancel(); stream.Close(); <-readerDone }()
	go func() {
		defer close(readerDone)
		for {
			e, err := stream.Recv()
			select {
			case received <- result{e, err}:
			case <-child.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	var sequence, wireSequence uint64
	send := func(e zamu.InputEvent) error {
		sequence++
		e.SessionID = l.cfg.SessionID
		e.Binding = l.cfg.Binding
		e.Source = l.cfg.Source
		e.StreamID = streamID
		e.Sequence = sequence
		return emit(e)
	}
	admitted := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c := <-l.controls:
			if !admitted {
				c.result <- zamu.ErrUnauthorized
				continue
			}
			err := send(c.event)
			c.result <- err
			if err != nil {
				return err
			}
		case received := <-received:
			if received.err != nil {
				if received.err == io.EOF {
					return fmt.Errorf("MISTT closed without terminal result: %w", zamu.ErrContract)
				}
				return received.err
			}
			e := received.event
			if e == nil || e.SessionId != streamID || e.Sequence != wireSequence+1 {
				return fmt.Errorf("MISTT stream ordering: %w", zamu.ErrContract)
			}
			wireSequence = e.Sequence
			var in zamu.InputEvent
			switch event := e.Payload.(type) {
			case *misttv1.StreamResponse_SessionStarted:
				if admitted || event.SessionStarted.CaptureMode != misttv1.CaptureMode_CAPTURE_MODE_MICROPHONE || event.SessionStarted.SampleRateHz != 16000 || event.SessionStarted.Channels != 1 || event.SessionStarted.Encoding != "s16le" {
					return zamu.ErrContract
				}
				admitted = true
				in.Kind = zamu.Activated
			case *misttv1.StreamResponse_SpeechStarted:
				if event.SpeechStarted.CaptureOffsetNs%62500 != 0 {
					return zamu.ErrContract
				}
				if event.SpeechStarted.Source != l.cfg.Source {
					return zamu.ErrUnauthorized
				}
				in.Kind = zamu.SpeechStarted
				in.Start = event.SpeechStarted.CaptureOffsetNs / 62500
			case *misttv1.StreamResponse_SpeechStopped:
				if event.SpeechStopped.CaptureOffsetNs%62500 != 0 {
					return zamu.ErrContract
				}
				if event.SpeechStopped.Source != l.cfg.Source {
					return zamu.ErrUnauthorized
				}
				in.Kind = zamu.SpeechStopped
				in.End = event.SpeechStopped.CaptureOffsetNs / 62500
			case *misttv1.StreamResponse_Transcription:
				t := event.Transcription
				if t.StartOffsetNs/62500 != t.StartSample || t.EndOffsetNs/62500 != t.EndSample || t.StartOffsetNs%62500 != 0 || t.EndOffsetNs%62500 != 0 {
					return zamu.ErrContract
				}
				if t.Source != l.cfg.Source {
					return zamu.ErrUnauthorized
				}
				in.Kind = zamu.TranscriptUpdated
				if t.IsFinal {
					in.Kind = zamu.TranscriptFinal
				}
				in.SegmentID = t.SegmentId
				in.Text = t.Text
				in.Start = t.StartSample
				in.End = t.EndSample
			case *misttv1.StreamResponse_RecognitionProgress:
				p := event.RecognitionProgress
				if p.FinalizedThroughOffsetNs%62500 != 0 || p.FinalizedThroughOffsetNs/62500 != p.FinalizedThroughSample {
					return zamu.ErrContract
				}
				if p.Source != l.cfg.Source {
					return zamu.ErrUnauthorized
				}
				in.Kind = zamu.RecognitionProgress
				in.End = p.FinalizedThroughSample
			case *misttv1.StreamResponse_SessionGap:
				if event.SessionGap.Source != l.cfg.Source {
					return zamu.ErrUnauthorized
				}
				in.Kind = zamu.Gap
			case *misttv1.StreamResponse_SessionStopped:
				if event.SessionStopped.GrpcCode != 0 {
					return fmt.Errorf("MISTT terminal error code %d: %w", event.SessionStopped.GrpcCode, zamu.ErrUncertain)
				}
				return nil
			default:
				return zamu.ErrUnsupported
			}
			if !admitted {
				return zamu.ErrUnauthorized
			}
			if err := send(in); err != nil {
				return err
			}
			if in.Kind == zamu.Activated {
				close(l.ready)
			}
		}
	}
}

// Feed carries native controls and playback receipts through the same ordered
// listener callback as recognition. The host must authenticate the caller.
func (l *Listener) Feed(ctx context.Context, e zamu.InputEvent) error {
	select {
	case <-l.ready:
	case <-l.done:
		return zamu.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	switch e.Kind {
	case zamu.Muted, zamu.Unmuted, zamu.Stopped, zamu.AdmissionClosed, zamu.Unavailable, zamu.PlaybackProgress, zamu.ClearInterruption:
	default:
		return zamu.ErrUnsupported
	}
	c := control{event: e, result: make(chan error, 1)}
	select {
	case l.controls <- c:
	case <-l.done:
		return zamu.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-c.result:
		return err
	case <-l.done:
		return zamu.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (l *Listener) write(ctx context.Context, fn func(*conversation.Stream) error) error {
	select {
	case l.sendSlot <- struct{}{}:
	case <-l.done:
		return zamu.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-l.sendSlot }()
	select {
	case <-l.ready:
	case <-l.done:
		return zamu.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	l.mu.Lock()
	stream := l.stream
	l.mu.Unlock()
	result := make(chan error, 1)
	go func() { result <- fn(stream) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		stream.Close()
		return ctx.Err()
	case <-l.done:
		return zamu.ErrClosed
	}
}
func (l *Listener) SendPCM(ctx context.Context, sequence, startSample uint64, pcm []byte) error {
	return l.write(ctx, func(s *conversation.Stream) error { return s.SendPCM(sequence, startSample, pcm) })
}
func (l *Listener) Activity(ctx context.Context, speaking bool, sample uint64) error {
	return l.write(ctx, func(s *conversation.Stream) error { return s.Activity(speaking, sample) })
}
func (l *Listener) Checkpoint(ctx context.Context, sample uint64) error {
	return l.write(ctx, func(s *conversation.Stream) error { return s.Checkpoint(sample) })
}
func (l *Listener) Close() error {
	l.mu.Lock()
	stream := l.stream
	l.mu.Unlock()
	if stream != nil {
		return stream.Close()
	}
	return nil
}

// Ready closes after the initial authenticated activation event is delivered.
func (l *Listener) Ready() <-chan struct{} { return l.ready }
