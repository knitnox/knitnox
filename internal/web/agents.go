package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"knitnox/internal/engine"
	"knitnox/internal/store"
)

// Agents: saved background workers. Each agent owns a thread, so its page is
// the ordinary chat page (/t/{thread}) with an agent panel instead of the
// share panel. /agents lists them; /store is where they are shared.

type scheduleOption struct {
	Minutes int
	Label   string
}

var scheduleOptions = []scheduleOption{
	{0, "Only when asked (on demand)"},
	{15, "Every 15 minutes"},
	{30, "Every 30 minutes"},
	{60, "Every hour"},
	{180, "Every 3 hours"},
	{360, "Every 6 hours"},
	{720, "Every 12 hours"},
	{1440, "Once a day"},
	{10080, "Once a week"},
}

// scheduleLabel is the short form shown on cards and in the top bar.
func scheduleLabel(minutes int) string {
	switch {
	case minutes <= 0:
		return "On demand"
	case minutes == 60:
		return "Every hour"
	case minutes == 1440:
		return "Daily"
	case minutes == 10080:
		return "Weekly"
	case minutes%1440 == 0:
		return fmt.Sprintf("Every %d days", minutes/1440)
	case minutes%60 == 0:
		return fmt.Sprintf("Every %d hours", minutes/60)
	}
	return fmt.Sprintf("Every %d min", minutes)
}

// agentTool is one of the owner's MCP servers with the agent's selection.
type agentTool struct {
	store.MCPServer
	Selected bool
	Tools    string // allowlist as typed, "" = all tools
}

// loadAgent returns the agent in the path if the current user owns it.
func (s *Server) loadAgent(w http.ResponseWriter, r *http.Request) (*store.Agent, bool) {
	a, err := s.st.AgentByID(pathID(r, "id"))
	if err != nil || a.UserID != currentUser(r).ID {
		s.errorPage(w, r, http.StatusNotFound, "Agent not found.")
		return nil, false
	}
	return a, true
}

// agentFormData is what the shared "agent-fields" template needs.
func (s *Server) agentFormData(userID int64, a *store.Agent, modelID int64) map[string]any {
	opts, _ := s.st.ModelOptions(userID)
	if findOption(opts, modelID) == nil {
		modelID = s.defaultModel(userID, opts)
	}
	servers, _ := s.st.ListMCPServers(userID)
	selected := map[int64]store.AgentTool{}
	if a.ID > 0 {
		selected, _ = s.st.AgentMCP(a.ID)
	}
	tools := make([]agentTool, 0, len(servers))
	for _, srv := range servers {
		sel, ok := selected[srv.ID]
		tools = append(tools, agentTool{MCPServer: srv, Selected: ok, Tools: strings.ReplaceAll(sel.Tools, ",", ", ")})
	}
	return map[string]any{"A": a, "ModelOptions": opts, "ModelID": modelID, "Tools": tools, "Schedules": scheduleOptions}
}

// readAgentForm copies the owner-editable fields from the form into a and
// returns the chosen model and tools.
func (s *Server) readAgentForm(r *http.Request, a *store.Agent) (modelID int64, tools []store.AgentTool) {
	r.ParseForm()
	a.Name = r.FormValue("name")
	a.Description = r.FormValue("description")
	a.Instructions = r.FormValue("instructions")
	a.ScheduleMinutes = int(formInt(r, "schedule_minutes", 0))
	a.MaxSteps = int(formInt(r, "max_steps", 20))
	a.DailyRuns = int(formInt(r, "daily_runs", 48))
	a.HistoryRuns = int(formInt(r, "history_runs", 5))
	a.OnlyWithTasks = r.FormValue("only_with_tasks") == "1"
	modelID = formInt(r, "model_id", 0)
	if opts, _ := s.st.ModelOptions(a.UserID); findOption(opts, modelID) == nil {
		modelID = s.defaultModel(a.UserID, opts)
	}
	for _, v := range r.Form["mcp"] {
		id, err := parseInt(v)
		if err != nil {
			continue
		}
		tools = append(tools, store.AgentTool{ServerID: id, Tools: r.FormValue(fmt.Sprintf("tools_%d", id))})
	}
	return modelID, tools
}

func newAgentDefaults(userID int64) *store.Agent {
	return &store.Agent{UserID: userID, MaxSteps: 20, DailyRuns: 48, HistoryRuns: 5, Enabled: true}
}

// ---- my agents ----

func (s *Server) agentsPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	agents, err := s.st.ListAgents(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := s.agentFormData(u.ID, newAgentDefaults(u.ID), 0)
	data["View"] = "list"
	data["PageTitle"] = "Agents"
	data["Agents"] = agents
	s.render(w, r, "agents", data)
}

func (s *Server) agentCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	a := newAgentDefaults(u.ID)
	modelID, tools := s.readAgentForm(r, a)
	if strings.TrimSpace(a.Name) == "" || strings.TrimSpace(a.Instructions) == "" {
		s.errorPage(w, r, http.StatusBadRequest, "An agent needs a name and instructions.")
		return
	}
	// Optional: one MCP server that only this agent uses. Checked before the
	// agent is created so a typo does not leave a half-configured agent behind.
	ownName, ownURL := strings.TrimSpace(r.FormValue("own_name")), strings.TrimSpace(r.FormValue("own_url"))
	if (ownName != "" || ownURL != "") && (ownName == "" || !validURL(ownURL)) {
		s.errorPage(w, r, http.StatusBadRequest, "The agent's own MCP server needs a name and a valid http(s) URL. Go back to fix it, or leave both empty.")
		return
	}
	if _, err := s.st.CreateAgent(a, modelID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.st.SetAgentMCP(a.ID, u.ID, tools); err != nil {
		s.serverError(w, r, err)
		return
	}
	if ownURL != "" {
		if err := s.st.CreateAgentMCPServer(a, ownName, ownURL, strings.TrimSpace(r.FormValue("own_headers")), r.FormValue("own_approval")); err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/t/%d", a.ThreadID), http.StatusSeeOther)
}

// agentPanel renders the side panel of an agent's page: tasks, settings, sharing.
func (s *Server) agentPanel(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	s.renderAgentPanel(w, r, a)
}

func (s *Server) renderAgentPanel(w http.ResponseWriter, r *http.Request, a *store.Agent) {
	var modelID int64
	if t, err := s.st.ThreadByID(a.ThreadID); err == nil && t.ModelID.Valid {
		modelID = t.ModelID.Int64
	}
	data := s.agentFormData(a.UserID, a, modelID)
	tasks, err := s.st.ListTasks(a.ID, false, 60)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data["Tasks"] = tasks
	data["OwnServers"], _ = s.st.ListAgentOwnServers(a.ID)
	data["NotesLog"], _ = s.st.AgentNotesLog(a.ID, 5)
	data["MemoryOn"] = s.st.MemoryEnabled(a.UserID)
	data["PlanOn"] = s.st.PlanEnabled(a.UserID)
	listing, _ := s.st.StoreListingFor(a.ID)
	data["Listing"] = listing
	if listing == nil { // suggest what a new listing should say about tools
		var req []string
		for _, t := range data["Tools"].([]agentTool) {
			if !t.Selected {
				continue
			}
			which := "all tools"
			if t.Tools != "" {
				which = t.Tools
			}
			req = append(req, fmt.Sprintf("- %s: %s", t.Name, which))
		}
		for _, srv := range data["OwnServers"].([]store.MCPServer) {
			req = append(req, fmt.Sprintf("- %s: all tools", srv.Name))
		}
		data["Requirements"] = strings.Join(req, "\n")
	}
	s.renderPartial(w, r, "agents", "agent-panel", data)
}

func (s *Server) agentSettings(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	modelID, tools := s.readAgentForm(r, a)
	if strings.TrimSpace(a.Name) == "" {
		s.flash(w, r, "An agent needs a name.")
		return
	}
	wasEnabled := a.Enabled
	a.Enabled = r.FormValue("enabled") == "1"
	if err := s.st.UpdateAgent(a, modelID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if a.Enabled && !wasEnabled { // restart the interval when resuming
		s.st.SetAgentEnabled(a.ID, true)
	}
	if err := s.st.SetAgentNotes(a.ID, r.FormValue("notes"), "user"); err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.st.SetAgentMCP(a.ID, a.UserID, tools); err != nil {
		s.serverError(w, r, err)
		return
	}
	if isHX(r) {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	back(w, r, fmt.Sprintf("/t/%d", a.ThreadID))
}

// agentToggle pauses or resumes an agent.
func (s *Server) agentToggle(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	if err := s.st.SetAgentEnabled(a.ID, !a.Enabled); err != nil {
		s.serverError(w, r, err)
		return
	}
	if isHX(r) {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	back(w, r, "/agents")
}

// agentRun starts the agent now, whether or not it is paused: the owner asked.
func (s *Server) agentRun(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	if _, err := s.eng.RunAgent(a, ""); err != nil {
		switch {
		case errors.Is(err, engine.ErrAgentBusy):
			s.flash(w, r, a.Name+" is already running.")
		case errors.Is(err, store.ErrQueueFull):
			s.flash(w, r, "This agent's queue is full.")
		default:
			s.serverError(w, r, err)
		}
		return
	}
	if isHX(r) {
		if r.FormValue("go") == "1" { // started from the list: show the run
			w.Header().Set("HX-Redirect", fmt.Sprintf("/t/%d", a.ThreadID))
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/t/%d", a.ThreadID), http.StatusSeeOther)
}

// agentClearMemory empties what the agent wrote down for later runs.
func (s *Server) agentClearMemory(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	if err := s.st.SetAgentMemory(a.ID, ""); err != nil {
		s.serverError(w, r, err)
		return
	}
	a.Memory = ""
	s.renderAgentPanel(w, r, a)
}

func (s *Server) agentDelete(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	s.eng.Stop(a.ThreadID)
	files := s.st.ThreadUploadFiles(a.ThreadID)
	if err := s.st.DeleteThread(a.ThreadID); err != nil { // the agent row goes with its thread
		s.serverError(w, r, err)
		return
	}
	s.st.RemoveFiles(files)
	s.hub.Publish(a.ThreadID, engine.EvMessages, nil)
	if isHX(r) {
		w.Header().Set("HX-Redirect", "/agents")
		return
	}
	http.Redirect(w, r, "/agents", http.StatusSeeOther)
}

// ---- the agent's own MCP servers ----

// agentMCPCreate adds an MCP server that only this agent uses.
func (s *Server) agentMCPCreate(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	u := strings.TrimSpace(r.FormValue("url"))
	if name == "" || !validURL(u) {
		s.flash(w, r, "Name and a valid http(s) URL are required. Only remote (HTTP) MCP servers are supported.")
		return
	}
	if err := s.st.CreateAgentMCPServer(a, name, u, strings.TrimSpace(r.FormValue("headers")), r.FormValue("approval")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderAgentPanel(w, r, a)
}

// agentMCPAction toggles, switches the approval mode of, or deletes one of
// the agent's own servers.
func (s *Server) agentMCPAction(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	srv, err := s.st.AgentOwnServer(a.ID, pathID(r, "sid"))
	if err != nil {
		s.flash(w, r, "That server no longer exists.")
		return
	}
	switch r.FormValue("action") {
	case "toggle":
		err = s.st.ToggleMCPServer(a.UserID, srv.ID)
	case "approval":
		mode := store.ApprovalAsk
		if srv.Asks() {
			mode = store.ApprovalAuto
		}
		err = s.st.SetMCPApproval(a.UserID, srv.ID, mode)
	case "delete":
		err = s.st.DeleteMCPServer(a.UserID, srv.ID)
	default:
		err = errors.New("unknown action")
	}
	if err != nil {
		s.flash(w, r, "Could not update the server: "+err.Error())
		return
	}
	s.renderAgentPanel(w, r, a)
}

// threadAgentToggle connects one of the owner's agents to a chat, or
// disconnects it (the Agents menu in the chat's top bar).
func (s *Server) threadAgentToggle(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	if a, _ := s.st.AgentByThread(t.ID); a != nil {
		s.errorPage(w, r, http.StatusBadRequest, "An agent's own page cannot manage other agents.")
		return
	}
	if _, err := s.st.ToggleThreadAgent(t.ID, t.OwnerUserID, pathID(r, "aid")); err != nil {
		s.errorPage(w, r, http.StatusNotFound, "Agent not found.")
		return
	}
	if isHX(r) {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	back(w, r, fmt.Sprintf("/t/%d", t.ID))
}

// agentOpt is one of the chat owner's agents and whether the chat is connected to it.
type agentOpt struct {
	store.AgentRow
	Connected bool
}

// agentOptions lists the chat owner's agents for the connect menus, and how
// many of them are connected.
func (s *Server) agentOptions(t *store.Thread) (opts []agentOpt, connected int) {
	agents, _ := s.st.ListAgents(t.OwnerUserID)
	list, _ := s.st.ConnectedAgents(t.ID, t.OwnerUserID)
	isConnected := map[int64]bool{}
	for _, a := range list {
		isConnected[a.ID] = true
	}
	for _, a := range agents {
		opts = append(opts, agentOpt{a, isConnected[a.ID]})
	}
	return opts, len(list)
}

// ---- tasks ----

func (s *Server) agentTaskCreate(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(r.FormValue("title")) == "" {
		s.flash(w, r, "A task needs a title.")
		return
	}
	if _, err := s.st.CreateTask(a.ID, r.FormValue("title"), r.FormValue("details"), "user", 0); err != nil {
		s.serverError(w, r, err)
		return
	}
	if r.FormValue("run_now") == "1" {
		if _, err := s.eng.RunAgent(a, ""); err != nil && !errors.Is(err, engine.ErrAgentBusy) {
			s.log.Warn("could not start agent after adding a task", "agent", a.ID, "err", err)
		}
	}
	s.renderAgentPanel(w, r, a)
}

// agentTaskAction lets the owner finish, cancel, reopen or delete a task.
func (s *Server) agentTaskAction(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	taskID := pathID(r, "tid")
	var err error
	setStatus := func(st string) error { return s.st.UpdateTask(a.ID, taskID, store.TaskPatch{Status: &st}) }
	switch r.FormValue("action") {
	case "done":
		err = setStatus(store.TaskDone)
	case "cancel":
		err = setStatus(store.TaskCancelled)
	case "reopen":
		err = setStatus(store.TaskPending)
	case "delete":
		err = s.st.DeleteTask(a.ID, taskID)
	default:
		err = errors.New("unknown action")
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.flash(w, r, "Could not update the task: "+err.Error())
		return
	}
	s.renderAgentPanel(w, r, a)
}

// ---- agent store ----

// agentPublish shares the agent's configuration in the store (or updates its listing).
func (s *Server) agentPublish(w http.ResponseWriter, r *http.Request) {
	a, ok := s.loadAgent(w, r)
	if !ok {
		return
	}
	desc := strings.TrimSpace(r.FormValue("description"))
	guide := strings.TrimSpace(r.FormValue("guide"))
	if desc == "" || guide == "" {
		s.flash(w, r, "Add a description and a short guide so others know how to use this agent.")
		return
	}
	if strings.TrimSpace(a.Instructions) == "" {
		s.flash(w, r, "Give the agent instructions before sharing it.")
		return
	}
	if _, err := s.st.PublishAgent(a, currentUser(r).Username, desc, guide, r.FormValue("requirements")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderAgentPanel(w, r, a)
}

func (s *Server) storePage(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if rs := []rune(q); len(rs) > 100 {
		q = string(rs[:100])
	}
	list, err := s.st.ListStoreAgents(q)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, "agents", map[string]any{"View": "store", "PageTitle": "Agent store", "Listings": list, "Query": q})
}

func (s *Server) storeDetail(w http.ResponseWriter, r *http.Request) {
	sa, err := s.st.StoreAgentByID(pathID(r, "sid"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "This agent is no longer in the store.")
		return
	}
	u := currentUser(r)
	s.render(w, r, "agents", map[string]any{"View": "detail", "PageTitle": "Agent store", "Listing": sa,
		"CanDelete": sa.AuthorUserID == u.ID || u.IsAdmin(), "Mine": sa.AuthorUserID == u.ID})
}

// storeInstall copies a listing into the user's own agents. The copy starts
// paused: the user still has to pick a model and their own tools.
func (s *Server) storeInstall(w http.ResponseWriter, r *http.Request) {
	sa, err := s.st.StoreAgentByID(pathID(r, "sid"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "This agent is no longer in the store.")
		return
	}
	u := currentUser(r)
	opts, _ := s.st.ModelOptions(u.ID)
	a, err := s.st.InstallStoreAgent(sa, u.ID, s.defaultModel(u.ID, opts))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/t/%d#setup", a.ThreadID), http.StatusSeeOther)
}

// storeDelete removes a listing. Its author may always do so; admins may
// remove anyone's (audited). Installed copies are not affected.
func (s *Server) storeDelete(w http.ResponseWriter, r *http.Request) {
	sa, err := s.st.StoreAgentByID(pathID(r, "sid"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "This agent is no longer in the store.")
		return
	}
	u := currentUser(r)
	if sa.AuthorUserID != u.ID && !u.IsAdmin() {
		s.errorPage(w, r, http.StatusForbidden, "Only the author or an admin can remove this.")
		return
	}
	if err := s.st.DeleteStoreAgent(sa.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if sa.AuthorUserID != u.ID {
		s.audit(r, "store.remove", sa.Name, "by "+sa.AuthorName)
	}
	if isHX(r) {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	http.Redirect(w, r, "/store", http.StatusSeeOther)
}
