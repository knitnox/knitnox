# MCP Codebase Server

A high-performance [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server built in Go that enables AI agents to efficiently navigate, search, understand, and modify codebases.

## ✨ Features

### 🔧 File Operations
| Tool | Description |
|------|-------------|
| `read_file` | Read file contents with line numbers, offset/limit for token efficiency |
| `write_file` | Create or overwrite files (auto-creates parent directories) |
| `edit_file` | Surgical find-and-replace edits with change preview |
| `delete_path` | Delete files or directories |
| `create_directory` | Create directories (like `mkdir -p`) |
| `list_directory` | List directory contents with type indicators and file sizes |

### 🔍 Search
| Tool | Description |
|------|-------------|
| `search_codebase` | Regex search across the entire codebase with context lines, file filtering, and smart exclusions |

### 📊 Codebase Understanding
| Tool | Description |
|------|-------------|
| `project_tree` | Compact tree view of project structure with .gitignore support |
| `analyze_codebase` | Analyze imports, symbols, and file connections (supports Go, Python, JS/TS, Rust, Java, C/C++) |

### 🎯 Token Efficiency
- **Line numbers** on all file reads for precise reference
- **Context lines** in search results (configurable)
- **Result limiting** to prevent token overflow
- **Line truncation** for very long lines
- **Compact output formats** with emoji indicators
- **Smart exclusions** (skips .git, node_modules, vendor, build dirs, binary files)

## 🚀 Quick Start

### Install
```bash
cd mcp-codebase-server
go mod tidy
go build -o mcp-codebase-server .
```

### Run
```bash
# Default: port 8080, current directory as root
./mcp-codebase-server

# Custom port and root directory
./mcp-codebase-server -port 9090 -root /path/to/project

# Allow additional directories
./mcp-codebase-server -allow "/tmp,/home/user/other-project"
```

### Command Line Flags
| Flag | Default | Description |
|------|---------|-------------|
| `-port` | 8080 | Server port |
| `-root` | `.` | Root directory for file operations |
| `-allow` | `""` | Comma-separated list of additional allowed directories |

## 🌐 Connecting AI Agents

The server uses the **Streamable HTTP** MCP transport with full CORS support (any origin allowed).

### Claude Desktop / Cursor / Other MCP Clients
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

### Programmatic (Go)
```go
client, err := mcp.NewStreamableHTTPClient("http://localhost:8080/mcp")
```

## 🛠 Tool Reference

### `read_file`
Read file contents with line numbers.
```json
{
  "path": "src/main.go",
  "offset": 10,      // Starting line (1-based, default: 1)
  "limit": 50        // Max lines to read (default: 2000, 0 = all)
}
```
Returns: File content with line numbers, total line count, and continuation hint.

### `write_file`
Create or overwrite a file.
```json
{
  "path": "src/new_file.go",
  "content": "package main\n\nfunc main() {}"
}
```

### `edit_file`
Surgical find-and-replace edit.
```json
{
  "path": "src/main.go",
  "old_text": "fmt.Println(\"hello\")",
  "new_text": "fmt.Println(\"world\")",
  "replace_all": false  // Replace all occurrences (default: false)
}
```
Returns: Number of replacements and a preview of changed lines.

### `delete_path`
Delete a file or directory.
```json
{
  "path": "tmp/cache",
  "recursive": true  // Required for non-empty directories
}
```

### `create_directory`
Create a directory (like `mkdir -p`).
```json
{
  "path": "src/pkg/utils"
}
```

### `list_directory`
List directory contents.
```json
{
  "path": ".",           // Default: root directory
  "show_hidden": false   // Show hidden files
}
```

### `search_codebase`
Regex search across the codebase.
```json
{
  "pattern": "func.*Handler",   // Required: regex pattern
  "path": ".",                  // Search root (default: root)
  "include": "*.go,*.ts",      // File glob filters
  "exclude": "vendor,tmp",     // Additional dirs to exclude
  "max_results": 100,           // Max results (default: 100)
  "context_lines": 2,           // Lines of context (default: 2)
  "case_sensitive": false       // Case sensitivity
}
```
Returns: Matches grouped by file with line numbers and context. `▶` marks match lines.

### `project_tree`
Get project structure as a tree.
```json
{
  "path": ".",                // Root directory
  "max_depth": 6,             // Max traversal depth
  "dirs_only": false,         // Only show directories
  "respect_gitignore": true   // Respect .gitignore
}
```

### `analyze_codebase`
Analyze codebase structure.
```json
{
  "path": ".",
  "analysis_type": "overview",    // "imports", "symbols", "connections", "overview"
  "file_pattern": "*.go"          // Filter files by glob
}
```

#### Analysis Types:
- **`imports`**: Lists import/require statements per file
- **`symbols`**: Extracts functions, classes, interfaces, types
- **`connections`**: Maps file dependency graph and symbol cross-references
- **`overview`**: All of the above with language statistics

## 🔒 Security

- All file operations are restricted to the root directory (and explicitly allowed directories)
- Path traversal prevention via path validation
- Root directory cannot be deleted
- Binary file detection and skipping

## 📝 License

MIT