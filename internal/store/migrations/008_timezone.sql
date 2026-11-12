-- Each user's time zone (an IANA name such as "Asia/Dhaka"), so the AI can be
-- told the user's local date and time. Empty = not known yet, UTC is used.
-- timezone_auto: the browser keeps it up to date; 0 = the user fixed it by hand.
ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN timezone_auto INTEGER NOT NULL DEFAULT 1;
