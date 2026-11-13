package store

import (
	"database/sql"
	"errors"
	"strings"
)

// Groups and read-only links
//
// A chat is one of two kinds for its whole life (threads.kind):
//
//   - a personal chat belongs to one person. Others can be shown it with a
//     read-only link (thread_shares) but can never write in it.
//   - a group is for several people. It has its own memory (MemOwner with a
//     GroupID), its own tool servers (mcp_servers.group_id) and one model, and
//     it never uses anybody's personal tools.
//
// The only bridge between the two is a reference: a personal chat may look
// into groups its owner is a member of (thread_groups).

// ---- a group's own settings ----

// GroupSettings is what a group's admin chooses besides the chat model: the
// embedding model of the group's memory. Provider 0 = the server's model.
type GroupSettings struct {
	EmbedProviderID int64
	EmbedModel      string
	EmbedDimensions int
}

func (s *Store) GroupSettingsFor(groupID int64) (GroupSettings, error) {
	var gs GroupSettings
	err := s.DB.QueryRow(`SELECT COALESCE(embed_provider_id, 0), embed_model, embed_dimensions
		FROM group_settings WHERE thread_id = ?`, groupID).Scan(&gs.EmbedProviderID, &gs.EmbedModel, &gs.EmbedDimensions)
	if errors.Is(err, sql.ErrNoRows) {
		return gs, nil
	}
	return gs, err
}

func (s *Store) SaveGroupSettings(groupID int64, gs GroupSettings) error {
	var pid any
	if gs.EmbedProviderID > 0 {
		pid = gs.EmbedProviderID
	}
	_, err := s.DB.Exec(`INSERT INTO group_settings(thread_id, embed_provider_id, embed_model, embed_dimensions)
		VALUES (?, ?, ?, ?) ON CONFLICT(thread_id) DO UPDATE SET embed_provider_id = excluded.embed_provider_id,
		embed_model = excluded.embed_model, embed_dimensions = excluded.embed_dimensions`,
		groupID, pid, strings.TrimSpace(gs.EmbedModel), max(gs.EmbedDimensions, 0))
	return err
}

// ---- looking into a group from a personal chat ----

// GroupRef is a group as a personal chat sees it.
type GroupRef struct {
	ID     int64
	Title  string
	Member bool // the person asking is (still) in the group
}

// UserGroups lists the groups a person is in, the most recently active first.
func (s *Store) UserGroups(userID int64) ([]GroupRef, error) {
	return s.groupRefs(`SELECT t.id, t.title, 1 FROM threads t
		JOIN thread_members tm ON tm.thread_id = t.id AND tm.user_id = ?
		WHERE t.kind = 'group' ORDER BY t.updated_at DESC, t.id DESC LIMIT 100`, userID)
}

// ThreadGroups lists the groups a personal chat looks into, oldest reference
// first so the list reads the same from turn to turn. Member says whether
// userID is in the group right now: a reference outlives membership, but
// nothing can be read through it any more.
func (s *Store) ThreadGroups(threadID, userID int64) ([]GroupRef, error) {
	return s.groupRefs(`SELECT t.id, t.title,
		EXISTS (SELECT 1 FROM thread_members tm WHERE tm.thread_id = t.id AND tm.user_id = ?)
		FROM thread_groups tg JOIN threads t ON t.id = tg.group_id
		WHERE tg.thread_id = ? AND t.kind = 'group' ORDER BY tg.rowid`, userID, threadID)
}

func (s *Store) groupRefs(q string, args ...any) ([]GroupRef, error) {
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupRef
	for rows.Next() {
		var g GroupRef
		if err := rows.Scan(&g.ID, &g.Title, &g.Member); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SetThreadGroup lets a personal chat look into a group, or stops it. Adding
// needs all of: the chat is a personal chat of userID (not an agent's), the
// target is a group, and userID is a member of it. It returns ErrNotFound
// when one of these does not hold.
func (s *Store) SetThreadGroup(threadID, userID, groupID int64, on bool) error {
	if !on {
		_, err := s.DB.Exec(`DELETE FROM thread_groups WHERE thread_id = ? AND group_id = ?
			AND thread_id IN (SELECT id FROM threads WHERE owner_user_id = ?)`, threadID, groupID, userID)
		return err
	}
	res, err := s.DB.Exec(`INSERT OR IGNORE INTO thread_groups(thread_id, group_id)
		SELECT c.id, g.id FROM threads c, threads g
		WHERE c.id = ? AND c.owner_user_id = ? AND c.kind = 'chat' AND c.id NOT IN (SELECT thread_id FROM agents)
		  AND g.id = ? AND g.kind = 'group'
		  AND EXISTS (SELECT 1 FROM thread_members tm WHERE tm.thread_id = g.id AND tm.user_id = ?)`,
		threadID, userID, groupID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var have int
		s.DB.QueryRow(`SELECT COUNT(*) FROM thread_groups WHERE thread_id = ? AND group_id = ?`, threadID, groupID).Scan(&have)
		if have == 0 {
			return ErrNotFound
		}
	}
	return nil
}

// RecentTurns returns the last n finished turns of a chat as plain
// conversation: what people wrote and what the AI answered, oldest first.
// Tool calls and tool results are left out.
func (s *Store) RecentTurns(threadID int64, n int) ([]Message, error) {
	rows, err := s.DB.Query(msgSelect+` WHERE m.thread_id = ? AND m.role IN ('user', 'assistant') AND m.content != ''
		AND m.run_id IN (SELECT id FROM runs WHERE thread_id = ? AND status = 'done' ORDER BY id DESC LIMIT ?)
		ORDER BY m.run_id, m.id`, threadID, threadID, n)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// ---- the read-only link of a personal chat ----

// ThreadShare is the read-only link of a personal chat.
type ThreadShare struct {
	ThreadID  int64
	Token     string
	UntilRun  int64 // the link shows the chat up to and including this run
	ShowTools bool  // also show tool calls and what the tools answered
	CreatedAt string
	UpdatedAt string
}

const shareCols = `thread_id, token, until_run, show_tools, created_at, updated_at`

func scanShare(row *sql.Row) (*ThreadShare, error) {
	sh := &ThreadShare{}
	err := row.Scan(&sh.ThreadID, &sh.Token, &sh.UntilRun, &sh.ShowTools, &sh.CreatedAt, &sh.UpdatedAt)
	return sh, notFound(err)
}

// ShareOf returns a chat's link, or ErrNotFound when it has none.
func (s *Store) ShareOf(threadID int64) (*ThreadShare, error) {
	return scanShare(s.DB.QueryRow(`SELECT `+shareCols+` FROM thread_shares WHERE thread_id = ?`, threadID))
}

// ShareByToken finds the link someone opened.
func (s *Store) ShareByToken(token string) (*ThreadShare, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	return scanShare(s.DB.QueryRow(`SELECT `+shareCols+` FROM thread_shares WHERE token = ?`, token))
}

// lastFinishedRun is the newest run of a chat that is over, whatever its outcome.
func (s *Store) lastFinishedRun(threadID int64) int64 {
	var id int64
	s.DB.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM runs WHERE thread_id = ?
		AND status IN ('done', 'failed', 'cancelled', 'interrupted')`, threadID).Scan(&id)
	return id
}

// SaveShare makes the link of a personal chat, or brings an existing one up
// to date: the link then shows the chat as it is now. The address stays the
// same. Groups and agents have no such link.
func (s *Store) SaveShare(threadID int64, showTools bool) (*ThreadShare, error) {
	until := s.lastFinishedRun(threadID)
	res, err := s.DB.Exec(`INSERT INTO thread_shares(thread_id, token, until_run, show_tools)
		SELECT id, ?, ?, ? FROM threads WHERE id = ? AND kind = 'chat' AND id NOT IN (SELECT thread_id FROM agents)
		ON CONFLICT(thread_id) DO UPDATE SET until_run = excluded.until_run, show_tools = excluded.show_tools, updated_at = ?`,
		randomToken()[:32], until, b2i(showTools), threadID, Now())
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.ShareOf(threadID)
}

// DeleteShare stops a link from working.
func (s *Store) DeleteShare(threadID int64) error {
	_, err := s.DB.Exec(`DELETE FROM thread_shares WHERE thread_id = ?`, threadID)
	return err
}

// SharedMessages returns what a link shows: the chat up to the run it was
// last updated at, newest messages last, at most limit of them.
func (s *Store) SharedMessages(sh *ThreadShare, limit int) ([]Message, error) {
	where := ` WHERE m.thread_id = ? AND m.run_id <= ?`
	if !sh.ShowTools {
		where += ` AND m.role IN ('user', 'assistant')`
	}
	rows, err := s.DB.Query(`SELECT * FROM (`+msgSelect+where+` ORDER BY m.run_id DESC, m.id DESC LIMIT ?)
		ORDER BY run_id, id`, sh.ThreadID, sh.UntilRun, limit)
	if err != nil {
		return nil, err
	}
	return scanMessages(rows)
}

// SharedAttachment reports whether a file belongs to a message the link shows.
func (s *Store) SharedAttachment(sh *ThreadShare, attachmentID int64) bool {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM attachments a JOIN messages m ON m.id = a.message_id
		WHERE a.id = ? AND m.thread_id = ? AND m.run_id <= ?`, attachmentID, sh.ThreadID, sh.UntilRun).Scan(&n)
	return n > 0
}
