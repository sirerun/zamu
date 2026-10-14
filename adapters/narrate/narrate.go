// Package narrate connects Zamu's synthesis boundary to Narrate's persistent
// local speech client. Credentials and endpoint selection remain host-owned.
package narrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/narrate-it/narrate/speech/local"
	"github.com/sirerun/zamu"
)

type Synthesizer struct{ client *local.Client }

func New(cfg local.Config) (*Synthesizer, error) {
	c, err := local.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("configure local speech: %w", err)
	}
	return &Synthesizer{client: c}, nil
}
func (s *Synthesizer) Synthesize(ctx context.Context, r zamu.SynthesisRequest, chunks <-chan zamu.SpeechChunk, emit func(zamu.AudioFrame) error) error {
	if chunks == nil || emit == nil {
		return zamu.ErrContract
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	input := make(chan local.TextChunk)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(input)
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-chunks:
				if !ok {
					return
				}
				select {
				case input <- local.TextChunk{Sequence: chunk.Sequence, Text: chunk.Text}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	err := s.client.Synthesize(ctx, local.Request{Generation: r.Generation, Voice: r.Voice, Chunks: input}, func(f local.Frame) error {
		if f.Samples < 0 {
			return zamu.ErrContract
		}
		return emit(zamu.AudioFrame{Generation: f.Generation, Sequence: f.Sequence, TextSequence: f.TextSequence, Offset: f.Offset, Samples: uint64(f.Samples), PCM: f.PCM, Format: zamu.Format{Encoding: f.Format.Encoding, SampleRate: f.Format.SampleRate, Channels: f.Format.Channels}, PhraseEnd: f.PhraseEnd})
	})
	cancel()
	<-done
	switch {
	case errors.Is(err, local.ErrAuthorization):
		return errors.Join(zamu.ErrUnauthorized, err)
	case errors.Is(err, local.ErrOverload):
		return errors.Join(zamu.ErrOverload, err)
	case errors.Is(err, local.ErrProtocol):
		return errors.Join(zamu.ErrContract, err)
	default:
		return err
	}
}
