package agent

import (
	"os"
	"strings"
	"testing"
)

// agentsMDPath is the relative path from the agent package to the workspace AGENTS.md.
const agentsMDPath = "../workspace/AGENTS.md"

func TestBuildSystemPromptIncludesAgentsSelfKnowledgeGuidance(t *testing.T) {
	data, err := os.ReadFile(agentsMDPath)
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}
	content := string(data)

	// Must contain the Self-Knowledge header
	if !strings.Contains(content, "# Self-Knowledge") {
		t.Error("AGENTS.md must contain '# Self-Knowledge' section header")
	}

	// Must contain the carve-out for self-questions (up to 10 sentences)
	if !strings.Contains(content, "you may respond with up to 10 sentences") {
		t.Error("AGENTS.md Self-Knowledge section must contain the 10-sentence carve-out")
	}

	// Must still contain the original 3-sentence brevity rule (not weakened)
	if !strings.Contains(content, "under 3 sentences") {
		t.Error("AGENTS.md must still contain the original 3-sentence response length constraint")
	}

	// The carve-out must explicitly state it's the ONLY exception
	if !strings.Contains(content, "ONLY carve-out to the 3-sentence response limit") {
		t.Error("AGENTS.md Self-Knowledge section must state this is the ONLY carve-out")
	}
}

func TestSelfKnowledgeQuestionPolicy(t *testing.T) {
	data, err := os.ReadFile(agentsMDPath)
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}
	content := string(data)

	// /status must be mapped to a self-status summary
	if !strings.Contains(content, "/status") {
		t.Error("AGENTS.md Self-Knowledge section must contain /status mapping")
	}
	if !strings.Contains(content, "self-status summary") {
		t.Error("AGENTS.md must map /status to a self-status summary")
	}

	// Must reference agent_self_status tool
	if !strings.Contains(content, "agent_self_status") {
		t.Error("AGENTS.md Self-Knowledge section must reference agent_self_status tool")
	}

	// Must instruct never to guess version/uptime
	if !strings.Contains(content, "Never guess your own version") {
		t.Error("AGENTS.md must instruct agent to never guess its own version")
	}

	// Must mention cache usage is unavailable
	if !strings.Contains(content, "Cache usage is always unavailable") {
		t.Error("AGENTS.md Self-Knowledge section must state cache usage is always unavailable")
	}

	// Self-Knowledge section must NOT weaken existing rules
	// Verify core directives are intact
	if !strings.Contains(content, "Minimize output") {
		t.Error("AGENTS.md core directive 'Minimize output' must remain intact")
	}
	if !strings.Contains(content, "Never ask permission") {
		t.Error("AGENTS.md core directive 'Never ask permission' must remain intact")
	}
}
