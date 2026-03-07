package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadSkills creates temp dir with 2 .md files, loads, verifies names and content
func TestLoadSkills(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first skill file
	skill1Content := `# Coding
Tags: programming, best-practices

This is content for the coding skill.
Learn how to write better code.`

	err := os.WriteFile(filepath.Join(tmpDir, "coding.md"), []byte(skill1Content), 0644)
	if err != nil {
		t.Fatalf("failed to write skill1: %v", err)
	}

	// Create second skill file
	skill2Content := `# Research
Tags: analysis, documentation

This is content for the research skill.
How to conduct thorough research.`

	err = os.WriteFile(filepath.Join(tmpDir, "research.md"), []byte(skill2Content), 0644)
	if err != nil {
		t.Fatalf("failed to write skill2: %v", err)
	}

	// Load skills
	skills, err := LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("LoadSkills failed: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	// Check sorted order (alphabetical)
	if skills[0].Name != "Coding" {
		t.Fatalf("expected first skill name 'Coding', got '%s'", skills[0].Name)
	}

	if skills[1].Name != "Research" {
		t.Fatalf("expected second skill name 'Research', got '%s'", skills[1].Name)
	}

	// Verify content is after heading
	if !strings.Contains(skills[0].Content, "This is content for the coding skill") {
		t.Fatalf("skill[0] content missing expected text")
	}

	if !strings.Contains(skills[1].Content, "This is content for the research skill") {
		t.Fatalf("skill[1] content missing expected text")
	}
}

// TestLoadSkillsWithTags creates .md file with Tags line, verifies tags parsed
func TestLoadSkillsWithTags(t *testing.T) {
	tmpDir := t.TempDir()

	skillContent := `# Testing
Tags: qa, automation, testing

Content about testing skills.`

	err := os.WriteFile(filepath.Join(tmpDir, "test_skill.md"), []byte(skillContent), 0644)
	if err != nil {
		t.Fatalf("failed to write skill: %v", err)
	}

	skills, err := LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("LoadSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	skill := skills[0]

	if len(skill.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(skill.Tags), skill.Tags)
	}

	expectedTags := []string{"qa", "automation", "testing"}
	for i, tag := range expectedTags {
		if i >= len(skill.Tags) || skill.Tags[i] != tag {
			t.Fatalf("expected tag '%s' at position %d, got '%v'", tag, i, skill.Tags)
		}
	}
}

// TestLoadSkillsEmptyDir empty dir returns empty slice
func TestLoadSkillsEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	skills, err := LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("LoadSkills failed: %v", err)
	}

	if len(skills) != 0 {
		t.Fatalf("expected 0 skills for empty dir, got %d", len(skills))
	}
}

// TestLoadSkillsMissingDir non-existent dir returns empty slice (not error)
func TestLoadSkillsMissingDir(t *testing.T) {
	nonExistentDir := "/nonexistent/path/that/does/not/exist/12345"

	skills, err := LoadSkills(nonExistentDir)
	if err != nil {
		t.Fatalf("LoadSkills should not return error for missing dir, got: %v", err)
	}

	if len(skills) != 0 {
		t.Fatalf("expected 0 skills for missing dir, got %d", len(skills))
	}
}

// TestFormatForInjection format 2 skills, verify XML-like output
func TestFormatForInjection(t *testing.T) {
	skills := []Skill{
		{
			Name:    "coding",
			Content: "This is coding content",
			Tags:    []string{"programming"},
		},
		{
			Name:    "research",
			Content: "This is research content",
			Tags:    []string{"analysis"},
		},
	}

	result := FormatForInjection(skills)

	// Check that output contains expected XML-like tags
	if !strings.Contains(result, `<skill name="coding">`) {
		t.Fatalf("output missing opening tag for 'coding'")
	}

	if !strings.Contains(result, `<skill name="research">`) {
		t.Fatalf("output missing opening tag for 'research'")
	}

	if !strings.Contains(result, "</skill>") {
		t.Fatalf("output missing closing tag")
	}

	if !strings.Contains(result, "This is coding content") {
		t.Fatalf("output missing coding content")
	}

	if !strings.Contains(result, "This is research content") {
		t.Fatalf("output missing research content")
	}

	// Verify proper newline separation
	if !strings.Contains(result, "</skill>\n\n<skill") {
		t.Fatalf("expected double newline between skills")
	}
}

// TestFormatForInjectionEmpty empty slice returns ""
func TestFormatForInjectionEmpty(t *testing.T) {
	skills := []Skill{}

	result := FormatForInjection(skills)

	if result != "" {
		t.Fatalf("expected empty string for empty skills slice, got: '%s'", result)
	}
}

// TestLoadSkillsNoHeading .md file without # heading uses filename as name
func TestLoadSkillsNoHeading(t *testing.T) {
	tmpDir := t.TempDir()

	skillContent := `This is content without a heading.
Just plain text about the skill.
No heading line here.`

	err := os.WriteFile(filepath.Join(tmpDir, "no_heading_skill.md"), []byte(skillContent), 0644)
	if err != nil {
		t.Fatalf("failed to write skill: %v", err)
	}

	skills, err := LoadSkills(tmpDir)
	if err != nil {
		t.Fatalf("LoadSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	// Name should be filename without extension
	if skills[0].Name != "no_heading_skill" {
		t.Fatalf("expected name 'no_heading_skill', got '%s'", skills[0].Name)
	}

	// Content should be the full file content
	if !strings.Contains(skills[0].Content, "This is content without a heading") {
		t.Fatalf("expected content to contain full file text")
	}
}

// TestIsEligibleWithAvailableBinary tests IsEligible returns true for git (usually available)
func TestIsEligibleWithAvailableBinary(t *testing.T) {
	skill := Skill{
		Name: "git skill",
		Requires: Requirements{
			Bins: []string{"git"},
		},
	}

	if !skill.IsEligible() {
		t.Error("Expected IsEligible to return true for available 'git' binary")
	}
}

// TestIsEligibleWithMissingBinary tests IsEligible returns false for nonexistent binary
func TestIsEligibleWithMissingBinary(t *testing.T) {
	skill := Skill{
		Name: "fake skill",
		Requires: Requirements{
			Bins: []string{"this_binary_does_not_exist_12345"},
		},
	}

	if skill.IsEligible() {
		t.Error("Expected IsEligible to return false for missing binary")
	}
}

// TestIsEligibleWithEnvironmentVariable tests env var requirement checking
func TestIsEligibleWithEnvironmentVariable(t *testing.T) {
	// Set env var for this test
	t.Setenv("TEST_SKILL_VAR", "test_value")

	skill := Skill{
		Name: "env var skill",
		Requires: Requirements{
			Env: []string{"TEST_SKILL_VAR"},
		},
	}

	if !skill.IsEligible() {
		t.Error("Expected IsEligible to return true when env var is set")
	}
}

// TestIsEligibleWithMissingEnvironmentVariable tests missing env var
func TestIsEligibleWithMissingEnvironmentVariable(t *testing.T) {
	skill := Skill{
		Name: "env var skill",
		Requires: Requirements{
			Env: []string{"NONEXISTENT_VAR_12345"},
		},
	}

	if skill.IsEligible() {
		t.Error("Expected IsEligible to return false for missing env var")
	}
}

// TestIsEligibleMultipleRequirements tests multiple requirements (all must pass)
func TestIsEligibleMultipleRequirements(t *testing.T) {
	t.Setenv("TEST_MULTI_VAR", "value")

	// All available: should pass
	skill := Skill{
		Name: "multi skill",
		Requires: Requirements{
			Bins: []string{"git"},
			Env:  []string{"TEST_MULTI_VAR"},
		},
	}

	if !skill.IsEligible() {
		t.Error("Expected IsEligible to return true when all requirements met")
	}

	// One unavailable: should fail
	skill.Requires.Bins = append(skill.Requires.Bins, "nonexistent_12345")
	if skill.IsEligible() {
		t.Error("Expected IsEligible to return false when any requirement unmet")
	}
}

// TestLoadSkillsFromMultipleSourcesDeduplication tests first occurrence wins
func TestLoadSkillsFromMultipleSourcesDeduplication(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Create same skill name in both directories with different content
	skill1Content := `# Duplicate Skill
\nContent from dir1.`
	skill2Content := `# Duplicate Skill
\nContent from dir2.`

	err := os.WriteFile(filepath.Join(dir1, "duplicate.md"), []byte(skill1Content), 0644)
	if err != nil {
		t.Fatalf("failed to write skill1: %v", err)
	}

	err = os.WriteFile(filepath.Join(dir2, "duplicate.md"), []byte(skill2Content), 0644)
	if err != nil {
		t.Fatalf("failed to write skill2: %v", err)
	}

	// Load from both dirs (dir1 first, should take precedence)
	skills, err := LoadSkillsFromMultipleSources([]string{dir1, dir2})
	if err != nil {
		t.Fatalf("LoadSkillsFromMultipleSources failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (deduplicated), got %d", len(skills))
	}

	if !strings.Contains(skills[0].Content, "Content from dir1") {
		t.Error("Expected first occurrence (dir1) to be used")
	}
}

// TestLoadSkillsFromMultipleSourcesEligibilityGating tests filtering by requirements
func TestLoadSkillsFromMultipleSourcesEligibilityGating(t *testing.T) {
	tmpDir := t.TempDir()

	// Create eligible skill (no requirements)
	eligibleContent := `---
name: Eligible Skill
requires:
  bins: []
  env: []
---
This skill is eligible.`

	// Create ineligible skill (missing binary requirement)
	ineligibleContent := `---
name: Ineligible Skill
requires:
  bins:
    - nonexistent_binary_12345
---
This skill is ineligible.`

	err := os.WriteFile(filepath.Join(tmpDir, "eligible.md"), []byte(eligibleContent), 0644)
	if err != nil {
		t.Fatalf("failed to write eligible skill: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "ineligible.md"), []byte(ineligibleContent), 0644)
	if err != nil {
		t.Fatalf("failed to write ineligible skill: %v", err)
	}

	// Load skills (should only include eligible one)
	skills, err := LoadSkillsFromMultipleSources([]string{tmpDir})
	if err != nil {
		t.Fatalf("LoadSkillsFromMultipleSources failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 eligible skill, got %d", len(skills))
	}

	if skills[0].Name != "Eligible Skill" {
		t.Errorf("expected 'Eligible Skill', got '%s'", skills[0].Name)
	}
}

// TestFilterByFileSize tests filtering skills by content size.
func TestFilterByFileSize(t *testing.T) {
	skills := []Skill{
		{Name: "small", Content: "abc"},                    // 3 bytes
		{Name: "medium", Content: "abcdefghij"},            // 10 bytes
		{Name: "large", Content: strings.Repeat("x", 100)}, // 100 bytes
	}

	t.Run("filters out skills exceeding maxBytes", func(t *testing.T) {
		result := FilterByFileSize(skills, 10)
		if len(result) != 2 {
			t.Fatalf("expected 2 skills, got %d", len(result))
		}
		if result[0].Name != "small" || result[1].Name != "medium" {
			t.Fatalf("expected [small, medium], got [%s, %s]", result[0].Name, result[1].Name)
		}
	})

	t.Run("includes skills exactly at maxBytes", func(t *testing.T) {
		result := FilterByFileSize(skills, 3)
		if len(result) != 1 {
			t.Fatalf("expected 1 skill at exactly 3 bytes, got %d", len(result))
		}
		if result[0].Name != "small" {
			t.Fatalf("expected 'small', got '%s'", result[0].Name)
		}
	})

	t.Run("maxBytes zero returns all skills", func(t *testing.T) {
		result := FilterByFileSize(skills, 0)
		if len(result) != 3 {
			t.Fatalf("expected 3 skills with maxBytes=0, got %d", len(result))
		}
	})

	t.Run("maxBytes negative returns all skills", func(t *testing.T) {
		result := FilterByFileSize(skills, -1)
		if len(result) != 3 {
			t.Fatalf("expected 3 skills with maxBytes=-1, got %d", len(result))
		}
	})

	t.Run("empty skills slice returns empty", func(t *testing.T) {
		result := FilterByFileSize([]Skill{}, 100)
		if len(result) != 0 {
			t.Fatalf("expected 0 skills for empty input, got %d", len(result))
		}
	})

	t.Run("large maxBytes returns all", func(t *testing.T) {
		result := FilterByFileSize(skills, 1000)
		if len(result) != 3 {
			t.Fatalf("expected 3 skills with large maxBytes, got %d", len(result))
		}
	})
}

// TestLimitSkillCount tests limiting the number of skills returned.
func TestLimitSkillCount(t *testing.T) {
	skills := []Skill{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
		{Name: "d"},
		{Name: "e"},
	}

	t.Run("maxCount zero returns all", func(t *testing.T) {
		result := LimitSkillCount(skills, 0)
		if len(result) != 5 {
			t.Fatalf("expected 5 skills with maxCount=0, got %d", len(result))
		}
	})

	t.Run("maxCount negative returns all", func(t *testing.T) {
		result := LimitSkillCount(skills, -1)
		if len(result) != 5 {
			t.Fatalf("expected 5 skills with maxCount=-1, got %d", len(result))
		}
	})

	t.Run("maxCount less than len truncates", func(t *testing.T) {
		result := LimitSkillCount(skills, 3)
		if len(result) != 3 {
			t.Fatalf("expected 3 skills, got %d", len(result))
		}
		if result[0].Name != "a" || result[1].Name != "b" || result[2].Name != "c" {
			t.Fatalf("expected [a, b, c], got [%s, %s, %s]", result[0].Name, result[1].Name, result[2].Name)
		}
	})

	t.Run("maxCount equal to len returns all", func(t *testing.T) {
		result := LimitSkillCount(skills, 5)
		if len(result) != 5 {
			t.Fatalf("expected 5 skills with maxCount=len, got %d", len(result))
		}
	})

	t.Run("maxCount greater than len returns all", func(t *testing.T) {
		result := LimitSkillCount(skills, 10)
		if len(result) != 5 {
			t.Fatalf("expected 5 skills with maxCount>len, got %d", len(result))
		}
	})

	t.Run("empty skills slice returns empty", func(t *testing.T) {
		result := LimitSkillCount([]Skill{}, 5)
		if len(result) != 0 {
			t.Fatalf("expected 0 skills for empty input, got %d", len(result))
		}
	})

	t.Run("maxCount 1 returns first skill only", func(t *testing.T) {
		result := LimitSkillCount(skills, 1)
		if len(result) != 1 {
			t.Fatalf("expected 1 skill, got %d", len(result))
		}
		if result[0].Name != "a" {
			t.Fatalf("expected 'a', got '%s'", result[0].Name)
		}
	})
}

// TestValidateVersion_Valid valid semver "v1.2.3" returns canonical form
func TestValidateVersion_Valid(t *testing.T) {
	canonical, err := ValidateVersion("v1.2.3")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if canonical != "v1.2.3" {
		t.Fatalf("expected \"v1.2.3\", got %q", canonical)
	}
}

// TestValidateVersion_Invalid missing "v" prefix returns error
func TestValidateVersion_Invalid(t *testing.T) {
	_, err := ValidateVersion("1.2.3")
	if err == nil {
		t.Fatal("expected error for missing 'v' prefix, got nil")
	}
	if !strings.Contains(err.Error(), "invalid semver") {
		t.Fatalf("expected error to contain 'invalid semver', got: %v", err)
	}
}

// TestValidateVersion_Empty empty string returns ("", nil)
func TestValidateVersion_Empty(t *testing.T) {
	canonical, err := ValidateVersion("")
	if err != nil {
		t.Fatalf("expected no error for empty string, got: %v", err)
	}
	if canonical != "" {
		t.Fatalf("expected empty string, got %q", canonical)
	}
}

// TestCheckAnyBins table-driven tests for checkAnyBins
func TestCheckAnyBins(t *testing.T) {
	tests := []struct {
		name    string
		anyBins [][]string
		want    bool
	}{
		{
			name:    "empty anyBins nil",
			anyBins: nil,
			want:    true,
		},
		{
			name:    "empty anyBins slice",
			anyBins: [][]string{},
			want:    true,
		},
		{
			name:    "single group found",
			anyBins: [][]string{{"go"}},
			want:    true,
		},
		{
			name:    "single group not found",
			anyBins: [][]string{{"__nonexistent_binary__"}},
			want:    false,
		},
		{
			name:    "OR first matches",
			anyBins: [][]string{{"go", "__nonexistent__"}},
			want:    true,
		},
		{
			name:    "OR second matches",
			anyBins: [][]string{{"__nonexistent__", "go"}},
			want:    true,
		},
		{
			name:    "OR none match",
			anyBins: [][]string{{"__nonexistent__", "__also_nonexistent__"}},
			want:    false,
		},
		{
			name:    "multi group all satisfied",
			anyBins: [][]string{{"go"}, {"go"}},
			want:    true,
		},
		{
			name:    "multi group one fails",
			anyBins: [][]string{{"go"}, {"__nonexistent__"}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkAnyBins(tt.anyBins)
			if got != tt.want {
				t.Errorf("checkAnyBins(%v) = %v, want %v", tt.anyBins, got, tt.want)
			}
		})
	}
}

// TestGetBinaryVersion_ExistingBinary "go" should return a non-empty version string
func TestGetBinaryVersion_ExistingBinary(t *testing.T) {
	v := getBinaryVersion("go")
	if v == "" {
		t.Error("expected non-empty version for 'go' binary")
	}
}

// TestGetBinaryVersion_NonexistentBinary nonexistent binary returns empty string
func TestGetBinaryVersion_NonexistentBinary(t *testing.T) {
	v := getBinaryVersion("__nonexistent_binary_xyz__")
	if v != "" {
		t.Errorf("expected empty version for nonexistent binary, got %q", v)
	}
}

// TestLoadSkillInventory tests that LoadSkillInventory scans once and classifies skills
func TestLoadSkillInventory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create eligible skill (no requirements)
	eligibleContent := `---
name: Eligible Skill
requires:
  bins: []
  env: []
---
This skill is eligible.`

	// Create ineligible skill (missing binary)
	ineligibleContent := `---
name: Ineligible Skill
requires:
  bins:
    - nonexistent_binary_12345
---
This skill is ineligible.`

	err := os.WriteFile(filepath.Join(tmpDir, "eligible.md"), []byte(eligibleContent), 0644)
	if err != nil {
		t.Fatalf("failed to write eligible skill: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "ineligible.md"), []byte(ineligibleContent), 0644)
	if err != nil {
		t.Fatalf("failed to write ineligible skill: %v", err)
	}

	// Load inventory
	inventory, err := LoadSkillInventory([]string{tmpDir})
	if err != nil {
		t.Fatalf("LoadSkillInventory failed: %v", err)
	}

	// Verify active count
	if len(inventory.Active) != 1 {
		t.Fatalf("expected 1 active skill, got %d", len(inventory.Active))
	}

	if inventory.Active[0].Name != "Eligible Skill" {
		t.Errorf("expected 'Eligible Skill', got '%s'", inventory.Active[0].Name)
	}

	// Verify inactive count and reason
	if len(inventory.Inactive) != 1 {
		t.Fatalf("expected 1 inactive skill, got %d", len(inventory.Inactive))
	}

	inactiveSkill := inventory.Inactive[0]
	if inactiveSkill.Name != "Ineligible Skill" {
		t.Errorf("expected 'Ineligible Skill', got '%s'", inactiveSkill.Name)
	}

	// Verify reason string format
	if !strings.Contains(inactiveSkill.Reason, "missing binary: nonexistent_binary_12345") {
		t.Errorf("expected reason to contain 'missing binary: nonexistent_binary_12345', got '%s'", inactiveSkill.Reason)
	}

	// Verify missing bins list
	if len(inactiveSkill.MissingBins) != 1 || inactiveSkill.MissingBins[0] != "nonexistent_binary_12345" {
		t.Errorf("expected MissingBins=[nonexistent_binary_12345], got %v", inactiveSkill.MissingBins)
	}
}

// TestGetInactiveSkills tests that GetInactiveSkills returns only inactive skills
func TestGetInactiveSkills(t *testing.T) {
	tmpDir := t.TempDir()

	// Create eligible skill
	eligibleContent := `---
name: Eligible
requires:
  bins: []
  env: []
---
Eligible content.`

	// Create ineligible skill with missing binary
	ineligibleBinContent := `---
name: Missing Binary
requires:
  bins:
    - fake_bin_xyz
---
Missing binary.`

	// Create ineligible skill with missing env
	ineligibleEnvContent := `---
name: Missing Env
requires:
  bins: []
  env:
    - FAKE_ENV_VAR_12345
---
Missing env var.`

	err := os.WriteFile(filepath.Join(tmpDir, "eligible.md"), []byte(eligibleContent), 0644)
	if err != nil {
		t.Fatalf("failed to write eligible: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "missing_bin.md"), []byte(ineligibleBinContent), 0644)
	if err != nil {
		t.Fatalf("failed to write missing bin: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "missing_env.md"), []byte(ineligibleEnvContent), 0644)
	if err != nil {
		t.Fatalf("failed to write missing env: %v", err)
	}

	// Get inactive skills
	inactiveSkills, err := GetInactiveSkills([]string{tmpDir})
	if err != nil {
		t.Fatalf("GetInactiveSkills failed: %v", err)
	}

	if len(inactiveSkills) != 2 {
		t.Fatalf("expected 2 inactive skills, got %d", len(inactiveSkills))
	}

	// Verify binary reason
	binSkill := inactiveSkills[0]
	if !strings.Contains(binSkill.Reason, "missing binary: fake_bin_xyz") {
		t.Errorf("expected reason to mention missing binary, got '%s'", binSkill.Reason)
	}

	// Verify env reason
	envSkill := inactiveSkills[1]
	if !strings.Contains(envSkill.Reason, "missing env var: FAKE_ENV_VAR_12345") {
		t.Errorf("expected reason to mention missing env var, got '%s'", envSkill.Reason)
	}
}

// TestLoadSkillInventoryDeduplication tests that earlier dirs take precedence
func TestLoadSkillInventoryDeduplication(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Create same skill name in both directories with different content
	skill1Content := `---
name: Duplicate Skill
requires:
  bins: []
  env: []
---
Content from dir1.`

	skill2Content := `---
name: Duplicate Skill
requires:
  bins: []
  env: []
---
Content from dir2.`

	err := os.WriteFile(filepath.Join(dir1, "duplicate.md"), []byte(skill1Content), 0644)
	if err != nil {
		t.Fatalf("failed to write skill1: %v", err)
	}

	err = os.WriteFile(filepath.Join(dir2, "duplicate.md"), []byte(skill2Content), 0644)
	if err != nil {
		t.Fatalf("failed to write skill2: %v", err)
	}

	// Load inventory from both dirs (dir1 first)
	inventory, err := LoadSkillInventory([]string{dir1, dir2})
	if err != nil {
		t.Fatalf("LoadSkillInventory failed: %v", err)
	}

	// Should only have 1 active skill (dir1 version)
	if len(inventory.Active) != 1 {
		t.Fatalf("expected 1 active skill (deduplicated), got %d", len(inventory.Active))
	}

	// Verify it's the dir1 content
	if !strings.Contains(inventory.Active[0].Content, "Content from dir1") {
		t.Error("Expected first occurrence (dir1) to be used")
	}

	// No inactive skills
	if len(inventory.Inactive) != 0 {
		t.Fatalf("expected 0 inactive skills, got %d", len(inventory.Inactive))
	}
}
