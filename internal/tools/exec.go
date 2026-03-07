package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/security"
)

const (
	defaultExecTimeout  = 60 * time.Second
	maxExecTimeout      = 600 * time.Second
	defaultMaxOutput    = 1 << 20 // 1 MB
	execToolName        = "exec"
	execToolDescription = "Execute a shell command on the server. Supports optional timeout (up to 600s) for long-running commands and optional stdin for piping input."
)

var execToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"command": {
			"type": "string",
			"description": "Shell command to execute"
		},
		"workdir": {
			"type": "string",
			"description": "Working directory (optional)"
		},
		"timeout": {
			"type": "integer",
			"description": "Timeout in seconds (optional, default 60, max 600). Use higher values for long-running commands like builds, OAuth flows, or file downloads."
		},
		"stdin": {
			"type": "string",
			"description": "Input to pipe to the command via stdin (optional). Use this for commands that require user input."
		}
	},
	"required": ["command"]
}`)

// ExecTool executes shell commands with deny-list filtering.
type ExecTool struct {
	denyList  *security.DenyList
	workDir   string
	timeout   time.Duration
	maxOutput int
	rtkConfig config.RTKConfig
}

// NewExecTool creates an ExecTool with the given deny list and defaults.
func NewExecTool(denyList *security.DenyList, workDir string, timeout time.Duration, rtkConfig config.RTKConfig) *ExecTool {
	if timeout <= 0 {
		timeout = defaultExecTimeout
	}
	return &ExecTool{
		denyList:  denyList,
		workDir:   workDir,
		timeout:   timeout,
		maxOutput: defaultMaxOutput,
		rtkConfig: rtkConfig,
	}
}

func (t *ExecTool) Name() string                { return execToolName }
func (t *ExecTool) Description() string         { return execToolDescription }
func (t *ExecTool) Parameters() json.RawMessage { return execToolParameters }

// wrapWithRTK prepends "rtk " to the command if RTK is enabled and the
// base command is in the configured allow-list.
func (t *ExecTool) wrapWithRTK(command string) string {
	if !t.rtkConfig.Enabled {
		return command
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return command
	}
	base := filepath.Base(fields[0])
	for _, allowed := range t.rtkConfig.Commands {
		if base == allowed {
			return "rtk " + command
		}
	}
	return command
}

// Execute runs a shell command after checking the deny list.
func (t *ExecTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
		Workdir string `json:"workdir"`
		Timeout int    `json:"timeout"`
		Stdin   string `json:"stdin"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("exec: invalid arguments: %w", err)
	}
	if params.Command == "" {
		return "", fmt.Errorf("exec: command is required")
	}

	// Check against deny list.
	if err := t.denyList.Check(params.Command); err != nil {
		return "", err
	}

	// Guard: opencode run without --dir produces wrong results.
	if strings.Contains(params.Command, "opencode run") && !strings.Contains(params.Command, "--dir") {
		return "", fmt.Errorf("exec: 'opencode run' requires --dir flag. Find the project directory first (e.g. find ~ -maxdepth 3 -name '.git' -type d) then add --dir /path/to/project")
	}

	// Wrap with RTK if the command's base binary is in the allow-list.
	params.Command = t.wrapWithRTK(params.Command)

	// Determine timeout: use param if provided, otherwise use default.
	timeout := t.timeout
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
		if timeout > maxExecTimeout {
			timeout = maxExecTimeout
		}
	}

	// Build the command with a timeout-scoped context.
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(timeoutCtx, "cmd", "/C", params.Command)
	} else {
		cmd = exec.CommandContext(timeoutCtx, "sh", "-c", params.Command)
	}

	// WaitDelay ensures the process is killed promptly after context cancellation.
	cmd.WaitDelay = 2 * time.Second

	// Set working directory.
	if params.Workdir != "" {
		cmd.Dir = params.Workdir
	} else if t.workDir != "" {
		cmd.Dir = t.workDir
	}

	// Pipe stdin if provided.
	if params.Stdin != "" {
		cmd.Stdin = strings.NewReader(params.Stdin)
	}

	output, err := cmd.CombinedOutput()

	// Truncate if output exceeds maxOutput.
	if len(output) > t.maxOutput {
		output = append(output[:t.maxOutput], []byte("\n... (output truncated)")...)
	}

	// Check if the context deadline was exceeded (timeout).
	if timeoutCtx.Err() != nil {
		return string(output), fmt.Errorf("exec: %w", timeoutCtx.Err())
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return string(output), fmt.Errorf("exec: %w", err)
		}
	}

	return fmt.Sprintf("%s\n[exit code: %d]", string(output), exitCode), nil
}
