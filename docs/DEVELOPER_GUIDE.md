# Developer guide

How to run Knitnox on your machine, find your way around the code, and change it safely. For the deeper "why", read `docs/ARCHITECTURE.md` in the repository.

## Run it in two minutes

You need **Go 1.26 or newer**. Nothing else: no Node, no C compiler, no database server.

```bash
# terminal 1: a fake AI provider and a fake MCP server, so you need no API key
go run ./cmd/mockllm          # http://localhost:9090

# terminal 2: Knitnox
go run ./cmd/knitnox          # http://localhost:8080
```

1. Open `http://localhost:8080` and create an account. **The first account is the admin.**
2. Go to **Admin panel → Providers → Add a provider**. Base URL `http://localhost:9090/v1`, any key, model `mock`.
3. Go to **Settings → MCP servers** and add `http://localhost:9090/mcp`.
4. Send a message. Try "what time is it?" to see a tool call.
5. To try the Plan Book: turn it on under **Plan Book**, then send "todo buy milk" and "plan".
   To try the Feed: open **Feed**, the settings button, **Let the AI post**, then send "post the weekly numbers are in" or "draw how a report gets made".
   For pictures, voice and video add the models `mock-image`, `mock-voice` and `mock-video` to the mock provider (their kind is guessed from the name), then send "picture a loaf of bread", "say good morning" or "film a bakery counter".
6. To try memory: turn it on under **Memory**, then send "remember that I like tea" and "recall tea". For smart search, set the embedding model `mock-embed` under **Admin panel → Settings**.

Data lives in `./data` (the database, the secret key and uploads). Delete that folder to start clean.

## The one idea to understand

> **The server owns the work, not the browser.**

Sending a message only stores it and puts it in a queue. A worker on the server does the AI call and publishes progress. Browsers just watch. Background answers, the queue, group chats and agents all follow from this.

```
browser ──POST message──► store (run = pending) ──► worker for that chat
                                                      │  LLM ⇄ tools loop
browser ◄──── live events (SSE) ◄──── hub ◄───────────┘
```

## Where things live

```
cmd/knitnox/     start-up, configuration, CLI commands
cmd/mockllm/     fake AI + MCP server for development
internal/
  store/         every SQL statement, and migrations/*.sql
  engine/        workers, queue, agent loop, tools, compaction, agents, scheduler
  llm/           streaming client for /chat/completions
  mcp/           MCP client (HTTP)
  hub/           in-memory live events per chat
  files/         upload checks and text extraction
  secret/        encryption for stored keys
  web/           routes, handlers, templates/, static/
docs/            these guides (shown in the app under Help)
```

Dependencies point one way: `web → engine → (llm, mcp, hub, store) → secret`.

## "I want to change…" lookup

| I want to change… | Look here |
|---|---|
| A page's HTML | `internal/web/templates/` |
| Colours, spacing, dark mode | The variables at the top of `internal/web/static/style.css`. Use the tokens (`--accent`, `--bg`, `--danger-soft`…), never a raw colour. The dark values exist twice; change both |
| An accent colour choice | `style.css` (`data-accent` rules and `.sw-…` swatch), `accents` in `web/server.go`, the list in `templates/settings.html` |
| What `Ctrl+K` offers | `templates/partials.html → palette` |
| Browser behaviour | `internal/web/static/app.js` |
| A URL or who may call it | `internal/web/server.go → Handler()` |
| What the AI is told | `engine/run.go → SystemPrompt` (chats), `engine/agents.go → AgentSystemPrompt` (agents) |
| The date and time the AI is told | `engine/clock.go` (a time stamp on each user message; nothing in the system prompt, to keep prompt caching working) |
| The LLM ⇄ tool loop | `engine/run.go → execute` |
| Which tools a run gets | `engine/tools.go → openTools` |
| Built-in agent tools | `engine/agents.go → addAgentTools` and `addManagerTools` |
| When agents run | `engine/agents.go → RunDueAgents` and `triggerAgent` |
| Who pays, quotas | `engine/run.go → prepare`, `checkQuota`, `recordUsage` |
| Summarising long chats | `engine/compact.go` |
| Plan Book: tables and queries | `store/planbook.go`, `migrations/009_planbook.sql` |
| Plan Book: the AI's tools | `engine/planbook.go` |
| The Plan Book page | `web/planbook.go`, `templates/planbook.html` |
| Feed: tables and queries | `store/feed.go`, `migrations/010_feed.sql` |
| Feed: the tools for the AI and agents | `engine/feed.go` |
| The Feed page, charts, live updates | `web/feed.go`, `templates/feed.html`, the Feed part of `static/app.js` |
| Feed settings, model choices, "add my own model", serving media | `web/feedmedia.go` |
| Making pictures, voice and video; the picture check | `engine/feedmedia.go`, `llm/media.go`, `store/feedmedia.go`, `migrations/011_media.sql` |
| What a model is for (chat, embedding, image, voice, video) | `store/providers.go` (`models.kind`, `GuessModelKind`, `KindModelOptions`) |
| Long-term memory: tools, search, indexing | `engine/memory.go` |
| Long-term memory: tables and queries | `store/memory.go`, `migrations/007_memory.sql` |
| The Memory page and its map | `web/memory.go`, `templates/memory.html`, `static/memory.js` |
| The embeddings call | `llm/embed.go` |
| An admin setting | `store/settings.go`, then `templates/admin.html` and `web/admin.go` |
| A table or column | A new file in `internal/store/migrations/`, then `store/*.go` |
| The guides | The Markdown files in `docs/` |

## Five rules that keep the code healthy

1. **SQL lives only in `store`.** Other packages call store functions.
2. **Never edit a migration that has shipped.** Add a new numbered file. Migrations run by themselves at start-up.
3. **No inline scripts or styles.** The browser policy forbids them. Put CSS in `style.css` and JS in `app.js`.
4. **Check ownership on every handler.** Use `loadThread`, `loadOwnedThread` or `loadAgent`; they return "not found" for other people's data.
5. **Restart after editing templates, CSS or JS.** They are embedded in the binary; `go run` rebuilds it.

## Recipes

### Add a page

1. Create `internal/web/templates/mypage.html` with `{{define "title"}}` and `{{define "content"}}`.
2. Add `"mypage"` to the `pages` list in `web/server.go → parseTemplates`.
3. Write a handler that ends with `s.render(w, r, "mypage", data)`.
4. Register it in `Handler()` with `u("GET /mypage", s.myPage)`. Use `a(...)` for admin-only pages.

Every request that is not a GET must carry the CSRF token. Forms rendered by htmx send it by themselves; plain forms need `<input type="hidden" name="_csrf" value="{{.CSRF}}">`.

### Update part of a page without a reload

1. Put the fragment in a `{{define "name"}}` block.
2. In the form or button, set `hx-post="/url"` and `hx-target="#id"`.
3. In the handler, return the fragment with `s.renderPartial(w, r, page, "name", data)`.
4. For an error message, call `s.flash(w, r, "text")`.

### Add a table

1. Create `internal/store/migrations/00N_name.sql`.
2. Add a Go file in `internal/store/` with a struct and the query functions.
3. Call those functions from `engine` or `web`.

### Add a built-in tool for agents

Built-in tools run inside the engine and need no MCP server.

```go
ts.addLocal("tool_name", "What it does, for the model.",
    `{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`,
    func(ctx context.Context, raw json.RawMessage) string {
        var in struct{ Text string }
        if msg := parseArgs(raw, &in); msg != "" {
            return msg
        }
        return "result as text"
    })
```

Add it in `addAgentTools` (for the agent itself), `addManagerTools` (for a chat that manages agents) `addMemoryTools` (long-term memory) or `addPlanTools` (Plan Book). Keep answers small: return snippets and ids, and let the model ask for more. Always return text; on failure return a sentence starting with `Error:` so the model can react.

### Send a new live event to browsers

1. Add an `Ev…` constant in `engine/engine.go` and call `e.hub.Publish(threadID, EvX, data)`.
2. Handle it in `app.js`, in the `es.onmessage` switch. When unsure, call `refresh()`.

### Add or edit a help guide

Guides are plain Markdown in `docs/`. To add one, list the file in `docs/docs.go` and add a line to `helpDocs` in `internal/web/help.go`. Each `##` heading becomes an entry in the page's "On this page" list.

## Tests

```bash
go test ./...        # everything; uses in-process fake AI and MCP servers, no network
go vet ./...
gofmt -l .           # should print nothing
```

| File | What it covers |
|---|---|
| `internal/engine/engine_test.go` | Turns, queue, tools, group chats, compaction, quotas, editing |
| `internal/engine/features_test.go` | Tool approval, parallel cap, instructions, files, search |
| `internal/engine/agents_test.go` | Agent runs, tasks, scheduler, tool allowlist, private servers |
| `internal/engine/planbook_test.go` | Plan Book tools, the bin, who is offered the Plan Book, agent access levels |
| `internal/engine/feed_test.go` | Feed tools, the agent daily limit, who is offered the feed, that shared posts of others never reach the model |
| `internal/engine/clock_test.go` | Time stamps on messages, and that the prompt prefix stays the same between turns |
| `internal/engine/memory_test.go` | Memory tools, per-user isolation, smart search, who is offered memory |
| `internal/web/web_test.go` | Login, CSRF, uploads, permissions, admin |
| `internal/web/agents_test.go` | Agent pages, the store, connecting agents |
| `internal/web/planbook_test.go` | Plan Book page, permissions, projects, bin |
| `internal/web/feed_test.go` | Feed page, privacy between users, sharing, read counts, paging, bin, charts and pictures |
| `internal/web/feedmedia_test.go` | Feed settings, model kinds, adding an own model in Feed and Memory settings, who may load media, picture sizes |
| `internal/engine/feedmedia_test.go` | Pictures, voice and video against a fake provider: the redraw after a check, failures, the daily limit, own models |
| `internal/web/memory_test.go` | Memory pages, permissions, search settings, agent access |
| `internal/web/help_test.go` | The guides and the issue tracker |

Write a test for every permission rule you add: one user must not be able to read or change another user's data.

## Checking how a page looks

Templates fail quietly: a mistake is logged, not shown. After changing a template:

1. Run the server and open the page.
2. Watch the terminal for a line containing `render`.

## Build and deploy

```bash
go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox
```

The result is one file with everything inside. Configuration is five environment variables:

| Variable | Default | Meaning |
|---|---|---|
| `KNITNOX_ADDR` | `:8080` | Address to listen on |
| `KNITNOX_DATA` | `./data` | Folder for the database, key and uploads |
| `KNITNOX_SECRET` | generated | Passphrase that encrypts stored keys |
| `KNITNOX_SECURE_COOKIES` | off | Set to `1` behind HTTPS |
| `KNITNOX_LOG` | `info` | `debug`, `info` or `warn` |

Run **one instance only**. The [Build guide](/help/build) has step-by-step builds for Windows, Linux and macOS, and cross-compilation. The README covers systemd, the reverse proxy and backups.

## Reporting and fixing issues

Users report problems under **Help → Issues**. Admins can reply and mark an issue done. When you fix something, mark the issue done and say in the reply which change fixed it.
