-- Plan Book: a person's own projects and tasks. The person, the AI in their
-- chats and (when allowed) their agents can all add and change tasks.
-- Everything belongs to one user; nothing here is shared between users.

-- Per-user switch: may the AI in my chats use my Plan Book?
CREATE TABLE plan_settings (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled INTEGER NOT NULL DEFAULT 0
);

-- A project groups tasks. name_key is the lower-cased name: one name, one project.
CREATE TABLE plan_projects (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    name_key   TEXT    NOT NULL,
    notes      TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, name_key)
);

-- One task. status: todo | doing | done. due_date is a calendar day
-- (YYYY-MM-DD) in the owner's time zone, '' = no date.
-- created_by: user | ai | agent (actor_name = the agent's name).
-- deleted_at set = in the bin; the janitor removes it for good after 30 days.
CREATE TABLE plan_tasks (
    id               INTEGER PRIMARY KEY,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id       INTEGER REFERENCES plan_projects(id) ON DELETE SET NULL,
    title            TEXT    NOT NULL,
    notes            TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'doing', 'done')),
    due_date         TEXT    NOT NULL DEFAULT '',
    created_by       TEXT    NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'ai', 'agent')),
    actor_name       TEXT    NOT NULL DEFAULT '',
    source_thread_id INTEGER REFERENCES threads(id) ON DELETE SET NULL,
    done_at          TEXT,
    deleted_at       TEXT,
    created_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX plan_tasks_user ON plan_tasks(user_id, deleted_at, status, due_date);
CREATE INDEX plan_tasks_project ON plan_tasks(project_id);

-- Who changed what, so a change made by the AI or an agent is never a mystery.
CREATE TABLE plan_log (
    id         INTEGER PRIMARY KEY,
    task_id    INTEGER NOT NULL REFERENCES plan_tasks(id) ON DELETE CASCADE,
    actor      TEXT    NOT NULL,              -- user | ai | agent
    actor_name TEXT    NOT NULL DEFAULT '',
    action     TEXT    NOT NULL,              -- short sentence, e.g. "marked it done"
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX plan_log_task ON plan_log(task_id, id);

-- What an agent may do with its owner's Plan Book: none | read | write.
ALTER TABLE agents ADD COLUMN plan_access TEXT NOT NULL DEFAULT 'none' CHECK (plan_access IN ('none', 'read', 'write'));
