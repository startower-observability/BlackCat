package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/startower-observability/blackcat/internal/agentapi"
	"github.com/startower-observability/blackcat/internal/skills"
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
	extras   *agentapi.SelfKnowledgeExtras
}

// NewAgentSelfStatusTool creates an AgentSelfStatusTool.
// extras may be nil for backward compatibility (pre-Phase 5 callers).
func NewAgentSelfStatusTool(provider agentapi.SelfKnowledgeProvider, extras ...*agentapi.SelfKnowledgeExtras) *AgentSelfStatusTool {
	var ext *agentapi.SelfKnowledgeExtras
	if len(extras) > 0 {
		ext = extras[0]
	}
	return &AgentSelfStatusTool{provider: provider, extras: ext}
}

func (t *AgentSelfStatusTool) Name() string                { return agentSelfStatusToolName }
func (t *AgentSelfStatusTool) Description() string         { return agentSelfStatusToolDescription }
func (t *AgentSelfStatusTool) Parameters() json.RawMessage { return agentSelfStatusToolParameters }

// selfStatusResponse wraps the snapshot with Phase 5 redacted fields.
// When Phase 5 extras are available, InactiveSkillSummaries replaces
// raw InactiveSkills to avoid exposing env var names.
type selfStatusResponse struct {
	agentapi.SelfKnowledgeSnapshot

	// Phase 5: redacted inactive skill summaries (safe reason categories only).
	// Populated when extras.SkillInventory is available.
	InactiveSkillSummaries []skills.InactiveSkillSummary `json:"inactive_skill_summaries,omitempty"`
}

func (t *AgentSelfStatusTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var args struct {
		Full bool `json:"full"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &args)
	}

	snap := agentapi.BuildSelfKnowledgeSnapshot(ctx, t.provider, args.Full, t.extras)

	resp := selfStatusResponse{SelfKnowledgeSnapshot: snap}

	// Phase 5: add redacted inactive skill summaries when inventory is available
	if t.extras != nil && t.extras.SkillInventory != nil && len(t.extras.SkillInventory.Inactive) > 0 {
		summaries := make([]skills.InactiveSkillSummary, 0, len(t.extras.SkillInventory.Inactive))
		for _, is := range t.extras.SkillInventory.Inactive {
			reasons := inactiveSkillReasonCategories(is.MissingEnv, is.MissingBins)
			summaries = append(summaries, skills.InactiveSkillSummary{
				Name:    is.Name,
				Reasons: reasons,
			})
		}
		resp.InactiveSkillSummaries = summaries
		// Clear raw inactive skills to avoid leaking env var names
		resp.InactiveSkills = nil
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}

	result := string(data)

	// Append human-readable Phase 5 summary for quick scanning
	var sections []string

	if len(snap.Roles) > 0 {
		roleLines := make([]string, len(snap.Roles))
		for i, r := range snap.Roles {
			roleLines[i] = fmt.Sprintf("  • %s (priority %d, %d keywords)", r.Name, r.Priority, r.KeywordCount)
		}
		sections = append(sections, fmt.Sprintf("\n--- Roles ---\n%s", strings.Join(roleLines, "\n")))
	}

	if snap.SchedulerEnabled {
		sections = append(sections, fmt.Sprintf("\n--- Scheduler ---\nEnabled: true, Tasks: %d", snap.SchedulerTaskCount))
	}

	if len(sections) > 0 {
		result += strings.Join(sections, "")
	}

	return result, nil
}

// inactiveSkillReasonCategories returns safe reason category strings.
// It does NOT expose raw env var names or binary names.
func inactiveSkillReasonCategories(missingEnv, missingBins []string) []skills.InactiveReason {
	var reasons []skills.InactiveReason
	if len(missingEnv) > 0 {
		reasons = append(reasons, skills.ReasonMissingEnv)
	}
	if len(missingBins) > 0 {
		reasons = append(reasons, skills.ReasonMissingBinary)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, skills.ReasonOther)
	}
	return reasons
}
