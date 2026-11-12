package store

import (
	"errors"
	"strings"
)

// Issue statuses.
const (
	IssueOpen = "open"
	IssueDone = "done"

	// MaxOpenIssuesPerUser stops one account from flooding the list.
	MaxOpenIssuesPerUser = 30
)

// ErrTooManyIssues is returned when a user already has too many open issues.
var ErrTooManyIssues = errors.New("too many open issues")

// Issue is a bug report, idea or question. Every signed-in user can read it.
type Issue struct {
	ID         int64
	UserID     int64
	AuthorName string
	Kind       string // bug | idea | question
	Title      string
	Body       string
	Status     string // open | done
	Reply      string // one answer from an admin
	ReplyBy    string
	CreatedAt  string
	UpdatedAt  string
}

func normalizeIssueKind(k string) string {
	switch k {
	case "idea", "question":
		return k
	}
	return "bug"
}

const issueCols = `id, COALESCE(user_id, 0), author_name, kind, title, body, status, reply, reply_by, created_at, updated_at`

func scanIssue(sc interface{ Scan(...any) error }) (*Issue, error) {
	i := &Issue{}
	err := sc.Scan(&i.ID, &i.UserID, &i.AuthorName, &i.Kind, &i.Title, &i.Body, &i.Status, &i.Reply, &i.ReplyBy, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

// CreateIssue files a new open issue.
func (s *Store) CreateIssue(userID int64, author, kind, title, body string) (int64, error) {
	title = clip(strings.Join(strings.Fields(title), " "), 150)
	if title == "" {
		return 0, errors.New("an issue needs a title")
	}
	var open int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM issues WHERE user_id = ? AND status = 'open'`, userID).Scan(&open); err != nil {
		return 0, err
	}
	if open >= MaxOpenIssuesPerUser {
		return 0, ErrTooManyIssues
	}
	res, err := s.DB.Exec(`INSERT INTO issues(user_id, author_name, kind, title, body) VALUES (?, ?, ?, ?, ?)`,
		userID, author, normalizeIssueKind(kind), title, clip(strings.TrimSpace(body), 4000))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) IssueByID(id int64) (*Issue, error) {
	i, err := scanIssue(s.DB.QueryRow(`SELECT `+issueCols+` FROM issues WHERE id = ?`, id))
	return i, notFound(err)
}

// ListIssues returns issues with the given status, newest first. A non-zero
// byUser returns that user's issues of any status instead.
func (s *Store) ListIssues(status string, byUser int64) ([]Issue, error) {
	q, args := `SELECT `+issueCols+` FROM issues WHERE status = ?`, []any{status}
	if byUser > 0 {
		q, args = `SELECT `+issueCols+` FROM issues WHERE user_id = ?`, []any{byUser}
	}
	rows, err := s.DB.Query(q+` ORDER BY id DESC LIMIT 300`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		i, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}

// IssueCounts returns how many issues are open and done.
func (s *Store) IssueCounts() (open, done int) {
	s.DB.QueryRow(`SELECT COALESCE(SUM(status = 'open'), 0), COALESCE(SUM(status = 'done'), 0) FROM issues`).Scan(&open, &done)
	return
}

// SetIssueStatus opens or closes an issue.
func (s *Store) SetIssueStatus(id int64, status string) error {
	if status != IssueOpen && status != IssueDone {
		return errors.New("unknown issue status")
	}
	_, err := s.DB.Exec(`UPDATE issues SET status = ?, updated_at = ? WHERE id = ?`, status, Now(), id)
	return err
}

// SetIssueReply stores an admin's answer (empty text removes it).
func (s *Store) SetIssueReply(id int64, reply, by string) error {
	reply = clip(strings.TrimSpace(reply), 2000)
	if reply == "" {
		by = ""
	}
	_, err := s.DB.Exec(`UPDATE issues SET reply = ?, reply_by = ?, updated_at = ? WHERE id = ?`, reply, by, Now(), id)
	return err
}

func (s *Store) DeleteIssue(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM issues WHERE id = ?`, id)
	return err
}
