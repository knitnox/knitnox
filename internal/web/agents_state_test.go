package web

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// The sidebar and the agents page say what each agent is doing: working now,
// stopped by today's limit of runs, broken, paused or simply active.
func TestAgentStates(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")

	mk := func(name string, schedule, daily int, enabled bool) *store.Agent {
		ag := &store.Agent{UserID: alice.id, Name: name, Instructions: "x", MaxSteps: 5, DailyRuns: daily, HistoryRuns: 3,
			ScheduleMinutes: schedule, Enabled: enabled}
		if _, err := a.st.CreateAgent(ag, 0); err != nil {
			t.Fatal(err)
		}
		return ag
	}
	addRun := func(ag *store.Agent, status string) {
		if _, err := a.st.DB.Exec(`INSERT INTO runs(thread_id, status) VALUES (?, ?)`, ag.ThreadID, status); err != nil {
			t.Fatal(err)
		}
	}
	calm := mk("Calm", 60, 48, true)
	busy := mk("Busy", 0, 48, true)
	capped := mk("Capped", 15, 2, true)
	broken := mk("Broken", 0, 48, true)
	paused := mk("Paused one", 60, 48, false)
	addRun(calm, store.RunDone)
	addRun(busy, store.RunRunning)
	addRun(capped, store.RunDone)
	addRun(capped, store.RunDone)
	addRun(broken, store.RunFailed)
	addRun(paused, store.RunFailed) // a paused agent is not nagged about

	var pulse struct {
		Working, Trouble int
		Agents           map[string]struct{ State, Label string }
	}
	if err := json.Unmarshal([]byte(alice.get("/agents/pulse")), &pulse); err != nil {
		t.Fatal(err)
	}
	if pulse.Working != 1 || pulse.Trouble != 2 {
		t.Fatalf("pulse: %d working, %d need a look; want 1 and 2", pulse.Working, pulse.Trouble)
	}
	for ag, want := range map[*store.Agent]string{calm: store.AgentOK, busy: store.AgentBusy, capped: store.AgentWarn, broken: store.AgentWarn, paused: store.AgentOff} {
		if got := pulse.Agents[fmt.Sprint(ag.ID)].State; got != want {
			t.Errorf("%s is %q, want %q", ag.Name, got, want)
		}
	}

	page := alice.get("/agents")
	for _, want := range []string{
		`<b>4</b> active`, `<b>1</b> working now`, `<b>1</b> paused`, `<b>2</b> need a look`,
		"Daily limit reached", "Last run failed", "2 of 2 runs today",
		`title="2 agents need a look">2</span>`, `title="1 agent working now">`, // next to "Agents" in the sidebar
		fmt.Sprintf(`class="status-dot is-warn" data-agent-dot="%d" title="Reached today&#39;s limit of runs"`, capped.ID),
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the agents page is missing %s", want)
		}
	}
	if own := alice.get(fmt.Sprintf("/t/%d", capped.ThreadID)); !strings.Contains(own, "Daily limit reached") {
		t.Error("the agent's own page does not say that today's limit is used up")
	}
	if own := alice.get(fmt.Sprintf("/t/%d", calm.ThreadID)); strings.Contains(own, "Daily limit reached") {
		t.Error("an agent within its limit is shown as stopped")
	}

	// A broken run and a run that could not reach a tool server are both said out
	// loud: at the top of the agent's page, under the run, and on the list.
	if own := alice.get(fmt.Sprintf("/t/%d", broken.ThreadID)); !strings.Contains(own, "The last run failed") || !strings.Contains(own, "Run again") {
		t.Error("the page of an agent whose run failed does not say so at the top")
	}
	if own := alice.get(fmt.Sprintf("/t/%d", calm.ThreadID)); strings.Contains(own, "agent-alert") {
		t.Error("a healthy agent's page shows a problem note")
	}
	res, err := a.st.DB.Exec(`INSERT INTO runs(thread_id, status, tools_down) VALUES (?, 'done', 'crm')`, calm.ThreadID)
	if err != nil {
		t.Fatal(err)
	}
	runID, _ := res.LastInsertId()
	if _, err := a.st.DB.Exec(`INSERT INTO messages(thread_id, run_id, role, content) VALUES (?, ?, 'assistant', 'Done, as far as I could.')`, calm.ThreadID, runID); err != nil {
		t.Fatal(err)
	}
	own := alice.get(fmt.Sprintf("/t/%d", calm.ThreadID))
	if !strings.Contains(own, "The last run was missing tools") || strings.Count(own, "Could not reach the tool server crm, so this run worked without its tools.") != 2 {
		t.Error("an unreachable tool server is not shown at the top of the page and under the run")
	}
	if list := alice.get("/agents"); !strings.Contains(list, "Tool server unreachable") || !strings.Contains(list, `<b>3</b> need a look`) {
		t.Error("the list does not flag the agent whose tool server was unreachable")
	}

	// Knitnox's own posts about an agent are marked as such in the feed.
	by := store.FeedActor{Kind: store.FeedByAgent, Name: "Calm", AgentID: calm.ID, ThreadID: calm.ThreadID, Notice: store.FeedNoticeProblem}
	if _, err := a.st.CreateFeedPost(alice.id, store.FeedPostInput{Body: "Could not reach the tool server crm."}, by); err != nil {
		t.Fatal(err)
	}
	if feed := alice.get("/feed"); !strings.Contains(feed, "Problem</span>") || !strings.Contains(feed, "Could not reach the tool server crm.") {
		t.Error("the feed does not mark the problem post")
	}

	// Somebody else sees none of it.
	bob := a.register("bob")
	if got := bob.get("/agents/pulse"); !strings.Contains(got, `"working":0`) || !strings.Contains(got, `"agents":{}`) {
		t.Errorf("bob's pulse = %s", got)
	}
}
