package narrate_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/narrate-it/narrate/speech/local"
	"github.com/sirerun/zamu"
	adapter "github.com/sirerun/zamu/adapters/narrate"
)

func TestPCMAndPhraseBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer explicit-token" {
			t.Error("missing credential")
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req["voice"] != "anna" || req["generation"] != float64(4) {
			t.Error(req)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		enc := json.NewEncoder(w)
		for _, v := range []any{
			map[string]any{"type": "format", "generation": 4, "text_sequence": 1, "format": map[string]any{"encoding": "pcm_s16le", "sample_rate": 24000, "channels": 1}},
			map[string]any{"type": "frame", "generation": 4, "text_sequence": 1, "sequence": 1, "offset": 0, "samples": 2, "pcm": []byte{1, 0, 2, 0}},
			map[string]any{"type": "done", "generation": 4, "text_sequence": 1, "sequence": 1, "offset": 2},
		} {
			if err := enc.Encode(v); err != nil {
				t.Error(err)
			}
		}
	}))
	defer server.Close()
	s, err := adapter.New(local.Config{Endpoint: server.URL, Token: "explicit-token"})
	if err != nil {
		t.Fatal(err)
	}
	chunks := make(chan zamu.SpeechChunk, 1)
	chunks <- zamu.SpeechChunk{Sequence: 1, Text: "Hello"}
	close(chunks)
	var frames []zamu.AudioFrame
	err = s.Synthesize(context.Background(), zamu.SynthesisRequest{Generation: 4, Voice: "anna"}, chunks, func(f zamu.AudioFrame) error { frames = append(frames, f); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 || frames[0].Samples != 2 || frames[0].Format.SampleRate != 24000 || !frames[1].PhraseEnd || frames[1].Offset != 2 {
		t.Fatalf("frames=%+v", frames)
	}
}
func TestUnauthorizedDoesNotWaitForMoreChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer server.Close()
	s, err := adapter.New(local.Config{Endpoint: server.URL, Token: "explicit-token"})
	if err != nil {
		t.Fatal(err)
	}
	chunks := make(chan zamu.SpeechChunk, 1)
	chunks <- zamu.SpeechChunk{Sequence: 1, Text: "Hello"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = s.Synthesize(ctx, zamu.SynthesisRequest{Generation: 1, Voice: "anna"}, chunks, func(zamu.AudioFrame) error { return nil }); !errors.Is(err, zamu.ErrUnauthorized) {
		t.Fatal(err)
	}
}
