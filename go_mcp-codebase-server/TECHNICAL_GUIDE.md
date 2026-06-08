# MCP Codebase Server — Technical Guide

> For developers who want to understand, modify, extend, or debug the server internals.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Source File Map](#source-file-map)
3. [Dependency Graph](#dependency-graph)
4. [Request Lifecycle](#request-lifecycle)
5. [Tool System Deep Dive](#tool-system-deep-dive)
6. [Path Validation & Security Model](#path-validation--security-model)
7. [Search Engine Internals](#search-engine-internals)
8. [Analysis Engine Internals](#analysis-engine-internals)
9. [Token Efficiency Mechanisms](#token-efficiency-mechanisms)
10. [CORS & HTTP Transport](#cors--http-transport)
11. [Extending the Server](#extending-the-server)
12. [Testing & Debugging](#testing--debugging)
13. [Known Limitations](#known-limitations)
14. [Build & Cross-Compilation](#build--cross-compilation)

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                         main.go                                  │
│  CLI flags → resolve paths → create server → start HTTP          │
└──────────┬─────────────────────────────────────────────┬────────┘
           │                                             │
           ▼                                             ▼
┌──────────────────────┐                  ┌──────────────────────┐
│     server.go        │                  │    HTTP Transport     │
│  MCP server init     │◄─────────────────│  (mcp-go Streamable  │
│  Tool registration   │                  │   HTTP + CORS)        │
│  Helper functions    │                  └──────────────────────┘
│  Gitignore parsing   │
│  Exclusion lists    │
└──────┬───────┬───────┴──────┬──────────────┐
       │       │              │              │
       ▼       ▼              ▼              ▼
┌────────┐ ┌────────┐ ┌────────────┐ ┌───────────┐
│fileops │ │search  │ │analysis    │ │ (future)  │
│  .go   │ │  .go   │ │   .go      │ │           │
└────────┘ └────────┘ └────────────┘ └───────────┘
```

**Key principle:** Each tool category lives in its own file. All handlers are closures created by `make*Handler(rootDir, allowedDirs)` factory functions, capturing the root directory and allowed directories for path validation.

---

## Source File Map

| File | Lines | Purpose |
|------|-------|---------|
| `main.go` | ~76 | Entry point, CLI parsing, server startup, graceful shutdown |
| `server.go` | ~423 | MCP server creation, tool definitions, CORS middleware, utility functions (path validation, arg extraction, gitignore, exclusion lists, binary detection, source file detection) |
| `fileops.go` | ~363 | `read_file`, `write_file`, `edit_file`, `delete_path`, `create_directory`, `list_directory` handlers + `formatSize`, `truncate` helpers |
| `search.go` | ~222 | `search_codebase` handler with regex search, file walking, context lines |
| `analysis.go` | ~789 | `project_tree` handler + `analyze_codebase` handler + language-specific parsers (Go, Python, JS/TS, Rust, Java, generic) + output formatters |
| `go.mod` | ~12 | Module definition, dependencies |
| `README.md` | ~195 | Basic documentation |
| `USER_GUIDE.md` | — | End-user guide |

---

## Dependency Graph

### External Dependencies

```
github.com/mark3labs/mcp-go v0.32.0
  ├── mcp          — Tool definitions, CallToolRequest/Result types
  └── server       — MCPServer, StreamableHTTPServer
       └── (internal: uuid, cast, uritemplate)
```

The `mcp-go` library provides:
- `mcp.NewTool()` — Tool definition with parameters
- `mcp.CallToolRequest` — Incoming tool call request
- `req.GetArguments()` — Extract arguments as `map[string]any`
- `mcp.NewToolResultText()` / `mcp.NewToolResultError()` — Response helpers
- `server.NewMCPServer()` — Server core
- `server.NewStreamableHTTPServer()` — HTTP transport

### Internal Package Dependencies

```
main.go
  ├── imports newServer(), corsMiddleware() from server.go
  └── imports getAbsPath() from server.go

server.go
  ├── imports all make*Handler / *Tool functions from fileops.go, search.go, analysis.go
  └── defines shared utilities used by all handlers

fileops.go
  └── uses validatePath, getStringArg, getIntArg, getBoolArg, textResult, errorResult, formatSize, truncate from server.go

search.go
  └── uses validatePath, getStringArg, getIntArg, getBoolArg, textResult, errorResult, shouldExcludeDir, isBinaryFile from server.go

analysis.go
  └── uses validatePath, getStringArg, getIntArg, getBoolArg, textResult, errorResult, shouldExcludeDir, loadGitignore, formatSize, isSourceFile from server.go
```

---

## Request Lifecycle

```
1. Client sends HTTP POST to /mcp
   │
   ▼
2. corsMiddleware (server.go:76)
   │  Sets CORS headers on every response
   │  Handles OPTIONS preflight requests
   │
   ▼
3. mcp-go StreamableHTTPServer
   │  Manages session (Mcp-Session-Id header)
   │  Parses JSON-RPC request
   │  Routes to appropriate method handler
   │
   ▼
4. For "tools/call" method:
   │  mcp-go dispatches to registered tool handler
   │
   ▼
5. Handler closure (e.g., makeReadFileHandler)
   │  Extracts arguments via getStringArg/getIntArg/getBoolArg
   │  Validates path via validatePath()
   │  Performs the operation
   │  Returns textResult() or errorResult()
   │
   ▼
6. mcp-go wraps result into JSON-RPC response
   │
   ▼
7. HTTP response sent to client
```

### Key Types

```go
// Request
mcp.CallToolRequest
  .GetArguments() → map[string]any

// Response
mcp.NewToolResultText(text string) → *CallToolResult
mcp.NewToolResultError(text string) → *CallToolResult
```

---

## Tool System Deep Dive

### Tool Registration Pattern

Every tool follows the same pattern in `server.go`:

```go
// 1. Define the tool schema
func readFileTool() mcp.Tool {
    return mcp.NewTool("read_file",
        mcp.WithDescription("..."),
        mcp.WithString("path", mcp.Required(), mcp.Description("...")),
        mcp.WithNumber("offset", mcp.Description("...")),
        // ... other parameters
    )
}

// 2. Define the handler factory
func makeReadFileHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // ... implementation
    }
}

// 3. Register in newServer()
mcpServer.AddTool(readFileTool(), makeReadFileHandler(rootDir, allowedDirs))
```

### Argument Extraction

Arguments come as `map[string]any` from JSON-RPC. Helper functions handle type coercion:

```go
// server.go:45-70
func getStringArg(args map[string]any, key string, defaultVal string) string
func getIntArg(args map[string]any, key string, defaultVal int) int      // JSON numbers → float64 → int
func getBoolArg(args map[string]any, key string, defaultVal bool) bool
```

**Important:** JSON numbers are decoded as `float64` by Go's `encoding/json`. `getIntArg` handles this by type-asserting to `float64` first, then converting to `int`.

### Response Helpers

```go
// server.go:278-292
func textResult(text string) *mcp.CallToolResult    // Success response with text
func errorResult(msg string) *mcp.CallToolResult     // Error response (not an error return)
func marshalResult(v any) (*mcp.CallToolResult, error) // JSON-marshal any struct
```

**Design choice:** Errors are returned as `errorResult()` in the tool result (not as Go errors). This is because MCP tool errors are informative messages for the AI agent, not transport-level failures. The only `error` return is for truly unexpected failures (marshal errors, etc.).

---

## Path Validation & Security Model

### `validatePath()` (server.go:27-43)

```go
func validatePath(path string, rootDir string, allowedDirs []string) (string, error)
```

1. Joins `path` with `rootDir` (handles relative paths like `"src/main.go"`)
2. Resolves to absolute path with `filepath.Abs()` and `filepath.Clean()`
3. Checks if the resolved path starts with `rootDir` → allowed
4. If not, checks each directory in `allowedDirs` → allowed
5. If neither → returns error: "path is outside allowed directories"

**This prevents:**
- `../../etc/passwd` — path traversal
- Absolute paths outside the sandbox
- Symlinks that escape the root (via `filepath.Clean`)

### `getAbsPath()` (server.go:19-25)

Used only during startup to resolve the `-root` and `-allow` flags to absolute paths.

### Root Deletion Guard

In `delete_path` handler (fileops.go:217-219):
```go
if absPath == rootDir {
    return errorResult("Cannot delete the root directory"), nil
}
```

---

## Search Engine Internals

### `search_codebase` Algorithm (search.go)

```
1. Compile regex (with (?i) prefix if case-insensitive)
2. Walk directory tree with filepath.WalkDir
   ├── Skip excluded dirs (shouldExcludeDir + extra exclude)
   ├── Skip hidden files (dotfiles)
   ├── Skip binary files (isBinaryFile)
   ├── Filter by include globs (filepath.Match)
   └── For each text file:
       ├── Read entire file
       ├── Run regex line-by-line
       └── Collect matches with line numbers
3. Stop at max_results total matches
4. Format output:
   ├── Per-file header: 📁 path
   ├── Per-match: context lines + ▶ marker on match line
   ├── Lines truncated at 300 chars
   └── Total output capped at 15,000 chars
```

### Binary Detection (server.go:370-389)

```go
func isBinaryFile(path string) bool
```

Reads the first 8KB of a file. If any null byte (`\x00`) is found, it's classified as binary. This is a simple heuristic that works well for most cases (compiled binaries, images, compressed files).

### Exclusion List (server.go:339-367)

`defaultExcludeDirs` is a hardcoded map of directory names to skip:
```
.git, .svn, .hg, node_modules, vendor, __pycache__, .idea, .vscode,
dist, build, target, .next, .nuxt, .cache, .tox, .venv, venv, env,
.env, .direnv, coverage, .coverage, bower_components
```

---

## Analysis Engine Internals

### `project_tree` (analysis.go:18-138)

A recursive directory tree builder that:
1. Reads `.gitignore` patterns (if `respect_gitignore=true`)
2. Sorts entries: directories first, then alphabetically
3. Filters out: hidden files (except `.env`), excluded dirs, gitignored files
4. Renders as Unicode tree (`├──`, `└──`) with emoji indicators
5. Shows file sizes using `formatSize()`
6. Limits depth; shows `...` at leaf nodes beyond `max_depth`

### `analyze_codebase` (analysis.go:158-240)

Collects source files, analyzes each one, then formats based on analysis type:

```
1. Walk directory → collect source files (isSourceFile filter)
2. Filter by file_pattern glob (if provided)
3. For each file → analyzeFile()
   ├── Skip files > 500KB
   ├── Detect language by extension
   └── Run language-specific parser
4. Format output based on analysis_type
```

### Language-Specific Parsers

Each language has two parsers: one for **imports** and one for **symbols**:

| Language | Import Parser | Symbol Parser | What it detects |
|----------|--------------|---------------|-----------------|
| **Go** | `parseGoImports` | `parseGoSymbols` | Single/multi-line imports, `func`, `type ... struct/interface`, `const`, `var` |
| **Python** | `parsePythonImports` | `parsePythonSymbols` | `import`/`from ... import`, `class`, `def` |
| **JS/TS** | `parseJSImports` | `parseJSSymbols` | `import ... from`, `require()`, function/class/interface/type declarations, arrow exports |
| **Rust** | `parseRustImports` | `parseRustSymbols` | `use` statements, `fn`, `struct`, `enum`, `trait`, `impl` |
| **Java** | `parseJavaImports` | `parseJavaSymbols` | `import` statements, class/interface/method declarations |
| **Other** | `parseGenericImports` | `parseGenericSymbols` | Best-effort regex matching for common patterns |

### Source File Detection (server.go:392-423)

`isSourceFile()` checks against a whitelist of 60+ file extensions (`.go`, `.py`, `.js`, `.ts`, `.rs`, `.java`, etc.) plus special filenames like `Makefile`, `Dockerfile`, `Gemfile`.

### Connection Analysis (analysis.go:658-721)

`formatConnections()` builds a dependency graph by:
1. Creating a `localFiles` map of all analyzed file paths
2. For each file's imports, checking if any match a local file's name (heuristic: `filepath.Base(import) == filepath.Base(localFile)` minus extension)
3. Building a symbol index: `map[string][]string` (symbol name → file paths)
4. Showing symbols defined in multiple files (potential interfaces/implementations)

---

## Token Efficiency Mechanisms

| Mechanism | Implementation | Location |
|-----------|----------------|----------|
| **Line truncation (500 chars)** | `if len(line) > maxLineLen` in read_file handler | fileops.go:64-68 |
| **Search line truncation (300 chars)** | `if len(lineContent) > 300` | search.go:211-213 |
| **Search output cap (15,000 chars)** | `if sb.Len() > 15000` | search.go:183-186 |
| **Max results limit** | Configurable `max_results` param (default 100) | search.go:157-159 |
| **Context lines** | Configurable `context_lines` (default 2) | search.go:196-203 |
| **File size skip in analysis** | Files > 500KB are skipped | analysis.go:254-256 |
| **Read file pagination** | `offset` + `limit` parameters | fileops.go:39-55 |
| **Edit preview limit** | Max 10 diff lines shown | fileops.go:181-188 |
| **Relative paths** | All output uses paths relative to rootDir | Throughout |
| **Tree depth limit** | `max_depth` parameter (default 6) | analysis.go:23 |

---

## CORS & HTTP Transport

### CORS Middleware (server.go:76-90)

```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Request-ID, Mcp-Session-Id")
        w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**Key detail:** `Mcp-Session-Id` is both in `Allow-Headers` and `Expose-Headers`. The `Expose-Headers` is critical because without it, browsers can't read the session ID from the response.

### Transport Flow

```
mcpServer (MCPServer) → httpServer (StreamableHTTPServer) → corsMiddleware → http.Server
```

The `corsMiddleware` wraps the `StreamableHTTPServer` which wraps the `MCPServer`. The middleware is applied via `http.Handler` composition:

```go
handler := corsMiddleware(mcpServer)  // mcpServer implements http.Handler
httpServer := &http.Server{Addr: addr, Handler: handler}
```

### Session Management

The `StreamableHTTPServer` from `mcp-go` handles MCP session management automatically:
- On `initialize`, it creates a session and returns `Mcp-Session-Id` in the response headers
- On subsequent requests, it validates the session ID from the `Mcp-Session-Id` request header
- Invalid or missing session IDs result in a 400 error

---

## Extending the Server

### Adding a New Tool

**Step 1:** Define the tool schema in `server.go`:

```go
func myNewTool() mcp.Tool {
    return mcp.NewTool("my_new_tool",
        mcp.WithDescription("Does something useful"),
        mcp.WithString("param1",
            mcp.Required(),
            mcp.Description("First parameter"),
        ),
        mcp.WithNumber("param2",
            mcp.Description("Optional number"),
        ),
    )
}
```

**Step 2:** Create the handler in a new or existing file:

```go
func makeMyNewToolHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        args := req.GetArguments()
        param1 := getStringArg(args, "param1", "")
        
        // Validate and use param1...
        
        return textResult("Success!"), nil
    }
}
```

**Step 3:** Register in `newServer()`:

```go
mcpServer.AddTool(myNewTool(), makeMyNewToolHandler(rootDir, allowedDirs))
```

**Step 4:** Rebuild and test:

```bash
go build -o mcp-codebase-server . && ./mcp-codebase-server -port 8080
```

### Adding a New Language Parser

In `analysis.go`, add two functions following the existing pattern:

```go
func parseSwiftImports(content string) []string {
    // Parse: import Foundation, import UIKit, etc.
    var imports []string
    re := regexp.MustCompile(`^import\s+(\w+)`)
    for _, line := range strings.Split(content, "\n") {
        m := re.FindStringSubmatch(strings.TrimSpace(line))
        if len(m) > 1 {
            imports = append(imports, m[1])
        }
    }
    return imports
}

func parseSwiftSymbols(content string) []symbolInfo {
    // Parse: func, class, struct, enum, protocol, var, let
    var symbols []symbolInfo
    // ... regex patterns ...
    return symbols
}
```

Then add the language to `detectLanguage()` and the `switch` in `analyzeFile()`:

```go
case ".swift":
    analysis.Imports = parseSwiftImports(content)
    analysis.Symbols = parseSwiftSymbols(content)
```

### Modifying CORS Policy

Edit `corsMiddleware()` in `server.go:76-90`. To restrict to specific origins:

```go
allowedOrigins := []string{"http://localhost:3000", "http://192.168.1.100:8080"}
origin := r.Header.Get("Origin")
for _, allowed := range allowedOrigins {
    if origin == allowed {
        w.Header().Set("Access-Control-Allow-Origin", origin)
        break
    }
}
```

---

## Testing & Debugging

### Manual Testing with curl

```bash
# Step 1: Initialize
curl -s -D headers.txt \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
  http://localhost:8080/mcp

# Extract session ID from headers
SESSION_ID=$(grep -i "mcp-session-id" headers.txt | awk '{print $2}' | tr -d '\r')

# Step 2: List tools
curl -s \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  http://localhost:8080/mcp | jq .

# Step 3: Call a tool
curl -s \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"go.mod"}}}' \
  http://localhost:8080/mcp | jq .
```

### PowerShell Testing

```powershell
# PowerShell has issues with curl (alias for Invoke-WebRequest)
# Use curl.exe directly or write JSON to a file first

$body = '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
[System.IO.File]::WriteAllText("C:\tmp\req.json", $body)

# Initialize
curl.exe -s -D - --data-binary "@C:\tmp\req.json" -H "Content-Type: application/json" http://localhost:8080/mcp

# Extract session ID from the Mcp-Session-Id header, then:
curl.exe -s --data-binary "@C:\tmp\req2.json" -H "Content-Type: application/json" -H "Mcp-Session-Id: <SESSION-ID>" http://localhost:8080/mcp
```

### Debugging Checklist

| Problem | Check |
|---------|-------|
| "Invalid session ID" | Send `initialize` first; include `Mcp-Session-Id` header |
| "request body is not valid json" | Validate JSON (no trailing commas, proper escaping) |
| Path not found | Use paths relative to `-root` directory |
| "path is outside allowed directories" | Add the directory to `-allow` flag |
| Tool not found | Verify tool name exactly (underscore, not hyphen) |
| Empty results from search | Check regex syntax (Go RE2, no lookaheads) |
| Build error | Run `go mod tidy` then `go build` |

---

## Known Limitations

1. **Gitignore support is basic** — Only handles simple glob patterns. Doesn't support negation patterns (`!foo`) or nested `.gitignore` files.

2. **Connection analysis is heuristic** — File dependency detection uses `filepath.Base` matching (import path's basename vs. local file basename). This works well for Go imports but may produce false positives/negatives for JavaScript's `from './utils'` style imports.

3. **No authentication** — The server is designed for local network use. There's no API key, token, or user authentication.

4. **Regex engine** — Uses Go's RE2 engine, which does not support lookahead/lookbehind assertions. Patterns like `(?=...)` will fail.

5. **Single-process** — No clustering, load balancing, or multiprocess support. One server, one process.

6. **No file watching** — No filesystem event notifications. Clients must poll or re-read files.

7. **In-memory sessions** — Session state is in-process. Restarting the server invalidates all sessions.

8. **No concurrent write protection** — Multiple simultaneous `edit_file` or `write_file` calls to the same file may race. No file locking is implemented.

9. **Binary detection heuristic** — The null-byte check (`isBinaryFile`) may misidentify some rare text files (e.g., UTF-16) as binary.

10. **Language parsers are regex-based** — Not AST-based. They can produce false positives for commented-out code or string literals containing code patterns.

---

## Build & Cross-Compilation

### Standard Build
```bash
go build -o mcp-codebase-server .
```

### Optimized Build (Smaller Binary)
```bash
go build -ldflags="-s -w" -o mcp-codebase-server .
# Strips debug info, reduces binary from ~9.3MB to ~6.5MB
```

### Cross-Compilation
```bash
# Linux (from Windows/Mac)
GOOS=linux GOARCH=amd64 go build -o mcp-codebase-server-linux .

# macOS
GOOS=darwin GOARCH=amd64 go build -o mcp-codebase-server-macos .

# ARM (Raspberry Pi, etc.)
GOOS=linux GOARCH=arm64 go build -o mcp-codebase-server-arm .
```

### Running as a Service (Linux)
```ini
# /etc/systemd/system/mcp-codebase.service
[Unit]
Description=MCP Codebase Server
After=network.target

[Service]
Type=simple
User=mcp
WorkingDirectory=/opt/mcp-codebase-server
ExecStart=/opt/mcp-codebase-server/mcp-codebase-server -port 8080 -root /home/user/projects
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable mcp-codebase
sudo systemctl start mcp-codebase
```

### Docker
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -ldflags="-s -w" -o mcp-codebase-server .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/mcp-codebase-server /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["mcp-codebase-server"]
```

```bash
docker build -t mcp-codebase-server .
docker run -p 8080:8080 -v /path/to/project:/workspace mcp-codebase-server -root /workspace
```

---

*Built with [mcp-go](https://github.com/mark3labs/mcp-go) v0.32.0 • MIT License*