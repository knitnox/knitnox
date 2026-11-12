<div align="center">

<img src="favicon.svg" width="96" height="96" alt="Knitnox logo">

# Knitnox

**A calm, self-hosted AI chat you can share with people.**
One Go binary · one SQLite file · no build step · any OpenAI-compatible model.

[![test](https://github.com/knitnox/knitnox/actions/workflows/test.yml/badge.svg)](https://github.com/knitnox/knitnox/actions/workflows/test.yml)
![Go 1.26+](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-no%20CGO-003B57?logo=sqlite&logoColor=white)
![Platforms](https://img.shields.io/badge/runs%20on-Linux%20·%20macOS%20·%20Windows-6b5cd6)

[**Website**](https://knitnox.github.io/) ·
[Quick start](#-quick-start-no-api-key-needed) ·
[Features](#-what-you-get) ·
[User guide](docs/USER_GUIDE.md) ·
[Developer guide](docs/DEVELOPER_GUIDE.md)

</div>

---

## 💡 What is it?

Knitnox is a private ChatGPT-style app that you run on **your own machine or server**. You connect it to any AI provider (OpenAI, OpenRouter, Groq, Ollama, LM Studio…) and invite your family, friends or team.

Its special trick is **turn-based group chats**: several people talk to the same AI in one chat, and their messages are answered **one at a time, in order**, so nobody talks over anybody.

```text
 You ──┐
 Sam ──┼──►  one shared queue  ──►  AI answers each in turn  ──►  everyone sees it live
 Ana ──┘
```

Download one file, run it, open your browser. No Docker, no Node, no database server required.

---

## ✨ What you get

### 💬 Chatting
| | |
|---|---|
| **Group chats** | Invite people with a link. One queue, answered in order. Billing and tool access stay safe by default. |
| **Keeps working when you leave** | Replies keep generating on the server after you close the tab. Up to 10 messages can wait in line per chat. |
| **Files and pictures** | Attach up to 6 files (click, paste or drag): images for vision models, plus PDF, text, Markdown, CSV, JSON and code. |
| **Thinking models** | The reasoning of DeepSeek, vLLM and others shows in a collapsible *Thinking* block. |
| **Edit, regenerate, retry** | Fix your last message, get another answer, or retry a failed one. |
| **Pictures and voice** | With a picture model, the AI can draw in the chat. With a voice model, every answer gets a **Read aloud** button. |
| **Long chats stay cheap** | Older turns are summarised automatically. Originals are never deleted. |

### 🧠 An AI that helps beyond the chat
| | |
|---|---|
| **Tools (MCP)** | Connect tool servers. Per server choose **Automatic** or **Ask before running** (Allow / Deny / Always allow). |
| **Agents** | Background helpers with their own instructions, tools and to-do list. Run on a schedule or on demand, or let a chat hand them tasks. Ready-made ones in the **agent store**. |
| **Memory** | Opt-in long-term memory. The AI remembers facts across chats and links people, places and projects on a map. You can read, edit and delete everything. |
| **Plan Book** | Your own tasks and projects with due dates. Allowed AI chats and agents can add and finish tasks for you. |
| **Feed** | A page of short posts by you, your chats and your agents, with reports, charts, diagrams, pictures, voice or video. Private until you share them. |
| **Tools menu per chat** | Switch any of the above on or off for each chat. |

### 🛠️ For the person who runs it
| | |
|---|---|
| **Admin panel** | Providers and models, users, daily or monthly token quotas, usage and cost per user and model. |
| **Bring your own key** | Admins share providers; users can also add their own. Keys are encrypted at rest. |
| **Built to stay up** | A cap on simultaneous replies, time limits for silent providers, checked automatic backups, hourly cleanup, login protection and an audit log. |
| **Install as an app** | A PWA with push notifications when a reply finishes or a tool needs your OK. |

### 🎨 A calm interface
Warm neutrals, one accent colour of six, light or dark. Works well on phones (slide-out drawer, bottom-sheet menus). `Ctrl+K` search and jump, full-text search across every chat, Markdown with code highlighting, export to Markdown or JSON. A strict security policy with no inline scripts at all.

> 📖 Help is built in: the user guide, feature list and developer guide are inside the app under **Help**, plus an **Issues** page for bugs and ideas.

---

## 🚀 Quick start (no API key needed)

You need **Go 1.26+**. No C compiler is needed: SQLite is the pure-Go `modernc.org/sqlite`.

```bash
# terminal 1: a fake AI provider + fake tool server, for trying things out
go run ./cmd/mockllm                 # http://localhost:9090

# terminal 2: Knitnox
go run ./cmd/knitnox                 # http://localhost:8080
```

1. Open **http://localhost:8080** and create an account. **The first account becomes the admin.**
2. **Admin → Providers → Add provider**: name `Mock`, base URL `http://localhost:9090/v1`, any API key. Click **Fetch models from provider** and tick `mock`.
3. **Settings → MCP servers → Add**: name `clock`, URL `http://localhost:9090/mcp`, mode **Ask me before running**, then **Test**.
4. Go to **Chats** and try:
   - *"what time is it?"* → the AI asks to use the clock tool. Click **Allow once**.
   - *"please think about cats"* → a streamed **Thinking** block appears.
   - attach a PDF or `.txt` → the mock echoes its text back.

**Using a real model:** add `https://api.openai.com/v1` with your key and model `gpt-4o-mini`.
**Fully local with Ollama:** `http://localhost:11434/v1`, any key, model `llama3.2`.

---

## 📚 Guides

| Guide | For |
|---|---|
| [User guide](docs/USER_GUIDE.md) | Everyone using Knitnox |
| [Features](docs/FEATURES.md) | The complete feature list |
| [Build guide](docs/BUILD_GUIDE.md) | Building and running on each OS |
| [Developer guide](docs/DEVELOPER_GUIDE.md) | Changing the code |
| [Architecture](docs/ARCHITECTURE.md) | How it works inside, in depth |

---

## 📖 Reference

Click a section to open it.

<details>
<summary><b>⚙️ Configuration</b> — environment variables and admin settings</summary>

All configuration is through environment variables. Everything else is set in the Admin UI and stored in SQLite.

| Variable | Default | Meaning |
|---|---|---|
| `KNITNOX_ADDR` | `:8080` | Listen address |
| `KNITNOX_DATA` | `./data` | Directory for `knitnox.db` (+ WAL files), `secret.key`, `uploads/`, `feedmedia/` and, unless moved, `backups/` |
| `KNITNOX_SECRET` | *(generated)* | Passphrase that encrypts API keys and MCP headers at rest. If unset, a random key is written to `$KNITNOX_DATA/secret.key` |
| `KNITNOX_SECURE_COOKIES` | off | Set to `1` when served over HTTPS |
| `KNITNOX_LOG` | `info` | `debug`, `info` or `warn` |
| `KNITNOX_LOG_FILE` | *(none)* | Also write the log to this file. It is turned over at 10 MB and one older file (`NAME.1`) is kept. Useful on Windows, where nothing collects what a program prints |
| `KNITNOX_BACKUP_DIR` | `$KNITNOX_DATA/backups` | Where automatic backups are written. Point it at **another disk** if you can. `off` disables them |

PowerShell: `$env:KNITNOX_ADDR=":3000"; go run ./cmd/knitnox`
Bash: `KNITNOX_ADDR=:3000 go run ./cmd/knitnox`

**Admin → Settings** (stored in the DB, take effect on the next message):

| Setting | Default | |
|---|---|---|
| Allow anyone to sign up | **off** | The first account can always be made. After that an admin creates accounts, or switches this on |
| Allowance for people who sign up | 0 (no limit) | Token quota given to accounts made by signing up, per day or month |
| Keep users' own providers and tools away from this server's network | off | See Security → SSRF |
| Housekeeping | 730 days / 730 days / 300 | How long the usage log and the audit log are kept, and how many past runs per agent. `0` = for ever. Chats, memories, tasks and posts are never removed |
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
| Give up on a silent model after | 300 s | If the provider sends no data for this long (before or during an answer), the reply fails with what it has so far instead of hanging. `0` = wait for ever |
| Longest time for one reply | 60 min | A reply or agent run still going after this is stopped, so a slot is always given back. `0` = no limit |
| Back up every / Backups to keep | 24 h / 7 | Automatic backups (see Maintenance). `0` hours = off |

Each user also has, under **Settings**: custom instructions, API keys, MCP servers (with the Automatic / Ask-first switch), notifications, and "Log out everywhere".

</details>

<details>
<summary><b>📦 Building for Windows, Linux and macOS</b> — one static binary, cross-compiles from anywhere</summary>

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
scripts/build-all.sh 1.0.0               # Linux / macOS / Git Bash
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

</details>

<details>
<summary><b>🌍 Deployment</b> — systemd, reverse proxy, Docker, Windows</summary>

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
RestartSec=5
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

**HTTPS matters:** installing Knitnox as an app and push notifications only work on `https://` (or `localhost`). The login limiter trusts `X-Forwarded-For` only when the connection comes from loopback, i.e. from a proxy on the same machine.

**Docker:**

```bash
docker build -t knitnox .
docker run -p 8080:8080 -v knitnox-data:/data -e KNITNOX_SECRET=... knitnox
```

Run **one instance only**. Workers and the event hub live in process memory (see Architecture → Scaling).

**On Windows**, a console window that is closed takes Knitnox with it and nothing keeps its log. Start it from Task Scheduler ("At startup", "Restart on failure") or wrap it as a service, and set `KNITNOX_LOG_FILE` (for example `C:\knitnox\knitnox.log`).

**Health check:** `GET /healthz` answers `ok` with 200 while the database answers, and 503 when it does not. Point your monitor or Docker `HEALTHCHECK` at it.

**Many tabs over plain `http://`:** a browser opens at most six connections to one server, and every open chat or Feed page holds one for live updates. With six or more Knitnox tabs open, further pages wait. Over HTTPS (Caddy, nginx with HTTP/2) all tabs share one connection and the limit is gone.

</details>

<details>
<summary><b>🧰 Maintenance</b> — backups, forgotten passwords, upgrades, logs</summary>

### Backups
**Automatic:** the server writes a backup every 24 hours into `KNITNOX_BACKUP_DIR` (default `data/backups`) and keeps the newest 7. Both numbers are under **Admin → Settings → Backups**, which also shows when the last backup ran, warns when it failed (it is then retried every hour) and has a **Back up now** button. Every backup's database copy is opened and checked (`PRAGMA integrity_check`) before the backup counts as written; a backup that fails half-way is removed.

The default folder is on the same disk as the data, which protects against a bad upgrade or a deleted chat but **not against a broken disk**. Set `KNITNOX_BACKUP_DIR` to another disk, or copy the folder elsewhere on a schedule. Each backup is a full copy, uploads included, so with many uploads keep fewer of them.

**By hand:** one command, safe to run while the server is up:
```bash
knitnox backup /backups            # → /backups/knitnox-20261109-150405/{knitnox.db, secret.key, uploads/, feedmedia/}
```
It uses `VACUUM INTO` for a consistent database copy (plain `cp` of `knitnox.db` can miss data still in `knitnox.db-wal`), checks the copy, copies `secret.key` if one exists, and copies the `uploads/` and `feedmedia/` folders. The command reads the same `KNITNOX_DATA` / `KNITNOX_SECRET` as the server.

What the backup contains and why:
1. **The database**: everything except files.
2. **`secret.key`, or your `KNITNOX_SECRET`.** Without it, stored API keys and MCP headers cannot be decrypted. Users would have to re-enter them; chats are unaffected. The VAPID keys for push notifications are in the database.
3. **`uploads/`**: images and documents (random file names, referenced from the `attachments` table).
4. **`feedmedia/`**: pictures, recordings and videos made for Feed posts.

To restore: stop the server, put `knitnox.db`, `secret.key`, `uploads/` and `feedmedia/` from one backup folder back into `KNITNOX_DATA` (delete the old `knitnox.db-wal` and `knitnox.db-shm` there first), start it.

### Forgotten passwords and usernames
Knitnox has no e-mail, so nothing is recovered by a link. Instead:

| Who | Forgot | What to do |
|---|---|---|
| A user | password | An admin sets a new one: **Admin → Users → ⋯ → New password**. The user is signed out everywhere and can change it again under Settings |
| A user | username | An admin sees every name under **Admin → Users**, and can rename an account there |
| Anyone signed in | wants another username or password | **Settings → Username** / **Password** (the current password is asked for). Changing the password signs the other devices out |
| The admin | password or username | On the machine where Knitnox runs (see below) |

```bash
knitnox list-users                            # every account: id, username, role, active/disabled
knitnox reset-password alice 'new-password'   # also re-enables the account and signs it out everywhere
knitnox rename-user alice alice.b
knitnox create-admin carol 'password123'      # a second admin is the simplest safety net
```
These commands use the same `KNITNOX_DATA` / `KNITNOX_SECRET` environment as the server and can be run while it is up. Whoever can run them owns the server, so they need no password. Sign-up asks for the password twice.

### Upgrading from Parley (the old name)
The app was renamed from Parley to Knitnox. Existing installs keep working without changes:
- `PARLEY_*` environment variables are still read when the matching `KNITNOX_*` one isn't set (switch when convenient).
- On first start, `DATA/parley.db` (and its `-wal`/`-shm` files) is renamed to `knitnox.db`. `secret.key` and `uploads/` are unchanged.
- Existing logins (the old `parley_session` cookie) stay valid.
- The binary is now `knitnox` / `knitnox.exe` and the systemd unit, Docker image and backup folders use the new name; update your scripts.

### Upgrades and migrations
Migrations in `internal/store/migrations/NNN_name.sql` run automatically at startup, in order, each in a transaction, and are recorded in `schema_migrations`. **Never edit a migration that has shipped. Add a new file.** Back up before upgrading (**Admin → Settings → Backups → Back up now**).

Changed defaults to know about when upgrading from a version before November 2026:
- **Sign-up is closed** unless the settings were saved with it on. Open it again under **Admin → Settings** if you want it.
- **Old log rows are removed**: usage and audit rows older than 730 days, and agent runs beyond the newest 300 per agent. Set the limits to `0` to keep everything.
- **Replies have time limits** (5 minutes of silence, 60 minutes in total). Raise them for very slow local models.

### Things that grow
- `usage_log` gets one row per LLM call, `audit_log` one per admin action, and every agent run stays as a run with its messages. The hourly clean-up removes the ones past the limits under **Admin → Settings → Housekeeping**, then runs `PRAGMA optimize` and folds the write-ahead log back into the database file.
- `messages` grows forever (compaction never deletes). Deleting a chat removes its messages.
- `backups/` holds the newest backups (7 unless changed), each a full copy including uploads.
- `feedmedia/` grows with the pictures, recordings and videos of Feed posts; an hourly sweep removes files no post points to.
- `uploads/` grows with images and documents, and with the pictures and recordings made in chats (they are stored like uploads and go when their chat is deleted). Deleting a chat deletes its files, and an hourly sweep removes files no row references any more (from deleted users, or queued messages that were cancelled). Uploads are capped at 6 files per message, 10 MB per image and 20 MB per document; browsers resize photos to ≤2048 px first, and extracted document text is capped at 120k characters per file.
- `messages_fts` (the search index) is roughly the size of the message text. It is maintained by triggers; rebuild it after a manual bulk import with `INSERT INTO messages_fts(messages_fts) VALUES('rebuild');`.
- `tool_approvals` gets one row per asked tool call. It is small.
- Expired sessions are purged hourly.

### Restarts
A reply that is generating during a restart is marked **interrupted**, and its partial text is kept. Queued messages **resume automatically** on startup.

### When something goes wrong inside a run
- An answer whose connection closes before the provider said it was finished is marked failed with the text so far, not saved as a complete answer.
- A provider that stops sending data fails the reply after the idle time above; the text so far is kept and the user can press Retry. Stalled calls are not retried automatically.
- A bug (panic) in one reply, agent run, media job or notification is caught: that run is marked failed, the error and stack go to the log (`panic while running a reply`, `panic in background work`), and the server keeps running.
- Still run Knitnox under something that restarts it (systemd `Restart=on-failure`, Docker `--restart unless-stopped`, or a Windows service), for the cases no program can catch, like running out of memory.

### Logs
Logs are structured text on stderr (`log/slog`). Worth watching: `run failed`, `run hit the time limit`, `backup failed`, `panic …`, `mcp connect failed`, `retrying LLM call`, `thread compacted`.

### Updating vendored JS and fonts
`internal/web/static/vendor/` holds htmx 2.0.4, marked 15.0.7, DOMPurify 3.2.4 and highlight.js 11.11.1 (common-languages build). Replace the files to upgrade, and keep DOMPurify current because it is the XSS barrier for model output.
`internal/web/static/fonts/` holds Inter and JetBrains Mono (variable, Latin subset, SIL Open Font License) from `@fontsource-variable`.

### Changing the look
All colours, radii and shadows are CSS variables at the top of `internal/web/static/style.css`, with a dark-mode block right below (`--content-w` sets the chat column width, 920 px by default). Icons are inline SVG templates in `internal/web/templates/icons.html`; use one with `{{template "i-name"}}`. The app icon is `static/icons/favicon.svg` (a copy of `favicon.svg` in the repo root); it is the favicon, the logo in the UI (`.logo` in `style.css`) and the source of the PNGs for home screens and notifications (`icon-192/512`, `icon-512-maskable`, `apple-touch-icon`, `favicon-32`). To change the brand icon, replace that SVG and re-export the PNGs at those sizes. Static URLs are versioned automatically (`style.css?v=<build hash>`), so edited CSS/JS shows up on the next page load, including in installed apps.

</details>

<details>
<summary><b>🧪 Development and tests</b> — project layout, running tests</summary>

```bash
go test ./...              # engine end-to-end tests with fake LLM + MCP servers
go vet ./...
go build -o knitnox ./cmd/knitnox
```

- Templates, CSS and JS are embedded with `go:embed`, so **restart the server after editing them** (`go run` rebuilds).
- `go test -race ./...` needs CGO (`CGO_ENABLED=1` and a C compiler). `.github/workflows/test.yml` runs it on Linux for every push.

**Project layout**

```
cmd/
  knitnox/       main: config, startup, graceful shutdown, CLI commands
  mockllm/       dev-only fake OpenAI + MCP server
internal/
  store/         ALL SQL lives here (+ migrations/*.sql, embedded)
  secret/        AES-256-GCM for API keys / MCP headers
  llm/           streaming client for /chat/completions (tool calls, reasoning, Retry-After)
  mcp/           MCP client (Streamable HTTP transport)
  netguard/      which addresses the server may call for a user (SSRF check at connect time)
  files/         upload classification + text extraction (PDF, text)
  hub/           in-memory pub/sub per thread (feeds SSE)
  engine/        thread workers, queue, slots, agent loop, tool approval, compaction, quotas,
                 agents and their scheduler, memory, Plan Book, Feed
  web/           router, auth/CSRF/login limiter, handlers, push, backups, templates/, static/ (all embedded)
docs/            the guides (shown in the app under Help), ARCHITECTURE.md, and the website (index.html)
.github/workflows/test.yml   vet, tests with the race detector and a build, on every push
```

Dependencies only point downward: `web → engine → (llm, mcp, hub, store) → secret`, `web → files → store`, and `llm`, `mcp → netguard`. Only `store` touches the database.

**Tests** (all with in-process fake LLM / MCP servers, no network):
- `internal/engine/engine_test.go`: a basic turn and usage, queue order and limit, the tool loop through MCP (JSON and SSE responses), group-chat name prefixes and tool hiding, compaction, quota blocking, stopping mid-stream, editing and regenerating, images as content parts.
- `internal/engine/features_test.go`: tool approval (allow, deny, timeout, stop while waiting), the parallel-run cap, custom instructions (and that personal ones stay out of group chats), inline file text, reasoning capture, search and paging.
- `internal/engine/reliability_test.go`, `safety_test.go`: a silent provider, the per-run time limit, a panic inside a run or a hook, a cut-off answer, and that a tool call which may have run is not repeated.
- `internal/llm/llm_test.go`, `stall_test.go`: `Retry-After` handling; the idle watchdog before and during an answer.
- `internal/web/web_test.go`: HTTP-level flows: CSRF, login lockout, uploads and their validation, permissions for non-members, rename/pin/archive/export/search, tool approval over HTTP, paging, admin settings and the audit log, PWA files.
- `internal/web/accounts_test.go`, `hardening_test.go`: the password typed twice, changing a username, signing other devices out, the admin backup; sign-up closed by default and limited per address, the login limiter's keys, `X-Forwarded-For`, the health check.
- `internal/store/backup_test.go`, `housekeeping_test.go`: a backup can be opened again, a failed one leaves nothing behind, only the newest are kept; old log rows and agent runs are removed and chats are not; exactly one first admin.
- `internal/netguard`, `internal/mcp`, `internal/secret`, `internal/files`: the address rules, which failed tool calls are safe to repeat, sealing and opening secrets, file types decided by content.
- Agents, memory, the Plan Book, the Feed and the help pages each have their own test files in `internal/engine` and `internal/web`; the Developer guide lists them.

The mock server (`go run ./cmd/mockllm`) exercises every feature without an API key.

</details>

<details>
<summary><b>🧩 Extending</b> — common recipes for changing the code</summary>

### Add an admin setting
1. Add a field and default in `store/settings.go` (`Settings`, `DefaultSettings`, `LoadSettings`, `SaveSettings`).
2. Add an input in `templates/admin.html` (settings tab) and parse it in `web/admin.go → adminSettingsSave`.
3. Read it in the engine through `tc.settings` (loaded once per run).

### Add a column or table
1. Create `internal/store/migrations/NNN_whatever.sql` with the next free number (`ALTER TABLE …` / `CREATE TABLE …`).
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

</details>

<details>
<summary><b>🔒 Security notes</b> — how users, keys and the server are protected</summary>

- **Passwords**: bcrypt. **Sessions**: random 256-bit tokens, only the SHA-256 is stored, HttpOnly + SameSite=Lax cookies.
- **CSRF**: per-session token, required on every non-GET (`X-CSRF-Token` header or `_csrf` field).
- **XSS**: templates auto-escape. Model output is rendered as Markdown and passed through DOMPurify (with `style` attributes stripped). The CSP is `default-src 'self'` with **no `unsafe-inline`** for scripts or styles, plus `frame-ancestors 'none'`, `base-uri 'self'` and `form-action 'self'`.
- **Uploads**: types are decided by sniffing the bytes, never by the file name. Documents are served as downloads (`Content-Disposition: attachment`), and HTML/XML uploads are served as plain text, so an uploaded page can never run in Knitnox's origin.
- **Sign-up** is closed unless an admin opens it, is limited to 10 accounts per address in 15 minutes, and the first-account-is-admin rule is decided inside one database statement.
- **Login**: within 15 minutes, 10 wrong passwords for one account from one address, 30 failures from one address, or 100 for one account from anywhere lock that out for 15 minutes. So a stranger cannot lock a known name out by failing on purpose. Whether a name exists, or is disabled, is told only to someone who knows its password. Behind a proxy on the same machine the caller's address is the **last** entry of `X-Forwarded-For`. Changing or resetting a password signs the other devices out.
- **Pictures from other sites** in an answer are not loaded by themselves: a button stands in their place and the picture loads on a click. Otherwise text the AI was fed (a web page, a file, a tool result) could make it write a picture address that carries your data to someone else's server.
- **Tool approval**: "Ask before running" is the safe default for anything with side effects. Only the chat owner or the person whose message triggered the call can decide, and "Always allow" (which switches the server to automatic) is owner-only.
- **API keys / MCP headers**: AES-256-GCM at rest and never sent back to the browser (shown as `sk-…abcd`).
- **Group chats**: members never get the owner's MCP tools or personal API key unless the owner opts in per chat. Personal custom instructions are never applied in group chats.
- **SSRF**: users can make the server call an address they enter (their own provider's base URL, a tool server, a picture link a provider returns). `internal/netguard` checks every such connection where it is opened, after the name was looked up, so redirects and names that change are covered. Link-local addresses (`169.254.0.0/16`, `fe80::/10`, where cloud hosts keep credentials) are always refused. With **Admin → Settings → Keep users' own providers and tools away from this server's network** on, this machine and private networks are refused too. It is off at first because a tool server or Ollama on the same machine is common; turn it on when not everyone with an account is fully trusted. Addresses an admin sets up (shared providers) are never checked.
- **Tool calls are not repeated blindly**: a call is retried only when it cannot have reached the tool (connection refused, or the server answered 429/503). After a timeout or a lost answer the model is told the tool may have run.
- **The key file**: `secret.key` lies next to the database and is copied into every backup, so the encryption of API keys protects against someone who gets only the database file, not the whole folder or a backup. To keep them apart, set `KNITNOX_SECRET` in the service's environment instead and delete `secret.key` (existing keys then have to be entered again unless the value is the file's content).
- **Audit**: every admin action (providers, models, users, quotas, password resets, settings) is recorded with who did it, under Admin → Audit.

</details>

<details>
<summary><b>⚠️ Known limitations</b></summary>

- Single instance (in-memory workers, hub, slots and pending approvals).
- MCP: Streamable HTTP transport only. The legacy HTTP+SSE transport (`GET /sse`), OAuth flows and stdio are not supported. Static auth headers work.
- Files: images, PDF (text layer only, no OCR) and text-like files. Images are sent for the current turn and the 3 before it, documents for the current turn and the 6 before it; older ones become a short note to save tokens, so the summary has to carry what matters. Word/Excel/PowerPoint are not parsed.
- Search matches the beginnings of words (prefix queries) in message text, not inside attached files.
- A tool call waiting for approval blocks that chat's queue until someone decides or the timeout passes.
- The sidebar doesn't live-update when someone invites you; reload to see new chats.
- No e-mail: a forgotten password is reset by an admin, or on the server's command line.
- Over plain `http://`, six or more open Knitnox tabs make further pages wait (see Deployment).
- Backups are full copies. There is no incremental backup and no built-in copy to another machine.

</details>

---

<div align="center">

Made with care · self-hosted · your data stays on your machine

</div>
