package mistt_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sirerun/MISTT/conversation"
	misttv1 "github.com/sirerun/MISTT/gen/mistt/v1"
	"github.com/sirerun/zamu"
	adapter "github.com/sirerun/zamu/adapters/mistt"
	"google.golang.org/grpc"
)

type recognizer struct{}

func (recognizer) Recognize(context.Context, []byte, string) (string, error) {
	return "Do not delete it", nil
}
func TestRealConversationWireAndControlOrdering(t *testing.T) {
	const token = "01234567890123456789012345678901"
	server, err := conversation.NewServer(conversation.Config{Token: token, Model: "fixture"}, recognizer{})
	if err != nil {
		t.Fatal(err)
	}
	socket, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	misttv1.RegisterMisttServer(grpcServer, server)
	go func() {
		if err := grpcServer.Serve(socket); err != nil {
			t.Log(err)
		}
	}()
	defer grpcServer.Stop()
	client, err := conversation.NewClient(conversation.ClientConfig{Address: socket.Addr().String(), Token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	l, err := adapter.New(adapter.Config{Client: client, SessionID: "host-session", Binding: "selected-conversation"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	events := make(chan zamu.InputEvent, 16)
	done := make(chan error, 1)
	go func() { done <- l.Listen(ctx, func(e zamu.InputEvent) error { events <- e; return nil }) }()
	next := func() zamu.InputEvent {
		select {
		case e := <-events:
			return e
		case <-ctx.Done():
			t.Fatal("no event")
			return zamu.InputEvent{}
		}
	}
	first := next()
	if first.Kind != zamu.Activated || first.SessionID != "host-session" || first.Binding != "selected-conversation" || first.StreamID == "" {
		t.Fatal(first)
	}
	if err = l.Activity(ctx, true, 0); err != nil {
		t.Fatal(err)
	}
	if err = l.SendPCM(ctx, 1, 0, make([]byte, 320)); err != nil {
		t.Fatal(err)
	}
	if err = l.Activity(ctx, false, 160); err != nil {
		t.Fatal(err)
	}
	if err = l.Checkpoint(ctx, 160); err != nil {
		t.Fatal(err)
	}
	last := first.Sequence
	foundFinal := false
	for {
		e := next()
		if e.Sequence != last+1 {
			t.Fatal("unordered adapter events")
		}
		last = e.Sequence
		if e.Kind == zamu.TranscriptFinal {
			foundFinal = e.Text == "Do not delete it"
		}
		if e.Kind == zamu.RecognitionProgress {
			if !foundFinal || e.End != 160 {
				t.Fatal(e)
			}
			break
		}
	}
	if err = l.Feed(ctx, zamu.InputEvent{Kind: zamu.Muted}); err != nil {
		t.Fatal(err)
	}
	if e := next(); e.Kind != zamu.Muted || e.Sequence != last+1 {
		t.Fatal(e)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reader did not stop")
	}
}
