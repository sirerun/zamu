// conversation demonstrates the same public Zamu/MISTT/Narrate boundaries used
// by a host app. It is deliberately tool-free; no generated text executes actions.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/narrate-it/narrate/speech/local"
	"github.com/sirerun/MISTT/conversation"
	"github.com/sirerun/zamu"
	mistt "github.com/sirerun/zamu/adapters/mistt"
	narrate "github.com/sirerun/zamu/adapters/narrate"
	"github.com/sirerun/zamu/adapters/native"
	"github.com/sirerun/zamu/adapters/openrouter"
)

type options struct {
	media, stt, sttToken, sttCA, tts, ttsToken, llmToken, model string
	wake, start, showText                                       bool
}

func readToken(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("explicit token file unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 8192 {
		return "", errors.New("token file must be private, regular and at most 8 KiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(data) > 8192 {
		return "", errors.New("token file unreadable")
	}
	token := strings.TrimSpace(string(data))
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return "", errors.New("token file contains invalid credential")
	}
	return token, nil
}
func loopback(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}
func speechTLS(address, caFile string) (*tls.Config, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("MISTT requires host:port")
	}
	if caFile == "" && loopback(host) {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, errors.New("MISTT CA file unavailable")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("MISTT CA file is invalid")
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}
func validTTS(origin string) bool {
	u, e := url.Parse(origin)
	return e == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http" && loopback(u.Hostname()))
}
func newSessionID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}
func main() {
	var o options
	flag.StringVar(&o.media, "media", "mistt-media", "MISTT native companion executable")
	flag.StringVar(&o.stt, "mistt-address", "", "Authenticated MISTT host:port (TLS except loopback)")
	flag.StringVar(&o.sttToken, "mistt-token-file", "", "Private MISTT bearer token file")
	flag.StringVar(&o.sttCA, "mistt-ca-file", "", "Optional private CA PEM for MISTT TLS")
	flag.StringVar(&o.tts, "narrate-origin", "", "Narrate HTTPS origin, or explicit HTTP loopback tunnel")
	flag.StringVar(&o.ttsToken, "narrate-token-file", "", "Private Narrate bearer token file")
	flag.StringVar(&o.llmToken, "openrouter-token-file", "", "Private OpenRouter bearer token file")
	flag.StringVar(&o.model, "model", "", "Exact OpenRouter reasoning model ID; no automatic selection")
	flag.BoolVar(&o.wake, "wake", false, "Arm local Hey Zati detection before admission")
	flag.BoolVar(&o.start, "start", false, "Explicitly admit and start conversation immediately")
	flag.BoolVar(&o.showText, "show-text", false, "Print committed/generated text to this terminal")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, o, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "Conversation ended:", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, o options, input io.ReadCloser, output io.Writer) (outcome error) {
	if strings.TrimSpace(o.model) == "" || !validTTS(o.tts) {
		return errors.New("explicit model and secure local speech origin required")
	}
	sttKey, err := readToken(o.sttToken)
	if err != nil {
		return err
	}
	ttsKey, err := readToken(o.ttsToken)
	if err != nil {
		return err
	}
	llmKey, err := readToken(o.llmToken)
	if err != nil {
		return err
	}
	tlsConfig, err := speechTLS(o.stt, o.sttCA)
	if err != nil {
		return err
	}
	client, err := conversation.NewClient(conversation.ClientConfig{Address: o.stt, Token: sttKey, TLS: tlsConfig})
	if err != nil {
		return err
	}
	defer client.Close()
	speech, err := narrate.New(local.Config{Endpoint: o.tts, Token: ttsKey})
	if err != nil {
		return err
	}
	var outputMu sync.Mutex
	print := func(format string, args ...any) {
		outputMu.Lock()
		defer outputMu.Unlock()
		fmt.Fprintf(output, format, args...)
	}
	agent, err := openrouter.New(openrouter.Config{Token: llmKey, Model: o.model, OnUsage: func(u openrouter.Usage) {
		print("usage: prompt=%d completion=%d reported_cost_usd=%.6f\n", u.PromptTokens, u.CompletionTokens, u.Cost)
	}})
	if err != nil {
		return err
	}
	sessionID, err := newSessionID()
	if err != nil {
		return err
	}
	media, err := native.New(o.media, sessionID)
	if err != nil {
		return err
	}
	life, cancel := context.WithCancel(ctx)
	defer cancel()
	if err = media.Start(life); err != nil {
		return err
	}
	defer func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		outcome = errors.Join(outcome, media.Close(closeCtx))
	}()
	capture := newCapture(mistt.Config{Client: client, SessionID: sessionID, Binding: sessionID, Source: "microphone"})
	session, err := zamu.New(zamu.Config{SessionID: sessionID, Binding: sessionID, Source: "microphone", Voice: "anna"}, zamu.Components{Listener: capture, Speech: speech, Agent: agent, Player: media})
	if err != nil {
		return err
	}
	commands := make(chan string, 8)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer close(commands)
		s := bufio.NewScanner(input)
		s.Buffer(make([]byte, 256), 256)
		for s.Scan() {
			select {
			case commands <- strings.ToLower(strings.TrimSpace(s.Text())):
			case <-life.Done():
				return
			}
		}
	}()
	defer func() { cancel(); _ = input.Close(); <-readDone }()
	result := make(chan error, 1)
	eventsDone := make(chan struct{})
	notices := make(chan zamu.Event, 8)
	var sessionCancel context.CancelFunc
	active, muted, armed := false, false, false
	var sequence uint64
	defer func() {
		if sessionCancel != nil {
			sessionCancel()
			closeCtx, stop := context.WithTimeout(context.Background(), 6*time.Second)
			defer stop()
			outcome = errors.Join(outcome, session.Close(closeCtx))
			<-eventsDone
		}
	}()
	print("Speech stays on your configured local services; committed text and conversation context go to OpenRouter (%s). No tools.\n", o.model)
	print("Commands: start, mute, unmute, wake-on, wake-off, stop (exit). Ctrl-C also stops.\n")
	start := func() error {
		if active {
			return nil
		}
		if sessionCancel == nil {
			sessionCtx, stop := context.WithCancel(life)
			sessionCancel = stop
			go func() { result <- session.Run(sessionCtx) }()
			go func() {
				defer close(eventsDone)
				for e := range session.Events() {
					print("%s generation=%d heard_samples=%d\n", e.Kind, e.Generation, e.Receipt.Played)
					if e.NeedsAdmission || e.Kind == "idle_timeout" {
						select {
						case notices <- e:
						case <-life.Done():
							return
						}
					}
					if o.showText && e.Text != "" {
						print("%s\n", e.Text)
					}
				}
			}()
		} else {
			resetCtx, done := context.WithTimeout(life, 5*time.Second)
			err := capture.reset(resetCtx, false)
			done()
			if err != nil {
				return err
			}
		}
		readyCtx, done := context.WithTimeout(life, 5*time.Second)
		defer done()
		if _, err := capture.stream(readyCtx); err != nil {
			return err
		}
		if err := media.SetState(readyCtx, "start"); err != nil {
			return err
		}
		sequence = 0
		active = true
		muted = false
		armed = false
		return nil
	}
	control := func(command string) error {
		call, done := context.WithTimeout(life, 5*time.Second)
		defer done()
		switch command {
		case "start":
			return start()
		case "wake-on":
			if sessionCancel != nil {
				print("Use start to readmit this existing session, or stop and relaunch --wake.\n")
				return nil
			}
			if active {
				return nil
			}
			err := media.SetState(call, "arm")
			armed = err == nil
			return err
		case "wake-off":
			if active || sessionCancel != nil {
				return nil
			}
			armed = false
			return media.SetState(call, "stop")
		case "mute":
			if !active {
				armed = false
				return media.SetState(call, "stop")
			}
			if muted {
				return nil
			}
			if err := media.SetState(call, "mute"); err != nil {
				return err
			}
			stream, err := capture.stream(call)
			if err != nil {
				return err
			}
			if err = stream.Feed(call, zamu.InputEvent{Kind: zamu.Muted}); err != nil {
				return err
			}
			muted = true
			// The mute reply follows all old capture records on native JSONL.
			// Discard buffered pre-mute observations before allowing a fresh timeline.
			for n := len(media.Events()); n > 0; n-- {
				<-media.Events()
			}
			return nil
		case "unmute":
			if !active {
				return start()
			}
			if !muted {
				return nil
			}
			if err := capture.reset(call, true); err != nil {
				return err
			}
			if err := media.SetState(call, "unmute"); err != nil {
				return err
			}
			sequence = 0
			muted = false
			return nil
		default:
			return fmt.Errorf("unknown command %q", command)
		}
	}
	if o.start {
		if err = start(); err != nil {
			return err
		}
	} else if o.wake {
		if err = control("wake-on"); err != nil {
			return err
		}
	}
	for {
		select {
		case <-life.Done():
			return life.Err()
		case err := <-result:
			return err
		case <-notices:
			call, done := context.WithTimeout(life, 3*time.Second)
			err := media.SetState(call, "mute")
			done()
			if err != nil {
				return err
			}
			for n := len(media.Events()); n > 0; n-- {
				<-media.Events()
			}
			active = false
			muted = true
			print("Capture paused. Type start for fresh admission; prior incomplete speech is discarded.\n")
		case command, ok := <-commands:
			if !ok || command == "stop" || command == "quit" {
				return nil
			}
			if command == "" {
				continue
			}
			if err := control(command); err != nil {
				return err
			}
		case event, ok := <-media.Events():
			if !ok {
				return errors.New("native media process ended")
			}
			if event.Type == "failure" {
				return errors.New("native capture unavailable")
			}
			if event.Type == "wake" {
				if !active && armed {
					if err := start(); err != nil {
						return err
					}
				}
				continue
			}
			if !active || muted {
				continue
			}
			call, done := context.WithTimeout(life, 3*time.Second)
			stream, e := capture.stream(call)
			if e == nil {
				switch event.Type {
				case "audio":
					if event.SessionID != sessionID {
						e = zamu.ErrUnauthorized
						break
					}
					sequence++
					e = stream.SendPCM(call, sequence, event.StartSample, event.PCM)
				case "speech_started":
					e = stream.Activity(call, true, event.CaptureSample)
				case "speech_stopped":
					e = stream.Activity(call, false, event.CaptureSample)
					if e == nil {
						e = stream.Checkpoint(call, event.CaptureSample)
					}
				case "recognition_checkpoint":
					e = stream.Checkpoint(call, event.CaptureSample)
				case "urgent_stop":
					e = stream.Feed(call, zamu.InputEvent{Kind: zamu.ClearInterruption})
				case "playback":
					e = stream.Feed(call, zamu.InputEvent{Kind: zamu.PlaybackProgress, Receipt: event.Receipt.Core()})
				}
			}
			done()
			if e != nil {
				return e
			}
		}
	}
}
