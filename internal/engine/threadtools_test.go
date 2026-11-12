package engine

import (
	"strings"
	"testing"

	"knitnox/internal/store"
)

// What the owner switches off in a chat's Tools menu is not offered to the
// AI in that chat, and other chats are not affected.
func TestToolsSwitchedOffForOneChat(t *testing.T) {
	llmSrv, seen := scriptLLM(t, func(llmRequest) (string, string, string) { return "ok", "", "" })
	mp := newMediaProvider(t)
	f := setup(t, llmSrv.URL)
	mcpSrv := fakeMCP(t)
	if err := f.st.CreateMCPServer(f.owner, "lookup", mcpSrv.URL, "", store.ApprovalAuto); err != nil {
		t.Fatal(err)
	}
	servers, _ := f.st.ListMCPServers(f.owner)
	serverKey := store.ToolServerKey(servers[0].ID)
	f.st.SetFeedEnabled(f.owner, true)
	f.st.SetPlanEnabled(f.owner, true)
	f.st.SaveMemorySettings(f.owner, store.MemorySettings{Enabled: true})
	pid, _ := f.st.CreateProvider(nil, "media", mp.URL, "k")
	f.st.CreateModelKind(pid, "img-1", store.ModelImage, 0, 0, 0, false)

	chat, _ := f.st.CreateThread(f.owner, f.modelID)
	other, _ := f.st.CreateThread(f.owner, f.modelID)
	offered := func(threadID int64) llmRequest {
		t.Helper()
		if r := waitRun(t, f.st, mustSubmit(t, f, threadID, f.owner, "hello")); r.Status != store.RunDone {
			t.Fatalf("run = %+v", r)
		}
		return seen()[len(seen())-1]
	}
	has := func(r llmRequest, part string) bool {
		for _, name := range r.Tools {
			if strings.Contains(name, part) {
				return true
			}
		}
		return false
	}
	groups := map[string]string{ // switch -> a tool that belongs to it
		store.ToolMemory: "memory_search", store.ToolPlan: "plan_list", store.ToolFeed: "feed_post",
		store.ToolPictures: "make_picture", serverKey: "_lookup",
	}

	all := offered(chat)
	for key, tool := range groups {
		if !has(all, tool) {
			t.Fatalf("with nothing switched off, %s (%s) is not offered: %v", tool, key, all.Tools)
		}
	}

	// One at a time: only that group goes.
	for key, tool := range groups {
		if err := f.st.SetThreadTool(chat, key, false); err != nil {
			t.Fatal(err)
		}
		r := offered(chat)
		if has(r, tool) {
			t.Errorf("%s is switched off but %s is still offered", key, tool)
		}
		for otherKey, otherTool := range groups {
			if otherKey != key && !has(r, otherTool) {
				t.Errorf("switching %s off also took %s away", key, otherTool)
			}
		}
		f.st.SetThreadTool(chat, key, true)
	}

	// Everything off: no tools at all, and the prompt no longer explains them.
	for key := range groups {
		f.st.SetThreadTool(chat, key, false)
	}
	none := offered(chat)
	if len(none.Tools) != 0 {
		t.Fatalf("everything is off but these are offered: %v", none.Tools)
	}
	for _, word := range []string{"make_picture", "## Feed", "memory_search", "plan_list"} {
		if strings.Contains(none.System, word) {
			t.Errorf("the system prompt still mentions %q", word)
		}
	}
	if r := offered(other); !has(r, "memory_search") || !has(r, "_lookup") || !has(r, "make_picture") {
		t.Fatalf("another chat lost its tools: %v", r.Tools)
	}

	if err := f.st.SetThreadTool(chat, "mcp:abc", false); err != store.ErrBadToolKey {
		t.Fatalf("a made-up key: %v", err)
	}
	if err := f.st.SetThreadTool(chat, "everything", false); err != store.ErrBadToolKey {
		t.Fatalf("a made-up group: %v", err)
	}
}

// Nobody has to choose a model first: the server's is used, and someone with
// only their own model gets that one.
func TestMediaModelDefaults(t *testing.T) {
	mp := newMediaProvider(t)
	f := setup(t, mp.URL)
	bob, _ := f.st.CreateUser("bob", "h", "user")
	model := func(user int64, kind string) string {
		m, _, err := f.st.FeedMediaModel(user, kind)
		if err != nil {
			t.Fatal(err)
		}
		if m == nil {
			return ""
		}
		return m.Name
	}
	if got := model(bob, store.MediaImage); got != "" {
		t.Fatalf("with nothing set up: %q", got)
	}

	// Bob adds his own picture model: it works without visiting the Feed settings.
	own, _ := f.st.CreateProvider(&bob, "bob's", mp.URL, "k")
	f.st.CreateModelKind(own, "bob-img", store.ModelImage, 0, 0, 0, false)
	if got := model(bob, store.MediaImage); got != "bob-img" {
		t.Fatalf("only an own model: %q", got)
	}
	if got := model(f.owner, store.MediaImage); got != "" {
		t.Fatalf("someone else gets bob's model: %q", got)
	}

	// The admin shares models: everyone has them at once; bob's default becomes the server's.
	shared := sharedProvider(t, f)
	f.st.CreateModelKind(shared, "server-img", store.ModelImage, 0, 0, 0, false)
	f.st.CreateModelKind(shared, "server-tts", store.ModelVoice, 0, 0, 0, false)
	for _, u := range []int64{f.owner, bob} {
		if model(u, store.MediaImage) != "server-img" || model(u, store.MediaVoice) != "server-tts" {
			t.Fatalf("user %d: picture %q, voice %q", u, model(u, store.MediaImage), model(u, store.MediaVoice))
		}
		if pic, voice := f.eng.CanMakeInChat(u); !pic || !voice {
			t.Fatalf("user %d cannot use the shared models in a chat", u)
		}
	}

	// Bob picks his own again, or turns pictures off: both are respected.
	bobImg := f.st.ModelIDByName(own, "bob-img")
	f.st.SaveFeedModels(bob, store.FeedModels{Image: bobImg})
	if got := model(bob, store.MediaImage); got != "bob-img" {
		t.Fatalf("after choosing his own: %q", got)
	}
	f.st.SaveFeedModels(bob, store.FeedModels{Image: -1})
	if got := model(bob, store.MediaImage); got != "" {
		t.Fatalf("after turning pictures off: %q", got)
	}
}
