package openrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirerun/zamu"
)

func TestStreamingSpeechAndUsage(t *testing.T) {
	var usage Usage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing authentication")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": keepalive\n\ndata: {\"choices\":[{\"delta\":{\"reasoning\":\"private\",\"content\":\"Hello. How are you?\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":6,\"cost\":0.001}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	a, err := New(Config{Endpoint: server.URL, Token: "secret", Model: "explicit-model", OnUsage: func(u Usage) { usage = u }})
	if err != nil {
		t.Fatal(err)
	}
	var replies []zamu.ReplyEvent
	err = a.Respond(context.Background(), zamu.Turn{ID: "1", Text: "Hi"}, func(e zamu.ReplyEvent) error { replies = append(replies, e); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 2 || replies[0].Chunk.Text != "Hello." || replies[1].Chunk.Sequence != 2 || !replies[1].AwaitingAnswer || usage.CompletionTokens != 6 {
		t.Fatalf("replies=%+v usage=%+v", replies, usage)
	}
	if len(a.history) != 2 || strings.Contains(a.history[1].Content, "private") {
		t.Fatalf("history=%+v", a.history)
	}
}
func TestStreamFailuresAreNeverSuccessful(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"truncated", "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n", zamu.ErrUncertain},
		{"missing_finish", "data: [DONE]\n\n", zamu.ErrContract},
		{"provider_error", "data: {\"error\":{\"message\":\"failed\"}}\n\n", zamu.ErrUncertain},
		{"bad_json", "data: nope\n\n", zamu.ErrContract},
		{"token_limit", "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n", zamu.ErrUncertain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			a, err := New(Config{Endpoint: server.URL, Token: "secret", Model: "m"})
			if err != nil {
				t.Fatal(err)
			}
			err = a.Respond(context.Background(), zamu.Turn{ID: "1", Text: "Hello"}, func(zamu.ReplyEvent) error { return nil })
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want=%v", err, tc.want)
			}
		})
	}
}
func TestCancellationInterruptsStreamingRequest(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	a, err := New(Config{Endpoint: server.URL, Token: "secret", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- a.Respond(ctx, zamu.Turn{ID: "1", Text: "Hi"}, func(zamu.ReplyEvent) error { return nil })
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation stalled")
	}
}
