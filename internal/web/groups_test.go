package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// addModelNamed adds one more shared chat model and returns its id.
func (a *app) addModelNamed(name string) int64 {
	pid, err := a.st.CreateProvider(nil, "other", "http://127.0.0.1:1/v1", "k")
	if err != nil {
		a.t.Fatal(err)
	}
	a.st.CreateModel(pid, name, 8000, 0, 0, true)
	return a.st.ModelIDByName(pid, name)
}

// anonGet loads a page with no session at all, as someone without an account would.
func (a *app) anonGet(path string) (int, string, http.Header) {
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, err := c.Get(a.srv.URL + path)
	if err != nil {
		a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b), r.Header
}

// A group is made on its own page, joined by invite, and managed by its
// admin only. A personal chat cannot be joined at all.
func TestGroupPagesAndPermissions(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	carol := a.register("carol")
	model := a.addModel("http://127.0.0.1:1/v1")

	if page := alice.get("/groups/new"); !strings.Contains(page, "Start a group") || !strings.Contains(page, "Shared on this server") {
		t.Fatal("the new group page is incomplete")
	}
	if code, _, _ := alice.post("/groups", url.Values{"name": {"  "}, "model_id": {fmt.Sprint(model)}}); code != http.StatusBadRequest {
		t.Fatalf("a group without a name: %d", code)
	}
	if code, _, _ := alice.post("/groups", url.Values{"name": {"Team"}, "model_id": {"99999"}}); code != http.StatusBadRequest {
		t.Fatalf("a group with a model that does not exist: %d", code)
	}
	code, _, h := alice.post("/groups", url.Values{"name": {"Next project"}, "model_id": {fmt.Sprint(model)}, "instructions": {"Keep it short."}})
	if code != http.StatusSeeOther {
		t.Fatalf("create group: %d", code)
	}
	gid := threadIDFromLocation(t, h)
	g, _ := a.st.ThreadByID(gid)
	if !g.IsGroup() || g.Title != "Next project" || g.Instructions != "Keep it short." || g.OwnerUserID != alice.id {
		t.Fatalf("the group = %+v", g)
	}
	gpath := fmt.Sprintf("/t/%d", gid)
	solo, _ := a.st.CreateThread(alice.id, model)
	spath := fmt.Sprintf("/t/%d", solo)

	// The two kinds look different, and are listed apart.
	gpage := alice.get(gpath)
	for _, want := range []string{"Next project is ready", "Invite people", gpath + "/memory", ">Groups<", "Delete group"} {
		if !strings.Contains(gpage, want) {
			t.Errorf("the group page lacks %q", want)
		}
	}
	if strings.Contains(gpage, "agent-menu") {
		t.Error("a group offers to connect the admin's personal agents")
	}
	if spage := alice.get(spath); strings.Contains(spage, spath+"/memory") || !strings.Contains(spage, "agent-menu") {
		t.Error("a personal chat looks like a group")
	}

	// Only a group can be joined.
	if code, _, _ := alice.hxPost(spath+"/invites", url.Values{"role": {"member"}}); code != http.StatusBadRequest {
		t.Fatalf("an invite to a personal chat: %d", code)
	}
	if code, _, _ := alice.hxPost(gpath+"/invites", url.Values{"role": {"member"}}); code != 200 {
		t.Fatalf("an invite to a group: %d", code)
	}
	invites, _ := a.st.ListInvites(gid)
	if len(invites) != 1 {
		t.Fatalf("invites = %+v", invites)
	}
	if code, _, _ := bob.post("/invite/"+invites[0].Token, url.Values{}); code != http.StatusSeeOther || a.st.MemberRole(gid, bob.id) != "member" {
		t.Fatalf("joining the group: %d, role %q", code, a.st.MemberRole(gid, bob.id))
	}
	if carol.getStatus(gpath) != http.StatusNotFound || carol.getStatus(gpath+"/memory") != http.StatusNotFound {
		t.Fatal("someone outside the group can open it or its memory")
	}

	// The panel: the admin manages, a member only sees who is there.
	if panel := alice.get(gpath + "/panel"); !strings.Contains(panel, "Group settings") || !strings.Contains(panel, "The group's tools") || strings.Contains(panel, "Share a link") {
		t.Error("the admin's panel of a group is wrong")
	}
	if panel := bob.get(gpath + "/panel"); strings.Contains(panel, "Group settings") || strings.Contains(panel, "Invite people") || !strings.Contains(panel, "Leave group") {
		t.Error("a member's panel of a group is wrong")
	}
	if panel := alice.get(spath + "/panel"); !strings.Contains(panel, "Share a link") || strings.Contains(panel, "Invite people") || strings.Contains(panel, "People in this") {
		t.Error("the panel of a personal chat is wrong")
	}

	// The group's own tool servers: the admin adds them, and they are not among the admin's personal ones.
	server := url.Values{"name": {"web search"}, "url": {"http://127.0.0.1:1/mcp"}, "approval": {"ask"}}
	if code, _, _ := bob.hxPost(gpath+"/mcp", server); code != http.StatusForbidden {
		t.Fatalf("a member adding a tool server: %d", code)
	}
	if code, _, _ := alice.hxPost(spath+"/mcp", server); code != http.StatusNotFound {
		t.Fatalf("a group tool server on a personal chat: %d", code)
	}
	if code, body, _ := alice.hxPost(gpath+"/mcp", server); code != 200 || !strings.Contains(body, "web search") {
		t.Fatalf("the admin adding a tool server: %d", code)
	}
	servers, _ := a.st.ListGroupServers(gid)
	if mine, _ := a.st.ListMCPServers(alice.id); len(servers) != 1 || !servers[0].Asks() || len(mine) != 0 {
		t.Fatalf("group servers %+v, the admin's own %+v", servers, mine)
	}
	if strings.Contains(alice.get("/settings"), "web search") {
		t.Error("the group's tool server is listed in the admin's personal settings")
	}
	act := fmt.Sprintf("%s/mcp/%d", gpath, servers[0].ID)
	if code, _, _ := bob.hxPost(act, url.Values{"action": {"delete"}}); code != http.StatusForbidden {
		t.Fatalf("a member deleting a tool server: %d", code)
	}
	if code, _, _ := alice.hxPost(act, url.Values{"action": {"delete"}}); code != 200 {
		t.Fatalf("the admin deleting a tool server: %d", code)
	}
	if left, _ := a.st.ListGroupServers(gid); len(left) != 0 {
		t.Fatalf("after deleting: %+v", left)
	}

	// The admin can move the group to another model at any time; the model name at the top leads there.
	cheaper := a.addModelNamed("cheaper")
	if !strings.Contains(alice.get(gpath), "meta-model") || strings.Contains(bob.get(gpath), "meta-model") {
		t.Error("the model name should be a button for the admin only")
	}
	if code, _, _ := alice.hxPost(gpath+"/settings", url.Values{"title": {"Next project"}, "model_id": {fmt.Sprint(cheaper)}}); code != 200 {
		t.Fatalf("the admin changing the model: %d", code)
	}
	if g, _ := a.st.ThreadByID(gid); g.ModelID.Int64 != cheaper || !g.IsGroup() {
		t.Fatalf("after changing the model: %+v", g)
	}
	if !strings.Contains(bob.get(gpath), "cheaper") {
		t.Error("members do not see the new model's name")
	}

	// Settings are the admin's.
	if code, _, _ := bob.hxPost(gpath+"/settings", url.Values{"title": {"Hacked"}}); code != http.StatusForbidden {
		t.Fatalf("a member changing the group: %d", code)
	}
	if code, _, _ := bob.post(gpath+"/memory/settings", url.Values{"embed_model_id": {"0"}}); code != http.StatusForbidden {
		t.Fatalf("a member changing the memory's search: %d", code)
	}
}

// A group's memory is read by its members, written by nobody, and a fact can
// be deleted by the admin or by the person whose message it came from.
func TestGroupMemoryPage(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	carol := a.register("carol")
	gid, _ := a.st.CreateGroup(alice.id, 0, "Team")
	g, _ := a.st.ThreadByID(gid)
	inv, _ := a.st.CreateInvite(gid, alice.id, "member", 24*3600*1e9, 5)
	a.st.AcceptInvite(inv, bob.id)
	owner := store.GroupMemory(g)
	fromAlice, _ := a.st.CreateMemory(owner, store.MemoryInput{Content: "The launch is in March.", Source: store.MemoryFromAI, ThreadID: gid, SaidBy: alice.id})
	fromBob, _ := a.st.CreateMemory(owner, store.MemoryInput{Content: "Bob writes the tests.", About: []string{"Bob"}, Source: store.MemoryFromAI, ThreadID: gid, SaidBy: bob.id})
	a.st.CreateMemory(store.UserMemory(alice.id), store.MemoryInput{Content: "Alice's private note."})
	path := fmt.Sprintf("/t/%d/memory", gid)

	page := bob.get(path)
	for _, want := range []string{"The launch is in March.", "Bob writes the tests.", "from a message by <b>alice</b>", "Only the AI adds and changes facts"} {
		if !strings.Contains(page, want) {
			t.Errorf("the group's memory page lacks %q", want)
		}
	}
	if strings.Contains(page, "private note") {
		t.Fatal("a personal memory shows up in the group's memory")
	}
	if strings.Contains(alice.get("/memory"), "launch is in March") {
		t.Fatal("a group memory shows up in the admin's personal memory")
	}
	if strings.Contains(page, `name="content"`) || strings.Contains(page, "/edit") {
		t.Error("the page offers to add or edit a fact")
	}
	// A member may delete only what came from their own messages.
	if n := strings.Count(page, `aria-label="Delete this memory"`); n != 1 {
		t.Errorf("a member sees %d delete buttons, want 1", n)
	}
	if n := strings.Count(alice.get(path), `aria-label="Delete this memory"`); n != 2 {
		t.Errorf("the admin sees %d delete buttons, want 2", n)
	}
	if found := bob.get(path + "?q=launch"); !strings.Contains(found, "The launch is in March.") || strings.Contains(found, "Bob writes") {
		t.Error("search in the group's memory is wrong")
	}

	del := func(c *client, id int64) int {
		code, _, _ := c.hxPost(fmt.Sprintf("%s/%d/delete", path, id), url.Values{})
		return code
	}
	if code := del(carol, fromBob); code != http.StatusNotFound {
		t.Fatalf("someone outside the group deleting: %d", code)
	}
	if code := del(bob, fromAlice); code != http.StatusForbidden {
		t.Fatalf("a member deleting a fact from someone else's message: %d", code)
	}
	if code := del(bob, fromBob); code != 200 {
		t.Fatalf("a member deleting a fact from their own message: %d", code)
	}
	if code := del(alice, fromAlice); code != 200 {
		t.Fatalf("the admin deleting: %d", code)
	}
	if left, _ := a.st.ListMemories(owner, "", 0, 10); len(left) != 0 {
		t.Fatalf("left in the group's memory: %+v", left)
	}

	// The personal Memory page cannot reach into a group's memory, and the other way round.
	kept, _ := a.st.CreateMemory(owner, store.MemoryInput{Content: "Kept by the group.", Source: store.MemoryFromAI, SaidBy: alice.id})
	for _, c := range []*client{alice, bob} {
		if c.getStatus(fmt.Sprintf("/memory/m/%d", kept)) != http.StatusNotFound {
			t.Fatal("a group memory can be opened through the personal Memory page")
		}
		if code, _, _ := c.hxPost(fmt.Sprintf("/memory/m/%d", kept), url.Values{"content": {"planted"}}); code != http.StatusNotFound {
			t.Fatalf("editing a group memory through the personal Memory page: %d", code)
		}
	}
	private, _ := a.st.ListMemories(store.UserMemory(alice.id), "", 0, 1)
	if code := del(alice, private[0].ID); code != http.StatusNotFound {
		t.Fatalf("deleting a personal memory through a group: %d", code)
	}

	// A personal chat has no such page.
	solo, _ := a.st.CreateThread(alice.id, 0)
	if alice.getStatus(fmt.Sprintf("/t/%d/memory", solo)) != http.StatusNotFound {
		t.Fatal("a personal chat has a group memory page")
	}
}

// The read-only link of a personal chat: anyone who has it can read, nobody
// can write, it shows the chat as it was when last updated, and tool results
// only when the owner says so.
func TestReadOnlyLink(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	tid, _ := a.st.CreateThread(alice.id, 0)
	a.st.SetThreadTitle(tid, "Bread plan")
	run1, _ := a.st.EnqueueMessage(tid, alice.id, "how do I bake bread?", 10, nil)
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run1, Role: "assistant",
		ToolCalls: `[{"id":"c1","type":"function","function":{"name":"memory_search","arguments":"{\"query\":\"flour\"}"}}]`})
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run1, Role: "tool", Content: "TOOLSECRET likes rye", ToolCallID: "c1", ToolName: "memory_search"})
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run1, Role: "assistant", Content: "Mix flour and water."})
	a.st.FinishRun(run1, store.RunDone, "", 2)
	path := fmt.Sprintf("/t/%d", tid)

	if code, _, _ := a.anonGet("/s/nothing"); code != http.StatusNotFound {
		t.Fatalf("a link that does not exist: %d", code)
	}
	if code, _, _ := bob.hxPost(path+"/share", url.Values{}); code != http.StatusNotFound {
		t.Fatalf("someone else making a link: %d", code)
	}
	if code, body, _ := alice.hxPost(path+"/share", url.Values{}); code != 200 || !strings.Contains(body, "/s/") {
		t.Fatalf("making the link: %d", code)
	}
	sh, err := a.st.ShareOf(tid)
	if err != nil || len(sh.Token) != 32 || sh.ShowTools {
		t.Fatalf("the link = %+v, %v", sh, err)
	}
	link := "/s/" + sh.Token

	// No account needed. Text yes, tools no, nothing to send, not for search engines.
	code, page, h := a.anonGet(link)
	if code != 200 {
		t.Fatalf("opening the link without an account: %d", code)
	}
	for _, want := range []string{"Bread plan", "how do I bake bread?", "Mix flour and water.", "Read-only copy", "alice"} {
		if !strings.Contains(page, want) {
			t.Errorf("the shared page lacks %q", want)
		}
	}
	for _, not := range []string{"TOOLSECRET", "memory_search", "<form", "<textarea", "data-csrf", `id="sidebar"`} {
		if strings.Contains(page, not) {
			t.Errorf("the shared page contains %q", not)
		}
	}
	if !strings.Contains(h.Get("X-Robots-Tag"), "noindex") || h.Get("Cache-Control") != "no-store" {
		t.Errorf("headers of the shared page: %v", h)
	}
	// The chat itself stays closed to everyone else.
	if code, _, _ := a.anonGet(path); code == 200 {
		t.Fatal("the chat page opens without an account")
	}
	if bob.getStatus(path) != http.StatusNotFound || bob.getStatus(path+"/messages") != http.StatusNotFound {
		t.Fatal("another user can open a chat because it has a link")
	}
	if code, _, _ := bob.post(path+"/messages", url.Values{"content": {"hi"}}); code != http.StatusNotFound {
		t.Fatalf("writing into a shared chat: %d", code)
	}

	// What is written later stays private until the owner updates the link.
	png := []byte("\x89PNG\r\n\x1a\n not really a picture")
	att, err := a.st.SaveUpload(png, "image/png", "oven.png", store.KindImage, "")
	if err != nil {
		t.Fatal(err)
	}
	run2, _ := a.st.EnqueueMessage(tid, alice.id, "LATER and private", 10, []store.NewAttachment{att})
	a.st.CreateMessage(&store.Message{ThreadID: tid, RunID: run2, Role: "assistant", Content: "Noted."})
	a.st.FinishRun(run2, store.RunDone, "", 1)
	msgs, _ := a.st.RunMessages(run2)
	files, _ := a.st.AttachmentsByMessage([]int64{msgs[0].ID})
	file := fmt.Sprintf("%s/files/%d", link, files[msgs[0].ID][0].ID)
	if _, page, _ := a.anonGet(link); strings.Contains(page, "LATER and private") {
		t.Fatal("a message written after sharing is shown by the link")
	}
	if code, _, _ := a.anonGet(file); code != http.StatusNotFound {
		t.Fatalf("a file sent after sharing: %d", code)
	}
	if panel := alice.get(path + "/panel"); !strings.Contains(panel, "share-behind") {
		t.Error("the panel does not say that the link is behind the chat")
	}

	// Updated, this time with tool results: same address, newer content.
	alice.hxPost(path+"/share", url.Values{"show_tools": {"1"}})
	if again, _ := a.st.ShareOf(tid); again.Token != sh.Token || !again.ShowTools || again.UntilRun != run2 {
		t.Fatalf("after updating: %+v", again)
	}
	_, page, _ = a.anonGet(link)
	if !strings.Contains(page, "LATER and private") || !strings.Contains(page, "TOOLSECRET") || !strings.Contains(page, file) {
		t.Fatal("the updated link does not show the newer messages, the tool result and the picture")
	}
	if code, _, h := a.anonGet(file); code != 200 || h.Get("Content-Type") != "image/png" {
		t.Fatalf("the picture through the link: %d %s", code, h.Get("Content-Type"))
	}
	if code, _, _ := a.anonGet(fmt.Sprintf("/files/%d", files[msgs[0].ID][0].ID)); code == 200 {
		t.Fatal("the member-only file address opens without an account")
	}

	// The admin can turn read-only links off for the whole server: none works, none can be made.
	settings, _ := a.st.LoadSettings()
	settings.ShareLinks = false
	a.st.SaveSettings(settings)
	if code, _, _ := a.anonGet(link); code != http.StatusNotFound {
		t.Fatalf("a link while links are turned off: %d", code)
	}
	if code, _, _ := a.anonGet(file); code != http.StatusNotFound {
		t.Fatalf("a file while links are turned off: %d", code)
	}
	if code, _, _ := alice.hxPost(path+"/share", url.Values{}); code != http.StatusForbidden {
		t.Fatalf("making a link while links are turned off: %d", code)
	}
	if panel := alice.get(path + "/panel"); !strings.Contains(panel, "turned off on this server") {
		t.Error("the panel does not say that links are turned off")
	}
	settings.ShareLinks = true
	a.st.SaveSettings(settings)
	if code, _, _ := a.anonGet(link); code != 200 {
		t.Fatalf("the link after links were turned on again: %d", code)
	}

	// Stopped: gone for everyone.
	if code, _, _ := bob.hxPost(path+"/share/stop", url.Values{}); code != http.StatusNotFound {
		t.Fatalf("someone else stopping the link: %d", code)
	}
	alice.hxPost(path+"/share/stop", url.Values{})
	if code, _, _ := a.anonGet(link); code != http.StatusNotFound {
		t.Fatalf("a stopped link: %d", code)
	}
	if code, _, _ := a.anonGet(file); code != http.StatusNotFound {
		t.Fatalf("a file of a stopped link: %d", code)
	}

	// A group has no public link.
	gid, _ := a.st.CreateGroup(alice.id, 0, "Team")
	if code, _, _ := alice.hxPost(fmt.Sprintf("/t/%d/share", gid), url.Values{}); code != http.StatusBadRequest {
		t.Fatalf("a read-only link to a group: %d", code)
	}
	if _, err := a.st.SaveShare(gid, false); err == nil {
		t.Fatal("the store made a link for a group")
	}
}

// "Take to my chat": a member opens a chat of their own that is connected to
// the group and quotes the message. Only members can connect a group.
func TestTakeToMyChat(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	carol := a.register("carol")
	model := a.addModel("http://127.0.0.1:1/v1") // the home screen shows the message box only when a model exists
	gid, _ := a.st.CreateGroup(alice.id, model, "Team")
	inv, _ := a.st.CreateInvite(gid, alice.id, "member", 24*3600*1e9, 5)
	a.st.AcceptInvite(inv, bob.id)
	run, _ := a.st.EnqueueMessage(gid, alice.id, "We chose Go.\nSecond line.", 10, nil)
	a.st.CreateMessage(&store.Message{ThreadID: gid, RunID: run, Role: "assistant", Content: "Good choice."})
	a.st.FinishRun(run, store.RunDone, "", 1)
	msgs, _ := a.st.RunMessages(run)
	key := fmt.Sprintf("group:%d", gid)
	take := fmt.Sprintf("/?group=%d&quote=%d", gid, msgs[0].ID)

	// Every finished message of a group offers it, to members too.
	if page := bob.get(fmt.Sprintf("/t/%d", gid)); strings.Count(page, "Take to my chat") < 2 || !strings.Contains(page, fmt.Sprintf("/?group=%d&amp;quote=%d", gid, msgs[0].ID)) {
		t.Fatal("the group's messages do not offer Take to my chat")
	}
	home := bob.get(take)
	for _, want := range []string{"In the group “Team”, alice wrote:", "&gt; We chose Go.", "&gt; Second line.", `value="` + key + `" data-group checked`, "connected to the group <b>Team</b>"} {
		if !strings.Contains(home, want) {
			t.Errorf("the home screen after Take to my chat lacks %q", want)
		}
	}
	// Without it the group is listed but not ticked.
	if plain := bob.get("/"); !strings.Contains(plain, `value="`+key+`" data-group>`) || strings.Contains(plain, "We chose Go") {
		t.Error("the plain home screen should list the group unticked and quote nothing")
	}
	// Someone outside the group gets neither the quote nor the group.
	if outside := carol.get(take); strings.Contains(outside, "We chose Go") || strings.Contains(outside, key) {
		t.Fatal("someone outside the group can read a message through Take to my chat")
	}

	newest := func(u int64) int64 {
		list, _ := a.st.ListThreadsForUser(u)
		var id int64
		for _, th := range list {
			if !th.Group {
				id = max(id, th.ID)
			}
		}
		return id
	}
	refs := func(c *client) []store.GroupRef {
		list, _ := a.st.ThreadGroups(newest(c.id), c.id)
		return list
	}
	if code, _, _ := bob.post("/t", url.Values{"tools_menu": {"1"}, "tool_on": {key}, "model_id": {fmt.Sprint(model)}}); code != http.StatusSeeOther {
		t.Fatalf("starting the chat: %d", code)
	}
	if got := refs(bob); len(got) != 1 || got[0].ID != gid || !got[0].Member {
		t.Fatalf("the new chat's groups = %+v", got)
	}
	carol.post("/t", url.Values{"tools_menu": {"1"}, "tool_on": {key}, "model_id": {fmt.Sprint(model)}})
	if got := refs(carol); len(got) != 0 {
		t.Fatalf("someone outside the group connected a new chat to it: %+v", got)
	}

	// Later, in the Tools menu of the chat.
	chat := fmt.Sprintf("/t/%d", newest(bob.id))
	if page := bob.get(chat); !strings.Contains(page, "1 group connected") || !strings.Contains(page, "Your groups") {
		t.Error("the chat does not say that a group is connected")
	}
	if code, _, _ := bob.hxPost(chat+"/tools", url.Values{"key": {key}, "on": {"0"}}); code != 200 || len(refs(bob)) != 0 {
		t.Fatalf("disconnecting: %d, %+v", code, refs(bob))
	}
	if code, _, _ := bob.hxPost(chat+"/tools", url.Values{"key": {key}, "on": {"1"}}); code != 200 || len(refs(bob)) != 1 {
		t.Fatalf("connecting again: %d, %+v", code, refs(bob))
	}
	if code, _, _ := carol.hxPost(fmt.Sprintf("/t/%d/tools", newest(carol.id)), url.Values{"key": {key}, "on": {"1"}}); code != http.StatusBadRequest || len(refs(carol)) != 0 {
		t.Fatalf("someone outside the group connecting it: %d", code)
	}
	// A group cannot be connected to a group.
	if code, _, _ := alice.hxPost(fmt.Sprintf("/t/%d/tools", gid), url.Values{"key": {key}, "on": {"1"}}); code != http.StatusBadRequest {
		t.Fatalf("connecting a group to itself: %d", code)
	}
}

// A chat that was shared before groups existed says so to its owner, once.
func TestNoteOnAChatThatBecameAGroup(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	gid, _ := a.st.CreateGroup(alice.id, 0, "Old chat")
	inv, _ := a.st.CreateInvite(gid, alice.id, "member", 24*3600*1e9, 5)
	a.st.AcceptInvite(inv, bob.id)
	a.st.DB.Exec(`UPDATE threads SET group_notice = 1 WHERE id = ?`, gid) // as the migration leaves it
	path := fmt.Sprintf("/t/%d", gid)

	if !strings.Contains(alice.get(path), "This chat is now a group") {
		t.Fatal("the owner is not told")
	}
	if strings.Contains(bob.get(path), "This chat is now a group") {
		t.Error("a member is shown the owner's note")
	}
	if code, _, _ := bob.hxPost(path+"/notice", url.Values{}); code != http.StatusForbidden {
		t.Fatalf("a member dismissing the note: %d", code)
	}
	if code, _, _ := alice.hxPost(path+"/notice", url.Values{}); code != 200 {
		t.Fatalf("dismissing the note: %d", code)
	}
	if strings.Contains(alice.get(path), "This chat is now a group") {
		t.Fatal("the note comes back")
	}
}
