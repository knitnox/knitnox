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
	"time"

	"knitnox/internal/store"
)

// waitApproval polls until the thread has a pending tool approval.
func waitApproval(t *testing.T, st *store.Store, tid int64) store.ToolApproval {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pend, _ := st.PendingApprovals(tid)
		for _, list := range pend {
			if len(list) > 0 {
				return list[0]
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no pending approval appeared")
	return store.ToolApproval{}
}

func toolResult(t *testing.T, st *store.Store, runID int64) string {
	msgs, _ := st.RunMessages(runID)
	for _, m := range msgs {
		if m.Role == "tool" {
			return m.Content
		}
	}
	t.Fatalf("no tool message in run %d", runID)
	return ""
}

func TestToolApprovalAllow(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	mcpSrv := fakeMCP(t)
	f := setup(t, llmSrv.URL)
	f.st.CreateMCPServer(f.owner, "fake", mcpSrv.URL, "", store.ApprovalAsk)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	runID, _ := f.eng.Submit(tid, f.owner, "look it up", nil)

	a := waitApproval(t, f.st, tid)
	if a.ToolName != "lookup" || a.RunID != runID || a.Status != store.ApprovalPending {
		t.Fatalf("unexpected approval %+v", a)
	}
	// The run is parked: no tool message yet.
	if msgs, _ := f.st.RunMessages(runID); len(msgs) != 2 {
		t.Fatalf("run should be waiting with 2 messages, has %d", len(msgs))
	}
	if err := f.eng.Decide(a.ID, f.owner, true); err != nil {
		t.Fatal(err)
	}
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	if got := lastAssistant(t, f.st, runID); got != "tool said: 42" {
		t.Fatalf("reply = %q", got)
	}
	if err := f.eng.Decide(a.ID, f.owner, true); err != ErrAlreadyDecided {
		t.Fatalf("second decision: err = %v", err)
	}
	if got, _ := f.st.ApprovalByID(a.ID); got.Status != store.ApprovalAllowed || got.DecidedBy != f.owner {
		t.Fatalf("stored approval = %+v", got)
	}
}

func TestToolApprovalDeny(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	mcpSrv := fakeMCP(t)
	f := setup(t, llmSrv.URL)
	f.st.CreateMCPServer(f.owner, "fake", mcpSrv.URL, "", store.ApprovalAsk)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	runID, _ := f.eng.Submit(tid, f.owner, "look it up", nil)
	a := waitApproval(t, f.st, tid)
	if err := f.eng.Decide(a.ID, f.owner, false); err != nil {
		t.Fatal(err)
	}
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	if got := toolResult(t, f.st, runID); !strings.HasPrefix(got, "The user did not allow") {
		t.Fatalf("tool result = %q", got)
	}
}

func TestToolApprovalTimeoutAndStop(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	mcpSrv := fakeMCP(t)
	f := setup(t, llmSrv.URL)
	f.eng.ApprovalTimeout = 150 * time.Millisecond
	f.st.CreateMCPServer(f.owner, "fake", mcpSrv.URL, "", store.ApprovalAsk)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)

	runID, _ := f.eng.Submit(tid, f.owner, "look it up", nil)
	a := waitApproval(t, f.st, tid)
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	if got, _ := f.st.ApprovalByID(a.ID); got.Status != store.ApprovalExpired {
		t.Fatalf("approval should have expired, is %s", got.Status)
	}
	if got := toolResult(t, f.st, runID); !strings.HasPrefix(got, "Nobody approved") {
		t.Fatalf("tool result = %q", got)
	}

	// Stopping a run that waits for approval cancels it and expires the approval.
	f.eng.ApprovalTimeout = time.Hour
	run2, _ := f.eng.Submit(tid, f.owner, "look again", nil)
	a2 := waitApproval(t, f.st, tid)
	f.eng.Stop(tid)
	if r := waitRun(t, f.st, run2); r.Status != store.RunCancelled {
		t.Fatalf("status = %s", r.Status)
	}
	if got, _ := f.st.ApprovalByID(a2.ID); got.Status != store.ApprovalExpired {
		t.Fatalf("approval after stop = %s", got.Status)
	}
}

func TestParallelRunLimit(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 60*time.Millisecond)
	f := setup(t, llmSrv.URL)
	st, _ := f.st.LoadSettings()
	st.MaxParallelRuns = 1
	f.st.SaveSettings(st)
	t1, _ := f.st.CreateThread(f.owner, f.modelID)
	t2, _ := f.st.CreateThread(f.owner, f.modelID)
	r1, _ := f.eng.Submit(t1, f.owner, "a b c d e f", nil)
	r2, _ := f.eng.Submit(t2, f.owner, "g h i j k l", nil)

	// With one slot, at some point the second thread must be waiting while
	// the first streams, and never both running.
	sawWait := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f.eng.RunningCount() > 1 {
			t.Fatal("two runs executing at once with MaxParallelRuns = 1")
		}
		if f.eng.Waiting(t1) || f.eng.Waiting(t2) {
			sawWait = true
		}
		a, _ := f.st.RunByID(r1)
		b, _ := f.st.RunByID(r2)
		if a.Status == store.RunDone && b.Status == store.RunDone {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !sawWait {
		t.Fatal("never observed a run waiting for a slot")
	}
	for _, id := range []int64{r1, r2} {
		if r := waitRun(t, f.st, id); r.Status != store.RunDone {
			t.Fatalf("run %d: %s %s", id, r.Status, r.Error)
		}
	}
}

// captureLLM records the last request body and replies with one word.
func captureLLM(t *testing.T) (*httptest.Server, *atomic.Value) {
	var body atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body.Store(string(b))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"let me \"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, &body
}

func TestInstructionsFileTextAndReasoning(t *testing.T) {
	srv, body := captureLLM(t)
	f := setup(t, srv.URL)
	f.st.SetInstructions(f.owner, "Always answer in haiku.")
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	f.st.SetThreadInstructions(tid, "This chat is about gardening.")
	doc, err := f.st.SaveUpload([]byte("tomatoes need sun"), "text/plain", "notes.txt", store.KindFile, "tomatoes need sun")
	if err != nil {
		t.Fatal(err)
	}
	runID, _ := f.eng.Submit(tid, f.owner, "what do tomatoes need?", []store.NewAttachment{doc})
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body.Load().(string)), &req); err != nil {
		t.Fatal(err)
	}
	sys := req.Messages[0].Content
	if !strings.Contains(sys, "Always answer in haiku.") || !strings.Contains(sys, "This chat is about gardening.") {
		t.Fatalf("system prompt lacks instructions: %q", sys)
	}
	last := req.Messages[len(req.Messages)-1].Content
	if !strings.Contains(last, `<attached_file name="notes.txt">`) || !strings.Contains(last, "tomatoes need sun") {
		t.Fatalf("file text not inlined: %q", last)
	}
	msgs, _ := f.st.RunMessages(runID)
	if msgs[1].Reasoning != "let me think" || msgs[1].Content != "ok" {
		t.Fatalf("reasoning/content = %q / %q", msgs[1].Reasoning, msgs[1].Content)
	}
	if used, window := f.eng.ContextEstimate(mustThread(t, f.st, tid)); used <= 0 || window != 100000 {
		t.Fatalf("context estimate = %d/%d", used, window)
	}
}

func TestGroupChatIgnoresPersonalInstructions(t *testing.T) {
	srv, body := captureLLM(t)
	f := setup(t, srv.URL)
	f.st.SetInstructions(f.owner, "SECRET-PERSONAL")
	bob, _ := f.st.CreateUser("bob", "x", "user")
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	tok, _ := f.st.CreateInvite(tid, f.owner, "member", time.Hour, 1)
	f.st.AcceptInvite(tok, bob)
	runID, _ := f.eng.Submit(tid, bob, "hi", nil)
	waitRun(t, f.st, runID)
	if strings.Contains(body.Load().(string), "SECRET-PERSONAL") {
		t.Fatal("owner's personal instructions leaked into a group chat")
	}
}

func TestSearchAndPaging(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	var last int64
	for i := 0; i < 5; i++ {
		last, _ = f.eng.Submit(tid, f.owner, fmt.Sprintf("message number %d about pelicans", i), nil)
		waitRun(t, f.st, last)
	}
	hits, err := f.st.SearchMessages(f.owner, "pelican", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 10 { // 5 user + 5 echoed replies
		t.Fatalf("hits = %d, want 10", len(hits))
	}
	if !strings.Contains(hits[0].Snippet, "[[pelican") {
		t.Fatalf("snippet lacks highlight: %q", hits[0].Snippet)
	}
	// Another user sees nothing.
	bob, _ := f.st.CreateUser("bob", "x", "user")
	if hits, _ := f.st.SearchMessages(bob, "pelican", 50); len(hits) != 0 {
		t.Fatalf("bob saw %d hits", len(hits))
	}
	// Paging: last 2 messages, then the page before them.
	page, _ := f.st.ListMessages(tid, 2)
	if len(page) != 2 || page[0].RunID != last {
		t.Fatalf("last page = %+v", page)
	}
	older, _ := f.st.ListMessagesBefore(tid, last, 4)
	if len(older) != 4 || older[len(older)-1].RunID >= last {
		t.Fatalf("older page wrong: %d messages", len(older))
	}
	if !f.st.HasMessagesBefore(tid, older[0].RunID) || f.st.HasMessagesBefore(tid, 1) {
		t.Fatal("HasMessagesBefore wrong")
	}
	from, _ := f.st.ListMessagesFrom(tid, older[0].RunID, 100)
	if len(from) != 6 {
		t.Fatalf("ListMessagesFrom = %d, want 6", len(from))
	}
}

func mustThread(t *testing.T, st *store.Store, id int64) *store.Thread {
	th, err := st.ThreadByID(id)
	if err != nil {
		t.Fatal(err)
	}
	return th
}
