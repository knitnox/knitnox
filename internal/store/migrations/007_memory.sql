-- Long-term memory: short facts saved by a person or by the AI, the things
-- they mention (entities) and how those things are connected (relations).
-- Everything belongs to one user; nothing here is shared between users.

-- Per-user switch and, optionally, the user's own embedding model.
CREATE TABLE memory_settings (
    user_id           INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled           INTEGER NOT NULL DEFAULT 0,  -- the AI may read and write this user's memory
    embed_provider_id INTEGER REFERENCES providers(id) ON DELETE SET NULL,
    embed_model       TEXT    NOT NULL DEFAULT '',
    embed_dimensions  INTEGER NOT NULL DEFAULT 0   -- 0 = the model's own size
);

-- One remembered fact. source: user (typed on the Memory page) | ai (saved in a chat).
-- embedding is a little-endian float32 vector, unit length; embed_key says
-- which model made it, so vectors from an old model are never compared.
CREATE TABLE memories (
    id               INTEGER PRIMARY KEY,
    user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content          TEXT    NOT NULL,
    source           TEXT    NOT NULL DEFAULT 'user' CHECK (source IN ('user', 'ai')),
    source_thread_id INTEGER REFERENCES threads(id) ON DELETE SET NULL,
    pinned           INTEGER NOT NULL DEFAULT 0,   -- always shown to the AI
    embedding        BLOB,
    embed_key        TEXT    NOT NULL DEFAULT '',
    created_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX memories_user ON memories(user_id, id);

-- A person, place, project or other thing memories talk about.
-- name_key is the lower-cased name, so "Tanim" and "tanim" are one thing.
CREATE TABLE memory_entities (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    name_key   TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, name_key)
);

-- Which memories are about which things.
CREATE TABLE memory_mentions (
    memory_id INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    entity_id INTEGER NOT NULL REFERENCES memory_entities(id) ON DELETE CASCADE,
    PRIMARY KEY (memory_id, entity_id)
) WITHOUT ROWID;
CREATE INDEX memory_mentions_entity ON memory_mentions(entity_id);

-- A connection stated by a memory: subject -predicate-> object.
CREATE TABLE memory_relations (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    memory_id  INTEGER NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    subject_id INTEGER NOT NULL REFERENCES memory_entities(id) ON DELETE CASCADE,
    predicate  TEXT    NOT NULL,
    object_id  INTEGER NOT NULL REFERENCES memory_entities(id) ON DELETE CASCADE
);
CREATE INDEX memory_relations_subject ON memory_relations(subject_id);
CREATE INDEX memory_relations_object ON memory_relations(object_id);
CREATE INDEX memory_relations_memory ON memory_relations(memory_id);

-- Word search over the text and the names of the things it is about.
-- Rows are written by the store; the trigger only cleans up after deletes
-- (including deletes that cascade from a removed user).
CREATE VIRTUAL TABLE memories_fts USING fts5(content, about, tokenize='unicode61');
CREATE TRIGGER memories_ad AFTER DELETE ON memories BEGIN
    DELETE FROM memories_fts WHERE rowid = old.id;
END;

-- An agent may be allowed to read (never write) its owner's memory.
ALTER TABLE agents ADD COLUMN use_memory INTEGER NOT NULL DEFAULT 0;
