-- Issues: a simple place for everyone on the server to report bugs, ideas and
-- questions. Every signed-in user can read them.
-- kind: bug | idea | question      status: open | done
CREATE TABLE issues (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    author_name TEXT    NOT NULL DEFAULT '',
    kind        TEXT    NOT NULL DEFAULT 'bug',
    title       TEXT    NOT NULL,
    body        TEXT    NOT NULL DEFAULT '',
    status      TEXT    NOT NULL DEFAULT 'open',
    reply       TEXT    NOT NULL DEFAULT '',  -- one answer from an admin
    reply_by    TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX issues_status ON issues(status, id);
