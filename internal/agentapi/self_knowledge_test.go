package agentapi

import (
	"encoding/json"
	"testing"
	"time"
)

// TestProviderModelViewRedaction verifies ProviderModelView is LLM-safe
func TestProviderModelViewRedaction(t *testing.T) {
	view := ProviderModelView{
		ID:            "gpt-4o",
		Name:          "GPT-4o",
		ContextWindow: 128000,
		Modalities:    []string{"text"},
		Freshness:     FreshnessMetadata{Source: SourceLive, LastAttemptAt: time.Now()},
	}
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	// Must contain expected fields
	for _, field := range []string{"id", "name", "freshness"} {
		if _, ok := m[field]; !ok {
			t.Errorf("expected field %q in ProviderModelView, not found", field)
		}
	}
	// Must NOT contain pricing/sensitive fields
	for _, banned := range []string{"pricing", "rate_limit", "env_var", "max_output"} {
		if _, ok := m[banned]; ok {
			t.Errorf("unexpected sensitive field %q found in ProviderModelView", banned)
		}
	}
}

// TestAgentSelfStatusBackwardCompatibleFields verifies existing JSON field names are unchanged
func TestAgentSelfStatusBackwardCompatibleFields(t *testing.T) {
	snap := SelfKnowledgeSnapshot{
		Version:            "1.3.2",
		Commit:             "abc1234",
		BuildDate:          "2026-03-07",
		AgentName:          "test-agent",
		ModelName:          "gpt-4o",
		ProviderName:       "openai",
		ChannelType:        "telegram",
		DaemonUptime:       "1h2m3s",
		ProcessUptime:      "30m",
		ActiveSkillCount:   3,
		ActiveSkillNames:   []string{"skill1", "skill2", "skill3"},
		InactiveSkillCount: 1,
		TokenUsage24h:      "1,234 tokens",
		CacheUsage:         "unavailable",
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	required := []string{
		"version", "commit", "build_date",
		"agent_name", "model_name", "provider_name", "channel_type",
		"daemon_uptime", "process_uptime",
		"active_skill_count", "active_skill_names", "inactive_skill_count",
		"token_usage_24h", "cache_usage",
	}
	for _, field := range required {
		if _, ok := m[field]; !ok {
			t.Errorf("backward compat: expected JSON field %q missing from SelfKnowledgeSnapshot", field)
		}
	}
}
