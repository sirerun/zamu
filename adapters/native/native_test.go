package native

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sirerun/zamu"
)

func TestMain(m *testing.M) {
	if os.Getenv("ZAMU_NATIVE_TEST_HELPER") == "1" {
		scan := bufio.NewScanner(os.Stdin)
		for scan.Scan() {
			var c command
			if json.Unmarshal(scan.Bytes(), &c) != nil {
				os.Exit(2)
			}
			if c.Command == "arm" && os.Getenv("ZAMU_NATIVE_BAD_EVENT") == "1" {
				fmt.Println("invalid-json")
				continue
			}
			e := Event{Type: "reply", ID: c.ID, OK: true, Receipt: Receipt{Generation: c.Generation, Token: c.Token, SampleRate: 24000}}
			if err := json.NewEncoder(os.Stdout).Encode(e); err != nil {
				os.Exit(3)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func helper(t *testing.T) *Player {
	t.Helper()
	t.Setenv("ZAMU_NATIVE_TEST_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(executable, "session")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := p.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeCtx, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		_ = p.Close(closeCtx)
		cancel()
	})
	return p
}
func TestNativeRoundTripAndGenerationFence(t *testing.T) {
	p := helper(t)
	ctx := context.Background()
	if err := p.SetState(ctx, "start"); err != nil {
		t.Fatal(err)
	}
	f := zamu.AudioFrame{Generation: 1, Sequence: 1, Samples: 2, PCM: []byte{0, 0, 1, 0}, Format: zamu.Format{Encoding: "pcm_s16le", Channels: 1, SampleRate: 24000}}
	if err := p.Write(ctx, f); err != nil {
		t.Fatal(err)
	}
	if err := p.Write(ctx, f); !errors.Is(err, zamu.ErrContract) {
		t.Fatalf("duplicate accepted: %v", err)
	}
	r, err := p.Control(ctx, zamu.PlaybackCommand{Kind: "pause", Generation: 1, Token: 42})
	if err != nil || r.Token != 42 || r.Generation != 1 {
		t.Fatalf("receipt=%+v err=%v", r, err)
	}
	if _, err := p.Control(ctx, zamu.PlaybackCommand{Kind: "resume", Generation: 1, Token: 42}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Control(ctx, zamu.PlaybackCommand{Kind: "flush", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	f.Sequence = 2
	f.Offset = 2
	if err := p.Write(ctx, f); !errors.Is(err, zamu.ErrContract) {
		t.Fatalf("stale generation accepted: %v", err)
	}
	f.Generation = 2
	f.Sequence = 1
	f.Offset = 0
	if err := p.Write(ctx, f); err != nil {
		t.Fatal(err)
	}
	if err := p.SetState(ctx, "mute"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetState(ctx, "unmute"); err != nil {
		t.Fatal(err)
	}
	f.Sequence = 2
	f.Offset = 2
	f.Samples = 0
	f.PCM = nil
	f.PhraseEnd = true
	if err := p.Write(ctx, f); err != nil {
		t.Fatal(err)
	}
}
func TestMalformedNativeOutputTerminates(t *testing.T) {
	t.Setenv("ZAMU_NATIVE_BAD_EVENT", "1")
	p := helper(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.SetState(ctx, "arm"); err == nil {
		t.Fatal("accepted malformed output")
	}
	if err := p.Close(ctx); !errors.Is(err, zamu.ErrContract) {
		t.Fatalf("terminal=%v", err)
	}
}
func TestNativeStartFailureClosesEvents(t *testing.T) {
	p, err := New("/usr/bin/true", "session")
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Start(ctx); err == nil {
		t.Fatal("started canceled process")
	}
	select {
	case _, ok := <-p.Events():
		if ok {
			t.Fatal("unexpected event")
		}
	case <-time.After(time.Second):
		t.Fatal("failed startup left events open")
	}
}
