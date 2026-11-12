package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Plan Book: a person's own projects and tasks. The person, the AI in their
// chats and their agents (when allowed) all work on the same list, so every
// change records who made it. Deleting moves a task to the bin, where it can
// be restored for PlanBinDays. Every query here filters by the owning user.

// Limits and fixed values.
const (
	PlanTitleMax    = 200
	PlanNotesMax    = 4000
	PlanProjectMax  = 80
	PlanBinDays     = 30
	PlanMaxProjects = 200

	PlanTodo  = "todo"
	PlanDoing = "doing"
	PlanDone  = "done"

	PlanByUser  = "user"
	PlanByAI    = "ai"
	PlanByAgent = "agent"

	PlanAccessNone  = "none"
	PlanAccessRead  = "read"
	PlanAccessWrite = "write"
)

var (
	ErrPlanTitle   = errors.New("a task needs a title")
	ErrPlanDue     = errors.New("the due date must look like 2026-12-31")
	ErrPlanStatus  = errors.New("the status must be todo, doing or done")
	ErrPlanProject = errors.New("a project needs a name")
)

// ValidPlanStatus reports whether s is a task status.
func ValidPlanStatus(s string) bool { return s == PlanTodo || s == PlanDoing || s == PlanDone }

// PlanEnabled reports whether the AI may use this user's Plan Book.
func (s *Store) PlanEnabled(userID int64) bool {
	var on bool
	s.DB.QueryRow(`SELECT enabled FROM plan_settings WHERE user_id = ?`, userID).Scan(&on)
	return on
}

func (s *Store) SetPlanEnabled(userID int64, on bool) error {
	_, err := s.DB.Exec(`INSERT INTO plan_settings(user_id, enabled) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET enabled = excluded.enabled`, userID, b2i(on))
	return err
}

// PlanActor says who is making a change.
type PlanActor struct {
	Kind     string // PlanByUser | PlanByAI | PlanByAgent
	Name     string // the agent's name
	ThreadID int64  // chat the change came from (new tasks remember it)
}

// PlanProject is a group of tasks.
type PlanProject struct {
	ID        int64
	Name      string
	Notes     string
	Open      int // tasks still to do
	Done      int
	CreatedAt string
}

// PlanTask is one task.
type PlanTask struct {
	ID             int64
	UserID         int64
	ProjectID      int64 // 0 = no project
	Project        string
	Title          string
	Notes          string
	Status         string
	Due            string // YYYY-MM-DD or ""
	CreatedBy      string
	ActorName      string
	SourceThreadID int64
	SourceTitle    string
	DoneAt         string
	DeletedAt      string // set while in the bin
	CreatedAt      string
	UpdatedAt      string
}

// InBin reports whether the task was deleted and can still be restored.
func (t PlanTask) InBin() bool { return t.DeletedAt != "" }

// PlanLogEntry is one recorded change to a task.
type PlanLogEntry struct {
	Actor, ActorName, Action, CreatedAt string
}

func cleanDue(due string) (string, error) {
	due = strings.TrimSpace(due)
	if due == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", due); err != nil {
		return "", ErrPlanDue
	}
	return due, nil
}

func cleanTitle(s string) string { return clip(strings.Join(strings.Fields(s), " "), PlanTitleMax) }

func logPlan(tx *sql.Tx, taskID int64, by PlanActor, action string) error {
	kind := by.Kind
	if kind != PlanByAI && kind != PlanByAgent {
		kind = PlanByUser
	}
	_, err := tx.Exec(`INSERT INTO plan_log(task_id, actor, actor_name, action) VALUES (?, ?, ?, ?)`,
		taskID, kind, clip(by.Name, 60), action)
	return err
}

// ---- projects ----

// CreateProject adds a project, or returns the existing one with that name.
func (s *Store) CreateProject(userID int64, name, notes string) (int64, error) {
	name = clip(strings.Join(strings.Fields(name), " "), PlanProjectMax)
	if name == "" {
		return 0, ErrPlanProject
	}
	key := strings.ToLower(name)
	var id int64
	err := s.DB.QueryRow(`SELECT id FROM plan_projects WHERE user_id = ? AND name_key = ?`, userID, key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM plan_projects WHERE user_id = ?`, userID).Scan(&n)
	if n >= PlanMaxProjects {
		return 0, fmt.Errorf("there are already %d projects; delete one first", n)
	}
	res, err := s.DB.Exec(`INSERT INTO plan_projects(user_id, name, name_key, notes) VALUES (?, ?, ?, ?)`,
		userID, name, key, clip(strings.TrimSpace(notes), PlanNotesMax))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateProject renames a project and replaces its notes.
func (s *Store) UpdateProject(userID, id int64, name, notes string) error {
	name = clip(strings.Join(strings.Fields(name), " "), PlanProjectMax)
	if name == "" {
		return ErrPlanProject
	}
	var other int
	s.DB.QueryRow(`SELECT COUNT(*) FROM plan_projects WHERE user_id = ? AND name_key = ? AND id != ?`,
		userID, strings.ToLower(name), id).Scan(&other)
	if other > 0 {
		return errors.New("another project already has that name")
	}
	res, err := s.DB.Exec(`UPDATE plan_projects SET name = ?, name_key = ?, notes = ? WHERE id = ? AND user_id = ?`,
		name, strings.ToLower(name), clip(strings.TrimSpace(notes), PlanNotesMax), id, userID)
	return affected(res, err)
}

// DeleteProject removes a project. Its tasks go to the bin (without a project),
// so nothing is lost by accident.
func (s *Store) DeleteProject(userID, id int64, by PlanActor) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var name string
	if err := tx.QueryRow(`SELECT name FROM plan_projects WHERE id = ? AND user_id = ?`, id, userID).Scan(&name); err != nil {
		return notFound(err)
	}
	rows, err := tx.Query(`SELECT id FROM plan_tasks WHERE project_id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, tid)
	}
	rows.Close()
	for _, tid := range ids {
		if err := logPlan(tx, tid, by, "moved it to the bin with its project “"+name+"”"); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE plan_tasks SET deleted_at = ?, updated_at = ? WHERE project_id = ? AND deleted_at IS NULL`, Now(), Now(), id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM plan_projects WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

const planProjectCols = `p.id, p.name, p.notes, p.created_at,
	(SELECT COUNT(*) FROM plan_tasks t WHERE t.project_id = p.id AND t.deleted_at IS NULL AND t.status != 'done'),
	(SELECT COUNT(*) FROM plan_tasks t WHERE t.project_id = p.id AND t.deleted_at IS NULL AND t.status = 'done')`

func scanProjects(rows *sql.Rows) ([]PlanProject, error) {
	defer rows.Close()
	var out []PlanProject
	for rows.Next() {
		var p PlanProject
		if err := rows.Scan(&p.ID, &p.Name, &p.Notes, &p.CreatedAt, &p.Open, &p.Done); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListProjects returns the user's projects by name.
func (s *Store) ListProjects(userID int64) ([]PlanProject, error) {
	rows, err := s.DB.Query(`SELECT `+planProjectCols+` FROM plan_projects p WHERE p.user_id = ? ORDER BY p.name_key`, userID)
	if err != nil {
		return nil, err
	}
	return scanProjects(rows)
}

func (s *Store) ProjectByID(userID, id int64) (*PlanProject, error) {
	rows, err := s.DB.Query(`SELECT `+planProjectCols+` FROM plan_projects p WHERE p.id = ? AND p.user_id = ?`, id, userID)
	if err != nil {
		return nil, err
	}
	list, err := scanProjects(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// ProjectByName finds a project by its name, ignoring letter case.
func (s *Store) ProjectByName(userID int64, name string) (*PlanProject, error) {
	var id int64
	err := s.DB.QueryRow(`SELECT id FROM plan_projects WHERE user_id = ? AND name_key = ?`,
		userID, strings.ToLower(strings.Join(strings.Fields(name), " "))).Scan(&id)
	if err != nil {
		return nil, notFound(err)
	}
	return s.ProjectByID(userID, id)
}

// ---- tasks ----

// PlanTaskInput is a new task.
type PlanTaskInput struct {
	Title     string
	Notes     string
	ProjectID int64 // 0 = no project
	Due       string
	Status    string // "" = todo
}

func (s *Store) ownsProject(userID, projectID int64) bool {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM plan_projects WHERE id = ? AND user_id = ?`, projectID, userID).Scan(&n)
	return n > 0
}

// CreatePlanTask adds a task for userID.
func (s *Store) CreatePlanTask(userID int64, in PlanTaskInput, by PlanActor) (int64, error) {
	title := cleanTitle(in.Title)
	if title == "" {
		return 0, ErrPlanTitle
	}
	due, err := cleanDue(in.Due)
	if err != nil {
		return 0, err
	}
	status := in.Status
	if status == "" {
		status = PlanTodo
	}
	if !ValidPlanStatus(status) {
		return 0, ErrPlanStatus
	}
	var project, thread, doneAt any
	if in.ProjectID > 0 {
		if !s.ownsProject(userID, in.ProjectID) {
			return 0, ErrNotFound
		}
		project = in.ProjectID
	}
	if by.ThreadID > 0 {
		thread = by.ThreadID
	}
	if status == PlanDone {
		doneAt = Now()
	}
	kind := by.Kind
	if kind != PlanByAI && kind != PlanByAgent {
		kind = PlanByUser
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO plan_tasks(user_id, project_id, title, notes, status, due_date, created_by, actor_name, source_thread_id, done_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, project, title, clip(strings.TrimSpace(in.Notes), PlanNotesMax),
		status, due, kind, clip(by.Name, 60), thread, doneAt)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if err := logPlan(tx, id, by, "added it"); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// PlanTaskPatch changes a task; nil fields stay as they are.
type PlanTaskPatch struct {
	Title     *string
	Notes     *string
	ProjectID *int64 // 0 = no project
	Due       *string
	Status    *string
}

var planStatusWords = map[string]string{PlanTodo: "put it back to “to do”", PlanDoing: "started it", PlanDone: "marked it done"}

// UpdatePlanTask applies a patch and records what changed and who did it.
func (s *Store) UpdatePlanTask(userID, id int64, p PlanTaskPatch, by PlanActor) error {
	old, err := s.PlanTaskByID(userID, id)
	if err != nil {
		return err
	}
	if old.InBin() {
		return errors.New("this task is in the bin; restore it first")
	}
	var sets, did []string
	var args []any
	if p.Title != nil {
		title := cleanTitle(*p.Title)
		if title == "" {
			return ErrPlanTitle
		}
		if title != old.Title {
			sets, args, did = append(sets, "title = ?"), append(args, title), append(did, "renamed it to “"+title+"”")
		}
	}
	if p.Notes != nil {
		if notes := clip(strings.TrimSpace(*p.Notes), PlanNotesMax); notes != old.Notes {
			sets, args, did = append(sets, "notes = ?"), append(args, notes), append(did, "changed the notes")
		}
	}
	if p.Due != nil {
		due, err := cleanDue(*p.Due)
		if err != nil {
			return err
		}
		if due != old.Due {
			what := "removed the due date"
			if due != "" {
				what = "set the due date to " + due
			}
			sets, args, did = append(sets, "due_date = ?"), append(args, due), append(did, what)
		}
	}
	if p.ProjectID != nil && *p.ProjectID != old.ProjectID {
		what := "took it out of its project"
		var project any
		if *p.ProjectID > 0 {
			pr, err := s.ProjectByID(userID, *p.ProjectID)
			if err != nil {
				return err
			}
			project, what = pr.ID, "moved it to “"+pr.Name+"”"
		}
		sets, args, did = append(sets, "project_id = ?"), append(args, project), append(did, what)
	}
	if p.Status != nil && *p.Status != old.Status {
		if !ValidPlanStatus(*p.Status) {
			return ErrPlanStatus
		}
		var doneAt any
		if *p.Status == PlanDone {
			doneAt = Now()
		}
		sets, args, did = append(sets, "status = ?", "done_at = ?"), append(args, *p.Status, doneAt), append(did, planStatusWords[*p.Status])
	}
	if len(sets) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sets, args = append(sets, "updated_at = ?"), append(args, Now(), id, userID)
	if _, err := tx.Exec(`UPDATE plan_tasks SET `+strings.Join(sets, ", ")+` WHERE id = ? AND user_id = ?`, args...); err != nil {
		return err
	}
	if err := logPlan(tx, id, by, strings.Join(did, "; ")); err != nil {
		return err
	}
	return tx.Commit()
}

// setPlanBin moves a task into or out of the bin.
func (s *Store) setPlanBin(userID, id int64, in bool, by PlanActor) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var deleted any
	cond, action := "deleted_at IS NOT NULL", "restored it from the bin"
	if in {
		deleted, cond, action = Now(), "deleted_at IS NULL", "moved it to the bin"
	}
	res, err := tx.Exec(`UPDATE plan_tasks SET deleted_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND `+cond, deleted, Now(), id, userID)
	if err := affected(res, err); err != nil {
		return err
	}
	if err := logPlan(tx, id, by, action); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletePlanTask moves a task to the bin.
func (s *Store) DeletePlanTask(userID, id int64, by PlanActor) error {
	return s.setPlanBin(userID, id, true, by)
}

// RestorePlanTask takes a task out of the bin.
func (s *Store) RestorePlanTask(userID, id int64, by PlanActor) error {
	return s.setPlanBin(userID, id, false, by)
}

// PurgePlanTask deletes a binned task for good.
func (s *Store) PurgePlanTask(userID, id int64) error {
	res, err := s.DB.Exec(`DELETE FROM plan_tasks WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL`, id, userID)
	return affected(res, err)
}

// EmptyPlanBin deletes everything in a user's bin for good.
func (s *Store) EmptyPlanBin(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM plan_tasks WHERE user_id = ? AND deleted_at IS NOT NULL`, userID)
	return err
}

// PurgeOldPlanBin removes tasks that sat in the bin longer than PlanBinDays (janitor).
func (s *Store) PurgeOldPlanBin() (int64, error) {
	res, err := s.DB.Exec(`DELETE FROM plan_tasks WHERE deleted_at IS NOT NULL AND deleted_at < ?`,
		FormatTime(time.Now().AddDate(0, 0, -PlanBinDays)))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

const planTaskCols = `t.id, t.user_id, COALESCE(t.project_id, 0), COALESCE(p.name, ''), t.title, t.notes, t.status, t.due_date,
	t.created_by, t.actor_name, COALESCE(t.source_thread_id, 0), COALESCE(th.title, ''),
	COALESCE(t.done_at, ''), COALESCE(t.deleted_at, ''), t.created_at, t.updated_at`
const planTaskFrom = ` FROM plan_tasks t LEFT JOIN plan_projects p ON p.id = t.project_id LEFT JOIN threads th ON th.id = t.source_thread_id `

func scanPlanTasks(rows *sql.Rows) ([]PlanTask, error) {
	defer rows.Close()
	var out []PlanTask
	for rows.Next() {
		var t PlanTask
		if err := rows.Scan(&t.ID, &t.UserID, &t.ProjectID, &t.Project, &t.Title, &t.Notes, &t.Status, &t.Due,
			&t.CreatedBy, &t.ActorName, &t.SourceThreadID, &t.SourceTitle, &t.DoneAt, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) PlanTaskByID(userID, id int64) (*PlanTask, error) {
	rows, err := s.DB.Query(`SELECT `+planTaskCols+planTaskFrom+`WHERE t.id = ? AND t.user_id = ?`, id, userID)
	if err != nil {
		return nil, err
	}
	list, err := scanPlanTasks(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// Views of the task list.
const (
	PlanViewOpen     = "open"     // everything still to do
	PlanViewToday    = "today"    // due today or overdue
	PlanViewUpcoming = "upcoming" // due after today
	PlanViewDone     = "done"
	PlanViewBin      = "bin"
)

// PlanFilter selects tasks. Today is the user's current day (YYYY-MM-DD).
type PlanFilter struct {
	View      string
	ProjectID int64 // 0 = any; -1 = tasks without a project
	Query     string
	Today     string
	Day       string // only open tasks due on this day (YYYY-MM-DD); "" = any
	Limit     int
	Offset    int
}

func (f PlanFilter) where(userID int64) (string, []any) {
	where, args := `t.user_id = ?`, []any{userID}
	switch f.View {
	case PlanViewBin:
		where += ` AND t.deleted_at IS NOT NULL`
	case PlanViewDone:
		where += ` AND t.deleted_at IS NULL AND t.status = 'done'`
	case PlanViewToday:
		where += ` AND t.deleted_at IS NULL AND t.status != 'done' AND t.due_date != '' AND t.due_date <= ?`
		args = append(args, f.Today)
	case PlanViewUpcoming:
		where += ` AND t.deleted_at IS NULL AND t.status != 'done' AND t.due_date > ?`
		args = append(args, f.Today)
	default:
		where += ` AND t.deleted_at IS NULL AND t.status != 'done'`
	}
	if f.Day != "" {
		where += ` AND t.due_date = ?`
		args = append(args, f.Day)
	}
	if f.ProjectID > 0 {
		where += ` AND t.project_id = ?`
		args = append(args, f.ProjectID)
	} else if f.ProjectID < 0 {
		where += ` AND t.project_id IS NULL`
	}
	if q := strings.ToLower(strings.TrimSpace(f.Query)); q != "" {
		where += ` AND (lower(t.title) LIKE ? ESCAPE '\' OR lower(t.notes) LIKE ? ESCAPE '\')`
		like := "%" + likeEscaper.Replace(q) + "%"
		args = append(args, like, like)
	}
	return where, args
}

// ListPlanTasks returns one page of tasks and how many match in total. Open
// tasks come soonest-due first (undated last); done and binned ones newest first.
func (s *Store) ListPlanTasks(userID int64, f PlanFilter) ([]PlanTask, int, error) {
	where, args := f.where(userID)
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM plan_tasks t WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := `t.due_date = '', t.due_date, t.status != 'doing', t.id DESC`
	switch f.View {
	case PlanViewDone:
		order = `t.done_at DESC, t.id DESC`
	case PlanViewBin:
		order = `t.deleted_at DESC, t.id DESC`
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.Query(`SELECT `+planTaskCols+planTaskFrom+`WHERE `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		append(args, limit, max(f.Offset, 0))...)
	if err != nil {
		return nil, 0, err
	}
	tasks, err := scanPlanTasks(rows)
	return tasks, total, err
}

// PlanCounts are the numbers on the view tabs (for one project, or all with 0).
type PlanCounts struct {
	Open, Today, Upcoming, Done, Bin int
	Overdue                          int // part of Today
}

func (s *Store) PlanCounts(userID, projectID int64, today string) PlanCounts {
	var c PlanCounts
	cond, args := ``, []any{today, today, today, userID}
	if projectID > 0 {
		cond, args = ` AND project_id = ?`, append(args, projectID)
	} else if projectID < 0 {
		cond = ` AND project_id IS NULL`
	}
	s.DB.QueryRow(`SELECT
		COALESCE(SUM(deleted_at IS NULL AND status != 'done'), 0),
		COALESCE(SUM(deleted_at IS NULL AND status != 'done' AND due_date != '' AND due_date <= ?), 0),
		COALESCE(SUM(deleted_at IS NULL AND status != 'done' AND due_date != '' AND due_date < ?), 0),
		COALESCE(SUM(deleted_at IS NULL AND status != 'done' AND due_date > ?), 0),
		COALESCE(SUM(deleted_at IS NULL AND status = 'done'), 0),
		COALESCE(SUM(deleted_at IS NOT NULL), 0)
		FROM plan_tasks WHERE user_id = ?`+cond, args...).
		Scan(&c.Open, &c.Today, &c.Overdue, &c.Upcoming, &c.Done, &c.Bin)
	return c
}

// PlanDayCounts says how many open tasks are due on each day from first to
// last (both YYYY-MM-DD), for one project or all (0), keyed by day.
func (s *Store) PlanDayCounts(userID, projectID int64, first, last string) map[string]int {
	out := map[string]int{}
	cond, args := ``, []any{userID, first, last}
	if projectID > 0 {
		cond, args = ` AND project_id = ?`, append(args, projectID)
	} else if projectID < 0 {
		cond = ` AND project_id IS NULL`
	}
	rows, err := s.DB.Query(`SELECT due_date, COUNT(*) FROM plan_tasks
		WHERE user_id = ? AND deleted_at IS NULL AND status != 'done' AND due_date >= ? AND due_date <= ?`+cond+` GROUP BY due_date`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var n int
		if rows.Scan(&day, &n) == nil {
			out[day] = n
		}
	}
	return out
}

// MoveLatePlanTasks gives every open task that is overdue (in one project, or
// all with 0) the due date today, and returns how many it moved.
func (s *Store) MoveLatePlanTasks(userID, projectID int64, today string, by PlanActor) (int, error) {
	cond, args := ``, []any{userID, today}
	if projectID > 0 {
		cond, args = ` AND project_id = ?`, append(args, projectID)
	} else if projectID < 0 {
		cond = ` AND project_id IS NULL`
	}
	rows, err := s.DB.Query(`SELECT id FROM plan_tasks
		WHERE user_id = ? AND deleted_at IS NULL AND status != 'done' AND due_date != '' AND due_date < ?`+cond, args...)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for i, id := range ids {
		if err := s.UpdatePlanTask(userID, id, PlanTaskPatch{Due: &today}, by); err != nil {
			return i, err
		}
	}
	return len(ids), nil
}

// PlanTaskLog returns the newest recorded changes of a task.
func (s *Store) PlanTaskLog(userID, taskID int64, limit int) ([]PlanLogEntry, error) {
	rows, err := s.DB.Query(`SELECT l.actor, l.actor_name, l.action, l.created_at FROM plan_log l
		JOIN plan_tasks t ON t.id = l.task_id WHERE l.task_id = ? AND t.user_id = ? ORDER BY l.id DESC LIMIT ?`, taskID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlanLogEntry
	for rows.Next() {
		var e PlanLogEntry
		if err := rows.Scan(&e.Actor, &e.ActorName, &e.Action, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetAgentPlanAccess sets what an agent may do with its owner's Plan Book.
func (s *Store) SetAgentPlanAccess(agentID int64, level string) error {
	if level != PlanAccessRead && level != PlanAccessWrite {
		level = PlanAccessNone
	}
	_, err := s.DB.Exec(`UPDATE agents SET plan_access = ? WHERE id = ?`, level, agentID)
	return err
}
