-- The voice a voice model speaks with ("alloy", "Kore"…), set on the model
-- itself, so the admin can choose it for a model shared with everyone.
-- '' = the model's usual voice. A person's own choice in their Feed settings wins.
ALTER TABLE models ADD COLUMN voice_name TEXT NOT NULL DEFAULT '';
