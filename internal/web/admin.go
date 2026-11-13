package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"knitnox/internal/netguard"
	"knitnox/internal/store"
)

// audit records an admin action by the current user.
func (s *Server) audit(r *http.Request, action, target, details string) {
	if u := currentUser(r); u != nil {
		s.st.Audit(u.ID, u.Username, action, target, details)
	}
}

// ---- providers & models (global) ----

func (s *Server) adminProviders(w http.ResponseWriter, r *http.Request) {
	list, err := s.st.ListProviders(nil)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, "admin", map[string]any{"Tab": "providers", "Providers": list, "Base": "/admin"})
}

func (s *Server) adminProviderCreate(w http.ResponseWriter, r *http.Request) {
	s.providerCreate(w, r, adminScope())
}
func (s *Server) adminProviderDelete(w http.ResponseWriter, r *http.Request) {
	s.providerAction(w, r, adminScope(), "provider.delete", s.st.DeleteProvider)
}
func (s *Server) adminProviderToggle(w http.ResponseWriter, r *http.Request) {
	s.providerAction(w, r, adminScope(), "provider.toggle", s.st.ToggleProvider)
}
func (s *Server) adminProviderDiscover(w http.ResponseWriter, r *http.Request) {
	s.providerDiscover(w, r, adminScope())
}
func (s *Server) adminModelsBulk(w http.ResponseWriter, r *http.Request) {
	s.modelsBulk(w, r, adminScope())
}
func (s *Server) adminModelCreate(w http.ResponseWriter, r *http.Request) {
	s.modelCreate(w, r, adminScope())
}
func (s *Server) adminModelDelete(w http.ResponseWriter, r *http.Request) {
	s.modelAction(w, r, adminScope(), "model.delete", s.st.DeleteModel)
}
func (s *Server) adminModelToggle(w http.ResponseWriter, r *http.Request) {
	s.modelAction(w, r, adminScope(), "model.toggle", s.st.ToggleModel)
}
func (s *Server) adminModelVision(w http.ResponseWriter, r *http.Request) {
	s.modelAction(w, r, adminScope(), "model.vision", s.st.ToggleModelVision)
}

// ---- users ----

func (s *Server) renderUsers(w http.ResponseWriter, r *http.Request, full bool) {
	users, err := s.st.ListUsersWithQuota()
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := map[string]any{"Tab": "users", "Users": users}
	if full {
		s.render(w, r, "admin", data)
		return
	}
	if !isHX(r) {
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}
	s.renderPartial(w, r, "admin", "users", data)
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) { s.renderUsers(w, r, true) }

func (s *Server) adminUserCreate(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if err := validateCredentials(username, password); err != nil {
		s.flash(w, r, err.Error())
		return
	}
	role := "user"
	if r.FormValue("role") == "admin" {
		role = "admin"
	}
	hash, err := hashPassword(password)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	id, err := s.st.CreateUser(username, hash, role)
	if err != nil {
		s.flash(w, r, "That username is taken.")
		return
	}
	if limit := formInt(r, "token_limit", 0); limit > 0 {
		if err := s.st.SetQuota(id, store.Quota{Period: periodOf(r), TokenLimit: limit}); err != nil {
			s.log.Error("could not set the new user's allowance", "user", id, "err", err)
		}
	}
	s.audit(r, "user.create", username, "role "+role)
	s.renderUsers(w, r, false)
}

func periodOf2(v string) string {
	if v == "daily" {
		return "daily"
	}
	return "monthly"
}

func periodOf(r *http.Request) string {
	if r.FormValue("period") == "daily" {
		return "daily"
	}
	return "monthly"
}

// guardSelf prevents admins from locking themselves out.
func (s *Server) guardSelf(w http.ResponseWriter, r *http.Request) (int64, bool) {
	uid := pathID(r, "uid")
	if uid == currentUser(r).ID {
		s.flash(w, r, "You cannot do that to your own account.")
		return 0, false
	}
	return uid, true
}

func (s *Server) userName(id int64) string {
	if u, err := s.st.UserByID(id); err == nil {
		return u.Username
	}
	return fmt.Sprintf("user #%d", id)
}

func (s *Server) adminUserQuota(w http.ResponseWriter, r *http.Request) {
	uid := pathID(r, "uid")
	q := store.Quota{Period: periodOf(r), TokenLimit: max(formInt(r, "token_limit", 0), 0)}
	if err := s.st.SetQuota(uid, q); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.audit(r, "user.quota", s.userName(uid), fmt.Sprintf("%d tokens %s", q.TokenLimit, q.Period))
	s.renderUsers(w, r, false)
}

func (s *Server) adminUserToggleDisabled(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.guardSelf(w, r)
	if !ok {
		return
	}
	u, err := s.st.UserByID(uid)
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "User not found.")
		return
	}
	if err := s.st.SetUserDisabled(uid, !u.Disabled); err != nil {
		s.serverError(w, r, err)
		return
	}
	action := "user.disable"
	if u.Disabled {
		action = "user.enable"
	}
	s.audit(r, action, u.Username, "")
	s.renderUsers(w, r, false)
}

func (s *Server) adminUserToggleAdmin(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.guardSelf(w, r)
	if !ok {
		return
	}
	u, err := s.st.UserByID(uid)
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "User not found.")
		return
	}
	role := "admin"
	if u.IsAdmin() {
		role = "user"
	}
	if err := s.st.SetUserRole(uid, role); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.audit(r, "user.role", u.Username, "now "+role)
	s.renderUsers(w, r, false)
}

func (s *Server) adminUserPassword(w http.ResponseWriter, r *http.Request) {
	p := r.FormValue("password")
	if len(p) < 8 {
		s.flash(w, r, "Password must be at least 8 characters.")
		return
	}
	hash, err := hashPassword(p)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	uid := pathID(r, "uid")
	if _, err := s.st.UserByID(uid); err != nil {
		s.errorPage(w, r, http.StatusNotFound, "User not found.")
		return
	}
	if err := s.st.SetPassword(uid, hash); err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.st.DeleteUserSessions(uid); err != nil {
		s.log.Error("could not sign the user out after a password reset", "user", uid, "err", err)
	}
	s.audit(r, "user.password_reset", s.userName(uid), "")
	s.flash(w, r, "Password updated. The user has been logged out everywhere.")
}

// adminUserRename changes someone's username (for a person who wants another
// name, or who cannot sign in to change it).
func (s *Server) adminUserRename(w http.ResponseWriter, r *http.Request) {
	uid := pathID(r, "uid")
	u, err := s.st.UserByID(uid)
	if err != nil {
		s.errorPage(w, r, http.StatusNotFound, "User not found.")
		return
	}
	name := strings.TrimSpace(r.FormValue("username"))
	if err := s.st.SetUsername(uid, name); err != nil {
		if msg := usernameProblem(err); msg != "" {
			s.flash(w, r, msg)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.audit(r, "user.rename", u.Username, "now "+name)
	s.renderUsers(w, r, false)
}

func (s *Server) adminUserDelete(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.guardSelf(w, r)
	if !ok {
		return
	}
	name := s.userName(uid)
	if err := s.st.DeleteUser(uid); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.audit(r, "user.delete", name, "")
	s.renderUsers(w, r, false)
}

// ---- usage ----

func (s *Server) adminUsage(w http.ResponseWriter, r *http.Request) {
	rng := r.URL.Query().Get("range")
	now := time.Now().UTC()
	var since time.Time
	switch rng {
	case "today":
		since = store.PeriodStart("daily", now)
	case "7d":
		since = now.AddDate(0, 0, -7)
	case "all":
		since = time.Time{}
	default:
		rng = "30d"
		since = now.AddDate(0, 0, -30)
	}
	byUser, err := s.st.UsageReport(since)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	byModel, err := s.st.ModelUsageReport(since)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	var total store.UsageRow
	for _, m := range byModel {
		total.Requests += m.Requests
		total.PromptTokens += m.PromptTokens
		total.CompletionTokens += m.CompletionTokens
		total.Cost += m.Cost
	}
	s.render(w, r, "admin", map[string]any{
		"Tab": "usage", "Range": rng, "ByUser": byUser, "ByModel": byModel, "Total": total,
		"Ranges": []string{"today", "7d", "30d", "all"}, "Running": s.eng.RunningCount(),
	})
}

// ---- audit log ----

func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := s.st.ListAudit(300)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, "admin", map[string]any{"Tab": "audit", "Entries": entries})
}

// ---- settings ----

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.st.LoadSettings()
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	models, _ := s.st.AllGlobalModelOptions()
	providers, _ := s.st.ListProviders(nil) // for the memory embedding model
	s.render(w, r, "admin", map[string]any{"Tab": "settings", "S": st, "Models": models, "Providers": providers, "Backup": s.backupStatus(st),
		"Saved": r.URL.Query().Get("saved") == "1"})
}

func (s *Server) adminSettingsSave(w http.ResponseWriter, r *http.Request) {
	clamp := func(name string, def, lo, hi int64) int {
		return int(min(max(formInt(r, name, def), lo), hi))
	}
	d := store.DefaultSettings()
	st := store.Settings{
		AllowSignup:            r.FormValue("allow_signup") == "1",
		QueueMax:               clamp("queue_max", int64(d.QueueMax), 1, 100),
		AgentMaxSteps:          clamp("agent_max_steps", int64(d.AgentMaxSteps), 1, 50),
		ToolMaxRetries:         clamp("tool_max_retries", int64(d.ToolMaxRetries), 0, 10),
		LLMMaxRetries:          clamp("llm_max_retries", int64(d.LLMMaxRetries), 0, 10),
		CompactionPairs:        clamp("compaction_pairs", int64(d.CompactionPairs), 0, 10000),
		CompactionKeep:         clamp("compaction_keep", int64(d.CompactionKeep), 1, 1000),
		CompactionContextRatio: min(max(formFloat(r, "compaction_context_ratio"), 0), 0.95),
		CompactionModelID:      formInt(r, "compaction_model_id", 0),
		SystemPrompt:           strings.TrimSpace(r.FormValue("system_prompt")),
		MaxParallelRuns:        clamp("max_parallel_runs", int64(d.MaxParallelRuns), 0, 1000),
		ApprovalTimeoutMin:     clamp("approval_timeout_min", int64(d.ApprovalTimeoutMin), 1, 1440),
		LLMIdleSeconds:         clamp("llm_idle_seconds", int64(d.LLMIdleSeconds), 0, 3600),
		RunMaxMinutes:          clamp("run_max_minutes", int64(d.RunMaxMinutes), 0, 1440),
		BackupEveryHours:       clamp("backup_every_hours", int64(d.BackupEveryHours), 0, 720),
		BackupKeep:             clamp("backup_keep", int64(d.BackupKeep), 1, 365),
		UsageKeepDays:          clamp("usage_keep_days", int64(d.UsageKeepDays), 0, 36500),
		AuditKeepDays:          clamp("audit_keep_days", int64(d.AuditKeepDays), 0, 36500),
		AgentRunsKeep:          clamp("agent_runs_keep", int64(d.AgentRunsKeep), 0, 100000),
		SignupTokenLimit:       max(formInt(r, "signup_token_limit", 0), 0),
		SignupPeriod:           periodOf2(r.FormValue("signup_period")),
		BlockPrivateURLs:       r.FormValue("block_private_urls") == "1",
		EmbedProviderID:        formInt(r, "embed_provider_id", 0),
		EmbedModel:             strings.TrimSpace(r.FormValue("embed_model")),
		EmbedDimensions:        clamp("embed_dimensions", 0, 0, 8192),
		FeedPublic:             r.FormValue("feed_public") == "1",
		FeedMediaDaily:         clamp("feed_media_daily", int64(d.FeedMediaDaily), 0, 10000),
		ShareLinks:             r.FormValue("share_links") == "1",
	}
	if p, err := s.st.ProviderByID(st.EmbedProviderID); err != nil || !p.IsGlobal() {
		st.EmbedProviderID = 0 // only shared providers can serve everyone
	}
	if st.AgentRunsKeep > 0 && st.AgentRunsKeep < 20 {
		st.AgentRunsKeep = 20 // an agent's daily limit and its digest of recent runs need some history
	}
	if err := s.st.SaveSettings(st); err != nil {
		s.serverError(w, r, err)
		return
	}
	netguard.SetBlockPrivate(st.BlockPrivateURLs)
	s.audit(r, "settings.save", "", fmt.Sprintf("signup=%v queue=%d steps=%d parallel=%d", st.AllowSignup, st.QueueMax, st.AgentMaxSteps, st.MaxParallelRuns))
	http.Redirect(w, r, "/admin/settings?saved=1", http.StatusSeeOther)
}
