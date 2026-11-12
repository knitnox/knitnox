# Knitnox

A small, self-hosted AI chat with **turn-based group chats**. It's one Go binary with one SQLite file, and the frontend has no build step.

- **Any OpenAI-compatible API** (OpenAI, OpenRouter, Groq, Together, Ollama, LM Studio, vLLM, LiteLLM…). Admins add shared providers, and users can add their own keys.
- **Admin panel**: providers and models, users, per-user token quotas (daily or monthly), usage per user and model with cost, and global settings.
- **Background generation**: replies keep generating on the server after you close the tab, and you see the result when you come back.
- **Message queue**: up to 10 pending messages per chat (configurable), answered in order.
- **Context compaction**: after 50 message pairs, or when the context reaches 70% of the model's window, older turns are summarised. Originals are never deleted.
- **MCP tools** with two modes per server: **Automatic** (tools run immediately) or **Ask before running** (the chat shows an Allow / Deny / Always-allow card and the reply waits for a person). Agent loops have a step limit and retries.
- **Agents**: saved background workers with their own instructions, a chosen set of MCP tools and a to-do list. They run on a schedule or on demand, start fresh every run, and can be connected to a chat so its AI hands them tasks. Ready-made agents are shared in the **agent store**. See `docs/ARCHITECTURE.md` §16.
- **Plan Book**: each user's own tasks and projects, with due dates, a bin and a change history. Once the user allows it, the AI in their chats and chosen agents can add, list and finish tasks. See `docs/ARCHITECTURE.md` §18.
- **Feed**: one page of short posts per user, newest first. The user, the AI in their chats and chosen agents post there, with an optional report, a chart, a diagram the AI draws, or a picture, voice recording or video made by a model of that kind. Posts are private until their owner shares them with everyone on the server; agents have a daily post limit. See `docs/ARCHITECTURE.md` §19.
- **Memory**: opt-in long-term memory per user. The AI saves short facts in one chat and looks them up in another; people, places and projects are linked into a small graph you can browse on a map. Word search works out of the box, an embedding model adds search by meaning. Everything is readable, editable and deletable. See `docs/ARCHITECTURE.md` §17.
- **Group chats**: invite people with a link. Everyone's messages go into one queue and the AI answers them one at a time. Billing and tool access stay safe by default.
- **Images and documents**: attach up to 6 files per message (button, paste or drag-and-drop): PNG/JPEG/WebP/GIF for vision models, plus **PDF, text, Markdown, CSV, JSON and code files**, whose text is extracted and given to the model.
- **Custom instructions**: per user (your solo chats) and per chat ("answer in Bengali", "we are planning a wedding").
- **Thinking models**: `reasoning_content` / `reasoning` streams (DeepSeek, vLLM, OpenRouter…) are shown in a collapsible "Thinking" block.
- **Edit and regenerate**: edit your latest (or any queued) message, regenerate the last answer, or retry a failed one. The old answer is replaced.
- **Find things again**: full-text search across every chat you belong to (SQLite FTS5), "Load earlier messages" for long chats, pin and archive, rename and delete from the sidebar, export as Markdown or JSON.
- **Install it as an app** (PWA) and get **push notifications** when a reply finishes while you're away or a tool needs your approval.
- **Operations**: a global cap on simultaneous generations (protects your provider from 429s, honours `Retry-After`), login brute-force protection, an audit log of admin actions, a context-window meter in every chat, and a `knitnox backup` command.
- **Markdown + code highlighting**: GitHub-style Markdown (tables, task lists, quotes), monochrome syntax highlighting for 30+ common languages, and a Copy button on every code block.
- **A calm monochrome UI** (neutral greys, dark mode follows the OS) in an app shell like the familiar chat apps: a sidebar with pinned and date-grouped chats, a "⋯" menu on every chat, keyboard shortcuts, a full-screen image viewer and copy buttons on messages. It is built for phones too, with a slide-out drawer, bottom-sheet menus, safe-area handling and no iOS zoom on inputs. htmx powers the forms and about 700 lines of vanilla JS run the chat. The CSP allows no inline scripts or styles at all.

- **Help built in**: a user guide, a feature list and a developer guide under **Help**, plus an **Issues** page where anyone can report a bug, idea or question and admins reply.

**Guides:** [User guide](docs/USER_GUIDE.md) · [Features](docs/FEATURES.md) · [Developer guide](docs/DEVELOPER_GUIDE.md) · [Build guide](docs/BUILD_GUIDE.md) · [Architecture](docs/ARCHITECTURE.md). All but the last are also shown inside the app under **Help**.

---

## Quick start (no API key needed)

Requires **Go 1.26+**. CGO is not needed: SQLite is the pure-Go `modernc.org/sqlite`.

```bash
# terminal 1: a fake OpenAI-compatible API + fake MCP server for development
go run ./cmd/mockllm                # http://localhost:9090

# terminal 2: Knitnox
go run ./cmd/knitnox                 # http://localhost:8080
```

1. Open http://localhost:8080. The **first account you create becomes the admin**.
2. **Admin → Providers → Add provider**
   - Name: `Mock`, Base URL: `http://localhost:9090/v1`, API key: anything. Then click **Fetch models from provider** and tick `mock` (or type `mock` in the Models box).
3. **Settings → MCP servers → Add**
   - Name `clock`, URL `http://localhost:9090/mcp`, mode **Ask me before running**, then click **Test**.
4. Go to **Chats**, type a message and press Enter.
   - "what time is it?" → the AI asks to run the clock tool; click **Allow once**, **Deny** or **Always allow**.
   - "please think about cats" → a streamed **Thinking** block appears above the answer.
   - attach a PDF or `.txt` → the mock echoes the extracted text back.

To use a real provider, add e.g. `https://api.openai.com/v1` with your key and model `gpt-4o-mini`.
For Ollama: `http://localhost:11434/v1`, any key, model `llama3.2`.

---

## Configuration

All configuration is through environment variables. Everything else is set in the Admin UI and stored in SQLite.

| Variable | Default | Meaning |
|---|---|---|
| `KNITNOX_ADDR` | `:8080` | Listen address |
| `KNITNOX_DATA` | `./data` | Directory for `knitnox.db` (+ WAL files) and `secret.key` |
| `KNITNOX_SECRET` | *(generated)* | Passphrase that encrypts API keys and MCP headers at rest. If unset, a random key is written to `$KNITNOX_DATA/secret.key` |
| `KNITNOX_SECURE_COOKIES` | off | Set to `1` when served over HTTPS |
| `KNITNOX_LOG` | `info` | `debug`, `info` or `warn` |

PowerShell: `$env:KNITNOX_ADDR=":3000"; go run ./cmd/knitnox`
Bash: `KNITNOX_ADDR=:3000 go run ./cmd/knitnox`

**Admin → Settings** (stored in the DB, take effect on the next message):

| Setting | Default | |
|---|---|---|
| Allow public sign-up | on | Off means only admins create accounts |
| Queue limit per chat | 10 | Pending messages per chat |
| Agent max steps per turn | 10 | LLM ↔ tool round trips per turn. At the limit, one final answer is forced without tools |
| Tool call retries | 2 | For transport failures of a single MCP call |
| LLM call retries | 2 | 429/5xx/network errors, only before the first token arrives |
| Compact after N pairs | 50 | A pair is one user message plus its answer. Tool steps are not counted |
| Keep last N pairs verbatim | 10 | Never summarised |
| Context ratio | 0.7 | Also compact when estimated tokens exceed ratio × model context |
| Compaction model | same as chat | Pick a cheap shared model to save cost |
| System prompt | … | Prepended to every chat (followed by the user's and the chat's custom instructions) |
| Max replies generating at once | 8 | Across all chats. Extra runs wait for a free slot ("Waiting for a free slot…"). `0` = unlimited |
| Tool approval timeout | 30 min | An "ask first" tool call nobody answers is treated as denied after this |

Each user also has, under **Settings**: custom instructions, API keys, MCP servers (with the Automatic / Ask-first switch), notifications, and "Log out everywhere".

---

## Project layout

```
cmd/
  knitnox/        main: config, startup, graceful shutdown, CLI commands
  mockllm/       dev-only fake OpenAI + MCP server
internal/
  store/         ALL SQL lives here (+ migrations/*.sql, embedded)
  secret/        AES-256-GCM for API keys / MCP headers
  llm/           streaming client for /chat/completions (tool calls, reasoning, Retry-After)
  mcp/           MCP client (Streamable HTTP transport)
  files/         upload classification + text extraction (PDF, text)
  hub/           in-memory pub/sub per thread (feeds SSE)
  engine/        thread workers, queue, slots, agent loop, tool approval, compaction, quotas
  web/           router, auth/CSRF/login limiter, handlers, push, templates/, static/ (all embedded)
docs/ARCHITECTURE.md
```

Dependencies only point downward: `web → engine → (llm, mcp, hub, store) → secret`, and `web → files → store`. Only `store` touches the database.

---

## Development

```bash
go test ./...              # engine end-to-end tests with fake LLM + MCP servers
go vet ./...
go build -o knitnox ./cmd/knitnox
```

- Templates, CSS and JS are embedded with `go:embed`, so **restart the server after editing them** (`go run` rebuilds).
- `go test -race ./...` needs CGO (`CGO_ENABLED=1` and a C compiler). Run it in CI on Linux.

---

## Building for Windows, Linux and macOS

Knitnox is one static binary with everything (templates, CSS, JS, fonts, migrations) embedded. There is **no C code**, so you can build for any OS from any OS — no cross-compilers, no Docker, nothing to install besides Go 1.26+.

**Build for the machine you're on:**

```bash
go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox        # Linux / macOS
go build -trimpath -ldflags="-s -w" -o knitnox.exe ./cmd/knitnox    # Windows
```

(`-s -w` strips debug symbols: ~15 MB instead of ~25 MB. `-trimpath` removes your local paths from the binary.)

**Cross-compile** by setting two environment variables:

| Target | Bash / zsh | PowerShell |
|---|---|---|
| Linux x86-64 (most servers, Docker) | `GOOS=linux GOARCH=amd64 go build -o knitnox ./cmd/knitnox` | `$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o knitnox ./cmd/knitnox` |
| Linux ARM64 (Raspberry Pi 4/5, AWS Graviton, Oracle free tier) | `GOOS=linux GOARCH=arm64 …` | `$env:GOOS="linux"; $env:GOARCH="arm64"; …` |
| macOS Apple Silicon (M1–M4) | `GOOS=darwin GOARCH=arm64 …` | `$env:GOOS="darwin"; $env:GOARCH="arm64"; …` |
| macOS Intel | `GOOS=darwin GOARCH=amd64 …` | `$env:GOOS="darwin"; $env:GOARCH="amd64"; …` |
| Windows x86-64 | `GOOS=windows GOARCH=amd64 go build -o knitnox.exe ./cmd/knitnox` | `$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o knitnox.exe ./cmd/knitnox` |
| Windows ARM64 (Surface Pro X, Snapdragon laptops) | `GOOS=windows GOARCH=arm64 …` | `$env:GOOS="windows"; $env:GOARCH="arm64"; …` |

In PowerShell, clear the variables afterwards (`Remove-Item Env:GOOS, Env:GOARCH`) or later builds will keep targeting that OS. `CGO_ENABLED=0` is the default when cross-compiling, so you don't need to set it.

**All six at once**, zipped with checksums into `dist/`:

```bash
scripts/build-all.sh 1.0.0          # Linux / macOS / Git Bash
.\scripts\build-all.ps1 -Version 1.0.0   # Windows PowerShell
```

Produces `dist/knitnox-1.0.0-<os>-<arch>.tar.gz` (`.zip` for Windows) and `dist/SHA256SUMS.txt`.

**Running the binary on the target machine:**

```bash
# Linux / macOS
chmod +x knitnox
./knitnox create-admin alice 'a-strong-password'   # optional; the first sign-up is admin anyway
./knitnox                                          # listens on :8080, data in ./data

# Windows (PowerShell)
.\knitnox.exe
```

On macOS, a binary you built yourself runs without complaint; one downloaded from the internet is quarantined by Gatekeeper and needs `xattr -d com.apple.quarantine ./knitnox` once (or code-signing/notarisation if you distribute it).

Notes:
- `go test -race` and the `go:embed` of fonts work the same on every OS. Windows builds also run fine under WSL.
- `mockllm` is built the same way (`./cmd/mockllm`) if you want the fake provider on a server.
- The `Dockerfile` does a Linux amd64 build inside the image; for an ARM server use `docker build --platform linux/arm64 …`.

Tests (all with in-process fake LLM / MCP servers, no network):
- `internal/engine/engine_test.go`: a basic turn and usage, queue order and limit, the tool loop through MCP (JSON and SSE responses), group-chat name prefixes and tool hiding, compaction, quota blocking, stopping mid-stream, editing and regenerating, images as content parts.
- `internal/engine/features_test.go`: tool approval (allow, deny, timeout, stop while waiting), the parallel-run cap, custom instructions (and that personal ones stay out of group chats), inline file text, reasoning capture, search and paging.
- `internal/llm/llm_test.go`: `Retry-After` handling.
- `internal/web/web_test.go`: HTTP-level flows: CSRF, login lockout, uploads and their validation, permissions for non-members, rename/pin/archive/export/search, tool approval over HTTP, paging, admin settings and the audit log, PWA files.

There is also a curl smoke-test flow in the session notes; the mock server exercises every feature without an API key.

---

## Deployment

**Binary + systemd (Linux):**

```ini
# /etc/systemd/system/knitnox.service
[Service]
Environment=KNITNOX_ADDR=127.0.0.1:8080
Environment=KNITNOX_DATA=/var/lib/knitnox
Environment=KNITNOX_SECRET=change-me-to-a-long-random-string
Environment=KNITNOX_SECURE_COOKIES=1
ExecStart=/usr/local/bin/knitnox
Restart=on-failure
User=knitnox
[Install]
WantedBy=multi-user.target
```

**Reverse proxy:** SSE needs buffering off. Knitnox sends `X-Accel-Buffering: no`, which nginx respects. Also allow long reads:

```nginx
location / {
  proxy_pass http://127.0.0.1:8080;
  proxy_set_header Host $host;
  proxy_set_header X-Forwarded-Proto $scheme;
  proxy_read_timeout 1h;
  proxy_buffering off;
}
```

Caddy works with no special config: `reverse_proxy 127.0.0.1:8080`.

**HTTPS matters more now:** installing Knitnox as an app and push notifications only work on `https://` (or `localhost`). The login limiter trusts `X-Forwarded-For` only when the connection comes from loopback, i.e. from a proxy on the same machine.

**Docker:**

```bash
docker build -t knitnox .
docker run -p 8080:8080 -v knitnox-data:/data -e KNITNOX_SECRET=... knitnox
```

Run **one instance only**. Workers and the event hub live in process memory (see Architecture → Scaling).

---

## Maintenance

### Backups
One command, safe to run while the server is up:
```bash
knitnox backup /backups            # → /backups/knitnox-20261109-150405/{knitnox.db, secret.key, uploads/}
```
It uses `VACUUM INTO` for a consistent database copy (plain `cp` of `knitnox.db` can miss data still in `knitnox.db-wal`), copies `secret.key` if one exists, and copies the `uploads/` folder. Put it in cron and rotate old folders. The command reads the same `KNITNOX_DATA` / `KNITNOX_SECRET` as the server.

What the backup contains and why:
1. **The database**: everything except files.
2. **`secret.key`, or your `KNITNOX_SECRET`.** Without it, stored API keys and MCP headers cannot be decrypted. Users would have to re-enter them; chats are unaffected. The VAPID keys for push notifications are in the database.
3. **`uploads/`**: images and documents (random file names, referenced from the `attachments` table).

To restore: stop the server, put `knitnox.db`, `secret.key` and `uploads/` back into `KNITNOX_DATA`, start it.

### Locked out / forgot the admin password
```bash
knitnox reset-password admin 'new-password'   # also re-enables the account
knitnox create-admin alice 'password123'
```
These commands use the same `KNITNOX_DATA` / `KNITNOX_SECRET` environment as the server.

### Upgrading from Parley (the old name)
The app was renamed from Parley to Knitnox. Existing installs keep working without changes:
- `PARLEY_*` environment variables are still read when the matching `KNITNOX_*` one isn't set (switch when convenient).
- On first start, `DATA/parley.db` (and its `-wal`/`-shm` files) is renamed to `knitnox.db`. `secret.key` and `uploads/` are unchanged.
- Existing logins (the old `parley_session` cookie) stay valid.
- The binary is now `knitnox` / `knitnox.exe` and the systemd unit, Docker image and backup folders use the new name; update your scripts.

### Upgrades and migrations
Migrations in `internal/store/migrations/NNN_name.sql` run automatically at startup, in order, each in a transaction, and are recorded in `schema_migrations`. **Never edit a migration that has shipped. Add a new file.** Back up before upgrading.

### Things that grow
- `usage_log` gets one row per LLM call. Archive old rows if needed:
  `DELETE FROM usage_log WHERE created_at < '2026-01-01';`
- `messages` grows forever (compaction never deletes). Deleting a chat removes its messages.
- `uploads/` grows with images and documents. Deleting a chat deletes its files, and an hourly sweep removes files no row references any more (from deleted users, or queued messages that were cancelled). Uploads are capped at 6 files per message, 10 MB per image and 20 MB per document; browsers resize photos to ≤2048 px first, and extracted document text is capped at 120k characters per file.
- `messages_fts` (the search index) is roughly the size of the message text. It is maintained by triggers; rebuild it after a manual bulk import with `INSERT INTO messages_fts(messages_fts) VALUES('rebuild');`.
- `audit_log` gets one row per admin action; `tool_approvals` one per asked tool call. Both are small.
- Expired sessions are purged hourly.

### Restarts
A reply that is generating during a restart is marked **interrupted**, and its partial text is kept. Queued messages **resume automatically** on startup.

### Logs
Logs are structured text on stderr (`log/slog`). Worth watching: `run failed`, `mcp connect failed`, `retrying LLM call`, `thread compacted`.

### Updating vendored JS and fonts
`internal/web/static/vendor/` holds htmx 2.0.4, marked 15.0.7, DOMPurify 3.2.4 and highlight.js 11.11.1 (common-languages build). Replace the files to upgrade, and keep DOMPurify current because it is the XSS barrier for model output.
`internal/web/static/fonts/` holds Inter and JetBrains Mono (variable, Latin subset, SIL Open Font License) from `@fontsource-variable`.

### Changing the look
All colours, radii and shadows are CSS variables at the top of `internal/web/static/style.css`, with a dark-mode block right below (`--content-w` sets the chat column width, 920 px by default). Icons are inline SVG templates in `internal/web/templates/icons.html`; use one with `{{template "i-name"}}`. The app icon is `static/icons/favicon.svg` (a copy of `favicon.svg` in the repo root); it is the favicon, the logo in the UI (`.logo` in `style.css`) and the source of the PNGs for home screens and notifications (`icon-192/512`, `icon-512-maskable`, `apple-touch-icon`, `favicon-32`). To change the brand icon, replace that SVG and re-export the PNGs at those sizes. Static URLs are versioned automatically (`style.css?v=<build hash>`), so edited CSS/JS shows up on the next page load, including in installed apps.

---

## Extending: common recipes

### Add an admin setting
1. Add a field and default in `store/settings.go` (`Settings`, `DefaultSettings`, `LoadSettings`, `SaveSettings`).
2. Add an input in `templates/admin.html` (settings tab) and parse it in `web/admin.go → adminSettingsSave`.
3. Read it in the engine through `tc.settings` (loaded once per run).

### Add a column or table
1. Create `internal/store/migrations/002_whatever.sql` (`ALTER TABLE …` / `CREATE TABLE …`).
2. Add the query functions in the relevant `store/*.go` file. Keep SQL out of other packages.

### Add a page or endpoint
1. Register the route in `web/server.go → Handler()` with `u(...)` (signed in) or `a(...)` (admin). CSRF is enforced on every non-GET automatically.
2. For htmx fragments, put a `{{define "name"}}` in the page template and return it with `s.renderPartial(w, r, page, "name", data)`. Return `HX-Retarget: #flash` for errors (`s.flash`).
3. New page template? Add its name to the `pages` list in `parseTemplates`.

### Push a new live event to browsers
1. Add a constant in `engine/engine.go` (`Ev...`) and call `e.hub.Publish(threadID, EvX, data)`.
2. Handle it in `static/app.js` in the `es.onmessage` switch. When in doubt, call `refresh()`, which re-fetches the message list.

### Support a non-OpenAI-compatible API (e.g. a native Anthropic or Gemini endpoint)
Everything above `llm` speaks `llm.Request` / `llm.Result`. Add a `kind` column to `providers`, write `llm/anthropic.go` exposing the same `Stream(ctx, req, onDelta)` signature, and switch on the kind in `engine/run.go → streamStep` and `engine/compact.go`. Many vendors also offer OpenAI-compatible endpoints, which work today without changes.

### Change who pays
All billing decisions are in `engine/run.go → prepare()` (key usage rules) and `recordUsage()` (which user is charged). Quota checks are in `checkQuota()`.

### Change compaction
`engine/compact.go`. The trigger rules are at the top of `maybeCompact`, followed by the prompt and the summary write. The summary is stored on `threads.summary`, and `threads.compacted_until_run` marks the boundary.

### Change what needs approval
Approval is decided per server (`mcp_servers.approval`) in `engine/tools.go → openTools` (`route.ask`) and waited for in `awaitApproval`. To approve per *tool* instead of per server, store a set of tool names on the server row and set `ask` only for those. Who may decide is checked in `web/chat.go → decideApproval` (run author or chat owner).

### Support another file type
`internal/files/files.go → Classify` sniffs the bytes and returns a kind, MIME type and extracted text. Add a case there (e.g. DOCX → unzip `word/document.xml` and strip tags) and the file's extension to `store.AllowedFileTypes`. Everything downstream only sees text.

### Allow stdio MCP servers (admin-only)
Deliberately not implemented, because a user-supplied command means remote code execution on the host. If you need it, add an admin-managed table and a stdio transport in `mcp/` (JSON-RPC over the process's stdin/stdout). Never let regular users configure the command.

---

## Security notes

- **Passwords**: bcrypt. **Sessions**: random 256-bit tokens, only the SHA-256 is stored, HttpOnly + SameSite=Lax cookies.
- **CSRF**: per-session token, required on every non-GET (`X-CSRF-Token` header or `_csrf` field).
- **XSS**: templates auto-escape. Model output is rendered as Markdown and passed through DOMPurify (with `style` attributes stripped). The CSP is `default-src 'self'` with **no `unsafe-inline`** for scripts or styles, plus `frame-ancestors 'none'`, `base-uri 'self'` and `form-action 'self'`.
- **Uploads**: types are decided by sniffing the bytes, never by the file name. Documents are served as downloads (`Content-Disposition: attachment`), and HTML/XML uploads are served as plain text, so an uploaded page can never run in Knitnox's origin.
- **Login**: 10 failed attempts for a username, or 30 from one IP, within 15 minutes lock that username / IP out for 15 minutes. A successful login clears the counters. Admin password resets log the user out everywhere.
- **Tool approval**: "Ask before running" is the safe default for anything with side effects. Only the chat owner or the person whose message triggered the call can decide, and "Always allow" (which switches the server to automatic) is owner-only.
- **API keys / MCP headers**: AES-256-GCM at rest and never sent back to the browser (shown as `sk-…abcd`).
- **Group chats**: members never get the owner's MCP tools or personal API key unless the owner opts in per chat. Personal custom instructions are never applied in group chats.
- **SSRF**: users can make the server call any URL they enter (provider base URL, MCP URL). On a multi-tenant deployment, put Knitnox in a network segment without access to internal services or cloud metadata endpoints, or add an allow/deny list in `web/settings.go → validURL`.
- **Audit**: every admin action (providers, models, users, quotas, password resets, settings) is recorded with who did it, under Admin → Audit.

## Known limitations

- Single instance (in-memory workers, hub, slots and pending approvals).
- MCP: Streamable HTTP transport only. The legacy HTTP+SSE transport (`GET /sse`), OAuth flows and stdio are not supported. Static auth headers work.
- Files: images, PDF (text layer only, no OCR) and text-like files. Images are sent for the current turn and the 3 before it, documents for the current turn and the 6 before it; older ones become a short note to save tokens, so the summary has to carry what matters. Word/Excel/PowerPoint are not parsed.
- Search matches the beginnings of words (prefix queries) in message text, not inside attached files.
- A tool call waiting for approval blocks that chat's queue until someone decides or the timeout passes.
- The sidebar doesn't live-update when someone invites you; reload to see new chats.
