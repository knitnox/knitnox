package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"knitnox/internal/store"
)

// The Tools menu next to "Attach"
//
// It lists what the AI may use in one chat and lets the owner switch any of
// it off for that chat. What is listed depends on the kind of chat:
//
//   - a personal chat: the owner's built-in groups (memory, Plan Book, Feed,
//     pictures, connected agents), each of their tool servers, and the groups
//     they are in, which the chat can be connected to;
//   - a group: only what belongs to the group, its own memory and its own
//     tool servers. Nobody's personal tools are ever offered there.
//
// It can only narrow: something the owner never turned on is shown greyed
// out with a link to where that is done. Switches are rows in
// thread_tools_off, connected groups rows in thread_groups;
// engine.openTools reads both at the start of every run.

// toolRow is one line of the menu.
type toolRow struct {
	Key   string
	Name  string
	Hint  string // one short line under the name
	On    bool   // used in this chat
	Ready bool   // set up, so it can be switched; false = greyed out with Setup
	Setup string // where to turn it on ("" = nowhere)
}

// groupKey is the menu key of a group a personal chat can look into.
func groupKey(groupID int64) string { return fmt.Sprintf("group:%d", groupID) }

// parseGroupKey returns the group id of such a key, or 0.
func parseGroupKey(key string) int64 {
	rest, ok := strings.CutPrefix(key, "group:")
	if !ok {
		return 0
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// toolRows builds the menu for a chat owned by owner. t is nil on the home
// screen, where the chat does not exist yet and everything set up is on.
func (s *Server) toolRows(owner *store.User, t *store.Thread) []toolRow {
	off := map[string]bool{}
	if t != nil {
		off = s.st.ThreadToolsOff(t.ID)
	}
	serverRows := func(servers []store.MCPServer, who string) []toolRow {
		var rows []toolRow
		for _, srv := range servers {
			hint := "Tool server · runs its tools by itself"
			if srv.Asks() {
				hint = "Tool server · asks " + who + " before each tool"
			}
			key := store.ToolServerKey(srv.ID)
			rows = append(rows, toolRow{Key: key, Name: srv.Name, Hint: hint, Ready: true, On: !off[key]})
		}
		return rows
	}
	if t != nil && t.IsGroup() {
		rows := []toolRow{{Key: store.ToolMemory, Name: "Group memory", Hint: "Keeps what the group decides and looks it up later",
			Ready: true, On: !off[store.ToolMemory]}}
		servers, _ := s.st.ThreadMCPServers(t)
		return append(rows, serverRows(servers, "the person who wrote")...)
	}

	var rows []toolRow
	personal := func(key, name, hint string, enabled bool, setup, how string) {
		r := toolRow{Key: key, Name: name, Hint: hint}
		if enabled {
			r.Ready, r.On = true, !off[key]
		} else {
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
		servers, _ = s.st.ThreadMCPServers(t)
	} else if all, err := s.st.ListMCPServers(owner.ID); err == nil {
		for _, srv := range all {
			if srv.Enabled {
				servers = append(servers, srv)
			}
		}
	}
	return append(rows, serverRows(servers, "you")...)
}

// groupRows lists the groups a personal chat can look into: every group its
// owner is in, and any group it is still connected to after the owner left
// (so the connection can be removed). preset is a group to tick on the home
// screen, where no chat exists yet.
func (s *Server) groupRows(owner *store.User, t *store.Thread, preset int64) []toolRow {
	if t != nil && t.IsGroup() {
		return nil
	}
	connected := map[int64]bool{}
	var rows []toolRow
	if t != nil {
		refs, _ := s.st.ThreadGroups(t.ID, owner.ID)
		for _, g := range refs {
			connected[g.ID] = true
			if !g.Member {
				rows = append(rows, toolRow{Key: groupKey(g.ID), Name: g.Title, Ready: true, On: true,
					Hint: "You are no longer in this group, so the AI cannot read it"})
			}
		}
	}
	mine, _ := s.st.UserGroups(owner.ID)
	for _, g := range mine {
		rows = append(rows, toolRow{Key: groupKey(g.ID), Name: g.Title, Ready: true, On: connected[g.ID] || g.ID == preset,
			Hint: "Group · the AI can read its memory and latest messages"})
	}
	return rows
}

// toolsData is what the menu template needs. preset is a group to connect on
// the home screen (0 = none).
func (s *Server) toolsData(owner *store.User, t *store.Thread, preset int64) map[string]any {
	rows := s.toolRows(owner, t)
	groups := s.groupRows(owner, t, preset)
	offCount, servers, groupsOn := 0, 0, 0
	for _, r := range rows {
		if r.Ready && !r.On {
			offCount++
		}
		if strings.HasPrefix(r.Key, "mcp:") {
			servers++
		}
	}
	for _, g := range groups {
		if g.On {
			groupsOn++
		}
	}
	data := map[string]any{"Tools": rows, "ToolGroups": groups, "ToolsOff": offCount, "ToolServers": servers,
		"GroupsOn": groupsOn, "Group": t != nil && t.IsGroup()}
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
	key, on := r.FormValue("key"), r.FormValue("on") == "1"
	if gid := parseGroupKey(key); gid > 0 {
		// Connecting a group: the store checks that this is a personal chat of
		// the owner and that they are a member of the group.
		if err := s.st.SetThreadGroup(t.ID, owner.ID, gid, on); err != nil {
			s.errorPage(w, r, http.StatusBadRequest, "You can only connect a group you are in, and only to a chat of your own.")
			return
		}
	} else {
		found := false
		for _, row := range s.toolRows(owner, t) {
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
	}
	if !isHX(r) {
		back(w, r, fmt.Sprintf("/t/%d", t.ID))
		return
	}
	data := s.toolsData(owner, t, 0)
	data["Changed"] = true
	s.renderPartial(w, r, "chat", "tools-body", data)
}

// newThreadTools stores the Tools menu of the home screen for a chat that was
// just created: every row that was set up but left unticked is switched off,
// and every ticked group is connected.
func (s *Server) newThreadTools(r *http.Request, owner *store.User, threadID int64) {
	if r.FormValue("tools_menu") != "1" { // a form without the menu: leave everything on
		return
	}
	ticked := map[string]bool{}
	for _, k := range r.Form["tool_on"] {
		ticked[k] = true
		if gid := parseGroupKey(k); gid > 0 {
			if err := s.st.SetThreadGroup(threadID, owner.ID, gid, true); err != nil {
				s.log.Warn("could not connect a group to the new chat", "thread", threadID, "group", gid, "err", err)
			}
		}
	}
	for _, row := range s.toolRows(owner, nil) {
		if row.Ready && !ticked[row.Key] {
			if err := s.st.SetThreadTool(threadID, row.Key, false); err != nil {
				s.log.Warn("could not switch a tool off for the new chat", "thread", threadID, "tool", row.Key, "err", err)
			}
		}
	}
}
