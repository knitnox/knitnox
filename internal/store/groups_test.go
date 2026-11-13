package store

import (
	"database/sql"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"knitnox/internal/secret"
)

// Migration 015 on a database from before groups existed: a chat with two
// people becomes a group and keeps its tool server; a personal chat stays
// personal and can no longer be joined; and every memory, thing and
// connection survives the rebuild of the memory tables.
func TestMigrationToGroupsKeepsEverything(t *testing.T) {
	box, _ := secret.New("test")
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// The schema as it was: every migration before 015.
	must(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
	files, _ := fs.Glob(migrationFS, "migrations/*.sql")
	sort.Strings(files)
	for _, f := range files {
		version := strings.TrimSuffix(filepath.Base(f), ".sql")
		if version >= "015" {
			break
		}
		body, _ := migrationFS.ReadFile(f)
		must(string(body))
		must(`INSERT INTO schema_migrations(version) VALUES (?)`, version)
	}
	headers, _ := box.Seal("Authorization: Bearer x")
	must(`INSERT INTO users(id, username, password_hash) VALUES (1, 'alice', 'x'), (2, 'bob', 'x')`)
	must(`INSERT INTO threads(id, owner_user_id, title) VALUES (1, 1, 'Mine'), (2, 1, 'Ours')`)
	must(`INSERT INTO thread_members(thread_id, user_id, role) VALUES (1, 1, 'owner'), (2, 1, 'owner'), (2, 2, 'member')`)
	must(`INSERT INTO thread_invites(token, thread_id, created_by, expires_at) VALUES ('to-mine', 1, 1, '2999-01-01 00:00:00'), ('to-ours', 2, 1, '2999-01-01 00:00:00')`)
	must(`INSERT INTO mcp_servers(id, user_id, name, url, headers_enc, approval) VALUES (1, 1, 'Mail', 'https://mail.example/mcp', ?, 'ask')`, headers)
	must(`INSERT INTO thread_mcp(thread_id, mcp_server_id) VALUES (2, 1)`)
	must(`INSERT INTO thread_tools_off(thread_id, key) VALUES (1, 'plan'), (2, 'memory')`)
	must(`INSERT INTO memories(id, user_id, content, source, pinned) VALUES (1, 1, 'Sam works at Acme Bakery.', 'ai', 1), (2, 2, 'Bob likes rye.', 'user', 0)`)
	must(`INSERT INTO memory_entities(id, user_id, name, name_key) VALUES (1, 1, 'Sam', 'sam'), (2, 1, 'Acme Bakery', 'acme bakery'), (3, 2, 'Sam', 'sam')`)
	must(`INSERT INTO memory_mentions(memory_id, entity_id) VALUES (1, 1), (1, 2), (2, 3)`)
	must(`INSERT INTO memory_relations(user_id, memory_id, subject_id, predicate, object_id) VALUES (1, 1, 1, 'works at', 2)`)
	must(`INSERT INTO memories_fts(rowid, content, about) VALUES (1, 'Sam works at Acme Bakery.', 'Sam · Acme Bakery'), (2, 'Bob likes rye.', 'Sam')`)
	db.Close()

	s, err := Open(path, box)
	if err != nil {
		t.Fatalf("opening the old database: %v", err)
	}
	defer s.Close()

	mine, _ := s.ThreadByID(1)
	ours, _ := s.ThreadByID(2)
	if mine.IsGroup() || mine.GroupNotice || !ours.IsGroup() || !ours.GroupNotice {
		t.Fatalf("kinds: mine %q notice %v, ours %q notice %v", mine.Kind, mine.GroupNotice, ours.Kind, ours.GroupNotice)
	}
	if _, err := s.InviteByToken("to-mine"); err != ErrInviteInvalid {
		t.Errorf("the invite to a personal chat still works: %v", err)
	}
	if _, err := s.InviteByToken("to-ours"); err != nil {
		t.Errorf("the invite to the group was lost: %v", err)
	}
	// The ticked server was copied to the group, key and all; the owner keeps theirs.
	gs, err := s.ListGroupServers(2)
	if err != nil || len(gs) != 1 || gs[0].URL != "https://mail.example/mcp" || gs[0].Headers != "Authorization: Bearer x" || !gs[0].Asks() || gs[0].GroupID != 2 {
		t.Fatalf("the group's servers = %+v, %v", gs, err)
	}
	if own, _ := s.ListMCPServers(1); len(own) != 1 || own[0].ID != 1 {
		t.Fatalf("the owner's servers = %+v", own)
	}
	if used, _ := s.ThreadMCPServers(ours); len(used) != 1 || used[0].ID != gs[0].ID {
		t.Fatalf("the group uses %+v", used)
	}
	if off := s.ThreadToolsOff(2); len(off) != 0 {
		t.Errorf("the group kept personal switches: %v", off)
	}
	if off := s.ThreadToolsOff(1); !off[ToolPlan] {
		t.Errorf("the personal chat lost its switches: %v", off)
	}

	// Memory came through the rebuild.
	m, err := s.MemoryByID(UserMemory(1), 1)
	if err != nil || len(m.Entities) != 2 || len(m.Relations) != 1 || m.Relations[0].Subject != "Sam" || m.Relations[0].Object != "Acme Bakery" || !m.Pinned {
		t.Fatalf("alice's memory = %+v, %v", m, err)
	}
	if hits, _ := s.SearchMemoryText(UserMemory(1), "bakery", 5); len(hits) != 1 {
		t.Fatalf("word search after the migration: %+v", hits)
	}
	if b, err := s.MemoryByID(UserMemory(2), 2); err != nil || len(b.Entities) != 1 {
		t.Fatalf("bob's memory = %+v, %v", b, err)
	}
	if st := s.MemoryStats(UserMemory(1), ""); st.Memories != 1 || st.Entities != 2 || st.Relations != 1 {
		t.Fatalf("alice's numbers = %+v", st)
	}

	// The group can now have a thing called Sam of its own, next to alice's.
	if _, err := s.CreateMemory(GroupMemory(ours), MemoryInput{Content: "Sam leads the launch.", About: []string{"Sam"}, Source: MemoryFromAI, SaidBy: 2}); err != nil {
		t.Fatalf("a group memory about a name the owner also has: %v", err)
	}
	if st := s.MemoryStats(GroupMemory(ours), ""); st.Memories != 1 || st.Entities != 1 {
		t.Fatalf("the group's numbers = %+v", st)
	}
	if st := s.MemoryStats(UserMemory(1), ""); st.Memories != 1 || st.Entities != 2 {
		t.Fatalf("alice's numbers changed: %+v", st)
	}
	if _, err := s.MemoryByID(UserMemory(1), 3); err != ErrNotFound {
		t.Fatalf("the group's memory is readable as the owner's own: %v", err)
	}
	// Deleting a personal memory still drops things nothing mentions, and only the owner's.
	if err := s.DeleteMemory(UserMemory(1), 1); err != nil {
		t.Fatal(err)
	}
	if st := s.MemoryStats(GroupMemory(ours), ""); st.Entities != 1 {
		t.Fatalf("deleting a personal memory removed the group's things: %+v", st)
	}

	var broken int
	s.DB.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&broken)
	if broken != 0 {
		t.Fatalf("%d broken references after the migration", broken)
	}
}
