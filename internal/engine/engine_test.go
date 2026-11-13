package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"knitnox/internal/hub"
	"knitnox/internal/secret"
	"knitnox/internal/store"
)

// fakeLLM is an OpenAI-compatible streaming endpoint.
//   - compaction prompt         -> "SUMMARY"
//   - tools offered, no results -> calls the first tool
//   - otherwise                 -> "echo: <last user message>"
func fakeLLM(t *testing.T, delay time.Duration) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct{ Name string } `json:"function"`
			} `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		send := func(v any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
		hasToolResult := false
		lastUser := ""
		for _, m := range req.Messages {
			if m.Role == "tool" {
				hasToolResult = true
			}
			if m.Role == "user" {
				lastUser = unstamp(m.Content)
			}
		}
		if strings.HasPrefix(req.Messages[0].Content, "You maintain the running memory") {
			send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "SUMMARY"}}}})
		} else if len(req.Tools) > 0 && !hasToolResult {
			send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "id": "c1", "type": "function", "function": map[string]any{"name": req.Tools[0].Function.Name, "arguments": `{"q":`}},
			}}}}})
			send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "function": map[string]any{"arguments": `"hi"}`}},
			}}, "finish_reason": "tool_calls"}}})
		} else {
			reply := "echo: " + lastUser
			if hasToolResult {
				reply = "tool said: " + req.Messages[len(req.Messages)-1].Content
			}
			for _, part := range strings.SplitAfter(reply, " ") {
				time.Sleep(delay)
				send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": part}}}})
			}
		}
		send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// fakeMCP is a Streamable-HTTP MCP server with one "lookup" tool.
func fakeMCP(t *testing.T) *httptest.Server {
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
		if req.ID == nil { // notification
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sess-1")
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fake"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "lookup", "description": "look", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			if r.Header.Get("Mcp-Session-Id") != "sess-1" {
				http.Error(w, "no session", 400)
				return
			}
			// Respond as SSE to exercise that path.
			w.Header().Set("Content-Type", "text/event-stream")
			b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "42"}}}})
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// unstamp removes the "[Wed, 11 Nov 2026, 19:05]" line the engine puts in
// front of every user message, so fake models can echo what the user typed.
func unstamp(s string) string {
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]\n"); i > 0 {
			return s[i+2:]
		}
	}
	return s
}

type fixture struct {
	st      *store.Store
	eng     *Engine
	owner   int64
	modelID int64
}

func setup(t *testing.T, llmURL string) *fixture {
	box, _ := secret.New("test")
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	owner, _ := st.CreateUser("alice", "x", "admin")
	pid, err := st.CreateProvider(nil, "fake", llmURL, "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateModel(pid, "fake-model", 100000, 1, 2, true); err != nil {
		t.Fatal(err)
	}
	models, _ := st.ListModels(pid)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	eng := New(st, hub.New(), log)
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Shutdown(2 * time.Second) })
	return &fixture{st: st, eng: eng, owner: owner, modelID: models[0].ID}
}

// newGroup makes a group owned by the fixture's owner and lets the given
// people join it as members.
func newGroup(t *testing.T, f *fixture, members ...int64) int64 {
	t.Helper()
	gid, err := f.st.CreateGroup(f.owner, f.modelID, "Team")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		tok, err := f.st.CreateInvite(gid, f.owner, "member", time.Hour, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.st.AcceptInvite(tok, m); err != nil {
			t.Fatal(err)
		}
	}
	return gid
}

func waitRun(t *testing.T, st *store.Store, runID int64) *store.Run {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r, err := st.RunByID(runID)
		if err == nil && r.Status != store.RunPending && r.Status != store.RunRunning {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %d did not finish", runID)
	return nil
}

func lastAssistant(t *testing.T, st *store.Store, runID int64) string {
	msgs, _ := st.RunMessages(runID)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return msgs[i].Content
		}
	}
	t.Fatalf("no assistant message in run %d", runID)
	return ""
}

func TestBasicTurnAndUsage(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)

	runID, err := f.eng.Submit(tid, f.owner, "hello world", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := waitRun(t, f.st, runID)
	if r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	if got := lastAssistant(t, f.st, runID); got != "echo: hello world" {
		t.Fatalf("reply = %q", got)
	}
	used, _ := f.st.UsedTokens(f.owner, "monthly")
	if used != 15 {
		t.Fatalf("used tokens = %d, want 15", used)
	}
	th, _ := f.st.ThreadByID(tid)
	if th.Title != "hello world" {
		t.Fatalf("auto title = %q", th.Title)
	}
}

func TestQueueOrderAndLimit(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 20*time.Millisecond)
	f := setup(t, llmSrv.URL)
	st, _ := f.st.LoadSettings()
	st.QueueMax = 3
	f.st.SaveSettings(st)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)

	var ids []int64
	var full bool
	for i := 0; i < 8; i++ {
		id, err := f.eng.Submit(tid, f.owner, fmt.Sprintf("m%d", i), nil)
		if err == store.ErrQueueFull {
			full = true
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if !full {
		t.Fatal("expected queue to fill up")
	}
	for _, id := range ids {
		if r := waitRun(t, f.st, id); r.Status != store.RunDone {
			t.Fatalf("run %d: %s %s", id, r.Status, r.Error)
		}
	}
	// History must be strictly ordered: each reply answers its own message.
	msgs, _ := f.st.ListMessages(tid, 100)
	for i := 0; i+1 < len(msgs); i += 2 {
		if msgs[i].Role != "user" || msgs[i+1].Content != "echo: "+msgs[i].Content {
			t.Fatalf("out of order at %d: %q -> %q", i, msgs[i].Content, msgs[i+1].Content)
		}
	}
}

func TestToolLoop(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	mcpSrv := fakeMCP(t)
	f := setup(t, llmSrv.URL)
	if err := f.st.CreateMCPServer(f.owner, "fake", mcpSrv.URL, "Authorization: Bearer x", store.ApprovalAuto); err != nil {
		t.Fatal(err)
	}
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	runID, _ := f.eng.Submit(tid, f.owner, "look something up", nil)
	r := waitRun(t, f.st, runID)
	if r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	if r.Steps != 2 {
		t.Fatalf("steps = %d, want 2", r.Steps)
	}
	if got := lastAssistant(t, f.st, runID); got != "tool said: 42" {
		t.Fatalf("reply = %q", got)
	}
}

// A group never uses its owner's personal tool servers, whoever writes.
func TestGroupDoesNotUseTheOwnersToolServers(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	mcpSrv := fakeMCP(t)
	f := setup(t, llmSrv.URL)
	f.st.CreateMCPServer(f.owner, "fake", mcpSrv.URL, "", store.ApprovalAuto)
	bob, _ := f.st.CreateUser("bob", "x", "user")
	tid := newGroup(t, f, bob)
	f.st.SetThreadTool(tid, store.ToolMemory, false) // without its own memory a group has no tools at all here
	runID, _ := f.eng.Submit(tid, bob, "hi all", nil)
	r := waitRun(t, f.st, runID)
	if r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	// None of the owner's tools here -> plain echo, with the sender's name prefixed.
	if got := lastAssistant(t, f.st, runID); got != "echo: [bob]: hi all" {
		t.Fatalf("reply = %q", got)
	}
	// Bob paid for his own turn.
	if used, _ := f.st.UsedTokens(bob, "monthly"); used != 15 {
		t.Fatalf("bob used %d", used)
	}
}

func TestCompaction(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	st, _ := f.st.LoadSettings()
	st.CompactionPairs, st.CompactionKeep = 4, 2
	f.st.SaveSettings(st)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)

	var last int64
	for i := 0; i < 6; i++ {
		id, err := f.eng.Submit(tid, f.owner, fmt.Sprintf("msg %d", i), nil)
		if err != nil {
			t.Fatal(err)
		}
		waitRun(t, f.st, id)
		last = id
	}
	th, _ := f.st.ThreadByID(tid)
	if th.Summary != "SUMMARY" {
		t.Fatalf("summary = %q", th.Summary)
	}
	// Before run 6, 5 pairs existed (>4): fold all but the last 2.
	kept, _ := f.st.FinishedRunIDs(tid, th.CompactedUntilRun, last)
	if len(kept) != 2 {
		t.Fatalf("pairs kept verbatim = %d, want 2", len(kept))
	}
	// Originals are never deleted.
	if msgs, _ := f.st.ListMessages(tid, 100); len(msgs) != 12 {
		t.Fatalf("messages = %d, want 12", len(msgs))
	}
}

func TestQuotaBlocks(t *testing.T) {
	llmSrv, calls := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	f.st.SetQuota(f.owner, store.Quota{Period: "daily", TokenLimit: 10})
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	r1, _ := f.eng.Submit(tid, f.owner, "one", nil)
	waitRun(t, f.st, r1) // uses 15 tokens, over the limit afterwards
	r2, _ := f.eng.Submit(tid, f.owner, "two", nil)
	r := waitRun(t, f.st, r2)
	if r.Status != store.RunFailed || !strings.Contains(r.Error, "quota") {
		t.Fatalf("want quota failure, got %s %q", r.Status, r.Error)
	}
	if calls.Load() != 1 {
		t.Fatalf("LLM called %d times, want 1", calls.Load())
	}
}

func TestStopAndSnapshot(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 150*time.Millisecond)
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	runID, _ := f.eng.Submit(tid, f.owner, "a b c d e f g h", nil)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, content, _, _, _, ok := f.eng.Snapshot(tid); ok && content != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no snapshot")
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.eng.Stop(tid)
	r := waitRun(t, f.st, runID)
	if r.Status != store.RunCancelled {
		t.Fatalf("status = %s", r.Status)
	}
	msgs, _ := f.st.RunMessages(runID)
	if len(msgs) != 2 || msgs[1].Status != store.MsgInterrupted || msgs[1].Content == "" {
		t.Fatalf("partial answer not kept: %+v", msgs)
	}
}

func TestEditLatestAndRegenerate(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	r1, _ := f.eng.Submit(tid, f.owner, "first", nil)
	waitRun(t, f.st, r1)
	r2, _ := f.eng.Submit(tid, f.owner, "second", nil)
	waitRun(t, f.st, r2)

	if err := f.eng.Edit(tid, r1, ptr("changed")); err != ErrNotLatest {
		t.Fatalf("editing an older message: err = %v, want ErrNotLatest", err)
	}
	if err := f.eng.Edit(tid, r2, ptr("second, edited")); err != nil {
		t.Fatal(err)
	}
	if r := waitRun(t, f.st, r2); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	msgs, _ := f.st.RunMessages(r2)
	if len(msgs) != 2 || !msgs[0].Edited || msgs[1].Content != "echo: second, edited" {
		t.Fatalf("after edit: %+v", msgs)
	}
	// Regenerate (nil content) keeps the text and replaces the answer.
	if err := f.eng.Edit(tid, r2, nil); err != nil {
		t.Fatal(err)
	}
	waitRun(t, f.st, r2)
	if msgs, _ = f.st.RunMessages(r2); len(msgs) != 2 || msgs[1].Content != "echo: second, edited" {
		t.Fatalf("after regenerate: %+v", msgs)
	}
}

func TestEditWhileStreaming(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 100*time.Millisecond)
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	r1, _ := f.eng.Submit(tid, f.owner, "a b c d e f g", nil)
	for f.eng.ActiveRunID(tid) != r1 {
		time.Sleep(5 * time.Millisecond)
	}
	if err := f.eng.Edit(tid, r1, ptr("short")); err != nil {
		t.Fatal(err)
	}
	if r := waitRun(t, f.st, r1); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	if got := lastAssistant(t, f.st, r1); got != "echo: short" {
		t.Fatalf("reply = %q", got)
	}
}

func TestImagesSentAsContentParts(t *testing.T) {
	var body atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body.Store(string(b))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a cat\"}}]}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	f := setup(t, srv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	png := []byte("\x89PNG\r\n\x1a\nfake")
	att, err := f.st.SaveUpload(png, "image/png", "cat.png", "", "")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := f.eng.Submit(tid, f.owner, "what is this?", []store.NewAttachment{att})
	if err != nil {
		t.Fatal(err)
	}
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	got, _ := body.Load().(string)
	if !strings.Contains(got, `"type":"image_url"`) || !strings.Contains(got, "data:image/png;base64,") ||
		!strings.Contains(got, `what is this?","type":"text"`) {
		t.Fatalf("request did not carry the image: %s", got)
	}
}

func ptr(s string) *string { return &s }
