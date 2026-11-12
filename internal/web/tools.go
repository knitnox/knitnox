package web

import (
	"fmt"
	"net/http"

	"knitnox/internal/store"
)

// The Tools menu next to "Attach"
//
// It lists what the AI may use in one chat and lets the owner switch any of
// it off for that chat: the built-in groups (memory, Plan Book, Feed,
// pictures, connected agents) and each tool server. It can only narrow: a
// group the owner never turned on is shown greyed out with a link to where
// that is done. Choices are rows in thread_tools_off; engine.openTools reads
// them at the start of every run.

// toolRow is one line of the menu.
type toolRow struct {
	Key   string
	Name  string
	Hint  string // one short line under the name
	On    bool   // used in this chat
	Ready bool   // set up, so it can be switched; false = greyed out with Setup
	Setup string // where to turn it on ("" = nowhere, e.g. not in group chats)
}

// toolRows builds the menu for a chat owned by owner. t is nil on the home
// screen, where the chat does not exist yet and everything set up is on.
func (s *Server) toolRows(owner *store.User, t *store.Thread, group bool) []toolRow {
	off := map[string]bool{}
	if t != nil {
		off = s.st.ThreadToolsOff(t.ID)
	}
	var rows []toolRow
	// The personal groups are never used in group chats (engine.openTools).
	personal := func(key, name, hint string, enabled bool, setup, how string) {
		r := toolRow{Key: key, Name: name, Hint: hint}
		switch {
		case group:
			r.Hint = "Not used in group chats: it is personal to you"
		case enabled:
			r.Ready, r.On = true, !off[key]
		default:
			r.Hint, r.Setup = how, setup
		}
		rows = append(rows, r)
	}
	picture, _ := s.eng.CanMakeInChat(owner.ID)
	personal(store.ToolMemory, "Memory", "Save facts about you and look them up", s.st.MemoryEnabled(owner.ID), "/memory", "Off for your account. Turn it on under Memory")
	personal(store.ToolPlan, "Plan Book", "Add, list and finish your tasks", s.st.PlanEnabled(owner.ID), "/plan/settings", "Off for your account. Turn it on under Plan Book → Settings")
	personal(store.ToolFeed, "Feed", "Post to your feed and read your posts", s.st.FeedEnabled(owner.ID), "/feed/settings", "Off for your account. Turn it on under Feed → Settings")
	personal(store.ToolPictures, "Pictures", "Make a picture and show it in the chat", picture, "/feed/settings", "No picture model yet. Choose one under Feed → Settings")

	if t != nil {
		if agents, _ := s.st.ConnectedAgents(t.ID, t.OwnerUserID); len(agents) > 0 {
			name := "Connected agents"
			if len(agents) == 1 {
				name = "Agent: " + agents[0].Name
			}
			rows = append(rows, toolRow{Key: store.ToolAgents, Name: name, Hint: "Give them tasks, start them, read their results",
				Ready: true, On: !off[store.ToolAgents]})
		}
	}

	var servers []store.MCPServer
	if t != nil {
		servers, _ = s.st.ThreadMCPServers(t, group)
	} else if all, err := s.st.ListMCPServers(owner.ID); err == nil {
		for _, srv := range all {
			if srv.Enabled {
				servers = append(servers, srv)
			}
		}
	}
	for _, srv := range servers {
		hint := "Tool server · runs its tools by itself"
		if srv.Asks() {
			hint = "Tool server · asks you before each tool"
		}
		key := store.ToolServerKey(srv.ID)
		rows = append(rows, toolRow{Key: key, Name: srv.Name, Hint: hint, Ready: true, On: !off[key]})
	}
	return rows
}

// toolsData is what the menu template needs.
func (s *Server) toolsData(owner *store.User, t *store.Thread, group bool) map[string]any {
	rows := s.toolRows(owner, t, group)
	offCount, servers := 0, 0
	for _, r := range rows {
		if r.Ready && !r.On {
			offCount++
		}
		if len(r.Key) > 4 && r.Key[:4] == "mcp:" {
			servers++
		}
	}
	data := map[string]any{"Tools": rows, "ToolsOff": offCount, "ToolServers": servers, "Group": group}
	if t != nil {
		data["Thread"] = t
		// Tool descriptions sit at the very start of what the provider reads, so a
		// change makes it read (and bill) the whole conversation again once.
		data["ToolsMidChat"] = s.st.LastRunID(t.ID) > 0
	}
	return data
}

// threadToolSet switches one row of the Tools menu for a chat (owner only)
// and answers with the menu drawn again.
func (s *Server) threadToolSet(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return
	}
	if a, _ := s.st.AgentByThread(t.ID); a != nil {
		s.errorPage(w, r, http.StatusBadRequest, "An agent's tools are chosen in its settings.")
		return
	}
	owner := currentUser(r)
	group := s.st.MemberCount(t.ID) > 1
	key, on := r.FormValue("key"), r.FormValue("on") == "1"
	found := false
	for _, row := range s.toolRows(owner, t, group) {
		if row.Key == key && row.Ready {
			found = true
		}
	}
	if !found {
		s.errorPage(w, r, http.StatusBadRequest, "That tool is not set up for this chat.")
		return
	}
	if err := s.st.SetThreadTool(t.ID, key, on); err != nil {
		s.serverError(w, r, err)
		return
	}
	if !isHX(r) {
		back(w, r, fmt.Sprintf("/t/%d", t.ID))
		return
	}
	data := s.toolsData(owner, t, group)
	data["Changed"] = true
	s.renderPartial(w, r, "chat", "tools-body", data)
}

// newThreadTools stores the Tools menu of the home screen for a chat that was
// just created: every row that was set up but left unticked is switched off.
func (s *Server) newThreadTools(r *http.Request, owner *store.User, threadID int64) {
	if r.FormValue("tools_menu") != "1" { // a form without the menu: leave everything on
		return
	}
	ticked := map[string]bool{}
	for _, k := range r.Form["tool_on"] {
		ticked[k] = true
	}
	for _, row := range s.toolRows(owner, nil, false) {
		if row.Ready && !ticked[row.Key] {
			if err := s.st.SetThreadTool(threadID, row.Key, false); err != nil {
				s.log.Warn("could not switch a tool off for the new chat", "thread", threadID, "tool", row.Key, "err", err)
			}
		}
	}
}
