package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryAfterIsHonoured(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", "7")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	req := Request{BaseURL: srv.URL, Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}
	_, err := Stream(context.Background(), req, func(string) {})
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 429 || he.RetryAfter != 7*time.Second {
		t.Fatalf("err = %v", err)
	}
	if d := RetryDelay(err, 0); d != 7*time.Second {
		t.Fatalf("RetryDelay = %v, want 7s", d)
	}
	if d := RetryDelay(errors.New("net"), 2); d != 4*time.Second {
		t.Fatalf("backoff = %v, want 4s", d)
	}
	if d := RetryDelay(&HTTPError{Status: 429, RetryAfter: time.Hour}, 0); d != 60*time.Second {
		t.Fatalf("cap = %v, want 60s", d)
	}
	res, err := Stream(context.Background(), req, func(string) {})
	if err != nil || res.Content != "hi" {
		t.Fatalf("second call: %v %+v", err, res)
	}
}

func TestParseRetryAfterDate(t *testing.T) {
	when := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	if d := parseRetryAfter(when); d < 25*time.Second || d > 31*time.Second {
		t.Fatalf("date form = %v", d)
	}
	if parseRetryAfter("garbage") != 0 || parseRetryAfter("") != 0 {
		t.Fatal("garbage should be 0")
	}
}
