package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/startower-observability/blackcat/internal/types"
)

// selfKnowledgeTriggers are keyword phrases that indicate the user is asking
// about the agent's own identity, capabilities, roles, skills, or provider models.
// Matching is case-insensitive and checks for substring presence.
var selfKnowledgeTriggers = []string{
	"what model",
	"which model",
	"what version",
	"what can you do",
	"your capabilities",
	"your skills",
	"what roles",
	"your roles",
	"what provider",
	"available models",
	"list models",
	"who are you",
	"what are you",
	"your identity",
	"self status",
	"about yourself",
	"github copilot model",
	"openai model",
	"gemini model",
	"your tools",
	"what tools",
}

// isSelfKnowledgeQuery returns true if the user message is likely asking about
// the agent's own identity, capabilities, roles, skills, or available models.
// This is a fast, pure-Go, no-LLM classification based on keyword matching.
func isSelfKnowledgeQuery(message string) bool {
	lower := strings.ToLower(message)
	for _, trigger := range selfKnowledgeTriggers {
		if strings.Contains(lower, trigger) {
			return true
		}
	}
	return false
}

// isProviderCatalogQuery returns true if the message specifically asks about
// provider model catalogs (e.g., listing available models).
func isProviderCatalogQuery(message string) bool {
	lower := strings.ToLower(message)
	catalogPhrases := []string{
		"available models",
		"list models",
		"github copilot model",
		"openai model",
		"gemini model",
		"what model",
		"which model",
		"what provider",
	}
	for _, phrase := range catalogPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// selfKnowledgeTools is the ordered list of tool names to pre-flight for
// self-knowledge queries. All tools in this list are attempted; any that are
// not registered are silently skipped.
var selfKnowledgeTools = []string{
	"agent_self_status",
}

// providerCatalogTools is the list of tool names to pre-flight for provider/model
// catalog queries (in addition to agent_self_status).
var providerCatalogTools = []string{
	"provider_catalog",
}

// enforceToolBackedSelfKnowledge inspects the user message and, if it is a
// self-knowledge / capability / provider-model query, executes the relevant
// tools directly (without going through the LLM) and injects the results as
// pre-flight tool-result messages into the execution context.
//
// This structural enforcement ensures the LLM always has fresh, runtime-backed
// facts before generating its response — instead of relying on training-time
// knowledge or system-prompt instructions alone.
//
// Tools that are not registered in the registry are silently skipped so that
// callers don't need to ensure every tool is wired before calling this.
func (l *Loop) enforceToolBackedSelfKnowledge(ctx context.Context, execution *Execution, userMessage string) {
	if !isSelfKnowledgeQuery(userMessage) {
		return
	}

	// Determine which tools to run.
	toolNames := make([]string, len(selfKnowledgeTools))
	copy(toolNames, selfKnowledgeTools)
	if isProviderCatalogQuery(userMessage) {
		toolNames = append(toolNames, providerCatalogTools...)
	}

	for _, toolName := range toolNames {
		result, err := l.tools.Execute(ctx, toolName, json.RawMessage(`{}`))
		if err != nil {
			// If tool is not registered or fails, log and skip — do not break the
			// main loop.
			continue
		}

		// Inject a synthetic assistant message that "called" the tool, followed
		// by the tool result. This mimics the standard tool-call / tool-result
		// message pair that the LLM expects in its conversation history.
		callID := fmt.Sprintf("preflight-%s", toolName)
		execution.AddAssistantMessage("", []types.ToolCall{
			{
				ID:        callID,
				Name:      toolName,
				Arguments: json.RawMessage(`{}`),
			},
		})
		execution.AddToolResult(callID, toolName, result)
	}
}
