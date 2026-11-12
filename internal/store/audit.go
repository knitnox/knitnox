package store

// AuditEntry records an administrative action.
type AuditEntry struct {
	ID        int64
	ActorID   int64
	ActorName string
	Action    string
	Target    string
	Details   string
	CreatedAt string
}

// Audit appends an entry. Errors are ignored on purpose: an audit failure
// must never block the action itself.
func (s *Store) Audit(actorID int64, actorName, action, target, details string) {
	s.DB.Exec(`INSERT INTO audit_log(actor_id, actor_name, action, target, details) VALUES (?, ?, ?, ?, ?)`,
		actorID, actorName, action, target, details)
}

func (s *Store) ListAudit(limit int) ([]AuditEntry, error) {
	rows, err := s.DB.Query(`SELECT id, COALESCE(actor_id, 0), actor_name, action, target, details, created_at
		FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Action, &e.Target, &e.Details, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
