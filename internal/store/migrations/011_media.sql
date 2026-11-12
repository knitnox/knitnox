-- Model kinds, and pictures, voice and video on feed posts.

-- What a model is for. Chat lists show only 'chat' models; the Feed picks from
-- image, voice and video models; Memory uses embedding models.
ALTER TABLE models ADD COLUMN kind TEXT NOT NULL DEFAULT 'chat' CHECK (kind IN ('chat', 'embedding', 'image', 'voice', 'video'));
UPDATE models SET kind = 'embedding' WHERE lower(name) LIKE '%embed%';

-- Which models make the pictures, voice and video for a user's feed.
-- 0 = the server's model (the first shared model of that kind), -1 = off, otherwise a model id.
ALTER TABLE feed_settings ADD COLUMN image_model_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feed_settings ADD COLUMN voice_model_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feed_settings ADD COLUMN video_model_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feed_settings ADD COLUMN voice_name TEXT NOT NULL DEFAULT '';

-- One piece of media per post, made by an AI model after the post is written.
-- media_kind: '' | image | voice | video. media_status: '' | pending | ready | failed.
-- media_file is the name on disk in the feed media folder. media_prompt is
-- what the model was asked for; media_note says what went wrong, or that the
-- picture was redrawn after a check. media_shared = made with a server model
-- (counts toward the daily limit the admin sets).
ALTER TABLE feed_posts ADD COLUMN media_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE feed_posts ADD COLUMN media_status TEXT NOT NULL DEFAULT '';
ALTER TABLE feed_posts ADD COLUMN media_file TEXT NOT NULL DEFAULT '';
ALTER TABLE feed_posts ADD COLUMN media_mime TEXT NOT NULL DEFAULT '';
ALTER TABLE feed_posts ADD COLUMN media_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE feed_posts ADD COLUMN media_note TEXT NOT NULL DEFAULT '';
ALTER TABLE feed_posts ADD COLUMN media_w INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feed_posts ADD COLUMN media_h INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feed_posts ADD COLUMN media_shared INTEGER NOT NULL DEFAULT 0;
