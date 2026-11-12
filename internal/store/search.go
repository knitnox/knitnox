package store

import (
	"strings"
)

// SearchHit is one message matching a full-text query.
type SearchHit struct {
	ThreadID    int64
	ThreadTitle string
	MessageID   int64
	RunID       int64
	Role        string
	AuthorName  string
	Snippet     string // plain text with [[ ]] around matches
	CreatedAt   string
}

// ftsQuery turns free text into a safe FTS5 query: every word becomes a
// quoted prefix term, so punctuation and operators cannot break the query.
func ftsQuery(q string) string {
	var terms []string
	for _, w := range strings.Fields(q) {
		w = strings.ReplaceAll(w, `"`, "")
		if w == "" {
			continue
		}
		terms = append(terms, `"`+w+`"*`)
	}
	return strings.Join(terms, " ")
}

// SearchMessages finds messages in chats the user belongs to.
func (s *Store) SearchMessages(userID int64, q string, limit int) ([]SearchHit, error) {
	fq := ftsQuery(q)
	if fq == "" {
		return nil, nil
	}
	rows, err := s.DB.Query(`SELECT t.id, t.title, m.id, m.run_id, m.role, COALESCE(u.username, ''),
		snippet(messages_fts, 0, '[[', ']]', '…', 18), m.created_at
		FROM messages_fts f
		JOIN messages m ON m.id = f.rowid
		JOIN threads t ON t.id = m.thread_id
		JOIN thread_members tm ON tm.thread_id = t.id AND tm.user_id = ?
		LEFT JOIN users u ON u.id = m.author_user_id
		WHERE messages_fts MATCH ? AND m.role IN ('user', 'assistant')
		ORDER BY bm25(messages_fts), m.id DESC LIMIT ?`, userID, fq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.ThreadID, &h.ThreadTitle, &h.MessageID, &h.RunID, &h.Role, &h.AuthorName, &h.Snippet, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
