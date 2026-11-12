-- Feed: short posts in one place. A person, the AI in their chats and (when
-- allowed) their agents post into the person's feed. A post is private unless
-- its owner shares it with everyone on this server.

-- Per-user: may the AI in my chats post to my feed, and up to which post have I looked?
CREATE TABLE feed_settings (
    user_id    INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    ai_enabled INTEGER NOT NULL DEFAULT 0,
    seen_id    INTEGER NOT NULL DEFAULT 0
);

-- One post. body is the short text; report is an optional longer text
-- (Markdown); chart is an optional small chart as JSON, drawn by the server;
-- svg is an optional picture drawn by the AI, only ever shown as an image.
-- created_by: user | ai | agent (actor_name = the agent's name).
-- deleted_at set = in the bin; the janitor removes it for good after 30 days.
CREATE TABLE feed_posts (
    id               INTEGER PRIMARY KEY,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body             TEXT    NOT NULL,
    report           TEXT    NOT NULL DEFAULT '',
    chart            TEXT    NOT NULL DEFAULT '',
    svg              TEXT    NOT NULL DEFAULT '',
    visibility       TEXT    NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
    created_by       TEXT    NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'ai', 'agent')),
    actor_name       TEXT    NOT NULL DEFAULT '',
    agent_id         INTEGER REFERENCES agents(id) ON DELETE SET NULL,
    source_thread_id INTEGER REFERENCES threads(id) ON DELETE SET NULL,
    edited_at        TEXT,              -- set when the owner changed the text
    deleted_at       TEXT,
    created_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX feed_posts_user ON feed_posts(user_id, deleted_at, id);
CREATE INDEX feed_posts_public ON feed_posts(visibility, deleted_at, id);
CREATE INDEX feed_posts_agent ON feed_posts(agent_id, created_at);

-- Who has been shown a public post (never its owner). The read count is the number of rows.
CREATE TABLE feed_reads (
    post_id INTEGER NOT NULL REFERENCES feed_posts(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, user_id)
) WITHOUT ROWID;

-- What an agent may do with its owner's feed (none | post), and how many posts it may add per day.
ALTER TABLE agents ADD COLUMN feed_access TEXT NOT NULL DEFAULT 'none' CHECK (feed_access IN ('none', 'post'));
ALTER TABLE agents ADD COLUMN feed_daily INTEGER NOT NULL DEFAULT 5;
