package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Groups of built-in tools that can be switched off for one chat.
const (
	ToolMemory   = "memory"
	ToolPlan     = "plan"
	ToolFeed     = "feed"
	ToolPictures = "pictures"
	ToolAgents   = "agents"
)

// ToolServerKey is the key of one tool (MCP) server.
func ToolServerKey(serverID int64) string { return fmt.Sprintf("mcp:%d", serverID) }

// ErrBadToolKey is returned for a key that names no tool group or server.
var ErrBadToolKey = errors.New("unknown tool")

func validToolKey(key string) bool {
	switch key {
	case ToolMemory, ToolPlan, ToolFeed, ToolPictures, ToolAgents:
		return true
	}
	rest, ok := strings.CutPrefix(key, "mcp:")
	if !ok {
		return false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	return err == nil && id > 0 && rest == strconv.FormatInt(id, 10)
}

// ThreadToolsOff returns the tools switched off for a chat.
func (s *Store) ThreadToolsOff(threadID int64) map[string]bool {
	out := map[string]bool{}
	rows, err := s.DB.Query(`SELECT key FROM thread_tools_off WHERE thread_id = ?`, threadID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			out[k] = true
		}
	}
	return out
}

// SetThreadTool switches one tool group or server on or off for a chat.
func (s *Store) SetThreadTool(threadID int64, key string, on bool) error {
	if !validToolKey(key) {
		return ErrBadToolKey
	}
	var err error
	if on {
		_, err = s.DB.Exec(`DELETE FROM thread_tools_off WHERE thread_id = ? AND key = ?`, threadID, key)
	} else {
		_, err = s.DB.Exec(`INSERT OR IGNORE INTO thread_tools_off(thread_id, key) VALUES (?, ?)`, threadID, key)
	}
	return err
}
