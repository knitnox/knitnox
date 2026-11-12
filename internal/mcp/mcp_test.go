package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// server answers initialize and lets the test decide what tools/call does.
func server(t *testing.T, call http.HandlerFunc) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		json.Unmarshal(body, &req)
		switch {
		case req.ID == nil:
			w.WriteHeader(http.StatusAccepted)
		case req.Method == "tools/call":
			call(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Only a call that cannot have reached the tool may be repeated.
func TestNotSent(t *testing.T) {
	ctx := context.Background()

	// Nothing listens here: the connection is refused, so nothing ran.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := "http://" + l.Addr().String()
	l.Close()
	if _, err := Connect(ctx, dead, ""); err == nil || !NotSent(err) {
		t.Fatalf("refused connection: err = %v, NotSent = %v", err, NotSent(err))
	}

	status := func(code int) error {
		srv := server(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", code) })
		c, err := Connect(ctx, srv.URL, "")
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = c.CallTool(ctx, "x", nil)
		return err
	}
	if err := status(503); !NotSent(err) {
		t.Fatalf("503 (busy, try later) should be safe to repeat: %v", err)
	}
	if err := status(429); !NotSent(err) {
		t.Fatalf("429 should be safe to repeat: %v", err)
	}
	if err := status(500); err == nil || NotSent(err) {
		t.Fatalf("500 may have run the tool; it must not be repeated: %v", err)
	}

	// The tool takes longer than we wait: it may well have run.
	release := make(chan struct{})
	slow := server(t, func(w http.ResponseWriter, r *http.Request) { <-release })
	defer close(release)
	c, err := Connect(ctx, slow.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if _, _, err := c.CallTool(short, "x", nil); err == nil || NotSent(err) {
		t.Fatalf("a timeout must not count as not sent: %v", err)
	}
}
