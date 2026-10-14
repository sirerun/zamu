package mistt_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/sirerun/MISTT/conversation"
	misttv1 "github.com/sirerun/MISTT/gen/mistt/v1"
	"github.com/sirerun/zamu"
	adapter "github.com/sirerun/zamu/adapters/mistt"
	"google.golang.org/grpc"
)

type scriptedServer struct {
	misttv1.UnimplementedMisttServer
	script func(string) []*misttv1.StreamResponse
}

func (s scriptedServer) Conversation(stream misttv1.Mistt_ConversationServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	id := first.GetStart().GetSessionId()
	for _, e := range s.script(id) {
		if err := stream.Send(e); err != nil {
			return err
		}
	}
	return nil
}
func started(id string) *misttv1.StreamResponse {
	return &misttv1.StreamResponse{SessionId: id, Sequence: 1, Payload: &misttv1.StreamResponse_SessionStarted{SessionStarted: &misttv1.SessionStarted{CaptureMode: misttv1.CaptureMode_CAPTURE_MODE_MICROPHONE, SampleRateHz: 16000, Channels: 1, Encoding: "s16le"}}}
}
func TestRejectsBrokenRecognitionContracts(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(string) []*misttv1.StreamResponse
		want   error
	}{
		{"wrong_session", func(id string) []*misttv1.StreamResponse { return []*misttv1.StreamResponse{started("other")} }, zamu.ErrContract},
		{"wrong_sequence", func(id string) []*misttv1.StreamResponse {
			e := started(id)
			e.Sequence = 2
			return []*misttv1.StreamResponse{e}
		}, zamu.ErrContract},
		{"bad_format", func(id string) []*misttv1.StreamResponse {
			e := started(id)
			e.GetSessionStarted().Channels = 2
			return []*misttv1.StreamResponse{e}
		}, zamu.ErrContract},
		{"unexpected_eof", func(id string) []*misttv1.StreamResponse { return []*misttv1.StreamResponse{started(id)} }, zamu.ErrContract},
		{"bad_activity_source", func(id string) []*misttv1.StreamResponse {
			return []*misttv1.StreamResponse{started(id), {SessionId: id, Sequence: 2, Payload: &misttv1.StreamResponse_SpeechStarted{SpeechStarted: &misttv1.SpeechActivity{Source: "system"}}}}
		}, zamu.ErrUnauthorized},
		{"fractional_sample", func(id string) []*misttv1.StreamResponse {
			return []*misttv1.StreamResponse{started(id), {SessionId: id, Sequence: 2, Payload: &misttv1.StreamResponse_SpeechStopped{SpeechStopped: &misttv1.SpeechActivity{Source: "microphone", CaptureOffsetNs: 1}}}}
		}, zamu.ErrContract},
		{"bad_transcript_source", func(id string) []*misttv1.StreamResponse {
			return []*misttv1.StreamResponse{started(id), {SessionId: id, Sequence: 2, Payload: &misttv1.StreamResponse_Transcription{Transcription: &misttv1.TranscriptionEvent{Source: "system"}}}}
		}, zamu.ErrUnauthorized},
		{"inconsistent_transcript_clock", func(id string) []*misttv1.StreamResponse {
			return []*misttv1.StreamResponse{started(id), {SessionId: id, Sequence: 2, Payload: &misttv1.StreamResponse_Transcription{Transcription: &misttv1.TranscriptionEvent{Source: "microphone", EndSample: 1}}}}
		}, zamu.ErrContract},
		{"inconsistent_coverage_clock", func(id string) []*misttv1.StreamResponse {
			return []*misttv1.StreamResponse{started(id), {SessionId: id, Sequence: 2, Payload: &misttv1.StreamResponse_RecognitionProgress{RecognitionProgress: &misttv1.RecognitionProgress{Source: "microphone", FinalizedThroughSample: 1}}}}
		}, zamu.ErrContract},
		{"bad_gap_source", func(id string) []*misttv1.StreamResponse {
			return []*misttv1.StreamResponse{started(id), {SessionId: id, Sequence: 2, Payload: &misttv1.StreamResponse_SessionGap{SessionGap: &misttv1.SessionGap{Source: "system"}}}}
		}, zamu.ErrUnauthorized},
		{"terminal_failure", func(id string) []*misttv1.StreamResponse {
			return []*misttv1.StreamResponse{started(id), {SessionId: id, Sequence: 2, Payload: &misttv1.StreamResponse_SessionStopped{SessionStopped: &misttv1.SessionStopped{GrpcCode: 14}}}}
		}, zamu.ErrUncertain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			socket, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			server := grpc.NewServer()
			misttv1.RegisterMisttServer(server, scriptedServer{script: tc.mutate})
			go server.Serve(socket)
			defer server.Stop()
			client, err := conversation.NewClient(conversation.ClientConfig{Address: socket.Addr().String(), Token: "01234567890123456789012345678901"})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			l, err := adapter.New(adapter.Config{Client: client, SessionID: "host", Binding: "binding"})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = l.Listen(ctx, func(zamu.InputEvent) error { return nil })
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want=%v", err, tc.want)
			}
			if err := l.Feed(ctx, zamu.InputEvent{Kind: zamu.Muted}); !errors.Is(err, zamu.ErrClosed) {
				t.Fatalf("closed listener accepted control: %v", err)
			}
			_ = l.Close()
		})
	}
}
