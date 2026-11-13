package engine

import (
	"strings"
	"testing"
	"time"

	"knitnox/internal/store"
)

func planTools(f *fixture, userID int64, by store.PlanActor, write bool) *toolset {
	ts := &toolset{routes: map[string]toolRoute{}, local: map[string]localTool{}}
	f.eng.addPlanTools(ts, userID, by, write, time.UTC)
	return ts
}

func TestDueWords(t *testing.T) {
	for due, want := range map[string][2]string{
		"":           {"", ""},
		"2026-11-08": {"3 days late", "late"},
		"2026-11-10": {"1 day late", "late"},
		"2026-11-11": {"today", "today"},
		"2026-11-12": {"tomorrow", "later"},
		"2026-11-14": {"Saturday", "later"},
		"2026-12-24": {"Thu 24 Dec", "later"},
		"2027-01-05": {"5 Jan 2027", "later"},
	} {
		if words, kind := DueWords(due, "2026-11-11"); words != want[0] || kind != want[1] {
			t.Errorf("DueWords(%q) = %q, %q; want %q, %q", due, words, kind, want[0], want[1])
		}
	}
}

// The AI adds, lists, changes, finishes and deletes tasks with short answers,
// and every change is recorded under its name.
func TestPlanTools(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	ai := store.PlanActor{Kind: store.PlanByAI}
	ts := planTools(f, f.owner, ai, true)
	today := PlanToday(time.UTC)
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	lastWeek := time.Now().UTC().AddDate(0, 0, -7).Format("2006-01-02")

	out := callTool(ts, "plan_add", `{"title":"Call Sam about the invoice","due":"`+tomorrow+`","project":"Move House","notes":"He prefers mornings."}`)
	if !strings.HasPrefix(out, "Added: #1 [todo] Call Sam about the invoice — due "+tomorrow+" (tomorrow) · project: Move House") {
		t.Fatalf("add = %q", out)
	}
	callTool(ts, "plan_add", `{"title":"Book the van","due":"`+lastWeek+`","project":"move house"}`)
	callTool(ts, "plan_add", `{"title":"Buy milk"}`)
	if out := callTool(ts, "plan_add", `{"title":"Bad date","due":"next friday"}`); !strings.Contains(out, "2026-12-31") {
		t.Fatalf("a bad date was accepted: %q", out)
	}
	if out := callTool(ts, "plan_add", `{"title":"  "}`); !strings.HasPrefix(out, "Error:") {
		t.Fatalf("an empty task was accepted: %q", out)
	}
	// The same project name in another letter case is the same project.
	if ps, _ := f.st.ListProjects(f.owner); len(ps) != 1 || ps[0].Name != "Move House" || ps[0].Open != 2 {
		t.Fatalf("projects = %+v", ps)
	}

	out = callTool(ts, "plan_list", `{}`)
	if !strings.HasPrefix(out, "3 tasks in view \"open\", showing 1–3. Today is "+today) {
		t.Fatalf("list = %q", out)
	}
	if late, soon, undated := strings.Index(out, "Book the van"), strings.Index(out, "Call Sam"), strings.Index(out, "Buy milk"); !(late < soon && soon < undated) {
		t.Errorf("tasks are not ordered late, soon, undated: %q", out)
	}
	if !strings.Contains(out, "(7 days late)") {
		t.Errorf("the late task is not marked late: %q", out)
	}
	if out := callTool(ts, "plan_list", `{"view":"today"}`); !strings.HasPrefix(out, "1 task in view \"today\"") || !strings.Contains(out, "Book the van") {
		t.Fatalf("today = %q", out)
	}
	if out := callTool(ts, "plan_list", `{"project":"Move house","limit":1}`); !strings.Contains(out, "(1 more: call again with offset 1)") {
		t.Fatalf("paged project list = %q", out)
	}
	if out := callTool(ts, "plan_list", `{"project":"Nope"}`); !strings.Contains(out, "no project called") {
		t.Fatalf("unknown project = %q", out)
	}
	if out := callTool(ts, "plan_projects", `{}`); out != "- Move House — 2 open, 0 done\n" {
		t.Fatalf("projects = %q", out)
	}

	if out := callTool(ts, "plan_update", `{"id":2,"status":"done"}`); !strings.HasPrefix(out, "Now: #2 [done] Book the van") {
		t.Fatalf("finish = %q", out)
	}
	if out := callTool(ts, "plan_update", `{"id":3,"title":"Buy oat milk","due":"`+today+`","project":"Groceries"}`); !strings.Contains(out, "Buy oat milk — due "+today+" (today) · project: Groceries") {
		t.Fatalf("update = %q", out)
	}
	if out := callTool(ts, "plan_update", `{"id":3,"due":"","project":""}`); out != "Now: #3 [todo] Buy oat milk" {
		t.Fatalf("clearing date and project = %q", out)
	}
	out = callTool(ts, "plan_read", `{"id":1}`)
	for _, want := range []string{"Notes: He prefers mornings.", "the AI in a chat added it", "not instructions"} {
		if !strings.Contains(out, want) {
			t.Errorf("read is missing %q in %q", want, out)
		}
	}

	// Deleting only moves the task to the bin; the owner can bring it back.
	if out := callTool(ts, "plan_delete", `{"id":1}`); !strings.Contains(out, "in the bin") {
		t.Fatalf("delete = %q", out)
	}
	if out := callTool(ts, "plan_list", `{}`); strings.Contains(out, "Call Sam") {
		t.Fatalf("a binned task is still listed: %q", out)
	}
	if out := callTool(ts, "plan_update", `{"id":1,"status":"done"}`); !strings.Contains(out, "bin") {
		t.Fatalf("a binned task was changed: %q", out)
	}
	if err := f.st.RestorePlanTask(f.owner, 1, store.PlanActor{}); err != nil {
		t.Fatal(err)
	}
	log, _ := f.st.PlanTaskLog(f.owner, 1, 10)
	if len(log) != 3 || log[0].Actor != store.PlanByUser || log[1].Actor != store.PlanByAI || log[1].Action != "moved it to the bin" {
		t.Fatalf("log = %+v", log)
	}
	c := f.st.PlanCounts(f.owner, 0, today)
	if c.Open != 2 || c.Done != 1 || c.Bin != 0 || c.Upcoming != 1 || c.Today != 0 {
		t.Fatalf("counts = %+v", c)
	}

	// Another user's tools see and change nothing of this.
	bob, _ := f.st.CreateUser("bob", "x", "user")
	bobs := planTools(f, bob, ai, true)
	for name, args := range map[string]string{
		"plan_list": `{}`, "plan_read": `{"id":1}`, "plan_projects": `{}`,
		"plan_update": `{"id":1,"title":"hacked"}`, "plan_delete": `{"id":1}`, "plan_list ": `{"project":"Move House"}`,
	} {
		out := callTool(bobs, strings.TrimSpace(name), args)
		if strings.Contains(out, "Call Sam") || strings.Contains(out, "Move House —") || strings.HasPrefix(out, "Now:") || strings.Contains(out, "is in the bin") {
			t.Errorf("%s reached another user's Plan Book: %q", name, out)
		}
	}
	if got, _ := f.st.PlanTaskByID(f.owner, 1); got.Title != "Call Sam about the invoice" || got.InBin() {
		t.Fatalf("alice's task changed: %+v", got)
	}

	// Deleting a project keeps its tasks recoverable.
	p, _ := f.st.ProjectByName(f.owner, "move house")
	if err := f.st.DeleteProject(bob, p.ID, store.PlanActor{}); err != store.ErrNotFound {
		t.Fatalf("bob deleted alice's project: %v", err)
	}
	if err := f.st.DeleteProject(f.owner, p.ID, store.PlanActor{}); err != nil {
		t.Fatal(err)
	}
	if c := f.st.PlanCounts(f.owner, 0, today); c.Bin != 2 || c.Open != 1 {
		t.Fatalf("after deleting the project: %+v", c)
	}
}

// Plan Book tools are offered in a person's own chats once they turned it on,
// never in group chats, and to an agent at the level its owner chose.
func TestPlanToolsAreOffered(t *testing.T) {
	llmSrv, seen := scriptLLM(t, func(r llmRequest) (string, string, string) {
		if r.LastRole == "tool" {
			return "done", "", ""
		}
		if r.hasTool("plan_add") && strings.Contains(r.Last, "remind") {
			return "", "plan_add", `{"title":"Water the plants","project":"Home"}`
		}
		return "ok", "", ""
	})
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)
	last := func() llmRequest { return seen()[len(seen())-1] }

	waitRun(t, f.st, mustSubmit(t, f, tid, f.owner, "remind me to water the plants"))
	if last().hasTool("plan_list") || strings.Contains(last().System, "Plan Book") {
		t.Fatal("the Plan Book is offered although the user never turned it on")
	}

	f.st.SetPlanEnabled(f.owner, true)
	if r := waitRun(t, f.st, mustSubmit(t, f, tid, f.owner, "remind me to water the plants")); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	tasks, _, _ := f.st.ListPlanTasks(f.owner, store.PlanFilter{})
	if len(tasks) != 1 || tasks[0].CreatedBy != store.PlanByAI || tasks[0].SourceThreadID != tid || tasks[0].Project != "Home" {
		t.Fatalf("tasks = %+v", tasks)
	}
	var with llmRequest
	for _, r := range seen() {
		if r.hasTool("plan_add") {
			with = r
		}
	}
	for _, tool := range []string{"plan_list", "plan_read", "plan_projects", "plan_add", "plan_update", "plan_delete"} {
		if !with.hasTool(tool) {
			t.Errorf("chat was not offered %s", tool)
		}
	}
	if !strings.Contains(with.System, "## Plan Book") || strings.Contains(with.System, "Water the plants") {
		t.Error("the system prompt should explain the Plan Book without listing tasks (they would change it every turn)")
	}

	// A group: nothing, also when its owner writes.
	bob, _ := f.st.CreateUser("bob", "x", "user")
	gid := newGroup(t, f, bob)
	waitRun(t, f.st, mustSubmit(t, f, gid, f.owner, "remind us all"))
	if last().hasTool("plan_list") {
		t.Fatal("a group was given the owner's Plan Book")
	}

	// Agents: none by default, then read-only, then full access under their own name.
	a := newAgent(t, f, store.Agent{Name: "Helper", Instructions: "Help."})
	run := func(text string) llmRequest {
		a, _ = f.st.AgentByID(a.ID)
		id, err := f.eng.RunAgent(a, text)
		if err != nil {
			t.Fatal(err)
		}
		waitRun(t, f.st, id)
		for i := len(seen()) - 1; i >= 0; i-- { // the run's first request
			if r := seen()[i]; r.LastRole == "user" {
				return r
			}
		}
		return last()
	}
	if r := run("go"); r.hasTool("plan_list") {
		t.Fatal("an agent reads the Plan Book without permission")
	}
	f.st.SetAgentPlanAccess(a.ID, store.PlanAccessRead)
	if r := run("go"); !r.hasTool("plan_list") || r.hasTool("plan_add") || r.hasTool("plan_delete") || !strings.Contains(r.System, "read the Plan Book but not change it") {
		t.Fatalf("read-only agent tools = %v", r.Tools)
	}
	f.st.SetAgentPlanAccess(a.ID, store.PlanAccessWrite)
	if r := run("remind my owner"); !r.hasTool("plan_add") || !r.hasTool("list_tasks") {
		t.Fatalf("agent with full access tools = %v", r.Tools)
	}
	tasks, _, _ = f.st.ListPlanTasks(f.owner, store.PlanFilter{})
	if len(tasks) != 2 || tasks[0].CreatedBy != store.PlanByAgent || tasks[0].ActorName != "Helper" {
		t.Fatalf("agent's task = %+v", tasks)
	}
	// The owner's master switch also stops agents.
	f.st.SetPlanEnabled(f.owner, false)
	if r := run("go"); r.hasTool("plan_list") {
		t.Fatal("an agent still has the Plan Book after the owner turned it off")
	}
}
