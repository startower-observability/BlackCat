package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/security"
	"github.com/startower-observability/blackcat/internal/skills"
	"github.com/startower-observability/blackcat/internal/version"
)

func TestBuildSystemPromptIncludesSelfKnowledgeSnapshot(t *testing.T) {
	ctx := context.Background()

	loop := NewLoop(LoopConfig{
		Scrubber:     security.NewScrubber(),
		AgentName:    "BlackCat",
		ModelName:    "gpt-4o",
		ProviderName: "openai",
		ChannelType:  "telegram",
		Skills: []skills.Skill{
			{Name: "weather"},
			{Name: "calendar"},
		},
		DaemonStartedAt: time.Now().Add(-30 * time.Minute),
	})

	prompt, err := loop.buildSystemPrompt(ctx)
	if err != nil {
		t.Fatalf("buildSystemPrompt() error: %v", err)
	}

	// Must contain version string from version package
	if !strings.Contains(prompt, version.Version) {
		t.Errorf("prompt should contain version %q, got:\n%s", version.Version, prompt)
	}

	// Must contain skill names
	if !strings.Contains(prompt, "weather") {
		t.Errorf("prompt should contain skill name 'weather', got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "calendar") {
		t.Errorf("prompt should contain skill name 'calendar', got:\n%s", prompt)
	}

	// Must contain cache usage (always unavailable)
	if !strings.Contains(prompt, "Cache usage: unavailable") {
		t.Errorf("prompt should contain 'Cache usage: unavailable', got:\n%s", prompt)
	}

	// Must contain "Runtime Context" header
	if !strings.Contains(prompt, "# Runtime Context") {
		t.Errorf("prompt should contain '# Runtime Context', got:\n%s", prompt)
	}

	// Must contain "Active skills: 2"
	if !strings.Contains(prompt, "Active skills: 2") {
		t.Errorf("prompt should contain 'Active skills: 2', got:\n%s", prompt)
	}
}

func TestBuildSystemPromptOmitsInactiveSkillDetails(t *testing.T) {
	ctx := context.Background()

	loop := NewLoop(LoopConfig{
		Scrubber:     security.NewScrubber(),
		AgentName:    "BlackCat",
		ModelName:    "gpt-4o",
		ProviderName: "openai",
		ChannelType:  "telegram",
		Skills: []skills.Skill{
			{Name: "weather"},
		},
		DaemonStartedAt: time.Now().Add(-10 * time.Minute),
	})

	prompt, err := loop.buildSystemPrompt(ctx)
	if err != nil {
		t.Fatalf("buildSystemPrompt() error: %v", err)
	}

	// Compact mode should NOT include inactive skill names like "linkedin" or "twitter"
	// (since GetInactiveSkills returns nil from Loop, InactiveSkillCount is 0)
	if strings.Contains(prompt, "linkedin") {
		t.Errorf("compact prompt should NOT include inactive skill name 'linkedin'")
	}
	if strings.Contains(prompt, "missing env") {
		t.Errorf("compact prompt should NOT include inactive skill reasons")
	}

	// Should still contain the inactive skills count line (0 in this case)
	if !strings.Contains(prompt, "Inactive skills: 0") {
		t.Errorf("prompt should contain 'Inactive skills: 0', got:\n%s", prompt)
	}
}
