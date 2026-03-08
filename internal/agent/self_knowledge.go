package agent

import (
	"context"

	"github.com/startower-observability/blackcat/internal/agentapi"
)

// Re-export types from agentapi to preserve the agent-package API.
// This avoids an import cycle: agent → tools, tools → agentapi (not agent).

// SelfKnowledgeProvider is the narrow interface for snapshot building.
type SelfKnowledgeProvider = agentapi.SelfKnowledgeProvider

// SelfKnowledgeSnapshot holds a point-in-time view of the agent's self-knowledge.
type SelfKnowledgeSnapshot = agentapi.SelfKnowledgeSnapshot

// BuildSelfKnowledgeSnapshot constructs a snapshot from the given provider.
// Delegates to agentapi.BuildSelfKnowledgeSnapshot.
func BuildSelfKnowledgeSnapshot(ctx context.Context, p SelfKnowledgeProvider, fullMode bool, extras *agentapi.SelfKnowledgeExtras, runtimeModelHolder *agentapi.RuntimeModelHolder) SelfKnowledgeSnapshot {
	return agentapi.BuildSelfKnowledgeSnapshot(ctx, p, fullMode, extras, runtimeModelHolder)
}

// formatDuration is kept as a package-level alias for tests that call it directly.
var formatDuration = agentapi.FormatDuration
