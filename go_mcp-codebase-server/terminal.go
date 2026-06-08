package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

type ProcessInfo struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	PID       int       `json:"pid"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"`
	Output    string    `json:"output"`
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	mu        sync.Mutex
}

var (
	processes   = make(map[string]*ProcessInfo)
	processesMu sync.RWMutex
)

func runCommandTool() mcp.Tool {
	return mcp.NewTool("run_terminal_command",
		mcp.WithDescription("Run a terminal command asynchronously. Returns a process ID that can be used to check status or terminate. Output is captured and can be retrieved."),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The shell command to run"),
		),
		mcp.WithString("cwd",
			mcp.Description("Current working directory for the command. Default: server root"),
		),
		mcp.WithBoolean("wait",
			mcp.Description("If true, wait for completion and return full output. Default: false (background)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds. Default: 300 (5 minutes)"),
		),
		mcp.WithNumber("max_lines",
			mcp.Description("Maximum number of recent lines to return if waiting. Default: 100"),
		),
	)
}

func terminateCommandTool() mcp.Tool {
	return mcp.NewTool("terminate_command",
		mcp.WithDescription("Terminate a running process by its ID."),
		mcp.WithString("process_id",
			mcp.Required(),
			mcp.Description("The ID returned by run_terminal_command"),
		),
	)
}

func listProcessesTool() mcp.Tool {
	return mcp.NewTool("list_processes",
		mcp.WithDescription("List all background processes managed by the server and their current status."),
	)
}

func getProcessOutputTool() mcp.Tool {
	return mcp.NewTool("get_process_output",
		mcp.WithDescription("Get the latest output (stdout/stderr) from a background process."),
		mcp.WithString("process_id",
			mcp.Required(),
			mcp.Description("The ID returned by run_terminal_command"),
		),
		mcp.WithNumber("max_lines",
			mcp.Description("Maximum number of recent lines to return. Default: 100"),
		),
	)
}

func limitLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return text
	}
	lines := bufio.NewScanner(strings.NewReader(text))
	var allLines []string
	for lines.Scan() {
		allLines = append(allLines, lines.Text())
	}
	if len(allLines) <= maxLines {
		return text
	}
	return strings.Join(allLines[len(allLines)-maxLines:], "\n")
}

func makeRunCommandHandler(rootDir string, allowedDirs []string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		commandStr := getStringArg(args, "command", "")
		cwd := getStringArg(args, "cwd", rootDir)
		wait := getBoolArg(args, "wait", false)
		timeoutSec := getIntArg(args, "timeout", 300)
		maxLines := getIntArg(args, "max_lines", 100)

		if commandStr == "" {
			return errorResult("command is required"), nil
		}

		// Validate CWD
		absCwd, err := validatePath(cwd, rootDir, allowedDirs)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid cwd: %v", err)), nil
		}

		// Prepare shell command
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell.exe", "-NoProfile", "-Command", commandStr)
		} else {
			cmd = exec.Command("sh", "-c", commandStr)
		}
		cmd.Dir = absCwd

		processID := fmt.Sprintf("proc_%d", time.Now().UnixNano())
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)

		pInfo := &ProcessInfo{
			ID:        processID,
			Command:   commandStr,
			StartTime: time.Now(),
			Status:    "running",
			cmd:       cmd,
			cancel:    cancel,
		}

		processesMu.Lock()
		processes[processID] = pInfo
		processesMu.Unlock()

		// Set up output pipes
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			cancel()
			processesMu.Lock()
			delete(processes, processID)
			processesMu.Unlock()
			return errorResult(fmt.Sprintf("failed to start command: %v", err)), nil
		}

		pInfo.PID = cmd.Process.Pid
		doneChan := make(chan struct{})

		// Asynchronously read output
		go func() {
			var wg sync.WaitGroup
			wg.Add(2)

			readPipe := func(r io.Reader) {
				defer wg.Done()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					pInfo.mu.Lock()
					pInfo.Output += scanner.Text() + "\n"
					if len(pInfo.Output) > 200000 { // Keep last 200k chars for safety
						pInfo.Output = pInfo.Output[len(pInfo.Output)-200000:]
					}
					pInfo.mu.Unlock()
				}
			}

			go readPipe(stdout)
			go readPipe(stderr)

			wg.Wait()
			err := cmd.Wait()

			pInfo.mu.Lock()
			if err != nil {
				pInfo.Status = fmt.Sprintf("failed: %v", err)
			} else {
				pInfo.Status = "completed"
			}
			pInfo.mu.Unlock()

			close(doneChan)
			cancel()
		}()

		if wait {
			select {
			case <-doneChan:
				pInfo.mu.Lock()
				res := map[string]any{
					"status": pInfo.Status,
					"output": limitLines(pInfo.Output, maxLines),
				}
				pInfo.mu.Unlock()
				return marshalResult(res)
			case <-ctx.Done():
				return errorResult("command timed out"), nil
			}
		}

		return marshalResult(map[string]any{
			"process_id": processID,
			"pid":        pInfo.PID,
			"status":     "started",
			"message":    "Command is running in background. Use get_process_output to see results.",
		})
	}
}

func makeTerminateCommandHandler() func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		processID := getStringArg(args, "process_id", "")
		if processID == "" {
			return errorResult("process_id is required"), nil
		}

		processesMu.RLock()
		pInfo, ok := processes[processID]
		processesMu.RUnlock()

		if !ok {
			return errorResult("process not found"), nil
		}

		pInfo.mu.Lock()
		defer pInfo.mu.Unlock()

		if pInfo.Status != "running" {
			return errorResult(fmt.Sprintf("process is already in status: %s", pInfo.Status)), nil
		}

		// Try to kill the process group on Unix or just the process on Windows
		if runtime.GOOS == "windows" {
			// On Windows, TaskKill is often more effective at killing child processes
			killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pInfo.PID))
			killCmd.Run()
		} else {
			// On Unix, try to kill the process group if we had set a PGID (skipped here for simplicity)
			pInfo.cmd.Process.Signal(syscall.SIGTERM)
			time.AfterFunc(2*time.Second, func() {
				pInfo.cmd.Process.Kill()
			})
		}

		pInfo.cancel()
		pInfo.Status = "terminated"

		return textResult(fmt.Sprintf("Process %s (PID %d) terminated.", processID, pInfo.PID)), nil
	}
}

func makeListProcessesHandler() func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		processesMu.RLock()
		defer processesMu.RUnlock()

		type PublicInfo struct {
			ID        string    `json:"id"`
			Command   string    `json:"command"`
			PID       int       `json:"pid"`
			Status    string    `json:"status"`
			StartTime time.Time `json:"start_time"`
		}

		result := make([]PublicInfo, 0, len(processes))
		for _, p := range processes {
			p.mu.Lock()
			result = append(result, PublicInfo{
				ID:        p.ID,
				Command:   p.Command,
				PID:       p.PID,
				Status:    p.Status,
				StartTime: p.StartTime,
			})
			p.mu.Unlock()
		}

		return marshalResult(result)
	}
}

func makeGetProcessOutputHandler() func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		processID := getStringArg(args, "process_id", "")
		maxLines := getIntArg(args, "max_lines", 1000)

		if processID == "" {
			return errorResult("process_id is required"), nil
		}

		processesMu.RLock()
		pInfo, ok := processes[processID]
		processesMu.RUnlock()

		if !ok {
			return errorResult("process not found"), nil
		}

		pInfo.mu.Lock()
		defer pInfo.mu.Unlock()

		return marshalResult(map[string]any{
			"status": pInfo.Status,
			"output": limitLines(pInfo.Output, maxLines),
		})
	}
}

