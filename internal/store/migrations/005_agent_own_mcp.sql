-- An MCP server can belong to one agent. Such a server is used by that agent
-- only: it is not offered to the owner's chats or to their other agents.
-- agent_id NULL = a normal (shared) server of the user.
ALTER TABLE mcp_servers ADD COLUMN agent_id INTEGER REFERENCES agents(id) ON DELETE CASCADE;
CREATE INDEX mcp_servers_agent ON mcp_servers(agent_id);
