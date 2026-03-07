package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/observability"
	"github.com/startower-observability/blackcat/internal/skills"
)

// testSelfKnowledgeProvider is a stub implementing SelfKnowledgeProvider for tests.
type testSelfKnowledgeProvider struct {
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

func (t *testSelfKnowledgeProvider) GetAgentName() string            { return t.agentName }
func (t *testSelfKnowledgeProvider) GetModelName() string            { return t.modelName }
func (t *testSelfKnowledgeProvider) GetProviderName() string         { return t.providerName }
func (t *testSelfKnowledgeProvider) GetChannelType() string          { return t.channelType }
func (t *testSelfKnowledgeProvider) GetDaemonStartedAt() time.Time   { return t.daemonStarted }
func (t *testSelfKnowledgeProvider) GetActiveSkills() []skills.Skill { return t.activeSkills }
func (t *testSelfKnowledgeProvider) GetInactiveSkills() []skills.InactiveSkill {
	return t.inactiveSkills
}
func (t *testSelfKnowledgeProvider) GetCostTracker() *observability.CostTracker { return t.costTracker }
func (t *testSelfKnowledgeProvider) GetUserID() string                          { return t.userID }

func TestSelfKnowledgeSnapshotCompact(t *testing.T) {
	ctx := context.Background()
	provider := &testSelfKnowledgeProvider{
		agentName:     "TestCat",
		modelName:     "gpt-4o",
		providerName:  "openai",
		channelType:   "telegram",
		daemonStarted: time.Now().Add(-15 * time.Minute),
		activeSkills: []skills.Skill{
			{Name: "weather"},
			{Name: "calendar"},
			{Name: "search"},
		},
		inactiveSkills: []skills.InactiveSkill{
			{Name: "linkedin", Reason: "missing env LINKEDIN_LI_AT"},
			{Name: "twitter", Reason: "missing binary bird"},
		},
		costTracker: nil, // nil tracker → "unavailable"
		userID:      "user-1",
	}

	snap := BuildSelfKnowledgeSnapshot(ctx, provider, false, nil)

	// FullMode must be false
	if snap.FullMode {
		t.Error("expected FullMode=false for compact snapshot")
	}

	// InactiveSkills slice must be empty in compact mode (only count populated)
	if len(snap.InactiveSkills) != 0 {
		t.Errorf("expected InactiveSkills to be empty in compact mode, got %d", len(snap.InactiveSkills))
	}

	// InactiveSkillCount should still reflect the provider's count
	if snap.InactiveSkillCount != 2 {
		t.Errorf("expected InactiveSkillCount=2, got %d", snap.InactiveSkillCount)
	}

	// ActiveSkillCount and names
	if snap.ActiveSkillCount != 3 {
		t.Errorf("expected ActiveSkillCount=3, got %d", snap.ActiveSkillCount)
	}
	expectedNames := []string{"weather", "calendar", "search"}
	if len(snap.ActiveSkillNames) != len(expectedNames) {
		t.Fatalf("expected %d active skill names, got %d", len(expectedNames), len(snap.ActiveSkillNames))
	}
	for i, name := range expectedNames {
		if snap.ActiveSkillNames[i] != name {
			t.Errorf("ActiveSkillNames[%d]: expected %q, got %q", i, name, snap.ActiveSkillNames[i])
		}
	}

	// Version fields populated from version package
	if snap.Version == "" {
		t.Error("expected Version to be non-empty")
	}
	if snap.Commit == "" {
		t.Error("expected Commit to be non-empty")
	}

	// Identity fields
	if snap.AgentName != "TestCat" {
		t.Errorf("expected AgentName=TestCat, got %q", snap.AgentName)
	}

	// TokenUsage24h should be "unavailable" when cost tracker is nil
	if snap.TokenUsage24h != "unavailable" {
		t.Errorf("expected TokenUsage24h=unavailable with nil tracker, got %q", snap.TokenUsage24h)
	}

	// CacheUsage is always "unavailable"
	if snap.CacheUsage != "unavailable" {
		t.Errorf("expected CacheUsage=unavailable, got %q", snap.CacheUsage)
	}

	// DaemonUptime should be a valid duration string (not "unavailable")
	if snap.DaemonUptime == "unavailable" {
		t.Error("expected DaemonUptime to be a valid duration, got unavailable")
	}

	// CompactSummary should contain key information
	summary := snap.CompactSummary()
	if !strings.Contains(summary, snap.Version) {
		t.Errorf("CompactSummary() should contain version %q:\n%s", snap.Version, summary)
	}
	if !strings.Contains(summary, "Active skills: 3") {
		t.Errorf("CompactSummary() should contain 'Active skills: 3':\n%s", summary)
	}
	if !strings.Contains(summary, "Inactive skills: 2") {
		t.Errorf("CompactSummary() should contain 'Inactive skills: 2':\n%s", summary)
	}
	if !strings.Contains(summary, "Cache usage: unavailable") {
		t.Errorf("CompactSummary() should contain 'Cache usage: unavailable':\n%s", summary)
	}
	if !strings.Contains(summary, "weather") {
		t.Errorf("CompactSummary() should list skill names:\n%s", summary)
	}
}

func TestSelfKnowledgeSnapshotFull(t *testing.T) {
	ctx := context.Background()

	inactiveList := []skills.InactiveSkill{
		{Name: "linkedin", FilePath: "/skills/linkedin.md", Reason: "missing env", MissingEnv: []string{"LINKEDIN_LI_AT"}},
		{Name: "twitter", FilePath: "/skills/twitter.md", Reason: "missing binary", MissingBins: []string{"bird"}},
		{Name: "tiktok", FilePath: "/skills/tiktok.md", Reason: "missing env", MissingEnv: []string{"TIKTOK_ACCESS_TOKEN"}},
	}

	provider := &testSelfKnowledgeProvider{
		agentName:     "BlackCat",
		modelName:     "claude-3-5-sonnet",
		providerName:  "anthropic",
		channelType:   "discord",
		daemonStarted: time.Time{}, // zero value → unavailable
		activeSkills: []skills.Skill{
			{Name: "weather"},
		},
		inactiveSkills: inactiveList,
		costTracker:    nil,
		userID:         "user-2",
	}

	snap := BuildSelfKnowledgeSnapshot(ctx, provider, true, nil)

	// FullMode must be true
	if !snap.FullMode {
		t.Error("expected FullMode=true for full snapshot")
	}

	// InactiveSkills must be populated in full mode
	if len(snap.InactiveSkills) != 3 {
		t.Errorf("expected 3 InactiveSkills in full mode, got %d", len(snap.InactiveSkills))
	}

	// Verify inactive skill details
	if snap.InactiveSkills[0].Name != "linkedin" {
		t.Errorf("expected first inactive skill=linkedin, got %q", snap.InactiveSkills[0].Name)
	}

	// InactiveSkillCount
	if snap.InactiveSkillCount != 3 {
		t.Errorf("expected InactiveSkillCount=3, got %d", snap.InactiveSkillCount)
	}

	// CacheUsage is ALWAYS "unavailable"
	if snap.CacheUsage != "unavailable" {
		t.Errorf("expected CacheUsage=unavailable, got %q", snap.CacheUsage)
	}

	// DaemonUptime should be "unavailable" with zero time
	if snap.DaemonUptime != "unavailable" {
		t.Errorf("expected DaemonUptime=unavailable with zero time, got %q", snap.DaemonUptime)
	}

	// UnavailableFields should contain DaemonUptime and CacheUsage
	hasDaemon := false
	hasCache := false
	for _, f := range snap.UnavailableFields {
		if f == "DaemonUptime" {
			hasDaemon = true
		}
		if f == "CacheUsage" {
			hasCache = true
		}
	}
	if !hasDaemon {
		t.Error("UnavailableFields should contain DaemonUptime when daemon start is zero")
	}
	if !hasCache {
		t.Error("UnavailableFields should always contain CacheUsage")
	}

	// ProcessUptime should always be a valid duration
	if snap.ProcessUptime == "" || snap.ProcessUptime == "unavailable" {
		t.Errorf("expected ProcessUptime to be a valid duration, got %q", snap.ProcessUptime)
	}

	// Identity
	if snap.ModelName != "claude-3-5-sonnet" {
		t.Errorf("expected ModelName=claude-3-5-sonnet, got %q", snap.ModelName)
	}
	if snap.ProviderName != "anthropic" {
		t.Errorf("expected ProviderName=anthropic, got %q", snap.ProviderName)
	}
}

func TestSelfKnowledgeFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{15 * time.Minute, "15m"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
		{48 * time.Hour, "48h 0m"},
		{-5 * time.Minute, "0s"}, // negative clamped to zero
	}
	for _, tc := range tests {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v): got %q, want %q", tc.d, got, tc.want)
		}
	}
}
