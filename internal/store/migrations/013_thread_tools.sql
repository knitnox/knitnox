-- Tools switched off for one chat. A chat gets every tool its owner has set
-- up unless a row here says otherwise, so new tools work without a visit to
-- every old chat. key is a built-in group (memory, plan, feed, pictures,
-- agents) or one tool server ("mcp:<id>").
CREATE TABLE thread_tools_off (
    thread_id INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    key       TEXT    NOT NULL,
    PRIMARY KEY (thread_id, key)
);
