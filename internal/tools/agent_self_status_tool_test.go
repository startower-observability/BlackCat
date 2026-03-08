package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/agentapi"
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

	tool := NewAgentSelfStatusTool(provider, nil)

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

	tool := NewAgentSelfStatusTool(provider, nil)

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

func TestAgentSelfStatusToolIncludesRuntimeModelState(t *testing.T) {
	ctx := context.Background()

	provider := &stubSelfKnowledgeProvider{
		agentName:    "RuntimeCat",
		modelName:    "legacy-model",
		providerName: "legacy-provider",
	}

	holder := agentapi.NewRuntimeModelHolder()
	holder.Set(agentapi.RuntimeModelStatus{
		ConfiguredModel: agentapi.RuntimeModelRef{CanonicalID: "anthropic/claude-opus-4-6"},
		AppliedModel:    agentapi.RuntimeModelRef{CanonicalID: "anthropic/claude-opus-4-6"},
		BackendProvider: "zen",
		ReloadCount:     2,
	})

	tool := NewAgentSelfStatusTool(provider, holder)

	result, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(result, "--- Model State ---") {
		t.Fatalf("output should include model state section, got:\n%s", result)
	}
	if !strings.Contains(result, "Configured: anthropic/claude-opus-4-6") {
		t.Errorf("configured canonical ID missing, got:\n%s", result)
	}
	if !strings.Contains(result, "Applied: anthropic/claude-opus-4-6") {
		t.Errorf("applied canonical ID missing, got:\n%s", result)
	}
	if !strings.Contains(result, "Backend: zen") {
		t.Errorf("backend provider missing, got:\n%s", result)
	}
	if !strings.Contains(result, "Reload Count: 2") {
		t.Errorf("reload count missing, got:\n%s", result)
	}
	if !strings.Contains(result, "Last Reload Error: (none)") {
		t.Errorf("last reload error fallback missing, got:\n%s", result)
	}
}

func TestAgentSelfStatusPhase5(t *testing.T) {
	ctx := context.Background()

	inactiveList := []skills.InactiveSkill{
		{Name: "linkedin", FilePath: "/skills/linkedin.md", Reason: "missing env", MissingEnv: []string{"LINKEDIN_LI_AT"}},
		{Name: "twitter", FilePath: "/skills/twitter.md", Reason: "missing binary", MissingBins: []string{"bird"}},
		{Name: "tiktok", FilePath: "/skills/tiktok.md", Reason: "missing env and binary", MissingEnv: []string{"TIKTOK_TOKEN"}, MissingBins: []string{"tiktok-cli"}},
	}

	provider := &stubSelfKnowledgeProvider{
		agentName:      "Phase5Cat",
		modelName:      "gpt-4o",
		providerName:   "copilot",
		channelType:    "telegram",
		daemonStarted:  time.Now().Add(-1 * time.Hour),
		activeSkills:   []skills.Skill{{Name: "weather"}, {Name: "coding"}},
		inactiveSkills: inactiveList,
		costTracker:    nil,
		userID:         "user-phase5",
	}

	extras := &agentapi.SelfKnowledgeExtras{
		Roles: []agentapi.RoleView{
			{Name: "wizard", Priority: 30, KeywordCount: 5},
			{Name: "oracle", Priority: 100, KeywordCount: 0},
		},
		SkillInventory: &skills.Inventory{
			Active: []skills.Skill{{Name: "weather"}, {Name: "coding"}},
			Inactive: []skills.InactiveSkill{
				{Name: "linkedin", MissingEnv: []string{"LINKEDIN_LI_AT"}},
				{Name: "twitter", MissingBins: []string{"bird"}},
				{Name: "tiktok", MissingEnv: []string{"TIKTOK_TOKEN"}, MissingBins: []string{"tiktok-cli"}},
			},
		},
		// SchedulerSubsystem is nil — scheduler will appear disabled
	}

	tool := NewAgentSelfStatusTool(provider, nil, extras)

	// Verify backward-compatible tool name
	if tool.Name() != "agent_self_status" {
		t.Fatalf("Name() = %q, want agent_self_status", tool.Name())
	}

	result, err := tool.Execute(ctx, json.RawMessage(`{"full":false}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// --- Verify roles appear in output ---
	if !strings.Contains(result, "wizard") {
		t.Error("output should contain role 'wizard'")
	}
	if !strings.Contains(result, "oracle") {
		t.Error("output should contain role 'oracle'")
	}

	// --- Verify inactive skill summaries use safe reason categories ---
	if !strings.Contains(result, "missing_env") {
		t.Error("output should contain safe reason category 'missing_env'")
	}
	if !strings.Contains(result, "missing_binary") {
		t.Error("output should contain safe reason category 'missing_binary'")
	}
	// Raw env var names must NOT appear
	if strings.Contains(result, "LINKEDIN_LI_AT") {
		t.Error("output must NOT contain raw env var name LINKEDIN_LI_AT")
	}
	if strings.Contains(result, "TIKTOK_TOKEN") {
		t.Error("output must NOT contain raw env var name TIKTOK_TOKEN")
	}

	// --- Verify scheduler disabled when subsystem nil ---
	if strings.Contains(result, `"scheduler_enabled": true`) {
		t.Error("scheduler_enabled should be false when SchedulerSubsystem is nil")
	}

	// --- Verify inactive_skill_summaries field present ---
	if !strings.Contains(result, "inactive_skill_summaries") {
		t.Error("output should contain inactive_skill_summaries field")
	}

	// --- Verify roles in human-readable section ---
	if !strings.Contains(result, "--- Roles ---") {
		t.Error("output should contain '--- Roles ---' section")
	}
	if !strings.Contains(result, "priority 30") {
		t.Error("output should contain 'priority 30' for wizard role")
	}
}

// TestAgentSelfStatusNilExtrasBackwardCompat verifies the tool works without extras (pre-Phase 5).
func TestAgentSelfStatusNilExtrasBackwardCompat(t *testing.T) {
	ctx := context.Background()

	provider := &stubSelfKnowledgeProvider{
		agentName:     "LegacyCat",
		modelName:     "gpt-3.5-turbo",
		providerName:  "openai",
		channelType:   "discord",
		daemonStarted: time.Now().Add(-5 * time.Minute),
		activeSkills:  []skills.Skill{{Name: "weather"}},
		costTracker:   nil,
		userID:        "user-legacy",
	}

	// No extras — backward compatible call
	tool := NewAgentSelfStatusTool(provider, nil)

	result, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// Should not contain Phase 5 sections
	if strings.Contains(result, "--- Roles ---") {
		t.Error("nil-extras output should NOT contain roles section")
	}
	if strings.Contains(result, "inactive_skill_summaries") {
		t.Error("nil-extras output should NOT contain inactive_skill_summaries")
	}

	// Basic fields should still be present
	if !strings.Contains(result, "LegacyCat") {
		t.Error("output should contain agent_name")
	}
}
