package agentapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/observability"
	"github.com/startower-observability/blackcat/internal/skills"
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

// stubProvider is a minimal SelfKnowledgeProvider for agentapi-level tests.
type stubProvider struct {
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

func (s *stubProvider) GetAgentName() string                       { return s.agentName }
func (s *stubProvider) GetModelName() string                       { return s.modelName }
func (s *stubProvider) GetProviderName() string                    { return s.providerName }
func (s *stubProvider) GetChannelType() string                     { return s.channelType }
func (s *stubProvider) GetDaemonStartedAt() time.Time              { return s.daemonStarted }
func (s *stubProvider) GetActiveSkills() []skills.Skill            { return s.activeSkills }
func (s *stubProvider) GetInactiveSkills() []skills.InactiveSkill  { return s.inactiveSkills }
func (s *stubProvider) GetCostTracker() *observability.CostTracker { return s.costTracker }
func (s *stubProvider) GetUserID() string                          { return s.userID }

// TestBuildFullRuntimeSelfKnowledgeSnapshot tests Phase 5 enrichment via SelfKnowledgeExtras.
func TestBuildFullRuntimeSelfKnowledgeSnapshot(t *testing.T) {
	ctx := context.Background()
	provider := &stubProvider{
		agentName:     "TestCat",
		modelName:     "gpt-4o",
		providerName:  "openai",
		channelType:   "telegram",
		daemonStarted: time.Now().Add(-10 * time.Minute),
		activeSkills:  []skills.Skill{{Name: "weather"}},
	}

	roles := []RoleView{
		{Name: "wizard", Priority: 30, KeywordCount: 5},
		{Name: "oracle", Priority: 100, KeywordCount: 0},
	}
	catalog := []CatalogEntry{
		{
			Provider: "openai",
			Models:   []ProviderModelView{{ID: "gpt-4o", Name: "GPT-4o"}},
		},
	}

	extras := &SelfKnowledgeExtras{
		Roles:           roles,
		ProviderCatalog: catalog,
		// SkillInventory and SchedulerSubsystem left nil to test partial enrichment
	}

	snap := BuildSelfKnowledgeSnapshot(ctx, provider, false, extras, nil)

	// Roles must be populated from extras
	if len(snap.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(snap.Roles))
	}
	if snap.Roles[0].Name != "wizard" {
		t.Errorf("expected first role=wizard, got %q", snap.Roles[0].Name)
	}
	if snap.Roles[1].Name != "oracle" {
		t.Errorf("expected second role=oracle, got %q", snap.Roles[1].Name)
	}

	// Provider catalog must be populated from extras
	if len(snap.ProviderCatalog) != 1 {
		t.Fatalf("expected 1 catalog entry, got %d", len(snap.ProviderCatalog))
	}
	if snap.ProviderCatalog[0].Provider != "openai" {
		t.Errorf("expected catalog provider=openai, got %q", snap.ProviderCatalog[0].Provider)
	}

	// Scheduler not provided — should remain defaults
	if snap.SchedulerEnabled {
		t.Error("expected SchedulerEnabled=false when SchedulerSubsystem is nil in extras")
	}
	if snap.SchedulerTaskCount != 0 {
		t.Errorf("expected SchedulerTaskCount=0, got %d", snap.SchedulerTaskCount)
	}

	// Base fields must still be populated
	if snap.AgentName != "TestCat" {
		t.Errorf("expected AgentName=TestCat, got %q", snap.AgentName)
	}
	if snap.ActiveSkillCount != 1 {
		t.Errorf("expected ActiveSkillCount=1 (from provider, no inventory override), got %d", snap.ActiveSkillCount)
	}
}

// TestSelfKnowledgeExtrasNil ensures nil extras does not panic and existing fields populate correctly.
func TestSelfKnowledgeExtrasNil(t *testing.T) {
	ctx := context.Background()
	provider := &stubProvider{
		agentName:     "NilCat",
		modelName:     "claude-3-5-sonnet",
		providerName:  "anthropic",
		channelType:   "discord",
		daemonStarted: time.Now().Add(-5 * time.Minute),
		activeSkills: []skills.Skill{
			{Name: "coding"},
			{Name: "search"},
		},
		inactiveSkills: []skills.InactiveSkill{
			{Name: "linkedin", Reason: "missing env"},
		},
	}

	snap := BuildSelfKnowledgeSnapshot(ctx, provider, false, nil, nil)

	// Must not panic — and base fields must be populated
	if snap.AgentName != "NilCat" {
		t.Errorf("expected AgentName=NilCat, got %q", snap.AgentName)
	}
	if snap.ActiveSkillCount != 2 {
		t.Errorf("expected ActiveSkillCount=2, got %d", snap.ActiveSkillCount)
	}
	if snap.InactiveSkillCount != 1 {
		t.Errorf("expected InactiveSkillCount=1, got %d", snap.InactiveSkillCount)
	}

	// Phase 5 fields should be zero values
	if len(snap.Roles) != 0 {
		t.Errorf("expected no Roles with nil extras, got %d", len(snap.Roles))
	}
	if snap.SchedulerEnabled {
		t.Error("expected SchedulerEnabled=false with nil extras")
	}
	if len(snap.ProviderCatalog) != 0 {
		t.Errorf("expected no ProviderCatalog with nil extras, got %d", len(snap.ProviderCatalog))
	}

	// CacheUsage is always "unavailable"
	if snap.CacheUsage != "unavailable" {
		t.Errorf("expected CacheUsage=unavailable, got %q", snap.CacheUsage)
	}
}

func TestBuildSelfKnowledgeSnapshotRuntimeModelStatus(t *testing.T) {
	ctx := context.Background()
	provider := &stubProvider{
		agentName:    "RuntimeSnapCat",
		modelName:    "legacy-model",
		providerName: "legacy-provider",
	}

	holder := NewRuntimeModelHolder()
	holder.Set(RuntimeModelStatus{
		ConfiguredModel: RuntimeModelRef{CanonicalID: "anthropic/claude-opus-4-6"},
		AppliedModel:    RuntimeModelRef{CanonicalID: "anthropic/claude-opus-4-6"},
		BackendProvider: "zen",
		LastReloadError: "reload failed once",
		ReloadCount:     3,
	})

	snap := BuildSelfKnowledgeSnapshot(ctx, provider, false, nil, holder)

	if snap.RuntimeModelStatus.ConfiguredModel.CanonicalID != "anthropic/claude-opus-4-6" {
		t.Errorf("configured model mismatch: got %q", snap.RuntimeModelStatus.ConfiguredModel.CanonicalID)
	}
	if snap.RuntimeModelStatus.AppliedModel.CanonicalID != "anthropic/claude-opus-4-6" {
		t.Errorf("applied model mismatch: got %q", snap.RuntimeModelStatus.AppliedModel.CanonicalID)
	}
	if snap.RuntimeModelStatus.BackendProvider != "zen" {
		t.Errorf("backend provider mismatch: got %q", snap.RuntimeModelStatus.BackendProvider)
	}
	if snap.RuntimeModelStatus.LastReloadError != "reload failed once" {
		t.Errorf("last reload error mismatch: got %q", snap.RuntimeModelStatus.LastReloadError)
	}
	if snap.RuntimeModelStatus.ReloadCount != 3 {
		t.Errorf("reload count mismatch: got %d", snap.RuntimeModelStatus.ReloadCount)
	}
}
