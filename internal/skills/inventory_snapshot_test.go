package skills

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillInventorySnapshotRedactsEnvVars(t *testing.T) {
	inv := &Inventory{
		Active: []Skill{},
		Inactive: []InactiveSkill{
			{
				Name:       "secret-skill",
				FilePath:   "/tmp/secret-skill.md",
				Reason:     "missing env var: SECRET_KEY",
				MissingEnv: []string{"SECRET_KEY"},
			},
		},
	}

	snap := BuildSkillInventorySnapshot(inv)

	if snap.InactiveCount != 1 {
		t.Fatalf("expected InactiveCount=1, got %d", snap.InactiveCount)
	}
	if len(snap.InactiveSkills) != 1 {
		t.Fatalf("expected 1 inactive skill summary, got %d", len(snap.InactiveSkills))
	}

	summary := snap.InactiveSkills[0]
	if summary.Name != "secret-skill" {
		t.Errorf("expected name 'secret-skill', got %q", summary.Name)
	}

	// Must contain the safe reason category
	foundMissingEnv := false
	for _, r := range summary.Reasons {
		if r == ReasonMissingEnv {
			foundMissingEnv = true
		}
	}
	if !foundMissingEnv {
		t.Errorf("expected reason %q in reasons %v", ReasonMissingEnv, summary.Reasons)
	}

	// Must NOT contain the raw env var name anywhere in JSON output
	jsonBytes, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	jsonStr := string(jsonBytes)
	if strings.Contains(jsonStr, "SECRET_KEY") {
		t.Errorf("snapshot JSON must not contain raw env var name 'SECRET_KEY', got: %s", jsonStr)
	}
}

func TestSkillInventorySnapshotActiveNames(t *testing.T) {
	inv := &Inventory{
		Active: []Skill{
			{Name: "coding"},
			{Name: "research"},
		},
		Inactive: []InactiveSkill{},
	}

	snap := BuildSkillInventorySnapshot(inv)

	if snap.ActiveCount != 2 {
		t.Fatalf("expected ActiveCount=2, got %d", snap.ActiveCount)
	}
	if len(snap.ActiveNames) != 2 {
		t.Fatalf("expected 2 active names, got %d", len(snap.ActiveNames))
	}

	nameSet := map[string]bool{}
	for _, n := range snap.ActiveNames {
		nameSet[n] = true
	}
	if !nameSet["coding"] {
		t.Errorf("expected 'coding' in ActiveNames, got %v", snap.ActiveNames)
	}
	if !nameSet["research"] {
		t.Errorf("expected 'research' in ActiveNames, got %v", snap.ActiveNames)
	}
}

func TestSkillInventorySnapshotInactiveReasonCategories(t *testing.T) {
	inv := &Inventory{
		Active: []Skill{},
		Inactive: []InactiveSkill{
			{
				Name:        "full-skill",
				FilePath:    "/tmp/full-skill.md",
				Reason:      "missing binary: docker; missing env var: API_KEY",
				MissingBins: []string{"docker"},
				MissingEnv:  []string{"API_KEY"},
			},
		},
	}

	snap := BuildSkillInventorySnapshot(inv)

	if len(snap.InactiveSkills) != 1 {
		t.Fatalf("expected 1 inactive skill, got %d", len(snap.InactiveSkills))
	}

	summary := snap.InactiveSkills[0]
	reasonSet := map[InactiveReason]bool{}
	for _, r := range summary.Reasons {
		reasonSet[r] = true
	}

	if !reasonSet[ReasonMissingEnv] {
		t.Errorf("expected %q in reasons, got %v", ReasonMissingEnv, summary.Reasons)
	}
	if !reasonSet[ReasonMissingBinary] {
		t.Errorf("expected %q in reasons, got %v", ReasonMissingBinary, summary.Reasons)
	}

	// Must NOT contain raw binary or env names in JSON
	jsonBytes, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	jsonStr := string(jsonBytes)
	if strings.Contains(jsonStr, "docker") {
		t.Errorf("snapshot JSON must not contain raw binary name 'docker', got: %s", jsonStr)
	}
	if strings.Contains(jsonStr, "API_KEY") {
		t.Errorf("snapshot JSON must not contain raw env var name 'API_KEY', got: %s", jsonStr)
	}
}

func TestSkillInventorySnapshotNilInventory(t *testing.T) {
	snap := BuildSkillInventorySnapshot(nil)

	if snap.ActiveCount != 0 {
		t.Errorf("expected ActiveCount=0 for nil inventory, got %d", snap.ActiveCount)
	}
	if snap.InactiveCount != 0 {
		t.Errorf("expected InactiveCount=0 for nil inventory, got %d", snap.InactiveCount)
	}
}

func TestSkillInventorySnapshotOtherReason(t *testing.T) {
	inv := &Inventory{
		Active: []Skill{},
		Inactive: []InactiveSkill{
			{
				Name:     "mystery-skill",
				FilePath: "/tmp/mystery.md",
				Reason:   "unknown issue",
				// No MissingBins or MissingEnv
			},
		},
	}

	snap := BuildSkillInventorySnapshot(inv)

	if len(snap.InactiveSkills) != 1 {
		t.Fatalf("expected 1 inactive skill, got %d", len(snap.InactiveSkills))
	}

	summary := snap.InactiveSkills[0]
	if len(summary.Reasons) != 1 || summary.Reasons[0] != ReasonOther {
		t.Errorf("expected single reason %q, got %v", ReasonOther, summary.Reasons)
	}
}
