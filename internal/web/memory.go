package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"knitnox/internal/engine"
	"knitnox/internal/store"
)

// Memory: the page where a user sees, searches, edits and adds to what the
// AI remembers about them (/memory), walks the things and connections those
// memories mention (/memory/things) and decides how memory is used
// (/memory/settings). Every query is scoped to the signed-in user.

const (
	memoryPageSize   = 30  // memories per page of the list
	memorySearchSize = 30  // results shown for a search
	memoryGraphNodes = 30  // things drawn on the map at first / per expansion
	memoryThingRels  = 100 // connections listed on a thing's page
	memoryThingMems  = 50  // memories listed on a thing's page
	memoryFormRows   = 3   // empty connection rows offered in the form
)

// memoryForm is what the "mem-fields" template needs to show a memory for editing.
type memoryForm struct {
	M     *store.Memory // nil for a new memory
	About string        // names, comma separated
	Rows  []store.RelationInput
}

func newMemoryForm(m *store.Memory) memoryForm {
	f := memoryForm{M: m}
	if m != nil {
		names := make([]string, 0, len(m.Entities))
		inRel := map[int64]bool{}
		for _, r := range m.Relations {
			f.Rows = append(f.Rows, store.RelationInput{Subject: r.Subject, Predicate: r.Predicate, Object: r.Object})
			inRel[r.SubjectID], inRel[r.ObjectID] = true, true
		}
		for _, e := range m.Entities { // things named in a connection are implied; list only the rest
			if !inRel[e.ID] {
				names = append(names, e.Name)
			}
		}
		f.About = strings.Join(names, ", ")
	}
	f.Rows = append(f.Rows, store.RelationInput{}) // always at least one empty row
	for len(f.Rows) < memoryFormRows {
		f.Rows = append(f.Rows, store.RelationInput{})
	}
	return f
}

// memoryInputFromForm reads the fields of "mem-fields".
func memoryInputFromForm(r *http.Request) store.MemoryInput {
	r.ParseForm()
	in := store.MemoryInput{Content: r.FormValue("content"), Pinned: r.FormValue("pinned") == "1"}
	in.About = strings.FieldsFunc(r.FormValue("about"), func(c rune) bool { return c == ',' || c == ';' || c == '\n' })
	subj, pred, obj := r.Form["rel_s"], r.Form["rel_p"], r.Form["rel_o"]
	for i := range subj {
		if i < len(pred) && i < len(obj) {
			in.Relations = append(in.Relations, store.RelationInput{Subject: subj[i], Predicate: pred[i], Object: obj[i]})
		}
	}
	return in
}

// embedView describes the search quality of a user's memory for the page.
type embedView struct {
	On       bool // matching by meaning is available
	Model    string
	Provider string
	Personal bool
	Pending  int // memories still waiting for a vector
}

// memoryData is the part of the page every tab shows.
func (s *Server) memoryData(r *http.Request, tab string) map[string]any {
	u := currentUser(r)
	ms, _ := s.st.MemorySettingsFor(u.ID)
	ev := embedView{}
	key := ""
	if em := s.eng.MemoryEmbedder(store.UserMemory(u.ID)); em != nil {
		ev = embedView{On: true, Model: em.Model, Provider: em.Provider, Personal: em.Personal}
		key = em.Key
	}
	stats := s.st.MemoryStats(store.UserMemory(u.ID), key)
	if ev.On {
		ev.Pending = stats.Memories - stats.Indexed
	}
	return map[string]any{"Tab": tab, "MS": ms, "Stats": stats, "Embed": ev, "Notice": r.URL.Query().Get("done")}
}

// ---- memories ----

func (s *Server) memoryPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if rs := []rune(q); len(rs) > 200 {
		q = string(rs[:200])
	}
	filter := r.URL.Query().Get("filter")
	switch filter {
	case "pinned", store.MemoryFromAI, store.MemoryFromUser:
	default:
		filter = ""
	}
	data := s.memoryData(r, "list")
	data["Query"], data["Filter"] = q, filter
	data["Form"] = newMemoryForm(nil)

	var mems []store.Memory
	var err error
	if q != "" {
		var smart bool
		mems, smart, err = s.eng.SearchMemory(r.Context(), store.UserMemory(u.ID), q, memorySearchSize)
		data["Smart"] = smart
	} else {
		before, _ := parseInt(r.URL.Query().Get("before"))
		mems, err = s.st.ListMemories(store.UserMemory(u.ID), filter, before, memoryPageSize+1)
		if len(mems) > memoryPageSize {
			mems = mems[:memoryPageSize]
			data["Older"] = "/memory?" + url.Values{"filter": {filter}, "before": {fmt.Sprint(mems[len(mems)-1].ID)}}.Encode()
		}
		data["Paged"] = before > 0
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data["Memories"] = mems
	s.render(w, r, "memory", data)
}

func (s *Server) memoryCreate(w http.ResponseWriter, r *http.Request) {
	in := memoryInputFromForm(r)
	if _, err := s.eng.SaveMemory(store.UserMemory(currentUser(r).ID), in); err != nil {
		if errors.Is(err, store.ErrEmptyMemory) {
			s.errorPage(w, r, http.StatusBadRequest, "Write what should be remembered first.")
			return
		}
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/memory?done=added", http.StatusSeeOther)
}

// loadMemory returns the memory in the path if the current user owns it.
func (s *Server) loadMemory(w http.ResponseWriter, r *http.Request) (*store.Memory, bool) {
	m, err := s.st.MemoryByID(store.UserMemory(currentUser(r).ID), pathID(r, "id"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "Memory not found.")
		return nil, false
	}
	return m, true
}

// memoryCard renders one memory (after a change, or when editing is cancelled).
func (s *Server) memoryCard(w http.ResponseWriter, r *http.Request) {
	if m, ok := s.loadMemory(w, r); ok {
		s.renderPartial(w, r, "memory", "mem-card", map[string]any{"M": *m})
	}
}

func (s *Server) memoryEdit(w http.ResponseWriter, r *http.Request) {
	if m, ok := s.loadMemory(w, r); ok {
		s.renderPartial(w, r, "memory", "mem-edit", map[string]any{"Form": newMemoryForm(m)})
	}
}

func (s *Server) memoryUpdate(w http.ResponseWriter, r *http.Request) {
	m, ok := s.loadMemory(w, r)
	if !ok {
		return
	}
	if err := s.eng.UpdateMemory(m.Owner(), m.ID, memoryInputFromForm(r)); err != nil {
		if errors.Is(err, store.ErrEmptyMemory) {
			s.flash(w, r, "A memory needs some text. To remove it, use the bin button instead.")
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.memoryCard(w, r)
}

func (s *Server) memoryPin(w http.ResponseWriter, r *http.Request) {
	m, ok := s.loadMemory(w, r)
	if !ok {
		return
	}
	if err := s.st.SetMemoryPinned(m.Owner(), m.ID, !m.Pinned); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.memoryCard(w, r)
}

func (s *Server) memoryDelete(w http.ResponseWriter, r *http.Request) {
	m, ok := s.loadMemory(w, r)
	if !ok {
		return
	}
	if err := s.st.DeleteMemory(m.Owner(), m.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	if isHX(r) || isFetch(r) {
		w.WriteHeader(http.StatusOK) // the card is swapped for nothing
		return
	}
	back(w, r, "/memory")
}

// ---- things and connections ----

func (s *Server) memoryThings(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	things, err := s.st.ListEntities(store.UserMemory(currentUser(r).ID), q, 300)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := s.memoryData(r, "things")
	data["Query"], data["Things"] = q, things
	s.render(w, r, "memory", data)
}

func (s *Server) memoryThing(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	thing, err := s.st.EntityByID(store.UserMemory(u.ID), pathID(r, "eid"))
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "That thing is not in your memory (any more).")
		return
	}
	rels, relTotal, err := s.st.EntityRelations(store.UserMemory(u.ID), thing.ID, memoryThingRels, 0)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	mems, memTotal, err := s.st.EntityMemories(store.UserMemory(u.ID), thing.ID, memoryThingMems)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := s.memoryData(r, "thing")
	data["Thing"], data["Relations"], data["RelTotal"] = thing, rels, relTotal
	data["Memories"], data["MemTotal"] = mems, memTotal
	s.render(w, r, "memory", data)
}

// memoryThingRename renames a thing, merging it into another with the same name.
func (s *Server) memoryThingRename(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := pathID(r, "eid")
	if _, err := s.st.EntityByID(store.UserMemory(u.ID), id); err != nil {
		s.errorPage(w, r, http.StatusNotFound, "That thing is not in your memory (any more).")
		return
	}
	keep, err := s.st.RenameEntity(store.UserMemory(u.ID), id, r.FormValue("name"))
	if err != nil {
		s.errorPage(w, r, http.StatusBadRequest, "Could not rename: "+err.Error())
		return
	}
	done := "renamed"
	if keep != id {
		done = "merged"
	}
	http.Redirect(w, r, fmt.Sprintf("/memory/things/%d?done=%s", keep, done), http.StatusSeeOther)
}

// memoryGraph answers the map on the page: a small part of the graph as JSON.
func (s *Server) memoryGraph(w http.ResponseWriter, r *http.Request) {
	focus, _ := parseInt(r.URL.Query().Get("focus"))
	g, err := s.st.MemoryGraphAround(store.UserMemory(currentUser(r).ID), focus, memoryGraphNodes)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	type node struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Memories int    `json:"memories"`
		Links    int    `json:"links"`
	}
	type edge struct {
		S int64  `json:"s"`
		T int64  `json:"t"`
		P string `json:"p"`
	}
	out := struct {
		Nodes []node `json:"nodes"`
		Edges []edge `json:"edges"`
		More  bool   `json:"more"`
	}{Nodes: []node{}, Edges: []edge{}, More: g.More}
	for _, n := range g.Nodes {
		out.Nodes = append(out.Nodes, node{n.ID, n.Name, n.Memories, n.Links})
	}
	for _, e := range g.Edges {
		out.Edges = append(out.Edges, edge{e.SubjectID, e.ObjectID, e.Predicate})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(out)
}

// ---- settings ----

func (s *Server) memorySettingsPage(w http.ResponseWriter, r *http.Request) {
	sc := userScope(r)
	providers, err := s.st.ListProviders(sc.owner)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := s.memoryData(r, "settings")
	data["Providers"] = providers
	s.render(w, r, "memory", data)
}

// memorySwitch turns the AI's use of the user's memory on or off.
func (s *Server) memorySwitch(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	ms, err := s.st.MemorySettingsFor(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	ms.Enabled = r.FormValue("enabled") == "1"
	if err := s.st.SaveMemorySettings(u.ID, ms); err != nil {
		s.serverError(w, r, err)
		return
	}
	done := "off"
	if ms.Enabled {
		done = "on"
	}
	to := "/memory/settings"
	if r.FormValue("from") == "list" {
		to = "/memory"
	}
	http.Redirect(w, r, to+"?done="+done, http.StatusSeeOther)
}

// ownEmbedForm reads the "my own embedding model" fields. The provider must
// be one of the user's own.
func (s *Server) ownEmbedForm(r *http.Request) (pid int64, model string, dims int, ok bool) {
	pid = formInt(r, "embed_provider_id", 0)
	model = strings.TrimSpace(r.FormValue("embed_model"))
	dims = int(min(max(formInt(r, "embed_dimensions", 0), 0), 8192))
	if pid == 0 || model == "" {
		return 0, "", 0, true // back to the server's model
	}
	uid := currentUser(r).ID
	return pid, model, dims, s.st.ProviderOwnedBy(&uid, pid)
}

func (s *Server) memorySearchSettings(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	pid, model, dims, ok := s.ownEmbedForm(r)
	if !ok {
		s.errorPage(w, r, http.StatusBadRequest, "Pick one of your own providers.")
		return
	}
	ms, err := s.st.MemorySettingsFor(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	ms.EmbedProviderID, ms.EmbedModel, ms.EmbedDimensions = pid, model, dims
	if err := s.st.SaveMemorySettings(u.ID, ms); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.eng.IndexMemoriesSoon(store.UserMemory(u.ID))
	http.Redirect(w, r, "/memory/settings?done=search", http.StatusSeeOther)
}

// testEmbedding tries an embedding model and writes one line about the result.
func testEmbedding(ctx context.Context, w http.ResponseWriter, p *store.Provider, model string, dims int) {
	if !p.Enabled {
		fmt.Fprint(w, "This provider is disabled. Enable it first.")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	em := &engine.Embedder{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: model, Dimensions: dims, Personal: !p.IsGlobal()}
	n, err := em.Test(ctx)
	if err != nil {
		fmt.Fprintf(w, "It did not work: %s", templateEscape(preview(err.Error(), 300)))
		return
	}
	fmt.Fprintf(w, "It works. %s answers with %d numbers per text.", templateEscape(model), n)
}

// memorySearchTest tries the model in the form (or, with no model typed, the one in use).
func (s *Server) memorySearchTest(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	pid, model, dims, ok := s.ownEmbedForm(r)
	if !ok {
		fmt.Fprint(w, "Pick one of your own providers.")
		return
	}
	if pid == 0 {
		em := s.eng.MemoryEmbedder(store.UserMemory(u.ID))
		if em == nil {
			fmt.Fprint(w, "Pick a provider and type a model name first.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if n, err := em.Test(ctx); err != nil {
			fmt.Fprintf(w, "The model in use did not answer: %s", templateEscape(preview(err.Error(), 300)))
		} else {
			fmt.Fprintf(w, "The model in use works. %s answers with %d numbers per text.", templateEscape(em.Model), n)
		}
		return
	}
	p, err := s.st.ProviderByID(pid)
	if err != nil {
		fmt.Fprint(w, "Provider not found.")
		return
	}
	testEmbedding(r.Context(), w, p, model, dims)
}

// memoryReindex makes vectors for everything that has none yet and reports.
func (s *Server) memoryReindex(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if s.eng.MemoryEmbedder(store.UserMemory(u.ID)) == nil {
		fmt.Fprint(w, "There is no embedding model yet, so there is nothing to prepare.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	n, err := s.eng.IndexMemories(ctx, store.UserMemory(u.ID))
	if err != nil {
		fmt.Fprintf(w, "Prepared %d, then stopped: %s", n, templateEscape(preview(err.Error(), 300)))
		return
	}
	if n == 0 {
		fmt.Fprint(w, "Everything was ready already.")
		return
	}
	fmt.Fprintf(w, "Done. %d memories are now ready for smart search.", n)
}

func (s *Server) memoryDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("confirm") != "delete" {
		s.errorPage(w, r, http.StatusBadRequest, "Type the word delete to confirm.")
		return
	}
	if err := s.st.DeleteAllMemories(store.UserMemory(currentUser(r).ID)); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/memory/settings?done=wiped", http.StatusSeeOther)
}

// memoryExport downloads everything as JSON, so nothing is locked in.
func (s *Server) memoryExport(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	type rel struct {
		Subject  string `json:"subject"`
		Relation string `json:"relation"`
		Object   string `json:"object"`
	}
	type item struct {
		ID        int64    `json:"id"`
		Content   string   `json:"content"`
		About     []string `json:"about,omitempty"`
		Relations []rel    `json:"relations,omitempty"`
		SavedBy   string   `json:"saved_by"`
		Pinned    bool     `json:"pinned,omitempty"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
	}
	items := []item{}
	var before int64
	for {
		page, err := s.st.ListMemories(store.UserMemory(u.ID), "", before, 500)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		for _, m := range page {
			it := item{ID: m.ID, Content: m.Content, SavedBy: m.Source, Pinned: m.Pinned, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
			for _, e := range m.Entities {
				it.About = append(it.About, e.Name)
			}
			for _, x := range m.Relations {
				it.Relations = append(it.Relations, rel{x.Subject, x.Predicate, x.Object})
			}
			items = append(items, it)
		}
		if len(page) < 500 {
			break
		}
		before = page[len(page)-1].ID
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="knitnox-memory.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]any{"exported_at": store.Now(), "user": u.Username, "memories": items})
}

// adminEmbedTest tries the embedding model typed into Admin → Settings.
func (s *Server) adminEmbedTest(w http.ResponseWriter, r *http.Request) {
	pid := formInt(r, "embed_provider_id", 0)
	model := strings.TrimSpace(r.FormValue("embed_model"))
	if pid == 0 || model == "" {
		fmt.Fprint(w, "Pick a provider and type a model name first.")
		return
	}
	p, err := s.st.ProviderByID(pid)
	if err != nil || !p.IsGlobal() {
		fmt.Fprint(w, "Pick one of the shared providers.")
		return
	}
	testEmbedding(r.Context(), w, p, model, int(min(max(formInt(r, "embed_dimensions", 0), 0), 8192)))
}
