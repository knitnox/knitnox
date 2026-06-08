package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// =============================================================================
// read_file handler
// =============================================================================

func makeReadFileHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", "")
		if path == "" {
			return errorResult("path is required"), nil
		}

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to read file: %v", err)), nil
		}

		lines := strings.Split(string(data), "\n")
		totalLines := len(lines)

		offset := getIntArg(args, "offset", 1)
		limit := getIntArg(args, "limit", 2000)

		if offset < 1 {
			offset = 1
		}
		if offset > totalLines {
			offset = totalLines
		}

		end := offset + limit - 1
		if limit == 0 {
			end = totalLines
		}
		if end > totalLines {
			end = totalLines
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📄 %s (%d lines total, showing lines %d-%d)\n", path, totalLines, offset, end))
		sb.WriteString(strings.Repeat("─", 60) + "\n")

		for i := offset; i <= end; i++ {
			line := lines[i-1]
			// Truncate very long lines for token efficiency
			maxLineLen := 500
			truncated := ""
			if len(line) > maxLineLen {
				truncated = " ... (truncated)"
				line = line[:maxLineLen]
			}
			sb.WriteString(fmt.Sprintf("%6d │ %s%s\n", i, line, truncated))
		}

		if end < totalLines {
			sb.WriteString(fmt.Sprintf("\n... %d more lines (use offset=%d to continue reading)", totalLines-end, end+1))
		}

		return textResult(sb.String()), nil
	}
}

// =============================================================================
// write_file handler
// =============================================================================

func makeWriteFileHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", "")
		content := getStringArg(args, "content", "")

		if path == "" {
			return errorResult("path is required"), nil
		}

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Create parent directories if needed
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errorResult(fmt.Sprintf("Failed to create directories: %v", err)), nil
		}

		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return errorResult(fmt.Sprintf("Failed to write file: %v", err)), nil
		}

		lineCount := strings.Count(content, "\n") + 1
		return textResult(fmt.Sprintf("✅ Wrote %s (%d lines, %d bytes)", path, lineCount, len(content))), nil
	}
}

// =============================================================================
// edit_file handler
// =============================================================================

func makeEditFileHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", "")
		oldText := getStringArg(args, "old_text", "")
		newText := getStringArg(args, "new_text", "")
		replaceAll := getBoolArg(args, "replace_all", false)

		if path == "" {
			return errorResult("path is required"), nil
		}
		if oldText == "" {
			return errorResult("old_text is required"), nil
		}

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to read file: %v", err)), nil
		}

		content := string(data)
		count := strings.Count(content, oldText)

		if count == 0 {
			return errorResult(fmt.Sprintf("old_text not found in %s", path)), nil
		}

		if !replaceAll && count > 1 {
			// For safety, warn if there are multiple matches and replace_all is not set
			// Still replace only the first occurrence
		}

		if replaceAll {
			content = strings.ReplaceAll(content, oldText, newText)
		} else {
			content = strings.Replace(content, oldText, newText, 1)
		}

		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return errorResult(fmt.Sprintf("Failed to write file: %v", err)), nil
		}

		replacements := count
		if !replaceAll {
			replacements = 1
		}

		// Find the changed lines for preview
		var preview strings.Builder
		newLines := strings.Split(content, "\n")
		oldLines := strings.Split(string(data), "\n")
		preview.WriteString(fmt.Sprintf("✅ Replaced %d occurrence(s) in %s\n", replacements, path))
		preview.WriteString("Changes preview:\n")

		// Find differing lines
		diffCount := 0
		maxDiff := 10
		for i := 0; i < len(oldLines) && diffCount < maxDiff; i++ {
			if i >= len(newLines) || oldLines[i] != newLines[i] {
				if i < len(newLines) {
					preview.WriteString(fmt.Sprintf("  %d: %s\n", i+1, truncate(newLines[i], 120)))
				}
				diffCount++
			}
		}
		if diffCount == maxDiff {
			preview.WriteString("  ... more changes\n")
		}

		return textResult(preview.String()), nil
	}
}

// =============================================================================
// delete_path handler
// =============================================================================

func makeDeletePathHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", "")
		recursive := getBoolArg(args, "recursive", false)

		if path == "" {
			return errorResult("path is required"), nil
		}

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Prevent deleting the root directory
		if absPath == rootDir {
			return errorResult("Cannot delete the root directory"), nil
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("Path not found: %v", err)), nil
		}

		if info.IsDir() {
			if !recursive {
				// Check if directory is empty
				entries, err := os.ReadDir(absPath)
				if err != nil {
					return errorResult(fmt.Sprintf("Failed to read directory: %v", err)), nil
				}
				if len(entries) > 0 {
					return errorResult("Directory is not empty. Use recursive=true to delete."), nil
				}
			}
			if err := os.RemoveAll(absPath); err != nil {
				return errorResult(fmt.Sprintf("Failed to delete directory: %v", err)), nil
			}
			return textResult(fmt.Sprintf("✅ Deleted directory: %s", path)), nil
		}

		if err := os.Remove(absPath); err != nil {
			return errorResult(fmt.Sprintf("Failed to delete file: %v", err)), nil
		}
		return textResult(fmt.Sprintf("✅ Deleted file: %s", path)), nil
	}
}

// =============================================================================
// create_directory handler
// =============================================================================

func makeCreateDirectoryHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", "")

		if path == "" {
			return errorResult("path is required"), nil
		}

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		if err := os.MkdirAll(absPath, 0755); err != nil {
			return errorResult(fmt.Sprintf("Failed to create directory: %v", err)), nil
		}
		return textResult(fmt.Sprintf("✅ Created directory: %s", path)), nil
	}
}

// =============================================================================
// list_directory handler
// =============================================================================

func makeListDirectoryHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", ".")
		showHidden := getBoolArg(args, "show_hidden", false)

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		entries, err := os.ReadDir(absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to read directory: %v", err)), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📂 %s/ (%d entries)\n", path, len(entries)))
		sb.WriteString(strings.Repeat("─", 60) + "\n")

		// Sort: directories first, then files, alphabetically
		sort.Slice(entries, func(i, j int) bool {
			iDir := entries[i].IsDir()
			jDir := entries[j].IsDir()
			if iDir != jDir {
				return iDir
			}
			return entries[i].Name() < entries[j].Name()
		})

		dirCount, fileCount := 0, 0
		for _, entry := range entries {
			name := entry.Name()
			if !showHidden && strings.HasPrefix(name, ".") {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			if entry.IsDir() {
				sb.WriteString(fmt.Sprintf("  📁 %-40s D\n", name+"/"))
				dirCount++
			} else {
				sizeStr := formatSize(info.Size())
				sb.WriteString(fmt.Sprintf("  📄 %-40s %s\n", name, sizeStr))
				fileCount++
			}
		}

		sb.WriteString(fmt.Sprintf("\n%d directories, %d files", dirCount, fileCount))
		return textResult(sb.String()), nil
	}
}

// =============================================================================
// Utility functions
// =============================================================================

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}