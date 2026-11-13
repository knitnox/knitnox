package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"knitnox/internal/store"
)

// Groups and read-only links
//
// A chat is one of two kinds for its whole life (store/groups.go):
//
//	/groups/new          start a group: a page of its own, like a new agent
//	/t/{id}/memory       what a group's AI keeps for the group, for every member to read
//	/t/{id}/mcp          the group's own tool servers (its admin only)
//	/t/{id}/share        the read-only link of a personal chat (its owner only)
//	/s/{token}           what that link shows; the only chat page that needs no account
//
// "Share" always means the read-only link and "group" always means talking
// together. The two are never mixed: a personal chat cannot be joined and a
// group has no public link.

const (
	groupMemoryPage  = 30   // memories per page of a group's memory
	sharedMaxLoad    = 2000 // messages a read-only link shows at most
	takenQuoteLength = 4000 // characters of a message carried into "Take to my chat"
)

// loadGroup returns the group in the path if the current user is in it.
func (s *Server) loadGroup(w http.ResponseWriter, r *http.Request) (*store.Thread, string, bool) {
	t, role, ok := s.loadThread(w, r)
	if !ok {
		return nil, "", false
	}
	if !t.IsGroup() {
		s.errorPage(w, r, http.StatusNotFound, "That is a personal chat, not a group.")
		return nil, "", false
	}
	return t, role, true
}

// loadOwnedGroup is loadGroup for things only the group's admin may do.
func (s *Server) loadOwnedGroup(w http.ResponseWriter, r *http.Request) (*store.Thread, bool) {
	t, role, ok := s.loadGroup(w, r)
	if !ok {
		return nil, false
	}
	if role != "owner" {
		s.errorPage(w, r, http.StatusForbidden, "Only the group's admin can do that.")
		return nil, false
	}
	return t, true
}

// ---- starting a group ----

// embedChoice turns the embedding model picked in a form into a group's
// settings. 0 means the server's model. The model must be an embedding model
// the group's admin may use: a shared one or one of their own.
func (s *Server) embedChoice(ownerID, modelID int64) (store.GroupSettings, bool) {
	if modelID <= 0 {
		return store.GroupSettings{}, true
	}
	opts, _ := s.st.KindModelOptions(ownerID, store.ModelEmbedding)
	if findOption(opts, modelID) == nil {
		return store.GroupSettings{}, false
	}
	m, err := s.st.ModelByID(modelID)
	if err != nil {
		return store.GroupSettings{}, false
	}
	return store.GroupSettings{EmbedProviderID: m.ProviderID, EmbedModel: m.Name}, true
}

// serverEmbedModel is the embedding model the admin set for everyone ("" = none).
func (s *Server) serverEmbedModel() string {
	st, err := s.st.LoadSettings()
	if err != nil || st.EmbedProviderID == 0 {
		return ""
	}
	return st.EmbedModel
}

func (s *Server) groupNewPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	opts, _ := s.st.ModelOptions(u.ID)
	embed, _ := s.st.KindModelOptions(u.ID, store.ModelEmbedding)
	s.render(w, r, "group", map[string]any{"Tab": "new", "ModelOptions": opts, "DefaultModel": s.defaultModel(u.ID, opts),
		"EmbedOptions": embed, "EmbedModelID": int64(0), "ServerEmbed": s.serverEmbedModel()})
}

func (s *Server) groupCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := strings.Join(strings.Fields(r.FormValue("name")), " ")
	if rs := []rune(name); len(rs) > 80 {
		name = string(rs[:80])
	}
	if name == "" {
		s.errorPage(w, r, http.StatusBadRequest, "Give the group a name.")
		return
	}
	opts, _ := s.st.ModelOptions(u.ID)
	modelID := formInt(r, "model_id", 0)
	if findOption(opts, modelID) == nil {
		s.errorPage(w, r, http.StatusBadRequest, "Pick a model for the group.")
		return
	}
	gs, ok := s.embedChoice(u.ID, formInt(r, "embed_model_id", 0))
	if !ok {
		s.errorPage(w, r, http.StatusBadRequest, "Pick an embedding model from the list.")
		return
	}
	instr := strings.TrimSpace(r.FormValue("instructions"))
	if rs := []rune(instr); len(rs) > 4000 {
		instr = string(rs[:4000])
	}
	id, err := s.st.CreateGroup(u.ID, modelID, name)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if instr != "" {
		if err := s.st.SetThreadInstructions(id, instr); err != nil {
			s.log.Warn("could not save the new group's instructions", "thread", id, "err", err)
		}
	}
	if gs.EmbedProviderID > 0 {
		if err := s.st.SaveGroupSettings(id, gs); err != nil {
			s.log.Warn("could not save the new group's embedding model", "thread", id, "err", err)
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/t/%d", id), http.StatusSeeOther)
}

// groupNoticeDismiss records that the admin read the note shown on a chat
// that was shared before groups existed.
func (s *Server) groupNoticeDismiss(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedGroup(w, r)
	if !ok {
		return
	}
	if err := s.st.ClearGroupNotice(t.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if isHX(r) || isFetch(r) {
		w.WriteHeader(http.StatusOK) // the note is swapped for nothing
		return
	}
	back(w, r, fmt.Sprintf("/t/%d", t.ID))
}

// ---- a group's memory ----

// groupMemView is one fact of a group's memory with what the viewer may do.
type groupMemView struct {
	store.Memory
	CanDelete bool
}

// canDeleteGroupMemory: the admin may delete any fact; a member the facts
// that came from their own messages. Nobody can add or edit one: only the
// group's AI writes to this memory.
func canDeleteGroupMemory(role string, userID int64, m *store.Memory) bool {
	return role == "owner" || (role == "member" && m.SaidBy == userID)
}

func (s *Server) groupMemory(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadGroup(w, r)
	if !ok {
		return
	}
	u := currentUser(r)
	owner := store.GroupMemory(t)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if rs := []rune(q); len(rs) > 200 {
		q = string(rs[:200])
	}
	ev, key := embedView{}, ""
	if em := s.eng.MemoryEmbedder(owner); em != nil {
		ev, key = embedView{On: true, Model: em.Model, Provider: em.Provider, Personal: em.Personal}, em.Key
	}
	stats := s.st.MemoryStats(owner, key)
	if ev.On {
		ev.Pending = stats.Memories - stats.Indexed
	}
	data := map[string]any{"Tab": "memory", "Thread": t, "Role": role, "Query": q, "Stats": stats, "Embed": ev,
		"Notice": r.URL.Query().Get("done"), "MemoryOn": !s.st.ThreadToolsOff(t.ID)[store.ToolMemory], "EmbedModelID": int64(0)}

	var mems []store.Memory
	var err error
	if q != "" {
		var smart bool
		mems, smart, err = s.eng.SearchMemory(r.Context(), owner, q, memorySearchSize)
		data["Smart"] = smart
	} else {
		before, _ := parseInt(r.URL.Query().Get("before"))
		mems, err = s.st.ListMemories(owner, "", before, groupMemoryPage+1)
		if len(mems) > groupMemoryPage {
			mems = mems[:groupMemoryPage]
			data["Older"] = fmt.Sprintf("/t/%d/memory?%s", t.ID, url.Values{"before": {fmt.Sprint(mems[len(mems)-1].ID)}}.Encode())
		}
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	views := make([]groupMemView, len(mems))
	for i := range mems {
		views[i] = groupMemView{mems[i], canDeleteGroupMemory(role, u.ID, &mems[i])}
	}
	data["Memories"] = views
	if role == "owner" { // the admin chooses how the group's memory is searched
		data["EmbedOptions"], _ = s.st.KindModelOptions(u.ID, store.ModelEmbedding)
		gs, _ := s.st.GroupSettingsFor(t.ID)
		if gs.EmbedProviderID > 0 {
			data["EmbedModelID"] = s.st.ModelIDByName(gs.EmbedProviderID, gs.EmbedModel)
		}
		data["ServerEmbed"] = s.serverEmbedModel()
	}
	s.render(w, r, "group", data)
}

func (s *Server) groupMemoryDelete(w http.ResponseWriter, r *http.Request) {
	t, role, ok := s.loadGroup(w, r)
	if !ok {
		return
	}
	owner := store.GroupMemory(t)
	m, err := s.st.MemoryByID(owner, pathID(r, "mid"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "That memory is not in this group (any more).")
		return
	}
	if !canDeleteGroupMemory(role, currentUser(r).ID, m) {
		s.errorPage(w, r, http.StatusForbidden, "You can delete what came from your own messages. The group's admin can delete the rest.")
		return
	}
	if err := s.st.DeleteMemory(owner, m.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, r, err)
		return
	}
	if isHX(r) || isFetch(r) {
		w.WriteHeader(http.StatusOK) // the card is swapped for nothing
		return
	}
	back(w, r, fmt.Sprintf("/t/%d/memory", t.ID))
}

// groupMemorySettings saves the embedding model of a group's memory.
func (s *Server) groupMemorySettings(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedGroup(w, r)
	if !ok {
		return
	}
	gs, ok := s.embedChoice(t.OwnerUserID, formInt(r, "embed_model_id", 0))
	if !ok {
		s.errorPage(w, r, http.StatusBadRequest, "Pick an embedding model from the list.")
		return
	}
	if err := s.st.SaveGroupSettings(t.ID, gs); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.eng.IndexMemoriesSoon(store.GroupMemory(t))
	http.Redirect(w, r, fmt.Sprintf("/t/%d/memory?done=search", t.ID), http.StatusSeeOther)
}

// ---- a group's own tool servers ----

func (s *Server) groupMCPCreate(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedGroup(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	u := strings.TrimSpace(r.FormValue("url"))
	if name == "" || !validURL(u) {
		s.flash(w, r, "Name and a valid http(s) URL are required. Only remote (HTTP) MCP servers are supported.")
		return
	}
	if err := s.st.CreateGroupMCPServer(t, name, u, strings.TrimSpace(r.FormValue("headers")), r.FormValue("approval")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPanel(w, r, t, "owner")
}

// groupMCPAction enables or disables, switches the approval mode of, or
// deletes one of the group's tool servers.
func (s *Server) groupMCPAction(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadOwnedGroup(w, r)
	if !ok {
		return
	}
	srv, err := s.st.GroupServer(t.ID, pathID(r, "sid"))
	if err != nil {
		s.flash(w, r, "That server no longer exists.")
		return
	}
	switch r.FormValue("action") {
	case "toggle":
		err = s.st.ToggleMCPServer(srv.UserID, srv.ID)
	case "approval":
		mode := store.ApprovalAsk
		if srv.Asks() {
			mode = store.ApprovalAuto
		}
		err = s.st.SetMCPApproval(srv.UserID, srv.ID, mode)
	case "delete":
		err = s.st.DeleteMCPServer(srv.UserID, srv.ID)
	default:
		err = errors.New("unknown action")
	}
	if err != nil {
		s.flash(w, r, "Could not update the server: "+err.Error())
		return
	}
	s.renderPanel(w, r, t, "owner")
}

// ---- "Take to my chat" ----

// takenView is what the home screen shows when a member takes a message of a
// group to a chat of their own: the group is ticked in the Tools menu and the
// message is quoted in the box, ready for them to say what to do with it.
type takenView struct {
	Group int64 // 0 = nothing was taken
	Title string
	Draft string
}

// takenFromGroup reads ?group= and ?quote= of the home screen. The group
// must be one the user is in, and the message must be in that group.
func (s *Server) takenFromGroup(r *http.Request, u *store.User) takenView {
	gid, _ := parseInt(r.URL.Query().Get("group"))
	if gid <= 0 {
		return takenView{}
	}
	t, err := s.st.ThreadByID(gid)
	if err != nil || !t.IsGroup() || s.st.MemberRole(t.ID, u.ID) == "" {
		return takenView{}
	}
	v := takenView{Group: t.ID, Title: t.Title}
	mid, _ := parseInt(r.URL.Query().Get("quote"))
	m, err := s.st.ThreadMessage(t.ID, mid)
	if mid <= 0 || err != nil || m.Role == "tool" || strings.TrimSpace(m.Content) == "" {
		return v
	}
	who := "the AI answered"
	if m.Role == "user" {
		who = m.AuthorName + " wrote"
		if m.AuthorUserID.Valid && m.AuthorUserID.Int64 == u.ID {
			who = "I wrote"
		}
	}
	text := strings.TrimSpace(m.Content)
	if rs := []rune(text); len(rs) > takenQuoteLength {
		text = string(rs[:takenQuoteLength]) + " …"
	}
	v.Draft = fmt.Sprintf("In the group “%s”, %s:\n> %s\n\n", t.Title, who, strings.ReplaceAll(text, "\n", "\n> "))
	return v
}

// ---- the read-only link of a personal chat ----

// loadShareable returns a personal chat the current user owns. Groups and
// agents have no read-only link.
func (s *Server) loadShareable(w http.ResponseWriter, r *http.Request) (*store.Thread, bool) {
	t, ok := s.loadOwnedThread(w, r)
	if !ok {
		return nil, false
	}
	if a, _ := s.st.AgentByThread(t.ID); t.IsGroup() || a != nil {
		s.errorPage(w, r, http.StatusBadRequest, "Only a personal chat can be shared with a read-only link.")
		return nil, false
	}
	return t, true
}

// shareLinksOn reports whether read-only links are allowed on this server (Admin → Settings).
func (s *Server) shareLinksOn() bool {
	st, err := s.st.LoadSettings()
	return err == nil && st.ShareLinks
}

// shareSave makes the link, or brings it up to date with the chat as it is now.
func (s *Server) shareSave(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadShareable(w, r)
	if !ok {
		return
	}
	if !s.shareLinksOn() {
		s.errorPage(w, r, http.StatusForbidden, "Read-only links are turned off on this server.")
		return
	}
	if _, err := s.st.SaveShare(t.ID, r.FormValue("show_tools") == "1"); err != nil {
		s.serverError(w, r, err)
		return
	}
	u := currentUser(r)
	s.st.Audit(u.ID, u.Username, "chat.share_link", t.Title, "anyone with the link can read the chat")
	if !isHX(r) {
		back(w, r, fmt.Sprintf("/t/%d", t.ID))
		return
	}
	s.renderPanel(w, r, t, "owner")
}

func (s *Server) shareStop(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadShareable(w, r)
	if !ok {
		return
	}
	if err := s.st.DeleteShare(t.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if !isHX(r) {
		back(w, r, fmt.Sprintf("/t/%d", t.ID))
		return
	}
	s.renderPanel(w, r, t, "owner")
}

// sharedMsg is one message as a read-only link shows it.
type sharedMsg struct {
	store.Message
	Calls  []callView
	Images []store.Attachment
	Files  []store.Attachment
}

// loadShare finds the link in the path. An unknown or stopped link is "not
// found", with nothing that tells the two apart.
func (s *Server) loadShare(r *http.Request) (*store.ThreadShare, *store.Thread, bool) {
	if !s.shareLinksOn() { // the admin turned links off: none of them works, whoever made it
		return nil, nil, false
	}
	sh, err := s.st.ShareByToken(r.PathValue("tok"))
	if err != nil {
		return nil, nil, false
	}
	t, err := s.st.ThreadByID(sh.ThreadID)
	if err != nil || t.IsGroup() {
		return nil, nil, false
	}
	return sh, t, true
}

// sharedPage shows a personal chat to anyone who has its link. It needs no
// account and offers nothing to send: the page is a copy to read. It shows
// the chat as it was when the owner last updated the link, and tool calls
// only if the owner chose to show them.
func (s *Server) sharedPage(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Cache-Control", "no-store")
	sh, t, ok := s.loadShare(r)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		s.render(w, r, "shared", map[string]any{"Gone": true})
		return
	}
	msgs, err := s.st.SharedMessages(sh, sharedMaxLoad)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	var ids []int64
	for _, m := range msgs {
		if m.Role != "tool" {
			ids = append(ids, m.ID)
		}
	}
	atts, _ := s.st.AttachmentsByMessage(ids)
	views := make([]sharedMsg, 0, len(msgs))
	for _, m := range msgs {
		if m.Status == store.MsgStreaming {
			continue
		}
		v := sharedMsg{Message: m}
		for _, a := range atts[m.ID] {
			switch {
			case a.IsImage():
				v.Images = append(v.Images, a)
			case a.IsAudio(): // recordings are made for one listener; not part of the copy
			default:
				v.Files = append(v.Files, a)
			}
		}
		if sh.ShowTools && m.ToolCalls != "" {
			var calls []struct {
				Function struct{ Name, Arguments string } `json:"function"`
			}
			json.Unmarshal([]byte(m.ToolCalls), &calls)
			for _, c := range calls {
				v.Calls = append(v.Calls, callView{Name: c.Function.Name, Args: c.Function.Arguments})
			}
		}
		if m.Role == "assistant" && strings.TrimSpace(m.Content) == "" && len(v.Calls) == 0 && len(v.Images) == 0 {
			continue // a step that only called tools, and tools are not shown
		}
		views = append(views, v)
	}
	owner := ""
	if u, err := s.st.UserByID(t.OwnerUserID); err == nil {
		owner = u.Username
	}
	s.render(w, r, "shared", map[string]any{"Title": t.Title, "Owner": owner, "Share": sh, "Messages": views})
}

// sharedFile serves a file of a message the link shows.
func (s *Server) sharedFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	sh, _, ok := s.loadShare(r)
	id := pathID(r, "aid")
	if !ok || !s.st.SharedAttachment(sh, id) {
		http.NotFound(w, r)
		return
	}
	a, err := s.st.AttachmentByID(id)
	if err != nil || a.IsAudio() {
		http.NotFound(w, r)
		return
	}
	s.sendAttachment(w, r, a, "private, max-age=3600")
}

// sendAttachment streams a stored file. Documents are sent as downloads with
// a safe content type so an uploaded HTML file can't run.
func (s *Server) sendAttachment(w http.ResponseWriter, r *http.Request, a *store.Attachment, cache string) {
	f, err := os.Open(s.st.AttachmentPath(a))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w.Header().Set("Cache-Control", cache)
	if a.IsImage() || a.IsAudio() {
		w.Header().Set("Content-Type", a.Mime)
		w.Header().Set("Content-Disposition", "inline")
	} else {
		ct := a.Mime
		if ct == "text/html" || ct == "text/xml" || ct == "application/xml" {
			ct = "text/plain"
		}
		w.Header().Set("Content-Type", ct+"; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFileName(a.OrigName)))
	}
	http.ServeContent(w, r, "", info.ModTime(), f)
}
