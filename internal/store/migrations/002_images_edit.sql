-- Image attachments, per-model vision flag, edited messages.

ALTER TABLE models ADD COLUMN vision INTEGER NOT NULL DEFAULT 1; -- model accepts image input

ALTER TABLE messages ADD COLUMN edited INTEGER NOT NULL DEFAULT 0;

-- Files live in DATA/uploads/<file_name>; rows are removed by cascade and
-- orphaned files are swept by the janitor.
CREATE TABLE attachments (
    id         INTEGER PRIMARY KEY,
    thread_id  INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    file_name  TEXT    NOT NULL UNIQUE,  -- random name on disk
    orig_name  TEXT    NOT NULL DEFAULT '',
    mime       TEXT    NOT NULL,
    size       INTEGER NOT NULL,
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX attachments_message ON attachments(message_id);
CREATE INDEX attachments_thread ON attachments(thread_id);
