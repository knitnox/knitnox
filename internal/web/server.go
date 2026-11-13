// Package web is the HTTP layer: routing, auth middleware, templates, handlers.
package web

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"knitnox/internal/engine"
	"knitnox/internal/hub"
	"knitnox/internal/secret"
	"knitnox/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	st     *store.Store
	eng    *engine.Engine
	hub    *hub.Hub
	log    *slog.Logger
	pages  map[string]*template.Template
	secure bool // set Secure on cookies (behind HTTPS)

	dataDir   string // where knitnox.db and secret.key live
	backupDir string // automatic backups go here ("" = off)
	logins    *loginLimiter

	vapidPublic, vapidPrivate string
	watchMu                   sync.Mutex
	watchers                  map[int64]map[int64]int // thread -> user -> open SSE streams
	assetVersion              string                  // hash of the embedded static files, for cache busting
}

// staticVersion hashes every embedded static file so asset URLs change on
// each build and browsers (and the service worker) never serve stale CSS/JS.
func staticVersion() string {
	h := sha256.New()
	fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, _ := staticFS.ReadFile(p)
		h.Write([]byte(p))
		h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:10]
}

type Options struct {
	SecureCookies bool
	DataDir       string // the data directory (for backups)
	BackupDir     string // where automatic backups are written; "" turns them off
}

func New(st *store.Store, eng *engine.Engine, h *hub.Hub, log *slog.Logger, opt Options) (*Server, error) {
	s := &Server{st: st, eng: eng, hub: h, log: log, secure: opt.SecureCookies,
		dataDir: opt.DataDir, backupDir: opt.BackupDir,
		logins: newLoginLimiter(), watchers: map[int64]map[int64]int{}, assetVersion: staticVersion()}
	if err := s.parseTemplates(); err != nil {
		return nil, err
	}
	if err := s.initPush(); err != nil {
		return nil, fmt.Errorf("push keys: %w", err)
	}
	eng.OnRunFinished = s.onRunFinished
	eng.OnApproval = s.onApproval
	return s, nil
}

// parseTemplates builds one template set per page: layout + partials + page.
func (s *Server) parseTemplates() error {
	funcs := template.FuncMap{
		"fmtInt": fmtInt,
		"int64":  func(n int) int64 { return int64(n) },
		"mask":   secret.Mask,
		"initial": func(name string) string { // avatar letter
			for _, r := range name {
				return strings.ToUpper(string(r))
			}
			return "?"
		},
		"fmtCost": func(f float64) string { return fmt.Sprintf("$%.4f", f) },
		"pct": func(used, limit int64) int {
			if limit <= 0 {
				return 0
			}
			return int(min(100, used*100/limit))
		},
		"short": func(s string) string { // "2026-11-09 12:00:00" -> "11-09 12:00"
			if len(s) >= 16 {
				return s[5:16]
			}
			return s
		},
		"fmtBytes": func(n int64) string {
			switch {
			case n >= 1<<20:
				return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
			case n >= 1<<10:
				return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
			}
			return fmt.Sprintf("%d B", n)
		},
		"list": func(items ...string) []string { return items },
		"dict_item": func(t store.ThreadListItem, cur any) map[string]any { // sidebar row + current id
			var id int64
			switch v := cur.(type) {
			case int64:
				id = v
			case int:
				id = int64(v)
			}
			return map[string]any{"T": t, "Cur": id}
		},
		"toolName": func(n string) string { return toolPrefix.ReplaceAllString(n, "") }, // s12_clock -> clock
		"asset":    func(p string) string { return "/static/" + p + "?v=" + s.assetVersion },
		"schedule": scheduleLabel,                                                            // 60 -> "Every hour"
		"wrap":     func(key string, v any) map[string]any { return map[string]any{key: v} }, // pass one value to a partial
		"aboutOnly": func(m store.Memory) []store.MemoryEntity { // things a memory names outside its connections
			inRel := map[int64]bool{}
			for _, r := range m.Relations {
				inRel[r.SubjectID], inRel[r.ObjectID] = true, true
			}
			var out []store.MemoryEntity
			for _, e := range m.Entities {
				if !inRel[e.ID] {
					out = append(out, e)
				}
			}
			return out
		},
		"toolIn": func(t *store.Thread, r toolRow) map[string]any { // one row of the Tools menu and its chat
			return map[string]any{"R": r, "Thread": t}
		},
		"sharedModels": func(opts []store.ModelOption) []store.ModelOption { return modelsWhere(opts, false) }, // the server's models
		"ownModels":    func(opts []store.ModelOption) []store.ModelOption { return modelsWhere(opts, true) },  // the person's own
		"add":          func(a, b int) int { return a + b },
		"kindLabel":    modelKindLabel, // image -> "Pictures"
		"modelKinds": func() []string { // every kind, in the order shown
			return []string{store.ModelChat, store.ModelEmbedding, store.ModelImage, store.ModelVoice, store.ModelVideo}
		},
		"fmtK": func(n int) string { // 123456 -> "123k"
			if n >= 1000 {
				return fmt.Sprintf("%dk", n/1000)
			}
			return strconv.Itoa(n)
		},
	}
	s.pages = map[string]*template.Template{}
	pages := []string{"auth", "chat", "settings", "admin", "invite", "error", "search", "agents", "help", "memory", "planbook", "feed", "group", "shared"}
	for _, p := range pages {
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(templateFS,
			"templates/layout.html", "templates/partials.html", "templates/icons.html", "templates/"+p+".html")
		if err != nil {
			return fmt.Errorf("template %s: %w", p, err)
		}
		s.pages[p] = t
	}
	return nil
}

// modelsWhere keeps the models that are (or are not) on a provider of the person's own.
func modelsWhere(opts []store.ModelOption, personal bool) []store.ModelOption {
	var out []store.ModelOption
	for _, o := range opts {
		if o.Personal == personal {
			out = append(out, o)
		}
	}
	return out
}

// toolPrefix is the per-server prefix the engine adds to exposed tool names.
var toolPrefix = regexp.MustCompile(`^s\d+_`)

func fmtInt(n int64) string {
	str := strconv.FormatInt(n, 10)
	if n < 0 {
		return str
	}
	var b strings.Builder
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// Handler builds the router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheStatic(http.FileServerFS(static))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { // "ok" only when the database answers
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.st.Ping(ctx); err != nil {
			s.log.Error("health check: the database does not answer", "err", err)
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	// PWA files must live at the root so the service worker can control every page.
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, static, "sw.js")
	})
	mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		http.ServeFileFS(w, r, static, "manifest.webmanifest")
	})

	// public
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /register", s.registerPage)
	mux.HandleFunc("POST /register", s.register)
	// The read-only link of a personal chat: for anyone who has it, no account needed.
	mux.HandleFunc("GET /s/{tok}", s.sharedPage)
	mux.HandleFunc("GET /s/{tok}/files/{aid}", s.sharedFile)

	// signed-in users
	u := func(p string, h http.HandlerFunc) { mux.Handle(p, s.requireUser(h)) }
	u("POST /logout", s.logout)
	u("GET /{$}", s.home)
	u("GET /search", s.searchPage)
	u("POST /t", s.newThread)
	u("GET /t/{id}", s.threadPage)
	u("GET /t/{id}/messages", s.threadMessages)
	u("GET /t/{id}/events", s.threadEvents)
	u("GET /t/{id}/export", s.exportThread)
	u("POST /t/{id}/messages", s.sendMessage)
	u("POST /t/{id}/stop", s.stopRun)
	u("POST /t/{id}/flag", s.threadFlag)
	u("POST /t/{id}/rename", s.renameThread)
	u("POST /t/{id}/runs/{rid}/cancel", s.cancelQueued)
	u("POST /t/{id}/runs/{rid}/edit", s.editRun)
	u("POST /t/{id}/approvals/{aid}", s.decideApproval)
	u("GET /files/{aid}", s.serveFile)
	u("POST /t/{id}/messages/{mid}/speak", s.speakMessage)
	u("GET /t/{id}/panel", s.threadPanel)
	u("POST /t/{id}/settings", s.threadSettings)
	u("POST /t/{id}/tools", s.threadToolSet)
	u("POST /t/{id}/delete", s.deleteThread)
	u("POST /t/{id}/leave", s.leaveThread)
	u("POST /t/{id}/invites", s.createInvite)
	u("POST /t/{id}/invites/{tok}/revoke", s.revokeInvite)
	u("POST /t/{id}/members/{uid}/remove", s.removeMember)
	u("GET /invite/{tok}", s.invitePage)
	u("POST /invite/{tok}", s.acceptInvite)

	u("GET /groups/new", s.groupNewPage)
	u("POST /groups", s.groupCreate)
	u("POST /t/{id}/notice", s.groupNoticeDismiss)
	u("GET /t/{id}/memory", s.groupMemory)
	u("POST /t/{id}/memory/settings", s.groupMemorySettings)
	u("POST /t/{id}/memory/{mid}/delete", s.groupMemoryDelete)
	u("POST /t/{id}/mcp", s.groupMCPCreate)
	u("POST /t/{id}/mcp/{sid}", s.groupMCPAction)
	u("POST /t/{id}/share", s.shareSave)
	u("POST /t/{id}/share/stop", s.shareStop)

	u("GET /agents", s.agentsPage)
	u("GET /agents/new", s.agentNewPage)
	u("GET /agents/pulse", s.agentsPulse)
	u("POST /agents", s.agentCreate)
	u("GET /agents/{id}/panel", s.agentPanel)
	u("POST /agents/{id}/settings", s.agentSettings)
	u("POST /agents/{id}/toggle", s.agentToggle)
	u("POST /agents/{id}/run", s.agentRun)
	u("POST /agents/{id}/memory/clear", s.agentClearMemory)
	u("POST /agents/{id}/delete", s.agentDelete)
	u("POST /agents/{id}/tasks", s.agentTaskCreate)
	u("POST /agents/{id}/tasks/{tid}", s.agentTaskAction)
	u("POST /agents/{id}/publish", s.agentPublish)
	u("POST /agents/{id}/mcp", s.agentMCPCreate)
	u("POST /agents/{id}/mcp/{sid}", s.agentMCPAction)
	u("POST /t/{id}/agents/{aid}", s.threadAgentToggle)
	u("GET /store", s.storePage)
	u("GET /store/{sid}", s.storeDetail)
	u("POST /store/{sid}/install", s.storeInstall)
	u("POST /store/{sid}/delete", s.storeDelete)

	u("GET /memory", s.memoryPage)
	u("POST /memory", s.memoryCreate)
	u("GET /memory/m/{id}", s.memoryCard)
	u("GET /memory/m/{id}/edit", s.memoryEdit)
	u("POST /memory/m/{id}", s.memoryUpdate)
	u("POST /memory/m/{id}/pin", s.memoryPin)
	u("POST /memory/m/{id}/delete", s.memoryDelete)
	u("GET /memory/things", s.memoryThings)
	u("GET /memory/things/{eid}", s.memoryThing)
	u("POST /memory/things/{eid}", s.memoryThingRename)
	u("GET /memory/graph", s.memoryGraph)
	u("GET /memory/settings", s.memorySettingsPage)
	u("POST /memory/settings", s.memorySwitch)
	u("POST /memory/settings/search", s.memorySearchSettings)
	u("POST /memory/settings/test", s.memorySearchTest)
	u("POST /memory/reindex", s.memoryReindex)
	u("POST /memory/delete-all", s.memoryDeleteAll)
	u("GET /memory/export", s.memoryExport)

	planUserActor := store.PlanActor{Kind: store.PlanByUser}
	u("GET /plan", s.planPage)
	u("POST /plan/tasks", s.planTaskCreate)
	u("GET /plan/tasks/{id}", s.planTaskCard)
	u("GET /plan/tasks/{id}/edit", s.planTaskEdit)
	u("POST /plan/tasks/{id}", s.planTaskUpdate)
	u("POST /plan/tasks/{id}/status", s.planTaskStatus)
	u("POST /plan/tasks/{id}/due", s.planTaskDue)
	u("POST /plan/tasks/{id}/delete", s.planTaskDelete)
	u("POST /plan/late/today", s.planLateToday)
	u("POST /plan/tasks/{id}/restore", s.planTaskAction(func(uid, id int64) error { return s.st.RestorePlanTask(uid, id, planUserActor) }))
	u("POST /plan/tasks/{id}/purge", s.planTaskAction(s.st.PurgePlanTask))
	u("POST /plan/bin/empty", s.planBinEmpty)
	u("GET /plan/projects", s.planProjectsPage)
	u("POST /plan/projects", s.planProjectCreate)
	u("POST /plan/projects/{pid}", s.planProjectUpdate)
	u("POST /plan/projects/{pid}/delete", s.planProjectDelete)
	u("GET /plan/settings", s.planSettingsPage)
	u("POST /plan/settings", s.planSwitch)

	u("GET /feed", s.feedPage)
	u("POST /feed", s.feedCreate)
	u("GET /feed/more", s.feedMore)
	u("GET /feed/new", s.feedNew)
	u("GET /feed/count", s.feedCount)
	u("GET /feed/events", s.feedEvents)
	u("GET /feed/p/{id}", s.feedCard)
	u("GET /feed/p/{id}/edit", s.feedEdit)
	u("GET /feed/p/{id}/picture.svg", s.feedPicture)
	u("POST /feed/p/{id}", s.feedUpdate)
	u("POST /feed/p/{id}/share", s.feedShare)
	u("POST /feed/p/{id}/delete", s.feedDelete)
	u("POST /feed/p/{id}/restore", s.feedRestore)
	u("POST /feed/p/{id}/purge", s.feedPurge)
	u("POST /feed/bin/empty", s.feedBinEmpty)
	u("GET /feed/settings", s.feedSettingsPage)
	u("POST /feed/settings", s.feedSwitch)
	u("POST /feed/settings/models", s.feedModelsSave)
	u("POST /feed/settings/models/add", s.feedModelAdd)
	u("GET /feed/p/{id}/media", s.feedMedia)
	u("POST /memory/settings/model/add", s.memoryModelAdd)

	u("GET /help", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/help/guide", http.StatusSeeOther) })
	u("GET /help/{doc}", s.helpPage)
	u("GET /issues", s.issuesPage)
	u("POST /issues", s.issueCreate)
	u("POST /issues/{iid}", s.issueAction)

	u("GET /settings", s.settingsPage)
	u("POST /settings/password", s.changePassword)
	u("POST /settings/username", s.changeUsername)
	u("POST /settings/instructions", s.saveInstructions)
	u("POST /settings/timezone", s.saveTimezone)
	u("POST /settings/logout-all", s.logoutEverywhere)
	u("POST /settings/providers", s.userProviderCreate)
	u("POST /settings/providers/{pid}/delete", s.userProviderDelete)
	u("POST /settings/providers/{pid}/toggle", s.userProviderToggle)
	u("POST /settings/providers/{pid}/models", s.userModelCreate)
	u("POST /settings/providers/{pid}/discover", s.userProviderDiscover)
	u("POST /settings/providers/{pid}/models/bulk", s.userModelsBulk)
	u("POST /settings/models/{mid}/delete", s.userModelDelete)
	u("POST /settings/models/{mid}/toggle", s.userModelToggle)
	u("POST /settings/models/{mid}/vision", s.userModelVision)
	u("POST /settings/models/{mid}/kind", s.userModelKind)
	u("POST /settings/mcp", s.mcpCreate)
	u("POST /settings/mcp/{sid}/delete", s.mcpDelete)
	u("POST /settings/mcp/{sid}/toggle", s.mcpToggle)
	u("POST /settings/mcp/{sid}/approval", s.mcpApproval)
	u("POST /settings/mcp/{sid}/test", s.mcpTest)
	u("POST /settings/push", s.pushSubscribe)
	u("POST /settings/push/unsubscribe", s.pushUnsubscribe)
	u("POST /settings/push/test", s.pushTest)

	// admins
	a := func(p string, h http.HandlerFunc) { mux.Handle(p, s.requireUser(s.requireAdmin(h))) }
	a("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/providers", http.StatusSeeOther)
	})
	a("GET /admin/providers", s.adminProviders)
	a("POST /admin/providers", s.adminProviderCreate)
	a("POST /admin/providers/{pid}/delete", s.adminProviderDelete)
	a("POST /admin/providers/{pid}/toggle", s.adminProviderToggle)
	a("POST /admin/providers/{pid}/models", s.adminModelCreate)
	a("POST /admin/providers/{pid}/discover", s.adminProviderDiscover)
	a("POST /admin/providers/{pid}/models/bulk", s.adminModelsBulk)
	a("POST /admin/models/{mid}/delete", s.adminModelDelete)
	a("POST /admin/models/{mid}/toggle", s.adminModelToggle)
	a("POST /admin/models/{mid}/vision", s.adminModelVision)
	a("POST /admin/models/{mid}/kind", s.adminModelKind)
	a("GET /admin/users", s.adminUsers)
	a("POST /admin/users", s.adminUserCreate)
	a("POST /admin/users/{uid}/quota", s.adminUserQuota)
	a("POST /admin/users/{uid}/toggle-disabled", s.adminUserToggleDisabled)
	a("POST /admin/users/{uid}/toggle-admin", s.adminUserToggleAdmin)
	a("POST /admin/users/{uid}/password", s.adminUserPassword)
	a("POST /admin/users/{uid}/delete", s.adminUserDelete)
	a("GET /admin/usage", s.adminUsage)
	a("GET /admin/audit", s.adminAudit)
	a("GET /admin/settings", s.adminSettings)
	a("POST /admin/settings", s.adminSettingsSave)
	a("POST /admin/settings/embed-test", s.adminEmbedTest)
	a("POST /admin/backup", s.adminBackupNow)
	a("POST /admin/users/{uid}/username", s.adminUserRename)
	a("POST /feed/p/{id}/unshare", s.feedUnshare)

	return s.recoverer(s.securityHeaders(mux))
}

// cacheStatic lets versioned asset URLs (?v=hash) be cached for a year; unversioned ones must revalidate.
func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if strings.HasPrefix(r.URL.Path, "fonts/") {
			// style.css names the fonts without a version, so every page used to
			// ask the server about them again before text could be drawn in its
			// real font. They change next to never: keep them for a week.
			w.Header().Set("Cache-Control", "public, max-age=604800")
		} else {
			w.Header().Set("Cache-Control", "no-cache") // unversioned (e.g. images referenced from CSS): always revalidate
		}
		h.ServeHTTP(w, r)
	})
}

// ---- rendering helpers ----

type threadGroup struct {
	Label string
	Items []store.ThreadListItem
}

// groupThreads buckets chats for the sidebar: Pinned, then the groups the
// person is in, then their own chats by last activity (UTC days). Archived
// chats are returned separately.
func groupThreads(list []store.ThreadListItem, now time.Time) (groups []threadGroup, archived []store.ThreadListItem) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	bounds := []struct {
		label string
		since time.Time
	}{
		{"Today", today},
		{"Yesterday", today.AddDate(0, 0, -1)},
		{"Previous 7 days", today.AddDate(0, 0, -7)},
		{"Previous 30 days", today.AddDate(0, 0, -30)},
		{"Older", time.Time{}},
	}
	add := func(label string, t store.ThreadListItem) {
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, threadGroup{Label: label})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, t)
	}
	// Pinned first, then groups: both lists are in order of activity already,
	// so two passes keep each section together.
	for _, t := range list {
		if !t.Archived && t.Pinned {
			add("Pinned", t)
		}
	}
	for _, t := range list {
		if !t.Archived && !t.Pinned && t.Group {
			add("Groups", t)
		}
	}
	for _, t := range list {
		if t.Archived {
			archived = append(archived, t)
			continue
		}
		if t.Pinned || t.Group {
			continue
		}
		at, _ := time.Parse("2006-01-02 15:04:05", t.UpdatedAt)
		for _, b := range bounds {
			if !at.Before(b.since) {
				add(b.label, t)
				break
			}
		}
	}
	return groups, archived
}

// render executes a page template (the full layout).
func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data map[string]any) {
	s.renderTemplate(w, r, page, "layout.html", data)
}

// renderPartial executes one named template from a page set (htmx fragment).
func (s *Server) renderPartial(w http.ResponseWriter, r *http.Request, page, name string, data map[string]any) {
	s.renderTemplate(w, r, page, name, data)
}

func (s *Server) renderTemplate(w http.ResponseWriter, r *http.Request, page, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["Theme"], data["Accent"] = appearance(r)
	data["Nav"] = page // which sidebar link is the current one
	if u := currentUser(r); u != nil {
		data["Me"] = u
		data["LocalNow"] = time.Now().In(u.Location()).Format("15:04 on Monday, 2 January")
		data["CSRF"] = csrfToken(r)
		data["VapidPublic"] = s.vapidPublic
		if name == "layout.html" { // full pages get the sidebar's chat list and agents
			threads, ok := data["Threads"].([]store.ThreadListItem)
			if !ok {
				threads, _ = s.st.ListThreadsForUser(u.ID)
			}
			data["ThreadGroups"], data["Archived"] = groupThreads(threads, time.Now().UTC())
			data["HasChats"] = len(threads) > 0
			agents, _ := s.st.ListAgents(u.ID)
			data["MyAgents"] = agents
			sum := countAgents(agents) // the two small numbers next to "Agents"
			data["AgentsWorking"], data["AgentsTrouble"] = sum.Working, sum.Trouble
			data["PlanDue"] = s.st.PlanCounts(u.ID, 0, engine.PlanToday(u.Location())).Today // tasks due today or late
			data["FeedNew"] = s.st.FeedUnread(u.ID, s.feedPublicOn())                        // posts not looked at yet
		}
	}
	t := s.pages[page]
	if t == nil {
		http.Error(w, "unknown page "+page, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		s.log.Error("render", "page", page, "tmpl", name, "err", err)
	}
}

// Appearance is a per-browser choice kept in two cookies that app.js writes
// (Settings → Appearance). The server only copies them onto <html>, so the
// right colours are there on first paint. Unknown values fall back to defaults.
var (
	themes  = map[string]bool{"auto": true, "light": true, "dark": true}
	accents = map[string]bool{"iris": true, "ocean": true, "forest": true, "sunset": true, "rose": true, "graphite": true}
)

func appearance(r *http.Request) (theme, accent string) {
	theme, accent = "auto", "iris"
	if c, err := r.Cookie("kn_theme"); err == nil && themes[c.Value] {
		theme = c.Value
	}
	if c, err := r.Cookie("kn_accent"); err == nil && accents[c.Value] {
		accent = c.Value
	}
	return theme, accent
}

func (s *Server) errorPage(w http.ResponseWriter, r *http.Request, code int, msg string) {
	if isFetch(r) {
		http.Error(w, msg, code)
		return
	}
	if isHX(r) {
		w.Header().Set("HX-Retarget", "#flash")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(code)
		template.HTMLEscape(w, []byte(msg))
		return
	}
	w.WriteHeader(code)
	s.render(w, r, "error", map[string]any{"Code": code, "Message": msg})
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "path", r.URL.Path, "err", err)
	s.errorPage(w, r, 500, "Something went wrong: "+err.Error())
}

func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// back redirects non-htmx form posts to the referring page.
func back(w http.ResponseWriter, r *http.Request, fallback string) {
	to := fallback
	if ref := r.Header.Get("Referer"); ref != "" {
		to = ref
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func pathID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id
}

func parseInt(v string) (int64, error) { return strconv.ParseInt(strings.TrimSpace(v), 10, 64) }

func formInt(r *http.Request, name string, def int64) int64 {
	v := strings.TrimSpace(r.FormValue(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func formFloat(r *http.Request, name string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue(name)), 64)
	return f
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				s.log.Error("panic", "path", r.URL.Path, "value", v)
				http.Error(w, "internal error", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		// No 'unsafe-inline' anywhere: all styles and scripts are files.
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob: https:; font-src 'self'; "+
			"style-src 'self'; script-src 'self'; connect-src 'self'; worker-src 'self'; manifest-src 'self'; "+
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

// housekeeping removes old rows by the limits in Admin → Settings and lets
// SQLite tidy itself.
func (s *Server) housekeeping() {
	st, err := s.st.LoadSettings()
	if err != nil {
		s.log.Warn("housekeeping: cannot read the settings", "err", err)
		return
	}
	report := func(what string, n int64, err error) {
		if err != nil {
			s.log.Warn("housekeeping failed", "what", what, "err", err)
		} else if n > 0 {
			s.log.Info("housekeeping removed old rows", "what", what, "count", n)
		}
	}
	n, err := s.st.PruneUsage(st.UsageKeepDays)
	report("usage log", n, err)
	n, err = s.st.PruneAudit(st.AuditKeepDays)
	report("audit log", n, err)
	n, err = s.st.PruneAgentRuns(st.AgentRunsKeep)
	report("old agent runs", n, err)
	if err := s.st.Optimize(); err != nil {
		s.log.Warn("housekeeping: optimize failed", "err", err)
	}
}

// StartJanitor periodically removes expired sessions, orphaned uploads and
// stale login-limiter entries.
func (s *Server) StartJanitor(ctx context.Context) {
	sweep := func() {
		s.st.PurgeExpiredSessions()
		s.logins.sweep()
		if n, err := s.st.PurgeOldPlanBin(); err != nil {
			s.log.Warn("plan book bin sweep failed", "err", err)
		} else if n > 0 {
			s.log.Info("removed old tasks from Plan Book bins", "count", n)
		}
		if n, err := s.st.PurgeOldFeedBin(); err != nil {
			s.log.Warn("feed bin sweep failed", "err", err)
		} else if n > 0 {
			s.log.Info("removed old posts from feed bins", "count", n)
		}
		if n, err := s.st.SweepFeedMedia(); err != nil {
			s.log.Warn("feed media sweep failed", "err", err)
		} else if n > 0 {
			s.log.Info("removed feed media files that no post uses", "count", n)
		}
		if n, err := s.st.SweepOrphanUploads(); err != nil {
			s.log.Warn("upload sweep failed", "err", err)
		} else if n > 0 {
			s.log.Info("removed orphaned uploads", "count", n)
		}
		s.housekeeping()
		s.autoBackup() // last: it copies the folders the sweeps above just tidied
	}
	safeSweep := func() {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic in the hourly clean-up", "value", fmt.Sprint(v))
			}
		}()
		sweep()
	}
	go func() {
		safeSweep()
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				safeSweep()
			}
		}
	}()
}
