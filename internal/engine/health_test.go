package engine

import (
	"strings"
	"testing"
	"time"

	"knitnox/internal/store"
)

// A tool server that cannot be reached no longer goes unnoticed: the run says
// so, the agent counts as "needs a look", and the owner's feed gets one post
// when the trouble starts and one when it is over, however many runs lie between.
func TestAgentHealthNotes(t *testing.T) {
	llm, _ := fakeLLM(t, 0)
	f := setup(t, llm.URL)
	a := newAgent(t, f, store.Agent{Name: "Helper", Instructions: "Help."})

	mcpSrv := fakeMCP(t)
	if err := f.st.CreateMCPServer(f.owner, "crm", mcpSrv.URL, "", store.ApprovalAuto); err != nil {
		t.Fatal(err)
	}
	servers, _ := f.st.ListMCPServers(f.owner)
	if err := f.st.SetAgentMCP(a.ID, f.owner, []store.AgentTool{{ServerID: servers[0].ID}}); err != nil {
		t.Fatal(err)
	}

	run := func() int64 {
		t.Helper()
		id, err := f.eng.RunAgent(a, "go")
		if err != nil {
			t.Fatal(err)
		}
		if r := waitRun(t, f.st, id); r.Status != store.RunDone {
			t.Fatalf("run ended as %s: %s", r.Status, r.Error)
		}
		return id
	}
	// The note is written just after the run is marked finished, so give it a moment.
	notes := func(want int) []store.FeedPost {
		t.Helper()
		var posts []store.FeedPost
		for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			posts, _, _ = f.st.ListFeed(f.owner, store.FeedFilter{View: store.FeedViewMine})
			if len(posts) == want || time.Now().After(deadline) {
				break
			}
		}
		if len(posts) != want {
			t.Fatalf("feed has %d posts, want %d: %+v", len(posts), want, posts)
		}
		return posts
	}
	state := func() store.AgentRow {
		rows, _ := f.st.ListAgents(f.owner)
		return rows[0]
	}

	run() // all is well: nothing is said
	notes(0)
	if s := state(); s.State() != store.AgentOK {
		t.Fatalf("a healthy agent is %q", s.State())
	}

	mcpSrv.Close() // the tool server goes down
	id := run()
	posts := notes(1)
	if p := posts[0]; p.Notice != store.FeedNoticeProblem || p.ActorName != "Helper" || p.AgentID != a.ID || p.Public ||
		!strings.Contains(p.Body, "Could not reach the tool server crm") {
		t.Fatalf("problem post = %+v", p)
	}
	if out, _ := f.st.LastRunOutcome(a.ThreadID, 0); out == nil || out.ID != id || out.ToolsDown != "crm" || !out.Bad() {
		t.Fatalf("the run does not record the unreachable server: %+v", out)
	}
	if msgs, _ := f.st.RunMessages(id); len(msgs) == 0 || msgs[0].RunDown != "crm" {
		t.Error("the run's messages do not carry the unreachable server")
	}
	if s := state(); s.State() != store.AgentWarn || s.StateLabel() != "Could not reach a tool server" {
		t.Fatalf("agent with a server down is %q (%s)", s.State(), s.StateLabel())
	}

	run() // still down: no second post
	run()
	notes(1)

	// These notes are Knitnox's, not the agent's: they do not use up its posts per day.
	if n := f.st.FeedAgentPostsSince(a.ID, "2000-01-01 00:00:00"); n != 0 {
		t.Errorf("health notes count as %d of the agent's own posts", n)
	}

	f.st.SetAgentMCP(a.ID, f.owner, nil) // the owner unticks the broken server: the next run is fine
	run()
	posts = notes(2)
	if p := posts[0]; p.Notice != store.FeedNoticeRecovered || !strings.Contains(p.Body, "Working again") {
		t.Fatalf("recovery post = %+v", p)
	}
	if s := state(); s.State() != store.AgentOK {
		t.Fatalf("a recovered agent is %q", s.State())
	}
	run() // and stays quiet while things work
	notes(2)
}

// A run that breaks is reported the same way, once.
func TestAgentHealthAfterFailedRuns(t *testing.T) {
	llm, _ := fakeLLM(t, 0)
	f := setup(t, llm.URL)
	a := newAgent(t, f, store.Agent{Name: "Helper", Instructions: "Help."})
	llm.Close() // the model cannot be reached: every run fails

	for range 2 {
		id, err := f.eng.RunAgent(a, "go")
		if err != nil {
			t.Fatal(err)
		}
		if r := waitRun(t, f.st, id); r.Status != store.RunFailed {
			t.Fatalf("run ended as %s", r.Status)
		}
	}
	var posts []store.FeedPost
	for deadline := time.Now().Add(2 * time.Second); len(posts) == 0 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		posts, _, _ = f.st.ListFeed(f.owner, store.FeedFilter{View: store.FeedViewMine})
	}
	time.Sleep(50 * time.Millisecond) // a second post would arrive now
	posts, _, _ = f.st.ListFeed(f.owner, store.FeedFilter{View: store.FeedViewMine})
	if len(posts) != 1 || posts[0].Notice != store.FeedNoticeProblem || !strings.Contains(posts[0].Body, "The last run could not finish") {
		t.Fatalf("posts after two failed runs = %+v", posts)
	}
}
