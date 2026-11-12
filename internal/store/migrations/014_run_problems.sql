-- Problems an agent runs into, made visible.

-- Tool (MCP) servers a run could not reach, as "name, name". The run goes on
-- without their tools, so until now nothing but the server log said so.
ALTER TABLE runs ADD COLUMN tools_down TEXT NOT NULL DEFAULT '';

-- A post Knitnox itself writes about an agent: 'problem' when it starts having
-- trouble, 'recovered' when it works again, '' for every ordinary post.
-- These do not count toward the agent's posts per day.
ALTER TABLE feed_posts ADD COLUMN notice TEXT NOT NULL DEFAULT '';
