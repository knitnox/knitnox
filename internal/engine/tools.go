package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"knitnox/internal/llm"
	"knitnox/internal/mcp"
	"knitnox/internal/store"
)

const maxToolOutput = 20000 // characters fed back to the model per tool call

// toolset is the set of tools available during one run: MCP tools plus the
// built-in tools (agents, memory), which run inside the engine.
type toolset struct {
	defs    []llm.Tool
	routes  map[string]toolRoute // exposed name -> server + real name
	local   map[string]localTool // built-in tools (see agents.go)
	clients []*mcp.Client
}

// localTool is a built-in tool. It always returns text for the model.
type localTool func(ctx context.Context, args json.RawMessage) string

func (ts *toolset) addLocal(name, description, schema string, fn localTool) {
	ts.local[name] = fn
	ts.defs = append(ts.defs, llm.Tool{Type: "function", Function: llm.FunctionDef{
		Name: name, Description: description, Parameters: json.RawMessage(schema),
	}})
}

type toolRoute struct {
	client   *mcp.Client
	name     string
	server   string
	serverID int64
	ask      bool // server is in "ask before running" mode
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// openTools builds the run's toolset. A chat gets its thread's MCP servers and,
// if agents are connected, the tools to manage them. An agent run gets the
// servers and tools its owner picked, plus its own task tools.
func (e *Engine) openTools(ctx context.Context, tc *turnCtx) (*toolset, error) {
	ts := &toolset{routes: map[string]toolRoute{}, local: map[string]localTool{}}
	if tc.agent != nil {
		servers, err := e.st.AgentMCPServers(tc.agent)
		if err != nil {
			return ts, err
		}
		allow, err := e.st.AgentMCP(tc.agent.ID)
		if err != nil {
			return ts, err
		}
		e.connectServers(ctx, ts, servers, allow)
		e.addAgentTools(ts, tc)
		if tc.agent.UseMemory && e.st.MemoryEnabled(tc.agent.UserID) {
			tc.memoryUser = tc.agent.UserID
			e.addMemoryTools(ts, tc.memoryUser, 0, false)
		}
		if tc.agent.PlanAccess != store.PlanAccessNone && e.st.PlanEnabled(tc.agent.UserID) {
			tc.planOn, tc.planWrite = true, tc.agent.PlanAccess == store.PlanAccessWrite
			e.addPlanTools(ts, tc.agent.UserID, store.PlanActor{Kind: store.PlanByAgent, Name: tc.agent.Name}, tc.planWrite, tc.loc)
		}
		if tc.agent.FeedAccess == store.FeedAccessPost {
			tc.feedOn, tc.feedDaily, tc.feedMedia = true, max(tc.agent.FeedDaily, 1), e.feedMediaNote(tc.agent.UserID)
			e.addFeedTools(ts, tc.agent.UserID, store.FeedActor{Kind: store.FeedByAgent, Name: tc.agent.Name, AgentID: tc.agent.ID, ThreadID: tc.agent.ThreadID}, tc.feedDaily, false, tc.loc, tc.checkModel())
		}
		return ts, nil
	}
	servers, err := e.st.ThreadMCPServers(tc.thread, tc.group)
	if err != nil {
		return ts, err
	}
	e.connectServers(ctx, ts, servers, nil)
	tc.connected = e.addManagerTools(ts, tc)
	// Long-term memory is personal, so it stays out of group chats.
	if !tc.group && e.st.MemoryEnabled(tc.thread.OwnerUserID) {
		tc.memoryUser, tc.memoryWrite = tc.thread.OwnerUserID, true
		e.addMemoryTools(ts, tc.memoryUser, tc.thread.ID, true)
	}
	// So is the Plan Book.
	if !tc.group && e.st.PlanEnabled(tc.thread.OwnerUserID) {
		tc.planOn, tc.planWrite = true, true
		e.addPlanTools(ts, tc.thread.OwnerUserID, store.PlanActor{Kind: store.PlanByAI, ThreadID: tc.thread.ID}, true, tc.loc)
	}
	// And the feed: the AI posts there only in the owner's own chats.
	if !tc.group && e.st.FeedEnabled(tc.thread.OwnerUserID) {
		tc.feedOn, tc.feedMedia = true, e.feedMediaNote(tc.thread.OwnerUserID)
		e.addFeedTools(ts, tc.thread.OwnerUserID, store.FeedActor{Kind: store.FeedByAI, ThreadID: tc.thread.ID}, 0, true, tc.loc, tc.checkModel())
	}
	// Pictures are made with the owner's model and count toward the owner's
	// daily limit, so the tool is offered only in the owner's own chats.
	if !tc.group {
		tc.pictureOn = e.addPictureTool(ts, tc)
	}
	return ts, nil
}

// connectServers connects to MCP servers in parallel and adds their tools. A
// server that fails to connect is skipped (logged), so one broken server does
// not block the run. allow, when non-nil, is a per-server tool allowlist.
func (e *Engine) connectServers(ctx context.Context, ts *toolset, servers []store.MCPServer, allow map[int64]store.AgentTool) {
	if len(servers) == 0 {
		return
	}

	type conn struct {
		client *mcp.Client
		tools  []mcp.Tool
	}
	results := make([]conn, len(servers))
	var wg sync.WaitGroup
	for i, srv := range servers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer e.recoverPanic("connecting to tool server " + srv.Name)
			cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			c, err := mcp.Connect(cctx, srv.URL, srv.Headers)
			if err != nil {
				e.log.Warn("mcp connect failed", "server", srv.Name, "err", err)
				return
			}
			tools, err := c.ListTools(cctx)
			if err != nil {
				e.log.Warn("mcp tools/list failed", "server", srv.Name, "err", err)
				c.Close()
				return
			}
			results[i] = conn{c, tools}
		}()
	}
	wg.Wait()

	for i, r := range results {
		if r.client == nil {
			continue
		}
		ts.clients = append(ts.clients, r.client)
		for _, t := range r.tools {
			if allow != nil && !allow[servers[i].ID].Allows(t.Name) {
				continue
			}
			// Prefix with server id so two servers can expose the same tool name.
			name := unsafeName.ReplaceAllString(fmt.Sprintf("s%d_%s", servers[i].ID, t.Name), "_")
			if len(name) > 64 {
				name = name[:64]
			}
			if _, dup := ts.routes[name]; dup {
				continue
			}
			params := t.InputSchema
			if len(params) == 0 || string(params) == "null" {
				params = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			ts.routes[name] = toolRoute{client: r.client, name: t.Name, server: servers[i].Name,
				serverID: servers[i].ID, ask: servers[i].Asks()}
			ts.defs = append(ts.defs, llm.Tool{Type: "function", Function: llm.FunctionDef{
				Name:        name,
				Description: "[" + servers[i].Name + "] " + t.Description,
				Parameters:  params,
			}})
		}
	}
}

// call executes one tool call and always returns text for the model. Transport
// errors are retried; tool-level errors (isError) are returned as-is so the
// model can react.
func (ts *toolset) call(ctx context.Context, call llm.ToolCall, retries int) string {
	args := json.RawMessage(call.Function.Arguments)
	if strings.TrimSpace(call.Function.Arguments) == "" {
		args = json.RawMessage("{}")
	}
	if !json.Valid(args) {
		return "Error: tool arguments were not valid JSON: " + call.Function.Arguments
	}
	if fn, ok := ts.local[call.Function.Name]; ok {
		return fn(ctx, args)
	}
	route, ok := ts.routes[call.Function.Name]
	if !ok {
		return "Error: unknown tool " + call.Function.Name
	}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return "Error: cancelled"
			}
		}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		text, isErr, err := route.client.CallTool(cctx, route.name, args)
		cancel()
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return "Error: cancelled"
			}
			if !mcp.NotSent(err) {
				// The request left this server, so the tool may have run. Asking
				// again could do the same thing twice; tell the model instead.
				return fmt.Sprintf("Error: tool %q did not answer (%v). It may or may not have run. Do not call it again blindly: "+
					"if it changes something (sends, books, pays, deletes), first check whether that already happened, or tell the user.", route.name, err)
			}
			continue
		}
		if r := []rune(text); len(r) > maxToolOutput {
			text = string(r[:maxToolOutput]) + "\n…[output truncated]"
		}
		if isErr {
			return "Tool reported an error: " + text
		}
		if strings.TrimSpace(text) == "" {
			return "(tool returned no output)"
		}
		return text
	}
	return fmt.Sprintf("Error: tool %q could not be reached after %d attempts, so it did not run: %v", route.name, retries+1, lastErr)
}

func (ts *toolset) close() {
	for _, c := range ts.clients {
		c.Close()
	}
}

// awaitApproval records a pending tool call and blocks until the run author
// or the thread owner decides, the approval times out, or the run is
// cancelled. It returns the final status (allowed / denied / expired).
func (e *Engine) awaitApproval(ctx context.Context, tc *turnCtx, msgID int64, call llm.ToolCall, route toolRoute) (string, error) {
	id, err := e.st.CreateApproval(&store.ToolApproval{
		ThreadID: tc.thread.ID, RunID: tc.run.ID, MessageID: msgID, ToolCallID: call.ID,
		ServerID: route.serverID, ServerName: route.server, ToolName: route.name, Args: call.Function.Arguments,
	})
	if err != nil {
		return "", err
	}
	ch := make(chan string, 1)
	e.mu.Lock()
	e.approvals[id] = ch
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.approvals, id)
		e.mu.Unlock()
	}()

	e.hub.Publish(tc.thread.ID, EvMessages, nil)
	e.hub.Publish(tc.thread.ID, EvApproval, map[string]any{"id": id, "tool": route.name, "server": route.server})
	if e.OnApproval != nil {
		approvers := []int64{tc.run.AuthorUserID}
		if tc.thread.OwnerUserID != tc.run.AuthorUserID {
			approvers = append(approvers, tc.thread.OwnerUserID)
		}
		ev := ApprovalEvent{ThreadID: tc.thread.ID, RunID: tc.run.ID, ApprovalID: id,
			Approvers: approvers, ToolName: route.name, ServerName: route.server}
		e.hook("approval hook", func() { e.OnApproval(ev) })
	}

	timeout := time.Duration(max(tc.settings.ApprovalTimeoutMin, 1)) * time.Minute
	if e.ApprovalTimeout > 0 { // tests
		timeout = e.ApprovalTimeout
	}
	select {
	case status := <-ch:
		e.hub.Publish(tc.thread.ID, EvMessages, nil)
		return status, nil
	case <-time.After(timeout):
		e.st.DecideApproval(id, 0, store.ApprovalExpired)
		e.hub.Publish(tc.thread.ID, EvMessages, nil)
		return store.ApprovalExpired, nil
	case <-ctx.Done():
		e.st.DecideApproval(id, 0, store.ApprovalExpired)
		return "", ctx.Err()
	}
}
