// Package mcp is a minimal Model Context Protocol client for the
// "Streamable HTTP" transport: initialize, tools/list, tools/call.
//
// Only remote (HTTP) servers are supported on purpose: launching stdio servers
// would let users run arbitrary programs on the host.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const protocolVersion = "2025-06-18"

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Client struct {
	url       string
	headers   map[string]string
	sessionID string
	nextID    atomic.Int64
	http      *http.Client
}

// ParseHeaders turns "Key: Value" lines into a map.
func ParseHeaders(lines string) map[string]string {
	out := map[string]string{}
	for _, l := range strings.Split(lines, "\n") {
		k, v, ok := strings.Cut(l, ":")
		if ok && strings.TrimSpace(k) != "" {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

// Connect performs the MCP initialize handshake.
func Connect(ctx context.Context, url, headerLines string) (*Client, error) {
	c := &Client{url: url, headers: ParseHeaders(headerLines), http: &http.Client{Timeout: 60 * time.Second}}
	_, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "knitnox", "version": "0.1.0"},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	if err := c.notify(ctx, "notifications/initialized"); err != nil {
		return nil, fmt.Errorf("initialized: %w", err)
	}
	return c, nil
}

// ListTools returns all tools, following pagination.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var all []Tool
	cursor := ""
	for i := 0; i < 20; i++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.call(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var res struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return all, nil
}

// CallTool runs a tool and flattens its result into text for the model.
// isError reports a tool-level error (the model should see it, not a retry).
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (text string, isError bool, err error) {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	raw, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", false, err
	}
	var res struct {
		Content []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			MimeType string          `json:"mimeType"`
			Resource json.RawMessage `json:"resource"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		IsError           bool            `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", false, err
	}
	var b strings.Builder
	for _, item := range res.Content {
		switch item.Type {
		case "text":
			b.WriteString(item.Text)
		case "resource":
			b.Write(item.Resource)
		default:
			fmt.Fprintf(&b, "[%s content omitted]", item.Type)
		}
		b.WriteString("\n")
	}
	if b.Len() == 0 && len(res.StructuredContent) > 0 {
		b.Write(res.StructuredContent)
	}
	return strings.TrimSpace(b.String()), res.IsError, nil
}

// Close ends the session (best effort).
func (c *Client) Close() {
	if c.sessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, c.url, nil)
	c.setHeaders(req)
	if resp, err := c.http.Do(req); err == nil {
		resp.Body.Close()
	}
}

// ---- JSON-RPC over HTTP ----

type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
}

func (c *Client) post(ctx context.Context, payload any) (*http.Response, error) {
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
		resp.Body.Close()
		return nil, fmt.Errorf("mcp server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return resp, nil
}

func (c *Client) notify(ctx context.Context, method string) error {
	resp, err := c.post(ctx, map[string]any{"jsonrpc": "2.0", "method": method})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	resp, err := c.post(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	wantID := fmt.Sprint(id)
	match := func(r rpcResponse) (json.RawMessage, bool, error) {
		if strings.Trim(string(r.ID), `"`) != wantID {
			return nil, false, nil
		}
		if r.Error != nil {
			return nil, true, fmt.Errorf("mcp error %d: %s", r.Error.Code, r.Error.Message)
		}
		return r.Result, true, nil
	}

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		// Server streams JSON-RPC messages as SSE events; find our response.
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
		var data strings.Builder
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "data:") {
				data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				continue
			}
			if line == "" && data.Len() > 0 {
				var r rpcResponse
				if json.Unmarshal([]byte(data.String()), &r) == nil {
					if res, ok, err := match(r); ok {
						return res, err
					}
				}
				data.Reset()
			}
		}
		if data.Len() > 0 {
			var r rpcResponse
			if json.Unmarshal([]byte(data.String()), &r) == nil {
				if res, ok, err := match(r); ok {
					return res, err
				}
			}
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("mcp: stream ended without a response")
	}

	var r rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("mcp: bad response: %w", err)
	}
	res, _, err := match(r)
	return res, err
}
