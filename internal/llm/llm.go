// Package llm is a minimal streaming client for OpenAI-compatible
// /chat/completions endpoints (OpenAI, OpenRouter, Groq, Ollama, vLLM, LiteLLM, ...).
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Images     []string   `json:"-"` // data: URLs, user messages only
}

// MarshalJSON emits OpenAI "content parts" when the message carries images:
// [{"type":"text",...}, {"type":"image_url","image_url":{"url":"data:..."}}].
func (m Message) MarshalJSON() ([]byte, error) {
	type plain Message // no methods: avoids recursion
	if len(m.Images) == 0 {
		return json.Marshal(plain(m))
	}
	parts := make([]any, 0, len(m.Images)+1)
	if m.Content != "" {
		parts = append(parts, map[string]any{"type": "text", "text": m.Content})
	}
	for _, url := range m.Images {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}})
	}
	return json.Marshal(struct {
		Role    string `json:"role"`
		Content []any  `json:"content"`
	}{m.Role, parts})
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON text
}

type Tool struct {
	Type     string      `json:"type"` // "function"
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type Request struct {
	BaseURL  string
	APIKey   string
	Model    string
	Messages []Message
	Tools    []Tool
	// OnReasoning receives "thinking" deltas (reasoning_content / reasoning)
	// from providers that stream them. Optional.
	OnReasoning func(string)
	// IdleTimeout ends the call when the provider sends nothing for this long,
	// before the answer starts or in the middle of it. 0 = DefaultIdleTimeout,
	// negative = never.
	IdleTimeout time.Duration
}

// DefaultIdleTimeout is how long a provider may stay silent before a call is
// given up. Models that think before they answer can be quiet for minutes.
const DefaultIdleTimeout = 5 * time.Minute

// ErrStalled is returned when the provider stopped sending data.
var ErrStalled = errors.New("the provider stopped answering (no data for too long)")

// idleBody cancels the request when no bytes arrive for the idle time.
type idleBody struct {
	io.ReadCloser
	timer *time.Timer
	idle  time.Duration
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.timer.Reset(b.idle)
	}
	return n, err
}

// Result is the fully assembled response of one streamed completion.
type Result struct {
	Content      string
	Reasoning    string
	ToolCalls    []ToolCall
	Usage        *Usage // nil if the provider did not report usage
	FinishReason string
}

// HTTPError is a non-2xx response; Retryable for 429 and 5xx.
type HTTPError struct {
	Status     int
	Body       string
	RetryAfter time.Duration // from the Retry-After header, 0 if absent
}

func (e *HTTPError) Error() string   { return fmt.Sprintf("provider returned %d: %s", e.Status, e.Body) }
func (e *HTTPError) Retryable() bool { return e.Status == 429 || e.Status >= 500 }

// parseRetryAfter understands both "120" (seconds) and an HTTP date.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// RetryDelay returns how long to wait before attempt n (0-based): the
// provider's Retry-After if given, else exponential back-off, capped.
func RetryDelay(err error, attempt int) time.Duration {
	wait := time.Duration(1<<attempt) * time.Second
	var he *HTTPError
	if errors.As(err, &he) && he.RetryAfter > wait {
		wait = he.RetryAfter
	}
	return min(wait, 60*time.Second)
}

var client = &http.Client{Timeout: 0} // streams can be long; cancellation via context

// Stream performs one streamed completion, calling onDelta for each content chunk.
func Stream(ctx context.Context, req Request, onDelta func(string)) (*Result, error) {
	body := map[string]any{
		"model":          req.Model,
		"messages":       req.Messages,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	idle := req.IdleTimeout
	if idle == 0 {
		idle = DefaultIdleTimeout
	}
	// The watchdog covers the wait for the first byte as well as every gap
	// after it, so a provider that hangs can never hold a run for ever.
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	var watchdog *time.Timer
	if idle > 0 {
		watchdog = time.AfterFunc(idle, func() { stop(ErrStalled) })
		defer watchdog.Stop()
	}
	stalled := func(err error) error {
		if err != nil && context.Cause(ctx) == ErrStalled {
			return ErrStalled
		}
		return err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "text/event-stream")
	if req.APIKey != "" {
		hreq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
	resp, err := client.Do(hreq)
	if err != nil {
		return nil, stalled(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nil, &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(b)),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	var stream io.Reader = resp.Body
	if watchdog != nil {
		watchdog.Reset(idle)
		stream = &idleBody{ReadCloser: resp.Body, timer: watchdog, idle: idle}
	}
	res, err := parseStream(stream, onDelta, req.OnReasoning)
	return res, stalled(err)
}

type chunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"` // DeepSeek, vLLM, SGLang
			Reasoning        string `json:"reasoning"`         // OpenRouter, some proxies
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func parseStream(r io.Reader, onDelta func(string), onReasoning func(string)) (*Result, error) {
	res := &Result{}
	var content, reasoning strings.Builder
	calls := map[int]*ToolCall{}
	maxIdx := -1

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var c chunk
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			continue // ignore keep-alives / unknown frames
		}
		if c.Error != nil {
			return nil, errors.New("provider error: " + c.Error.Message)
		}
		if c.Usage != nil {
			res.Usage = c.Usage
		}
		for _, ch := range c.Choices {
			if think := ch.Delta.ReasoningContent + ch.Delta.Reasoning; think != "" {
				reasoning.WriteString(think)
				if onReasoning != nil {
					onReasoning(think)
				}
			}
			if ch.Delta.Content != "" {
				content.WriteString(ch.Delta.Content)
				onDelta(ch.Delta.Content)
			}
			for _, tc := range ch.Delta.ToolCalls {
				call, ok := calls[tc.Index]
				if !ok {
					call = &ToolCall{Type: "function"}
					calls[tc.Index] = call
					if tc.Index > maxIdx {
						maxIdx = tc.Index
					}
				}
				if tc.ID != "" {
					call.ID = tc.ID
				}
				if tc.Function.Name != "" {
					call.Function.Name += tc.Function.Name
				}
				call.Function.Arguments += tc.Function.Arguments
			}
			if ch.FinishReason != nil {
				res.FinishReason = *ch.FinishReason
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	res.Content = content.String()
	res.Reasoning = reasoning.String()
	for i := 0; i <= maxIdx; i++ {
		if c, ok := calls[i]; ok && c.Function.Name != "" {
			if c.ID == "" {
				c.ID = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i)
			}
			if strings.TrimSpace(c.Function.Arguments) == "" {
				c.Function.Arguments = "{}"
			}
			res.ToolCalls = append(res.ToolCalls, *c)
		}
	}
	return res, nil
}

// Complete is a non-streaming convenience wrapper (used for compaction).
func Complete(ctx context.Context, req Request) (*Result, error) {
	return Stream(ctx, req, func(string) {})
}

// IsRetryable reports whether an error from Stream is worth retrying.
func IsRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, ErrStalled) {
		return false // a provider that went silent is not asked again; the user can retry
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Retryable()
	}
	return true // network errors
}

// EstimateTokens is a rough chars/4 estimate used when usage is missing.
func EstimateTokens(s string) int { return (len(s) + 3) / 4 }
