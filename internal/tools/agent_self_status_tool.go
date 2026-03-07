package tools

import (
	"context"
	"encoding/json"

	"github.com/startower-observability/blackcat/internal/agentapi"
)

const (
	agentSelfStatusToolName        = "agent_self_status"
	agentSelfStatusToolDescription = "Returns current agent runtime status including version, uptime, active skills, token usage, and model information."
)

var agentSelfStatusToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"full": {
			"type": "boolean",
			"description": "If true, include inactive skill details."
		}
	}
}`)

// AgentSelfStatusTool allows the agent to introspect its own runtime state.
type AgentSelfStatusTool struct {
	provider agentapi.SelfKnowledgeProvider
}

// NewAgentSelfStatusTool creates an AgentSelfStatusTool.
func NewAgentSelfStatusTool(provider agentapi.SelfKnowledgeProvider) *AgentSelfStatusTool {
	return &AgentSelfStatusTool{provider: provider}
}

func (t *AgentSelfStatusTool) Name() string                { return agentSelfStatusToolName }
func (t *AgentSelfStatusTool) Description() string         { return agentSelfStatusToolDescription }
func (t *AgentSelfStatusTool) Parameters() json.RawMessage { return agentSelfStatusToolParameters }

func (t *AgentSelfStatusTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var args struct {
		Full bool `json:"full"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &args)
	}

	snap := agentapi.BuildSelfKnowledgeSnapshot(ctx, t.provider, args.Full)

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
