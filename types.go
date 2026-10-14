// Package zamu coordinates interruptible voice conversations without owning
// agent tools, credentials, durable conversation history, or microphone devices.
package zamu

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnsupported  = errors.New("unsupported capability")
	ErrUnauthorized = errors.New("voice admission invalid")
	ErrOverload     = errors.New("voice queue capacity exceeded")
	ErrContract     = errors.New("adapter contract violated")
	ErrClosed       = errors.New("session closed")
	ErrRunning      = errors.New("session already running")
	ErrUncertain    = errors.New("provider execution uncertain")
)

type Format struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

type AudioFrame struct {
	Generation   uint64 `json:"generation"`
	Sequence     uint64 `json:"sequence"`
	TextSequence uint64 `json:"text_sequence"`
	Offset       uint64 `json:"offset"` // Interleaved sample frames, not bytes.
	Samples      uint64 `json:"samples"`
	Format       Format `json:"format"`
	PCM          []byte `json:"pcm"`
	PhraseEnd    bool   `json:"phrase_end,omitempty"`
}

type PlaybackCommand struct {
	Kind       string `json:"kind"` // pause, resume, flush, drain
	Generation uint64 `json:"generation"`
	Token      uint64 `json:"token,omitempty"`
}

type PlaybackReceipt struct {
	Generation  uint64 `json:"generation"`
	Token       uint64 `json:"token,omitempty"`
	Played      uint64 `json:"played"`
	SampleRate  int    `json:"sample_rate"`
	Interrupted bool   `json:"interrupted"`
	Uncertain   bool   `json:"uncertain"`
}

type InputKind string

const (
	SpeechStarted       InputKind = "speech_started"
	SpeechStopped       InputKind = "speech_stopped"
	TranscriptUpdated   InputKind = "transcript_updated"
	TranscriptFinal     InputKind = "transcript_final"
	RecognitionProgress InputKind = "recognition_progress"
	Gap                 InputKind = "gap"
	Activated           InputKind = "activated"
	CaptureReset        InputKind = "capture_reset" // Same admission, fresh capture stream after mute.
	Muted               InputKind = "muted"
	Unmuted             InputKind = "unmuted"
	Stopped             InputKind = "stopped"
	AdmissionClosed     InputKind = "admission_closed"
	Unavailable         InputKind = "unavailable"
	PlaybackProgress    InputKind = "playback_progress"
	ClearInterruption   InputKind = "clear_interruption"
)

// InputEvent is ordered within a stream. All capture offsets are sample frames
// at Config.CaptureRate. RecognitionProgress certifies all results through End.
// Activated requires a fresh StreamID and binding after any unrecoverable gap.
type InputEvent struct {
	Kind      InputKind       `json:"kind"`
	SessionID string          `json:"session_id"`
	StreamID  string          `json:"stream_id"`
	Source    string          `json:"source"`
	Sequence  uint64          `json:"sequence"`
	SegmentID string          `json:"segment_id,omitempty"`
	Text      string          `json:"text,omitempty"`
	Start     uint64          `json:"start,omitempty"`
	End       uint64          `json:"end,omitempty"`
	Binding   string          `json:"binding,omitempty"`
	Receipt   PlaybackReceipt `json:"receipt,omitempty"`
}

type Turn struct {
	Sequence      uint64          `json:"sequence"` // Monotonic within SessionID; host admission fence.
	ID            string          `json:"id"`
	SessionID     string          `json:"session_id"`
	Binding       string          `json:"binding"`
	Text          string          `json:"text"`
	Start         uint64          `json:"start"`
	End           uint64          `json:"end"`
	PriorPlayback PlaybackReceipt `json:"prior_playback"`
}

type SpeechChunk struct {
	Sequence uint64 `json:"sequence"`
	Text     string `json:"text"`
}

type ReplyEvent struct {
	Chunk          SpeechChunk `json:"chunk"`
	AwaitingAnswer bool        `json:"awaiting_answer"`
}

type SynthesisRequest struct {
	SessionID     string
	Generation    uint64
	Voice         string
	StyleRevision string
}

type Listener interface {
	Listen(context.Context, func(InputEvent) error) error
}
type Agent interface {
	Respond(context.Context, Turn, func(ReplyEvent) error) error
}
type Synthesizer interface {
	Synthesize(context.Context, SynthesisRequest, <-chan SpeechChunk, func(AudioFrame) error) error
}

// Player must fence generations atomically against control and write, retain
// paused audio, and return the actual rendered cursor. Drain acknowledges actual
// output completion. Write may not reinterpret the declared PCM format.
type Player interface {
	Write(context.Context, AudioFrame) error
	Control(context.Context, PlaybackCommand) (PlaybackReceipt, error)
}

type Classification string

const (
	Backchannel Classification = "backchannel"
	Substantive Classification = "substantive"
	Noise       Classification = "noise"
	Unknown     Classification = "unknown"
)

type Detector interface {
	Classify(text string, awaitingAnswer bool) Classification
	Complete(text string) bool
}

type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

type Config struct {
	SessionID             string
	Binding               string
	Source                string
	Voice                 string
	StyleRevision         string
	CaptureRate           int
	EndpointSilence       time.Duration
	ThinkingSilence       time.Duration
	RecognitionTimeout    time.Duration
	ClassificationTimeout time.Duration
	MaxUtterance          time.Duration
	IdleTimeout           time.Duration
	MaxSession            time.Duration
	OperationTimeout      time.Duration
	CloseTimeout          time.Duration
	MaxTextBytes          int
	MaxFrameBytes         int
	MaxQueuedTurns        int
	MaxQueuedPhrases      int
	EventBuffer           int
	InitiallyActive       bool // Host has already admitted the actor and binding.
	Clock                 Clock
}

type Components struct {
	Listener Listener
	Agent    Agent
	Speech   Synthesizer
	Player   Player
	Detector Detector // Default uses conservative English acknowledgements.
}

type Event struct {
	NeedsAdmission bool            `json:"needs_admission,omitempty"`
	Kind           string          `json:"kind"`
	SessionID      string          `json:"session_id"`
	TurnID         string          `json:"turn_id,omitempty"`
	Generation     uint64          `json:"generation"`
	Text           string          `json:"text,omitempty"` // Human/assistant text is never diagnostic logging.
	Receipt        PlaybackReceipt `json:"receipt,omitempty"`
	Error          string          `json:"error,omitempty"`
}
