// Command mockllm is a development stand-in for an OpenAI-compatible API and
// an MCP server, so Knitnox can be tried and tested without real API keys.
//
//	go run ./cmd/mockllm            # listens on :9090
//
// In Knitnox add a provider with base URL http://localhost:9090/v1 (any key)
// and a model named "mock". Add an MCP server with URL http://localhost:9090/mcp.
//
// Behaviour: replies with a streamed echo of your message. If your message
// contains the word "time" and MCP tools are offered, it calls the "clock" tool
// first; "think" streams a fake reasoning trace; images are counted back to you.
// With memory turned on (Memory page), "remember ..." saves your message as a
// memory and "recall ..." searches it. With the Plan Book turned on, "todo ..."
// adds a task and "plan" lists the open ones. With the Feed turned on, "post ..."
// adds a post with a short report and a small chart, "draw ..." a post with a
// diagram, and "picture ...", "say ..." and "film ..." a post with a picture, a
// recording or a video (add the models mock-image, mock-voice and mock-video and
// mark them as Pictures, Voice and Video). POST /v1/embeddings returns simple
// word-based vectors, so "smart search" can be tried with a model named "mock-embed".
// In a group, "remember ..." and "recall ..." use the group's own memory. In a
// personal chat that is connected to a group (Tools menu, or "Take to my chat"),
// "group recall ..." searches that group's memory and "group messages" reads its
// latest messages.
package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := ":9090"
	if v := os.Getenv("MOCK_ADDR"); v != "" {
		addr = v
	}
	http.HandleFunc("POST /v1/chat/completions", chat)
	http.HandleFunc("GET /v1/models", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "mock"}, map[string]any{"id": "mock-embed"},
			map[string]any{"id": "mock-image"}, map[string]any{"id": "mock-voice"}, map[string]any{"id": "mock-video"}}})
	})
	registerMedia() // pictures, voice and video (media.go)
	http.HandleFunc("POST /v1/embeddings", embeddings)
	http.HandleFunc("/mcp", mcp)
	log.Printf("mock LLM on http://localhost%s/v1  ·  mock MCP on http://localhost%s/mcp", addr, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

type msg struct {
	Role    string  `json:"role"`
	Content content `json:"content"`
}

// content accepts both a plain string and OpenAI content parts (text + images).
type content struct {
	text   string
	images int
}

func (c content) String() string { return c.text }

func (c *content) UnmarshalJSON(b []byte) error {
	if json.Unmarshal(b, &c.text) == nil {
		return nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &parts); err != nil {
		return nil // null etc.
	}
	for _, p := range parts {
		switch p.Type {
		case "text":
			c.text += p.Text
		case "image_url":
			c.images++
		}
	}
	return nil
}

func chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []msg `json:"messages"`
		Tools    []struct {
			Function struct{ Name string } `json:"function"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	fl := w.(http.Flusher)
	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
	}
	delta := func(d map[string]any) { send(map[string]any{"choices": []any{map[string]any{"delta": d}}}) }

	var lastUser, lastTool string
	var sawTool bool
	var images int
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			lastUser, sawTool, images = m.Content.text, false, m.Content.images
			if i := strings.Index(lastUser, "]\n"); i > 0 && strings.HasPrefix(lastUser, "[") {
				lastUser = lastUser[i+2:] // Knitnox puts "[date, time]" on the first line of each message
			}
			if i := strings.Index(lastUser, "]: "); i > 0 && i < 66 && strings.HasPrefix(lastUser, "[") && !strings.Contains(lastUser[:i], "\n") {
				lastUser = lastUser[i+3:] // in a group every message starts with "[sender]: "
			}
		case "tool":
			lastTool, sawTool = m.Content.text, true
		}
	}

	if images > 0 && !sawTool && !strings.HasPrefix(req.Messages[0].Content.text, "You check a picture") {
		reply := fmt.Sprintf("I can see **%d image(s)** in your message. (This is the mock server — a real vision model would describe them here.)", images)
		if lastUser != "" {
			reply += "\n\nYou also wrote:\n\n> " + lastUser
		}
		for _, word := range strings.SplitAfter(reply, " ") {
			time.Sleep(40 * time.Millisecond)
			delta(map[string]any{"content": word})
		}
		send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 800 * images, "completion_tokens": len(reply) / 4}})
		fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}

	toolNamed := func(suffix string) string {
		for _, t := range req.Tools {
			if strings.HasSuffix(t.Function.Name, suffix) {
				return t.Function.Name
			}
		}
		return ""
	}
	callTool := func(name string, args any) {
		b, _ := json.Marshal(args)
		delta(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call_%d", time.Now().UnixNano()),
			"type": "function", "function": map[string]any{"name": name, "arguments": string(b)}}}})
		send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 10}})
		fmt.Fprint(w, "data: [DONE]\n\n")
	}
	lower := strings.ToLower(lastUser)
	// The check of a finished picture (Feed): fine, unless the description says "blurry".
	if strings.HasPrefix(req.Messages[0].Content.text, "You check a picture") {
		answer := "OK"
		if strings.Contains(lower, "blurry") {
			answer = "A sharp, clear photo of " + strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(lastUser, "The description: "), "blurry", ""))
		}
		delta(map[string]any{"content": answer})
		send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 900, "completion_tokens": 10}})
		fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}
	// Long-term memory (offered when the user turned it on): "remember ..." saves, "recall ..." searches.
	if save := toolNamed("memory_save"); save != "" && !sawTool && strings.HasPrefix(lower, "remember") {
		fact := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lastUser[len("remember"):]), "that"))
		callTool(save, map[string]any{"content": fact})
		return
	}
	if search := toolNamed("memory_search"); search != "" && !sawTool && strings.HasPrefix(lower, "recall") {
		callTool(search, map[string]any{"query": strings.TrimSpace(lastUser[len("recall"):])})
		return
	}
	// A group this personal chat is connected to: "group recall ..." searches its memory, "group messages" reads its latest messages.
	if search := toolNamed("group_memory_search"); search != "" && !sawTool && strings.HasPrefix(lower, "group recall") {
		callTool(search, map[string]any{"query": strings.TrimSpace(lastUser[len("group recall"):])})
		return
	}
	if read := toolNamed("group_messages"); read != "" && !sawTool && strings.HasPrefix(lower, "group messages") {
		callTool(read, map[string]any{"turns": 5})
		return
	}
	// Plan Book (offered when the user turned it on): "todo ..." adds a task, "plan" lists what is open.
	if add := toolNamed("plan_add"); add != "" && !sawTool && strings.HasPrefix(lower, "todo ") {
		callTool(add, map[string]any{"title": strings.TrimSpace(lastUser[len("todo "):])})
		return
	}
	if list := toolNamed("plan_list"); list != "" && !sawTool && (lower == "plan" || strings.HasPrefix(lower, "plan ")) {
		callTool(list, map[string]any{"view": "open"})
		return
	}

	// Feed (offered when the user let the AI post): "post ..." adds a post with a report and a chart.
	if post := toolNamed("feed_post"); post != "" && !sawTool && strings.HasPrefix(lower, "post ") {
		callTool(post, map[string]any{
			"text": strings.TrimSpace(lastUser[len("post "):]),
			"report": strings.Join([]string{"## Mock report", "", "This report was written by the **mock** model.", "",
				"- It opens behind the words *Read the full report*.", "- Lists, tables and `code` work.", "",
				"| day | visits |", "|---|---|", "| Mon | 12 |", "| Tue | 18 |"}, "\n"),
			"chart": map[string]any{"type": "bar", "title": "Visits this week", "labels": []string{"Mon", "Tue", "Wed", "Thu", "Fri"}, "values": []float64{12, 18, 9, 22, 15}},
		})
		return
	}

	// A picture in the chat itself (offered when a picture model is set up): "paint ...".
	if pic := toolNamed("make_picture"); pic != "" && !sawTool && strings.HasPrefix(lower, "paint ") {
		callTool(pic, map[string]any{"description": strings.TrimSpace(lastUser[len("paint "):])})
		return
	}

	if post := toolNamed("feed_post"); post != "" && !sawTool {
		for word, arg := range map[string]string{"picture ": "image", "say ": "voice", "film ": "video", "draw ": "svg"} {
			if !strings.HasPrefix(lower, word) {
				continue
			}
			rest := strings.TrimSpace(lastUser[len(word):])
			args := map[string]any{"text": rest, arg: rest}
			if arg == "svg" {
				args[arg] = mockDiagram(rest)
			}
			callTool(post, args)
			return
		}
	}

	if strings.HasPrefix(req.Messages[0].Content.text, "You maintain the running memory") {
		delta(map[string]any{"content": "- (mock summary of the earlier conversation)"})
	} else if !sawTool && len(req.Tools) > 0 && strings.Contains(strings.ToLower(lastUser), "time") {
		name := ""
		for _, t := range req.Tools {
			if strings.HasSuffix(t.Function.Name, "clock") {
				name = t.Function.Name
			}
		}
		if name != "" {
			delta(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call_%d", time.Now().UnixNano()),
				"type": "function", "function": map[string]any{"name": name, "arguments": "{}"}}}})
			send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 10}})
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
	}
	// "think" in the message: stream some reasoning first, like DeepSeek-R1 / vLLM do.
	if strings.Contains(strings.ToLower(lastUser), "think") {
		for _, word := range strings.SplitAfter("Let me reason about this step by step. The user wants a thoughtful answer, so I will outline the key points first and then reply.", " ") {
			time.Sleep(40 * time.Millisecond)
			delta(map[string]any{"reasoning_content": word})
		}
	}
	reply := "You said:\n\n> " + lastUser + "\n\nThis is a **mock** reply streamed word by word, so you can watch streaming, queueing and stopping work." +
		"\n\nMarkdown works:\n\n- lists, *emphasis*, `inline code`\n- tables and links\n\n| feature | status |\n|---|---|\n| streaming | ✓ |\n| highlighting | ✓ |\n\n" +
		"```go\n// Code blocks get highlighting and a Copy button.\nfunc greet(name string) string {\n\treturn \"hello, \" + name // a comment\n}\n```"
	if sawTool {
		reply = "The clock tool says it is **" + lastTool + "**."
		if strings.Contains(lastTool, "emor") { // an answer from a memory tool
			reply = "Here is what the memory tool answered:\n\n> " + strings.ReplaceAll(strings.TrimSpace(lastTool), "\n", "\n> ")
		}
		if strings.Contains(lastTool, " of the group ") || strings.Contains(lastTool, "no longer a member") { // an answer from a tool that looks into a group
			reply = "Here is what I found in the group:\n\n> " + strings.ReplaceAll(strings.TrimSpace(lastTool), "\n", "\n> ")
		}
		if strings.HasPrefix(lastTool, "The picture was made") { // an answer from make_picture
			reply = "Here is your picture."
		}
		if strings.HasPrefix(lastTool, "Posted as #") { // an answer from the feed tool
			reply = "Done. " + lastTool + " Open **Feed** in the sidebar to see it."
		}
		if strings.HasPrefix(lastTool, "Added: #") || strings.Contains(lastTool, " in view ") { // an answer from a Plan Book tool
			reply = "Here is what the Plan Book tool answered:\n\n> " + strings.ReplaceAll(strings.TrimSpace(lastTool), "\n", "\n> ")
		}
	}
	for _, word := range strings.SplitAfter(reply, " ") {
		time.Sleep(60 * time.Millisecond)
		delta(map[string]any{"content": word})
	}
	prompt := 0
	for _, m := range req.Messages {
		prompt += len(m.Content.text)/4 + 800*m.Content.images
	}
	send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": prompt, "completion_tokens": len(reply) / 4}})
	fmt.Fprint(w, "data: [DONE]\n\n")
}

// embeddings answers with a small vector per text: each word adds to one of
// 256 buckets, so texts sharing words (or word stems) end up close together.
func embeddings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var texts []string
	if json.Unmarshal(req.Input, &texts) != nil {
		var one string
		json.Unmarshal(req.Input, &one)
		texts = []string{one}
	}
	data := make([]any, 0, len(texts))
	for i, text := range texts {
		vec := make([]float32, 256)
		for _, word := range strings.FieldsFunc(strings.ToLower(text), func(c rune) bool { return c < 'a' || c > 'z' }) {
			if len(word) < 4 {
				continue
			}
			if len(word) > 5 {
				word = word[:5] // crude stemming: "cooking" and "cooked" share a bucket
			}
			h := fnv.New32a()
			h.Write([]byte(word))
			vec[h.Sum32()%256]++
		}
		data = append(data, map[string]any{"object": "embedding", "index": i, "embedding": vec})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data, "model": "mock-embed"})
}

// mcp implements just enough of the Streamable HTTP transport for one tool.
func mcp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct {
		ID     any    `json:"id"`
		Method string `json:"method"`
	}
	json.Unmarshal(body, &req)
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	switch req.Method {
	case "initialize":
		w.Header().Set("Mcp-Session-Id", fmt.Sprint(time.Now().UnixNano()))
		result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{"name": "mock", "version": "1"}}
	case "tools/list":
		result = map[string]any{"tools": []any{map[string]any{"name": "clock", "description": "Returns the current server time.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}}}}
	case "tools/call":
		result = map[string]any{"content": []any{map[string]any{"type": "text", "text": time.Now().Format(time.RFC1123)}}}
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}
