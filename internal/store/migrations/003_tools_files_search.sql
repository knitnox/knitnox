-- Tool approval, custom instructions, pin/archive, file attachments,
-- reasoning, full-text search, audit log, push subscriptions.

-- MCP servers: 'auto' runs tools immediately, 'ask' pauses for approval.
ALTER TABLE mcp_servers ADD COLUMN approval TEXT NOT NULL DEFAULT 'auto' CHECK (approval IN ('auto', 'ask'));

-- A tool call waiting for (or decided by) a person.
-- status: pending | allowed | denied | expired
CREATE TABLE tool_approvals (
    id           INTEGER PRIMARY KEY,
    thread_id    INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    run_id       INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    message_id   INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    tool_call_id TEXT    NOT NULL,
    server_id    INTEGER NOT NULL,
    server_name  TEXT    NOT NULL,
    tool_name    TEXT    NOT NULL,
    args         TEXT    NOT NULL DEFAULT '',
    status       TEXT    NOT NULL DEFAULT 'pending',
    decided_by   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    decided_at   TEXT
);
CREATE INDEX tool_approvals_thread ON tool_approvals(thread_id, status);

-- Custom instructions: per user (solo chats) and per thread (always).
ALTER TABLE users ADD COLUMN instructions TEXT NOT NULL DEFAULT '';
ALTER TABLE threads ADD COLUMN instructions TEXT NOT NULL DEFAULT '';

-- Pin / archive are per member, so shared chats can be organised individually.
ALTER TABLE thread_members ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE thread_members ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;

-- Attachments can now be documents too. kind: image | file.
-- text holds extracted text for files (PDF / plain text), capped by the app.
ALTER TABLE attachments ADD COLUMN kind TEXT NOT NULL DEFAULT 'image';
ALTER TABLE attachments ADD COLUMN text TEXT NOT NULL DEFAULT '';

-- Reasoning / thinking tokens streamed by some models (kept, shown collapsed).
ALTER TABLE messages ADD COLUMN reasoning TEXT NOT NULL DEFAULT '';

-- Full-text search over message content (external-content FTS5 table).
CREATE VIRTUAL TABLE messages_fts USING fts5(content, content='messages', content_rowid='id', tokenize='unicode61');
CREATE TRIGGER messages_ai AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;
CREATE TRIGGER messages_ad AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
END;
CREATE TRIGGER messages_au AFTER UPDATE OF content ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;
INSERT INTO messages_fts(messages_fts) VALUES ('rebuild');

-- Who changed what in the admin panel.
CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY,
    actor_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    actor_name TEXT    NOT NULL DEFAULT '',
    action     TEXT    NOT NULL,
    target     TEXT    NOT NULL DEFAULT '',
    details    TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX audit_time ON audit_log(created_at);

-- Web Push subscriptions (one row per browser/device).
CREATE TABLE push_subscriptions (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT    NOT NULL UNIQUE,
    p256dh     TEXT    NOT NULL,
    auth       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX push_user ON push_subscriptions(user_id);
