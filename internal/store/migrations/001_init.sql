-- Parley initial schema.
-- Timestamps are UTC text: 'YYYY-MM-DD HH:MM:SS'.

CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    disabled      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,           -- sha256 of the cookie value
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf       TEXT    NOT NULL,
    expires_at TEXT    NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

-- owner_user_id NULL = global provider managed by admins (quota-billed).
-- owner_user_id set  = a user's personal provider (logged, never quota-billed).
CREATE TABLE providers (
    id            INTEGER PRIMARY KEY,
    owner_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT    NOT NULL,
    base_url      TEXT    NOT NULL,
    api_key_enc   BLOB,
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX providers_owner ON providers(owner_user_id);

CREATE TABLE models (
    id             INTEGER PRIMARY KEY,
    provider_id    INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name           TEXT    NOT NULL,              -- upstream model id, e.g. gpt-4o-mini
    context_window INTEGER NOT NULL DEFAULT 128000,
    input_price    REAL    NOT NULL DEFAULT 0,    -- USD per 1M prompt tokens
    output_price   REAL    NOT NULL DEFAULT 0,    -- USD per 1M completion tokens
    enabled        INTEGER NOT NULL DEFAULT 1,
    UNIQUE (provider_id, name)
);

-- token_limit 0 = unlimited.
CREATE TABLE quotas (
    user_id     INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    period      TEXT    NOT NULL DEFAULT 'monthly' CHECK (period IN ('daily', 'monthly')),
    token_limit INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE usage_log (
    id                INTEGER PRIMARY KEY,
    user_id           INTEGER REFERENCES users(id) ON DELETE SET NULL,
    thread_id         INTEGER,
    provider_id       INTEGER,
    model_name        TEXT    NOT NULL,
    kind              TEXT    NOT NULL DEFAULT 'chat',  -- chat | compaction
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost              REAL    NOT NULL DEFAULT 0,
    billable          INTEGER NOT NULL DEFAULT 1,       -- counts toward quota
    estimated         INTEGER NOT NULL DEFAULT 0,       -- provider sent no usage; chars/4 estimate
    created_at        TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX usage_user_time ON usage_log(user_id, created_at);
CREATE INDEX usage_time ON usage_log(created_at);

CREATE TABLE mcp_servers (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    url         TEXT    NOT NULL,
    headers_enc BLOB,                     -- encrypted "Key: Value" lines
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE threads (
    id                  INTEGER PRIMARY KEY,
    owner_user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title               TEXT    NOT NULL DEFAULT 'New chat',
    model_id            INTEGER REFERENCES models(id) ON DELETE SET NULL,
    allow_owner_key     INTEGER NOT NULL DEFAULT 0,  -- members may spend owner's personal provider
    summary             TEXT    NOT NULL DEFAULT '', -- compacted history
    compacted_until_run INTEGER NOT NULL DEFAULT 0,  -- runs with id <= this are in summary
    created_at          TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE thread_members (
    thread_id INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      TEXT    NOT NULL CHECK (role IN ('owner', 'member', 'viewer')),
    joined_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (thread_id, user_id)
);
CREATE INDEX thread_members_user ON thread_members(user_id);

CREATE TABLE thread_invites (
    token      TEXT    PRIMARY KEY,
    thread_id  INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT    NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'viewer')),
    expires_at TEXT    NOT NULL,
    max_uses   INTEGER NOT NULL DEFAULT 10,
    uses       INTEGER NOT NULL DEFAULT 0
);

-- MCP servers (owned by the thread owner) approved for use in a *group* thread.
CREATE TABLE thread_mcp (
    thread_id     INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    mcp_server_id INTEGER NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    PRIMARY KEY (thread_id, mcp_server_id)
);

-- A run = one queued user turn processed by the thread worker.
-- status: pending (queued) | running | done | failed | cancelled | interrupted
CREATE TABLE runs (
    id             INTEGER PRIMARY KEY,
    thread_id      INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    author_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    status         TEXT    NOT NULL DEFAULT 'pending',
    error          TEXT    NOT NULL DEFAULT '',
    steps          INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at     TEXT,
    finished_at    TEXT
);
CREATE INDEX runs_thread_status ON runs(thread_id, status);

-- Messages belong to a run; display order is (run_id, id).
-- role: user | assistant | tool
-- status: done | streaming | interrupted | failed
CREATE TABLE messages (
    id             INTEGER PRIMARY KEY,
    thread_id      INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    run_id         INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    author_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    role           TEXT    NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    content        TEXT    NOT NULL DEFAULT '',
    tool_calls     TEXT    NOT NULL DEFAULT '',  -- JSON array (assistant)
    tool_call_id   TEXT    NOT NULL DEFAULT '',  -- (tool)
    tool_name      TEXT    NOT NULL DEFAULT '',  -- (tool)
    status         TEXT    NOT NULL DEFAULT 'done',
    created_at     TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX messages_thread_order ON messages(thread_id, run_id, id);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
