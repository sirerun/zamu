package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirerun/zamu"
)

type fakeStream struct{ stopped chan struct{} }

func (f *fakeStream) Listen(ctx context.Context, emit func(zamu.InputEvent) error) error {
	defer close(f.stopped)
	if err := emit(zamu.InputEvent{Kind: zamu.Activated}); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}
func (*fakeStream) SendPCM(context.Context, uint64, uint64, []byte) error { return nil }
func (*fakeStream) Activity(context.Context, bool, uint64) error          { return nil }
func (*fakeStream) Checkpoint(context.Context, uint64) error              { return nil }
func (*fakeStream) Feed(context.Context, zamu.InputEvent) error           { return nil }
func (*fakeStream) Close() error                                          { return nil }

func TestCaptureResetJoinsAndPreservesAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var count atomic.Int32
	var previous *fakeStream
	c := newCaptureFactory(func() (captureStream, error) {
		if previous != nil {
			select {
			case <-previous.stopped:
			default:
				t.Error("old listener still running")
			}
		}
		previous = &fakeStream{stopped: make(chan struct{})}
		count.Add(1)
		return previous, nil
	})
	events := make(chan zamu.InputKind, 4)
	done := make(chan error, 1)
	go func() { done <- c.Listen(ctx, func(e zamu.InputEvent) error { events <- e.Kind; return nil }) }()
	if _, err := c.stream(ctx); err != nil {
		t.Fatal(err)
	}
	if got := <-events; got != zamu.Activated {
		t.Fatal(got)
	}
	if err := c.reset(ctx, true); err != nil {
		t.Fatal(err)
	}
	if got := <-events; got != zamu.CaptureReset {
		t.Fatal(got)
	}
	if err := c.reset(ctx, false); err != nil {
		t.Fatal(err)
	}
	if got := <-events; got != zamu.Activated {
		t.Fatal(got)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if count.Load() != 3 {
		t.Fatal(count.Load())
	}
}
func TestCaptureUnavailableFailsClosed(t *testing.T) {
	expected := errors.New("offline")
	c := newCaptureFactory(func() (captureStream, error) { return nil, expected })
	if err := c.Listen(context.Background(), func(zamu.InputEvent) error { return nil }); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	if _, err := c.stream(context.Background()); !errors.Is(err, zamu.ErrClosed) {
		t.Fatal(err)
	}
	if err := c.reset(context.Background(), true); !errors.Is(err, zamu.ErrClosed) {
		t.Fatal(err)
	}
}
func TestExplicitPrivateToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(p, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := readToken(p); err != nil || got != "secret" {
		t.Fatal(got, err)
	}
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readToken(p); err == nil {
		t.Fatal("accepted public token")
	}
}
func TestSpeechTransportPolicy(t *testing.T) {
	if cfg, err := speechTLS("127.0.0.1:8080", ""); err != nil || cfg != nil {
		t.Fatal(cfg, err)
	}
	if cfg, err := speechTLS("spark.example:8080", ""); err != nil || cfg == nil || cfg.ServerName != "spark.example" {
		t.Fatal(cfg, err)
	}
	for _, origin := range []string{"http://spark.example:8080", "file:///token", ""} {
		if validTTS(origin) {
			t.Fatal(origin)
		}
	}
	for _, origin := range []string{"https://spark.example:8080", "http://127.0.0.1:8080"} {
		if !validTTS(origin) {
			t.Fatal(origin)
		}
	}
}

func TestNativeSessionUUID(t *testing.T) {
	first, err := newSessionID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newSessionID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(first) || first == second {
		t.Fatal("invalid native UUID")
	}
}

func TestCaptureResetFactoryFailureJoins(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	expected := errors.New("recognition unavailable")
	calls := 0
	first := &fakeStream{stopped: make(chan struct{})}
	c := newCaptureFactory(func() (captureStream, error) {
		calls++
		if calls > 1 {
			return nil, expected
		}
		return first, nil
	})
	done := make(chan error, 1)
	go func() { done <- c.Listen(ctx, func(zamu.InputEvent) error { return nil }) }()
	if _, err := c.stream(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.reset(ctx, true); !errors.Is(err, expected) && !errors.Is(err, zamu.ErrClosed) {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, expected) {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("reset failure stranded listener cleanup")
	}
	select {
	case <-first.stopped:
	default:
		t.Fatal("old stream still live")
	}
}
func TestCaptureCancelledBeforeActivation(t *testing.T) {
	c := newCaptureFactory(func() (captureStream, error) { t.Fatal("not listening"); return nil, nil })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.stream(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := c.reset(ctx, true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
