package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"knitnox/internal/store"
)

// hx posts a form the way htmx does and returns the fragment that comes back.
func (c *client) hx(path string, form url.Values) (int, string) {
	req, _ := http.NewRequest(http.MethodPost, c.a.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.Header.Set("HX-Request", "true")
	r, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b)
}

// The Plan Book page: add, group, tick off, change, delete and restore tasks;
// nobody can see or touch another person's tasks.
func TestPlanBookPagesAndPermissions(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	page := alice.get("/plan")
	for _, want := range []string{`href="/plan" class="sb-link is-active" aria-current="page"`, "The AI cannot use your Plan Book yet",
		"Your Plan Book is empty", "How does the Plan Book work?", `id="plan-main"`, `name="title"`} {
		if !strings.Contains(page, want) {
			t.Errorf("empty Plan Book page is missing %q", want)
		}
	}
	if code, _, h := alice.post("/plan/settings", url.Values{"enabled": {"1"}}); code != http.StatusSeeOther || h.Get("Location") != "/plan?done=on" {
		t.Fatalf("turn on: %d %s", code, h.Get("Location"))
	}
	if !a.st.PlanEnabled(alice.id) || a.st.PlanEnabled(bob.id) {
		t.Fatal("the switch did not change exactly alice's setting")
	}
	// The Settings tab has the same switch and stays on that tab; the Projects tab starts empty.
	if p := alice.get("/plan/settings"); !strings.Contains(p, "Who may use my Plan Book") || !strings.Contains(p, "How the Plan Book works") ||
		!strings.Contains(p, `href="/plan/settings" class="is-active" aria-current="page"`) || strings.Contains(p, "The AI cannot use your Plan Book yet") {
		t.Error("the Settings tab is wrong")
	}
	if code, _, h := alice.post("/plan/settings", url.Values{"enabled": {"1"}, "from": {"settings"}}); code != http.StatusSeeOther || h.Get("Location") != "/plan/settings?done=on" {
		t.Errorf("turn on from the Settings tab: %d %s", code, h.Get("Location"))
	}
	if p := alice.get("/plan/projects"); !strings.Contains(p, "No projects yet") || !strings.Contains(p, `action="/plan/projects"`) {
		t.Error("the empty Projects tab is wrong")
	}

	// A project, then tasks in and outside it. Each change answers with the fresh list.
	code, _, h := alice.post("/plan/projects", url.Values{"name": {"Kitchen refit"}, "notes": {"By March."}})
	if code != http.StatusSeeOther || !strings.HasPrefix(h.Get("Location"), "/plan?project=") {
		t.Fatalf("create project: %d %s", code, h.Get("Location"))
	}
	projects, _ := a.st.ListProjects(alice.id)
	pid := fmt.Sprint(projects[0].ID)
	code, list := alice.hx("/plan/tasks", url.Values{"title": {"Book the van"}, "due": {yesterday}, "project_id": {pid}, "project": {pid}})
	if code != http.StatusOK || !strings.Contains(list, "Book the van") || !strings.Contains(list, "1 day late") || !strings.Contains(list, `class="plan-group is-late"`) {
		t.Fatalf("create in project: %d %s", code, list)
	}
	// The answer carries the row of controls too, to be swapped in place (fresh counts); the full page does not mark it so.
	if !strings.Contains(list, `id="plan-bar" hx-swap-oob="true"`) || strings.Contains(alice.get("/plan"), "hx-swap-oob") {
		t.Error("the row of controls is not sent along with the list, or the full page marks it out-of-band")
	}
	alice.hx("/plan/tasks", url.Values{"title": {"Buy milk"}, "due": {today}, "notes": {"Oat."}})
	if code, _ := alice.hx("/plan/tasks", url.Values{"title": {" "}}); code != http.StatusBadRequest {
		t.Errorf("an empty task was accepted: %d", code)
	}
	if code, _ := alice.hx("/plan/tasks", url.Values{"title": {"x"}, "due": {"soon"}}); code != http.StatusBadRequest {
		t.Errorf("a bad date was accepted: %d", code)
	}

	page = alice.get("/plan")
	for _, want := range []string{"Book the van", "Buy milk", "Due today", ">Kitchen refit<", "Today <span", `title="2 task`} {
		if !strings.Contains(page, want) {
			t.Errorf("task list is missing %q", want)
		}
	}
	if p := alice.get("/plan?project=" + pid); !strings.Contains(p, "By March.") || !strings.Contains(p, "0 of 1 done") || strings.Contains(p, "Buy milk") {
		t.Error("the project view is wrong")
	}
	if p := alice.get("/plan/projects"); !strings.Contains(p, `href="/plan?project=`+pid+`"`) || !strings.Contains(p, "1 to do · 0 done") || strings.Contains(p, "No projects yet") {
		t.Error("the Projects tab does not list the project with its progress")
	}
	if p := bob.get("/plan/projects"); strings.Contains(p, "Kitchen refit") {
		t.Error("bob can see alice's projects")
	}
	if p := alice.get("/plan?project=-1"); !strings.Contains(p, "Buy milk") || strings.Contains(p, "Book the van") {
		t.Error("the “No project” view is wrong")
	}
	if p := alice.get("/plan?q=milk"); !strings.Contains(p, "Buy milk") || strings.Contains(p, "Book the van") {
		t.Error("finding a task by word is wrong")
	}

	tasks, _, _ := a.st.ListPlanTasks(alice.id, store.PlanFilter{})
	van, milk := tasks[0], tasks[1]
	if van.Title != "Book the van" {
		t.Fatalf("order = %+v", tasks)
	}
	item := fmt.Sprintf("/plan/tasks/%d", van.ID)

	// Bob sees none of it and cannot reach it by id.
	if p := bob.get("/plan"); strings.Contains(p, "Book the van") || strings.Contains(p, "Kitchen refit") {
		t.Fatal("bob can see alice's Plan Book")
	}
	for _, path := range []string{item, item + "/edit"} {
		if code := bob.getStatus(path); code != http.StatusNotFound {
			t.Errorf("bob GET %s = %d, want 404", path, code)
		}
	}
	for _, path := range []string{item, item + "/status", item + "/delete", item + "/restore", item + "/purge", "/plan/projects/" + pid, "/plan/projects/" + pid + "/delete"} {
		if code, _, _ := bob.post(path, url.Values{"title": {"hacked"}, "status": {"done"}, "name": {"hacked"}}); code != http.StatusNotFound {
			t.Errorf("bob POST %s = %d, want 404", path, code)
		}
	}
	if code, _, _ := bob.post("/plan/tasks", url.Values{"title": {"sneak"}, "project_id": {pid}}); code != http.StatusNotFound {
		t.Errorf("bob added a task to alice's project: %d", code)
	}
	if got, _ := a.st.PlanTaskByID(alice.id, van.ID); got.Title != "Book the van" || got.Status != store.PlanTodo || got.InBin() {
		t.Fatalf("alice's task changed: %+v", got)
	}

	// Start it, change it in place, tick it off.
	if _, list := alice.hx(item+"/status", url.Values{"status": {"doing"}}); !strings.Contains(list, "In progress") {
		t.Error("starting a task does not show it as in progress")
	}
	form := alice.get(item + "/edit")
	for _, want := range []string{`hx-post="` + item + `"`, `value="Book the van"`, `value="` + yesterday + `"`, "History: who changed this task", "started it"} {
		if !strings.Contains(form, want) {
			t.Errorf("edit form is missing %q", want)
		}
	}
	code, list = alice.hx(item, url.Values{"title": {"Book a big van"}, "due": {""}, "project_id": {"0"}, "status": {"doing"}, "notes": {"Ask Sam."}})
	if code != http.StatusOK || !strings.Contains(list, "Book a big van") || strings.Contains(list, "day late") || !strings.Contains(list, "No date") {
		t.Fatalf("update: %d %s", code, list)
	}
	card := fmt.Sprintf(`id="pt-%d"`, van.ID)
	_, list = alice.hx(item+"/status", url.Values{"status": {"done"}})
	if strings.Contains(list, card) {
		t.Error("a finished task is still in the to-do list")
	}
	// Ticking off says what happened and offers the way back.
	if !strings.Contains(list, "Done: “Book a big van”") || !strings.Contains(list, `hx-post="`+item+`/status"`) || !strings.Contains(list, `name="status" value="doing"`) {
		t.Errorf("ticking off does not offer Undo: %s", list)
	}
	if _, list := alice.hx(item+"/status", url.Values{"status": {"doing"}}); !strings.Contains(list, card) || strings.Contains(list, "plan-undo") {
		t.Error("Undo did not bring the task back as it was")
	}
	alice.hx(item+"/status", url.Values{"status": {"done"}})
	if code, _ := alice.hx(item+"/status", url.Values{"status": {"nonsense"}}); code != http.StatusBadRequest {
		t.Errorf("an unknown status was accepted: %d", code)
	}
	if !strings.Contains(alice.get("/plan?view=done"), "Book a big van") {
		t.Error("the done view does not list the finished task")
	}

	// Delete goes to the bin; restore brings it back; purge is final.
	bin := url.Values{"view": {"bin"}}
	if _, list := alice.hx(fmt.Sprintf("/plan/tasks/%d/delete", milk.ID), bin); !strings.Contains(list, "Buy milk") || !strings.Contains(list, "Empty the bin") {
		t.Fatalf("the bin does not show the deleted task: %s", list)
	}
	if strings.Contains(alice.get("/plan"), "Buy milk") {
		t.Error("a deleted task is still on the page")
	}
	alice.hx(fmt.Sprintf("/plan/tasks/%d/restore", milk.ID), bin)
	if !strings.Contains(alice.get("/plan"), "Buy milk") {
		t.Error("a restored task did not come back")
	}
	if code, _ := alice.hx(fmt.Sprintf("/plan/tasks/%d/purge", milk.ID), bin); code != http.StatusNotFound {
		t.Errorf("a task outside the bin was purged: %d", code)
	}
	alice.hx(fmt.Sprintf("/plan/tasks/%d/delete", milk.ID), bin)
	alice.hx(fmt.Sprintf("/plan/tasks/%d/purge", milk.ID), bin)
	if _, err := a.st.PlanTaskByID(alice.id, milk.ID); err != store.ErrNotFound {
		t.Fatalf("purged task still exists: %v", err)
	}

	// Deleting the project is announced; the page falls back to all tasks.
	if code, _, h := alice.post("/plan/projects/"+pid+"/delete", nil); code != http.StatusSeeOther || h.Get("Location") != "/plan?done=project-deleted" {
		t.Fatalf("delete project: %d %s", code, h.Get("Location"))
	}
	if code := alice.getStatus("/plan?project=" + pid); code != http.StatusOK {
		t.Errorf("the page of a deleted project: %d", code)
	}
}

// Dates without a calendar: the quick buttons, moving a task to another day,
// the week strip, and "move all late tasks to today".
func TestPlanBookQuickDates(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	day := func(n int) string { return time.Now().UTC().AddDate(0, 0, n).Format("2006-01-02") }
	due := func(id int64) string {
		got, err := a.st.PlanTaskByID(alice.id, id)
		if err != nil {
			t.Fatal(err)
		}
		return got.Due
	}

	// "Tomorrow" needs no date; the answer says so and can be undone.
	code, list := alice.hx("/plan/tasks", url.Values{"title": {"Water the plants"}, "when": {"tomorrow"}, "due": {"2001-01-01"}})
	if code != http.StatusOK || !strings.Contains(list, "Added “Water the plants”, due tomorrow.") || !strings.Contains(list, ">Undo<") {
		t.Fatalf("quick add: %d %s", code, list)
	}
	tasks, _, _ := a.st.ListPlanTasks(alice.id, store.PlanFilter{})
	plants := tasks[0].ID
	if due(plants) != day(1) {
		t.Fatalf("when=tomorrow gave %q, want %q", due(plants), day(1))
	}
	alice.hx("/plan/tasks", url.Values{"title": {"No date"}, "when": {"none"}, "due": {day(3)}})
	alice.hx("/plan/tasks", url.Values{"title": {"Picked"}, "when": {"pick"}, "due": {day(3)}})
	tasks, _, _ = a.st.ListPlanTasks(alice.id, store.PlanFilter{Query: "No date"})
	if tasks[0].Due != "" {
		t.Errorf("when=none kept the date %q", tasks[0].Due)
	}
	tasks, _, _ = a.st.ListPlanTasks(alice.id, store.PlanFilter{Query: "Picked"})
	if tasks[0].Due != day(3) {
		t.Errorf("when=pick gave %q, want %q", tasks[0].Due, day(3))
	}

	// Move it with one click; Undo carries the old date.
	item := fmt.Sprintf("/plan/tasks/%d", plants)
	_, list = alice.hx(item+"/due", url.Values{"when": {"today"}})
	if due(plants) != day(0) || !strings.Contains(list, "is now due today.") || !strings.Contains(list, `name="due" value="`+day(1)+`"`) {
		t.Fatalf("move to today: due %q, %s", due(plants), list)
	}
	alice.hx(item+"/due", url.Values{"due": {day(1)}})
	if due(plants) != day(1) {
		t.Errorf("undoing the move gave %q", due(plants))
	}
	if _, list := alice.hx(item+"/due", url.Values{"when": {"none"}}); due(plants) != "" || !strings.Contains(list, "has no date now.") {
		t.Errorf("removing the date: %q", due(plants))
	}
	alice.hx(item+"/due", url.Values{"when": {"nextweek"}})
	if d, _ := time.Parse("2006-01-02", due(plants)); d.Weekday() != time.Monday || due(plants) <= day(0) || due(plants) > day(7) {
		t.Errorf("next week = %q, want the coming Monday", due(plants))
	}
	if code, _ := alice.hx(item+"/due", url.Values{"due": {"soon"}}); code != http.StatusBadRequest {
		t.Errorf("a bad date was accepted: %d", code)
	}
	if code, _, _ := bob.post(item+"/due", url.Values{"when": {"today"}}); code != http.StatusNotFound {
		t.Errorf("bob moved alice's task: %d", code)
	}

	// The week strip counts each day and shows one day on its own.
	alice.hx(item+"/due", url.Values{"due": {day(2)}})
	page := alice.get("/plan")
	if !strings.Contains(page, `href="/plan?day=`+day(2)+`"`) || !strings.Contains(page, "1 to do") {
		t.Error("the week strip does not link to the day with a task")
	}
	page = alice.get("/plan?day=" + day(2))
	if !strings.Contains(page, "Water the plants") || strings.Contains(page, fmt.Sprintf(`id="pt-%d"`, tasks[0].ID)) || !strings.Contains(page, "Show everything") ||
		!strings.Contains(page, `name="due" type="date" value="`+day(2)+`"`) {
		t.Error("the page for one day is wrong")
	}
	if code := alice.getStatus("/plan?day=nonsense"); code != http.StatusOK {
		t.Errorf("a bad day in the address: %d", code)
	}

	// Deleting offers Undo, which restores.
	if _, list := alice.hx(item+"/delete", nil); !strings.Contains(list, "is in the bin.") || !strings.Contains(list, `hx-post="`+item+`/restore"`) {
		t.Errorf("deleting does not offer Undo: %s", list)
	}
	alice.hx(item+"/restore", nil)

	// Late tasks move to today together; bob's stay late.
	alice.hx(item+"/due", url.Values{"due": {day(-3)}})
	alice.hx("/plan/tasks", url.Values{"title": {"Also late"}, "due": {day(-1)}})
	bob.hx("/plan/tasks", url.Values{"title": {"Bob is late"}, "due": {day(-1)}})
	if p := alice.get("/plan"); !strings.Contains(p, "Move all to today") || !strings.Contains(p, "2 tasks are late") {
		t.Error("late tasks are not announced")
	}
	if _, list := alice.hx("/plan/late/today", nil); !strings.Contains(list, "Moved 2 late tasks to today.") || strings.Contains(list, "Move all to today") {
		t.Errorf("move late: %s", list)
	}
	if due(plants) != day(0) {
		t.Errorf("a late task is now due %q", due(plants))
	}
	if c := a.st.PlanCounts(bob.id, 0, day(0)); c.Overdue != 1 {
		t.Errorf("bob's late tasks = %d, want 1", c.Overdue)
	}
}

// An agent gets the level of Plan Book access its owner picked, and only the owner picks.
func TestAgentPlanAccess(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	ag := &store.Agent{UserID: alice.id, Name: "Helper", Instructions: "x", MaxSteps: 5, DailyRuns: 5, HistoryRuns: 5, Enabled: true}
	if _, err := a.st.CreateAgent(ag, 0); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/agents/%d/settings", ag.ID)
	form := url.Values{"name": {"Helper"}, "instructions": {"x"}, "enabled": {"1"}, "plan_access": {"write"}}
	if code, _, _ := bob.post(path, form); code != http.StatusNotFound {
		t.Fatalf("bob changed alice's agent: %d", code)
	}
	alice.post(path, form)
	panel := alice.get(fmt.Sprintf("/agents/%d/panel", ag.ID))
	if !strings.Contains(panel, `value="write" selected`) || !strings.Contains(panel, "The AI may not use your Plan Book yet") {
		t.Fatal("the panel does not show the access after allowing")
	}
	if got, _ := a.st.AgentByID(ag.ID); got.PlanAccess != store.PlanAccessWrite {
		t.Fatalf("access = %q", got.PlanAccess)
	}
	form.Set("plan_access", "everything")
	alice.post(path, form)
	if got, _ := a.st.AgentByID(ag.ID); got.PlanAccess != store.PlanAccessNone {
		t.Fatalf("an unknown level became %q", got.PlanAccess)
	}
}
