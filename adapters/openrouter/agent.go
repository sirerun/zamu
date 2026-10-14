// Package openrouter supplies a minimal tool-free host for the standalone demo.
// Zati integrations use Zati's governed agent instead of this host.
package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/sirerun/zamu"
)

type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}
type Config struct {
	Endpoint        string
	Token           string
	Model           string
	MaxTokens       int
	MaxHistoryBytes int
	HTTPClient      *http.Client
	OnUsage         func(Usage)
}
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Agent struct {
	cfg     Config
	http    *http.Client
	mu      sync.Mutex
	history []chatMessage
}

func New(cfg Config) (*Agent, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://openrouter.ai/api/v1/chat/completions"
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, zamu.ErrContract
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("reasoning requires HTTPS: %w", zamu.ErrContract)
		}
	}
	if strings.TrimSpace(cfg.Token) == "" || strings.ContainsAny(cfg.Token, "\r\n") || strings.TrimSpace(cfg.Model) == "" {
		return nil, zamu.ErrUnauthorized
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 512
	}
	if cfg.MaxTokens < 1 || cfg.MaxTokens > 4096 {
		return nil, zamu.ErrContract
	}
	if cfg.MaxHistoryBytes == 0 {
		cfg.MaxHistoryBytes = 32 << 10
	}
	if cfg.MaxHistoryBytes < 1024 || cfg.MaxHistoryBytes > 1<<20 {
		return nil, zamu.ErrContract
	}
	c := cfg.HTTPClient
	if c == nil {
		c = &http.Client{Timeout: 45 * time.Second}
	}
	copy := *c
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Agent{cfg: cfg, http: &copy}, nil
}
func (a *Agent) Respond(ctx context.Context, t zamu.Turn, emit func(zamu.ReplyEvent) error) (result error) {
	if emit == nil || t.ID == "" || t.Text == "" || !utf8.ValidString(t.Text) || len(t.Text) > 16384 {
		return zamu.ErrContract
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	a.history = append(a.history, chatMessage{Role: "user", Content: t.Text})
	size := func() int {
		n := 0
		for _, m := range a.history {
			n += len(m.Content)
		}
		return n
	}
	for len(a.history) > 1 && size() > a.cfg.MaxHistoryBytes {
		a.history = a.history[1:]
	}
	if size() > a.cfg.MaxHistoryBytes {
		return zamu.ErrOverload
	}
	system := "You are a helpful English voice conversation demo. Give brief natural spoken answers. Use plain speech, no Markdown or raw URLs. You cannot execute tools or external actions. Do not claim actions were performed. Respect pauses and corrections."
	if t.PriorPlayback.Interrupted || t.PriorPlayback.Uncertain {
		system += fmt.Sprintf(" The previous response was interrupted or has uncertain delivery at playback sample %d; do not assume its complete generated text was heard.", t.PriorPlayback.Played)
	}
	messages := append([]chatMessage{{Role: "system", Content: system}}, a.history...)
	payload, err := json.Marshal(struct {
		Model     string          `json:"model"`
		Messages  []chatMessage   `json:"messages"`
		Stream    bool            `json:"stream"`
		MaxTokens int             `json:"max_tokens"`
		Provider  map[string]bool `json:"provider"`
	}{a.cfg.Model, messages, true, a.cfg.MaxTokens, map[string]bool{"allow_fallbacks": false}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("reasoning request outcome unknown: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		switch resp.StatusCode {
		case 401, 403:
			return zamu.ErrUnauthorized
		case 402, 429:
			return zamu.ErrOverload
		default:
			return fmt.Errorf("reasoning HTTP %d: %w", resp.StatusCode, zamu.ErrUncertain)
		}
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return zamu.ErrContract
	}
	var generated, pending strings.Builder
	var sequence uint64
	finished := false
	done := false
	defer func() {
		if generated.Len() > 0 {
			a.history = append(a.history, chatMessage{Role: "assistant", Content: generated.String()})
		}
	}()
	flush := func() error {
		if strings.TrimSpace(pending.String()) == "" {
			pending.Reset()
			return nil
		}
		sequence++
		text := pending.String()
		pending.Reset()
		return emit(zamu.ReplyEvent{Chunk: zamu.SpeechChunk{Sequence: sequence, Text: text}, AwaitingAnswer: strings.HasSuffix(strings.TrimSpace(text), "?")})
	}
	decode := func(data string) error {
		if data == "[DONE]" {
			if !finished {
				return zamu.ErrContract
			}
			done = true
			return flush()
		}
		var part struct {
			Error   json.RawMessage `json:"error"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &part); err != nil {
			return zamu.ErrContract
		}
		if len(part.Error) > 0 && string(part.Error) != "null" {
			return zamu.ErrUncertain
		}
		if part.Usage != nil && a.cfg.OnUsage != nil {
			a.cfg.OnUsage(*part.Usage)
		}
		if len(part.Choices) > 1 {
			return zamu.ErrContract
		}
		for _, choice := range part.Choices {
			if finished && choice.Delta.Content != "" {
				return zamu.ErrContract
			}
			if generated.Len()+len(choice.Delta.Content) > 16384 {
				return zamu.ErrOverload
			}
			generated.WriteString(choice.Delta.Content)
			for _, r := range choice.Delta.Content {
				pending.WriteRune(r)
				if strings.ContainsRune(".!?\n", r) || pending.Len() >= 240 && r == ' ' {
					if err := flush(); err != nil {
						return err
					}
				}
			}
			if choice.FinishReason != nil {
				if *choice.FinishReason != "stop" {
					return fmt.Errorf("incomplete reasoning response: %w", zamu.ErrUncertain)
				}
				finished = true
			}
		}
		return nil
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var data []string
	dataBytes := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(data) > 0 {
				if err := decode(strings.Join(data, "\n")); err != nil {
					return err
				}
				data = nil
				dataBytes = 0
				if done {
					return nil
				}
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			part := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			dataBytes += len(part)
			if dataBytes > 1<<20 {
				return zamu.ErrOverload
			}
			data = append(data, part)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("reasoning stream ended without DONE: %w", zamu.ErrUncertain)
}
