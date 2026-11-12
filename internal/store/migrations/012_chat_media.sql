-- Pictures the AI makes in a chat, and answers read aloud, are stored like
-- uploads: a file in DATA/uploads and a row in attachments, linked to the AI's
-- message. "made" tells them apart from files a person attached:
--   0 = uploaded by a person
--   1 = made by the user's own model
--   2 = made by a model the server shares (counts toward the daily media limit)
ALTER TABLE attachments ADD COLUMN made INTEGER NOT NULL DEFAULT 0;
CREATE INDEX attachments_made ON attachments(user_id, made, created_at);
