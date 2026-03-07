package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/observability"
	"github.com/startower-observability/blackcat/internal/skills"
)

// stubSelfKnowledgeProvider satisfies agentapi.SelfKnowledgeProvider for tests.
type stubSelfKnowledgeProvider struct {
	agentName      string
	modelName      string
	providerName   string
	channelType    string
	daemonStarted  time.Time
	activeSkills   []skills.Skill
	inactiveSkills []skills.InactiveSkill
	costTracker    *observability.CostTracker
	userID         string
}

func (s *stubSelfKnowledgeProvider) GetAgentName() string            { return s.agentName }
func (s *stubSelfKnowledgeProvider) GetModelName() string            { return s.modelName }
func (s *stubSelfKnowledgeProvider) GetProviderName() string         { return s.providerName }
func (s *stubSelfKnowledgeProvider) GetChannelType() string          { return s.channelType }
func (s *stubSelfKnowledgeProvider) GetDaemonStartedAt() time.Time   { return s.daemonStarted }
func (s *stubSelfKnowledgeProvider) GetActiveSkills() []skills.Skill { return s.activeSkills }
func (s *stubSelfKnowledgeProvider) GetInactiveSkills() []skills.InactiveSkill {
	return s.inactiveSkills
}
func (s *stubSelfKnowledgeProvider) GetCostTracker() *observability.CostTracker { return s.costTracker }
func (s *stubSelfKnowledgeProvider) GetUserID() string                          { return s.userID }

func TestAgentSelfStatusToolCompact(t *testing.T) {
	ctx := context.Background()

	provider := &stubSelfKnowledgeProvider{
		agentName:     "TestCat",
		modelName:     "gpt-4o",
		providerName:  "openai",
		channelType:   "telegram",
		daemonStarted: time.Now().Add(-15 * time.Minute),
		activeSkills: []skills.Skill{
			{Name: "weather"},
			{Name: "calendar"},
		},
		inactiveSkills: []skills.InactiveSkill{
			{Name: "linkedin", Reason: "missing env"},
			{Name: "twitter", Reason: "missing binary"},
		},
		costTracker: nil,
		userID:      "user-1",
	}

	tool := NewAgentSelfStatusTool(provider)

	// Verify tool metadata
	if tool.Name() != "agent_self_status" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "agent_self_status")
	}

	result, err := tool.Execute(ctx, json.RawMessage(`{"full":false}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	var snap map[string]interface{}
	if err := json.Unmarshal([]byte(result), &snap); err != nil {
		t.Fatalf("failed to parse JSON result: %v", err)
	}

	// Check key fields present
	if snap["agent_name"] != "TestCat" {
		t.Errorf("agent_name = %v, want TestCat", snap["agent_name"])
	}
	if snap["model_name"] != "gpt-4o" {
		t.Errorf("model_name = %v, want gpt-4o", snap["model_name"])
	}
	if snap["provider_name"] != "openai" {
		t.Errorf("provider_name = %v, want openai", snap["provider_name"])
	}
	if snap["cache_usage"] != "unavailable" {
		t.Errorf("cache_usage = %v, want unavailable", snap["cache_usage"])
	}

	// Compact mode: inactive_skills should be absent (omitempty)
	if _, ok := snap["inactive_skills"]; ok {
		t.Errorf("compact mode should not include inactive_skills, but got: %v", snap["inactive_skills"])
	}

	// active_skill_count should be 2
	if snap["active_skill_count"].(float64) != 2 {
		t.Errorf("active_skill_count = %v, want 2", snap["active_skill_count"])
	}

	// inactive_skill_count should be 2
	if snap["inactive_skill_count"].(float64) != 2 {
		t.Errorf("inactive_skill_count = %v, want 2", snap["inactive_skill_count"])
	}

	// full_mode should be false
	if snap["full_mode"].(bool) != false {
		t.Errorf("full_mode = %v, want false", snap["full_mode"])
	}
}

func TestAgentSelfStatusToolFull(t *testing.T) {
	ctx := context.Background()

	inactiveList := []skills.InactiveSkill{
		{Name: "linkedin", FilePath: "/skills/linkedin.md", Reason: "missing env", MissingEnv: []string{"LINKEDIN_LI_AT"}},
		{Name: "twitter", FilePath: "/skills/twitter.md", Reason: "missing binary", MissingBins: []string{"bird"}},
	}

	provider := &stubSelfKnowledgeProvider{
		agentName:      "BlackCat",
		modelName:      "claude-3-5-sonnet",
		providerName:   "anthropic",
		channelType:    "discord",
		daemonStarted:  time.Now().Add(-2 * time.Hour),
		activeSkills:   []skills.Skill{{Name: "weather"}},
		inactiveSkills: inactiveList,
		costTracker:    nil,
		userID:         "user-2",
	}

	tool := NewAgentSelfStatusTool(provider)

	result, err := tool.Execute(ctx, json.RawMessage(`{"full":true}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	var snap map[string]interface{}
	if err := json.Unmarshal([]byte(result), &snap); err != nil {
		t.Fatalf("failed to parse JSON result: %v", err)
	}

	// Full mode: inactive_skills should be present
	inactiveRaw, ok := snap["inactive_skills"]
	if !ok {
		t.Fatalf("full mode should include inactive_skills")
	}

	inactiveArr, ok := inactiveRaw.([]interface{})
	if !ok {
		t.Fatalf("inactive_skills should be an array, got %T", inactiveRaw)
	}

	if len(inactiveArr) != 2 {
		t.Errorf("inactive_skills count = %d, want 2", len(inactiveArr))
	}

	// full_mode should be true
	if snap["full_mode"].(bool) != true {
		t.Errorf("full_mode = %v, want true", snap["full_mode"])
	}

	// Verify first inactive skill has the name
	firstInactive, ok := inactiveArr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("inactive_skills[0] should be an object")
	}
	if firstInactive["Name"] != "linkedin" {
		t.Errorf("inactive_skills[0].Name = %v, want linkedin", firstInactive["Name"])
	}

	// cache_usage should always be unavailable
	if snap["cache_usage"] != "unavailable" {
		t.Errorf("cache_usage = %v, want unavailable", snap["cache_usage"])
	}
}
