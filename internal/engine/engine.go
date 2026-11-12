// Package engine runs AI turns in the background, independent of any browser.
//
// Each thread has at most one worker goroutine. The worker pulls pending runs
// (queued user messages) from the database in FIFO order and executes them one
// at a time. Progress is published on the hub; browsers are only viewers.
// A global slot limit caps how many generations run at once across all threads.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
	"unicode/utf16"

	"knitnox/internal/hub"
	"knitnox/internal/netguard"
	"knitnox/internal/store"
)

// Event types published on the hub.
const (
	EvMessages  = "messages"  // message list changed: refetch
	EvToken     = "token"     // {message_id, delta, len[, kind:"reasoning"]}
	EvSnapshot  = "snapshot"  // {message_id, content, len, reasoning, rlen}
	EvRun       = "run"       // {run_id, status, error}
	EvQueue     = "queue"     // {pending}
	EvCompacted = "compacted" // {}
	EvApproval  = "approval"  // {id, tool, server}: a tool call needs a decision
)

// RunWaiting is a UI-only status published while a run waits for a free slot.
const RunWaiting = "waiting"

// RunEvent is passed to the OnRunFinished hook.
type RunEvent struct {
	ThreadID, RunID, AuthorID int64
	Status, Error             string
}

// ApprovalEvent is passed to the OnApproval hook when a tool call needs a decision.
type ApprovalEvent struct {
	ThreadID, RunID, ApprovalID int64
	Approvers                   []int64 // users who may decide (run author + thread owner)
	ToolName, ServerName        string
}

type Engine struct {
	st  *store.Store
	hub *hub.Hub
	log *slog.Logger

	// Optional hooks, called in their own goroutine (used for push notifications).
	OnRunFinished func(RunEvent)
	OnApproval    func(ApprovalEvent)
	// ApprovalTimeout overrides the admin setting when > 0 (tests).
	ApprovalTimeout time.Duration
	// runLimit, when set, replaces the length of the per-run time limit (tests).
	runLimit func(time.Duration) time.Duration
	// beforeRun is called before every run (tests use it to make a run panic).
	beforeRun func(*store.Run)
	// VideoPoll is how often a video being made is looked at (0 = every 5 seconds; tests shorten it).
	VideoPoll time.Duration

	baseCtx context.Context
	stopAll context.CancelFunc
	wg      sync.WaitGroup
	slots   *slots

	mu        sync.Mutex
	workers   map[int64]bool         // thread has a live worker
	active    map[int64]*activeRun   // thread -> currently executing run
	live      map[int64]*liveMessage // thread -> message being streamed
	waiting   map[int64]bool         // thread's run is waiting for a slot
	approvals map[int64]chan string  // approval id -> decision
}

type activeRun struct {
	runID   int64
	cancel  context.CancelFunc
	stopped bool // user pressed stop
}

// liveMessage mirrors the streaming message in memory so that late joiners get
// a consistent snapshot. Lengths are in UTF-16 code units to match JS.
type liveMessage struct {
	mu        sync.Mutex
	id        int64
	content   []byte
	u16       int
	reasoning []byte
	r16       int
}

func New(st *store.Store, h *hub.Hub, log *slog.Logger) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		st: st, hub: h, log: log,
		baseCtx: ctx, stopAll: cancel, slots: newSlots(),
		workers:   map[int64]bool{},
		active:    map[int64]*activeRun{},
		live:      map[int64]*liveMessage{},
		waiting:   map[int64]bool{},
		approvals: map[int64]chan string{},
	}
}

// Start recovers from a previous crash and resumes queued work.
func (e *Engine) Start() error {
	if st, err := e.st.LoadSettings(); err == nil {
		netguard.SetBlockPrivate(st.BlockPrivateURLs)
	}
	if err := e.st.RecoverAfterRestart(); err != nil {
		return err
	}
	if err := e.st.FailPendingFeedMedia(); err != nil {
		return err
	}
	ids, err := e.st.ThreadsWithPendingRuns()
	if err != nil {
		return err
	}
	for _, id := range ids {
		e.wake(id)
	}
	e.wg.Add(1)
	go e.scheduler() // starts scheduled agents (agents.go)
	return nil
}

// Shutdown cancels in-flight runs (they become "interrupted") and waits.
func (e *Engine) Shutdown(timeout time.Duration) {
	e.stopAll()
	done := make(chan struct{})
	go func() { e.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
		e.log.Warn("engine shutdown timed out")
	}
}

// Submit queues a user message. It returns store.ErrQueueFull when the
// thread already has the maximum number of pending messages.
func (e *Engine) Submit(threadID, userID int64, content string, files []store.NewAttachment) (int64, error) {
	st, err := e.st.LoadSettings()
	if err != nil {
		return 0, err
	}
	runID, err := e.st.EnqueueMessage(threadID, userID, content, st.QueueMax, files)
	if err != nil {
		return 0, err
	}
	e.hub.Publish(threadID, EvMessages, nil)
	e.publishQueue(threadID)
	e.wake(threadID)
	return runID, nil
}

// ErrNotLatest is returned when editing a message that already has later turns.
var ErrNotLatest = errors.New("only the latest message can be edited")

// Edit changes a user's message and re-runs it. content == nil regenerates the
// answer with the same text (used by "Regenerate" and "Retry").
//   - queued message:   the text is replaced in place
//   - latest message:   any running answer is stopped, the old answer removed,
//     and the turn goes back into the queue (keeping its position)
func (e *Engine) Edit(threadID, runID int64, content *string) error {
	run, err := e.st.RunByID(runID)
	if err != nil || run.ThreadID != threadID {
		return store.ErrNotFound
	}
	if run.Status == store.RunPending {
		if content == nil {
			return nil
		}
		if err := e.st.UpdateUserMessage(runID, *content); err != nil {
			return err
		}
		e.hub.Publish(threadID, EvMessages, nil)
		return nil
	}
	if e.st.LastRunID(threadID) != runID {
		return ErrNotLatest
	}
	if e.ActiveRunID(threadID) == runID {
		e.Stop(threadID)
		deadline := time.Now().Add(15 * time.Second)
		for e.ActiveRunID(threadID) == runID {
			if time.Now().After(deadline) {
				return errors.New("the current answer did not stop in time; try again")
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	if err := e.st.ResetRun(runID, content); err != nil {
		return err
	}
	e.hub.Publish(threadID, EvMessages, nil)
	e.publishQueue(threadID)
	e.wake(threadID)
	return nil
}

// Stop cancels the run currently executing in the thread, if any.
func (e *Engine) Stop(threadID int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if a := e.active[threadID]; a != nil {
		a.stopped = true
		a.cancel()
		return true
	}
	return false
}

// CancelQueued removes a pending run (and its message) from the queue.
func (e *Engine) CancelQueued(threadID, runID int64) error {
	ok, err := e.st.DeletePendingRun(threadID, runID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("message is no longer queued")
	}
	e.hub.Publish(threadID, EvMessages, nil)
	e.publishQueue(threadID)
	return nil
}

// ErrAlreadyDecided is returned when a tool approval was answered before.
var ErrAlreadyDecided = errors.New("this tool call was already decided")

// Decide answers a pending tool approval. allow=false denies it.
func (e *Engine) Decide(approvalID, userID int64, allow bool) error {
	status := store.ApprovalDenied
	if allow {
		status = store.ApprovalAllowed
	}
	ok, err := e.st.DecideApproval(approvalID, userID, status)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAlreadyDecided
	}
	e.mu.Lock()
	ch := e.approvals[approvalID]
	e.mu.Unlock()
	if ch != nil {
		select {
		case ch <- status:
		default:
		}
	}
	return nil
}

// Snapshot returns the in-progress streamed message for a thread, if any.
func (e *Engine) Snapshot(threadID int64) (id int64, content string, u16 int, reasoning string, r16 int, ok bool) {
	e.mu.Lock()
	lm := e.live[threadID]
	e.mu.Unlock()
	if lm == nil {
		return 0, "", 0, "", 0, false
	}
	lm.mu.Lock()
	defer lm.mu.Unlock()
	return lm.id, string(lm.content), lm.u16, string(lm.reasoning), lm.r16, true
}

// ActiveRunID returns the running run of a thread (0 if idle).
func (e *Engine) ActiveRunID(threadID int64) int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if a := e.active[threadID]; a != nil {
		return a.runID
	}
	return 0
}

// Waiting reports whether the thread's current run is waiting for a free slot.
func (e *Engine) Waiting(threadID int64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.waiting[threadID]
}

// RunningCount is the number of generations in flight (admin dashboard).
func (e *Engine) RunningCount() int { return e.slots.used() }

func (e *Engine) publishQueue(threadID int64) {
	e.hub.Publish(threadID, EvQueue, map[string]any{"pending": e.st.PendingCount(threadID)})
}

// wake ensures a worker is running for the thread.
func (e *Engine) wake(threadID int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.workers[threadID] || e.baseCtx.Err() != nil {
		return
	}
	e.workers[threadID] = true
	e.wg.Add(1)
	go e.work(threadID)
}

func (e *Engine) work(threadID int64) {
	defer e.wg.Done()
	exit := func() {
		e.mu.Lock()
		delete(e.workers, threadID)
		e.mu.Unlock()
	}
	for {
		if e.baseCtx.Err() != nil {
			exit()
			return
		}
		run, err := e.st.NextPendingRun(threadID)
		if err != nil {
			e.log.Error("next run", "thread", threadID, "err", err)
			exit()
			return
		}
		if run == nil {
			// Re-check under the lock so a concurrent Submit cannot be missed:
			// Submit commits before calling wake, and wake needs this lock.
			e.mu.Lock()
			run, err = e.st.NextPendingRun(threadID)
			if run == nil || err != nil {
				delete(e.workers, threadID)
				e.mu.Unlock()
				return
			}
			e.mu.Unlock()
		}
		e.safeProcess(run)
	}
}

// finishRun records how a run ended. The write is tried a few times: if it
// were lost, the run would look as if it was still going until the next restart.
func (e *Engine) finishRun(runID int64, status, errMsg string, steps int) {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		if err = e.st.FinishRun(runID, status, errMsg, steps); err == nil {
			return
		}
	}
	e.log.Error("could not record how a run ended; it will be marked interrupted at the next restart", "run", runID, "status", status, "err", err)
}

// recoverPanic keeps one broken piece of background work from taking the
// whole server down. Use it as: defer e.recoverPanic("what was running").
func (e *Engine) recoverPanic(what string) {
	if v := recover(); v != nil {
		e.log.Error("panic in background work", "where", what, "value", fmt.Sprint(v), "stack", string(debug.Stack()))
	}
}

// safeProcess runs one run. If the code behind it panics, the run is marked
// failed (so it is not picked up again) and the worker carries on.
func (e *Engine) safeProcess(run *store.Run) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		e.log.Error("panic while running a reply", "run", run.ID, "thread", run.ThreadID, "value", fmt.Sprint(v), "stack", string(debug.Stack()))
		const msg = "something went wrong inside Knitnox; the details are in the server log"
		if err := e.st.FailStreamingMessages(run.ID); err != nil {
			e.log.Error("close messages after panic", "run", run.ID, "err", err)
		}
		e.finishRun(run.ID, store.RunFailed, msg, 0)
		e.hub.Publish(run.ThreadID, EvRun, map[string]any{"run_id": run.ID, "status": store.RunFailed, "error": msg})
		e.hub.Publish(run.ThreadID, EvMessages, nil)
	}()
	if e.beforeRun != nil {
		e.beforeRun(run)
	}
	e.process(run)
}

// hook calls an optional callback in its own goroutine, shielded from panics.
func (e *Engine) hook(name string, fn func()) {
	go func() {
		defer e.recoverPanic(name)
		fn()
	}()
}

// process executes one run and records its final status.
func (e *Engine) process(run *store.Run) {
	ctx, cancel := context.WithCancel(e.baseCtx)
	ar := &activeRun{runID: run.ID, cancel: cancel}
	e.mu.Lock()
	e.active[run.ThreadID] = ar
	e.mu.Unlock()
	defer func() {
		cancel()
		e.mu.Lock()
		delete(e.active, run.ThreadID)
		delete(e.live, run.ThreadID)
		delete(e.waiting, run.ThreadID)
		e.mu.Unlock()
	}()

	// Global concurrency cap: wait for a slot while still "pending" so a
	// restart simply re-queues the run.
	limit := func() int {
		st, err := e.st.LoadSettings()
		if err != nil {
			return 0
		}
		return st.MaxParallelRuns
	}
	e.mu.Lock()
	e.waiting[run.ThreadID] = true
	e.mu.Unlock()
	e.hub.Publish(run.ThreadID, EvRun, map[string]any{"run_id": run.ID, "status": RunWaiting})
	if _, err := e.slots.acquire(ctx, limit); err != nil {
		if ar.stopped { // user cancelled while waiting
			e.finishRun(run.ID, store.RunCancelled, "stopped", 0)
			e.hub.Publish(run.ThreadID, EvRun, map[string]any{"run_id": run.ID, "status": store.RunCancelled})
			e.hub.Publish(run.ThreadID, EvMessages, nil)
		}
		return // shutdown: stays pending
	}
	defer e.slots.release()
	e.mu.Lock()
	delete(e.waiting, run.ThreadID)
	e.mu.Unlock()

	if err := e.st.StartRun(run.ID); err != nil {
		e.log.Error("start run", "run", run.ID, "err", err)
	}
	e.hub.Publish(run.ThreadID, EvRun, map[string]any{"run_id": run.ID, "status": store.RunRunning})
	e.publishQueue(run.ThreadID)
	e.hub.Publish(run.ThreadID, EvMessages, nil)

	// Hard limit on one run, counted from the moment it starts (waiting for a
	// slot does not count). Together with the idle timeout on model calls it
	// guarantees that a slot is always given back.
	runCtx, cancelRun := ctx, context.CancelFunc(func() {})
	maxMin := 0
	if st, err := e.st.LoadSettings(); err == nil {
		maxMin = st.RunMaxMinutes
	}
	if maxMin > 0 {
		limit := time.Duration(maxMin) * time.Minute
		if e.runLimit != nil {
			limit = e.runLimit(limit)
		}
		runCtx, cancelRun = context.WithTimeout(ctx, limit)
	}
	defer cancelRun()

	steps, err := e.execute(runCtx, run)

	status, errMsg := store.RunDone, ""
	if err != nil {
		e.mu.Lock()
		stopped := ar.stopped
		e.mu.Unlock()
		switch {
		case stopped:
			status, errMsg = store.RunCancelled, "stopped"
		case e.baseCtx.Err() != nil:
			status, errMsg = store.RunInterrupted, "server shutting down"
		case ctx.Err() == nil && runCtx.Err() != nil:
			status, errMsg = store.RunFailed, fmt.Sprintf("stopped after %d minutes, the longest a reply may take (an admin can change this limit)", maxMin)
			e.log.Warn("run hit the time limit", "run", run.ID, "thread", run.ThreadID, "minutes", maxMin)
		default:
			status, errMsg = store.RunFailed, err.Error()
			e.log.Warn("run failed", "run", run.ID, "thread", run.ThreadID, "err", err)
		}
	}
	e.finishRun(run.ID, status, errMsg, steps)
	e.st.TouchThread(run.ThreadID)
	e.agentHealth(run, status) // tell the owner's feed when an agent starts having trouble, or works again (agents.go)
	e.hub.Publish(run.ThreadID, EvRun, map[string]any{"run_id": run.ID, "status": status, "error": errMsg})
	e.hub.Publish(run.ThreadID, EvMessages, nil)
	if e.OnRunFinished != nil {
		ev := RunEvent{ThreadID: run.ThreadID, RunID: run.ID, AuthorID: run.AuthorUserID, Status: status, Error: errMsg}
		e.hook("run-finished hook", func() { e.OnRunFinished(ev) })
	}
}

// ---- live streaming buffer ----

func (e *Engine) beginLive(threadID, msgID int64) *liveMessage {
	lm := &liveMessage{id: msgID}
	e.mu.Lock()
	e.live[threadID] = lm
	e.mu.Unlock()
	return lm
}

func (e *Engine) endLive(threadID int64) {
	e.mu.Lock()
	delete(e.live, threadID)
	e.mu.Unlock()
}

func (lm *liveMessage) append(delta string) (int, string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.content = append(lm.content, delta...)
	lm.u16 += utf16Len(delta)
	return lm.u16, string(lm.content)
}

func (lm *liveMessage) appendReasoning(delta string) int {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.reasoning = append(lm.reasoning, delta...)
	lm.r16 += utf16Len(delta)
	return lm.r16
}

func (lm *liveMessage) text() (content, reasoning string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	return string(lm.content), string(lm.reasoning)
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s { // invalid bytes decode to U+FFFD (1 unit), like JS
		n += utf16.RuneLen(r)
	}
	return n
}
