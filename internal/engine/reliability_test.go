package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"knitnox/internal/store"
)

// hangingLLM sends the start of an answer and then nothing, like a provider
// whose connection stays open but dead.
func hangingLLM(t *testing.T) *httptest.Server {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"half an \"}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	return srv
}

func saveSettings(t *testing.T, st *store.Store, change func(*store.Settings)) {
	t.Helper()
	s, err := st.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	change(&s)
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
}

// A provider that goes silent fails the run, keeps the text so far, gives the
// slot back and lets the next message in the chat run.
func TestSilentProviderFailsTheRunAndFreesTheSlot(t *testing.T) {
	f := setup(t, hangingLLM(t).URL)
	saveSettings(t, f.st, func(s *store.Settings) { s.LLMIdleSeconds = 1; s.MaxParallelRuns = 1 })
	tid, _ := f.st.CreateThread(f.owner, f.modelID)

	runID, err := f.eng.Submit(tid, f.owner, "hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := waitRun(t, f.st, runID)
	if r.Status != store.RunFailed || !strings.Contains(r.Error, "stopped answering") {
		t.Fatalf("run = %+v", r)
	}
	if got := lastAssistant(t, f.st, runID); got != "half an " {
		t.Fatalf("partial answer = %q", got)
	}
	if n := f.eng.RunningCount(); n != 0 {
		t.Fatalf("%d slots still held", n)
	}
	// The only slot is free again: a chat on a healthy model answers.
	good, _ := fakeLLM(t, 0)
	pid, _ := f.st.CreateProvider(nil, "good", good.URL, "k")
	f.st.CreateModel(pid, "good-model", 100000, 0, 0, false)
	models, _ := f.st.ListModels(pid)
	tid2, _ := f.st.CreateThread(f.owner, models[0].ID)
	run2, _ := f.eng.Submit(tid2, f.owner, "still alive?", nil)
	if r := waitRun(t, f.st, run2); r.Status != store.RunDone {
		t.Fatalf("second run = %+v", r)
	}
}

// The hard limit on one run ends it even when the idle timeout is off.
func TestRunTimeLimit(t *testing.T) {
	f := setup(t, hangingLLM(t).URL)
	saveSettings(t, f.st, func(s *store.Settings) { s.LLMIdleSeconds = 0; s.RunMaxMinutes = 1 })
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	// One minute is the smallest setting; shorten it by starting the clock late.
	f.eng.runLimit = func(time.Duration) time.Duration { return 300 * time.Millisecond }

	runID, _ := f.eng.Submit(tid, f.owner, "hello", nil)
	r := waitRun(t, f.st, runID)
	if r.Status != store.RunFailed || !strings.Contains(r.Error, "longest a reply may take") {
		t.Fatalf("run = %+v", r)
	}
	if n := f.eng.RunningCount(); n != 0 {
		t.Fatalf("%d slots still held", n)
	}
}

// A panic inside one run must not take the server down: the run is marked
// failed, its slot is free, and the same chat answers the next message.
func TestPanicInARunIsContained(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)

	var boom int64
	f.eng.beforeRun = func(r *store.Run) {
		if r.ID == boom {
			panic("boom")
		}
	}
	first, _ := f.eng.Submit(tid, f.owner, "one", nil)
	waitRun(t, f.st, first)

	// Queue the run that panics and one behind it.
	f.eng.mu.Lock() // hold the worker off until boom is known
	bad, err := f.st.EnqueueMessage(tid, f.owner, "two", 10, nil)
	if err != nil {
		f.eng.mu.Unlock()
		t.Fatal(err)
	}
	boom = bad
	f.eng.mu.Unlock()
	next, _ := f.eng.Submit(tid, f.owner, "three", nil)

	if r := waitRun(t, f.st, bad); r.Status != store.RunFailed || !strings.Contains(r.Error, "went wrong inside Knitnox") {
		t.Fatalf("panicking run = %+v", r)
	}
	if r := waitRun(t, f.st, next); r.Status != store.RunDone {
		t.Fatalf("run after the panic = %+v", r)
	}
	if got := lastAssistant(t, f.st, next); got != "echo: three" {
		t.Fatalf("answer after the panic = %q", got)
	}
	if n := f.eng.RunningCount(); n != 0 {
		t.Fatalf("%d slots still held", n)
	}
}

// A panic in a hook (push notifications) is contained too.
func TestPanicInAHookIsContained(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	f.eng.OnRunFinished = func(RunEvent) { panic("push library exploded") }
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	for _, text := range []string{"one", "two"} {
		runID, _ := f.eng.Submit(tid, f.owner, text, nil)
		if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
			t.Fatalf("run = %+v", r)
		}
	}
	time.Sleep(50 * time.Millisecond) // let the second hook goroutine panic and be caught
}
