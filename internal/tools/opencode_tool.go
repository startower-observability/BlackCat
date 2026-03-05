package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/startower-observability/blackcat/internal/opencode"
)

const (
	defaultOpenCodeTimeout  = 30 * time.Minute
	openCodeToolName        = "opencode_task"
	openCodeToolDescription = "Delegate a coding task to OpenCode. Use this for code generation, refactoring, debugging, and complex code changes. CRITICAL: You MUST specify 'dir' with the project directory path. Without it, OpenCode runs in ~ and cannot find project files. Determine the correct dir from context: if user cloned a repo, use that clone path. If unsure, use exec to find it first (e.g. find ~ -maxdepth 3 -name '.git' -type d)."
)

var openCodeToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"prompt": {
			"type": "string",
			"description": "The coding task description"
		},
		"dir": {
			"type": "string",
			"description": "Project directory path (REQUIRED). OpenCode needs this to find project files. Determine from context: after git clone use that path, or find it with 'find ~ -maxdepth 3 -name .git -type d'."
		},
		"session_id": {
			"type": "string",
			"description": "Reuse an existing session (optional)"
		},
		"model": {
			"type": "string",
			"description": "Override model for this task (optional)"
		}
	},
	"required": ["prompt"]
}`)

// OpenCodeTool delegates coding tasks to an OpenCode instance via its REST API.
type OpenCodeTool struct {
	client     *opencode.Client
	sessionMgr *opencode.SessionManager
	autoPermit bool
	timeout    time.Duration
}

// NewOpenCodeTool creates an OpenCodeTool backed by the given client.
// If timeout is 0, defaults to 30 minutes.
func NewOpenCodeTool(client *opencode.Client, autoPermit bool, timeout time.Duration) *OpenCodeTool {
	if timeout <= 0 {
		timeout = defaultOpenCodeTimeout
	}
	return &OpenCodeTool{
		client:     client,
		sessionMgr: opencode.NewSessionManager(client),
		autoPermit: autoPermit,
		timeout:    timeout,
	}
}

func (t *OpenCodeTool) Name() string                { return openCodeToolName }
func (t *OpenCodeTool) Description() string         { return openCodeToolDescription }
func (t *OpenCodeTool) Parameters() json.RawMessage { return openCodeToolParameters }

// Execute delegates a coding task to OpenCode and returns a structured summary.
func (t *OpenCodeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Prompt    string `json:"prompt"`
		Dir       string `json:"dir"`
		SessionID string `json:"session_id"`
		Model     string `json:"model"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Sprintf("error: invalid arguments: %s", err), nil
	}
	if params.Prompt == "" {
		return "error: prompt is required", nil
	}

	req := opencode.TaskRequest{
		Prompt:     params.Prompt,
		SessionID:  params.SessionID,
		Dir:        params.Dir,
		ModelID:    params.Model,
		AutoPermit: t.autoPermit,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	result, err := t.sessionMgr.Run(timeoutCtx, req)
	if err != nil {
		return fmt.Sprintf("error: %s", err), nil
	}

	// Extract the last assistant message.
	assistantContent := "(no assistant response)"
	for i := len(result.Messages) - 1; i >= 0; i-- {
		if result.Messages[i].Info.Role == "assistant" {
			assistantContent = extractMessageContent(result.Messages[i])
			break
		}
	}

	return fmt.Sprintf("OpenCode Task Complete\nSession: %s\nMessages: %d\n\nAssistant Response:\n%s",
		result.SessionID,
		len(result.Messages),
		assistantContent,
	), nil
}

// extractMessageContent returns a human-readable summary of a message.
// It concatenates all text parts from the message.
func extractMessageContent(msg opencode.MessageWithParts) string {
	var texts []string
	for _, p := range msg.Parts {
		if p.Type == "text" && p.Text != nil && *p.Text != "" {
			texts = append(texts, *p.Text)
		}
	}
	if len(texts) > 0 {
		return strings.Join(texts, "\n")
	}
	agent := msg.Info.Agent
	if agent == "" {
		agent = "assistant"
	}
	return fmt.Sprintf("[%s] message %s (no text parts)", agent, msg.Info.ID)
}
