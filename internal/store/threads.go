package store

import (
	"database/sql"
	"errors"
	"time"
)

type Thread struct {
	ID                int64
	OwnerUserID       int64
	Title             string
	ModelID           sql.NullInt64
	AllowOwnerKey     bool
	Summary           string
	CompactedUntilRun int64
	Instructions      string // chat-specific custom instructions
	Kind              string // KindChat or KindGroup; never changes after the chat is made
	GroupNotice       bool   // was shared before groups existed; the owner has not been told yet
	UpdatedAt         string
	CreatedAt         string
}

// The two kinds of chat. The kind is fixed when a chat is made: a personal
// chat never becomes a group and a group never becomes a personal chat.
const (
	KindChat  = "chat"  // one person's own chat (also every agent's thread)
	KindGroup = "group" // a chat for several people, with its own memory and tools
)

// IsGroup reports whether this is a group. A group never uses anybody's
// personal tools, however many people are in it.
func (t *Thread) IsGroup() bool { return t.Kind == KindGroup }

type ThreadListItem struct {
	ID        int64
	Title     string
	Group     bool
	Members   int
	Role      string
	UpdatedAt string
	Pinned    bool
	Archived  bool
}

type Member struct {
	UserID   int64
	Username string
	Role     string
}

// CreateThread makes a personal chat.
func (s *Store) CreateThread(ownerID int64, modelID int64) (int64, error) {
	return s.createThread(ownerID, modelID, KindChat, "")
}

// CreateGroup makes a group with ownerID as its admin. Others join by invite.
func (s *Store) CreateGroup(ownerID, modelID int64, title string) (int64, error) {
	return s.createThread(ownerID, modelID, KindGroup, title)
}

func (s *Store) createThread(ownerID, modelID int64, kind, title string) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var mid any
	if modelID > 0 {
		mid = modelID
	}
	res, err := tx.Exec(`INSERT INTO threads(owner_user_id, model_id, kind) VALUES (?, ?, ?)`, ownerID, mid, kind)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if title != "" {
		if _, err := tx.Exec(`UPDATE threads SET title = ? WHERE id = ?`, title, id); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`INSERT INTO thread_members(thread_id, user_id, role) VALUES (?, ?, 'owner')`, id, ownerID); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) ThreadByID(id int64) (*Thread, error) {
	t := &Thread{}
	err := s.DB.QueryRow(`SELECT id, owner_user_id, title, model_id, allow_owner_key, summary, compacted_until_run, instructions, kind, group_notice, updated_at, created_at
		FROM threads WHERE id = ?`, id).
		Scan(&t.ID, &t.OwnerUserID, &t.Title, &t.ModelID, &t.AllowOwnerKey, &t.Summary, &t.CompactedUntilRun, &t.Instructions, &t.Kind, &t.GroupNotice, &t.UpdatedAt, &t.CreatedAt)
	return t, notFound(err)
}

// MemberRole returns owner|member|viewer, or "" if the user is not in the thread.
func (s *Store) MemberRole(threadID, userID int64) string {
	var role string
	s.DB.QueryRow(`SELECT role FROM thread_members WHERE thread_id = ? AND user_id = ?`, threadID, userID).Scan(&role)
	return role
}

// ListThreadsForUser returns the user's chats, pinned first, then by activity.
// Archived chats are included (flagged) so the sidebar can tuck them away.
func (s *Store) ListThreadsForUser(userID int64) ([]ThreadListItem, error) {
	rows, err := s.DB.Query(`SELECT t.id, t.title, t.kind = 'group', tm.role, t.updated_at, tm.pinned, tm.archived,
		(SELECT COUNT(*) FROM thread_members x WHERE x.thread_id = t.id)
		FROM threads t JOIN thread_members tm ON tm.thread_id = t.id AND tm.user_id = ?
		WHERE t.id NOT IN (SELECT thread_id FROM agents)
		ORDER BY tm.archived, tm.pinned DESC, t.updated_at DESC, t.id DESC LIMIT 300`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ThreadListItem
	for rows.Next() {
		var t ThreadListItem
		if err := rows.Scan(&t.ID, &t.Title, &t.Group, &t.Role, &t.UpdatedAt, &t.Pinned, &t.Archived, &t.Members); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SetThreadFlag sets a member's pinned/archived flag for a thread.
func (s *Store) SetThreadFlag(threadID, userID int64, flag string, on bool) error {
	if flag != "pinned" && flag != "archived" {
		return errors.New("unknown flag")
	}
	_, err := s.DB.Exec(`UPDATE thread_members SET `+flag+` = ? WHERE thread_id = ? AND user_id = ?`, b2i(on), threadID, userID)
	return err
}

// ThreadFlags returns the member's pinned/archived flags.
func (s *Store) ThreadFlags(threadID, userID int64) (pinned, archived bool) {
	s.DB.QueryRow(`SELECT pinned, archived FROM thread_members WHERE thread_id = ? AND user_id = ?`, threadID, userID).Scan(&pinned, &archived)
	return
}

// SetThreadInstructions saves chat-specific instructions.
func (s *Store) SetThreadInstructions(id int64, text string) error {
	_, err := s.DB.Exec(`UPDATE threads SET instructions = ? WHERE id = ?`, text, id)
	return err
}

func (s *Store) ThreadMembers(threadID int64) ([]Member, error) {
	rows, err := s.DB.Query(`SELECT u.id, u.username, tm.role FROM thread_members tm
		JOIN users u ON u.id = tm.user_id WHERE tm.thread_id = ?
		ORDER BY tm.role = 'owner' DESC, tm.joined_at`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) MemberCount(threadID int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM thread_members WHERE thread_id = ?`, threadID).Scan(&n)
	return n
}

func (s *Store) UpdateThreadSettings(id int64, title string, modelID int64, instructions string) error {
	var mid any
	if modelID > 0 {
		mid = modelID
	}
	_, err := s.DB.Exec(`UPDATE threads SET title = ?, model_id = ?, instructions = ?, updated_at = ? WHERE id = ?`,
		title, mid, instructions, Now(), id)
	return err
}

// ClearGroupNotice records that the owner read the note about a chat that
// was shared before groups existed.
func (s *Store) ClearGroupNotice(id int64) error {
	_, err := s.DB.Exec(`UPDATE threads SET group_notice = 0 WHERE id = ?`, id)
	return err
}

func (s *Store) SetThreadTitle(id int64, title string) error {
	_, err := s.DB.Exec(`UPDATE threads SET title = ? WHERE id = ?`, title, id)
	return err
}

func (s *Store) TouchThread(id int64) error {
	_, err := s.DB.Exec(`UPDATE threads SET updated_at = ? WHERE id = ?`, Now(), id)
	return err
}

func (s *Store) DeleteThread(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM threads WHERE id = ?`, id)
	return err
}

func (s *Store) RemoveMember(threadID, userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM thread_members WHERE thread_id = ? AND user_id = ? AND role != 'owner'`, threadID, userID)
	return err
}

func (s *Store) SetSummary(threadID int64, summary string, untilRun int64) error {
	_, err := s.DB.Exec(`UPDATE threads SET summary = ?, compacted_until_run = ? WHERE id = ?`, summary, untilRun, threadID)
	return err
}

// ---- invites ----

type Invite struct {
	Token       string
	ThreadID    int64
	ThreadTitle string
	Role        string
	ExpiresAt   string
	MaxUses     int
	Uses        int
}

var ErrInviteInvalid = errors.New("invite is invalid, expired or used up")

// ErrNotGroup is returned when something only a group can do is asked of a personal chat.
var ErrNotGroup = errors.New("only a group can be joined")

// CreateInvite makes a link with which people can join a group.
func (s *Store) CreateInvite(threadID, by int64, role string, ttl time.Duration, maxUses int) (string, error) {
	tok := randomToken()[:32]
	res, err := s.DB.Exec(`INSERT INTO thread_invites(token, thread_id, created_by, role, expires_at, max_uses)
		SELECT ?, id, ?, ?, ?, ? FROM threads WHERE id = ? AND kind = 'group'`,
		tok, by, role, FormatTime(time.Now().Add(ttl)), maxUses, threadID)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrNotGroup
	}
	return tok, nil
}

func (s *Store) InviteByToken(tok string) (*Invite, error) {
	inv := &Invite{}
	err := s.DB.QueryRow(`SELECT i.token, i.thread_id, t.title, i.role, i.expires_at, i.max_uses, i.uses
		FROM thread_invites i JOIN threads t ON t.id = i.thread_id
		WHERE i.token = ? AND i.expires_at > ? AND i.uses < i.max_uses AND t.kind = 'group'`, tok, Now()).
		Scan(&inv.Token, &inv.ThreadID, &inv.ThreadTitle, &inv.Role, &inv.ExpiresAt, &inv.MaxUses, &inv.Uses)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInviteInvalid
	}
	return inv, err
}

func (s *Store) ListInvites(threadID int64) ([]Invite, error) {
	rows, err := s.DB.Query(`SELECT token, thread_id, role, expires_at, max_uses, uses FROM thread_invites
		WHERE thread_id = ? AND expires_at > ? AND uses < max_uses ORDER BY expires_at DESC`, threadID, Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invite
	for rows.Next() {
		var i Invite
		if err := rows.Scan(&i.Token, &i.ThreadID, &i.Role, &i.ExpiresAt, &i.MaxUses, &i.Uses); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) RevokeInvite(threadID int64, tok string) error {
	_, err := s.DB.Exec(`DELETE FROM thread_invites WHERE thread_id = ? AND token = ?`, threadID, tok)
	return err
}

// AcceptInvite adds the user to the thread (no-op if already a member) and counts the use.
func (s *Store) AcceptInvite(tok string, userID int64) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var threadID int64
	var role string
	// Only a group can be joined; a personal chat is shown with a read-only link instead.
	err = tx.QueryRow(`SELECT i.thread_id, i.role FROM thread_invites i JOIN threads t ON t.id = i.thread_id
		WHERE i.token = ? AND i.expires_at > ? AND i.uses < i.max_uses AND t.kind = 'group'`, tok, Now()).Scan(&threadID, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInviteInvalid
	} else if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT OR IGNORE INTO thread_members(thread_id, user_id, role) VALUES (?, ?, ?)`, threadID, userID, role)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		if _, err := tx.Exec(`UPDATE thread_invites SET uses = uses + 1 WHERE token = ?`, tok); err != nil {
			return 0, err
		}
	}
	return threadID, tx.Commit()
}
