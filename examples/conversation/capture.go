package main

import (
	"context"
	"sync"

	"github.com/sirerun/zamu"
	mistt "github.com/sirerun/zamu/adapters/mistt"
)

type captureStream interface {
	Listen(context.Context, func(zamu.InputEvent) error) error
	SendPCM(context.Context, uint64, uint64, []byte) error
	Activity(context.Context, bool, uint64) error
	Checkpoint(context.Context, uint64) error
	Feed(context.Context, zamu.InputEvent) error
	Close() error
}
type resetRequest struct {
	result   chan error
	preserve bool
}

// capture rotates recognition identity after mute while the player and host live on.
// The old listener is joined before fresh-stream activation is delivered.
type capture struct {
	mu      sync.Mutex
	current captureStream
	ready   chan struct{}
	resets  chan resetRequest
	done    chan struct{}
	factory func() (captureStream, error)
}

func newCapture(cfg mistt.Config) *capture {
	return newCaptureFactory(func() (captureStream, error) { return mistt.New(cfg) })
}
func newCaptureFactory(factory func() (captureStream, error)) *capture {
	return &capture{factory: factory, ready: make(chan struct{}), resets: make(chan resetRequest), done: make(chan struct{})}
}
func (c *capture) Listen(ctx context.Context, emit func(zamu.InputEvent) error) error {
	defer close(c.done)
	var cancel context.CancelFunc
	var ended chan error
	start := func(reset bool) error {
		stream, err := c.factory()
		if err != nil {
			return err
		}
		child, stop := context.WithCancel(ctx)
		cancel = stop
		result := make(chan error, 1)
		ended = result
		c.mu.Lock()
		c.current = stream
		ready := c.ready
		c.mu.Unlock()
		go func() {
			result <- stream.Listen(child, func(e zamu.InputEvent) error {
				if child.Err() != nil {
					return child.Err()
				}
				activation := e.Kind == zamu.Activated
				if activation && reset {
					e.Kind = zamu.CaptureReset
				}
				if err := emit(e); err != nil {
					return err
				}
				if activation {
					close(ready)
				}
				return nil
			})
		}()
		return nil
	}
	if err := start(false); err != nil {
		return err
	}
	defer func() { cancel(); <-ended }()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-ended:
			ended <- err
			return err
		case request := <-c.resets:
			cancel()
			<-ended
			c.mu.Lock()
			c.ready = make(chan struct{})
			c.mu.Unlock()
			if err := start(request.preserve); err != nil {
				request.result <- err
				ended <- err
				return err
			}
			c.mu.Lock()
			ready := c.ready
			c.mu.Unlock()
			select {
			case <-ready:
				request.result <- nil
			case err := <-ended:
				request.result <- err
				ended <- err
				return err
			case <-ctx.Done():
				request.result <- ctx.Err()
				return ctx.Err()
			}
		}
	}
}
func (c *capture) stream(ctx context.Context) (captureStream, error) {
	c.mu.Lock()
	ready := c.ready
	c.mu.Unlock()
	select {
	case <-ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, zamu.ErrClosed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return nil, zamu.ErrClosed
	}
	return c.current, nil
}
func (c *capture) reset(ctx context.Context, preserve bool) error {
	request := resetRequest{result: make(chan error, 1), preserve: preserve}
	select {
	case c.resets <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return zamu.ErrClosed
	}
	select {
	case err := <-request.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return zamu.ErrClosed
	}
}
