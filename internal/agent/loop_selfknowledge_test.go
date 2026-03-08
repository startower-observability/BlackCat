package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/startower-observability/blackcat/internal/tools"
)

// mockSelfKnowledgeTool is a minimal types.Tool implementation for testing.
type mockSelfKnowledgeTool struct {
	name   string
	result string
}

func (m *mockSelfKnowledgeTool) Name() string                { return m.name }
func (m *mockSelfKnowledgeTool) Description() string         { return "mock tool" }
func (m *mockSelfKnowledgeTool) Parameters() json.RawMessage { return json.RawMessage(`{}`) }
func (m *mockSelfKnowledgeTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return m.result, nil
}

// newTestLoop creates a minimal Loop with only the tools registry populated.
func newTestLoop(registry *tools.Registry) *Loop {
	return NewLoop(LoopConfig{
		Tools: registry,
	})
}

// TestSelfKnowledgeQuestionsForceToolBackedResolution verifies that when a user
// asks a self-knowledge question, the enforcement injects tool-backed results
// into the execution message history before the LLM call.
func TestSelfKnowledgeQuestionsForceToolBackedResolution(t *testing.T) {
	selfKnowledgeQueries := []string{
		"who are you",
		"what are you",
		"what can you do",
		"what model are you using",
		"which model are you?",
		"your capabilities",
		"tell me about yourself",
		"what tools do you have",
		"your skills",
		"what roles do you support",
		"your roles",
		"self status",
		"what version are you",
		"what provider are you using",
	}

	for _, query := range selfKnowledgeQueries {
		t.Run(query, func(t *testing.T) {
			registry := tools.NewRegistry()
			registry.Register(&mockSelfKnowledgeTool{
				name:   "agent_self_status",
				result: `{"agent_name":"test","model":"gpt-4o"}`,
			})

			loop := newTestLoop(registry)
			execution := NewExecution(10)

			loop.enforceToolBackedSelfKnowledge(context.Background(), execution, query)

			// Expect: at least 2 messages injected (assistant tool-call + tool result)
			if len(execution.Messages) < 2 {
				t.Errorf("query=%q: expected at least 2 injected messages, got %d", query, len(execution.Messages))
				return
			}

			// First injected message should be an assistant message with a tool call.
			assistantMsg := execution.Messages[0]
			if assistantMsg.Role != "assistant" {
				t.Errorf("query=%q: messages[0].Role = %q, want %q", query, assistantMsg.Role, "assistant")
			}
			if len(assistantMsg.ToolCalls) == 0 {
				t.Errorf("query=%q: messages[0].ToolCalls is empty", query)
			} else if assistantMsg.ToolCalls[0].Name != "agent_self_status" {
				t.Errorf("query=%q: ToolCalls[0].Name = %q, want %q", query, assistantMsg.ToolCalls[0].Name, "agent_self_status")
			}

			// Second injected message should be the tool result.
			toolResultMsg := execution.Messages[1]
			if toolResultMsg.Role != "tool" {
				t.Errorf("query=%q: messages[1].Role = %q, want %q", query, toolResultMsg.Role, "tool")
			}
			if !strings.Contains(toolResultMsg.Content, "gpt-4o") {
				t.Errorf("query=%q: tool result content = %q, expected to contain tool output", query, toolResultMsg.Content)
			}
		})
	}
}

// TestSelfKnowledgeEnforcementScopedToRelevantQuestions verifies that the
// enforcement does NOT inject messages for unrelated conversational questions.
func TestSelfKnowledgeEnforcementScopedToRelevantQuestions(t *testing.T) {
	unrelatedQueries := []string{
		"what is the weather today",
		"help me write a poem",
		"translate this to French",
		"summarize this article",
		"write a Go function that sorts a slice",
		"what is 2+2",
		"tell me a joke",
		"how do I use Docker",
		"deploy my application",
	}

	for _, query := range unrelatedQueries {
		t.Run(query, func(t *testing.T) {
			registry := tools.NewRegistry()
			registry.Register(&mockSelfKnowledgeTool{
				name:   "agent_self_status",
				result: `{"agent_name":"test"}`,
			})

			loop := newTestLoop(registry)
			execution := NewExecution(10)

			loop.enforceToolBackedSelfKnowledge(context.Background(), execution, query)

			// No messages should be injected for unrelated queries.
			if len(execution.Messages) != 0 {
				t.Errorf("query=%q: expected 0 injected messages, got %d", query, len(execution.Messages))
			}
		})
	}
}

// TestSelfKnowledgeEnforcementProviderCatalogInjected verifies that provider
// catalog queries also inject results from the provider_catalog tool.
func TestSelfKnowledgeEnforcementProviderCatalogInjected(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&mockSelfKnowledgeTool{
		name:   "agent_self_status",
		result: `{"agent_name":"test","model":"gpt-4o"}`,
	})
	registry.Register(&mockSelfKnowledgeTool{
		name:   "provider_catalog",
		result: `{"providers":[{"name":"openai","models":[]}]}`,
	})

	loop := newTestLoop(registry)
	execution := NewExecution(10)

	loop.enforceToolBackedSelfKnowledge(context.Background(), execution, "what models are available")

	// Should inject 4 messages: (assistant + tool) x 2 tools
	if len(execution.Messages) < 4 {
		t.Errorf("expected at least 4 injected messages for provider query, got %d", len(execution.Messages))
	}

	// Verify provider_catalog result is present.
	found := false
	for _, msg := range execution.Messages {
		if msg.Role == "tool" && strings.Contains(msg.Content, "providers") {
			found = true
			break
		}
	}
	if !found {
		t.Error("provider_catalog tool result not found in injected messages")
	}
}

// TestSelfKnowledgeEnforcementMissingToolSkipped verifies that if a tool is not
// registered, the enforcement silently skips it without panicking or erroring.
func TestSelfKnowledgeEnforcementMissingToolSkipped(t *testing.T) {
	registry := tools.NewRegistry()
	// Intentionally do NOT register agent_self_status

	loop := newTestLoop(registry)
	execution := NewExecution(10)

	// Should not panic; should produce 0 injected messages.
	loop.enforceToolBackedSelfKnowledge(context.Background(), execution, "who are you")

	if len(execution.Messages) != 0 {
		t.Errorf("expected 0 messages when tool missing, got %d", len(execution.Messages))
	}
}

// TestIsSelfKnowledgeQuery validates the intent classifier directly.
func TestIsSelfKnowledgeQuery(t *testing.T) {
	positives := []struct {
		msg  string
		desc string
	}{
		{"who are you", "who are you"},
		{"What are you?", "what are you (case insensitive)"},
		{"What can you do?", "what can you do"},
		{"What model are you?", "what model"},
		{"Which model do you use?", "which model"},
		{"Tell me about your capabilities", "capabilities"},
		{"List your skills", "your skills"},
		{"What are your roles?", "your roles"},
		{"What roles do you have", "what roles"},
		{"What version is this?", "what version"},
		{"What provider are you using?", "what provider"},
		{"Show available models", "available models"},
		{"List models please", "list models"},
		{"What is your identity?", "identity"},
		{"agent self status", "self status"},
		{"what tools do you have", "your tools"},
	}

	for _, tc := range positives {
		if !isSelfKnowledgeQuery(tc.msg) {
			t.Errorf("[%s] isSelfKnowledgeQuery(%q) = false, want true", tc.desc, tc.msg)
		}
	}

	negatives := []struct {
		msg  string
		desc string
	}{
		{"write me a poem", "poem"},
		{"help me debug this code", "debug code"},
		{"translate this", "translate"},
		{"what is the capital of France", "geography"},
		{"deploy my app", "deploy"},
		{"how do I use Docker", "docker"},
	}

	for _, tc := range negatives {
		if isSelfKnowledgeQuery(tc.msg) {
			t.Errorf("[%s] isSelfKnowledgeQuery(%q) = true, want false", tc.desc, tc.msg)
		}
	}
}

// TestEnforcementDoesNotDuplicateResultsOnRepeatedCalls verifies that calling
// enforcement twice for different turns doesn't double-inject on first call only.
func TestEnforcementIdempotentPerMessage(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&mockSelfKnowledgeTool{
		name:   "agent_self_status",
		result: `{"agent_name":"test"}`,
	})

	loop := newTestLoop(registry)
	execution := NewExecution(10)

	// First call for a self-knowledge query — should inject.
	loop.enforceToolBackedSelfKnowledge(context.Background(), execution, "who are you")
	firstCount := len(execution.Messages)
	if firstCount < 2 {
		t.Errorf("expected messages after first enforcement call, got %d", firstCount)
	}

	// Second call for an UNRELATED query — should NOT inject again.
	loop.enforceToolBackedSelfKnowledge(context.Background(), execution, "write me a poem")
	if len(execution.Messages) != firstCount {
		t.Errorf("expected message count to stay at %d after unrelated query, got %d",
			firstCount, len(execution.Messages))
	}
}
