package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"knitnox/internal/engine"
	"knitnox/internal/store"
)

// Plan Book: the page where a user keeps their own tasks, grouped in projects
// (/plan). The same list is used by the AI in their chats and by agents they
// allowed (engine/planbook.go). Every query is scoped to the signed-in user.
//
// The page has three tabs, like Memory: Tasks (/plan), Projects
// (/plan/projects) and Settings (/plan/settings).
//
// On the Tasks tab the list ("plan-main": the week, the open project, the
// tasks) is re-rendered after every change, and the row of controls above it
// ("plan-bar") is sent along and swapped in place, so counts and groups are
// always right without a page reload. Which list is shown travels with each request as
// four fields: view, project, q, day.
//
// A change that is easy to make by accident (tick off, delete, new date)
// answers with a short line saying what happened and an Undo button.

const planPageSize = 200 // tasks shown per view

var planUser = store.PlanActor{Kind: store.PlanByUser}

// planState is which list the page shows.
type planState struct {
	View    string
	Project int64 // 0 = all tasks, -1 = tasks without a project
	Query   string
	Day     string // only tasks due on this day (picked in the week strip); "" = any
}

// URL is the page address for this state, optionally with another view or project.
func (st planState) URL(view string, project int64) string {
	v := url.Values{}
	if view != "" && view != store.PlanViewOpen {
		v.Set("view", view)
	}
	if project != 0 {
		v.Set("project", fmt.Sprint(project))
	}
	if len(v) == 0 {
		return "/plan"
	}
	return "/plan?" + v.Encode()
}

// DayURL is the address of the to-do list for one day of the week strip ("" = every day).
func (st planState) DayURL(day string) string {
	v := url.Values{}
	if st.Project != 0 {
		v.Set("project", fmt.Sprint(st.Project))
	}
	if day != "" {
		v.Set("day", day)
	}
	if len(v) == 0 {
		return "/plan"
	}
	return "/plan?" + v.Encode()
}

func readPlanState(r *http.Request) planState {
	st := planState{View: r.FormValue("view"), Query: strings.TrimSpace(r.FormValue("q"))}
	switch st.View {
	case store.PlanViewToday, store.PlanViewUpcoming, store.PlanViewDone, store.PlanViewBin:
	default:
		st.View = store.PlanViewOpen
	}
	if rs := []rune(st.Query); len(rs) > 100 {
		st.Query = string(rs[:100])
	}
	if day := r.FormValue("day"); day != "" {
		if _, err := time.Parse("2006-01-02", day); err == nil {
			st.Day, st.View = day, store.PlanViewOpen
		}
	}
	st.Project, _ = parseInt(r.FormValue("project"))
	if st.Project < -1 {
		st.Project = 0
	}
	return st
}

// planTaskView is a task with its due date in words.
type planTaskView struct {
	store.PlanTask
	DueWords string // "today", "tomorrow", "3 days late", "Fri 13 Nov"
	DueKind  string // late | today | later | ""
	NextWeek string // the day "Next week" stands for, e.g. "Mon 16 Nov"
}

// planUndo is the line shown after a change: what happened, and how to take it back.
type planUndo struct {
	Text   string
	URL    string // posting Fields here undoes the change; "" = nothing to undo
	Fields map[string]string
}

// planDay is one day in the week strip above the list.
type planDay struct {
	Date    string // YYYY-MM-DD
	Weekday string // "Wed"
	Num     int    // day of the month
	Label   string // "Wednesday 11 November"
	Count   int    // open tasks due that day
	IsToday bool
}

// planNextWeek is the day the "Next week" button means: the coming Monday.
func planNextWeek(today string) time.Time {
	day, _ := time.Parse("2006-01-02", today)
	ahead := (8 - int(day.Weekday())) % 7
	if ahead == 0 {
		ahead = 7
	}
	return day.AddDate(0, 0, ahead)
}

// planWhen is the due date a form asks for. The quick buttons send
// when=today|tomorrow|nextweek|none; otherwise the date field "due" counts.
func planWhen(r *http.Request, today string) string {
	day, _ := time.Parse("2006-01-02", today)
	switch r.FormValue("when") {
	case "none":
		return ""
	case "today":
		return today
	case "tomorrow":
		return day.AddDate(0, 0, 1).Format("2006-01-02")
	case "nextweek":
		return planNextWeek(today).Format("2006-01-02")
	}
	return r.FormValue("due")
}

// quoted is a task title for a one-line message.
func quoted(title string) string {
	if rs := []rune(title); len(rs) > 60 {
		title = string(rs[:59]) + "…"
	}
	return "“" + title + "”"
}

// planGlance sums up the day in one plain sentence.
func planGlance(c store.PlanCounts, empty bool) string {
	n := func(k int, one, many string) string {
		if k == 1 {
			return "1 " + one
		}
		return fmt.Sprintf("%d %s", k, many)
	}
	late, today := c.Overdue, c.Today-c.Overdue
	switch {
	case empty:
		return "Nothing here yet. Add your first task above."
	case late > 0 && today > 0:
		return n(late, "task is", "tasks are") + " late and " + n(today, "is", "are") + " due today."
	case late > 0:
		return n(late, "task is", "tasks are") + " late. Nothing else is due today."
	case today > 0:
		return n(today, "task is", "tasks are") + " due today."
	case c.Open > 0:
		return "Nothing is due today. " + n(c.Open, "task is", "tasks are") + " waiting for later."
	}
	return "You are all caught up."
}

func viewTask(t store.PlanTask, today string) planTaskView {
	v := planTaskView{PlanTask: t, NextWeek: planNextWeek(today).Format("Mon 2 Jan")}
	if t.Status != store.PlanDone {
		v.DueWords, v.DueKind = engine.DueWords(t.Due, today)
	} else if t.Due != "" {
		v.DueWords, v.DueKind = t.Due, "later"
	}
	return v
}

// planGroup is a headed block of tasks: Late, Today, Tomorrow, …
type planGroup struct {
	Label string
	Kind  string // late | today | "" (for colour)
	Tasks []planTaskView
}

// groupPlanTasks splits open tasks (already sorted by due date) by when they are due.
func groupPlanTasks(tasks []store.PlanTask, today string) []planGroup {
	day, _ := time.Parse("2006-01-02", today)
	tomorrow, week := day.AddDate(0, 0, 1).Format("2006-01-02"), day.AddDate(0, 0, 7).Format("2006-01-02")
	var groups []planGroup
	add := func(label, kind string, t store.PlanTask) {
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, planGroup{Label: label, Kind: kind})
		}
		g := &groups[len(groups)-1]
		g.Tasks = append(g.Tasks, viewTask(t, today))
	}
	for _, t := range tasks {
		switch {
		case t.Due == "":
			add("No date", "", t)
		case t.Due < today:
			add("Late", "late", t)
		case t.Due == today:
			add("Today", "today", t)
		case t.Due == tomorrow:
			add("Tomorrow", "", t)
		case t.Due < week:
			add("This week", "", t)
		default:
			add("Later", "", t)
		}
	}
	return groups
}

// planMainData is everything the "plan-main" partial shows for one state.
func (s *Server) planMainData(r *http.Request, st planState) (map[string]any, error) {
	u := currentUser(r)
	today := engine.PlanToday(u.Location())
	projects, err := s.st.ListProjects(u.ID)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"State": st, "Projects": projects, "Today": today, "BinDays": store.PlanBinDays}
	if st.Project > 0 {
		p, err := s.st.ProjectByID(u.ID, st.Project)
		if err != nil { // deleted meanwhile: fall back to all tasks
			st.Project = 0
			data["State"] = st
		} else {
			data["Project"] = p
		}
	}
	tasks, total, err := s.st.ListPlanTasks(u.ID, store.PlanFilter{View: st.View, ProjectID: st.Project, Query: st.Query, Day: st.Day, Today: today, Limit: planPageSize})
	if err != nil {
		return nil, err
	}
	switch st.View {
	case store.PlanViewDone, store.PlanViewBin:
		g := planGroup{}
		for _, t := range tasks {
			g.Tasks = append(g.Tasks, viewTask(t, today))
		}
		if len(g.Tasks) > 0 {
			data["Groups"] = []planGroup{g}
		}
	default:
		data["Groups"] = groupPlanTasks(tasks, today)
	}
	data["Total"], data["Shown"] = total, len(tasks)
	counts := s.st.PlanCounts(u.ID, st.Project, today)
	data["Counts"] = counts
	all := s.st.PlanCounts(u.ID, 0, today)
	data["AllOpen"], data["NoProjectOpen"] = all.Open, s.st.PlanCounts(u.ID, -1, today).Open
	empty := all.Open+all.Done+all.Bin == 0 && len(projects) == 0 // nothing at all yet
	data["Empty"] = empty

	// The top of the list: today in words, and the next seven days to pick from.
	day, _ := time.Parse("2006-01-02", today)
	perDay := s.st.PlanDayCounts(u.ID, st.Project, today, day.AddDate(0, 0, 6).Format("2006-01-02"))
	week := make([]planDay, 7)
	for i := range week {
		d := day.AddDate(0, 0, i)
		date := d.Format("2006-01-02")
		week[i] = planDay{Date: date, Weekday: d.Format("Mon"), Num: d.Day(), Label: d.Format("Monday 2 January"), Count: perDay[date], IsToday: i == 0}
	}
	data["Week"], data["DateWords"], data["Glance"] = week, day.Format("Monday 2 January"), planGlance(counts, empty)
	data["NextWeek"] = planNextWeek(today).Format("Monday 2 January")
	if st.Day != "" {
		if d, err := time.Parse("2006-01-02", st.Day); err == nil {
			data["DayWords"] = d.Format("Monday 2 January")
		}
	}
	return data, nil
}

func (s *Server) planPage(w http.ResponseWriter, r *http.Request) {
	st := readPlanState(r)
	data, err := s.planMainData(r, st)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data["Tab"] = "tasks"
	data["On"] = s.st.PlanEnabled(currentUser(r).ID)
	data["Notice"] = r.URL.Query().Get("done")
	s.render(w, r, "planbook", data)
}

// planProjectsPage is the Projects tab: every project with how far along it is, and a form for a new one.
func (s *Server) planProjectsPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	projects, err := s.st.ListProjects(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, "planbook", map[string]any{"Tab": "projects", "Projects": projects, "On": s.st.PlanEnabled(u.ID),
		"BinDays": store.PlanBinDays, "Notice": r.URL.Query().Get("done")})
}

// planSettingsPage is the Settings tab: who may use the Plan Book, and how it works.
func (s *Server) planSettingsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "planbook", map[string]any{"Tab": "settings", "On": s.st.PlanEnabled(currentUser(r).ID),
		"BinDays": store.PlanBinDays, "Notice": r.URL.Query().Get("done")})
}

// planRefresh answers a change: the list again for htmx (with a line saying
// what happened, when undo is given), a redirect otherwise.
func (s *Server) planRefresh(w http.ResponseWriter, r *http.Request, undo *planUndo) {
	st := readPlanState(r)
	if !isHX(r) {
		if isFetch(r) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(w, r, st.URL(st.View, st.Project), http.StatusSeeOther)
		return
	}
	data, err := s.planMainData(r, st)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if undo != nil {
		data["Undo"] = undo
	}
	data["OOB"] = true // send the row of controls along: its counts changed
	s.renderPartial(w, r, "planbook", "plan-main", data)
}

// planFail shows why a change was refused.
func (s *Server) planFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.errorPage(w, r, http.StatusNotFound, "That task or project does not exist (any more).")
	case errors.Is(err, store.ErrPlanTitle), errors.Is(err, store.ErrPlanDue), errors.Is(err, store.ErrPlanStatus), errors.Is(err, store.ErrPlanProject):
		s.errorPage(w, r, http.StatusBadRequest, strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
	default:
		s.errorPage(w, r, http.StatusBadRequest, "That did not work: "+err.Error())
	}
}

// ---- tasks ----

func (s *Server) planTaskCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	today := engine.PlanToday(u.Location())
	in := store.PlanTaskInput{Title: r.FormValue("title"), Notes: r.FormValue("notes"), Due: planWhen(r, today)}
	if pid := formInt(r, "project_id", 0); pid > 0 {
		in.ProjectID = pid
	}
	id, err := s.st.CreatePlanTask(u.ID, in, planUser)
	if err != nil {
		s.planFail(w, r, err)
		return
	}
	var undo *planUndo
	if t, err := s.st.PlanTaskByID(u.ID, id); err == nil {
		text := "Added " + quoted(t.Title)
		if words, _ := engine.DueWords(t.Due, today); words != "" {
			text += ", due " + words
		}
		undo = &planUndo{Text: text + ".", URL: fmt.Sprintf("/plan/tasks/%d/delete", id)}
	}
	s.planRefresh(w, r, undo)
}

// loadPlanTask returns the task in the path if the current user owns it.
func (s *Server) loadPlanTask(w http.ResponseWriter, r *http.Request) (*store.PlanTask, bool) {
	t, err := s.st.PlanTaskByID(currentUser(r).ID, pathID(r, "id"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "Task not found.")
		return nil, false
	}
	return t, true
}

// planTaskCard renders one task (when editing is cancelled).
func (s *Server) planTaskCard(w http.ResponseWriter, r *http.Request) {
	if t, ok := s.loadPlanTask(w, r); ok {
		s.renderPartial(w, r, "planbook", "plan-card", map[string]any{"T": viewTask(*t, engine.PlanToday(currentUser(r).Location()))})
	}
}

// planTaskEdit renders the edit form in place of the card, with the task's history.
func (s *Server) planTaskEdit(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadPlanTask(w, r)
	if !ok {
		return
	}
	projects, _ := s.st.ListProjects(t.UserID)
	log, _ := s.st.PlanTaskLog(t.UserID, t.ID, 20)
	s.renderPartial(w, r, "planbook", "plan-edit", map[string]any{"T": t, "Projects": projects, "Log": log})
}

func (s *Server) planTaskUpdate(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadPlanTask(w, r)
	if !ok {
		return
	}
	title, notes, due, status := r.FormValue("title"), r.FormValue("notes"), r.FormValue("due"), r.FormValue("status")
	pid := max(formInt(r, "project_id", 0), 0)
	if err := s.st.UpdatePlanTask(t.UserID, t.ID, store.PlanTaskPatch{Title: &title, Notes: &notes, Due: &due, Status: &status, ProjectID: &pid}, planUser); err != nil {
		s.planFail(w, r, err)
		return
	}
	s.planRefresh(w, r, nil)
}

// planTaskStatus ticks a task off, starts it, or puts it back.
func (s *Server) planTaskStatus(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadPlanTask(w, r)
	if !ok {
		return
	}
	status := r.FormValue("status")
	if err := s.st.UpdatePlanTask(t.UserID, t.ID, store.PlanTaskPatch{Status: &status}, planUser); err != nil {
		s.planFail(w, r, err)
		return
	}
	var undo *planUndo
	if status == store.PlanDone && t.Status != store.PlanDone {
		undo = &planUndo{Text: "Done: " + quoted(t.Title) + ". Nice.", URL: fmt.Sprintf("/plan/tasks/%d/status", t.ID), Fields: map[string]string{"status": t.Status}}
	}
	s.planRefresh(w, r, undo)
}

// planTaskDue gives a task another due date in one click ("Tomorrow", "No date", …).
func (s *Server) planTaskDue(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadPlanTask(w, r)
	if !ok {
		return
	}
	today := engine.PlanToday(currentUser(r).Location())
	due := planWhen(r, today)
	if err := s.st.UpdatePlanTask(t.UserID, t.ID, store.PlanTaskPatch{Due: &due}, planUser); err != nil {
		s.planFail(w, r, err)
		return
	}
	var undo *planUndo
	if now, err := s.st.PlanTaskByID(t.UserID, t.ID); err == nil && now.Due != t.Due {
		text := quoted(t.Title) + " has no date now."
		if words, _ := engine.DueWords(now.Due, today); words != "" {
			text = quoted(t.Title) + " is now due " + words + "."
		}
		undo = &planUndo{Text: text, URL: fmt.Sprintf("/plan/tasks/%d/due", t.ID), Fields: map[string]string{"due": t.Due}}
	}
	s.planRefresh(w, r, undo)
}

// planTaskDelete moves a task to the bin.
func (s *Server) planTaskDelete(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadPlanTask(w, r)
	if !ok {
		return
	}
	if err := s.st.DeletePlanTask(t.UserID, t.ID, planUser); err != nil {
		s.planFail(w, r, err)
		return
	}
	s.planRefresh(w, r, &planUndo{Text: quoted(t.Title) + " is in the bin.", URL: fmt.Sprintf("/plan/tasks/%d/restore", t.ID)})
}

// planLateToday moves every late task in the shown project to today.
func (s *Server) planLateToday(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	n, err := s.st.MoveLatePlanTasks(u.ID, readPlanState(r).Project, engine.PlanToday(u.Location()), planUser)
	if err != nil {
		s.planFail(w, r, err)
		return
	}
	text := fmt.Sprintf("Moved %d late tasks to today.", n)
	if n == 1 {
		text = "Moved 1 late task to today."
	}
	s.planRefresh(w, r, &planUndo{Text: text})
}

// planTaskAction runs a bin action (restore, purge) and shows the list again.
func (s *Server) planTaskAction(fn func(userID, id int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, ok := s.loadPlanTask(w, r)
		if !ok {
			return
		}
		if err := fn(t.UserID, t.ID); err != nil {
			s.planFail(w, r, err)
			return
		}
		s.planRefresh(w, r, nil)
	}
}

func (s *Server) planBinEmpty(w http.ResponseWriter, r *http.Request) {
	if err := s.st.EmptyPlanBin(currentUser(r).ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.planRefresh(w, r, nil)
}

// ---- projects ----

func (s *Server) planProjectCreate(w http.ResponseWriter, r *http.Request) {
	id, err := s.st.CreateProject(currentUser(r).ID, r.FormValue("name"), r.FormValue("notes"))
	if err != nil {
		s.planFail(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/plan?project=%d", id), http.StatusSeeOther)
}

func (s *Server) planProjectUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "pid")
	if err := s.st.UpdateProject(currentUser(r).ID, id, r.FormValue("name"), r.FormValue("notes")); err != nil {
		s.planFail(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/plan?project=%d", id), http.StatusSeeOther)
}

func (s *Server) planProjectDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.st.DeleteProject(currentUser(r).ID, pathID(r, "pid"), planUser); err != nil {
		s.planFail(w, r, err)
		return
	}
	http.Redirect(w, r, "/plan?done=project-deleted", http.StatusSeeOther)
}

// ---- who may use it ----

// planSwitch turns the AI's use of the user's Plan Book on or off.
func (s *Server) planSwitch(w http.ResponseWriter, r *http.Request) {
	on := r.FormValue("enabled") == "1"
	if err := s.st.SetPlanEnabled(currentUser(r).ID, on); err != nil {
		s.serverError(w, r, err)
		return
	}
	done := "off"
	if on {
		done = "on"
	}
	to := "/plan"
	if r.FormValue("from") == "settings" { // switched on the Settings tab: stay there
		to = "/plan/settings"
	}
	http.Redirect(w, r, to+"?done="+done, http.StatusSeeOther)
}

// agentPlanAccess sets what one agent may do with its owner's Plan Book.
func (s *Server) agentPlanAccess(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	if err := s.st.SetAgentPlanAccess(a.ID, r.FormValue("level")); err != nil {
		s.serverError(w, r, err)
		return
	}
	a, _ = s.st.AgentByID(a.ID)
	s.renderAgentPanel(w, r, a)
}
