package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// =============================================================================
// search_codebase handler
// =============================================================================

type searchMatch struct {
	LineNum int
	Line    string
}

type fileMatches struct {
	Path    string
	Matches []searchMatch
	Lines   []string // full file lines for context
}

func makeSearchCodebaseHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		pattern := getStringArg(args, "pattern", "")
		searchPath := getStringArg(args, "path", ".")
		includeGlobs := getStringArg(args, "include", "")
		excludeExtra := getStringArg(args, "exclude", "")
		maxResults := getIntArg(args, "max_results", 100)
		contextLines := getIntArg(args, "context_lines", 2)
		caseSensitive := getBoolArg(args, "case_sensitive", false)

		if pattern == "" {
			return errorResult("pattern is required"), nil
		}

		absPath, err := validatePath(searchPath, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Compile regex
		flags := ""
		if !caseSensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + pattern)
		if err != nil {
			return errorResult(fmt.Sprintf("Invalid regex pattern: %v", err)), nil
		}

		// Parse include globs
		var includePatterns []string
		if includeGlobs != "" {
			for _, g := range strings.Split(includeGlobs, ",") {
				g = strings.TrimSpace(g)
				if g != "" {
					includePatterns = append(includePatterns, g)
				}
			}
		}

		// Parse extra exclude dirs
		extraExclude := map[string]bool{}
		if excludeExtra != "" {
			for _, d := range strings.Split(excludeExtra, ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					extraExclude[d] = true
				}
			}
		}

		// Search
		var results []fileMatches
		totalMatches := 0

		err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			// Skip excluded directories
			if d.IsDir() {
				name := d.Name()
				if shouldExcludeDir(name) || extraExclude[name] {
					return filepath.SkipDir
				}
				if name == "." || name == ".." {
					return nil
				}
				return nil
			}

			// Skip hidden files
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			// Check include patterns
			if len(includePatterns) > 0 {
				matched := false
				for _, pat := range includePatterns {
					ok, _ := filepath.Match(pat, d.Name())
					if ok {
						matched = true
						break
					}
				}
				if !matched {
					return nil
				}
			}

			// Skip binary files
			if isBinaryFile(path) {
				return nil
			}

			// Read and search file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			content := string(data)
			lines := strings.Split(content, "\n")
			var matches []searchMatch

			for i, line := range lines {
				if re.MatchString(line) {
					matches = append(matches, searchMatch{
						LineNum: i + 1,
						Line:    line,
					})
				}
			}

			if len(matches) > 0 {
				// Make path relative to rootDir
				relPath, _ := filepath.Rel(rootDir, path)

				results = append(results, fileMatches{
					Path:    relPath,
					Matches: matches,
					Lines:   lines,
				})
				totalMatches += len(matches)

				if totalMatches >= maxResults {
					return fmt.Errorf("max results reached")
				}
			}

			return nil
		})

		// max results reached is not a real error
		if err != nil && err.Error() != "max results reached" {
			return errorResult(fmt.Sprintf("Search error: %v", err)), nil
		}

		// Format output
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🔍 Search: /%s/", pattern))
		if !caseSensitive {
			sb.WriteString(" (case-insensitive)")
		}
		sb.WriteString(fmt.Sprintf("\n%d matches in %d files", totalMatches, len(results)))
		if totalMatches >= maxResults {
			sb.WriteString(fmt.Sprintf(" (truncated at %d results)", maxResults))
		}
		sb.WriteString("\n" + strings.Repeat("─", 60) + "\n")

		for _, fm := range results {
			if sb.Len() > 15000 { // Token limit safety
				sb.WriteString("\n... results truncated for token efficiency")
				break
			}

			sb.WriteString(fmt.Sprintf("\n📁 %s\n", fm.Path))

			for _, match := range fm.Matches {
				if sb.Len() > 15000 {
					break
				}

				// Show context lines before match
				startLine := match.LineNum - contextLines
				if startLine < 1 {
					startLine = 1
				}
				endLine := match.LineNum + contextLines
				if endLine > len(fm.Lines) {
					endLine = len(fm.Lines)
				}

				for l := startLine; l <= endLine; l++ {
					prefix := "  "
					if l == match.LineNum {
						prefix = "▶ " // Match indicator
					}
					lineContent := fm.Lines[l-1]
					if len(lineContent) > 300 {
						lineContent = lineContent[:300] + "..."
					}
					sb.WriteString(fmt.Sprintf("%s%6d │ %s\n", prefix, l, lineContent))
				}
				sb.WriteString("\n")
			}
		}

		return textResult(sb.String()), nil
	}
}