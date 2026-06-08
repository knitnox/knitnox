# MCP Codebase Server — User Guide

> A Model Context Protocol (MCP) server that gives AI agents the ability to read, write, edit, delete, search, and understand codebases — all over HTTP with full CORS support.

---

## Table of Contents

1. [Overview](#overview)
2. [Installation & Build](#installation--build)
3. [Starting the Server](#starting-the-server)
4. [Connecting MCP Clients](#connecting-mcp-clients)
5. [Tool Reference](#tool-reference)
   - [File Operations](#1-file-operations)
   - [Search](#2-search)
   - [Codebase Understanding](#3-codebase-understanding)
6. [Session Management](#session-management)
7. [Security & Path Rules](#security--path-rules)
8. [Token Efficiency Design](#token-efficiency-design)
9. [Common Workflows](#common-workflows)
10. [Troubleshooting](#troubleshooting)

---

## Overview

The MCP Codebase Server exposes 9 tools through the Model Context Protocol that allow AI agents (like Claude, GPT, etc.) to:

| Category | Tools | Purpose |
|---|---|---|
| **File I/O** | `read_file`, `write_file`, `edit_file`, `delete_path`, `create_directory`, `list_directory` | Navigate and modify files |
| **Search** | `search_codebase` | Find text or regex patterns across the codebase |
| **Understanding** | `project_tree`, `analyze_codebase` | Map project structure and analyze code relationships |

The server uses the **Streamable HTTP** MCP transport, meaning all communication happens via `POST` requests to a single `/mcp` endpoint. It supports CORS for **any origin** (`Access-Control-Allow-Origin: *`), making it easy to integrate with browser-based clients and agents on your local network.

---

## Installation & Build

### Prerequisites
- **Go 1.23** or later

### Build from Source

```bash
cd mcp-codebase-server
go mod tidy
go build -o mcp-codebase-server .    # Produces mcp-codebase-server (or .exe on Windows)
```

The resulting binary is self-contained (~9 MB). No runtime dependencies needed.

### Verify

```bash
./mcp-codebase-server -port 8080 -root /path/to/your/project
```

You should see:
```
🚀 MCP Codebase Server starting on :8080
📁 Root directory: /path/to/your/project
🌐 CORS: allowing all origins
```

---

## Starting the Server

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | TCP port to listen on |
| `-root` | `.` (current directory) | Root directory that file operations are restricted to |
| `-allow` | `""` | Comma-separated list of additional directories to permit access to |

### Examples

```bash
# Serve current directory on default port
./mcp-codebase-server

# Serve a specific project on port 9090
./mcp-codebase-server -port 9090 -root /home/user/my-project

# Allow access to two additional directories
./mcp-codebase-server -port 8080 -root /home/user/my-project -allow "/tmp,/home/user/shared-libs"

# Windows
mcp-codebase-server.exe -port 8080 -root "C:\Users\user\project"
```

### Graceful Shutdown

The server handles `SIGINT` (Ctrl+C) and `SIGTERM` gracefully, draining in-flight requests for up to 5 seconds before exiting.

---

## Connecting MCP Clients

### Claude Desktop / Cursor / VS Code (MCP Settings)

Add to your MCP client configuration:

```json
{
  "mcpServers": {
    "codebase": {
      "url": "http://localhost:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

### Using with `curl`

The server speaks JSON-RPC 2.0 over HTTP. Here's the minimal flow:

**Step 1 — Initialize (get a session ID):**
```bash
curl -s -D - \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"my-client","version":"1.0"}}}' \
  http://localhost:8080/mcp
```

The response will include a `Mcp-Session-Id` header. Save it.

**Step 2 — List tools (requires session ID):**
```bash
curl -s \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <YOUR-SESSION-ID>" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  http://localhost:8080/mcp
```

**Step 3 — Call a tool:**
```bash
curl -s \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <YOUR-SESSION-ID>" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"main.go"}}}' \
  http://localhost:8080/mcp
```

### Programmatic (Go)

```go
import "github.com/mark3labs/mcp-go/client"

client, _ := client.NewStreamableHTTPClient("http://localhost:8080/mcp")
// Use the client to call tools...
```

---

## Tool Reference

### 1. File Operations

---

#### `read_file`
Read file contents with line numbers. Designed for token efficiency — you can read specific line ranges instead of entire files.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | ✅ | — | File path (relative to root or absolute) |
| `offset` | number | — | 1 | Starting line number (1-based) |
| `limit` | number | — | 2000 | Max lines to read. Use `0` to read the entire file |

**Example:**
```json
{
  "name": "read_file",
  "arguments": {
    "path": "src/main.go",
    "offset": 10,
    "limit": 50
  }
}
```

**Output format:**
```
📄 src/main.go (142 lines total, showing lines 10-59)
────────────────────────────────────────────────────────────
    10 │ import (
    11 │   "fmt"
    12 │   "os"
...
    59 │ }
... 83 more lines (use offset=60 to continue reading)
```

**Key behaviors:**
- Lines longer than 500 characters are truncated with `... (truncated)`.
- If you don't read the whole file, the output tells you how many lines remain and what `offset` to use next.
- Returns an error if the path is outside allowed directories.

---

#### `write_file`
Create a new file or overwrite an existing one. Parent directories are created automatically (like `mkdir -p`).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | ✅ | — | File path |
| `content` | string | ✅ | — | File content |

**Example:**
```json
{
  "name": "write_file",
  "arguments": {
    "path": "src/utils/helper.go",
    "content": "package utils\n\nfunc Hello() string {\n\treturn \"hello\"\n}\n"
  }
}
```

**Output:** `✅ Wrote src/utils/helper.go (4 lines, 49 bytes)`

---

#### `edit_file`
Make a surgical edit by finding and replacing exact text. Best for small, targeted changes.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | ✅ | — | File path |
| `old_text` | string | ✅ | — | Exact text to find (must match exactly, including whitespace) |
| `new_text` | string | ✅ | — | Replacement text |
| `replace_all` | boolean | — | false | Replace all occurrences instead of just the first |

**Example:**
```json
{
  "name": "edit_file",
  "arguments": {
    "path": "src/main.go",
    "old_text": "fmt.Println(\"hello\")",
    "new_text": "fmt.Println(\"world\")",
    "replace_all": false
  }
}
```

**Output:**
```
✅ Replaced 1 occurrence(s) in src/main.go
Changes preview:
  15: fmt.Println("world")
```

**Important notes:**
- `old_text` must match **exactly** — including indentation, whitespace, and newlines.
- If `old_text` is not found, you get an error.
- If `old_text` appears multiple times and `replace_all` is false, only the **first** occurrence is replaced.
- The response includes a diff-like preview of up to 10 changed lines.

---

#### `delete_path`
Delete a file or directory.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | ✅ | — | Path to delete |
| `recursive` | boolean | — | false | Set to `true` to delete non-empty directories |

**Examples:**
```json
// Delete a single file
{"name": "delete_path", "arguments": {"path": "tmp/test.go"}}

// Delete a directory and all its contents
{"name": "delete_path", "arguments": {"path": "tmp/cache", "recursive": true}}
```

**Safety:** You **cannot** delete the root directory. Non-empty directories require `recursive: true`.

---

#### `create_directory`
Create a directory and all parent directories (equivalent to `mkdir -p`).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | ✅ | — | Directory path to create |

**Example:**
```json
{"name": "create_directory", "arguments": {"path": "src/pkg/utils"}}
```

---

#### `list_directory`
List the contents of a directory with type indicators and file sizes.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | — | `.` (root) | Directory path |
| `show_hidden` | boolean | — | false | Whether to show dotfiles |

**Example:**
```json
{"name": "list_directory", "arguments": {"path": "src", "show_hidden": true}}
```

**Output format:**
```
📂 src/ (12 entries)
────────────────────────────────────────────────────────────
  📁 cmd/                                     D
  📁 internal/                                D
  📄 main.go                                 1.7 KB
  📄 utils.go                                523 B

3 directories, 2 files
```

---

### 2. Search

---

#### `search_codebase`
Search across the entire codebase using regex. Results include file paths, line numbers, and surrounding context.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `pattern` | string | ✅ | — | Regex pattern (Go RE2 syntax) |
| `path` | string | — | `.` (root) | Root directory to search in |
| `include` | string | — | all files | Comma-separated glob patterns (e.g. `*.go,*.ts`) |
| `exclude` | string | — | — | Additional directory names to exclude (comma-separated) |
| `max_results` | number | — | 100 | Maximum number of match results |
| `context_lines` | number | — | 2 | Lines of context before/after each match |
| `case_sensitive` | boolean | — | false | Case-sensitive matching |

**Examples:**
```json
// Find all function definitions in Go files
{
  "name": "search_codebase",
  "arguments": {
    "pattern": "^func\\s+\\w+",
    "include": "*.go"
  }
}

// Find TODO comments (case-insensitive by default)
{
  "name": "search_codebase",
  "arguments": {
    "pattern": "TODO|FIXME|HACK",
    "context_lines": 3,
    "max_results": 50
  }
}

// Find a specific string in a specific subdirectory
{
  "name": "search_codebase",
  "arguments": {
    "pattern": "validatePath",
    "path": "src/internal",
    "case_sensitive": true
  }
}
```

**Output format:**
```
🔍 Search: /func\s+\w+/ (case-insensitive)
12 matches in 3 files
────────────────────────────────────────────────────────────

📁 server.go
▶    18 │ func readFileTool() mcp.Tool {
  19 │ }
  20 │ }
▶    96 │ func makeReadFileHandler(
```

The `▶` marker indicates the matched line. Context lines appear above and below.

**Smart exclusions:** Automatically skips `.git`, `node_modules`, `vendor`, `__pycache__`, `dist`, `build`, `target`, `.next`, `.cache`, `.venv`, and other common non-source directories, as well as binary files.

---

### 3. Codebase Understanding

---

#### `project_tree`
Display the project directory structure in a compact tree format. Respects `.gitignore`, skips common exclusion dirs, shows file sizes.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | — | `.` (root) | Root directory |
| `max_depth` | number | — | 6 | Maximum tree depth |
| `dirs_only` | boolean | — | false | Only show directories |
| `respect_gitignore` | boolean | — | true | Respect `.gitignore` patterns |

**Example:**
```json
{
  "name": "project_tree",
  "arguments": {
    "path": ".",
    "max_depth": 4,
    "respect_gitignore": true
  }
}
```

**Output format:**
```
my-project/
├── 📁 cmd/
│   └── 📄 main.go (1.7 KB)
├── 📁 internal/
│   ├── 📄 server.go (13.1 KB)
│   ├── 📄 fileops.go (10.7 KB)
│   └── 📄 search.go (5.1 KB)
├── 📄 go.mod (234 B)
└── 📄 README.md (5.3 KB)

📊 3 directories, 5 files
```

**Key behaviors:**
- Hidden files (dotfiles) are hidden except `.env` and `.env.example`.
- Common exclusion directories (`.git`, `node_modules`, `vendor`, etc.) are automatically skipped.
- At maximum depth, subtrees show `...` instead of expanding.

---

#### `analyze_codebase`
Analyze codebase structure: imports, symbols (functions/types/classes), and file connections. Supports **Go, Python, JavaScript/TypeScript, Rust, Java, and C/C++** with a generic fallback for other languages.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `analysis_type` | string | ✅ | — | One of: `imports`, `symbols`, `connections`, `overview` |
| `path` | string | — | `.` (root) | Root directory to analyze |
| `file_pattern` | string | — | all source files | Glob pattern to filter files (e.g. `*.go`) |

**Analysis Types:**

| Type | What it shows |
|------|---------------|
| `imports` | Import/require/use statements per file |
| `symbols` | Functions, classes, interfaces, types per file with line numbers |
| `connections` | File dependency graph + symbol cross-references |
| `overview` | Language breakdown + imports + symbols (the default comprehensive view) |

**Examples:**
```json
// Get a full overview
{"name": "analyze_codebase", "arguments": {"analysis_type": "overview"}}

// Just see the function/type definitions
{"name": "analyze_codebase", "arguments": {"analysis_type": "symbols", "file_pattern": "*.go"}}

// Understand which files depend on which
{"name": "analyze_codebase", "arguments": {"analysis_type": "connections"}}
```

**Symbol type icons:**

| Icon | Meaning |
|------|---------|
| ƒ | Function / Method |
| © | Class |
| ◇ | Struct |
| ◈ | Interface / Trait |
| T | Type alias |
| E | Enum |
| C | Constant |
| V | Variable |
| ⊕ | Impl block |

---

## Session Management

The server uses the MCP Streamable HTTP transport, which requires **session management**:

1. **Initialize** first — send an `initialize` JSON-RPC request.
2. The server responds with a `Mcp-Session-Id` header.
3. **All subsequent requests** must include this `Mcp-Session-Id` header.
4. Without a valid session ID, the server returns `Invalid session ID`.

**Example flow:**
```
POST /mcp  →  initialize        →  Response with Mcp-Session-Id header
POST /mcp  →  tools/list        →  Must include Mcp-Session-Id
POST /mcp  →  tools/call        →  Must include Mcp-Session-Id
```

Most MCP clients handle this automatically. If you're using `curl` or writing a custom client, you must track the session ID yourself.

---

## Security & Path Rules

### Path Sandboxing
- All file operations are restricted to the **root directory** (set via `-root` flag).
- You can grant access to additional directories with the `-allow` flag.
- Path traversal attacks are prevented: `../../etc/passwd` will be rejected.
- Absolute paths are allowed **only** if they fall within the root or allowed directories.

### What You Can't Do
- **Delete the root directory** — blocked explicitly.
- **Access files outside** the root/allowed directories — `validatePath()` rejects them.
- **Write to binary files** — the search tool skips binary files automatically.

### CORS Policy
The server allows requests from **any origin** (`Access-Control-Allow-Origin: *`). This is intentional for local network usage with browser-based AI tools. If you need stricter CORS, modify the `corsMiddleware` in `server.go`.

---

## Token Efficiency Design

Every tool is designed to minimize token usage for AI agents:

| Feature | Benefit |
|---------|---------|
| Line numbers on all file reads | No need to re-read files just to find line positions |
| Offset/limit pagination | Read only the lines you need, not entire 10,000-line files |
| Line truncation at 500 chars | Prevents giant minified lines from wasting tokens |
| Search result truncation at 15,000 chars | Caps search output to avoid context overflow |
| Compact tree format | `project_tree` uses `├──`/`└──` notation instead of full paths |
| Smart directory exclusions | Automatically skips `.git`, `node_modules`, etc. |
| Binary file skipping | Never reads or searches binary files |
| Gitignore support | `project_tree` respects `.gitignore` to hide build artifacts |
| Diff-style edit previews | `edit_file` shows only changed lines, not the entire file |
| Relative paths in output | All paths are relative to root, keeping them short |

---

## Common Workflows

### Workflow 1: Explore an unknown codebase

```
1. project_tree      → Get the lay of the land
2. analyze_codebase   → Understand languages, imports, symbols
3. read_file          → Dive into specific files of interest
```

### Workflow 2: Find and fix a bug

```
1. search_codebase    → Find the relevant code
2. read_file          → Read the file with context
3. edit_file          → Make the surgical fix
4. read_file          → Verify the change
```

### Workflow 3: Add a new feature

```
1. project_tree      → Understand project structure
2. analyze_codebase   → Find related symbols and connections
3. search_codebase    → Find patterns to follow
4. create_directory   → Set up new package/folder
5. write_file         → Create new source files
6. edit_file          → Wire into existing code
```

### Workflow 4: Refactor across files

```
1. search_codebase    → Find all usages of the old pattern
2. edit_file          → Replace in each file (or use replace_all)
3. analyze_codebase   → Verify no broken imports/connections
```

---

## Troubleshooting

### "Invalid session ID"
You must send an `initialize` request first and include the `Mcp-Session-Id` header in all subsequent requests. Most MCP clients handle this automatically.

### "path is outside allowed directories"
The path you specified resolves outside the server's root directory. Use paths relative to the root, or add the directory with the `-allow` flag.

### "old_text not found"
The exact text in `edit_file` must match character-for-character, including whitespace and indentation. Use `read_file` first to see the exact content.

### "request body is not valid json"
The JSON payload is malformed. Common causes:
- Single quotes instead of double quotes
- Trailing commas (not allowed in JSON)
- Unescaped newlines in strings

### Server doesn't respond
- Check that the port isn't already in use.
- Verify the `-root` directory exists and is accessible.
- On Windows, use `curl.exe` instead of the PowerShell `curl` alias.

---

*Built with [mcp-go](https://github.com/mark3labs/mcp-go) v0.32.0 • MIT License*