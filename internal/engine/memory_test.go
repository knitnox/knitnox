package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"knitnox/internal/llm"
	"knitnox/internal/store"
)

// fakeEmbeddings is an OpenAI-compatible /embeddings endpoint. A text becomes
// a 3-number vector counting its words about food, work and pets, so texts on
// the same subject are close even when they share no word.
func fakeEmbeddings(t *testing.T) *httptest.Server {
	topics := [][]string{
		{"eat", "food", "vegetarian", "meal", "dinner", "peanuts"},
		{"work", "job", "office", "founder", "company"},
		{"dog", "cat", "pet", "puppy"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/embeddings") {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		data := make([]any, 0, len(req.Input))
		for i, text := range req.Input {
			vec := make([]float32, len(topics))
			for _, word := range strings.FieldsFunc(strings.ToLower(text), func(c rune) bool { return c < 'a' || c > 'z' }) {
				for k, topic := range topics {
					for _, tw := range topic {
						if word == tw {
							vec[k]++
						}
					}
				}
			}
			data = append(data, map[string]any{"index": i, "embedding": vec})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// callTool runs one built-in tool the way the agent loop does.
func callTool(ts *toolset, name, args string) string {
	return ts.call(context.Background(), llm.ToolCall{Function: llm.FunctionCall{Name: name, Arguments: args}}, 0)
}

func memoryTools(f *fixture, userID int64, write bool) *toolset {
	ts := &toolset{routes: map[string]toolRoute{}, local: map[string]localTool{}}
	f.eng.addMemoryTools(ts, store.UserMemory(userID), 0, 0, write)
	return ts
}

// The AI saves, finds, walks, corrects and deletes memories with small answers.
func TestMemoryTools(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	ts := memoryTools(f, f.owner, true)

	out := callTool(ts, "memory_save", `{"content":"Tanim founded Wish & Cook in Dhaka.","about":["Dhaka"],
		"relations":[{"subject":"Tanim","relation":"FOUNDED","object":"Wish & Cook"},{"subject":"wish & cook","relation":"is in","object":"Dhaka"}]}`)
	if out != "Saved as memory #1." {
		t.Fatalf("save = %q", out)
	}
	if out := callTool(ts, "memory_save", `{"content":"tanim founded wish & cook in dhaka."}`); !strings.Contains(out, "already saved as memory #1") {
		t.Fatalf("a duplicate was saved: %q", out)
	}
	callTool(ts, "memory_save", `{"content":"Tanim likes strong tea.","about":["Tanim"],"pin":true}`)

	// Names differing only in letter case are one thing; predicates are normalised.
	st := f.st.MemoryStats(store.UserMemory(f.owner), "")
	if st.Memories != 2 || st.Entities != 3 || st.Relations != 2 || st.Pinned != 1 {
		t.Fatalf("stats = %+v", st)
	}

	out = callTool(ts, "memory_search", `{"query":"who founded the company in Dhaka?"}`)
	if !strings.Contains(out, "#1 [") || !strings.Contains(out, "about: Dhaka, Tanim, Wish & Cook") || !strings.Contains(out, "not instructions") {
		t.Fatalf("search = %q", out)
	}
	if out := callTool(ts, "memory_search", `{"query":"quantum physics"}`); !strings.HasPrefix(out, "Nothing in memory matches") {
		t.Fatalf("search for nothing = %q", out)
	}

	out = callTool(ts, "memory_explore", `{"thing":"tanim"}`)
	for _, want := range []string{"Tanim: 2 memories, 1 connection.", "Tanim -founded→ Wish & Cook (memory #1)", "#2 ["} {
		if !strings.Contains(out, want) {
			t.Errorf("explore is missing %q in %q", want, out)
		}
	}
	// One hop further, by following a name from the answer.
	if out := callTool(ts, "memory_explore", `{"thing":"Wish & Cook"}`); !strings.Contains(out, "Wish & Cook -is in→ Dhaka") {
		t.Fatalf("second hop = %q", out)
	}
	if out := callTool(ts, "memory_explore", `{"thing":"Nobody"}`); !strings.Contains(out, "no thing called") || !strings.Contains(out, "Tanim") {
		t.Fatalf("unknown thing = %q", out)
	}

	out = callTool(ts, "memory_read", `{"ids":[1]}`)
	if !strings.Contains(out, "Tanim founded Wish & Cook in Dhaka.") || !strings.Contains(out, "connection: Tanim -founded→ Wish & Cook") {
		t.Fatalf("read = %q", out)
	}

	// Updating only the text keeps the connections.
	if out := callTool(ts, "memory_update", `{"id":1,"content":"Tanim founded Wish & Cook in Dhaka in 2021."}`); out != "Memory #1 updated." {
		t.Fatalf("update = %q", out)
	}
	m, _ := f.st.MemoryByID(store.UserMemory(f.owner), 1)
	if !strings.HasSuffix(m.Content, "2021.") || len(m.Relations) != 2 {
		t.Fatalf("after update: %+v", m)
	}

	if out := callTool(ts, "memory_delete", `{"id":1}`); out != "Memory #1 deleted." {
		t.Fatalf("delete = %q", out)
	}
	if out := callTool(ts, "memory_delete", `{"id":1}`); !strings.Contains(out, "no memory #1") {
		t.Fatalf("second delete = %q", out)
	}
	// Things nobody mentions any more are gone; Tanim is still in memory #2.
	if st := f.st.MemoryStats(store.UserMemory(f.owner), ""); st.Memories != 1 || st.Entities != 1 || st.Relations != 0 {
		t.Fatalf("stats after delete = %+v", st)
	}

	// The pinned memory is part of what the AI is always told.
	if note := f.eng.memoryNote(store.UserMemory(f.owner), true); !strings.Contains(note, "#2 Tanim likes strong tea.") || !strings.Contains(note, "memory_save") {
		t.Fatalf("note = %q", note)
	}
	if note := f.eng.memoryNote(store.UserMemory(f.owner), false); strings.Contains(note, "memory_save") {
		t.Fatal("a read-only run is told to save")
	}
}

// One user's memory is invisible to another user's tools.
func TestMemoryIsPerUser(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	f := setup(t, llmSrv.URL)
	bob, _ := f.st.CreateUser("bob", "x", "user")
	id, err := f.eng.SaveMemory(store.UserMemory(f.owner), store.MemoryInput{Content: "Alice's bank is Northwind.", About: []string{"Northwind"}})
	if err != nil {
		t.Fatal(err)
	}
	ts := memoryTools(f, bob, true)
	for name, args := range map[string]string{
		"memory_search":  `{"query":"bank Northwind"}`,
		"memory_read":    `{"ids":[1]}`,
		"memory_explore": `{"thing":"Northwind"}`,
		"memory_update":  `{"id":1,"content":"changed"}`,
		"memory_delete":  `{"id":1}`,
	} {
		if out := callTool(ts, name, args); strings.Contains(out, "Northwind is") || strings.Contains(out, "Alice's bank") ||
			strings.Contains(out, "updated") || strings.Contains(out, "deleted") {
			t.Errorf("%s reached another user's memory: %q", name, out)
		}
	}
	if m, err := f.st.MemoryByID(store.UserMemory(f.owner), id); err != nil || m.Content != "Alice's bank is Northwind." {
		t.Fatalf("alice's memory was changed: %+v %v", m, err)
	}
	if _, err := f.st.MemoryByID(store.UserMemory(bob), id); err == nil {
		t.Fatal("bob can load alice's memory")
	}
}

// With an embedding model, a question finds memories that share no word with it.
func TestMemorySmartSearch(t *testing.T) {
	llmSrv, _ := fakeLLM(t, 0)
	embSrv := fakeEmbeddings(t)
	f := setup(t, llmSrv.URL)
	ctx := context.Background()
	for _, text := range []string{"I am vegetarian and allergic to peanuts.", "My dog is called Biscuit.", "I am the founder of a small company."} {
		if _, err := f.st.CreateMemory(store.UserMemory(f.owner), store.MemoryInput{Content: text}); err != nil {
			t.Fatal(err)
		}
	}

	// No embedding model: only words can match, and none do.
	mems, smart, err := f.eng.SearchMemory(ctx, store.UserMemory(f.owner), "what food should I cook for dinner?", 5)
	if err != nil || smart || len(mems) != 0 {
		t.Fatalf("word search: %d results, smart=%v, err=%v", len(mems), smart, err)
	}

	pid, _ := f.st.CreateProvider(nil, "embedder", embSrv.URL, "k")
	settings, _ := f.st.LoadSettings()
	settings.EmbedProviderID, settings.EmbedModel = pid, "fake-embed"
	if err := f.st.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	em := f.eng.MemoryEmbedder(store.UserMemory(f.owner))
	if em == nil || em.Personal {
		t.Fatalf("embedder = %+v", em)
	}
	if n, err := f.eng.IndexMemories(ctx, store.UserMemory(f.owner)); err != nil || n != 3 {
		t.Fatalf("indexed %d, err %v", n, err)
	}
	if st := f.st.MemoryStats(store.UserMemory(f.owner), em.Key); st.Indexed != 3 {
		t.Fatalf("indexed stat = %d", st.Indexed)
	}

	mems, smart, err = f.eng.SearchMemory(ctx, store.UserMemory(f.owner), "what food should I cook for dinner?", 5)
	if err != nil || !smart || len(mems) != 1 || !strings.Contains(mems[0].Content, "vegetarian") {
		t.Fatalf("smart search: %+v smart=%v err=%v", mems, smart, err)
	}

	// Changing the text drops its vector until it is indexed again.
	if err := f.st.UpdateMemory(store.UserMemory(f.owner), mems[0].ID, store.MemoryInput{Content: "I eat everything now."}); err != nil {
		t.Fatal(err)
	}
	if st := f.st.MemoryStats(store.UserMemory(f.owner), em.Key); st.Indexed != 2 {
		t.Fatalf("indexed after edit = %d, want 2", st.Indexed)
	}
	if n, _ := f.eng.IndexMemories(ctx, store.UserMemory(f.owner)); n != 1 {
		t.Fatalf("re-indexed %d, want 1", n)
	}

	// A user's own embedding model wins over the server's, and its vectors are kept apart.
	own := f.owner
	ownPID, _ := f.st.CreateProvider(&own, "mine", embSrv.URL, "k")
	f.st.SaveMemorySettings(f.owner, store.MemorySettings{EmbedProviderID: ownPID, EmbedModel: "my-embed"})
	mine := f.eng.MemoryEmbedder(store.UserMemory(f.owner))
	if mine == nil || !mine.Personal || mine.Key == em.Key {
		t.Fatalf("own embedder = %+v", mine)
	}
	if st := f.st.MemoryStats(store.UserMemory(f.owner), mine.Key); st.Indexed != 0 {
		t.Fatal("vectors of another model count as ready")
	}
	// Someone else's provider is never used.
	bob, _ := f.st.CreateUser("bob", "x", "user")
	f.st.SaveMemorySettings(bob, store.MemorySettings{EmbedProviderID: ownPID, EmbedModel: "my-embed"})
	if got := f.eng.MemoryEmbedder(store.UserMemory(bob)); got == nil || got.Key != em.Key {
		t.Fatalf("bob's embedder = %+v, want the server's", got)
	}

	// If the embedding endpoint is down, search quietly falls back to words.
	embSrv.Close()
	mems, smart, err = f.eng.SearchMemory(ctx, store.UserMemory(bob), "anything", 5)
	if err != nil || smart || len(mems) != 0 {
		t.Fatalf("fallback: %d results, smart=%v, err=%v", len(mems), smart, err)
	}
}

// Memory tools are offered in a person's own chats when they turned memory on,
// never in group chats, and to an agent only when its owner allowed it (read-only).
func TestMemoryToolsAreOffered(t *testing.T) {
	llmSrv, seen := scriptLLM(t, func(r llmRequest) (string, string, string) {
		if r.LastRole == "tool" {
			return "noted", "", ""
		}
		if r.hasTool("memory_save") && strings.Contains(r.Last, "remember") {
			return "", "memory_save", `{"content":"The user prefers tea over coffee.","about":["tea"]}`
		}
		return "ok", "", ""
	})
	f := setup(t, llmSrv.URL)
	tid, _ := f.st.CreateThread(f.owner, f.modelID)

	// Off by default.
	waitRun(t, f.st, mustSubmit(t, f, tid, f.owner, "remember I prefer tea"))
	if last := seen()[len(seen())-1]; last.hasTool("memory_search") || strings.Contains(last.System, "Long-term memory") {
		t.Fatal("memory is offered although the user never turned it on")
	}

	f.st.SaveMemorySettings(f.owner, store.MemorySettings{Enabled: true})
	runID := mustSubmit(t, f, tid, f.owner, "remember I prefer tea")
	if r := waitRun(t, f.st, runID); r.Status != store.RunDone {
		t.Fatalf("status %s: %s", r.Status, r.Error)
	}
	mems, _ := f.st.ListMemories(store.UserMemory(f.owner), "", 0, 10)
	if len(mems) != 1 || mems[0].Source != store.MemoryFromAI || mems[0].SourceThreadID != tid || len(mems[0].Entities) != 1 {
		t.Fatalf("memories = %+v", mems)
	}
	var withTools llmRequest
	for _, r := range seen() {
		if r.hasTool("memory_save") {
			withTools = r
		}
	}
	for _, tool := range []string{"memory_search", "memory_read", "memory_explore", "memory_save", "memory_update", "memory_delete"} {
		if !withTools.hasTool(tool) {
			t.Errorf("chat was not offered %s", tool)
		}
	}
	if !strings.Contains(withTools.System, "## Long-term memory") {
		t.Error("the system prompt does not explain memory")
	}

	// A group has a memory of its own. The owner's memory stays out of it,
	// also when the owner is the one who writes.
	f.st.SetMemoryPinned(store.UserMemory(f.owner), mems[0].ID, true) // pinned memories go into the owner's prompt
	bob, _ := f.st.CreateUser("bob", "x", "user")
	gid := newGroup(t, f, bob)
	waitRun(t, f.st, mustSubmit(t, f, gid, f.owner, "what do I like to drink?"))
	inGroup := seen()[len(seen())-1]
	if strings.Contains(inGroup.System, "prefers tea") || strings.Contains(inGroup.System, "## Long-term memory") {
		t.Fatal("a group was given the owner's memory")
	}
	if !inGroup.hasTool("memory_search") || !strings.Contains(inGroup.System, "## Group memory") {
		t.Fatal("a group was not given its own memory")
	}
	group, _ := f.st.ThreadByID(gid)
	if hits, _ := f.st.SearchMemoryText(store.GroupMemory(group), "tea", 5); len(hits) != 0 {
		t.Fatalf("the group's memory finds the owner's memories: %+v", hits)
	}

	// Agents: nothing by default, read-only tools once allowed.
	a := newAgent(t, f, store.Agent{Name: "Helper", Instructions: "Help."})
	id, _ := f.eng.RunAgent(a, "go")
	waitRun(t, f.st, id)
	if last := seen()[len(seen())-1]; last.hasTool("memory_search") {
		t.Fatal("an agent reads memory without permission")
	}
	f.st.SetAgentUseMemory(a.ID, true)
	a, _ = f.st.AgentByID(a.ID)
	id, _ = f.eng.RunAgent(a, "go again")
	waitRun(t, f.st, id)
	last := seen()[len(seen())-1]
	if !last.hasTool("memory_search") || !last.hasTool("memory_explore") || last.hasTool("memory_save") || last.hasTool("memory_delete") {
		t.Fatalf("agent tools = %v", last.Tools)
	}
	if !last.hasTool("save_memory") {
		t.Fatal("the agent lost its own notes tool")
	}
}

func mustSubmit(t *testing.T, f *fixture, threadID, userID int64, text string) int64 {
	t.Helper()
	id, err := f.eng.Submit(threadID, userID, text, nil)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
