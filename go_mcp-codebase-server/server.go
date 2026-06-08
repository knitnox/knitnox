package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// =============================================================================
// Helper functions
// =============================================================================

func getAbsPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func validatePath(path string, rootDir string, allowedDirs []string) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(rootDir, path))
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	if strings.HasPrefix(absPath, rootDir) {
		return absPath, nil
	}
	for _, dir := range allowedDirs {
		if strings.HasPrefix(absPath, dir) {
			return absPath, nil
		}
	}
	return "", fmt.Errorf("path '%s' is outside allowed directories", path)
}

func getStringArg(args map[string]any, key string, defaultVal string) string {
	if v, ok := args[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getIntArg(args map[string]any, key string, defaultVal int) int {
	if v, ok := args[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultVal
}

func getBoolArg(args map[string]any, key string, defaultVal bool) bool {
	if v, ok := args[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// =============================================================================
// CORS Middleware
// =============================================================================

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

// =============================================================================
// Server creation and tool registration
// =============================================================================

func newMCPServer(rootDir string, allowedDirs []string) *server.MCPServer {
	mcpServer := server.NewMCPServer(
		"codebase-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// File operations
	mcpServer.AddTool(readFileTool(), makeReadFileHandler(rootDir, allowedDirs))
	mcpServer.AddTool(writeFileTool(), makeWriteFileHandler(rootDir, allowedDirs))
	mcpServer.AddTool(editFileTool(), makeEditFileHandler(rootDir, allowedDirs))
	mcpServer.AddTool(deletePathTool(), makeDeletePathHandler(rootDir, allowedDirs))
	mcpServer.AddTool(createDirectoryTool(), makeCreateDirectoryHandler(rootDir, allowedDirs))
	mcpServer.AddTool(listDirectoryTool(), makeListDirectoryHandler(rootDir, allowedDirs))

	// Search
	mcpServer.AddTool(searchCodebaseTool(), makeSearchCodebaseHandler(rootDir, allowedDirs))

	// Analysis
	mcpServer.AddTool(projectTreeTool(), makeProjectTreeHandler(rootDir, allowedDirs))
	mcpServer.AddTool(analyzeCodebaseTool(), makeAnalyzeCodebaseHandler(rootDir, allowedDirs))

	return mcpServer
}

func newStreamableHTTPHandler(mcpServer *server.MCPServer) *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(mcpServer)
}

func newSSEHandler(mcpServer *server.MCPServer) *server.SSEServer {
	return server.NewSSEServer(mcpServer)
}

// =============================================================================
// Tool definitions
// =============================================================================

func readFileTool() mcp.Tool {
	return mcp.NewTool("read_file",
		mcp.WithDescription("Read file contents with line numbers. Returns a specific line range for token efficiency. Always shows total line count so you know if there's more content. Use offset/limit for large files."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file (relative to root or absolute)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Starting line number (1-based). Default: 1"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of lines to read. Default: 2000. Use 0 for entire file."),
		),
	)
}

func writeFileTool() mcp.Tool {
	return mcp.NewTool("write_file",
		mcp.WithDescription("Create or overwrite a file with the given content. Creates parent directories if needed."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to write"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Content to write to the file"),
		),
	)
}

func editFileTool() mcp.Tool {
	return mcp.NewTool("edit_file",
		mcp.WithDescription("Make a surgical edit to a file by replacing exact text. Best for small, targeted changes. Returns the number of replacements and a preview of changed lines."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to edit"),
		),
		mcp.WithString("old_text",
			mcp.Required(),
			mcp.Description("Exact text to find and replace"),
		),
		mcp.WithString("new_text",
			mcp.Required(),
			mcp.Description("Replacement text"),
		),
		mcp.WithBoolean("replace_all",
			mcp.Description("Replace all occurrences. Default: false (first only)"),
		),
	)
}

func deletePathTool() mcp.Tool {
	return mcp.NewTool("delete_path",
		mcp.WithDescription("Delete a file or directory. Use recursive=true for directories."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to delete"),
		),
		mcp.WithBoolean("recursive",
			mcp.Description("Delete directory and all contents. Default: false"),
		),
	)
}

func createDirectoryTool() mcp.Tool {
	return mcp.NewTool("create_directory",
		mcp.WithDescription("Create a directory. Creates parent directories if needed (like mkdir -p)."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Directory path to create"),
		),
	)
}

func listDirectoryTool() mcp.Tool {
	return mcp.NewTool("list_directory",
		mcp.WithDescription("List directory contents with type indicators. Shows files (F) and directories (D) with sizes."),
		mcp.WithString("path",
			mcp.Description("Directory path. Default: root directory"),
		),
		mcp.WithBoolean("show_hidden",
			mcp.Description("Show hidden files/directories. Default: false"),
		),
	)
}

func searchCodebaseTool() mcp.Tool {
	return mcp.NewTool("search_codebase",
		mcp.WithDescription("Search codebase using regex. Returns matches with file paths, line numbers, and context lines. Token-efficient: groups results by file, limits output, skips binary files and common exclusion dirs (node_modules, .git, vendor, etc.)."),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Regex pattern to search for"),
		),
		mcp.WithString("path",
			mcp.Description("Root directory to search in. Default: root directory"),
		),
		mcp.WithString("include",
			mcp.Description("File glob patterns to include (comma-separated). E.g. '*.go,*.ts'. Default: all files"),
		),
		mcp.WithString("exclude",
			mcp.Description("Additional directory names to exclude (comma-separated). E.g. 'vendor,tmp'. Default: .git,node_modules,etc."),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of results. Default: 100"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("Lines of context around each match. Default: 2"),
		),
		mcp.WithBoolean("case_sensitive",
			mcp.Description("Case-sensitive search. Default: false"),
		),
	)
}

func projectTreeTool() mcp.Tool {
	return mcp.NewTool("project_tree",
		mcp.WithDescription("Get a tree view of the project structure. Token-efficient: uses compact format, respects .gitignore, skips common exclusion dirs, shows file sizes."),
		mcp.WithString("path",
			mcp.Description("Root directory. Default: root directory"),
		),
		mcp.WithNumber("max_depth",
			mcp.Description("Maximum depth to traverse. Default: 6"),
		),
		mcp.WithBoolean("dirs_only",
			mcp.Description("Only show directories. Default: false"),
		),
		mcp.WithBoolean("respect_gitignore",
			mcp.Description("Respect .gitignore patterns. Default: true"),
		),
	)
}

func analyzeCodebaseTool() mcp.Tool {
	return mcp.NewTool("analyze_codebase",
		mcp.WithDescription("Analyze codebase structure: imports, symbols (functions/types/classes), and file connections. Supports Go, Python, JavaScript/TypeScript, Rust, Java, and C/C++. Returns compact, structured output."),
		mcp.WithString("path",
			mcp.Description("Root directory to analyze. Default: root directory"),
		),
		mcp.WithString("analysis_type",
			mcp.Required(),
			mcp.Description("Type of analysis: 'imports' (import/require statements), 'symbols' (function/class/type definitions), 'connections' (file dependency graph), or 'overview' (all of the above)"),
		),
		mcp.WithString("file_pattern",
			mcp.Description("Only analyze files matching this glob pattern. E.g. '*.go'. Default: all source files"),
		),
	)
}

// =============================================================================
// Utility: marshal result
// =============================================================================

func marshalResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(data)), nil
}

func textResult(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

func errorResult(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(msg)
}

// =============================================================================
// Gitignore parsing
// =============================================================================

type gitignoreMatcher struct {
	patterns []string
}

func loadGitignore(root string) *gitignoreMatcher {
	gi := &gitignoreMatcher{}
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return gi
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		gi.patterns = append(gi.patterns, line)
	}
	return gi
}

func (gi *gitignoreMatcher) match(name string, isDir bool) bool {
	for _, pattern := range gi.patterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
		// Handle directory-only patterns (ending with /)
		cleanPattern := strings.TrimSuffix(pattern, "/")
		matched, _ = filepath.Match(cleanPattern, name)
		if matched {
			return true
		}
	}
	return false
}

// =============================================================================
// Common exclusion dirs
// =============================================================================

var defaultExcludeDirs = map[string]bool{
	".git":          true,
	".svn":          true,
	".hg":           true,
	"node_modules":  true,
	"vendor":        true,
	"__pycache__":   true,
	".idea":         true,
	".vscode":       true,
	"dist":          true,
	"build":         true,
	"target":        true,
	".next":         true,
	".nuxt":         true,
	".cache":        true,
	".tox":          true,
	".venv":         true,
	"venv":          true,
	"env":           true,
	".env":          true,
	".direnv":       true,
	"coverage":      true,
	".coverage":     true,
	"bower_components": true,
}

func shouldExcludeDir(name string) bool {
	return defaultExcludeDirs[name]
}

// isBinaryFile checks if a file appears to be binary
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil {
		return true
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// sourceFileExtensions identifies source code files
var sourceFileExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true,
	".jsx": true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
	".h": true, ".hpp": true, ".cs": true, ".rb": true, ".php": true,
	".swift": true, ".kt": true, ".scala": true, ".sh": true, ".bash": true,
	".zsh": true, ".fish": true, ".ps1": true, ".sql": true, ".html": true,
	".css": true, ".scss": true, ".less": true, ".xml": true, ".yaml": true,
	".yml": true, ".json": true, ".toml": true, ".ini": true, ".cfg": true,
	".conf": true, ".md": true, ".rst": true, ".txt": true, ".csv": true,
	".env": true, ".gitignore": true, ".dockerignore": true,
	".tf": true, ".hcl": true, ".proto": true, ".graphql": true,
	".vue": true, ".svelte": true, ".dart": true, ".lua": true,
	".zig": true, ".nim": true, ".ex": true, ".exs": true,
	".erl": true, ".hs": true, ".ml": true, ".fs": true,
	".clj": true, ".lisp": true, ".rkt": true, ".sol": true,
	".Makefile": true, ".mk": true,
}

func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if sourceFileExtensions[ext] {
		return true
	}
	// Check filename without extension for files like Makefile, Dockerfile
	base := filepath.Base(path)
	switch base {
	case "Makefile", "Dockerfile", "Vagrantfile", "Gemfile",
		"Rakefile", "Procfile", ".gitignore", ".env.example":
		return true
	}
	return false
}