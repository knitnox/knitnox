package store

import (
	"database/sql"
	"errors"
)

// Run statuses.
const (
	RunPending     = "pending"
	RunRunning     = "running"
	RunDone        = "done"
	RunFailed      = "failed"
	RunCancelled   = "cancelled"
	RunInterrupted = "interrupted"
)

// Message statuses.
const (
	MsgDone        = "done"
	MsgStreaming   = "streaming"
	MsgInterrupted = "interrupted"
	MsgFailed      = "failed"
)

var ErrQueueFull = errors.New("queue is full")

type Run struct {
	ID           int64
	ThreadID     int64
	AuthorUserID int64
	Status       string
	Error        string
	Steps        int
}

type Message struct {
	ID           int64
	ThreadID     int64
	RunID        int64
	AuthorUserID sql.NullInt64
	AuthorName   string
	Role         string
	Content      string
	ToolCalls    string
	ToolCallID   string
	ToolName     string
	Status       string
	RunStatus    string
	RunError     string
	CreatedAt    string
	Edited       bool
	Reasoning    string // model "thinking" text, if the provider streams it
}

// EnqueueMessage stores a user message (with optional image attachments) and a
// pending run for it, atomically enforcing the per-thread queue limit.
func (s *Store) EnqueueMessage(threadID, authorID int64, content string, queueMax int, files []NewAttachment) (runID int64, err error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var pending int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM runs WHERE thread_id = ? AND status = 'pending'`, threadID).Scan(&pending); err != nil {
		return 0, err
	}
	if pending >= queueMax {
		return 0, ErrQueueFull
	}
	res, err := tx.Exec(`INSERT INTO runs(thread_id, author_user_id) VALUES (?, ?)`, threadID, authorID)
	if err != nil {
		return 0, err
	}
	runID, _ = res.LastInsertId()
	res, err = tx.Exec(`INSERT INTO messages(thread_id, run_id, author_user_id, role, content) VALUES (?, ?, ?, 'user', ?)`,
		threadID, runID, authorID, content)
	if err != nil {
		return 0, err
	}
	msgID, _ := res.LastInsertId()
	for _, f := range files {
		if _, err := tx.Exec(`INSERT INTO attachments(thread_id, message_id, user_id, file_name, orig_name, mime, size, kind, text)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, threadID, msgID, authorID, f.FileName, f.OrigName, f.Mime, f.Size, f.Kind, f.Text); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`UPDATE threads SET updated_at = ? WHERE id = ?`, Now(), threadID); err != nil {
		return 0, err
	}
	return runID, tx.Commit()
}

// LastRunID returns the newest run of a thread (0 if none).
func (s *Store) LastRunID(threadID int64) int64 {
	var id int64
	s.DB.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM runs WHERE thread_id = ?`, threadID).Scan(&id)
	return id
}

// UpdateUserMessage replaces the text of a run's user message.
func (s *Store) UpdateUserMessage(runID int64, content string) error {
	_, err := s.DB.Exec(`UPDATE messages SET content = ?, edited = 1 WHERE run_id = ? AND role = 'user'`, content, runID)
	return err
}

// ResetRun turns a finished run back into a pending one: its AI/tool messages
// are deleted and, if content is non-nil, the user message is replaced.
func (s *Store) ResetRun(runID int64, content *string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM messages WHERE run_id = ? AND role != 'user'`, runID); err != nil {
		return err
	}
	if content != nil {
		if _, err := tx.Exec(`UPDATE messages SET content = ?, edited = 1 WHERE run_id = ? AND role = 'user'`, *content, runID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE runs SET status = 'pending', error = '', steps = 0, started_at = NULL, finished_at = NULL
		WHERE id = ?`, runID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) NextPendingRun(threadID int64) (*Run, error) {
	r := &Run{}
	err := s.DB.QueryRow(`SELECT id, thread_id, COALESCE(author_user_id, 0), status, error, steps FROM runs
		WHERE thread_id = ? AND status = 'pending' ORDER BY id LIMIT 1`, threadID).
		Scan(&r.ID, &r.ThreadID, &r.AuthorUserID, &r.Status, &r.Error, &r.Steps)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return r, err
}

func (s *Store) RunByID(id int64) (*Run, error) {
	r := &Run{}
	err := s.DB.QueryRow(`SELECT id, thread_id, COALESCE(author_user_id, 0), status, error, steps FROM runs WHERE id = ?`, id).
		Scan(&r.ID, &r.ThreadID, &r.AuthorUserID, &r.Status, &r.Error, &r.Steps)
	return r, notFound(err)
}

func (s *Store) PendingCount(threadID int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM runs WHERE thread_id = ? AND status = 'pending'`, threadID).Scan(&n)
	return n
}

func (s *Store) StartRun(id int64) error {
	_, err := s.DB.Exec(`UPDATE runs SET status = 'running', started_at = ? WHERE id = ?`, Now(), id)
	return err
}

func (s *Store) FinishRun(id int64, status, errMsg string, steps int) error {
	_, err := s.DB.Exec(`UPDATE runs SET status = ?, error = ?, steps = ?, finished_at = ? WHERE id = ?`,
		status, errMsg, steps, Now(), id)
	return err
}

// DeletePendingRun removes a queued run and its user message.
func (s *Store) DeletePendingRun(threadID, runID int64) (bool, error) {
	res, err := s.DB.Exec(`DELETE FROM runs WHERE id = ? AND thread_id = ? AND status = 'pending'`, runID, threadID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RecoverAfterRestart marks work that was in flight when the process died.
func (s *Store) RecoverAfterRestart() error {
	if _, err := s.DB.Exec(`UPDATE runs SET status = 'interrupted', error = 'server restarted', finished_at = ?
		WHERE status = 'running'`, Now()); err != nil {
		return err
	}
	if _, err := s.DB.Exec(`UPDATE messages SET status = 'interrupted' WHERE status = 'streaming'`); err != nil {
		return err
	}
	return s.ExpirePendingApprovals()
}

// FailStreamingMessages closes the half-written answers of a run that broke.
func (s *Store) FailStreamingMessages(runID int64) error {
	_, err := s.DB.Exec(`UPDATE messages SET status = 'failed' WHERE run_id = ? AND status = 'streaming'`, runID)
	return err
}

func (s *Store) ThreadsWithPendingRuns() ([]int64, error) {
	rows, err := s.DB.Query(`SELECT DISTINCT thread_id FROM runs WHERE status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ---- messages ----

func (s *Store) CreateMessage(m *Message) (int64, error) {
	var author any
	if m.AuthorUserID.Valid {
		author = m.AuthorUserID.Int64
	}
	status := m.Status
	if status == "" {
		status = MsgDone
	}
	res, err := s.DB.Exec(`INSERT INTO messages(thread_id, run_id, author_user_id, role, content, tool_calls, tool_call_id, tool_name, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ThreadID, m.RunID, author, m.Role, m.Content, m.ToolCalls, m.ToolCallID, m.ToolName, status)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateMessage(id int64, content, toolCalls, status string) error {
	_, err := s.DB.Exec(`UPDATE messages SET content = ?, tool_calls = ?, status = ? WHERE id = ?`, content, toolCalls, status, id)
	return err
}

// UpdateMessageFull also stores the model's reasoning text.
func (s *Store) UpdateMessageFull(id int64, content, reasoning, toolCalls, status string) error {
	_, err := s.DB.Exec(`UPDATE messages SET content = ?, reasoning = ?, tool_calls = ?, status = ? WHERE id = ?`,
		content, reasoning, toolCalls, status, id)
	return err
}

func (s *Store) DeleteMessage(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM messages WHERE id = ?`, id)
	return err
}

const msgSelect = `SELECT m.id, m.thread_id, m.run_id, m.author_user_id, COALESCE(u.username, ''), m.role, m.content,
	m.tool_calls, m.tool_call_id, m.tool_name, m.status, r.status, r.error, m.created_at, m.edited, m.reasoning
	FROM messages m JOIN runs r ON r.id = m.run_id LEFT JOIN users u ON u.id = m.author_user_id `

func scanMessages(rows *sql.Rows) ([]Message, error) {
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.RunID, &m.AuthorUserID, &m.AuthorName, &m.Role, &m.Content,
			&m.ToolCalls, &m.ToolCallID, &m.ToolName, &m.Status, &m.RunStatus, &m.RunError, &m.CreatedAt, &m.Edited, &m.Reasoning); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListMessages returns the last `limit` messages of a thread in display order.
func (s *Store) ListMessages(threadID int64, limit int) ([]Message, error) {
	rows, err := s.DB.Query(`SELECT * FROM (`+msgSelect+` WHERE m.thread_id = ? ORDER BY m.run_id DESC, m.id DESC LIMIT ?)
		ORDER BY run_id, id`, threadID, limit)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// ListMessagesFrom returns every message with run_id >= fromRun (capped), so a
// page that has loaded older history can be refreshed without losing it.
func (s *Store) ListMessagesFrom(threadID, fromRun int64, limit int) ([]Message, error) {
	rows, err := s.DB.Query(`SELECT * FROM (`+msgSelect+` WHERE m.thread_id = ? AND m.run_id >= ? ORDER BY m.run_id DESC, m.id DESC LIMIT ?)
		ORDER BY run_id, id`, threadID, fromRun, limit)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// ListMessagesBefore returns the `limit` messages just before run beforeRun.
func (s *Store) ListMessagesBefore(threadID, beforeRun int64, limit int) ([]Message, error) {
	rows, err := s.DB.Query(`SELECT * FROM (`+msgSelect+` WHERE m.thread_id = ? AND m.run_id < ? ORDER BY m.run_id DESC, m.id DESC LIMIT ?)
		ORDER BY run_id, id`, threadID, beforeRun, limit)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// HasMessagesBefore reports whether older messages exist than run beforeRun.
func (s *Store) HasMessagesBefore(threadID, beforeRun int64) bool {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM messages WHERE thread_id = ? AND run_id < ? LIMIT 1`, threadID, beforeRun).Scan(&n)
	return n > 0
}

// AllMessages returns the whole thread (export).
func (s *Store) AllMessages(threadID int64) ([]Message, error) {
	rows, err := s.DB.Query(msgSelect+` WHERE m.thread_id = ? ORDER BY m.run_id, m.id`, threadID)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// RunMessages returns every message of one run (user, assistant, tool).
func (s *Store) RunMessages(runID int64) ([]Message, error) {
	rows, err := s.DB.Query(msgSelect+` WHERE m.run_id = ? ORDER BY m.id`, runID)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// HistoryMessages returns the conversational messages (user + final assistant
// answers, no tool traffic) of finished runs with afterRun < id < beforeRun.
func (s *Store) HistoryMessages(threadID, afterRun, beforeRun int64) ([]Message, error) {
	rows, err := s.DB.Query(msgSelect+` WHERE m.thread_id = ? AND m.run_id > ? AND m.run_id < ?
		AND r.status IN ('done', 'interrupted', 'failed')
		AND (m.role = 'user' OR (m.role = 'assistant' AND m.tool_calls = '' AND m.content != ''))
		ORDER BY m.run_id, m.id`, threadID, afterRun, beforeRun)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// FinishedRunIDs lists finished runs after afterRun (the un-summarized pairs).
func (s *Store) FinishedRunIDs(threadID, afterRun, beforeRun int64) ([]int64, error) {
	rows, err := s.DB.Query(`SELECT id FROM runs WHERE thread_id = ? AND id > ? AND id < ?
		AND status IN ('done', 'interrupted', 'failed') ORDER BY id`, threadID, afterRun, beforeRun)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
