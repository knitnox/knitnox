package store

import (
	"sync"
	"testing"
)

func count(t *testing.T, st *Store, q string, args ...any) int {
	t.Helper()
	var n int
	if err := st.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Old usage and audit rows go; recent ones, and everything when the limit is 0, stay.
func TestPruneUsageAndAudit(t *testing.T) {
	st, _ := testStore(t)
	u, _ := st.CreateUser("alice", "h", "admin")
	for _, when := range []string{"2020-01-01 00:00:00", "2021-06-01 00:00:00"} {
		st.DB.Exec(`INSERT INTO usage_log(user_id, model_name, kind, created_at) VALUES (?, 'm', 'chat', ?)`, u, when)
		st.DB.Exec(`INSERT INTO audit_log(actor_id, actor_name, action, target, details, created_at) VALUES (?, 'alice', 'x', '', '', ?)`, u, when)
	}
	st.RecordUsage(UsageEntry{UserID: u, ModelName: "m", Kind: "chat", PromptTokens: 5, Billable: true})
	st.Audit(u, "alice", "today", "", "")

	if n, err := st.PruneUsage(0); err != nil || n != 0 {
		t.Fatalf("limit 0 removed %d (%v)", n, err)
	}
	if n, err := st.PruneUsage(365); err != nil || n != 2 {
		t.Fatalf("usage: removed %d (%v), want 2", n, err)
	}
	if n, err := st.PruneAudit(365); err != nil || n != 2 {
		t.Fatalf("audit: removed %d (%v), want 2", n, err)
	}
	if got := count(t, st, `SELECT COUNT(*) FROM usage_log`); got != 1 {
		t.Fatalf("%d usage rows left", got)
	}
	if used, _ := st.UsedTokens(u, "monthly"); used != 5 {
		t.Fatalf("this month's use changed: %d", used)
	}
	if list, _ := st.ListAudit(10); len(list) != 1 || list[0].Action != "today" {
		t.Fatalf("audit left: %+v", list)
	}
}

// Only an agent's old finished runs go. Chats keep everything.
func TestPruneAgentRuns(t *testing.T) {
	st, _ := testStore(t)
	u, _ := st.CreateUser("alice", "h", "admin")
	agent := Agent{UserID: u, Name: "Watcher"}
	if _, err := st.CreateAgent(&agent, 0); err != nil {
		t.Fatal(err)
	}
	a, err := st.ListAgents(u)
	if err != nil || len(a) != 1 {
		t.Fatalf("agents: %v %v", a, err)
	}
	agentThread := a[0].ThreadID
	chat, _ := st.CreateThread(u, 0)

	finish := func(thread int64, text string) int64 {
		id, err := st.EnqueueMessage(thread, u, text, 100, nil)
		if err != nil {
			t.Fatal(err)
		}
		st.CreateMessage(&Message{ThreadID: thread, RunID: id, Role: "assistant", Content: "done " + text})
		st.FinishRun(id, RunDone, "", 1)
		return id
	}
	var agentRuns []int64
	for i := 0; i < 6; i++ {
		agentRuns = append(agentRuns, finish(agentThread, "agent"))
		finish(chat, "chat")
	}
	queued, _ := st.EnqueueMessage(agentThread, u, "still waiting", 100, nil) // pending: never removed

	n, err := st.PruneAgentRuns(2)
	if err != nil || n != 4 {
		t.Fatalf("removed %d (%v), want 4", n, err)
	}
	// The two newest finished runs stay, and so does the one still waiting.
	for _, id := range []int64{agentRuns[4], agentRuns[5], queued} {
		if _, err := st.RunByID(id); err != nil {
			t.Fatalf("run %d was removed: %v", id, err)
		}
	}
	if _, err := st.RunByID(agentRuns[0]); err != ErrNotFound {
		t.Fatalf("the oldest agent run is still there: %v", err)
	}
	if got := count(t, st, `SELECT COUNT(*) FROM messages WHERE run_id = ?`, agentRuns[0]); got != 0 {
		t.Fatalf("%d messages of a removed run are left", got)
	}
	if got := count(t, st, `SELECT COUNT(*) FROM runs WHERE thread_id = ?`, chat); got != 6 {
		t.Fatalf("the chat has %d runs left, want all 6", got)
	}
	if n, _ := st.PruneAgentRuns(0); n != 0 {
		t.Fatalf("limit 0 removed %d", n)
	}
	if err := st.Optimize(); err != nil {
		t.Fatal(err)
	}
}

// However many people sign up at the same moment, exactly one is the admin.
func TestOnlyTheFirstSignupIsAdmin(t *testing.T) {
	st, _ := testStore(t)
	var wg sync.WaitGroup
	roles := make([]string, 12)
	for i := range roles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, role, err := st.CreateSignupUser("user"+string(rune('a'+i)), "h")
			if err != nil {
				t.Error(err)
			}
			roles[i] = role
		}()
	}
	wg.Wait()
	admins := 0
	for _, r := range roles {
		if r == "admin" {
			admins++
		}
	}
	if admins != 1 {
		t.Fatalf("%d admins among %v", admins, roles)
	}
}
