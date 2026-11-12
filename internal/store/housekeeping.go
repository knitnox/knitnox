package store

import (
	"context"
	"time"
)

// Housekeeping: rows that would otherwise grow for ever. The hourly clean-up
// calls these with the limits from Admin → Settings; 0 always means "keep".

// PruneUsage deletes usage rows older than keepDays. Quotas only look at the
// current day or month, so they are not affected; the "all time" usage report
// then starts at the oldest row kept.
func (s *Store) PruneUsage(keepDays int) (int64, error) {
	if keepDays <= 0 {
		return 0, nil
	}
	res, err := s.DB.Exec(`DELETE FROM usage_log WHERE created_at < ?`, FormatTime(time.Now().AddDate(0, 0, -keepDays)))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneAudit deletes audit rows older than keepDays.
func (s *Store) PruneAudit(keepDays int) (int64, error) {
	if keepDays <= 0 {
		return 0, nil
	}
	res, err := s.DB.Exec(`DELETE FROM audit_log WHERE created_at < ?`, FormatTime(time.Now().AddDate(0, 0, -keepDays)))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneAgentRuns keeps the newest keep finished runs of every agent and
// deletes the older ones with their messages. An agent starts every run
// fresh and reads only a digest of its last few, so nothing it needs is lost;
// its tasks, notes and memory live elsewhere. Chats are never touched.
func (s *Store) PruneAgentRuns(keep int) (int64, error) {
	if keep <= 0 {
		return 0, nil
	}
	res, err := s.DB.Exec(`DELETE FROM runs
		WHERE thread_id IN (SELECT thread_id FROM agents)
		  AND status NOT IN ('pending', 'running')
		  AND id NOT IN (SELECT r2.id FROM runs r2 WHERE r2.thread_id = runs.thread_id
		                     AND r2.status NOT IN ('pending', 'running') ORDER BY r2.id DESC LIMIT ?)`, keep)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Optimize lets SQLite refresh the statistics its query planner uses and
// folds the write-ahead log back into the database file, so neither drifts
// over months of running.
func (s *Store) Optimize() error {
	if _, err := s.DB.Exec(`PRAGMA optimize`); err != nil {
		return err
	}
	_, err := s.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// Ping reports whether the database answers (the health check).
func (s *Store) Ping(ctx context.Context) error {
	var one int
	return s.DB.QueryRowContext(ctx, `SELECT 1`).Scan(&one)
}

// CreateSignupUser adds an account made by signing up. The very first account
// becomes the admin; the role is decided inside the same statement, so two
// people signing up at the same moment cannot both become one.
func (s *Store) CreateSignupUser(username, passwordHash string) (id int64, role string, err error) {
	res, err := s.DB.Exec(`INSERT INTO users(username, password_hash, role)
		VALUES (?, ?, CASE WHEN (SELECT COUNT(*) FROM users) = 0 THEN 'admin' ELSE 'user' END)`, username, passwordHash)
	if err != nil {
		return 0, "", err
	}
	if id, err = res.LastInsertId(); err != nil {
		return 0, "", err
	}
	err = s.DB.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&role)
	return id, role, err
}
