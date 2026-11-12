package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"knitnox/internal/store"
)

// The Memory page: add, find, change, pin, forget; things and the map; and
// nobody can see or touch another person's memory.
func TestMemoryPagesAndPermissions(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")

	// Memory starts off, and the page says so in plain words.
	page := alice.get("/memory")
	for _, want := range []string{`href="/memory" class="sb-link is-active" aria-current="page"`, "Memory is turned off", "Turn on memory",
		"Nothing remembered yet", "How does memory work?", `src="/static/memory.js`} {
		if !strings.Contains(page, want) {
			t.Errorf("empty memory page is missing %q", want)
		}
	}
	if !strings.Contains(alice.get("/"), `href="/memory"`) {
		t.Error("Memory is not reachable from the sidebar or the palette")
	}
	if code, _, h := alice.post("/memory/settings", url.Values{"enabled": {"1"}, "from": {"list"}}); code != http.StatusSeeOther || h.Get("Location") != "/memory?done=on" {
		t.Fatalf("turn on: %d %s", code, h.Get("Location"))
	}
	if !a.st.MemoryEnabled(alice.id) || a.st.MemoryEnabled(bob.id) {
		t.Fatal("the switch did not change exactly alice's setting")
	}
	if strings.Contains(alice.get("/memory"), "Memory is turned off") {
		t.Error("the page still says memory is off")
	}

	// Add a memory with a thing and a connection.
	code, _, h := alice.post("/memory", url.Values{
		"content": {"Tanim founded Wish & Cook."}, "about": {"Dhaka, Tanim"}, "pinned": {"1"},
		"rel_s": {"Tanim", ""}, "rel_p": {"Founded", ""}, "rel_o": {"Wish & Cook", ""},
	})
	if code != http.StatusSeeOther || h.Get("Location") != "/memory?done=added" {
		t.Fatalf("create: %d %s", code, h.Get("Location"))
	}
	mems, _ := a.st.ListMemories(alice.id, "", 0, 10)
	if len(mems) != 1 || !mems[0].Pinned || len(mems[0].Entities) != 3 || len(mems[0].Relations) != 1 || mems[0].Relations[0].Predicate != "founded" {
		t.Fatalf("stored memory = %+v", mems)
	}
	id := mems[0].ID
	item := fmt.Sprintf("/memory/m/%d", id)
	if code, _, _ := alice.post("/memory", url.Values{"content": {"  "}}); code != http.StatusBadRequest {
		t.Errorf("an empty memory was accepted: %d", code)
	}

	page = alice.get("/memory")
	for _, want := range []string{"Tanim founded Wish &amp; Cook.", "Added by you", "Always kept in mind", `class="chip thing-chip"`, ">Dhaka<", "<i>founded</i>"} {
		if !strings.Contains(page, want) {
			t.Errorf("memory list is missing %q", want)
		}
	}
	if !strings.Contains(alice.get("/memory?q=who+founded+the+company"), "Tanim founded Wish &amp; Cook.") {
		t.Error("search did not find the memory")
	}
	if strings.Contains(alice.get("/memory?filter=ai"), "Tanim founded") {
		t.Error("the “Saved by the AI” filter shows a memory the user added")
	}

	// Bob sees none of it and cannot reach it by id.
	if strings.Contains(bob.get("/memory"), "Tanim") || strings.Contains(bob.get("/memory?q=Tanim"), "Wish") ||
		strings.Contains(bob.get("/memory/things"), "Tanim") || strings.Contains(bob.get("/memory/graph"), "Tanim") ||
		strings.Contains(bob.get("/memory/export"), "Tanim") {
		t.Fatal("bob can see alice's memory")
	}
	things, _ := a.st.ListEntities(alice.id, "", 10)
	thing := fmt.Sprintf("/memory/things/%d", things[0].ID)
	for _, path := range []string{item, item + "/edit", thing} {
		if code := bob.getStatus(path); code != http.StatusNotFound {
			t.Errorf("bob GET %s = %d, want 404", path, code)
		}
	}
	for _, path := range []string{item, item + "/pin", item + "/delete", thing} {
		if code, _, _ := bob.post(path, url.Values{"content": {"hacked"}, "name": {"hacked"}}); code != http.StatusNotFound {
			t.Errorf("bob POST %s = %d, want 404", path, code)
		}
	}
	if m, err := a.st.MemoryByID(alice.id, id); err != nil || m.Content != "Tanim founded Wish & Cook." || !m.Pinned {
		t.Fatalf("alice's memory changed: %+v %v", m, err)
	}

	// Change it in place: the edit form comes back filled in, saving returns the card.
	form := alice.get(item + "/edit")
	for _, want := range []string{`hx-post="` + item + `"`, "Tanim founded Wish &amp; Cook.", `name="about" value="Dhaka"`, `name="rel_p" value="founded"`} {
		if !strings.Contains(form, want) {
			t.Errorf("edit form is missing %q", want)
		}
	}
	code, card, _ := alice.post(item, url.Values{"content": {"Tanim founded Wish & Cook in 2021."}, "about": {"Dhaka"},
		"rel_s": {"Tanim"}, "rel_p": {"founded"}, "rel_o": {"Wish & Cook"}})
	if code != http.StatusOK || !strings.Contains(card, "in 2021.") || !strings.Contains(card, fmt.Sprintf(`id="mem-%d"`, id)) {
		t.Fatalf("update: %d %q", code, card)
	}
	if _, card, _ := alice.post(item+"/pin", nil); strings.Contains(card, "Always kept in mind") {
		t.Error("unpinning did not unpin")
	}

	// Things: the list, one thing's page, the map data, renaming into a merge.
	page = alice.get("/memory/things")
	for _, want := range []string{"Things and how they connect", `id="mem-graph"`, ">Tanim<", ">Wish &amp; Cook<"} {
		if !strings.Contains(page, want) {
			t.Errorf("things page is missing %q", want)
		}
	}
	tanim, _ := a.st.EntityByName(alice.id, "tanim")
	page = alice.get(fmt.Sprintf("/memory/things/%d", tanim.ID))
	for _, want := range []string{"<h2>Tanim</h2>", "<b>Tanim</b>", "<i>founded</i>", "in 2021.", fmt.Sprintf(`data-focus="%d"`, tanim.ID)} {
		if !strings.Contains(page, want) {
			t.Errorf("thing page is missing %q", want)
		}
	}
	graph := alice.get("/memory/graph")
	if !strings.Contains(graph, `"name":"Tanim"`) || !strings.Contains(graph, `"p":"founded"`) {
		t.Errorf("graph = %s", graph)
	}
	if got := alice.get(fmt.Sprintf("/memory/graph?focus=%d", tanim.ID)); !strings.Contains(got, ` Cook"`) || strings.Contains(got, "Dhaka") {
		t.Errorf("focused graph = %s", got)
	}
	a.st.CreateMemory(alice.id, store.MemoryInput{Content: "Tanim A. drinks tea.", About: []string{"Tanim A."}})
	other, _ := a.st.EntityByName(alice.id, "Tanim A.")
	code, _, h = alice.post(fmt.Sprintf("/memory/things/%d", other.ID), url.Values{"name": {"tanim"}})
	if code != http.StatusSeeOther || h.Get("Location") != fmt.Sprintf("/memory/things/%d?done=merged", tanim.ID) {
		t.Fatalf("merge: %d %s", code, h.Get("Location"))
	}
	if merged, _ := a.st.EntityByID(alice.id, tanim.ID); merged.Memories != 2 {
		t.Fatalf("merged thing has %d memories, want 2", merged.Memories)
	}
	if hits, _ := a.st.SearchMemoryText(alice.id, "Tanim tea", 5); len(hits) != 2 {
		t.Errorf("search after merge found %d memories, want 2", len(hits))
	}

	// Export, then forget one, then everything.
	if exp := alice.get("/memory/export"); !strings.Contains(exp, `"relation": "founded"`) || !strings.Contains(exp, "drinks tea") {
		t.Errorf("export = %s", exp)
	}
	if code, body, _ := alice.post(item+"/delete", nil); code != http.StatusOK || body != "" {
		t.Fatalf("delete: %d %q", code, body)
	}
	if code, _, _ := alice.post("/memory/delete-all", url.Values{"confirm": {"yes"}}); code != http.StatusBadRequest {
		t.Errorf("delete-all without the word delete: %d", code)
	}
	if code, _, _ := alice.post("/memory/delete-all", url.Values{"confirm": {"delete"}}); code != http.StatusSeeOther {
		t.Fatalf("delete-all: %d", code)
	}
	if st := a.st.MemoryStats(alice.id, ""); st.Memories != 0 || st.Entities != 0 || st.Relations != 0 {
		t.Fatalf("after delete-all: %+v", st)
	}
}

// Search settings: only your own provider can be chosen; the admin sets the shared one.
func TestMemorySearchSettings(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice") // admin
	bob := a.register("bob")

	page := bob.get("/memory/settings")
	for _, want := range []string{"Let the AI use my memory", "Word search", "What is an embedding model?", "ask your admin", "Download everything"} {
		if !strings.Contains(page, want) {
			t.Errorf("settings page is missing %q", want)
		}
	}
	bobID := bob.id
	bobs, _ := a.st.CreateProvider(&bobID, "bobs", "http://127.0.0.1:1/v1", "k")
	shared, _ := a.st.CreateProvider(nil, "shared", "http://127.0.0.1:1/v1", "k")

	// Bob may use his own provider, not alice's and not a shared one as "his own".
	aliceID := alice.id
	alices, _ := a.st.CreateProvider(&aliceID, "alices", "http://127.0.0.1:1/v1", "k")
	for _, pid := range []int64{alices, shared} {
		if code, _, _ := bob.post("/memory/settings/search", url.Values{"embed_provider_id": {fmt.Sprint(pid)}, "embed_model": {"e"}}); code != http.StatusBadRequest {
			t.Errorf("bob chose provider %d: %d", pid, code)
		}
	}
	if code, _, _ := bob.post("/memory/settings/search", url.Values{"embed_provider_id": {fmt.Sprint(bobs)}, "embed_model": {"my-embed"}}); code != http.StatusSeeOther {
		t.Fatalf("bob's own provider: %d", code)
	}
	if em := a.eng.MemoryEmbedder(bob.id); em == nil || em.Model != "my-embed" || !em.Personal {
		t.Fatalf("bob's embedder = %+v", em)
	}
	if !strings.Contains(bob.get("/memory/settings"), "Smart search") {
		t.Error("settings do not show smart search as on")
	}
	// The switch keeps the search choice.
	bob.post("/memory/settings", url.Values{"enabled": {"1"}})
	if ms, _ := a.st.MemorySettingsFor(bob.id); !ms.Enabled || ms.EmbedModel != "my-embed" {
		t.Fatalf("settings after switch = %+v", ms)
	}
	// A test against an unreachable endpoint explains itself instead of failing.
	if _, body, _ := bob.post("/memory/settings/test", url.Values{"embed_provider_id": {fmt.Sprint(bobs)}, "embed_model": {"my-embed"}}); !strings.HasPrefix(body, "It did not work") {
		t.Errorf("test result = %q", body)
	}

	// Only an admin sets the model for everyone, and only from shared providers.
	if code, _, _ := bob.post("/admin/settings/embed-test", url.Values{}); code != http.StatusForbidden {
		t.Errorf("member reached the admin test: %d", code)
	}
	st, _ := a.st.LoadSettings()
	save := func(pid int64) store.Settings {
		alice.post("/admin/settings", url.Values{"embed_provider_id": {fmt.Sprint(pid)}, "embed_model": {"shared-embed"},
			"queue_max": {fmt.Sprint(st.QueueMax)}, "system_prompt": {st.SystemPrompt}, "compaction_keep": {"10"}})
		out, _ := a.st.LoadSettings()
		return out
	}
	if got := save(alices); got.EmbedProviderID != 0 {
		t.Error("a personal provider was accepted as the server's embedding provider")
	}
	if got := save(shared); got.EmbedProviderID != shared || got.EmbedModel != "shared-embed" {
		t.Fatalf("admin settings = %+v", got)
	}
	if em := a.eng.MemoryEmbedder(alice.id); em == nil || em.Model != "shared-embed" || em.Personal {
		t.Fatalf("alice's embedder = %+v", em)
	}
	if !strings.Contains(alice.get("/admin/settings"), `value="shared-embed"`) {
		t.Error("admin settings do not show the embedding model")
	}
}

// An agent reads its owner's memory only after the owner allows it.
func TestAgentMemoryAccess(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")
	bob := a.register("bob")
	ag := &store.Agent{UserID: alice.id, Name: "Helper", Instructions: "x", MaxSteps: 5, DailyRuns: 5, HistoryRuns: 5, Enabled: true}
	if _, err := a.st.CreateAgent(ag, 0); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/agents/%d/memory/access", ag.ID)
	if code, _, _ := bob.post(path, url.Values{"use_memory": {"1"}}); code != http.StatusNotFound {
		t.Fatalf("bob changed alice's agent: %d", code)
	}
	code, panel, _ := alice.post(path, url.Values{"use_memory": {"1"}})
	if code != http.StatusOK || !strings.Contains(panel, "Your memory is turned off") || !strings.Contains(panel, "The agent's own notes") {
		t.Fatalf("panel after allowing: %d", code)
	}
	if got, _ := a.st.AgentByID(ag.ID); !got.UseMemory {
		t.Fatal("access was not saved")
	}
	alice.post(path, url.Values{})
	if got, _ := a.st.AgentByID(ag.ID); got.UseMemory {
		t.Fatal("access was not removed")
	}
}
