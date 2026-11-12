package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"knitnox/internal/store"
)

// llmRequest is what a scripted model sees of each call.
type llmRequest struct {
	System   string
	Messages int
	LastRole string
	Last     string
	Tools    []string
}

func (r llmRequest) hasTool(name string) bool {
	for _, t := range r.Tools {
		if t == name {
			return true
		}
	}
	return false
}

// scriptLLM is an OpenAI-compatible endpoint driven by a function: it returns
// either text, or a tool name plus JSON arguments to call. Every request is recorded.
func scriptLLM(t *testing.T, script func(llmRequest) (text, tool, args string)) (*httptest.Server, func() []llmRequest) {
	var mu sync.Mutex
	var seen []llmRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		in := llmRequest{Messages: len(req.Messages)}
		if len(req.Messages) > 0 {
			in.System = req.Messages[0].Content
			in.LastRole = req.Messages[len(req.Messages)-1].Role
			in.Last = unstamp(req.Messages[len(req.Messages)-1].Content)
		}
		for _, tool := range req.Tools {
			in.Tools = append(in.Tools, tool.Function.Name)
		}
		mu.Lock()
		seen = append(seen, in)
		mu.Unlock()

		text, tool, args := script(in)
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(v any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		if tool != "" {
			send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "id": "c1", "type": "function", "function": map[string]any{"name": tool, "arguments": args}},
			}}, "finish_reason": "tool_calls"}}})
		} else {
			send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": text}}}})
		}
		send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, func() []llmRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]llmRequest(nil), seen...)
	}
}

func newAgent(t *testing.T, f *fixture, a store.Agent) *store.Agent {
	t.Helper()
	a.UserID = f.owner
	if a.MaxSteps == 0 {
		a.MaxSteps = 20
	}
	a.DailyRuns, a.HistoryRuns, a.Enabled = 48, 5, true
	if _, err := f.st.CreateAgent(&a, f.modelID); err != nil {
		t.Fatal(err)
	}
	return &a
}

func waitThreadIdle(t *testing.T, f *fixture, threadID int64) *store.Run {
	t.Helper()
	return waitRun(t, f.st, f.st.LastRunID(threadID))
}

// An agent run sees its job and open tasks, not earlier runs; finishing a task
// reports the result into the chat that asked for it.
func TestAgentRunFreshContextAndTaskReport(t *testing.T) {
	var taskID int64
	llmSrv, seen := scriptLLM(t, func(r llmRequest) (string, string, string) {
		if r.LastRole == "tool" {
			return "all done", "", ""
		}
		if strings.Contains(r.Last, "second") {
			return "nothing to do", "", ""
		}
		return "", "update_task", fmt.Sprintf(`{"task_id":%d,"status":"done","result":"reply drafted"}`, taskID)
	})
	f := setup(t, llmSrv.URL)
	chat, _ := f.st.CreateThread(f.owner, f.modelID)
	a := newAgent(t, f, store.Agent{Name: "Inbox helper", Instructions: "Draft replies to email."})
	taskID, _ = f.st.CreateTask(a.ID, "Reply to Sam", "about the invoice", "chat", chat)

	runID, err := f.eng.RunAgent(a, "first run")
	if err != nil {
		t.Fatal(err)
	}
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("run status %s: %s", r.Status, r.Error)
	}
	task, _ := f.st.TaskByID(taskID)
	if task.Status != store.TaskDone || task.Result != "reply drafted" {
		t.Fatalf("task = %+v", task)
	}
	first := seen()[0]
	for _, want := range []string{"Inbox helper", "Draft replies to email.", "Reply to Sam", "about the invoice"} {
		if !strings.Contains(first.System, want) {
			t.Errorf("agent prompt is missing %q", want)
		}
	}
	for _, tool := range []string{"list_tasks", "update_task", "add_task", "save_memory"} {
		if !first.hasTool(tool) {
			t.Errorf("agent run was not offered %s", tool)
		}
	}
	if first.hasTool("create_agent_task") {
		t.Error("an agent must not get the manager tools")
	}

	// The result was posted into the chat that created the task, without starting a turn there.
	msgs, _ := f.st.AllMessages(chat)
	if len(msgs) != 1 || msgs[0].Role != "assistant" || !strings.Contains(msgs[0].Content, "reply drafted") ||
		!strings.Contains(msgs[0].Content, "Inbox helper") {
		t.Fatalf("chat messages = %+v", msgs)
	}
	if f.st.ThreadBusy(chat) {
		t.Fatal("reporting a task must not queue a run in the chat")
	}

	// A second run starts fresh: system prompt + its own message only.
	runID, _ = f.eng.RunAgent(a, "second run")
	waitRun(t, f.st, runID)
	all := seen()
	last := all[len(all)-1]
	if last.Messages != 2 {
		t.Fatalf("second run sent %d messages, want 2 (no history)", last.Messages)
	}
	if strings.Contains(last.System, "- #") && strings.Contains(last.System, "[pending] Reply to Sam") {
		t.Error("finished task still listed as open")
	}
	if !strings.Contains(last.System, "all done") {
		t.Error("digest of the previous run is missing from the prompt")
	}
}

// A chat only gets the manager tools for agents its owner connected, and can
// hand a connected agent work.
func TestConnectedChatManagesAgent(t *testing.T) {
	llmSrv, seen := scriptLLM(t, func(r llmRequest) (string, string, string) {
		switch {
		case strings.HasPrefix(r.System, `You are "Researcher"`):
			return "agent finished", "", ""
		case r.LastRole == "tool":
			return "handed over", "", ""
		case r.hasTool("create_agent_task"):
			return "", "create_agent_task", `{"title":"Find three sources","details":"about tides","run_now":true}`
		}
		return "no agents here", "", ""
	})
	f := setup(t, llmSrv.URL)
	a := newAgent(t, f, store.Agent{Name: "Researcher", Description: "Looks things up", Instructions: "Research topics."})
	chat, _ := f.st.CreateThread(f.owner, f.modelID)

	// Not connected: no manager tools.
	runID, _ := f.eng.Submit(chat, f.owner, "hello", nil)
	waitRun(t, f.st, runID)
	if got := lastAssistant(t, f.st, runID); got != "no agents here" {
		t.Fatalf("unconnected chat reply = %q", got)
	}

	// Someone else's agent cannot be connected.
	bob, _ := f.st.CreateUser("bob", "x", "user")
	bobChat, _ := f.st.CreateThread(bob, f.modelID)
	f.st.SetThreadAgents(bobChat, bob, []int64{a.ID})
	if got, _ := f.st.ConnectedAgents(bobChat, bob); len(got) != 0 {
		t.Fatal("a user connected an agent they do not own")
	}

	if err := f.st.SetThreadAgents(chat, f.owner, []int64{a.ID}); err != nil {
		t.Fatal(err)
	}
	runID, _ = f.eng.Submit(chat, f.owner, "ask the researcher about tides", nil)
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	tasks, _ := f.st.ListTasks(a.ID, false, 10)
	if len(tasks) != 1 || tasks[0].Title != "Find three sources" || tasks[0].CreatedBy != "chat" || tasks[0].SourceThreadID != chat {
		t.Fatalf("tasks = %+v", tasks)
	}
	if r := waitThreadIdle(t, f, a.ThreadID); r.Status != store.RunDone {
		t.Fatalf("agent run %s: %s", r.Status, r.Error)
	}
	if got := lastAssistant(t, f.st, f.st.LastRunID(a.ThreadID)); got != "agent finished" {
		t.Fatalf("agent reply = %q", got)
	}
	var sawNote bool
	for _, r := range seen() {
		if strings.Contains(r.System, "agent "+fmt.Sprint(a.ID)+": Researcher") {
			sawNote = true
		}
	}
	if !sawNote {
		t.Error("the chat's system prompt does not list the connected agent")
	}
}

func TestSchedulerStartsDueAgents(t *testing.T) {
	llmSrv, _ := scriptLLM(t, func(llmRequest) (string, string, string) { return "ok", "", "" })
	f := setup(t, llmSrv.URL)
	due := func(a *store.Agent) {
		t.Helper()
		if _, err := f.st.DB.Exec(`UPDATE agents SET next_run_at = '2000-01-01 00:00:00' WHERE id = ?`, a.ID); err != nil {
			t.Fatal(err)
		}
	}
	hourly := newAgent(t, f, store.Agent{Name: "Hourly", Instructions: "x", ScheduleMinutes: 60})
	paused := newAgent(t, f, store.Agent{Name: "Paused", Instructions: "x", ScheduleMinutes: 60})
	f.st.SetAgentEnabled(paused.ID, false)
	idle := newAgent(t, f, store.Agent{Name: "Idle", Instructions: "x", ScheduleMinutes: 60, OnlyWithTasks: true})
	manual := newAgent(t, f, store.Agent{Name: "Manual", Instructions: "x"})

	if n := f.eng.RunDueAgents(); n != 0 {
		t.Fatalf("started %d agents before any was due", n)
	}
	for _, a := range []*store.Agent{hourly, paused, idle, manual} {
		due(a)
	}
	if n := f.eng.RunDueAgents(); n != 1 {
		t.Fatalf("started %d agents, want 1 (only the active hourly one)", n)
	}
	waitThreadIdle(t, f, hourly.ThreadID)
	for _, a := range []*store.Agent{paused, idle, manual} {
		if f.st.LastRunID(a.ThreadID) != 0 {
			t.Errorf("%s should not have run", a.Name)
		}
	}
	if n := f.eng.RunDueAgents(); n != 0 {
		t.Fatalf("agent ran again before its next interval (%d started)", n)
	}

	// With an open task, the tasks-only agent runs on its next turn.
	f.st.CreateTask(idle.ID, "do it", "", "user", 0)
	due(idle)
	if n := f.eng.RunDueAgents(); n != 1 {
		t.Fatalf("tasks-only agent with a task: started %d", n)
	}
	waitThreadIdle(t, f, idle.ThreadID)

	// The daily cap stops automatic runs.
	f.st.DB.Exec(`UPDATE agents SET daily_runs = 1 WHERE id = ?`, hourly.ID)
	due(hourly)
	if n := f.eng.RunDueAgents(); n != 0 {
		t.Fatalf("agent ran past its daily limit (%d started)", n)
	}
}

// An agent only sees the MCP tools on its allowlist.
func TestAgentToolAllowlist(t *testing.T) {
	llmSrv, seen := scriptLLM(t, func(llmRequest) (string, string, string) { return "ok", "", "" })
	f := setup(t, llmSrv.URL)
	mcpSrv := fakeMCP(t)
	if err := f.st.CreateMCPServer(f.owner, "tools", mcpSrv.URL, "", "auto"); err != nil {
		t.Fatal(err)
	}
	servers, _ := f.st.ListMCPServers(f.owner)
	sid := servers[0].ID
	exposed := fmt.Sprintf("s%d_lookup", sid)
	a := newAgent(t, f, store.Agent{Name: "Picky", Instructions: "x"})

	run := func() llmRequest {
		t.Helper()
		id, err := f.eng.RunAgent(a, "go")
		if err != nil {
			t.Fatal(err)
		}
		waitRun(t, f.st, id)
		all := seen()
		return all[len(all)-1]
	}
	if run().hasTool(exposed) {
		t.Fatal("agent got a server it was never given")
	}
	f.st.SetAgentMCP(a.ID, f.owner, []store.AgentTool{{ServerID: sid, Tools: "something_else"}})
	if run().hasTool(exposed) {
		t.Fatal("agent got a tool outside its allowlist")
	}
	f.st.SetAgentMCP(a.ID, f.owner, []store.AgentTool{{ServerID: sid, Tools: "lookup, other"}})
	if !run().hasTool(exposed) {
		t.Fatal("allowlisted tool was not offered")
	}
	f.st.SetAgentMCP(a.ID, f.owner, []store.AgentTool{{ServerID: sid}})
	if !run().hasTool(exposed) {
		t.Fatal("an empty allowlist should allow every tool of the server")
	}
}

// A server added to one agent is used by that agent only: not by the owner's
// chats and not by their other agents.
func TestAgentOwnServerIsPrivate(t *testing.T) {
	llmSrv, seen := scriptLLM(t, func(llmRequest) (string, string, string) { return "ok", "", "" })
	f := setup(t, llmSrv.URL)
	mcpSrv := fakeMCP(t)
	owner := newAgent(t, f, store.Agent{Name: "Owner", Instructions: "x"})
	other := newAgent(t, f, store.Agent{Name: "Other", Instructions: "x"})
	if err := f.st.CreateAgentMCPServer(owner, "private", mcpSrv.URL, "", "auto"); err != nil {
		t.Fatal(err)
	}
	own, _ := f.st.ListAgentOwnServers(owner.ID)
	if len(own) != 1 {
		t.Fatalf("own servers = %+v", own)
	}
	exposed := fmt.Sprintf("s%d_lookup", own[0].ID)
	last := func() llmRequest { all := seen(); return all[len(all)-1] }

	id, _ := f.eng.RunAgent(owner, "go")
	waitRun(t, f.st, id)
	if !last().hasTool(exposed) {
		t.Fatal("the agent was not offered its own server's tool")
	}

	// Ticking it for another agent is ignored: it is not a shared server.
	f.st.SetAgentMCP(other.ID, f.owner, []store.AgentTool{{ServerID: own[0].ID}})
	id, _ = f.eng.RunAgent(other, "go")
	waitRun(t, f.st, id)
	if last().hasTool(exposed) {
		t.Fatal("another agent got a private server")
	}

	// A solo chat uses all of the owner's shared servers, but not this one.
	chat, _ := f.st.CreateThread(f.owner, f.modelID)
	id, _ = f.eng.Submit(chat, f.owner, "hi", nil)
	waitRun(t, f.st, id)
	if last().hasTool(exposed) {
		t.Fatal("a normal chat got an agent's private server")
	}
	if shared, _ := f.st.ListMCPServers(f.owner); len(shared) != 0 {
		t.Fatalf("private server listed among the shared ones: %+v", shared)
	}
}
