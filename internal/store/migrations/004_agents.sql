-- Agents: saved background workers with their own instructions, tools and
-- to-do list, plus the agent store where users share them.

-- An agent owns one thread (its activity log / control chat). Each run of the
-- agent is an ordinary run in that thread, built with a fresh context.
CREATE TABLE agents (
    id               INTEGER PRIMARY KEY,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    thread_id        INTEGER NOT NULL UNIQUE REFERENCES threads(id) ON DELETE CASCADE,
    name             TEXT    NOT NULL,
    description      TEXT    NOT NULL DEFAULT '',
    instructions     TEXT    NOT NULL DEFAULT '',  -- core job; edited by the owner only
    notes            TEXT    NOT NULL DEFAULT '',  -- adjustments; a connected chat may edit these
    memory           TEXT    NOT NULL DEFAULT '',  -- the agent's own scratchpad between runs
    max_steps        INTEGER NOT NULL DEFAULT 20,  -- LLM <-> tool round trips per run
    schedule_minutes INTEGER NOT NULL DEFAULT 0,   -- 0 = on demand only
    only_with_tasks  INTEGER NOT NULL DEFAULT 0,   -- scheduled runs are skipped when nothing is pending
    next_run_at      TEXT,
    daily_runs       INTEGER NOT NULL DEFAULT 48,  -- cap on automatic runs per UTC day
    history_runs     INTEGER NOT NULL DEFAULT 5,   -- past runs a connected chat may read at once
    enabled          INTEGER NOT NULL DEFAULT 1,   -- 0 = paused
    store_id         INTEGER,                      -- store listing it was installed from
    created_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX agents_user ON agents(user_id);
CREATE INDEX agents_due ON agents(enabled, next_run_at);

-- MCP servers an agent may use. tools = '' means every tool of the server,
-- otherwise a comma-separated allowlist of tool names.
CREATE TABLE agent_mcp (
    agent_id      INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id INTEGER NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tools         TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (agent_id, mcp_server_id)
);

-- One to-do list for all agents.
-- status: pending | in_progress | done | failed | cancelled
CREATE TABLE agent_tasks (
    id               INTEGER PRIMARY KEY,
    agent_id         INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    title            TEXT    NOT NULL,
    details          TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'pending',
    result           TEXT    NOT NULL DEFAULT '',
    created_by       TEXT    NOT NULL DEFAULT 'user',  -- user | chat | agent
    source_thread_id INTEGER REFERENCES threads(id) ON DELETE SET NULL,
    created_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX agent_tasks_agent ON agent_tasks(agent_id, status);

-- Every change to an agent's notes, so the owner can see who changed what.
CREATE TABLE agent_notes_log (
    id         INTEGER PRIMARY KEY,
    agent_id   INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    notes      TEXT    NOT NULL,
    changed_by TEXT    NOT NULL DEFAULT 'user',  -- user | chat
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX agent_notes_log_agent ON agent_notes_log(agent_id, id);

-- Agents connected to a chat: that chat's AI may manage their tasks.
CREATE TABLE thread_agents (
    thread_id INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    agent_id  INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    PRIMARY KEY (thread_id, agent_id)
);

-- The agent store: shared, ready-made agents. A listing never contains MCP
-- URLs or credentials; `requirements` only describes which tools are needed.
CREATE TABLE store_agents (
    id               INTEGER PRIMARY KEY,
    author_user_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    author_name      TEXT    NOT NULL DEFAULT '',
    source_agent_id  INTEGER REFERENCES agents(id) ON DELETE SET NULL,
    name             TEXT    NOT NULL,
    description      TEXT    NOT NULL DEFAULT '',
    guide            TEXT    NOT NULL DEFAULT '',  -- markdown: how to set up and use
    requirements     TEXT    NOT NULL DEFAULT '',  -- which MCP tools it needs
    instructions     TEXT    NOT NULL DEFAULT '',
    max_steps        INTEGER NOT NULL DEFAULT 20,
    schedule_minutes INTEGER NOT NULL DEFAULT 0,
    only_with_tasks  INTEGER NOT NULL DEFAULT 0,
    installs         INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
