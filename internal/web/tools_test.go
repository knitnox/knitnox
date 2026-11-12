package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// The Tools menu: the owner switches things off for one chat; nobody else
// can; what is not set up cannot be switched; the cost note shows only once
// the chat has messages.
func TestToolsMenu(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	a.st.SaveMemorySettings(alice.id, store.MemorySettings{Enabled: true})
	if err := a.st.CreateMCPServer(alice.id, "Mail", "http://127.0.0.1:1/mcp", "", store.ApprovalAsk); err != nil {
		t.Fatal(err)
	}
	servers, _ := a.st.ListMCPServers(alice.id)
	mail := store.ToolServerKey(servers[0].ID)

	tid, _ := a.st.CreateThread(alice.id, 0)
	path := fmt.Sprintf("/t/%d", tid)
	set := func(c *client, key, on string) (int, string) {
		code, body, _ := c.hxPost(path+"/tools", url.Values{"key": {key}, "on": {on}})
		return code, body
	}

	page := alice.get(path)
	for _, want := range []string{`id="tools-body"`, "Memory", "Mail", "asks you before each tool", "Off for your account. Turn it on under Plan Book"} {
		if !strings.Contains(page, want) {
			t.Errorf("the chat page lacks %q", want)
		}
	}
	if strings.Contains(page, "tools-cost") {
		t.Error("the cost note is shown in a chat without messages")
	}
	if n := strings.Count(page, `id="tools-count"`); n != 1 {
		t.Errorf("%d elements with the id tools-count", n)
	}

	// Switch memory and the mail server off.
	code, body := set(alice, store.ToolMemory, "0")
	if code != 200 || !strings.Contains(body, ">1 off<") {
		t.Fatalf("memory off: %d %q", code, body)
	}
	set(alice, mail, "0")
	off := a.st.ThreadToolsOff(tid)
	if !off[store.ToolMemory] || !off[mail] || len(off) != 2 {
		t.Fatalf("stored: %v", off)
	}
	if page := alice.get(path); !strings.Contains(page, ">2 off<") {
		t.Error("the pill does not say 2 off")
	}
	set(alice, store.ToolMemory, "1")
	if off := a.st.ThreadToolsOff(tid); off[store.ToolMemory] || !off[mail] {
		t.Fatalf("after switching memory back on: %v", off)
	}

	// Not set up, made up, or someone else's server: refused, nothing stored.
	for _, key := range []string{store.ToolPlan, "everything", "mcp:99999", ""} {
		if code, _ := set(alice, key, "0"); code != http.StatusBadRequest {
			t.Errorf("key %q: %d, want 400", key, code)
		}
	}
	if off := a.st.ThreadToolsOff(tid); len(off) != 1 {
		t.Fatalf("refused requests stored something: %v", off)
	}

	// Someone outside the chat, and a member who is not the owner.
	if code, _ := set(bob, mail, "1"); code != http.StatusNotFound {
		t.Fatalf("outside the chat: %d", code)
	}
	inv, _ := a.st.CreateInvite(tid, alice.id, "member", 24*3600*1e9, 5)
	a.st.AcceptInvite(inv, bob.id)
	if code, _ := set(bob, mail, "1"); code != http.StatusForbidden {
		t.Fatalf("a member who is not the owner: %d", code)
	}
	if strings.Contains(bob.get(path), `id="tools-body"`) {
		t.Error("a member sees the Tools menu")
	}
	// In a group chat the personal groups cannot be switched: they are never used there.
	if code, _ := set(alice, store.ToolMemory, "0"); code != http.StatusBadRequest {
		t.Fatalf("memory in a group chat: %d", code)
	}
	if !strings.Contains(alice.get(path), "Not used in group chats") {
		t.Error("the menu does not say why memory is unavailable in a group chat")
	}

	// With messages in the chat, the menu warns about the cost; after a change it says so.
	runID, _ := a.st.EnqueueMessage(tid, alice.id, "hello", 10, nil)
	a.st.FinishRun(runID, store.RunDone, "", 1)
	if !strings.Contains(alice.get(path), "tools-cost") {
		t.Error("no cost note in a chat that has messages")
	}
	a.st.SetThreadMCP(tid, alice.id, []int64{servers[0].ID}) // shared with the group, so it can be switched
	if _, body := set(alice, mail, "1"); !strings.Contains(body, "Changed.") || !strings.Contains(body, "read the conversation again") {
		t.Fatalf("after a change mid-chat: %q", body)
	}
}

// On the home screen the choice is made before the chat exists and applies to its first message.
func TestToolsChosenBeforeTheFirstMessage(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	a.st.SaveMemorySettings(alice.id, store.MemorySettings{Enabled: true})
	a.st.SetPlanEnabled(alice.id, true)
	pid, _ := a.st.CreateProvider(nil, "shared", "http://127.0.0.1:1/v1", "k") // the home screen shows the message box only when a model exists
	a.st.CreateModel(pid, "chat-model", 100000, 0, 0, false)
	a.st.CreateMCPServer(alice.id, "Mail", "http://127.0.0.1:1/mcp", "", store.ApprovalAuto)
	servers, _ := a.st.ListMCPServers(alice.id)
	mail := store.ToolServerKey(servers[0].ID)

	home := alice.get("/")
	for _, want := range []string{`name="tools_menu"`, `value="memory" checked`, `value="plan" checked`, `value="` + mail + `" checked`} {
		if !strings.Contains(home, want) {
			t.Errorf("the home screen lacks %q", want)
		}
	}
	newest := func() int64 {
		list, _ := a.st.ListThreadsForUser(alice.id)
		var id int64
		for _, th := range list {
			id = max(id, th.ID)
		}
		return id
	}

	// Plan Book left ticked, memory and mail unticked.
	if code, _, _ := alice.post("/t", url.Values{"tools_menu": {"1"}, "tool_on": {store.ToolPlan}}); code != http.StatusSeeOther {
		t.Fatalf("new chat: %d", code)
	}
	off := a.st.ThreadToolsOff(newest())
	if !off[store.ToolMemory] || !off[mail] || off[store.ToolPlan] || len(off) != 2 {
		t.Fatalf("stored for the new chat: %v", off)
	}
	// A form without the menu (an older page still open): everything stays on.
	alice.post("/t", url.Values{})
	if off := a.st.ThreadToolsOff(newest()); len(off) != 0 {
		t.Fatalf("without the menu: %v", off)
	}
}
