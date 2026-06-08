package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// =============================================================================
// project_tree handler
// =============================================================================

func makeProjectTreeHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", ".")
		maxDepth := getIntArg(args, "max_depth", 6)
		dirsOnly := getBoolArg(args, "dirs_only", false)
		respectGitignore := getBoolArg(args, "respect_gitignore", true)

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Load gitignore if needed
		var gitignore *gitignoreMatcher
		if respectGitignore {
			gitignore = loadGitignore(absPath)
		}

		var sb strings.Builder
		dirName := filepath.Base(absPath)
		if dirName == "" || dirName == "." {
			dirName = filepath.Base(absPath)
		}

		fileCount, dirCount := 0, 0
		sb.WriteString(fmt.Sprintf("%s/\n", dirName))

		err = buildTree(absPath, absPath, &sb, "", maxDepth, 0, dirsOnly, gitignore, &fileCount, &dirCount)
		if err != nil {
			// Continue with partial results
		}

		sb.WriteString(fmt.Sprintf("\n📊 %d directories, %d files", dirCount, fileCount))
		return textResult(sb.String()), nil
	}
}

func buildTree(root, currentPath string, sb *strings.Builder, prefix string, maxDepth, currentDepth int, dirsOnly bool, gitignore *gitignoreMatcher, fileCount, dirCount *int) error {
	if currentDepth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return err
	}

	// Sort entries: directories first, then files
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return entries[i].Name() < entries[j].Name()
	})

	// Filter entries
	var filtered []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files/dirs
		if strings.HasPrefix(name, ".") && name != ".env" && name != ".env.example" {
			continue
		}

		// Skip common exclusion dirs
		if entry.IsDir() && shouldExcludeDir(name) {
			continue
		}

		// Skip gitignored entries
		if gitignore != nil && gitignore.match(name, entry.IsDir()) {
			continue
		}

		// Skip files if dirsOnly
		if dirsOnly && !entry.IsDir() {
			continue
		}

		filtered = append(filtered, entry)
	}

	for i, entry := range filtered {
		isLast := i == len(filtered)-1
		connector := "├── "
		nextPrefix := "│   "
		if isLast {
			connector = "└── "
			nextPrefix = "    "
		}

		name := entry.Name()

		if entry.IsDir() {
			(*dirCount)++
			sb.WriteString(fmt.Sprintf("%s%s📁 %s/\n", prefix, connector, name))

			// Limit tree depth for readability
			if currentDepth+1 < maxDepth {
				subPath := filepath.Join(currentPath, name)
				buildTree(root, subPath, sb, prefix+nextPrefix, maxDepth, currentDepth+1, dirsOnly, gitignore, fileCount, dirCount)
			} else {
				sb.WriteString(fmt.Sprintf("%s    └── ...\n", prefix+nextPrefix))
			}
		} else {
			(*fileCount)++
			info, _ := entry.Info()
			sizeStr := ""
			if info != nil {
				sizeStr = formatSize(info.Size())
			}
			sb.WriteString(fmt.Sprintf("%s%s📄 %s (%s)\n", prefix, connector, name, sizeStr))
		}
	}

	return nil
}

// =============================================================================
// analyze_codebase handler
// =============================================================================

type fileAnalysis struct {
	Path     string
	Language string
	Imports  []string
	Symbols  []symbolInfo
}

type symbolInfo struct {
	Name string
	Type string // "function", "method", "class", "interface", "struct", "type", "const", "var"
	Line int
}

func makeAnalyzeCodebaseHandler(rootDir string, allowedDirs []string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		path := getStringArg(args, "path", ".")
		analysisType := getStringArg(args, "analysis_type", "overview")
		filePattern := getStringArg(args, "file_pattern", "")

		absPath, err := validatePath(path, rootDir, allowedDirs)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Collect source files
		var files []string
		err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if shouldExcludeDir(name) || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}

			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			if !isSourceFile(path) {
				return nil
			}

			// Apply file pattern filter
			if filePattern != "" {
				matched, _ := filepath.Match(filePattern, d.Name())
				if !matched {
					return nil
				}
			}

			files = append(files, path)
			return nil
		})
		if err != nil {
			return errorResult(fmt.Sprintf("Error walking directory: %v", err)), nil
		}

		if len(files) == 0 {
			return textResult("No source files found in the specified path."), nil
		}

		// Analyze files
		var analyses []fileAnalysis
		for _, f := range files {
			analysis := analyzeFile(f, rootDir)
			if analysis != nil {
				analyses = append(analyses, *analysis)
			}
		}

		// Format output based on analysis type
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📊 Codebase Analysis: %s\n", analysisType))
		sb.WriteString(fmt.Sprintf("   %d source files analyzed\n", len(analyses)))
		sb.WriteString(strings.Repeat("═", 60) + "\n\n")

		switch analysisType {
		case "imports":
			formatImports(&sb, analyses)
		case "symbols":
			formatSymbols(&sb, analyses)
		case "connections":
			formatConnections(&sb, analyses)
		case "overview":
			formatOverview(&sb, analyses)
		default:
			formatOverview(&sb, analyses)
		}

		return textResult(sb.String()), nil
	}
}

// =============================================================================
// File analysis by language
// =============================================================================

func analyzeFile(filePath, rootDir string) *fileAnalysis {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	content := string(data)
	// Skip very large files
	if len(data) > 500000 {
		return nil
	}

	relPath, _ := filepath.Rel(rootDir, filePath)
	ext := strings.ToLower(filepath.Ext(filePath))
	language := detectLanguage(ext)

	analysis := &fileAnalysis{
		Path:     relPath,
		Language: language,
	}

	switch language {
	case "Go":
		analysis.Imports = parseGoImports(content)
		analysis.Symbols = parseGoSymbols(content)
	case "Python":
		analysis.Imports = parsePythonImports(content)
		analysis.Symbols = parsePythonSymbols(content)
	case "JavaScript", "TypeScript":
		analysis.Imports = parseJSImports(content)
		analysis.Symbols = parseJSSymbols(content)
	case "Rust":
		analysis.Imports = parseRustImports(content)
		analysis.Symbols = parseRustSymbols(content)
	case "Java":
		analysis.Imports = parseJavaImports(content)
		analysis.Symbols = parseJavaSymbols(content)
	default:
		analysis.Imports = parseGenericImports(content)
		analysis.Symbols = parseGenericSymbols(content)
	}

	return analysis
}

func detectLanguage(ext string) string {
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".js":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".jsx":
		return "JavaScript"
	case ".rs":
		return "Rust"
	case ".java":
		return "Java"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc":
		return "C++"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".swift":
		return "Swift"
	case ".kt":
		return "Kotlin"
	case ".scala":
		return "Scala"
	case ".cs":
		return "C#"
	case ".vue", ".svelte":
		return "JavaScript"
	default:
		return "Other"
	}
}

// =============================================================================
// Language-specific parsers
// =============================================================================

// --- Go ---
func parseGoImports(content string) []string {
	var imports []string
	re := regexp.MustCompile(`import\s+(?:\([\s\S]*?\)|"([^"]+)")`)
	matches := re.FindAllStringSubmatch(content, -1)

	// Handle multi-line imports
	inImportBlock := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock {
			if line == ")" {
				inImportBlock = false
				continue
			}
			impRe := regexp.MustCompile(`"([^"]+)"`)
			m := impRe.FindStringSubmatch(line)
			if len(m) > 1 {
				imports = append(imports, m[1])
			}
		}
	}

	// Handle single-line imports
	singleRe := regexp.MustCompile(`^import\s+"([^"]+)"`)
	for _, line := range strings.Split(content, "\n") {
		m := singleRe.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) > 1 {
			// Check if not already in block imports
			found := false
			for _, imp := range imports {
				if imp == m[1] {
					found = true
					break
				}
			}
			if !found {
				imports = append(imports, m[1])
			}
		}
	}

	_ = matches // fallback: also collect from submatches
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			imports = appendIfNotExists(imports, match[1])
		}
	}

	return imports
}

func parseGoSymbols(content string) []symbolInfo {
	var symbols []symbolInfo
	lineNum := 0

	for _, line := range strings.Split(content, "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)

		// Functions
		if m := regexp.MustCompile(`^func\s+(?:\([\w\*]+\s+\w+\)\s+)?(\w+)\s*\(`).FindStringSubmatch(trimmed); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "function", Line: lineNum})
		}
		// Types (struct, interface)
		if m := regexp.MustCompile(`^type\s+(\w+)\s+(struct|interface)\s*\{?`).FindStringSubmatch(trimmed); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: m[2], Line: lineNum})
		}
		// Type aliases
		if m := regexp.MustCompile(`^type\s+(\w+)\s+=`).FindStringSubmatch(trimmed); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "type", Line: lineNum})
		}
		// Constants
		if m := regexp.MustCompile(`^(?:const|var)\s+(\w+)\s`).FindStringSubmatch(trimmed); len(m) > 1 {
			if !strings.HasPrefix(trimmed, "//") {
				symbols = append(symbols, symbolInfo{Name: m[1], Type: "const", Line: lineNum})
			}
		}
	}

	return symbols
}

// --- Python ---
func parsePythonImports(content string) []string {
	var imports []string
	re := regexp.MustCompile(`^import\s+([\w.]+)|^from\s+([\w.]+)\s+import`)
	for _, line := range strings.Split(content, "\n") {
		m := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) > 0 {
			if m[1] != "" {
				imports = append(imports, m[1])
			} else if m[2] != "" {
				imports = append(imports, m[2])
			}
		}
	}
	return imports
}

func parsePythonSymbols(content string) []symbolInfo {
	var symbols []symbolInfo
	lineNum := 0
	classRe := regexp.MustCompile(`^class\s+(\w+)`)
	funcRe := regexp.MustCompile(`^def\s+(\w+)`)

	for _, line := range strings.Split(content, "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if m := classRe.FindStringSubmatch(trimmed); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "class", Line: lineNum})
		} else if m := funcRe.FindStringSubmatch(trimmed); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "function", Line: lineNum})
		}
	}
	return symbols
}

// --- JavaScript/TypeScript ---
func parseJSImports(content string) []string {
	var imports []string
	// import ... from '...' / import ... from "..."
	re := regexp.MustCompile(`import\s+.*?(?:from|['"])\s*['"]([^'"]+)['"]`)
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			imports = append(imports, m[1])
		}
	}
	// require('...')
	reqRe := regexp.MustCompile(`require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	for _, m := range reqRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			imports = appendIfNotExists(imports, m[1])
		}
	}
	return imports
}

func parseJSSymbols(content string) []symbolInfo {
	var symbols []symbolInfo
	lineNum := 0

	// Function declarations
	funcRe := regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	// Class declarations
	classRe := regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?class\s+(\w+)`)
	// Arrow function exports
	arrowRe := regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(?`)
	// Interface (TypeScript)
	ifaceRe := regexp.MustCompile(`^\s*(?:export\s+)?interface\s+(\w+)`)
	// Type (TypeScript)
	typeRe := regexp.MustCompile(`^\s*(?:export\s+)?type\s+(\w+)`)

	for _, line := range strings.Split(content, "\n") {
		lineNum++
		if m := funcRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "function", Line: lineNum})
		} else if m := classRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "class", Line: lineNum})
		} else if m := ifaceRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "interface", Line: lineNum})
		} else if m := typeRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "type", Line: lineNum})
		} else if m := arrowRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "function", Line: lineNum})
		}
	}
	return symbols
}

// --- Rust ---
func parseRustImports(content string) []string {
	var imports []string
	re := regexp.MustCompile(`use\s+([\w:]+)`)
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			imports = append(imports, m[1])
		}
	}
	return imports
}

func parseRustSymbols(content string) []symbolInfo {
	var symbols []symbolInfo
	lineNum := 0
	funcRe := regexp.MustCompile(`^\s*(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`)
	structRe := regexp.MustCompile(`^\s*(?:pub\s+)?struct\s+(\w+)`)
	enumRe := regexp.MustCompile(`^\s*(?:pub\s+)?enum\s+(\w+)`)
	traitRe := regexp.MustCompile(`^\s*(?:pub\s+)?trait\s+(\w+)`)
	implRe := regexp.MustCompile(`^\s*impl\s+(?:<[^>]+>\s+)?(\w+)`)

	for _, line := range strings.Split(content, "\n") {
		lineNum++
		if m := funcRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "function", Line: lineNum})
		} else if m := structRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "struct", Line: lineNum})
		} else if m := enumRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "enum", Line: lineNum})
		} else if m := traitRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "trait", Line: lineNum})
		} else if m := implRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: "impl " + m[1], Type: "impl", Line: lineNum})
		}
	}
	return symbols
}

// --- Java ---
func parseJavaImports(content string) []string {
	var imports []string
	re := regexp.MustCompile(`^import\s+([\w.]+)\s*;`)
	for _, line := range strings.Split(content, "\n") {
		m := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) > 1 {
			imports = append(imports, m[1])
		}
	}
	return imports
}

func parseJavaSymbols(content string) []symbolInfo {
	var symbols []symbolInfo
	lineNum := 0
	classRe := regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?(?:final\s+)?(?:abstract\s+)?class\s+(\w+)`)
	ifaceRe := regexp.MustCompile(`^\s*(?:public|private|protected)?\s*interface\s+(\w+)`)
	methodRe := regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?(?:[\w<>\[\]]+\s+)+(\w+)\s*\(`)

	for _, line := range strings.Split(content, "\n") {
		lineNum++
		trimmed := strings.TrimSpace(line)
		if m := classRe.FindStringSubmatch(trimmed); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "class", Line: lineNum})
		} else if m := ifaceRe.FindStringSubmatch(trimmed); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "interface", Line: lineNum})
		} else if m := methodRe.FindStringSubmatch(trimmed); len(m) > 1 {
			if m[1] != "if" && m[1] != "while" && m[1] != "for" && m[1] != "switch" {
				symbols = append(symbols, symbolInfo{Name: m[1], Type: "method", Line: lineNum})
			}
		}
	}
	return symbols
}

// --- Generic (fallback) ---
func parseGenericImports(content string) []string {
	var imports []string
	// Try common patterns
	patterns := []string{
		`import\s+['"]([^'"]+)['"]`,
		`import\s+([\w.]+)`,
		`require\s*\(\s*['"]([^'"]+)['"]\s*\)`,
		`use\s+([\w:]+)`,
		`#include\s+[<"]([^>"]+)[>"]`,
		`from\s+([\w.]+)\s+import`,
		`using\s+([\w.]+)`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			if len(m) > 1 {
				imports = appendIfNotExists(imports, m[1])
			}
		}
	}
	return imports
}

func parseGenericSymbols(content string) []symbolInfo {
	var symbols []symbolInfo
	// Generic function/class detection
	funcRe := regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?(?:function|def|fn|func|sub|proc)\s+(\w+)`)
	classRe := regexp.MustCompile(`^\s*(?:export\s+)?(?:class|struct|interface|type|enum)\s+(\w+)`)
	lineNum := 0
	for _, line := range strings.Split(content, "\n") {
		lineNum++
		if m := funcRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "function", Line: lineNum})
		} else if m := classRe.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, symbolInfo{Name: m[1], Type: "class", Line: lineNum})
		}
	}
	return symbols
}

// =============================================================================
// Output formatting
// =============================================================================

func formatImports(sb *strings.Builder, analyses []fileAnalysis) {
	sb.WriteString("📦 IMPORTS\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")

	for _, a := range analyses {
		if len(a.Imports) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n  %s [%s]\n", a.Path, a.Language))
		for _, imp := range a.Imports {
			sb.WriteString(fmt.Sprintf("    ← %s\n", imp))
		}
	}
}

func formatSymbols(sb *strings.Builder, analyses []fileAnalysis) {
	sb.WriteString("🔤 SYMBOLS\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")

	for _, a := range analyses {
		if len(a.Symbols) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n  %s [%s]\n", a.Path, a.Language))
		for _, sym := range a.Symbols {
			sb.WriteString(fmt.Sprintf("    L%-4d %s %s\n", sym.Line, symbolTypeIcon(sym.Type), sym.Name))
		}
	}
}

func formatConnections(sb *strings.Builder, analyses []fileAnalysis) {
	sb.WriteString("🔗 FILE CONNECTIONS\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n\n")

	// Build a map of local file paths
	localFiles := map[string]bool{}
	for _, a := range analyses {
		localFiles[a.Path] = true
	}

	// Find connections between local files
	for _, a := range analyses {
		var localImports []string
		var externalImports []string

		for _, imp := range a.Imports {
			// Check if this import refers to another local file
			isLocal := false
			for lf := range localFiles {
				// Heuristic: check if import path contains local file path patterns
				impBase := filepath.Base(imp)
				lfBase := strings.TrimSuffix(filepath.Base(lf), filepath.Ext(lf))
				if impBase == lfBase || strings.Contains(imp, lfBase) {
					isLocal = true
					localImports = append(localImports, lf)
					break
				}
			}
			if !isLocal {
				externalImports = append(externalImports, imp)
			}
		}

		if len(localImports) > 0 || len(externalImports) > 0 {
			sb.WriteString(fmt.Sprintf("  %s\n", a.Path))
			for _, li := range localImports {
				sb.WriteString(fmt.Sprintf("    → %s (local)\n", li))
			}
			if len(externalImports) > 0 && len(externalImports) <= 10 {
				sb.WriteString(fmt.Sprintf("    ← %d external packages\n", len(externalImports)))
			} else if len(externalImports) > 10 {
				sb.WriteString(fmt.Sprintf("    ← %d external packages (truncated)\n", len(externalImports)))
			}
		}
	}

	// Also show symbol cross-references
	sb.WriteString("\n📌 SYMBOL DEFINITIONS BY FILE\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")

	// Build symbol index
	symbolIndex := map[string][]string{} // symbol name -> []file paths
	for _, a := range analyses {
		for _, sym := range a.Symbols {
			symbolIndex[sym.Name] = append(symbolIndex[sym.Name], a.Path)
		}
	}

	// Find symbols defined in multiple files or referenced across files
	for sym, files := range symbolIndex {
		if len(files) > 1 {
			sb.WriteString(fmt.Sprintf("  %s → %s\n", sym, strings.Join(files, ", ")))
		}
	}
}

func formatOverview(sb *strings.Builder, analyses []fileAnalysis) {
	// Language statistics
	langCount := map[string]int{}
	langFiles := map[string][]string{}
	for _, a := range analyses {
		langCount[a.Language]++
		langFiles[a.Language] = append(langFiles[a.Language], a.Path)
	}

	sb.WriteString("📈 LANGUAGE BREAKDOWN\n")
	sb.WriteString(strings.Repeat("─", 40) + "\n")

	// Sort by count
	var langs []string
	for lang := range langCount {
		langs = append(langs, lang)
	}
	sort.Slice(langs, func(i, j int) bool {
		return langCount[langs[i]] > langCount[langs[j]]
	})

	for _, lang := range langs {
		sb.WriteString(fmt.Sprintf("  %-12s %d files\n", lang, langCount[lang]))
	}

	sb.WriteString("\n")
	formatImports(sb, analyses)
	sb.WriteString("\n")
	formatSymbols(sb, analyses)
}

func symbolTypeIcon(t string) string {
	switch t {
	case "function", "method":
		return "ƒ"
	case "class":
		return "©"
	case "struct":
		return "◇"
	case "interface":
		return "◈"
	case "trait":
		return "◈"
	case "type":
		return "T"
	case "enum":
		return "E"
	case "const":
		return "C"
	case "var":
		return "V"
	case "impl":
		return "⊕"
	default:
		return "?"
	}
}

func appendIfNotExists(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}