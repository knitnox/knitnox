package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

func TestAgentPagesTasksAndStore(t *testing.T) {
	a := newApp(t)
	llm := fakeLLM(t)
	alice := a.register("alice")
	modelID := a.addModel(llm.URL)
	bob := a.register("bob")

	// Create an agent from the form.
	code, body, h := alice.post("/agents", url.Values{
		"name": {"Inbox helper"}, "description": {"Sorts email"}, "instructions": {"Draft replies. Never send."},
		"model_id": {fmt.Sprint(modelID)}, "schedule_minutes": {"60"}, "max_steps": {"12"},
		"use_memory": {"1"}, "plan_access": {"read"}, "feed_post": {"1"}, "feed_daily": {"7"},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("create agent = %d %s", code, body)
	}
	threadID := threadIDFromLocation(t, h)
	agents, _ := a.st.ListAgents(alice.id)
	if len(agents) != 1 || agents[0].ThreadID != threadID || agents[0].MaxSteps != 12 || agents[0].ScheduleMinutes != 60 || agents[0].NextRunAt == "" {
		t.Fatalf("agents = %+v", agents)
	}
	ag := agents[0]
	// The built-in tools ticked on the form are saved with the agent.
	if !ag.UseMemory || ag.PlanAccess != store.PlanAccessRead || ag.FeedAccess != store.FeedAccessPost || ag.FeedDaily != 7 {
		t.Fatalf("built-in tools after creating: memory %v, plan %q, feed %q, %d a day", ag.UseMemory, ag.PlanAccess, ag.FeedAccess, ag.FeedDaily)
	}

	// Its thread is the agent page, and stays out of the chat list.
	page := alice.get(fmt.Sprintf("/t/%d", threadID))
	if !strings.Contains(page, fmt.Sprintf(`hx-get="/agents/%d/panel"`, ag.ID)) || !strings.Contains(page, "Run now") {
		t.Fatal("agent page is missing the agent controls")
	}
	if strings.Contains(alice.get("/"), fmt.Sprintf(`data-thread-row="%d"`, threadID)) {
		t.Fatal("agent thread is listed among the chats")
	}
	if list := alice.get("/agents"); !strings.Contains(list, "Inbox helper") || !strings.Contains(list, "Every hour") {
		t.Fatal("agents page does not list the agent")
	}

	// Tasks: add, finish, and see them in the panel.
	base := fmt.Sprintf("/agents/%d", ag.ID)
	if code, body, _ := alice.post(base+"/tasks", url.Values{"title": {"Reply to Sam"}, "details": {"invoice"}}); code != 200 || !strings.Contains(body, "Reply to Sam") {
		t.Fatalf("add task = %d %s", code, body)
	}
	tasks, _ := a.st.ListTasks(ag.ID, true, 10)
	if len(tasks) != 1 || tasks[0].CreatedBy != "user" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if code, _, _ := alice.post(fmt.Sprintf("%s/tasks/%d", base, tasks[0].ID), url.Values{"action": {"done"}}); code != 200 {
		t.Fatalf("task done = %d", code)
	}
	if got, _ := a.st.TaskByID(tasks[0].ID); got.Status != store.TaskDone {
		t.Fatalf("task status = %s", got.Status)
	}

	// Nobody else can see or change the agent.
	if got := bob.getStatus(base + "/panel"); got != http.StatusNotFound {
		t.Fatalf("other user's panel = %d", got)
	}
	for _, p := range []string{"/tasks", "/run", "/toggle", "/delete", "/settings", "/publish"} {
		if code, _, _ := bob.post(base+p, url.Values{"title": {"x"}, "name": {"x"}}); code != http.StatusNotFound {
			t.Fatalf("bob POST %s = %d, want 404", p, code)
		}
	}

	// Settings: tools must belong to the owner; pausing works.
	a.st.CreateMCPServer(alice.id, "mail", "http://127.0.0.1:1/mcp", "Authorization: Bearer secret-token", "auto")
	a.st.CreateMCPServer(bob.id, "bobs", "http://127.0.0.1:1/mcp", "", "auto")
	mine, _ := a.st.ListMCPServers(alice.id)
	theirs, _ := a.st.ListMCPServers(bob.id)
	code, body, _ = alice.post(base+"/settings", url.Values{
		"name": {"Inbox helper"}, "instructions": {"Draft replies. Never send."}, "model_id": {fmt.Sprint(modelID)},
		"schedule_minutes": {"60"}, "max_steps": {"12"}, "daily_runs": {"10"}, "history_runs": {"7"}, "enabled": {"1"},
		"notes": {"Keep it short"}, "mcp": {fmt.Sprint(mine[0].ID), fmt.Sprint(theirs[0].ID)},
		fmt.Sprintf("tools_%d", mine[0].ID): {"search, create_draft"},
	})
	if code >= 400 {
		t.Fatalf("save settings = %d %s", code, body)
	}
	sel, _ := a.st.AgentMCP(ag.ID)
	if len(sel) != 1 || sel[mine[0].ID].Tools != "search,create_draft" {
		t.Fatalf("agent tools = %+v", sel)
	}
	saved, _ := a.st.AgentByID(ag.ID)
	if saved.Notes != "Keep it short" || saved.HistoryRuns != 7 || saved.DailyRuns != 10 || !saved.Enabled {
		t.Fatalf("saved agent = %+v", saved)
	}

	// Publish to the store; the listing carries no tool URLs or secrets.
	if code, body, _ := alice.post(base+"/publish", url.Values{"description": {"Sorts email"}, "guide": {"Connect a mail server."}, "requirements": {"- mail: search, create_draft"}}); code != 200 {
		t.Fatalf("publish = %d %s", code, body)
	}
	listings, _ := a.st.ListStoreAgents("")
	if len(listings) != 1 {
		t.Fatalf("listings = %+v", listings)
	}
	sid := listings[0].ID
	detail := bob.get(fmt.Sprintf("/store/%d", sid))
	for _, want := range []string{"Inbox helper", "Connect a mail server.", "Draft replies. Never send.", "Add to my agents"} {
		if !strings.Contains(detail, want) {
			t.Errorf("store page is missing %q", want)
		}
	}
	for _, leak := range []string{"127.0.0.1", "secret-token", "Keep it short", "Reply to Sam"} {
		if strings.Contains(detail, leak) {
			t.Errorf("store page leaks %q", leak)
		}
	}
	if !strings.Contains(bob.get("/store?q=inbox"), "Inbox helper") || strings.Contains(bob.get("/store?q=zzz"), "Inbox helper") {
		t.Error("store search is wrong")
	}

	// Bob installs it: his own paused copy, no tools, nothing of Alice's.
	code, _, h = bob.post(fmt.Sprintf("/store/%d/install", sid), nil)
	if code != http.StatusSeeOther {
		t.Fatalf("install = %d", code)
	}
	bobThread := threadIDFromLocation(t, h)
	copyOf, _ := a.st.AgentByThread(bobThread)
	if copyOf == nil || copyOf.UserID != bob.id || copyOf.Enabled || copyOf.Instructions != "Draft replies. Never send." || copyOf.Notes != "" || copyOf.StoreID != sid {
		t.Fatalf("installed copy = %+v", copyOf)
	}
	if tools, _ := a.st.AgentMCP(copyOf.ID); len(tools) != 0 {
		t.Fatal("installed copy inherited tools")
	}
	if l, _ := a.st.StoreAgentByID(sid); l.Installs != 1 {
		t.Fatalf("installs = %d", l.Installs)
	}

	// Only the author (or an admin) can remove the listing. Alice registered first, so she is the admin.
	carol := a.register("carol")
	if code, _, _ := carol.post(fmt.Sprintf("/store/%d/delete", sid), nil); code != http.StatusForbidden {
		t.Fatalf("stranger removing a listing = %d", code)
	}
	if code, _, _ := alice.post(fmt.Sprintf("/store/%d/delete", sid), nil); code != http.StatusSeeOther {
		t.Fatalf("author removing a listing = %d", code)
	}
	if _, err := a.st.AgentByID(copyOf.ID); err != nil {
		t.Fatal("removing a listing deleted an installed copy")
	}

	// Run now, then delete: the thread goes with the agent.
	if code, body, _ := alice.post(base+"/run", nil); code != http.StatusSeeOther {
		t.Fatalf("run = %d %s", code, body)
	}
	a.waitDone(threadID)
	if code, _, _ := alice.post(base+"/delete", nil); code != http.StatusSeeOther {
		t.Fatalf("delete = %d", code)
	}
	if _, err := a.st.ThreadByID(threadID); err == nil {
		t.Fatal("agent thread survived deleting the agent")
	}
}

// A chat owner connects an agent in the chat's settings; members cannot.
func TestConnectAgentFromChatSettings(t *testing.T) {
	a := newApp(t)
	llm := fakeLLM(t)
	alice := a.register("alice")
	modelID := a.addModel(llm.URL)
	bob := a.register("bob")

	ag := &store.Agent{UserID: alice.id, Name: "Helper", Instructions: "x", MaxSteps: 5, DailyRuns: 5, HistoryRuns: 5, Enabled: true}
	if _, err := a.st.CreateAgent(ag, modelID); err != nil {
		t.Fatal(err)
	}
	bobs := &store.Agent{UserID: bob.id, Name: "Bobs", Instructions: "x", MaxSteps: 5, DailyRuns: 5, HistoryRuns: 5, Enabled: true}
	a.st.CreateAgent(bobs, modelID)

	chat, _ := a.st.CreateThread(alice.id, modelID)
	path := fmt.Sprintf("/t/%d", chat)
	if panel := alice.get(path + "/panel"); !strings.Contains(panel, "Connected agents") || !strings.Contains(panel, "Helper") || strings.Contains(panel, "Bobs") {
		t.Fatal("chat panel should offer the owner's agents only")
	}
	form := url.Values{"title": {"Chat"}, "model_id": {fmt.Sprint(modelID)}, "agent": {fmt.Sprint(ag.ID), fmt.Sprint(bobs.ID)}}
	if code, body, _ := alice.post(path+"/settings", form); code >= 400 {
		t.Fatalf("save chat settings = %d %s", code, body)
	}
	got, _ := a.st.ConnectedAgents(chat, alice.id)
	if len(got) != 1 || got[0].ID != ag.ID {
		t.Fatalf("connected = %+v", got)
	}
	// Saving without the box unticked disconnects; the agent itself is untouched.
	form.Del("agent")
	alice.post(path+"/settings", form)
	if got, _ := a.st.ConnectedAgents(chat, alice.id); len(got) != 0 {
		t.Fatal("agent still connected")
	}
	if _, err := a.st.AgentByID(ag.ID); err != nil {
		t.Fatal("disconnecting removed the agent")
	}
	if code, _, _ := bob.post(path+"/settings", form); code != http.StatusNotFound {
		t.Fatalf("non-member changing chat settings = %d", code)
	}
}

// The Agents menu connects with one click, and an agent can have its own MCP servers.
func TestAgentMenuAndOwnServers(t *testing.T) {
	a := newApp(t)
	llm := fakeLLM(t)
	alice := a.register("alice")
	modelID := a.addModel(llm.URL)
	bob := a.register("bob")

	ag := &store.Agent{UserID: alice.id, Name: "Helper", Instructions: "x", MaxSteps: 5, DailyRuns: 5, HistoryRuns: 5, Enabled: true}
	if _, err := a.st.CreateAgent(ag, modelID); err != nil {
		t.Fatal(err)
	}
	chat, _ := a.st.CreateThread(alice.id, modelID)
	toggle := fmt.Sprintf("/t/%d/agents/%d", chat, ag.ID)

	if page := alice.get(fmt.Sprintf("/t/%d", chat)); !strings.Contains(page, `hx-post="`+toggle+`"`) {
		t.Fatal("chat page has no Agents menu entry for the agent")
	}
	if code, body, _ := alice.post(toggle, nil); code >= 400 {
		t.Fatalf("connect = %d %s", code, body)
	}
	if got, _ := a.st.ConnectedAgents(chat, alice.id); len(got) != 1 {
		t.Fatal("agent was not connected")
	}
	if page := alice.get(fmt.Sprintf("/t/%d", chat)); !strings.Contains(page, "1 agent connected") {
		t.Fatal("chat page does not show the connection")
	}
	if code, _, _ := bob.post(toggle, nil); code != http.StatusNotFound {
		t.Fatalf("stranger toggling = %d", code)
	}
	alice.post(toggle, nil)
	if got, _ := a.st.ConnectedAgents(chat, alice.id); len(got) != 0 {
		t.Fatal("agent was not disconnected")
	}
	// An agent's own page cannot manage agents.
	if code, _, _ := alice.post(fmt.Sprintf("/t/%d/agents/%d", ag.ThreadID, ag.ID), nil); code != http.StatusBadRequest {
		t.Fatalf("connecting an agent to its own page = %d", code)
	}

	// A dedicated server can be given while creating the agent.
	code, body, h := alice.post("/agents", url.Values{"name": {"With server"}, "instructions": {"x"}, "model_id": {fmt.Sprint(modelID)},
		"own_name": {"born-with"}, "own_url": {"http://127.0.0.1:1/mcp"}, "own_approval": {"ask"}})
	if code != http.StatusSeeOther {
		t.Fatalf("create with own server = %d %s", code, body)
	}
	born, _ := a.st.AgentByThread(threadIDFromLocation(t, h))
	if list, _ := a.st.ListAgentOwnServers(born.ID); len(list) != 1 || list[0].Name != "born-with" || !list[0].Asks() {
		t.Fatalf("own servers at creation = %+v", list)
	}
	before, _ := a.st.ListAgents(alice.id)
	if code, _, _ := alice.post("/agents", url.Values{"name": {"Bad"}, "instructions": {"x"}, "own_name": {"oops"}, "own_url": {"not a url"}}); code != http.StatusBadRequest {
		t.Fatalf("bad own server URL = %d", code)
	}
	if after, _ := a.st.ListAgents(alice.id); len(after) != len(before) {
		t.Fatal("a rejected form still created an agent")
	}

	// The form has a page of its own. A mistake comes back on that page, with
	// what was typed still there and a message next to the field.
	if page := alice.get("/agents/new"); !strings.Contains(page, `action="/agents"`) || !strings.Contains(page, "When should it work?") || !strings.Contains(page, "Tools and access") {
		t.Fatal("the new agent page is missing its form")
	}
	if list := alice.get("/agents"); !strings.Contains(list, `href="/agents/new"`) || strings.Contains(list, `name="instructions"`) {
		t.Fatal("the list should link to the new agent page and no longer carry the form")
	}
	code, body, _ = alice.post("/agents", url.Values{"name": {"Half done"}, "description": {"kept"}, "schedule_minutes": {"1440"},
		"use_memory": {"1"}, "own_name": {"oops"}, "own_url": {"not a url"}})
	if code != http.StatusBadRequest {
		t.Fatalf("form with mistakes = %d", code)
	}
	for _, want := range []string{`value="Half done"`, `value="kept"`, `value="1440" checked`, `name="use_memory" value="1" checked`,
		`value="oops"`, `id="err-instructions"`, `id="err-own"`, "Nothing was created yet"} {
		if !strings.Contains(body, want) {
			t.Errorf("the form that came back is missing %s", want)
		}
	}
	if strings.Contains(body, `id="err-name"`) {
		t.Error("the name was fine but is marked as wrong")
	}

	// Own MCP servers: added in the panel, never on the Settings page.
	base := fmt.Sprintf("/agents/%d", ag.ID)
	code, body, _ = alice.post(base+"/mcp", url.Values{"name": {"crm-private"}, "url": {"http://127.0.0.1:1/mcp"}, "headers": {"Authorization: Bearer s3cret"}, "approval": {"ask"}})
	if code != 200 || !strings.Contains(body, "crm-private") || strings.Contains(body, "s3cret") {
		t.Fatalf("add own server = %d", code)
	}
	own, _ := a.st.ListAgentOwnServers(ag.ID)
	if len(own) != 1 || !own[0].Asks() || own[0].AgentID != ag.ID {
		t.Fatalf("own servers = %+v", own)
	}
	if strings.Contains(alice.get("/settings"), "crm-private") {
		t.Fatal("an agent's own server is listed on the Settings page")
	}
	if code, _, _ := bob.post(fmt.Sprintf("%s/mcp/%d", base, own[0].ID), url.Values{"action": {"delete"}}); code != http.StatusNotFound {
		t.Fatalf("stranger deleting an own server = %d", code)
	}
	alice.post(fmt.Sprintf("%s/mcp/%d", base, own[0].ID), url.Values{"action": {"approval"}})
	if own, _ = a.st.ListAgentOwnServers(ag.ID); own[0].Asks() {
		t.Fatal("approval mode did not switch")
	}
	alice.post(fmt.Sprintf("%s/mcp/%d", base, own[0].ID), url.Values{"action": {"delete"}})
	if own, _ = a.st.ListAgentOwnServers(ag.ID); len(own) != 0 {
		t.Fatal("own server was not deleted")
	}
}

// Agents ticked on the home screen are connected before the first message runs.
func TestConnectAgentsFromHomeScreen(t *testing.T) {
	a := newApp(t)
	llm := fakeLLM(t)
	alice := a.register("alice")
	modelID := a.addModel(llm.URL)
	bob := a.register("bob")

	mk := func(owner int64, name string) *store.Agent {
		ag := &store.Agent{UserID: owner, Name: name, Instructions: "x", MaxSteps: 5, DailyRuns: 5, HistoryRuns: 5, Enabled: true}
		if _, err := a.st.CreateAgent(ag, modelID); err != nil {
			t.Fatal(err)
		}
		return ag
	}
	one, two, bobs := mk(alice.id, "Helper one"), mk(alice.id, "Helper two"), mk(bob.id, "Bobs agent")

	home := alice.get("/")
	if !strings.Contains(home, `name="agent" value="`+fmt.Sprint(one.ID)+`"`) || !strings.Contains(home, "Helper two") || strings.Contains(home, "Bobs agent") {
		t.Fatal("home screen should offer the user's own agents")
	}
	if strings.Contains(bob.get("/"), "Helper one") {
		t.Fatal("home screen shows someone else's agents")
	}

	code, body, h := alice.post("/t", url.Values{"content": {"hello"}, "model_id": {fmt.Sprint(modelID)},
		"agent": {fmt.Sprint(one.ID), fmt.Sprint(two.ID), fmt.Sprint(bobs.ID)}})
	if code != http.StatusSeeOther {
		t.Fatalf("new chat = %d %s", code, body)
	}
	chat := threadIDFromLocation(t, h)
	got, _ := a.st.ConnectedAgents(chat, alice.id)
	if len(got) != 2 || got[0].ID != one.ID || got[1].ID != two.ID {
		t.Fatalf("connected = %+v", got)
	}
	a.waitDone(chat)
	// The very first turn was already offered the manager tools: the fake model calls the first tool it sees.
	msgs, _ := a.st.AllMessages(chat)
	var usedTool bool
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolName == "list_agents" {
			usedTool = true
		}
	}
	if !usedTool {
		t.Fatal("the first message ran without the connected agents' tools")
	}
	if page := alice.get(fmt.Sprintf("/t/%d", chat)); !strings.Contains(page, "2 agents connected") {
		t.Fatal("chat page does not show the connections")
	}
}
