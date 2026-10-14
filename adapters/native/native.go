// Package native controls MISTT's macOS media companion over bounded JSONL.
// The companion alone owns capture, wake detection, AEC and hardware playback.
package native

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirerun/zamu"
)

type Receipt struct {
	Generation uint64 `json:"generation"`
	Played     uint64 `json:"rendered_samples"`
	SampleRate int    `json:"sample_rate"`
	Token      string `json:"token"`
	State      string `json:"state"`
}

func (r Receipt) Core() zamu.PlaybackReceipt {
	token, _ := strconv.ParseUint(r.Token, 10, 64)
	return zamu.PlaybackReceipt{Generation: r.Generation, Played: r.Played, SampleRate: r.SampleRate, Token: token}
}

type Event struct {
	Type          string  `json:"type"`
	ID            string  `json:"id"`
	OK            bool    `json:"ok"`
	Error         string  `json:"error"`
	SessionID     string  `json:"session_id"`
	Sequence      uint64  `json:"sequence"`
	StartSample   uint64  `json:"start_sample"`
	CaptureSample uint64  `json:"capture_sample"`
	PCM           []byte  `json:"pcm16"`
	State         string  `json:"state"`
	Detail        string  `json:"detail"`
	Receipt       Receipt `json:"receipt"`
}
type command struct {
	ID         string `json:"id"`
	Command    string `json:"command"`
	SessionID  string `json:"session_id,omitempty"`
	Generation uint64 `json:"generation,omitempty"`
	Token      string `json:"token,omitempty"`
	PCM        []byte `json:"pcm16,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

type Player struct {
	generation, sequence, offset, minGeneration uint64
	format                                      zamu.Format
	executable                                  string
	sessionID                                   string
	mu                                          sync.Mutex
	writeMu                                     sync.Mutex
	cmd                                         *exec.Cmd
	stdin                                       io.WriteCloser
	pending                                     map[string]chan Event
	events                                      chan Event
	done                                        chan struct{}
	counter                                     atomic.Uint64
	started                                     bool
	terminal                                    error
}

func New(executable, sessionID string) (*Player, error) {
	if executable == "" || sessionID == "" {
		return nil, zamu.ErrContract
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return nil, fmt.Errorf("native media executable: %w", err)
	}
	return &Player{executable: path, sessionID: sessionID, pending: map[string]chan Event{}, events: make(chan Event, 128), done: make(chan struct{})}, nil
}
func (p *Player) Events() <-chan Event { return p.events }
func (p *Player) Start(ctx context.Context) (result error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return zamu.ErrRunning
	}
	p.started = true
	defer func() {
		if result != nil {
			p.terminal = result
			close(p.events)
			close(p.done)
		}
	}()
	cmd := exec.CommandContext(ctx, p.executable)
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return err
	}
	p.cmd = cmd
	p.stdin = input
	go func() {
		scan := bufio.NewScanner(output)
		scan.Buffer(make([]byte, 4096), 256<<10)
		var terminal error
		for scan.Scan() {
			var event Event
			if err := json.Unmarshal(scan.Bytes(), &event); err != nil {
				terminal = fmt.Errorf("invalid native event: %w", zamu.ErrContract)
				break
			}
			if event.Type == "reply" {
				p.mu.Lock()
				reply := p.pending[event.ID]
				p.mu.Unlock()
				if reply == nil {
					continue
				}
				select {
				case reply <- event:
				default:
					terminal = zamu.ErrContract
				}
				if terminal != nil {
					break
				}
			} else {
				select {
				case p.events <- event:
				default:
					terminal = fmt.Errorf("native consumer stalled: %w", zamu.ErrOverload)
				}
				if terminal != nil {
					break
				}
			}
		}
		if err := scan.Err(); err != nil {
			terminal = errors.Join(terminal, err)
		}
		if terminal != nil {
			_ = cmd.Process.Kill()
		}
		err := cmd.Wait()
		terminal = errors.Join(terminal, err)
		p.mu.Lock()
		p.terminal = terminal
		p.mu.Unlock()
		close(p.events)
		close(p.done)
	}()
	return nil
}
func (p *Player) call(ctx context.Context, c command) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	c.ID = strconv.FormatUint(p.counter.Add(1), 10)
	c.SessionID = p.sessionID
	result := make(chan Event, 1)
	p.mu.Lock()
	if p.cmd == nil {
		p.mu.Unlock()
		return Event{}, zamu.ErrClosed
	}
	if len(p.pending) >= 16 {
		p.mu.Unlock()
		return Event{}, zamu.ErrOverload
	}
	p.pending[c.ID] = result
	input := p.stdin
	process := p.cmd.Process
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.pending, c.ID); p.mu.Unlock() }()
	data, err := json.Marshal(c)
	if err != nil {
		return Event{}, err
	}
	if len(data) > 128<<10 {
		return Event{}, zamu.ErrOverload
	}
	data = append(data, '\n')
	wrote := make(chan error, 1)
	go func() { p.writeMu.Lock(); defer p.writeMu.Unlock(); _, err := input.Write(data); wrote <- err }()
	select {
	case err := <-wrote:
		if err != nil {
			return Event{}, err
		}
	case <-ctx.Done():
		_ = process.Kill()
		return Event{}, ctx.Err()
	case <-p.done:
		return Event{}, zamu.ErrClosed
	}
	select {
	case event := <-result:
		if !event.OK {
			return event, fmt.Errorf("native %s failed: %s", c.Command, event.Error)
		}
		return event, nil
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case <-p.done:
		return Event{}, zamu.ErrClosed
	}
}
func (p *Player) SetState(ctx context.Context, state string) error {
	switch state {
	case "arm", "start", "mute", "unmute", "stop":
	default:
		return zamu.ErrUnsupported
	}
	_, err := p.call(ctx, command{Command: state})
	return err
}
func (p *Player) Write(ctx context.Context, f zamu.AudioFrame) error {
	if f.Format.Encoding != "pcm_s16le" || f.Format.Channels != 1 || f.Format.SampleRate <= 0 || f.Samples > 32768 || len(f.PCM) != int(f.Samples)*2 {
		return zamu.ErrUnsupported
	}
	p.mu.Lock()
	if f.Generation < p.minGeneration || f.Generation < p.generation || f.Generation == 0 {
		p.mu.Unlock()
		return zamu.ErrContract
	}
	if f.Generation > p.generation {
		p.generation = f.Generation
		p.sequence = 0
		p.offset = 0
		p.format = f.Format
	}
	if f.Sequence != p.sequence+1 || f.Offset != p.offset || f.Format != p.format {
		p.mu.Unlock()
		return zamu.ErrContract
	}
	p.sequence = f.Sequence
	p.offset += f.Samples
	p.mu.Unlock()
	if f.Samples == 0 && f.PhraseEnd {
		return nil
	}
	_, err := p.call(ctx, command{Command: "audio", Generation: f.Generation, PCM: f.PCM, SampleRate: f.Format.SampleRate})
	return err
}
func (p *Player) Control(ctx context.Context, c zamu.PlaybackCommand) (zamu.PlaybackReceipt, error) {
	switch c.Kind {
	case "pause", "resume", "flush", "drain":
	default:
		return zamu.PlaybackReceipt{}, zamu.ErrUnsupported
	}
	if c.Kind == "flush" {
		p.mu.Lock()
		if c.Generation >= p.minGeneration {
			p.minGeneration = c.Generation + 1
		}
		p.mu.Unlock()
	}
	event, err := p.call(ctx, command{Command: c.Kind, Generation: c.Generation, Token: strconv.FormatUint(c.Token, 10)})
	if err != nil {
		return zamu.PlaybackReceipt{}, err
	}
	return event.Receipt.Core(), nil
}
func (p *Player) Close(ctx context.Context) error {
	p.mu.Lock()
	input := p.stdin
	cmd := p.cmd
	p.mu.Unlock()
	if cmd == nil {
		return nil
	}
	if err := input.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		_ = cmd.Process.Kill()
	}
	select {
	case <-p.done:
		p.mu.Lock()
		err := p.terminal
		p.mu.Unlock()
		return err
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		select {
		case <-p.done:
		case <-time.After(time.Second):
		}
		return ctx.Err()
	}
}
