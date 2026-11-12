package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Task statuses.
const (
	TaskPending    = "pending"
	TaskInProgress = "in_progress"
	TaskDone       = "done"
	TaskFailed     = "failed"
	TaskCancelled  = "cancelled"
)

// ValidTaskStatus reports whether s is a known task status.
func ValidTaskStatus(s string) bool {
	switch s {
	case TaskPending, TaskInProgress, TaskDone, TaskFailed, TaskCancelled:
		return true
	}
	return false
}

// Limits applied wherever an agent is created or edited.
const (
	AgentMaxStepsLimit   = 50
	AgentMinSchedule     = 5 // minutes
	AgentHistoryRunsMax  = 20
	AgentDailyRunsMax    = 500
	AgentNotesMaxLen     = 2000
	AgentMemoryMaxLen    = 4000
	agentInstructionsMax = 8000
)

// Agent is a saved background worker. Its thread holds every run it makes.
type Agent struct {
	ID              int64
	UserID          int64
	ThreadID        int64
	Name            string
	Description     string
	Instructions    string // core job, edited by the owner only
	Notes           string // adjustments, also editable by a connected chat
	Memory          string // the agent's own notes between runs
	MaxSteps        int
	ScheduleMinutes int // 0 = on demand only
	OnlyWithTasks   bool
	NextRunAt       string
	DailyRuns       int
	HistoryRuns     int
	Enabled         bool
	StoreID         int64
	CreatedAt       string
	UseMemory       bool   // may read (never write) its owner's long-term memory
	PlanAccess      string // none | read | write: what it may do with its owner's Plan Book
	FeedAccess      string // none | post: may it add posts to its owner's feed
	FeedDaily       int    // posts it may add per day
}

// Scheduled reports whether the agent runs by itself.
func (a *Agent) Scheduled() bool { return a.ScheduleMinutes > 0 }

// normalize clamps every field to its allowed range.
func (a *Agent) normalize() {
	a.Name = clip(strings.Join(strings.Fields(a.Name), " "), 60)
	a.Description = clip(strings.TrimSpace(a.Description), 300)
	a.Instructions = clip(strings.TrimSpace(a.Instructions), agentInstructionsMax)
	a.Notes = clip(strings.TrimSpace(a.Notes), AgentNotesMaxLen)
	a.MaxSteps = min(max(a.MaxSteps, 1), AgentMaxStepsLimit)
	if a.ScheduleMinutes > 0 {
		a.ScheduleMinutes = min(max(a.ScheduleMinutes, AgentMinSchedule), 60*24*30)
	} else {
		a.ScheduleMinutes = 0
	}
	a.DailyRuns = min(max(a.DailyRuns, 1), AgentDailyRunsMax)
	a.HistoryRuns = min(max(a.HistoryRuns, 1), AgentHistoryRunsMax)
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

const agentCols = `a.id, a.user_id, a.thread_id, a.name, a.description, a.instructions, a.notes, a.memory, a.max_steps,
	a.schedule_minutes, a.only_with_tasks, COALESCE(a.next_run_at, ''), a.daily_runs, a.history_runs, a.enabled,
	COALESCE(a.store_id, 0), a.created_at, a.use_memory, a.plan_access, a.feed_access, a.feed_daily`

func scanAgent(sc interface{ Scan(...any) error }, extra ...any) (*Agent, error) {
	a := &Agent{}
	dest := []any{&a.ID, &a.UserID, &a.ThreadID, &a.Name, &a.Description, &a.Instructions, &a.Notes, &a.Memory, &a.MaxSteps,
		&a.ScheduleMinutes, &a.OnlyWithTasks, &a.NextRunAt, &a.DailyRuns, &a.HistoryRuns, &a.Enabled, &a.StoreID, &a.CreatedAt, &a.UseMemory, &a.PlanAccess, &a.FeedAccess, &a.FeedDaily}
	return a, sc.Scan(append(dest, extra...)...)
}

func nextRun(minutes int) any {
	if minutes <= 0 {
		return nil
	}
	return FormatTime(time.Now().Add(time.Duration(minutes) * time.Minute))
}

// CreateAgent creates the agent and the thread that will hold its runs.
func (s *Store) CreateAgent(a *Agent, modelID int64) (int64, error) {
	a.normalize()
	if a.Name == "" {
		return 0, errors.New("an agent needs a name")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var mid any
	if modelID > 0 {
		mid = modelID
	}
	res, err := tx.Exec(`INSERT INTO threads(owner_user_id, title, model_id) VALUES (?, ?, ?)`, a.UserID, a.Name, mid)
	if err != nil {
		return 0, err
	}
	a.ThreadID, _ = res.LastInsertId()
	if _, err := tx.Exec(`INSERT INTO thread_members(thread_id, user_id, role) VALUES (?, ?, 'owner')`, a.ThreadID, a.UserID); err != nil {
		return 0, err
	}
	var storeID any
	if a.StoreID > 0 {
		storeID = a.StoreID
	}
	res, err = tx.Exec(`INSERT INTO agents(user_id, thread_id, name, description, instructions, notes, max_steps,
		schedule_minutes, only_with_tasks, next_run_at, daily_runs, history_runs, enabled, store_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.UserID, a.ThreadID, a.Name, a.Description, a.Instructions, a.Notes, a.MaxSteps,
		a.ScheduleMinutes, b2i(a.OnlyWithTasks), nextRun(a.ScheduleMinutes), a.DailyRuns, a.HistoryRuns, b2i(a.Enabled), storeID)
	if err != nil {
		return 0, err
	}
	a.ID, _ = res.LastInsertId()
	return a.ID, tx.Commit()
}

func (s *Store) AgentByID(id int64) (*Agent, error) {
	a, err := scanAgent(s.DB.QueryRow(`SELECT `+agentCols+` FROM agents a WHERE a.id = ?`, id))
	return a, notFound(err)
}

// AgentByThread returns the agent that owns a thread, or nil for a normal chat.
func (s *Store) AgentByThread(threadID int64) (*Agent, error) {
	a, err := scanAgent(s.DB.QueryRow(`SELECT `+agentCols+` FROM agents a WHERE a.thread_id = ?`, threadID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// AgentRow is an agent with the numbers shown on the agents page.
type AgentRow struct {
	Agent
	OpenTasks  int
	LastStatus string // status of its newest run ("" if it never ran)
	LastRunAt  string
	Published  bool
}

// ListAgents returns a user's agents, newest first.
func (s *Store) ListAgents(userID int64) ([]AgentRow, error) {
	rows, err := s.DB.Query(`SELECT `+agentCols+`,
		(SELECT COUNT(*) FROM agent_tasks t WHERE t.agent_id = a.id AND t.status IN ('pending', 'in_progress')),
		COALESCE((SELECT r.status FROM runs r WHERE r.thread_id = a.thread_id ORDER BY r.id DESC LIMIT 1), ''),
		COALESCE((SELECT r.created_at FROM runs r WHERE r.thread_id = a.thread_id ORDER BY r.id DESC LIMIT 1), ''),
		EXISTS (SELECT 1 FROM store_agents sa WHERE sa.source_agent_id = a.id)
		FROM agents a WHERE a.user_id = ? ORDER BY a.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentRow
	for rows.Next() {
		var r AgentRow
		a, err := scanAgent(rows, &r.OpenTasks, &r.LastStatus, &r.LastRunAt, &r.Published)
		if err != nil {
			return nil, err
		}
		r.Agent = *a
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateAgent saves the owner-editable fields and keeps the thread's title and
// model in step. The next scheduled run is recalculated only when the interval changed.
func (s *Store) UpdateAgent(a *Agent, modelID int64) error {
	a.normalize()
	if a.Name == "" {
		return errors.New("an agent needs a name")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old int
	if err := tx.QueryRow(`SELECT schedule_minutes FROM agents WHERE id = ?`, a.ID).Scan(&old); err != nil {
		return notFound(err)
	}
	if _, err := tx.Exec(`UPDATE agents SET name = ?, description = ?, instructions = ?, max_steps = ?,
		schedule_minutes = ?, only_with_tasks = ?, daily_runs = ?, history_runs = ?, enabled = ? WHERE id = ?`,
		a.Name, a.Description, a.Instructions, a.MaxSteps, a.ScheduleMinutes, b2i(a.OnlyWithTasks),
		a.DailyRuns, a.HistoryRuns, b2i(a.Enabled), a.ID); err != nil {
		return err
	}
	if old != a.ScheduleMinutes {
		if _, err := tx.Exec(`UPDATE agents SET next_run_at = ? WHERE id = ?`, nextRun(a.ScheduleMinutes), a.ID); err != nil {
			return err
		}
	}
	var mid any
	if modelID > 0 {
		mid = modelID
	}
	if _, err := tx.Exec(`UPDATE threads SET title = ?, model_id = ? WHERE id = ?`, a.Name, mid, a.ThreadID); err != nil {
		return err
	}
	return tx.Commit()
}

// SetAgentEnabled pauses or resumes an agent. Resuming restarts its interval
// so a long-paused agent does not fire the moment it is switched on.
func (s *Store) SetAgentEnabled(id int64, on bool) error {
	if !on {
		_, err := s.DB.Exec(`UPDATE agents SET enabled = 0 WHERE id = ?`, id)
		return err
	}
	a, err := s.AgentByID(id)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`UPDATE agents SET enabled = 1, next_run_at = ? WHERE id = ?`, nextRun(a.ScheduleMinutes), id)
	return err
}

// SetAgentNotes replaces the adjustments text and records the change.
// by is "user" or "chat".
func (s *Store) SetAgentNotes(id int64, notes, by string) error {
	notes = clip(strings.TrimSpace(notes), AgentNotesMaxLen)
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	if err := tx.QueryRow(`SELECT notes FROM agents WHERE id = ?`, id).Scan(&old); err != nil {
		return notFound(err)
	}
	if old == notes {
		return nil
	}
	if _, err := tx.Exec(`UPDATE agents SET notes = ? WHERE id = ?`, notes, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO agent_notes_log(agent_id, notes, changed_by) VALUES (?, ?, ?)`, id, notes, by); err != nil {
		return err
	}
	return tx.Commit()
}

// NotesChange is one saved version of an agent's notes.
type NotesChange struct {
	Notes, ChangedBy, CreatedAt string
}

// AgentNotesLog returns the latest versions of the notes, newest first.
func (s *Store) AgentNotesLog(id int64, limit int) ([]NotesChange, error) {
	rows, err := s.DB.Query(`SELECT notes, changed_by, created_at FROM agent_notes_log WHERE agent_id = ? ORDER BY id DESC LIMIT ?`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotesChange
	for rows.Next() {
		var c NotesChange
		if err := rows.Scan(&c.Notes, &c.ChangedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetAgentMemory replaces the agent's own scratchpad.
func (s *Store) SetAgentMemory(id int64, memory string) error {
	_, err := s.DB.Exec(`UPDATE agents SET memory = ? WHERE id = ?`, clip(strings.TrimSpace(memory), AgentMemoryMaxLen), id)
	return err
}

// ---- tools ----

// AgentTool is one MCP server an agent may use. Tools == "" allows every tool.
type AgentTool struct {
	ServerID int64
	Tools    string // comma-separated allowlist
}

// NormalizeToolList turns "a, b\nc" into "a,b,c".
func NormalizeToolList(v string) string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t' || r == ';' })
	seen := map[string]bool{}
	out := parts[:0]
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return clip(strings.Join(out, ","), 2000)
}

// Allows reports whether the tool name passes the allowlist.
func (t AgentTool) Allows(name string) bool {
	if t.Tools == "" {
		return true
	}
	for _, n := range strings.Split(t.Tools, ",") {
		if n == name {
			return true
		}
	}
	return false
}

// SetAgentMCP replaces the agent's tool selection. Servers that do not belong
// to ownerID are ignored.
func (s *Store) SetAgentMCP(agentID, ownerID int64, tools []AgentTool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM agent_mcp WHERE agent_id = ?`, agentID); err != nil {
		return err
	}
	for _, t := range tools {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO agent_mcp(agent_id, mcp_server_id, tools)
			SELECT ?, id, ? FROM mcp_servers WHERE id = ? AND user_id = ? AND agent_id IS NULL`, agentID, NormalizeToolList(t.Tools), t.ServerID, ownerID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AgentMCP returns the agent's selection keyed by server id.
func (s *Store) AgentMCP(agentID int64) (map[int64]AgentTool, error) {
	rows, err := s.DB.Query(`SELECT mcp_server_id, tools FROM agent_mcp WHERE agent_id = ?`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]AgentTool{}
	for rows.Next() {
		var t AgentTool
		if err := rows.Scan(&t.ServerID, &t.Tools); err != nil {
			return nil, err
		}
		out[t.ServerID] = t
	}
	return out, rows.Err()
}

// AgentMCPServers returns the enabled servers the agent may use: its own
// servers plus the owner's shared servers ticked for it.
func (s *Store) AgentMCPServers(a *Agent) ([]MCPServer, error) {
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE user_id = ? AND enabled = 1
		AND (agent_id = ? OR (agent_id IS NULL AND id IN (SELECT mcp_server_id FROM agent_mcp WHERE agent_id = ?)))
		ORDER BY id`, a.UserID, a.ID, a.ID)
	if err != nil {
		return nil, err
	}
	return s.scanMCP(rows)
}

// CreateAgentMCPServer adds a server that only this agent uses. The owner's
// chats and other agents never see it.
func (s *Store) CreateAgentMCPServer(a *Agent, name, url, headers, approval string) error {
	enc, err := s.Box.Seal(headers)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO mcp_servers(user_id, name, url, headers_enc, approval, agent_id) VALUES (?, ?, ?, ?, ?, ?)`,
		a.UserID, name, url, enc, normalizeApproval(approval), a.ID)
	return err
}

// ListAgentOwnServers returns the servers that belong to this agent only.
func (s *Store) ListAgentOwnServers(agentID int64) ([]MCPServer, error) {
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE agent_id = ? ORDER BY id`, agentID)
	if err != nil {
		return nil, err
	}
	return s.scanMCP(rows)
}

// AgentOwnServer returns one of the agent's own servers.
func (s *Store) AgentOwnServer(agentID, serverID int64) (*MCPServer, error) {
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE id = ? AND agent_id = ?`, serverID, agentID)
	if err != nil {
		return nil, err
	}
	list, err := s.scanMCP(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// ToggleThreadAgent connects an agent to a chat, or disconnects it, and
// reports the new state. The agent must belong to ownerID.
func (s *Store) ToggleThreadAgent(threadID, ownerID, agentID int64) (connected bool, err error) {
	res, err := s.DB.Exec(`DELETE FROM thread_agents WHERE thread_id = ? AND agent_id = ?`, threadID, agentID)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return false, nil
	}
	res, err = s.DB.Exec(`INSERT INTO thread_agents(thread_id, agent_id)
		SELECT ?, id FROM agents WHERE id = ? AND user_id = ? AND thread_id != ?`, threadID, agentID, ownerID, threadID)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, ErrNotFound
	}
	return true, nil
}

// ---- tasks ----

type AgentTask struct {
	ID             int64
	AgentID        int64
	Title          string
	Details        string
	Status         string
	Result         string
	CreatedBy      string // user | chat | agent
	SourceThreadID int64
	CreatedAt      string
	UpdatedAt      string
}

// Open reports whether the task still needs work.
func (t *AgentTask) Open() bool { return t.Status == TaskPending || t.Status == TaskInProgress }

const taskCols = `id, agent_id, title, details, status, result, created_by, COALESCE(source_thread_id, 0), created_at, updated_at`

func scanTask(sc interface{ Scan(...any) error }) (*AgentTask, error) {
	t := &AgentTask{}
	err := sc.Scan(&t.ID, &t.AgentID, &t.Title, &t.Details, &t.Status, &t.Result, &t.CreatedBy, &t.SourceThreadID, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// CreateTask adds a pending task. sourceThread (0 = none) is the chat that asked.
func (s *Store) CreateTask(agentID int64, title, details, createdBy string, sourceThread int64) (int64, error) {
	title = clip(strings.Join(strings.Fields(title), " "), 200)
	if title == "" {
		return 0, errors.New("a task needs a title")
	}
	var src any
	if sourceThread > 0 {
		src = sourceThread
	}
	res, err := s.DB.Exec(`INSERT INTO agent_tasks(agent_id, title, details, created_by, source_thread_id) VALUES (?, ?, ?, ?, ?)`,
		agentID, title, clip(strings.TrimSpace(details), 4000), createdBy, src)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) TaskByID(id int64) (*AgentTask, error) {
	t, err := scanTask(s.DB.QueryRow(`SELECT `+taskCols+` FROM agent_tasks WHERE id = ?`, id))
	return t, notFound(err)
}

// ListTasks returns an agent's tasks: open ones first (oldest first), then the
// most recently finished. openOnly leaves the finished ones out.
func (s *Store) ListTasks(agentID int64, openOnly bool, limit int) ([]AgentTask, error) {
	q := `SELECT ` + taskCols + ` FROM agent_tasks WHERE agent_id = ?`
	if openOnly {
		q += ` AND status IN ('pending', 'in_progress')`
	}
	q += ` ORDER BY status IN ('pending', 'in_progress') DESC,
		CASE WHEN status IN ('pending', 'in_progress') THEN id ELSE -id END LIMIT ?`
	rows, err := s.DB.Query(q, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// TaskPatch holds the fields of a task to change; nil fields are kept.
type TaskPatch struct {
	Title, Details, Status, Result *string
}

// UpdateTask applies a patch to a task of the given agent.
func (s *Store) UpdateTask(agentID, taskID int64, p TaskPatch) error {
	var sets []string
	var args []any
	add := func(col string, v *string, n int) {
		if v != nil {
			sets = append(sets, col+" = ?")
			args = append(args, clip(strings.TrimSpace(*v), n))
		}
	}
	add("title", p.Title, 200)
	add("details", p.Details, 4000)
	add("result", p.Result, 4000)
	if p.Status != nil {
		if !ValidTaskStatus(*p.Status) {
			return errors.New("unknown task status " + *p.Status)
		}
		add("status", p.Status, 20)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, Now(), taskID, agentID)
	res, err := s.DB.Exec(`UPDATE agent_tasks SET `+strings.Join(sets, ", ")+` WHERE id = ? AND agent_id = ?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTask(agentID, taskID int64) error {
	_, err := s.DB.Exec(`DELETE FROM agent_tasks WHERE id = ? AND agent_id = ?`, taskID, agentID)
	return err
}

// OpenTaskCount is the number of pending or in-progress tasks.
func (s *Store) OpenTaskCount(agentID int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM agent_tasks WHERE agent_id = ? AND status IN ('pending', 'in_progress')`, agentID).Scan(&n)
	return n
}

// ---- connecting agents to chats ----

// SetThreadAgents replaces the agents connected to a chat. Only agents owned
// by ownerID can be connected, and never an agent to its own thread.
func (s *Store) SetThreadAgents(threadID, ownerID int64, ids []int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM thread_agents WHERE thread_id = ?`, threadID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO thread_agents(thread_id, agent_id)
			SELECT ?, id FROM agents WHERE id = ? AND user_id = ? AND thread_id != ?`, threadID, id, ownerID, threadID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ConnectedAgents returns the agents a chat may manage. The owner check is
// repeated here so a connection never outlives a change of ownership.
func (s *Store) ConnectedAgents(threadID, ownerID int64) ([]Agent, error) {
	rows, err := s.DB.Query(`SELECT `+agentCols+` FROM agents a JOIN thread_agents ta ON ta.agent_id = a.id
		WHERE ta.thread_id = ? AND a.user_id = ? ORDER BY a.id`, threadID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// ---- scheduling and run history ----

// DueAgents returns enabled, scheduled agents whose next run time has passed.
func (s *Store) DueAgents(now string) ([]Agent, error) {
	rows, err := s.DB.Query(`SELECT `+agentCols+` FROM agents a
		WHERE a.enabled = 1 AND a.schedule_minutes > 0 AND a.next_run_at IS NOT NULL AND a.next_run_at <= ? ORDER BY a.next_run_at`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// BumpAgentNextRun moves the next scheduled run one interval into the future.
func (s *Store) BumpAgentNextRun(a *Agent) error {
	_, err := s.DB.Exec(`UPDATE agents SET next_run_at = ? WHERE id = ?`, nextRun(a.ScheduleMinutes), a.ID)
	return err
}

// ThreadBusy reports whether the thread has a queued or running run.
func (s *Store) ThreadBusy(threadID int64) bool {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM runs WHERE thread_id = ? AND status IN ('pending', 'running')`, threadID).Scan(&n)
	return n > 0
}

// RunsToday counts the runs a thread started since midnight UTC.
func (s *Store) RunsToday(threadID int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM runs WHERE thread_id = ? AND created_at >= ?`,
		threadID, time.Now().UTC().Format("2006-01-02")+" 00:00:00").Scan(&n)
	return n
}

// RunSummary is one past run in short form: what started it and how it ended.
type RunSummary struct {
	ID        int64
	Status    string
	Error     string
	CreatedAt string
	Trigger   string // the message that started the run
	Answer    string // the final assistant text
}

// RunSummaries returns the newest finished runs of a thread, newest first.
func (s *Store) RunSummaries(threadID int64, limit int) ([]RunSummary, error) {
	rows, err := s.DB.Query(`SELECT r.id, r.status, r.error, r.created_at,
		COALESCE((SELECT m.content FROM messages m WHERE m.run_id = r.id AND m.role = 'user' ORDER BY m.id LIMIT 1), ''),
		COALESCE((SELECT m.content FROM messages m WHERE m.run_id = r.id AND m.role = 'assistant' AND m.content != ''
			ORDER BY m.id DESC LIMIT 1), '')
		FROM runs r WHERE r.thread_id = ? AND r.status NOT IN ('pending', 'running') ORDER BY r.id DESC LIMIT ?`, threadID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunSummary
	for rows.Next() {
		var r RunSummary
		if err := rows.Scan(&r.ID, &r.Status, &r.Error, &r.CreatedAt, &r.Trigger, &r.Answer); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// InsertNote adds a finished, AI-authored message to a thread without
// starting a turn (used to report an agent's result back to the chat that asked).
func (s *Store) InsertNote(threadID int64, content string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := Now()
	res, err := tx.Exec(`INSERT INTO runs(thread_id, status, started_at, finished_at) VALUES (?, 'done', ?, ?)`, threadID, now, now)
	if err != nil {
		return err
	}
	runID, _ := res.LastInsertId()
	if _, err := tx.Exec(`INSERT INTO messages(thread_id, run_id, role, content) VALUES (?, ?, 'assistant', ?)`, threadID, runID, content); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE threads SET updated_at = ? WHERE id = ?`, now, threadID); err != nil {
		return err
	}
	return tx.Commit()
}

// ---- agent store ----

// StoreAgent is a shared, ready-made agent anyone can install.
type StoreAgent struct {
	ID              int64
	AuthorUserID    int64
	AuthorName      string
	SourceAgentID   int64
	Name            string
	Description     string
	Guide           string
	Requirements    string
	Instructions    string
	MaxSteps        int
	ScheduleMinutes int
	OnlyWithTasks   bool
	Installs        int
	UpdatedAt       string
}

const storeCols = `id, COALESCE(author_user_id, 0), author_name, COALESCE(source_agent_id, 0), name, description, guide,
	requirements, instructions, max_steps, schedule_minutes, only_with_tasks, installs, updated_at`

func scanStoreAgent(sc interface{ Scan(...any) error }) (*StoreAgent, error) {
	a := &StoreAgent{}
	err := sc.Scan(&a.ID, &a.AuthorUserID, &a.AuthorName, &a.SourceAgentID, &a.Name, &a.Description, &a.Guide,
		&a.Requirements, &a.Instructions, &a.MaxSteps, &a.ScheduleMinutes, &a.OnlyWithTasks, &a.Installs, &a.UpdatedAt)
	return a, err
}

// PublishAgent shares an agent in the store, or updates its existing listing.
// Only the configuration is copied: no tasks, memory, notes, runs or MCP servers.
func (s *Store) PublishAgent(a *Agent, authorName, description, guide, requirements string) (int64, error) {
	description = clip(strings.TrimSpace(description), 300)
	guide = clip(strings.TrimSpace(guide), 8000)
	requirements = clip(strings.TrimSpace(requirements), 2000)
	var id int64
	err := s.DB.QueryRow(`SELECT id FROM store_agents WHERE source_agent_id = ? AND author_user_id = ?`, a.ID, a.UserID).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := s.DB.Exec(`INSERT INTO store_agents(author_user_id, author_name, source_agent_id, name, description, guide,
			requirements, instructions, max_steps, schedule_minutes, only_with_tasks) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			a.UserID, authorName, a.ID, a.Name, description, guide, requirements, a.Instructions, a.MaxSteps,
			a.ScheduleMinutes, b2i(a.OnlyWithTasks))
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	case err != nil:
		return 0, err
	}
	_, err = s.DB.Exec(`UPDATE store_agents SET author_name = ?, name = ?, description = ?, guide = ?, requirements = ?,
		instructions = ?, max_steps = ?, schedule_minutes = ?, only_with_tasks = ?, updated_at = ? WHERE id = ?`,
		authorName, a.Name, description, guide, requirements, a.Instructions, a.MaxSteps, a.ScheduleMinutes,
		b2i(a.OnlyWithTasks), Now(), id)
	return id, err
}

func (s *Store) StoreAgentByID(id int64) (*StoreAgent, error) {
	a, err := scanStoreAgent(s.DB.QueryRow(`SELECT `+storeCols+` FROM store_agents WHERE id = ?`, id))
	return a, notFound(err)
}

// StoreListingFor returns the listing published from an agent, or nil.
func (s *Store) StoreListingFor(agentID int64) (*StoreAgent, error) {
	a, err := scanStoreAgent(s.DB.QueryRow(`SELECT `+storeCols+` FROM store_agents WHERE source_agent_id = ?`, agentID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// ListStoreAgents returns listings, most installed first; q filters by name or description.
func (s *Store) ListStoreAgents(q string) ([]StoreAgent, error) {
	query := `SELECT ` + storeCols + ` FROM store_agents`
	var args []any
	if q = strings.TrimSpace(q); q != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
		query += ` WHERE name LIKE ? ESCAPE '\' OR description LIKE ? ESCAPE '\'`
		args = append(args, like, like)
	}
	rows, err := s.DB.Query(query+` ORDER BY installs DESC, id DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoreAgent
	for rows.Next() {
		a, err := scanStoreAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (s *Store) DeleteStoreAgent(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM store_agents WHERE id = ?`, id)
	return err
}

// InstallStoreAgent copies a listing into a new, paused agent for userID.
func (s *Store) InstallStoreAgent(sa *StoreAgent, userID, modelID int64) (*Agent, error) {
	a := &Agent{UserID: userID, Name: sa.Name, Description: sa.Description, Instructions: sa.Instructions,
		MaxSteps: sa.MaxSteps, ScheduleMinutes: sa.ScheduleMinutes, OnlyWithTasks: sa.OnlyWithTasks,
		DailyRuns: 48, HistoryRuns: 5, Enabled: false, StoreID: sa.ID}
	if _, err := s.CreateAgent(a, modelID); err != nil {
		return nil, err
	}
	s.DB.Exec(`UPDATE store_agents SET installs = installs + 1 WHERE id = ?`, sa.ID)
	return a, nil
}
