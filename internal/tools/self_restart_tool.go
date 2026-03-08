package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/startower-observability/blackcat/internal/service"
	"github.com/startower-observability/blackcat/internal/types"
)

const (
	selfRestartToolName        = "self_restart"
	selfRestartToolDescription = "Restart the BlackCat daemon service. Requires explicit confirmation. Only works on Linux with systemd. Use when model configuration changes require a daemon restart to take effect."
)

var selfRestartToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"confirm": {
			"type": "boolean",
			"description": "Must be true to confirm the restart. Prevents accidental invocation."
		}
	},
	"required": ["confirm"]
}`)

// ServiceManagerFactory creates a service.Manager. Exists for test injection.
type ServiceManagerFactory func() service.Manager

// SelfRestartTool allows the agent to restart the BlackCat daemon via the
// service.Manager infrastructure. A confirmation guard prevents accidental
// invocation.
type SelfRestartTool struct {
	newManager ServiceManagerFactory
}

var _ types.Tool = (*SelfRestartTool)(nil)

// NewSelfRestartTool creates a SelfRestartTool that uses the default
// service.New factory.
func NewSelfRestartTool() *SelfRestartTool {
	return &SelfRestartTool{
		newManager: func() service.Manager { return service.New() },
	}
}

// NewSelfRestartToolWithFactory creates a SelfRestartTool with a custom
// manager factory (useful for testing).
func NewSelfRestartToolWithFactory(factory ServiceManagerFactory) *SelfRestartTool {
	return &SelfRestartTool{newManager: factory}
}

func (t *SelfRestartTool) Name() string                { return selfRestartToolName }
func (t *SelfRestartTool) Description() string         { return selfRestartToolDescription }
func (t *SelfRestartTool) Parameters() json.RawMessage { return selfRestartToolParameters }

func (t *SelfRestartTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Confirm bool `json:"confirm"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("self_restart: invalid arguments: %w", err)
		}
	}

	// Guard: require explicit confirmation.
	if !params.Confirm {
		return "Restart cancelled: confirm parameter must be true", nil
	}

	// Platform guard: only Linux with systemd is supported.
	if runtime.GOOS != "linux" {
		return "self_restart is only supported on Linux with systemd", nil
	}

	mgr := t.newManager()

	if !mgr.IsInstalled() {
		return "", fmt.Errorf("self_restart: daemon is not installed as a service. Run 'blackcat onboard' first")
	}

	go func() {
		time.Sleep(600 * time.Millisecond)
		_ = mgr.Restart()
	}()

	return "Daemon restart initiated. Connection will be lost. Reconnect shortly.", nil
}
