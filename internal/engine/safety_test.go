package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"knitnox/internal/store"
)

// A provider that hangs up in the middle of an answer: the run fails and
// keeps the text so far, instead of passing the half answer off as whole.
func TestCutOffAnswerIsNotSavedAsComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"The answer is\"}}]}\n\n")
		// no finish_reason, no [DONE]: the connection just ends
	}))
	t.Cleanup(srv.Close)
	f := setup(t, srv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	runID, _ := f.eng.Submit(tid, f.owner, "what is it?", nil)
	r := waitRun(t, f.st, runID)
	if r.Status != store.RunFailed || !strings.Contains(r.Error, "cut off") {
		t.Fatalf("run = %+v", r)
	}
	msgs, _ := f.st.RunMessages(runID)
	last := msgs[len(msgs)-1]
	if last.Content != "The answer is" || last.Status != store.MsgFailed {
		t.Fatalf("message = %q (%s)", last.Content, last.Status)
	}
}

// brokenToolMCP has one tool whose call fails with the given HTTP status
// after "doing its work"; it counts how often the tool was called.
func brokenToolMCP(t *testing.T, status int) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
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
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "broken"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "send_email", "description": "send", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			calls.Add(1) // the e-mail is sent ...
			http.Error(w, "gateway trouble", status)
			return // ... and then the answer gets lost
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// A tool call whose answer got lost may have run, so it is made once and the
// model is told. A call the server turned away as busy did not run, so it is
// repeated.
func TestToolCallIsNotRepeatedWhenItMayHaveRun(t *testing.T) {
	for _, c := range []struct {
		status    int
		wantCalls int32
		wantText  string
	}{
		{502, 1, "may or may not have run"},
		{503, 3, "did not run"}, // 1 try + 2 retries (the default)
	} {
		llmSrv, _ := fakeLLM(t, 0)
		mcpSrv, calls := brokenToolMCP(t, c.status)
		f := setup(t, llmSrv.URL)
		if err := f.st.CreateMCPServer(f.owner, "mail", mcpSrv.URL, "", store.ApprovalAuto); err != nil {
			t.Fatal(err)
		}
		tid, _ := f.st.CreateThread(f.owner, f.modelID)
		runID, _ := f.eng.Submit(tid, f.owner, "send it", nil)
		if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
			t.Fatalf("%d: run = %+v", c.status, r)
		}
		if got := calls.Load(); got != c.wantCalls {
			t.Fatalf("%d: the tool was called %d times, want %d", c.status, got, c.wantCalls)
		}
		toolMsg := ""
		msgs, _ := f.st.RunMessages(runID)
		for _, m := range msgs {
			if m.Role == "tool" {
				toolMsg = m.Content
			}
		}
		if !strings.Contains(toolMsg, c.wantText) {
			t.Fatalf("%d: the model was told %q", c.status, toolMsg)
		}
	}
}
