-- Groups and read-only links.
--
-- Until now a chat became a "group chat" when a second person joined it, and
-- went back to a personal chat when the last one left. From here on the kind
-- of a chat is fixed when it is made:
--   chat   one person's own chat. It can be shown to others with a read-only
--          link (thread_shares), but nobody else can join or write.
--   group  a chat for several people. It has its own memory, its own tool
--          servers and one model chosen by its admin, and never uses anyone's
--          personal tools (memory, Plan Book, Feed, pictures, agents).

ALTER TABLE threads ADD COLUMN kind TEXT NOT NULL DEFAULT 'chat';
-- 1 = this chat was shared before groups existed; its owner is told once that
-- earlier messages may hold results of their personal tools.
ALTER TABLE threads ADD COLUMN group_notice INTEGER NOT NULL DEFAULT 0;

-- Chats that already have more than one person become groups.
UPDATE threads SET kind = 'group', group_notice = 1
 WHERE id NOT IN (SELECT thread_id FROM agents)
   AND (SELECT COUNT(*) FROM thread_members m WHERE m.thread_id = threads.id) > 1;

-- A personal chat can no longer be joined, so open invites to one are void.
DELETE FROM thread_invites WHERE thread_id IN (SELECT id FROM threads WHERE kind = 'chat');
-- A group has no personal tools: forget their per-chat switches and the
-- owner's agents that were connected.
DELETE FROM thread_tools_off WHERE thread_id IN (SELECT id FROM threads WHERE kind = 'group');
DELETE FROM thread_agents WHERE thread_id IN (SELECT id FROM threads WHERE kind = 'group');

-- A tool server can belong to one group (like agent_id for agents). Such a
-- server is used in that group only. group_id NULL = not a group's server.
ALTER TABLE mcp_servers ADD COLUMN group_id INTEGER REFERENCES threads(id) ON DELETE CASCADE;
CREATE INDEX mcp_servers_group ON mcp_servers(group_id);

-- The owner's servers that were ticked for a shared chat are copied, so the
-- group keeps working and stops depending on the owner's personal list.
INSERT INTO mcp_servers(user_id, name, url, headers_enc, enabled, approval, group_id)
SELECT s.user_id, s.name, s.url, s.headers_enc, s.enabled, s.approval, tm.thread_id
  FROM thread_mcp tm
  JOIN mcp_servers s ON s.id = tm.mcp_server_id
  JOIN threads t ON t.id = tm.thread_id
 WHERE t.kind = 'group' AND s.agent_id IS NULL;

-- The embedding model of a group's memory, chosen by its admin. No row, or no
-- provider = the server's model.
CREATE TABLE group_settings (
    thread_id         INTEGER PRIMARY KEY REFERENCES threads(id) ON DELETE CASCADE,
    embed_provider_id INTEGER REFERENCES providers(id) ON DELETE SET NULL,
    embed_model       TEXT    NOT NULL DEFAULT '',
    embed_dimensions  INTEGER NOT NULL DEFAULT 0
);

-- Groups a personal chat may look into: its AI can search the group's memory
-- and read its recent messages, as long as the chat's owner is a member.
CREATE TABLE thread_groups (
    thread_id INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    group_id  INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    PRIMARY KEY (thread_id, group_id)
);
CREATE INDEX thread_groups_group ON thread_groups(group_id);

-- The read-only link of a personal chat. Anyone who has the link can read the
-- chat up to run until_run; newer messages stay private until the owner
-- updates the link. show_tools = 1 also shows tool calls and their results.
CREATE TABLE thread_shares (
    thread_id  INTEGER PRIMARY KEY REFERENCES threads(id) ON DELETE CASCADE,
    token      TEXT    NOT NULL UNIQUE,
    until_run  INTEGER NOT NULL DEFAULT 0,
    show_tools INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Memory gets a second kind of owner. group_id 0 = a person's own memory (as
-- before); otherwise the memory of that group, and user_id is the group's
-- owner. said_by is the person whose message made the AI save a group fact.
ALTER TABLE memories ADD COLUMN group_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE memories ADD COLUMN said_by INTEGER REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX memories_group ON memories(group_id, id);

-- Things are unique per owner, and a group is an owner of its own, so the
-- UNIQUE rule of memory_entities has to include the group. SQLite cannot
-- change such a rule in place: the table is made again, and with it the two
-- tables that point at it (their rows would be removed with the old table).
CREATE TABLE memory_mentions_old AS SELECT memory_id, entity_id FROM memory_mentions;
CREATE TABLE memory_relations_old AS
    SELECT id, user_id, memory_id, subject_id, predicate, object_id FROM memory_relations;
DROP TABLE memory_mentions;
DROP TABLE memory_relations;

CREATE TABLE memory_entities_new (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id   INTEGER NOT NULL DEFAULT 0,
    name       TEXT    NOT NULL,
    name_key   TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, group_id, name_key)
);
INSERT INTO memory_entities_new(id, user_id, group_id, name, name_key, created_at)
    SELECT id, user_id, 0, name, name_key, created_at FROM memory_entities;
DROP TABLE memory_entities;
ALTER TABLE memory_entities_new RENAME TO memory_entities;
CREATE INDEX memory_entities_group ON memory_entities(group_id);

CREATE TABLE memory_mentions (
    memory_id INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    entity_id INTEGER NOT NULL REFERENCES memory_entities(id) ON DELETE CASCADE,
    PRIMARY KEY (memory_id, entity_id)
) WITHOUT ROWID;
CREATE INDEX memory_mentions_entity ON memory_mentions(entity_id);
INSERT INTO memory_mentions(memory_id, entity_id) SELECT memory_id, entity_id FROM memory_mentions_old;
DROP TABLE memory_mentions_old;

CREATE TABLE memory_relations (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id   INTEGER NOT NULL DEFAULT 0,
    memory_id  INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    subject_id INTEGER NOT NULL REFERENCES memory_entities(id) ON DELETE CASCADE,
    predicate  TEXT    NOT NULL,
    object_id  INTEGER NOT NULL REFERENCES memory_entities(id) ON DELETE CASCADE
);
CREATE INDEX memory_relations_subject ON memory_relations(subject_id);
CREATE INDEX memory_relations_object ON memory_relations(object_id);
CREATE INDEX memory_relations_memory ON memory_relations(memory_id);
INSERT INTO memory_relations(id, user_id, group_id, memory_id, subject_id, predicate, object_id)
    SELECT id, user_id, 0, memory_id, subject_id, predicate, object_id FROM memory_relations_old;
DROP TABLE memory_relations_old;

-- A group's memory goes with the group.
CREATE TRIGGER threads_ad_group_memory AFTER DELETE ON threads BEGIN
    DELETE FROM memories WHERE group_id = old.id;
    DELETE FROM memory_entities WHERE group_id = old.id;
END;
