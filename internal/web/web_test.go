package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"knitnox/internal/engine"
	"knitnox/internal/hub"
	"knitnox/internal/secret"
	"knitnox/internal/store"
)

// fakeLLM echoes the last user message; with tools offered it calls the first.
func fakeLLM(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct{ Name string } `json:"function"`
			} `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(v any) {
			b, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		var last string
		hasTool := false
		for _, m := range req.Messages {
			if m.Role == "user" {
				var s string
				if json.Unmarshal(m.Content, &s) == nil {
					last = s
					if i := strings.Index(s, "]\n"); i > 0 && strings.HasPrefix(s, "[") { // drop the time stamp line
						last = s[i+2:]
					}
				} else {
					last = "(parts)"
				}
			}
			if m.Role == "tool" {
				hasTool = true
			}
		}
		if len(req.Tools) > 0 && !hasTool {
			send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{
				map[string]any{"index": 0, "id": "c1", "type": "function", "function": map[string]any{"name": req.Tools[0].Function.Name, "arguments": "{}"}},
			}}}}})
		} else {
			send(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": "echo: " + last}}}})
		}
		send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fakeMCP(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		json.Unmarshal(body, &req)
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "fake"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "danger", "description": "does things", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "did it"}}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

type app struct {
	t   *testing.T
	st  *store.Store
	eng *engine.Engine
	srv *httptest.Server
	web *Server
}

func newApp(t *testing.T) *app {
	box, _ := secret.New("test")
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := hub.New()
	eng := engine.New(st, h, log)
	eng.ApprovalTimeout = time.Hour
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Shutdown(2 * time.Second) })
	s, err := New(st, eng, h, log, Options{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &app{t: t, st: st, eng: eng, srv: srv, web: s}
}

// client is a signed-in browser with its own cookie jar and CSRF token.
type client struct {
	a    *app
	http *http.Client
	csrf string
	id   int64
}

var csrfRe = regexp.MustCompile(`data-csrf="([^"]+)"`)

func (a *app) register(username string) *client {
	jar, _ := cookiejar.New(nil)
	c := &client{a: a, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	r, err := c.http.PostForm(a.srv.URL+"/register", url.Values{"username": {username}, "password": {"password123"}, "password2": {"password123"}})
	if err != nil {
		a.t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusSeeOther {
		a.t.Fatalf("register %s: %d", username, r.StatusCode)
	}
	page := c.get("/")
	m := csrfRe.FindStringSubmatch(page)
	if m == nil {
		a.t.Fatal("no csrf token on page")
	}
	c.csrf = m[1]
	u, _, err := a.st.UserAuth(username)
	if err != nil {
		a.t.Fatal(err)
	}
	c.id = u.ID
	return c
}

func (c *client) get(path string) string {
	r, err := c.http.Get(c.a.srv.URL + path)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func (c *client) getStatus(path string) int {
	r, err := c.http.Get(c.a.srv.URL + path)
	if err != nil {
		c.a.t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}

func (c *client) post(path string, form url.Values) (int, string, http.Header) {
	req, _ := http.NewRequest(http.MethodPost, c.a.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.Header.Set("X-Fetch", "1")
	r, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, strings.TrimSpace(string(b)), r.Header
}

func (c *client) upload(path string, fields map[string]string, files map[string][]byte) (int, string, http.Header) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	for name, data := range files {
		fw, _ := mw.CreateFormFile("files", name)
		fw.Write(data)
	}
	mw.Close()
	req, _ := http.NewRequest(http.MethodPost, c.a.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.Header.Set("X-Fetch", "1")
	r, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, strings.TrimSpace(string(b)), r.Header
}

func (a *app) addModel(llmURL string) int64 {
	pid, err := a.st.CreateProvider(nil, "fake", llmURL, "k")
	if err != nil {
		a.t.Fatal(err)
	}
	a.st.CreateModel(pid, "m", 8000, 0, 0, true)
	ms, _ := a.st.ListModels(pid)
	return ms[0].ID
}

func (a *app) waitDone(threadID int64) {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		last := a.st.LastRunID(threadID)
		if r, err := a.st.RunByID(last); err == nil && r.Status != store.RunPending && r.Status != store.RunRunning {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	a.t.Fatal("run did not finish")
}

func threadIDFromLocation(t *testing.T, h http.Header) int64 {
	loc := h.Get("Location")
	var id int64
	if _, err := fmt.Sscanf(loc, "/t/%d", &id); err != nil {
		t.Fatalf("bad redirect %q", loc)
	}
	return id
}

// ---- tests ----

func TestAuthCSRFAndLockout(t *testing.T) {
	a := newApp(t)
	alice := a.register("alice")

	// POST without CSRF is refused.
	req, _ := http.NewRequest(http.MethodPost, a.srv.URL+"/t", strings.NewReader("content=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, _ := alice.http.Do(req)
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("no-CSRF post = %d", r.StatusCode)
	}
	r.Body.Close()

	// Security headers: strict CSP without unsafe-inline.
	r, _ = alice.http.Get(a.srv.URL + "/")
	csp := r.Header.Get("Content-Security-Policy")
	r.Body.Close()
	if strings.Contains(csp, "unsafe-inline") || !strings.Contains(csp, "style-src 'self'") {
		t.Fatalf("csp = %q", csp)
	}

	// Ten wrong passwords lock the account for a while.
	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var code int
	for i := 0; i < 11; i++ {
		r, _ := anon.PostForm(a.srv.URL+"/login", url.Values{"username": {"alice"}, "password": {"wrong"}})
		code = r.StatusCode
		r.Body.Close()
	}
	if code != http.StatusTooManyRequests {
		t.Fatalf("after 11 failures status = %d, want 429", code)
	}
	r, _ = anon.PostForm(a.srv.URL+"/login", url.Values{"username": {"alice"}, "password": {"password123"}})
	r.Body.Close()
	if r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("correct password while locked = %d, want 429", r.StatusCode)
	}
}

func TestChatFlowPermissionsAndFiles(t *testing.T) {
	a := newApp(t)
	llm := fakeLLM(t)
	mid := a.addModel(llm.URL)
	alice := a.register("alice")
	bob := a.register("bob")

	// New chat with a text file attached.
	code, body, h := alice.upload("/t", map[string]string{"content": "summarise", "model_id": fmt.Sprint(mid)},
		map[string][]byte{"notes.txt": []byte("the quick brown fox")})
	if code != http.StatusSeeOther {
		t.Fatalf("new thread = %d %s", code, body)
	}
	tid := threadIDFromLocation(t, h)
	a.waitDone(tid)
	page := alice.get(fmt.Sprintf("/t/%d", tid))
	if !strings.Contains(page, "file-chip") || !strings.Contains(page, "notes.txt") || !strings.Contains(page, "echo: ") {
		t.Fatal("thread page lacks the file chip or the reply")
	}
	// Bob is not a member: thread, messages, files and export are 404.
	msgs, _ := a.st.AllMessages(tid)
	atts, _ := a.st.AttachmentsByMessage([]int64{msgs[0].ID})
	fileID := atts[msgs[0].ID][0].ID
	for _, p := range []string{fmt.Sprintf("/t/%d", tid), fmt.Sprintf("/t/%d/messages", tid), fmt.Sprintf("/files/%d", fileID), fmt.Sprintf("/t/%d/export", tid)} {
		if s := bob.getStatus(p); s != http.StatusNotFound {
			t.Fatalf("bob %s = %d, want 404", p, s)
		}
	}
	// Documents download with a safe content type.
	r, _ := alice.http.Get(fmt.Sprintf("%s/files/%d", a.srv.URL, fileID))
	r.Body.Close()
	if !strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("file disposition = %q", r.Header.Get("Content-Disposition"))
	}
	// A binary pretending to be text is rejected; a fake image too.
	if code, body, _ := alice.upload(fmt.Sprintf("/t/%d/messages", tid), map[string]string{"content": "x"}, map[string][]byte{"evil.txt": {0, 1, 2, 3}}); code != http.StatusBadRequest {
		t.Fatalf("binary txt = %d %s", code, body)
	}
	if code, _, _ := alice.upload(fmt.Sprintf("/t/%d/messages", tid), map[string]string{"content": "x"}, map[string][]byte{"a.png": []byte("not a png")}); code != http.StatusBadRequest {
		t.Fatalf("fake png = %d", code)
	}
	// Bob cannot edit alice's message even if he knew the ids.
	if code, _, _ := bob.post(fmt.Sprintf("/t/%d/runs/%d/edit", tid, msgs[0].RunID), url.Values{"content": {"hack"}}); code != http.StatusNotFound {
		t.Fatalf("bob edit = %d, want 404", code)
	}
	// Rename, pin, archive, export.
	if code, body, _ := alice.post(fmt.Sprintf("/t/%d/rename", tid), url.Values{"title": {"  Fox   notes "}}); code != 200 || body != "Fox notes" {
		t.Fatalf("rename = %d %q", code, body)
	}
	if code, _, _ := bob.post(fmt.Sprintf("/t/%d/rename", tid), url.Values{"title": {"x"}}); code != http.StatusNotFound {
		t.Fatalf("bob rename = %d", code)
	}
	if code, _, _ := alice.post(fmt.Sprintf("/t/%d/flag", tid), url.Values{"flag": {"pinned"}, "on": {"1"}}); code != http.StatusNoContent {
		t.Fatalf("pin = %d", code)
	}
	if !strings.Contains(alice.get("/"), "Pinned") {
		t.Fatal("sidebar lacks the Pinned group")
	}
	if code, _, _ := alice.post(fmt.Sprintf("/t/%d/flag", tid), url.Values{"flag": {"archived"}, "on": {"1"}}); code != http.StatusNoContent {
		t.Fatalf("archive = %d", code)
	}
	if !strings.Contains(alice.get("/"), "Archived (1)") {
		t.Fatal("sidebar lacks the Archived group")
	}
	exp := alice.get(fmt.Sprintf("/t/%d/export", tid))
	if !strings.HasPrefix(exp, "# Fox notes") || !strings.Contains(exp, "echo: ") {
		t.Fatalf("export = %q", exp[:min(80, len(exp))])
	}
	if j := alice.get(fmt.Sprintf("/t/%d/export?format=json", tid)); !strings.Contains(j, `"title":"Fox notes"`) {
		t.Fatal("json export wrong")
	}
	// Search finds the message, only for members.
	if s := alice.get("/search?q=summar"); !strings.Contains(s, "<mark>summarise</mark>") {
		t.Fatal("search did not highlight the hit")
	}
	if s := bob.get("/search?q=summar"); strings.Contains(s, "<mark>") {
		t.Fatal("bob can search alice's chat")
	}
	// Custom instructions are saved and shown back.
	if code, _, _ := alice.post("/settings/instructions", url.Values{"instructions": {"Reply in haiku"}}); code/100 != 2 && code != http.StatusSeeOther {
		t.Fatalf("instructions = %d", code)
	}
	if !strings.Contains(alice.get("/settings"), "Reply in haiku") {
		t.Fatal("instructions not shown")
	}
	// Logout everywhere invalidates the session.
	alice.post("/settings/logout-all", nil)
	if s := alice.getStatus("/settings"); s != http.StatusSeeOther {
		t.Fatalf("after logout-all settings = %d, want redirect", s)
	}
}

func TestToolApprovalOverHTTP(t *testing.T) {
	a := newApp(t)
	llm := fakeLLM(t)
	mcp := fakeMCP(t)
	mid := a.addModel(llm.URL)
	alice := a.register("alice")
	bob := a.register("bob")
	if code, _, _ := alice.post("/settings/mcp", url.Values{"name": {"danger"}, "url": {mcp.URL}, "approval": {"ask"}}); code/100 != 2 && code != http.StatusSeeOther {
		t.Fatalf("add mcp = %d", code)
	}
	code, body, h := alice.post("/t", url.Values{"content": {"do the thing"}, "model_id": {fmt.Sprint(mid)}})
	if code != http.StatusSeeOther {
		t.Fatalf("new thread = %d %s", code, body)
	}
	tid := threadIDFromLocation(t, h)

	var approval store.ToolApproval
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pend, _ := a.st.PendingApprovals(tid)
		for _, l := range pend {
			approval = l[0]
		}
		if approval.ID != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if approval.ID == 0 {
		t.Fatal("no approval appeared")
	}
	page := alice.get(fmt.Sprintf("/t/%d", tid))
	if !strings.Contains(page, `data-decision="allow"`) || !strings.Contains(page, `data-decision="always"`) {
		t.Fatal("approval card lacks the owner's buttons")
	}
	// A stranger can't decide.
	if code, _, _ := bob.post(fmt.Sprintf("/t/%d/approvals/%d", tid, approval.ID), url.Values{"decision": {"allow"}}); code != http.StatusNotFound {
		t.Fatalf("bob decide = %d", code)
	}
	// "always" allows and switches the server to automatic.
	if code, body, _ := alice.post(fmt.Sprintf("/t/%d/approvals/%d", tid, approval.ID), url.Values{"decision": {"always"}}); code != http.StatusNoContent {
		t.Fatalf("decide = %d %s", code, body)
	}
	a.waitDone(tid)
	msgs, _ := a.st.AllMessages(tid)
	var toolOut string
	for _, m := range msgs {
		if m.Role == "tool" {
			toolOut = m.Content
		}
	}
	if toolOut != "did it" {
		t.Fatalf("tool output = %q", toolOut)
	}
	servers, _ := a.st.ListMCPServers(alice.id)
	if servers[0].Approval != store.ApprovalAuto {
		t.Fatalf("server mode after 'always' = %s", servers[0].Approval)
	}
	if code, _, _ := alice.post(fmt.Sprintf("/t/%d/approvals/%d", tid, approval.ID), url.Values{"decision": {"deny"}}); code != http.StatusConflict {
		t.Fatalf("second decision = %d, want 409", code)
	}
	// Second turn now runs the tool without asking.
	alice.post(fmt.Sprintf("/t/%d/messages", tid), url.Values{"content": {"again"}})
	a.waitDone(tid)
	if pend, _ := a.st.PendingApprovals(tid); len(pend) != 0 {
		t.Fatal("automatic server still asked")
	}
}

func TestPagingAndAdminAudit(t *testing.T) {
	a := newApp(t)
	llm := fakeLLM(t)
	mid := a.addModel(llm.URL)
	admin := a.register("admin") // first user is admin
	code, _, h := admin.post("/t", url.Values{"content": {"m0"}, "model_id": {fmt.Sprint(mid)}})
	if code != http.StatusSeeOther {
		t.Fatal("new thread")
	}
	tid := threadIDFromLocation(t, h)
	a.waitDone(tid)
	for i := 1; i < 6; i++ {
		admin.post(fmt.Sprintf("/t/%d/messages", tid), url.Values{"content": {fmt.Sprintf("m%d", i)}})
		a.waitDone(tid)
	}
	msgs, _ := a.st.AllMessages(tid)
	last := msgs[len(msgs)-1].RunID
	older := admin.get(fmt.Sprintf("/t/%d/messages?before=%d", tid, last))
	if !strings.Contains(older, `class="older"`) || !strings.Contains(older, `<div class="bubble">m4</div>`) || strings.Contains(older, `<div class="bubble">m5</div>`) {
		t.Fatalf("older page wrong: %s", older[:min(200, len(older))])
	}
	from := admin.get(fmt.Sprintf("/t/%d/messages?from=%d", tid, msgs[0].RunID))
	if strings.Count(from, "msg-user") != 6 {
		t.Fatalf("from= page has %d user messages, want 6", strings.Count(from, "msg-user"))
	}
	if !strings.Contains(from, "data-context-pct") {
		t.Fatal("context meter data missing")
	}
	// Admin actions are audited.
	admin.post("/admin/settings", url.Values{"allow_signup": {"1"}, "queue_max": {"7"}, "max_parallel_runs": {"3"}, "approval_timeout_min": {"5"}, "system_prompt": {"x"}})
	st, _ := a.st.LoadSettings()
	if st.QueueMax != 7 || st.MaxParallelRuns != 3 || st.ApprovalTimeoutMin != 5 {
		t.Fatalf("settings not saved: %+v", st)
	}
	audit := admin.get("/admin/audit")
	if !strings.Contains(audit, "settings.save") {
		entries, err := a.st.ListAudit(10)
		t.Fatalf("audit page lacks the settings entry; db entries=%+v err=%v page=%s", entries, err, audit[:min(300, len(audit))])
	}
	// Non-admin is refused.
	bob := a.register("bob")
	if s := bob.getStatus("/admin/audit"); s != http.StatusForbidden {
		t.Fatalf("bob audit = %d", s)
	}
	// PWA files and push subscription validation.
	if s := bob.getStatus("/manifest.webmanifest"); s != 200 {
		t.Fatal("manifest missing")
	}
	if s := bob.getStatus("/sw.js"); s != 200 {
		t.Fatal("service worker missing")
	}
	req, _ := http.NewRequest(http.MethodPost, a.srv.URL+"/settings/push", strings.NewReader(`{"endpoint":"http://insecure","keys":{"p256dh":"a","auth":"b"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", bob.csrf)
	r, _ := bob.http.Do(req)
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("insecure push endpoint accepted: %d", r.StatusCode)
	}
}
