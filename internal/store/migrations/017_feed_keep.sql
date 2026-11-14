-- Feed: old posts are removed by themselves, people add their own pictures.

-- How many days a user's posts are kept. Older ones are removed for good,
-- with their media files, by the janitor. 0 = never remove.
ALTER TABLE feed_settings ADD COLUMN keep_days INTEGER NOT NULL DEFAULT 30;

-- kept = the owner marked the post "Keep": it is never removed for being old.
ALTER TABLE feed_posts ADD COLUMN kept INTEGER NOT NULL DEFAULT 0;

-- media_own = the picture was added by the owner (a photo from their device),
-- not made by an AI model.
ALTER TABLE feed_posts ADD COLUMN media_own INTEGER NOT NULL DEFAULT 0;
