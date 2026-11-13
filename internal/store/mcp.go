package store

// Tool approval modes for an MCP server.
const (
	ApprovalAuto = "auto" // run tools immediately
	ApprovalAsk  = "ask"  // pause and ask a person before each tool call
)

type MCPServer struct {
	ID       int64
	UserID   int64
	Name     string
	URL      string
	Headers  string // decrypted "Key: Value" lines; never render
	Enabled  bool
	Approval string // ApprovalAuto | ApprovalAsk
	AgentID  int64  // > 0: belongs to that agent only; 0: one of the user's shared servers
	GroupID  int64  // > 0: belongs to that group only (the group's thread id)
}

func (m *MCPServer) Asks() bool { return m.Approval == ApprovalAsk }

func normalizeApproval(a string) string {
	if a == ApprovalAsk {
		return ApprovalAsk
	}
	return ApprovalAuto
}

func (s *Store) CreateMCPServer(userID int64, name, url, headers, approval string) error {
	enc, err := s.Box.Seal(headers)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO mcp_servers(user_id, name, url, headers_enc, approval) VALUES (?, ?, ?, ?, ?)`,
		userID, name, url, enc, normalizeApproval(approval))
	return err
}

func (s *Store) DeleteMCPServer(userID, id int64) error {
	_, err := s.DB.Exec(`DELETE FROM mcp_servers WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func (s *Store) ToggleMCPServer(userID, id int64) error {
	_, err := s.DB.Exec(`UPDATE mcp_servers SET enabled = 1 - enabled WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// SetMCPApproval switches a server between automatic and ask-first mode.
func (s *Store) SetMCPApproval(userID, id int64, approval string) error {
	_, err := s.DB.Exec(`UPDATE mcp_servers SET approval = ? WHERE id = ? AND user_id = ?`, normalizeApproval(approval), id, userID)
	return err
}

func (s *Store) scanMCP(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]MCPServer, error) {
	defer rows.Close()
	var out []MCPServer
	for rows.Next() {
		var m MCPServer
		var enc []byte
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.URL, &enc, &m.Enabled, &m.Approval, &m.AgentID, &m.GroupID); err != nil {
			return nil, err
		}
		h, err := s.Box.Open(enc)
		if err != nil {
			return nil, err
		}
		m.Headers = h
		out = append(out, m)
	}
	return out, rows.Err()
}

const mcpCols = `id, user_id, name, url, headers_enc, enabled, approval, COALESCE(agent_id, 0), COALESCE(group_id, 0)`

// ListMCPServers returns a user's shared servers (not the ones that belong to
// a single agent or to a group; see ListAgentOwnServers and ListGroupServers).
func (s *Store) ListMCPServers(userID int64) ([]MCPServer, error) {
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE user_id = ? AND agent_id IS NULL AND group_id IS NULL ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	return s.scanMCP(rows)
}

func (s *Store) MCPServerByID(userID, id int64) (*MCPServer, error) {
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE id = ? AND user_id = ?`, id, userID)
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

// ThreadMCPServers returns the enabled servers the AI may use in a thread:
//   - personal chat: all of the owner's enabled servers
//   - group:         the group's own servers, never the owner's personal ones
func (s *Store) ThreadMCPServers(t *Thread) ([]MCPServer, error) {
	q, arg := `user_id = ? AND agent_id IS NULL AND group_id IS NULL`, t.OwnerUserID
	if t.IsGroup() {
		q, arg = `group_id = ?`, t.ID
	}
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE enabled = 1 AND `+q+` ORDER BY id`, arg)
	if err != nil {
		return nil, err
	}
	return s.scanMCP(rows)
}

// CreateGroupMCPServer adds a tool server that belongs to one group. It is
// kept under the group owner's account, who is the only one to manage it.
func (s *Store) CreateGroupMCPServer(t *Thread, name, url, headers, approval string) error {
	if !t.IsGroup() {
		return ErrNotGroup
	}
	enc, err := s.Box.Seal(headers)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO mcp_servers(user_id, name, url, headers_enc, approval, group_id) VALUES (?, ?, ?, ?, ?, ?)`,
		t.OwnerUserID, name, url, enc, normalizeApproval(approval), t.ID)
	return err
}

// ListGroupServers returns every tool server of a group, enabled or not.
func (s *Store) ListGroupServers(groupID int64) ([]MCPServer, error) {
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE group_id = ? ORDER BY id`, groupID)
	if err != nil {
		return nil, err
	}
	return s.scanMCP(rows)
}

// GroupServer returns one of a group's tool servers.
func (s *Store) GroupServer(groupID, serverID int64) (*MCPServer, error) {
	rows, err := s.DB.Query(`SELECT `+mcpCols+` FROM mcp_servers WHERE id = ? AND group_id = ?`, serverID, groupID)
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

// ---- tool approvals ----

// Approval statuses.
const (
	ApprovalPending = "pending"
	ApprovalAllowed = "allowed"
	ApprovalDenied  = "denied"
	ApprovalExpired = "expired"
)

// ToolApproval is one tool call waiting for a person's decision.
type ToolApproval struct {
	ID         int64
	ThreadID   int64
	RunID      int64
	MessageID  int64
	ToolCallID string
	ServerID   int64
	ServerName string
	ToolName   string
	Args       string
	Status     string
	DecidedBy  int64
	CreatedAt  string
}

func (s *Store) CreateApproval(a *ToolApproval) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO tool_approvals(thread_id, run_id, message_id, tool_call_id, server_id, server_name, tool_name, args)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, a.ThreadID, a.RunID, a.MessageID, a.ToolCallID, a.ServerID, a.ServerName, a.ToolName, a.Args)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const approvalCols = `id, thread_id, run_id, message_id, tool_call_id, server_id, server_name, tool_name, args, status, COALESCE(decided_by, 0), created_at`

func scanApproval(sc interface{ Scan(...any) error }) (*ToolApproval, error) {
	a := &ToolApproval{}
	err := sc.Scan(&a.ID, &a.ThreadID, &a.RunID, &a.MessageID, &a.ToolCallID, &a.ServerID, &a.ServerName, &a.ToolName, &a.Args, &a.Status, &a.DecidedBy, &a.CreatedAt)
	return a, err
}

func (s *Store) ApprovalByID(id int64) (*ToolApproval, error) {
	a, err := scanApproval(s.DB.QueryRow(`SELECT `+approvalCols+` FROM tool_approvals WHERE id = ?`, id))
	return a, notFound(err)
}

// DecideApproval records a decision; it reports false if it was already decided.
func (s *Store) DecideApproval(id, byUser int64, status string) (bool, error) {
	var by any // NULL when the system decided (timeout / cancel)
	if byUser > 0 {
		by = byUser
	}
	res, err := s.DB.Exec(`UPDATE tool_approvals SET status = ?, decided_by = ?, decided_at = ? WHERE id = ? AND status = 'pending'`,
		status, by, Now(), id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// PendingApprovals lists a thread's undecided tool calls keyed by assistant message.
func (s *Store) PendingApprovals(threadID int64) (map[int64][]ToolApproval, error) {
	rows, err := s.DB.Query(`SELECT `+approvalCols+` FROM tool_approvals WHERE thread_id = ? AND status = 'pending' ORDER BY id`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]ToolApproval{}
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out[a.MessageID] = append(out[a.MessageID], *a)
	}
	return out, rows.Err()
}

// ExpirePendingApprovals marks every undecided approval expired (server restart).
func (s *Store) ExpirePendingApprovals() error {
	_, err := s.DB.Exec(`UPDATE tool_approvals SET status = 'expired', decided_at = ? WHERE status = 'pending'`, Now())
	return err
}
