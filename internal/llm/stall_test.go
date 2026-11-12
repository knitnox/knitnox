package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A provider that goes silent must end the call instead of holding it for ever:
// before the first byte, and in the middle of an answer.
func TestSilentProviderIsGivenUp(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("mode") == "mid" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
			w.(http.Flusher).Flush()
		}
		select { // then: nothing
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	for _, mode := range []string{"start", "mid"} {
		req := Request{BaseURL: srv.URL + "?mode=" + mode + "&x=", Model: "m", Messages: []Message{{Role: "user", Content: "x"}},
			IdleTimeout: 150 * time.Millisecond}
		got := ""
		start := time.Now()
		_, err := Stream(context.Background(), req, func(d string) { got += d })
		if !errors.Is(err, ErrStalled) {
			t.Fatalf("%s: err = %v, want ErrStalled", mode, err)
		}
		if time.Since(start) > 3*time.Second {
			t.Fatalf("%s: took %v", mode, time.Since(start))
		}
		if mode == "mid" && got != "hi" {
			t.Fatalf("mid: the text before the silence was lost: %q", got)
		}
		if IsRetryable(err) {
			t.Fatalf("%s: a stalled call must not be retried automatically", mode)
		}
	}
}

// A slow but steady stream is not a stall: every chunk resets the clock.
func TestSlowStreamIsNotAStall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 6; i++ {
			time.Sleep(60 * time.Millisecond)
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n")
			w.(http.Flusher).Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	req := Request{BaseURL: srv.URL, Model: "m", Messages: []Message{{Role: "user", Content: "x"}}, IdleTimeout: 200 * time.Millisecond}
	res, err := Stream(context.Background(), req, func(string) {})
	if err != nil || res.Content != "aaaaaa" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}

// Stopping the run is still reported as a cancel, not as a stall.
func TestCancelIsNotAStall(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err := Stream(ctx, Request{BaseURL: srv.URL, Model: "m", IdleTimeout: 5 * time.Second}, func(string) {})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrStalled) {
		t.Fatalf("err = %v", err)
	}
}
