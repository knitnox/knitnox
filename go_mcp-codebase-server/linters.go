package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// =============================================================================
// Types
// =============================================================================

// Finding represents a single issue discovered by a linter.
type Finding struct {
	Tool       string `json:"tool"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column,omitempty"`
	Severity   string `json:"severity"`             // error, warning, info, hint
	Category   string `json:"category"`             // bug, security, performance, style, complexity
	Code       string `json:"code"`                 // e.g. "G104", "F821", "no-undef"
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Context    string `json:"context,omitempty"`
}

// LinterDef describes a linter tool and how to run/parse it.
type LinterDef struct {
	Name         string   // Display name
	Cmd          string   // The executable name (e.g. "golangci-lint")
	Args         []string // Arguments (excluding the target path)
	InstallHint  string   // One-liner to install
	OutputParser func(string, string) ([]Finding, error)
	// Optional: if true, append target path as last arg
	AppendTarget bool
}

// LinterResult groups findings + metadata from a single linter run.
type LinterResult struct {
	Tool     string    `json:"tool"`
	Language string    `json:"language"`
	Findings []Finding `json:"findings"`
	Status   string    `json:"status"`   // "success", "not_found", "error"
	Error    string    `json:"error,omitempty"`
}

// LintSummary provides an overview across all linters run.
type LintSummary struct {
	TotalErrors   int `json:"errors"`
	TotalWarnings int `json:"warnings"`
	TotalInfo     int `json:"info"`
	TotalHints    int `json:"hints"`
	TotalFindings int `json:"total"`
}

// =============================================================================
// Embedded ESLint default config (catches real bugs, not style preferences)
// =============================================================================

const eslintDefaultConfigJSON = `{
  "env": {
    "browser": true,
    "es2021": true,
    "node": true
  },
  "parserOptions": {
    "ecmaVersion": "latest",
    "sourceType": "module"
  },
  "rules": {
    "no-undef": "error",
    "no-unused-vars": ["warn", { "argsIgnorePattern": "^_" }],
    "no-redeclare": "error",
    "no-unreachable": "error",
    "no-constant-condition": "warn",
    "no-empty": "warn",
    "no-eval": "error",
    "no-implied-eval": "error",
    "eqeqeq": "warn",
    "no-unused-expressions": "warn",
    "no-extra-boolean-cast": "warn",
    "no-duplicate-case": "error",
    "no-func-assign": "error",
    "no-import-assign": "error",
    "no-dupe-keys": "error",
    "no-dupe-args": "error",
    "no-duplicate-imports": "warn",
    "no-self-assign": "warn",
    "no-self-compare": "warn",
    "no-throw-literal": "warn",
    "no-useless-catch": "warn",
    "no-useless-escape": "warn",
    "no-var": "warn",
    "prefer-const": "warn",
    "prefer-template": "info",
    "no-console": "info"
  }
}`

// =============================================================================
// Linter registry
// =============================================================================

// linterRegistry maps language names to their recommended linter(s).
// Multiple linters per language are supported (run in order).
var linterRegistry = map[string][]LinterDef{
	"Go": {
		{
			Name:        "golangci-lint",
			Cmd:         "golangci-lint",
			Args:        []string{"run", "--out-format=json"},
			InstallHint: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest",
			OutputParser: parseGolangciLintOutput,
			AppendTarget: true,
		},
		{
			Name:        "go vet",
			Cmd:         "go",
			Args:        []string{"vet"},
			InstallHint: "Part of the Go toolchain (pre-installed with Go)",
			OutputParser: parseGoVetOutput,
			AppendTarget: true,
		},
	},
	"Python": {
		{
			Name:        "ruff",
			Cmd:         "ruff",
			Args:        []string{"check", "--output-format=json"},
			InstallHint: "pip install ruff",
			OutputParser: parseRuffOutput,
			AppendTarget: true,
		},
		{
			Name:        "pylint",
			Cmd:         "pylint",
			Args:        []string{"--output-format=json"},
			InstallHint: "pip install pylint",
			OutputParser: parsePylintJSONOutput,
			AppendTarget: true,
		},
	},
	"JavaScript": {
		{
			Name:        "eslint",
			Cmd:         "eslint",
			Args:        []string{"--format", "json", "--no-eslintrc", "--config"},
			InstallHint: "npm install -g eslint",
			OutputParser: parseESLintOutput,
			AppendTarget: true,
		},
	},
	"TypeScript": {
		{
			Name:        "eslint",
			Cmd:         "eslint",
			Args:        []string{"--format", "json", "--no-eslintrc", "--config"},
			InstallHint: "npm install -g eslint",
			OutputParser: parseESLintOutput,
			AppendTarget: true,
		},
		{
			Name:        "tsc",
			Cmd:         "tsc",
			Args:        []string{"--noEmit", "--pretty", "false"},
			InstallHint: "npm install -g typescript",
			OutputParser: parseTSCErrorOutput,
			AppendTarget: true,
		},
	},
	"Rust": {
		{
			Name:        "clippy",
			Cmd:         "cargo",
			Args:        []string{"clippy", "--message-format=json"},
			InstallHint: "rustup component add clippy",
			OutputParser: parseClippyOutput,
			AppendTarget: false, // Must run from Cargo.toml directory
		},
	},
	"C": {
		{
			Name:        "cppcheck",
			Cmd:         "cppcheck",
			Args:        []string{"--template={file}:{line}:{column}:{severity}:{message}"},
			InstallHint: "apt install cppcheck  OR  brew install cppcheck",
			OutputParser: parseCppcheckOutput,
			AppendTarget: true,
		},
	},
	"C++": {
		{
			Name:        "cppcheck",
			Cmd:         "cppcheck",
			Args:        []string{"--template={file}:{line}:{column}:{severity}:{message}", "--language=c++"},
			InstallHint: "apt install cppcheck  OR  brew install cppcheck",
			OutputParser: parseCppcheckOutput,
			AppendTarget: true,
		},
	},
	"Java": {
		{
			Name:        "checkstyle",
			Cmd:         "checkstyle",
			Args:        []string{"-c", "/sun_checks.xml", "-f", "json"},
			InstallHint: "Download from: https://checkstyle.org/download.html",
			OutputParser: parseCheckstyleOutput,
			AppendTarget: true,
		},
	},
	"Ruby": {
		{
			Name:        "rubocop",
			Cmd:         "rubocop",
			Args:        []string{"--format", "json", "--force-default-config"},
			InstallHint: "gem install rubocop",
			OutputParser: parseRubocopOutput,
			AppendTarget: true,
		},
	},
	"PHP": {
		{
			Name:        "phpstan",
			Cmd:         "phpstan",
			Args:        []string{"analyse", "--level=0", "--error-format=json"},
			InstallHint: "composer global require phpstan/phpstan",
			OutputParser: parsePHPStanOutput,
			AppendTarget: true,
		},
	},
}

// htmlCSSLinters are run separately when HTML/CSS files are found.
var htmlCSSLinters = []LinterDef{
	{
		Name:        "htmlhint",
		Cmd:         "htmlhint",
		Args:        []string{"--format=json"},
		InstallHint: "npm install -g htmlhint",
		OutputParser: parseHTMLHintOutput,
		AppendTarget: true,
	},
	{
		Name:        "stylelint",
		Cmd:         "stylelint",
		Args:        []string{"--formatter=json"},
		InstallHint: "npm install -g stylelint stylelint-config-standard",
		OutputParser: parseStylelintOutput,
		AppendTarget: true,
	},
}

// =============================================================================
// Tool definition
// =============================================================================

func findBugsTool() mcp.Tool {
	return mcp.NewTool("find_bugs",
		mcp.WithDescription("Run static analysis on a project or file to find bugs, vulnerabilities, and code issues before runtime. Auto-detects language and uses the appropriate linter."),
		mcp.WithString("path",
			mcp.Description("File or directory to analyze. Default: root directory"),
		),
		mcp.WithString("severity",
			mcp.Description("Minimum severity: 'critical', 'error', 'warning', 'info', or 'hint'. Default: 'info' (all findings)"),
		),
		mcp.WithString("linter",
			mcp.Description("Specific linter to use (e.g. 'golangci-lint', 'eslint', 'ruff'). Default: auto-detect based on language"),
		),
		mcp.WithString("categories",
			mcp.Description("Comma-separated list: 'bug,security,performance,style,complexity,all'. Default: 'all'"),
		),
	)
}

// =============================================================================
// Handler factory
// =============================================================================

func makeFindBugsHandler(rootDir string, allowedDirs []string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()

		targetPath := getStringArg(args, "path", rootDir)
		severityFilter := getStringArg(args, "severity", "info")
		requestedLinter := getStringArg(args, "linter", "")
		categoriesStr := getStringArg(args, "categories", "all")

		// Validate and resolve path
		absPath, err := validatePath(targetPath, rootDir, allowedDirs)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid path: %v", err)), nil
		}

		startTime := time.Now()

		// Determine if path is a file or directory
		info, err := os.Stat(absPath)
		if err != nil {
			return errorResult(fmt.Sprintf("cannot access path: %v", err)), nil
		}

		// Parse category filter
		categorySet := parseCategoryFilter(categoriesStr)

		var results []LinterResult
		var allFindings []Finding

		if info.IsDir() {
			// Directory mode: detect project language and run appropriate linters
			results = analyzeDirectory(absPath, rootDir, requestedLinter)
		} else {
			// Single file mode: detect language from extension
			results = lintFile(absPath, rootDir, requestedLinter)
		}

		// Collect and filter findings
		for _, res := range results {
			for _, f := range res.Findings {
				// Filter by severity
				if !meetsSeverityThreshold(f.Severity, severityFilter) {
					continue
				}
				// Filter by category
				if !matchesCategoryFilter(f.Category, categorySet) {
					continue
				}
				allFindings = append(allFindings, f)
			}
		}

		// Build summary
		summary := LintSummary{}
		for _, f := range allFindings {
			switch f.Severity {
			case "error", "critical":
				summary.TotalErrors++
			case "warning":
				summary.TotalWarnings++
			case "info":
				summary.TotalInfo++
			case "hint":
				summary.TotalHints++
			}
		}
		summary.TotalFindings = len(allFindings)

		// Collect notes about which tools ran / were missing
		var notes []string
		var toolsUsed []string
		for _, res := range results {
			toolsUsed = append(toolsUsed, res.Tool)
			if res.Status == "not_found" {
				notes = append(notes, fmt.Sprintf("%s not installed. Install with appropriate package manager.", res.Tool))
			} else if res.Status == "error" && res.Error != "" {
				notes = append(notes, fmt.Sprintf("%s encountered an issue: %s", res.Tool, res.Error))
			}
		}

		// If no linters could run at all, give actionable guidance
		if len(allFindings) == 0 && len(notes) == 0 && len(results) == 0 {
			notes = append(notes, "No applicable linter found for this project. Supported languages: Go, Python, JavaScript, TypeScript, Rust, C/C++, Java, Ruby, PHP, HTML, CSS.")
		}

		if len(allFindings) == 0 && len(notes) > 0 {
			// Only notes, no findings — suggest installation
		}

		elapsed := time.Since(startTime)

		result := map[string]any{
			"findings":     allFindings,
			"summary":      summary,
			"tools_used":   toolsUsed,
			"duration_ms":  elapsed.Milliseconds(),
			"notes":        notes,
			"analyzed_at":  filepath.Base(absPath),
		}

		return marshalResult(result)
	}
}

// =============================================================================
// Analysis dispatchers
// =============================================================================

// analyzeDirectory detects the project language and runs appropriate linters.
func analyzeDirectory(dirPath string, rootDir string, requestedLinter string) []LinterResult {
	// Step 1: Detect primary language(s) in the project
	langs := detectLanguagesInDir(dirPath)

	if len(langs) == 0 {
		return nil
	}

	var results []LinterResult

	// Step 2: Check for HTML/CSS files separately (they use different linters)
	hasHTML := false
	hasCSS := false
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".html" || ext == ".htm" {
			hasHTML = true
		}
		if ext == ".css" {
			hasCSS = true
		}
		return nil
	})

	// Step 3: Run language-specific linters
	for _, lang := range langs {
		linters, ok := linterRegistry[lang]
		if !ok {
			continue
		}

		if requestedLinter != "" {
			// Filter to requested linter only
			for _, l := range linters {
				if l.Name == requestedLinter {
					result := runSingleLinter(l, dirPath, lang, rootDir)
					results = append(results, result)
				}
			}
		} else {
			for _, l := range linters {
				result := runSingleLinter(l, dirPath, lang, rootDir)
				results = append(results, result)
			}
		}
	}

	// Step 4: Run HTML/CSS linters if applicable
	if hasHTML || hasCSS {
		for _, l := range htmlCSSLinters {
			if requestedLinter != "" && l.Name != requestedLinter {
				continue
			}
			if (l.Name == "htmlhint" && !hasHTML) ||
				(l.Name == "stylelint" && !hasCSS) {
				continue
			}
			result := runSingleLinter(l, dirPath, "HTML/CSS", rootDir)
			results = append(results, result)
		}
	}

	return results
}

// lintFile detects the language of a single file and runs the appropriate linter.
func lintFile(filePath string, rootDir string, requestedLinter string) []LinterResult {
	ext := strings.ToLower(filepath.Ext(filePath))
	lang := detectLanguage(ext)

	if lang == "Other" {
		return nil
	}

	var results []LinterResult

	// Check if it's HTML or CSS
	if ext == ".html" || ext == ".htm" {
		for _, l := range htmlCSSLinters {
			if l.Name == "htmlhint" {
				if requestedLinter == "" || requestedLinter == l.Name {
					result := runSingleLinter(l, filePath, "HTML", rootDir)
					results = append(results, result)
				}
			}
		}
		return results
	}
	if ext == ".css" {
		for _, l := range htmlCSSLinters {
			if l.Name == "stylelint" {
				if requestedLinter == "" || requestedLinter == l.Name {
					result := runSingleLinter(l, filePath, "CSS", rootDir)
					results = append(results, result)
				}
			}
		}
		return results
	}

	// Language-specific linters
	linters, ok := linterRegistry[lang]
	if !ok {
		return nil
	}

	if requestedLinter != "" {
		for _, l := range linters {
			if l.Name == requestedLinter {
				result := runSingleLinter(l, filePath, lang, rootDir)
				results = append(results, result)
			}
		}
	} else {
		for _, l := range linters {
			result := runSingleLinter(l, filePath, lang, rootDir)
			results = append(results, result)
		}
	}

	return results
}

// =============================================================================
// Single linter runner
// =============================================================================

func runSingleLinter(linter LinterDef, targetPath string, language string, rootDir string) LinterResult {
	result := LinterResult{
		Tool:     linter.Name,
		Language: language,
		Status:   "success",
	}

	// Check if tool is installed
	if !isToolInstalled(linter.Cmd) {
		result.Status = "not_found"
		result.Error = fmt.Sprintf("'%s' not found. %s", linter.Cmd, linter.InstallHint)
		return result
	}

	// Build command arguments
	args := make([]string, len(linter.Args))
	copy(args, linter.Args)

	// Special handling: eslint needs a config file passed after --config
	if linter.Name == "eslint" {
		// Write temp config file
		tmpDir := os.TempDir()
		configPath := filepath.Join(tmpDir, ".eslintrc-findbugs.json")
		if err := os.WriteFile(configPath, []byte(eslintDefaultConfigJSON), 0644); err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("failed to write eslint config: %v", err)
			return result
		}
		defer os.Remove(configPath)

		// Replace "--config" placeholder with actual config path
		for i, a := range args {
			if a == "--config" && i+1 < len(args) {
				// Insert config path after --config
				args = append(args[:i+1], append([]string{configPath}, args[i+1:]...)...)
				break
			}
		}

		if linter.AppendTarget {
			args = append(args, targetPath)
		}
	} else if linter.Name == "clippy" {
		// clippy must run from the directory containing Cargo.toml
		// We'll set the working directory below
	} else if linter.AppendTarget {
		args = append(args, targetPath)
	}

	// Determine working directory
	workDir := rootDir
	if linter.Name == "clippy" {
		// Find the directory containing Cargo.toml
		cargoDir := findCargoDir(targetPath)
		if cargoDir != "" {
			workDir = cargoDir
		}
	} else if isDir(targetPath) {
		workDir = targetPath
	}

	// Run the command
	cmd := exec.Command(linter.Cmd, args...)
	cmd.Dir = workDir

	// Set timeout (5 minutes max for linters)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(ctx, linter.Cmd, args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Handle timeout
	if ctx.Err() != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("linter timed out after 5 minutes")
		return result
	}

	// Even if exit code != 0, we still parse the output (linters return non-zero on findings)
	findings, parseErr := linter.OutputParser(outputStr, targetPath)
	if parseErr != nil {
		// If parsing fails, try generic parser as fallback
		genericFindings := parseGenericLinterOutput(outputStr, linter.Name)
		if len(genericFindings) > 0 {
			result.Findings = genericFindings
		} else {
			result.Status = "error"
			result.Error = fmt.Sprintf("failed to parse output: %v", parseErr)
			if len(outputStr) > 0 {
				result.Error += "\nRaw output: " + truncateString(outputStr, 500)
			}
		}
	} else {
		result.Findings = findings
	}

	// If findings are empty but there was output and no error, still note it
	if len(result.Findings) == 0 && result.Status == "success" && err != nil {
		// Exit code error but no findings — could be a real error
		if !isLinterExitCode(err) {
			result.Status = "error"
			result.Error = fmt.Sprintf("execution error: %v", truncateString(outputStr, 300))
		}
	}

	return result
}

// =============================================================================
// Language detection (directory-level)
// =============================================================================

// detectLanguagesInDir scans a directory and returns the detected programming languages.
func detectLanguagesInDir(dirPath string) []string {
	langCount := make(map[string]int)

	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip vendor/build directories
		rel, _ := filepath.Rel(dirPath, path)
		if rel != "" {
			parts := strings.Split(rel, string(filepath.Separator))
			for _, part := range parts {
				if shouldExcludeDir(part) {
					return nil
				}
			}
		}
		if !isSourceFile(path) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		lang := detectLanguage(ext)
		if lang != "Other" {
			langCount[lang]++
		}
		return nil
	})

	// Also check for config files that indicate the primary language
	if hasFile(dirPath, "go.mod") || hasFile(dirPath, "go.sum") {
		langCount["Go"] += 5 // Boost Go if go.mod exists
	}
	if hasFile(dirPath, "Cargo.toml") || hasFile(dirPath, "Cargo.lock") {
		langCount["Rust"] += 5
	}
	if hasFile(dirPath, "package.json") {
		// Could be JS or TS — check for tsconfig.json
		if hasFile(dirPath, "tsconfig.json") {
			langCount["TypeScript"] += 5
		} else {
			langCount["JavaScript"] += 5
		}
	}
	if hasFile(dirPath, "pom.xml") || hasFile(dirPath, "build.gradle") {
		langCount["Java"] += 5
	}
	if hasFile(dirPath, "requirements.txt") || hasFile(dirPath, "setup.py") || hasFile(dirPath, "pyproject.toml") {
		langCount["Python"] += 5
	}
	if hasFile(dirPath, "Gemfile") {
		langCount["Ruby"] += 5
	}
	if hasFile(dirPath, "composer.json") {
		langCount["PHP"] += 5
	}
	if hasFile(dirPath, "CMakeLists.txt") || hasFile(dirPath, "Makefile") {
		// Could be C/C++: check for .c or .cpp files
		// Already counted by file walk above
	}

	// Sort by count descending and return top languages
	type langEntry struct {
		name  string
		count int
	}
	var entries []langEntry
	for name, count := range langCount {
		entries = append(entries, langEntry{name, count})
	}
	// Sort descending
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].count > entries[i].count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var result []string
	for _, e := range entries {
		if e.count > 0 {
			result = append(result, e.name)
		}
	}
	return result
}

// =============================================================================
// Output parsers
// =============================================================================

// --- golangci-lint JSON parser ---

type golangciLintIssue struct {
	FromLinter string `json:"FromLinter"`
	Text       string `json:"Text"`
	Severity   string `json:"Severity"`
	Pos        struct {
		Filename string `json:"Filename"`
		Line     int    `json:"Line"`
		Column   int    `json:"Column"`
	} `json:"Pos"`
}

type golangciLintOutput struct {
	Issues []golangciLintIssue `json:"Issues"`
}

func parseGolangciLintOutput(output string, targetPath string) ([]Finding, error) {
	var parsed golangciLintOutput
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return nil, fmt.Errorf("golangci-lint JSON parse error: %w", err)
	}

	var findings []Finding
	for _, issue := range parsed.Issues {
		category := categorizeGoLinter(issue.FromLinter)
		severity := normalizeSeverity(issue.Severity)
		if severity == "" {
			severity = "warning"
		}

		f := Finding{
			Tool:     "golangci-lint",
			File:     issue.Pos.Filename,
			Line:     issue.Pos.Line,
			Column:   issue.Pos.Column,
			Severity: severity,
			Category: category,
			Code:     issue.FromLinter,
			Message:  issue.Text,
		}
		findings = append(findings, f)
	}
	return findings, nil
}

func categorizeGoLinter(linterName string) string {
	switch linterName {
	case "gosec", "errcheck":
		return "security"
	case "govet", "staticcheck":
		return "bug"
	case "ineffassign", "deadcode", "unused":
		return "bug"
	case "gocyclo", "cyclop":
		return "complexity"
	case "gofmt", "gofumpt", "goimports":
		return "style"
	default:
		return "warning"
	}
}

// --- go vet text parser ---

var goVetLineRe = regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*(.+)$`)

func parseGoVetOutput(output string, targetPath string) ([]Finding, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var findings []Finding
	for scanner.Scan() {
		line := scanner.Text()
		matches := goVetLineRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		f := Finding{
			Tool:     "go vet",
			File:     matches[1],
			Line:     parseInt(matches[2]),
			Column:   parseInt(matches[3]),
			Severity: "warning",
			Category: "bug",
			Code:     "vet",
			Message:  matches[4],
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// --- ruff JSON parser ---

type ruffResult struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Filename string `json:"filename"`
	Location struct {
		Row    int `json:"row"`
		Column int `json:"column"`
	} `json:"location"`
	Level    string `json:"level"`
	Suggestion string `json:"suggestion,omitempty"`
}

func parseRuffOutput(output string, targetPath string) ([]Finding, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var findings []Finding
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var result ruffResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue // Skip non-JSON lines (e.g. "x files checked" summary)
		}
		if result.Filename == "" {
			continue
		}

		severity := "warning"
		switch result.Level {
		case "error":
			severity = "error"
		case "warning", "warn":
			severity = "warning"
		default:
			severity = "info"
		}

		category := categorizeRuffCode(result.Code)

		f := Finding{
			Tool:       "ruff",
			File:       result.Filename,
			Line:       result.Location.Row,
			Column:     result.Location.Column,
			Severity:   severity,
			Category:   category,
			Code:       result.Code,
			Message:    result.Message,
			Suggestion: result.Suggestion,
		}
		findings = append(findings, f)
	}
	return findings, nil
}

func categorizeRuffCode(code string) string {
	if len(code) < 1 {
		return "warning"
	}
	prefix := code[0]
	switch prefix {
	case 'F': // pyflakes errors (undefined names, unused imports, etc.)
		return "bug"
	case 'E': // pep8 errors
		return "style"
	case 'W': // pep8 warnings
		return "style"
	case 'B': // flake8-bugbear
		return "bug"
	case 'S': // flake8-security
		return "security"
	case 'T': // flake8-print
		return "style"
	case 'N': // pep8-naming
		return "style"
	case 'A': // builtins
		return "bug"
	case 'C': // complexity
		return "complexity"
	case 'D': // pydocstyle
		return "style"
	case 'R': // refactoring
		return "style"
	default:
		return "warning"
	}
}

// --- eslint JSON parser ---

type eslintMessage struct {
	RuleID   string `json:"ruleId"`
	Severity int    `json:"severity"` // 1=warn, 2=error
	Message  string `json:"message"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	FixText  string `json:"fix,omitempty"`
}

type eslintFileResult struct {
	FilePath string          `json:"filePath"`
	Messages []eslintMessage `json:"messages"`
}

func parseESLintOutput(output string, targetPath string) ([]Finding, error) {
	var results []eslintFileResult
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		return nil, fmt.Errorf("eslint JSON parse error: %w", err)
	}

	var findings []Finding
	for _, fileRes := range results {
		for _, msg := range fileRes.Messages {
			severity := "warning"
			if msg.Severity >= 2 {
				severity = "error"
			}

			category := categorizeESLintRule(msg.RuleID)

			f := Finding{
				Tool:     "eslint",
				File:     fileRes.FilePath,
				Line:     msg.Line,
				Column:   msg.Column,
				Severity: severity,
				Category: category,
				Code:     msg.RuleID,
				Message:  msg.Message,
			}
			if msg.FixText != "" {
				f.Suggestion = fmt.Sprintf("Auto-fix: %s", msg.FixText)
			}
			findings = append(findings, f)
		}
	}
	return findings, nil
}

func categorizeESLintRule(ruleID string) string {
	// Categorize based on well-known eslint rule prefixes
	if strings.HasPrefix(ruleID, "no-") {
		return "bug"
	}
	if strings.HasPrefix(ruleID, "prefer-") {
		return "style"
	}
	if strings.HasPrefix(ruleID, "@typescript-eslint/no-unsafe-") ||
		strings.HasPrefix(ruleID, "no-eval") ||
		strings.Contains(ruleID, "security") {
		return "security"
	}
	if strings.HasPrefix(ruleID, "max-") || ruleID == "complexity" {
		return "complexity"
	}
	return "warning"
}

// --- TypeScript compiler error parser ---

var tscErrorRe = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\):\s*(error|warning)\s*(TS\d+):\s*(.+)$`)

func parseTSCErrorOutput(output string, targetPath string) ([]Finding, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var findings []Finding
	for scanner.Scan() {
		line := scanner.Text()
		matches := tscErrorRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		severity := "error"
		if matches[4] == "warning" {
			severity = "warning"
		}
		f := Finding{
			Tool:     "tsc",
			File:     matches[1],
			Line:     parseInt(matches[2]),
			Column:   parseInt(matches[3]),
			Severity: severity,
			Category: "bug",
			Code:     matches[5],
			Message:  matches[6],
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// --- Clippy JSON parser ---

type clippyMessage struct {
	Reason  string `json:"$reason"`
	Message struct {
		Level    string `json:"level"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		Spans    []struct {
			FileName string `json:"file_name"`
			LineStart int  `json:"line_start"`
			ColumnStart int `json:"column_start"`
		} `json:"spans"`
	} `json:"message"`
}

func parseClippyOutput(output string, targetPath string) ([]Finding, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var findings []Finding
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg clippyMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Reason != "clippy::" && msg.Reason != "" {
			continue
		}
		if msg.Message.Level != "warning" && msg.Message.Level != "error" {
			continue
		}
		if len(msg.Message.Spans) == 0 {
			continue
		}

		span := msg.Message.Spans[0]
		severity := "warning"
		if msg.Message.Level == "error" {
			severity = "error"
		}

		category := "style"
		if strings.Contains(msg.Message.Code, "unwrap") ||
			strings.Contains(msg.Message.Code, "panic") {
			category = "bug"
		} else if strings.Contains(msg.Message.Code, "unsafe") {
			category = "security"
		} else if strings.Contains(msg.Message.Code, "clone") ||
			strings.Contains(msg.Message.Code, "perf") {
			category = "performance"
		} else if strings.Contains(msg.Message.Code, "complex") ||
			strings.Contains(msg.Message.Code, "cognitive") {
			category = "complexity"
		}

		f := Finding{
			Tool:     "clippy",
			File:     span.FileName,
			Line:     span.LineStart,
			Column:   span.ColumnStart,
			Severity: severity,
			Category: category,
			Code:     msg.Message.Code,
			Message:  msg.Message.Message,
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// --- Cppcheck text parser ---

var cppcheckRe = regexp.MustCompile(`^(.+?):(\d+):(\d+):(\w+):\s*(.+)$`)

func parseCppcheckOutput(output string, targetPath string) ([]Finding, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var findings []Finding
	for scanner.Scan() {
		line := scanner.Text()
		matches := cppcheckRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		severity := normalizeSeverity(matches[4])
		category := "warning"
		switch severity {
		case "error":
			category = "bug"
		case "performance":
			category = "performance"
		case "style", "information":
			category = "style"
		case "portability":
			category = "warning"
		}

		f := Finding{
			Tool:     "cppcheck",
			File:     matches[1],
			Line:     parseInt(matches[2]),
			Column:   parseInt(matches[3]),
			Severity: severity,
			Category: category,
			Code:     "cppcheck",
			Message:  matches[5],
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// --- HTMLHint JSON parser ---

type htmlHintResult struct {
	File  string `json:"file"`
	Messages []struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Line    int    `json:"line"`
		Col     int    `json:"col"`
		Rule    string `json:"rule"`
		Evidence string `json:"evidence,omitempty"`
	} `json:"messages"`
}

func parseHTMLHintOutput(output string, targetPath string) ([]Finding, error) {
	// HTMLHint can output JSON array or JSON object per file
	output = strings.TrimSpace(output)

	var results []htmlHintResult
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		// Try single object
		var single htmlHintResult
		if err2 := json.Unmarshal([]byte(output), &single); err2 != nil {
			return nil, fmt.Errorf("htmlhint JSON parse error: %w", err)
		}
		results = append(results, single)
	}

	var findings []Finding
	for _, fileRes := range results {
		for _, msg := range fileRes.Messages {
			severity := "warning"
			switch msg.Type {
			case "error":
				severity = "error"
			case "warning":
				severity = "warning"
			default:
				severity = "info"
			}

			f := Finding{
				Tool:     "htmlhint",
				File:     fileRes.File,
				Line:     msg.Line,
				Column:   msg.Col,
				Severity: severity,
				Category: "bug",
				Code:     msg.Rule,
				Message:  msg.Message,
				Context:  msg.Evidence,
			}
			findings = append(findings, f)
		}
	}
	return findings, nil
}

// --- Stylelint JSON parser ---

type stylelintResult struct {
	Source string `json:"source"`
	Warnings []struct {
		Rule    string `json:"rule"`
		Severity string `json:"severity"`
		Text    string `json:"text"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
	} `json:"warnings"`
}

func parseStylelintOutput(output string, targetPath string) ([]Finding, error) {
	var results []stylelintResult
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		return nil, fmt.Errorf("stylelint JSON parse error: %w", err)
	}

	var findings []Finding
	for _, fileRes := range results {
		for _, warn := range fileRes.Warnings {
			severity := warn.Severity
			if severity == "" {
				severity = "warning"
			}

			f := Finding{
				Tool:     "stylelint",
				File:     fileRes.Source,
				Line:     warn.Line,
				Column:   warn.Column,
				Severity: severity,
				Category: "style",
				Code:     warn.Rule,
				Message:  warn.Text,
			}
			findings = append(findings, f)
		}
	}
	return findings, nil
}

// --- Pylint JSON parser ---

type pylintMessage struct {
	Type    string `json:"type"`
	Module  string `json:"module"`
	Obj     string `json:"obj"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	EndLine int    `json:"endLine"`
	EndColumn int `json:"endColumn"`
	Path    string `json:"path"`
	Symbol  string `json:"symbol"`
	Message string `json:"message"`
}

func parsePylintJSONOutput(output string, targetPath string) ([]Finding, error) {
	var parsed struct {
		Messages []pylintMessage `json:"messages"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return nil, fmt.Errorf("pylint JSON parse error: %w", err)
	}

	var findings []Finding
	for _, msg := range parsed.Messages {
		severity := normalizeSeverity(msg.Type)
		category := "style"
		switch msg.Symbol {
		case "undefined-variable", "unused-import", "unused-variable",
			"used-before-assignment", "redefined-builtin":
			category = "bug"
		case "dangerous-default-value", "exec-used", "eval-used":
			category = "security"
		case "too-many-arguments", "too-many-branches", "too-many-locals",
			"too-many-statements", "too-many-return-statements":
			category = "complexity"
		}

		f := Finding{
			Tool:     "pylint",
			File:     msg.Path,
			Line:     msg.Line,
			Column:   msg.Column,
			Severity: severity,
			Category: category,
			Code:     msg.Symbol,
			Message:  msg.Message,
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// --- Rubocop JSON parser ---

type rubocopOffense struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	CopName  string `json:"cop_name"`
	Location struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"location"`
}

type rubocopFile struct {
	Path     string           `json:"path"`
	Offenses []rubocopOffense `json:"offenses"`
}

type rubocopResult struct {
	Files []rubocopFile `json:"files"`
}

func parseRubocopOutput(output string, targetPath string) ([]Finding, error) {
	var result rubocopResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("rubocop JSON parse error: %w", err)
	}

	var findings []Finding
	for _, file := range result.Files {
		for _, off := range file.Offenses {
			severity := normalizeSeverity(off.Severity)
			category := "style"
			if off.Severity == "error" || off.Severity == "fatal" {
				category = "bug"
			}

			f := Finding{
				Tool:     "rubocop",
				File:     file.Path,
				Line:     off.Location.Line,
				Column:   off.Location.Column,
				Severity: severity,
				Category: category,
				Code:     off.CopName,
				Message:  off.Message,
			}
			findings = append(findings, f)
		}
	}
	return findings, nil
}

// --- PHPStan JSON parser ---

type phpstanFileError struct {
	Message  string `json:"message"`
	Line     int    `json:"line"`
	LineFrom int    `json:"line_from"`
	LineTo   int    `json:"line_to"`
	Type     string `json:"type"`
}

type phpstanResult struct {
	Files  map[string][]phpstanFileError `json:"files"`
	Errors []string                     `json:"errors"`
}

func parsePHPStanOutput(output string, targetPath string) ([]Finding, error) {
	var result phpstanResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("phpstan JSON parse error: %w", err)
	}

	var findings []Finding
	for filePath, errors := range result.Files {
		for _, err := range errors {
			severity := "error"
			category := "bug"

			f := Finding{
				Tool:     "phpstan",
				File:     filePath,
				Line:     err.Line,
				Severity: severity,
				Category: category,
				Code:     err.Type,
				Message:  err.Message,
			}
			findings = append(findings, f)
		}
	}
	return findings, nil
}

// --- Checkstyle JSON parser ---

type checkstyleError struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

type checkstyleFile struct {
	Name   string           `json:"name"`
	Errors []checkstyleError `json:"errors"`
}

type checkstyleResult struct {
	Files []checkstyleFile `json:"files"`
}

func parseCheckstyleOutput(output string, targetPath string) ([]Finding, error) {
	var result checkstyleResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("checkstyle JSON parse error: %w", err)
	}

	var findings []Finding
	for _, file := range result.Files {
		for _, err := range file.Errors {
			severity := normalizeSeverity(err.Severity)
			category := "style"
			if severity == "error" {
				category = "bug"
			}

			f := Finding{
				Tool:     "checkstyle",
				File:     file.Name,
				Line:     err.Line,
				Column:   err.Column,
				Severity: severity,
				Category: category,
				Code:     err.Source,
				Message:  err.Message,
			}
			findings = append(findings, f)
		}
	}
	return findings, nil
}

// --- Generic fallback parser (file:line:col: message pattern) ---

var genericLinterRe = regexp.MustCompile(`^(?:\[?)?(.+?)(?:\]?)?:(\d+)(?::(\d+))?:\s*(.+)$`)

func parseGenericLinterOutput(output string, toolName string) []Finding {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var findings []Finding
	seen := make(map[string]bool)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		matches := genericLinterRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		file := matches[1]
		lineNum := parseInt(matches[2])
		colNum := 0
		message := matches[4]
		if matches[3] != "" {
			colNum = parseInt(matches[3])
		}

		// Deduplicate
		key := fmt.Sprintf("%s:%d:%d:%s", file, lineNum, colNum, message)
		if seen[key] {
			continue
		}
		seen[key] = true

		f := Finding{
			Tool:     toolName,
			File:     file,
			Line:     lineNum,
			Column:   colNum,
			Severity: "warning",
			Category: "warning",
			Code:     toolName,
			Message:  message,
		}
		findings = append(findings, f)
	}
	return findings
}

// =============================================================================
// Utility functions
// =============================================================================

// isToolInstalled checks if a command is available on PATH.
func isToolInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// isDir checks if a path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// hasFile checks if a file exists in a directory.
func hasFile(dir string, filename string) bool {
	path := filepath.Join(dir, filename)
	_, err := os.Stat(path)
	return err == nil
}

// findCargoDir walks up from the given path to find a Cargo.toml.
func findCargoDir(path string) string {
	dir := path
	if !isDir(dir) {
		dir = filepath.Dir(dir)
	}
	// Walk up at most 5 levels
	for i := 0; i < 5; i++ {
		if hasFile(dir, "Cargo.toml") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// parseInt safely converts a string to int.
func parseInt(s string) int {
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		} else {
			break
		}
	}
	return n
}

// normalizeSeverity maps linter-specific severity strings to our standard set.
func normalizeSeverity(s string) string {
	switch strings.ToLower(s) {
	case "error", "critical", "fatal", "high":
		return "error"
	case "warning", "warn", "medium":
		return "warning"
	case "info", "information", "note", "low":
		return "info"
	case "hint", "style", "refactor", "convention":
		return "hint"
	case "performance":
		return "warning"
	case "portability":
		return "info"
	default:
		return "warning"
	}
}

// meetsSeverityThreshold checks if a severity meets the minimum threshold.
func meetsSeverityThreshold(findingSeverity string, threshold string) bool {
	levels := []string{"hint", "info", "warning", "error", "critical"}
	findingIdx := -1
	thresholdIdx := -1

	for i, level := range levels {
		if level == findingSeverity {
			findingIdx = i
		}
		if level == threshold {
			thresholdIdx = i
		}
	}

	if thresholdIdx == -1 {
		return true // Unknown threshold, show everything
	}
	if findingIdx == -1 {
		return true // Unknown severity, show it
	}
	return findingIdx >= thresholdIdx
}

// parseCategoryFilter parses a comma-separated category filter string.
func parseCategoryFilter(categoriesStr string) map[string]bool {
	result := make(map[string]bool)
	if categoriesStr == "" || categoriesStr == "all" {
		return nil // nil means "all categories"
	}
	for _, cat := range strings.Split(categoriesStr, ",") {
		result[strings.TrimSpace(cat)] = true
	}
	return result
}

// matchesCategoryFilter checks if a finding's category matches the filter.
func matchesCategoryFilter(category string, filter map[string]bool) bool {
	if filter == nil {
		return true // No filter = show all
	}
	return filter[category]
}

// isLinterExitCode returns true if the error is a non-zero exit from a linter
// (which is normal — it means issues were found).
func isLinterExitCode(err error) bool {
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode() > 0 && exitErr.ExitCode() <= 128
	}
	return false
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// init registers the platform-specific shell detection for linting.
func init() {
	// Ensure that exec.LookPath works correctly on all platforms
	_ = runtime.GOOS
}
