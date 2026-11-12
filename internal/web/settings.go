package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"knitnox/internal/mcp"
	"knitnox/internal/netguard"
	"knitnox/internal/store"
)

// Providers are managed by the same handlers for two scopes:
//   - admin:  global providers (owner NULL), under /admin
//   - user:   personal providers, under /settings
type scope struct {
	owner *int64
	base  string // URL prefix for forms
	page  string // template set that contains the "providers" partial
}

func userScope(r *http.Request) scope {
	id := currentUser(r).ID
	return scope{owner: &id, base: "/settings", page: "settings"}
}

func adminScope() scope { return scope{owner: nil, base: "/admin", page: "admin"} }

func validURL(u string) bool {
	p, err := url.Parse(strings.TrimSpace(u))
	return err == nil && (p.Scheme == "http" || p.Scheme == "https") && p.Host != ""
}

func (s *Server) renderProviders(w http.ResponseWriter, r *http.Request, sc scope) {
	list, err := s.st.ListProviders(sc.owner)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPartial(w, r, sc.page, "providers", map[string]any{"Providers": list, "Base": sc.base})
}

func (s *Server) providerCreate(w http.ResponseWriter, r *http.Request, sc scope) {
	name := strings.TrimSpace(r.FormValue("name"))
	base := strings.TrimSpace(r.FormValue("base_url"))
	if name == "" || !validURL(base) {
		s.flash(w, r, "Name and a valid http(s) base URL are required.")
		return
	}
	id, err := s.st.CreateProvider(sc.owner, name, base, strings.TrimSpace(r.FormValue("api_key")))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	// Optional comma-separated model list on creation.
	for _, m := range strings.Split(r.FormValue("models"), ",") {
		if m = strings.TrimSpace(m); m != "" {
			s.st.CreateModel(id, m, 0, 0, 0, GuessVision(m))
		}
	}
	if sc.owner == nil {
		s.audit(r, "provider.create", name, base)
	}
	s.afterProviderChange(w, r, sc)
}

func (s *Server) afterProviderChange(w http.ResponseWriter, r *http.Request, sc scope) {
	if isHX(r) {
		s.renderProviders(w, r, sc)
		return
	}
	back(w, r, sc.base)
}

func (s *Server) modelInScope(sc scope, modelID int64) bool {
	m, err := s.st.ModelByID(modelID)
	return err == nil && s.st.ProviderOwnedBy(sc.owner, m.ProviderID)
}

func (s *Server) modelCreate(w http.ResponseWriter, r *http.Request, sc scope) {
	pid := pathID(r, "pid")
	if !s.st.ProviderOwnedBy(sc.owner, pid) {
		s.errorPage(w, r, http.StatusNotFound, "Provider not found.")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.flash(w, r, "Model name is required.")
		return
	}
	kind := r.FormValue("kind")
	if !store.ValidModelKind(kind) {
		kind = store.GuessModelKind(name)
	}
	if err := s.st.CreateModelKind(pid, name, kind, int(formInt(r, "context_window", 128000)),
		formFloat(r, "input_price"), formFloat(r, "output_price"), r.FormValue("vision") == "1"); err != nil {
		s.serverError(w, r, err)
		return
	}
	if sc.owner == nil {
		s.audit(r, "model.create", name, "")
	}
	s.afterProviderChange(w, r, sc)
}

func (s *Server) modelAction(w http.ResponseWriter, r *http.Request, sc scope, action string, fn func(int64) error) {
	mid := pathID(r, "mid")
	if !s.modelInScope(sc, mid) {
		s.errorPage(w, r, http.StatusNotFound, "Model not found.")
		return
	}
	if err := fn(mid); err != nil {
		s.serverError(w, r, err)
		return
	}
	if sc.owner == nil {
		s.audit(r, action, fmt.Sprintf("model #%d", mid), "")
	}
	s.afterProviderChange(w, r, sc)
}

func (s *Server) providerAction(w http.ResponseWriter, r *http.Request, sc scope, action string, fn func(*int64, int64) error) {
	pid := pathID(r, "pid")
	if err := fn(sc.owner, pid); err != nil {
		s.serverError(w, r, err)
		return
	}
	if sc.owner == nil {
		s.audit(r, action, fmt.Sprintf("provider #%d", pid), "")
	}
	s.afterProviderChange(w, r, sc)
}

// ---- model discovery (GET {base_url}/models) ----

// GuessVision guesses from the model id whether it accepts images.
func GuessVision(name string) bool {
	n := strings.ToLower(name)
	for _, hint := range []string{"gpt-4o", "gpt-4.1", "gpt-4.5", "gpt-5", "chatgpt-4o", "o1", "o3", "o4", "claude", "gemini", "gemma-3",
		"vision", "llava", "pixtral", "qwen-vl", "qwen2-vl", "qwen2.5-vl", "qwen3-vl", "minicpm-v", "llama-4", "llama-3.2-11b", "llama-3.2-90b",
		"grok-2-vision", "grok-4", "mistral-small-3", "mistral-medium-3", "phi-3.5-vision", "phi-4-multimodal", "idefics", "molmo", "internvl"} {
		if strings.Contains(n, hint) {
			return !strings.Contains(n, "o1-mini") && !strings.Contains(n, "o3-mini") && !strings.Contains(n, "embed") &&
				!strings.Contains(n, "whisper") && !strings.Contains(n, "tts")
		}
	}
	return false
}

type discoveredModel struct {
	Name   string
	Vision bool
	Known  bool // already configured
}

// providerDiscover fetches the provider's model list and renders a picker.
func (s *Server) providerDiscover(w http.ResponseWriter, r *http.Request, sc scope) {
	pid := pathID(r, "pid")
	if !s.st.ProviderOwnedBy(sc.owner, pid) {
		s.errorPage(w, r, http.StatusNotFound, "Provider not found.")
		return
	}
	p, err := s.st.ProviderByID(pid)
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "Provider not found.")
		return
	}
	fail := func(msg string) {
		s.renderPartial(w, r, sc.page, "discover", map[string]any{"Provider": p, "Base": sc.base, "Error": msg})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/models", nil)
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := netguard.Client(sc.owner != nil).Do(req) // a user's own provider: guarded addresses are refused
	if err != nil {
		fail("Could not reach the provider: " + err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		fail(fmt.Sprintf("The provider answered %d: %s", resp.StatusCode, preview(string(body), 200)))
		return
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil || len(list.Data) == 0 {
		// Ollama's native API and some proxies use other shapes; try a few.
		var alt struct {
			Models []struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			} `json:"models"`
		}
		if json.Unmarshal(body, &alt) == nil {
			for _, m := range alt.Models {
				id := m.ID
				if id == "" {
					id = m.Name
				}
				list.Data = append(list.Data, struct {
					ID string `json:"id"`
				}{id})
			}
		}
	}
	if len(list.Data) == 0 {
		fail("The provider returned no models. Add them by name instead.")
		return
	}
	known := map[string]bool{}
	for _, m := range p.Models {
		known[m.Name] = true
	}
	if ms, err := s.st.ListModels(pid); err == nil {
		for _, m := range ms {
			known[m.Name] = true
		}
	}
	var out []discoveredModel
	seen := map[string]bool{}
	for _, m := range list.Data {
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, discoveredModel{Name: m.ID, Vision: GuessVision(m.ID), Known: known[m.ID]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	s.renderPartial(w, r, sc.page, "discover", map[string]any{"Provider": p, "Base": sc.base, "Models": out})
}

// modelsBulk adds the selected discovered models.
func (s *Server) modelsBulk(w http.ResponseWriter, r *http.Request, sc scope) {
	pid := pathID(r, "pid")
	if !s.st.ProviderOwnedBy(sc.owner, pid) {
		s.errorPage(w, r, http.StatusNotFound, "Provider not found.")
		return
	}
	r.ParseForm()
	visionSet := map[string]bool{}
	for _, v := range r.Form["vision"] {
		visionSet[v] = true
	}
	n := 0
	for _, name := range r.Form["names"] {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 200 {
			continue
		}
		if err := s.st.CreateModel(pid, name, 0, 0, 0, visionSet[name]); err == nil {
			n++
		}
	}
	if sc.owner == nil {
		s.audit(r, "model.bulk_add", fmt.Sprintf("provider #%d", pid), fmt.Sprintf("%d models", n))
	}
	s.afterProviderChange(w, r, sc)
}

// ---- user settings page ----

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	sc := userScope(r)
	providers, err := s.st.ListProviders(sc.owner)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	servers, err := s.st.ListMCPServers(u.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	q, _ := s.st.QuotaFor(u.ID)
	used, _ := s.st.UsedTokens(u.ID, q.Period)
	threads, _ := s.st.ListThreadsForUser(u.ID)
	s.render(w, r, "settings", map[string]any{
		"Providers": providers, "Base": sc.base, "MCPServers": servers,
		"Quota": q, "Used": used, "Threads": threads,
		"PushDevices": s.st.PushSubscriptionCount(u.ID), "Sessions": s.st.SessionCount(u.ID),
	})
}

func (s *Server) userProviderCreate(w http.ResponseWriter, r *http.Request) {
	s.providerCreate(w, r, userScope(r))
}
func (s *Server) userProviderDelete(w http.ResponseWriter, r *http.Request) {
	s.providerAction(w, r, userScope(r), "provider.delete", s.st.DeleteProvider)
}
func (s *Server) userProviderToggle(w http.ResponseWriter, r *http.Request) {
	s.providerAction(w, r, userScope(r), "provider.toggle", s.st.ToggleProvider)
}
func (s *Server) userProviderDiscover(w http.ResponseWriter, r *http.Request) {
	s.providerDiscover(w, r, userScope(r))
}
func (s *Server) userModelsBulk(w http.ResponseWriter, r *http.Request) {
	s.modelsBulk(w, r, userScope(r))
}
func (s *Server) userModelCreate(w http.ResponseWriter, r *http.Request) {
	s.modelCreate(w, r, userScope(r))
}
func (s *Server) userModelDelete(w http.ResponseWriter, r *http.Request) {
	s.modelAction(w, r, userScope(r), "model.delete", s.st.DeleteModel)
}
func (s *Server) userModelToggle(w http.ResponseWriter, r *http.Request) {
	s.modelAction(w, r, userScope(r), "model.toggle", s.st.ToggleModel)
}
func (s *Server) userModelVision(w http.ResponseWriter, r *http.Request) {
	s.modelAction(w, r, userScope(r), "model.vision", s.st.ToggleModelVision)
}

// ---- MCP servers ----

func (s *Server) renderMCP(w http.ResponseWriter, r *http.Request) {
	if !isHX(r) {
		back(w, r, "/settings")
		return
	}
	servers, err := s.st.ListMCPServers(currentUser(r).ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPartial(w, r, "settings", "mcp", map[string]any{"MCPServers": servers})
}

func (s *Server) mcpCreate(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	u := strings.TrimSpace(r.FormValue("url"))
	if name == "" || !validURL(u) {
		s.flash(w, r, "Name and a valid http(s) URL are required. Only remote (HTTP) MCP servers are supported.")
		return
	}
	if err := s.st.CreateMCPServer(currentUser(r).ID, name, u, strings.TrimSpace(r.FormValue("headers")), r.FormValue("approval")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderMCP(w, r)
}

func (s *Server) mcpDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.st.DeleteMCPServer(currentUser(r).ID, pathID(r, "sid")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderMCP(w, r)
}

func (s *Server) mcpToggle(w http.ResponseWriter, r *http.Request) {
	if err := s.st.ToggleMCPServer(currentUser(r).ID, pathID(r, "sid")); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderMCP(w, r)
}

// mcpApproval flips a server between automatic and ask-before-running.
func (s *Server) mcpApproval(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	srv, err := s.st.MCPServerByID(u.ID, pathID(r, "sid"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	mode := store.ApprovalAsk
	if srv.Asks() {
		mode = store.ApprovalAuto
	}
	if v := r.FormValue("mode"); v == store.ApprovalAsk || v == store.ApprovalAuto {
		mode = v
	}
	if err := s.st.SetMCPApproval(u.ID, srv.ID, mode); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderMCP(w, r)
}

// mcpTest connects to a server and lists its tools (returns a text snippet).
func (s *Server) mcpTest(w http.ResponseWriter, r *http.Request) {
	srv, err := s.st.MCPServerByID(currentUser(r).ID, pathID(r, "sid"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	c, err := mcp.Connect(ctx, srv.URL, srv.Headers)
	if err != nil {
		fmt.Fprintf(w, "Failed: %s", templateEscape(err.Error()))
		return
	}
	defer c.Close()
	tools, err := c.ListTools(ctx)
	if err != nil {
		fmt.Fprintf(w, "Connected, but tools/list failed: %s", templateEscape(err.Error()))
		return
	}
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	fmt.Fprintf(w, "OK — %d tools: %s", len(tools), templateEscape(strings.Join(names, ", ")))
}
