package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInitWorkspace tests basic initialization of workspace
func TestInitWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	err := InitWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("InitWorkspace failed: %v", err)
	}

	// Verify key files exist
	expectedFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"MEMORY.md",
		filepath.Join("skills", "coding.md"),
		filepath.Join("skills", "research.md"),
		filepath.Join("skills", "self-management.md"),
		filepath.Join("skills", "opencode-ulw.md"),
		filepath.Join("skills", "opencode-start-work.md"),
		filepath.Join("skills", "opencode-handoff.md"),
		filepath.Join("skills", "pinchtab-browsing.md"),
		filepath.Join("skills", "opencode-commands.md"),
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file not found: %s", file)
		}
	}
}

// TestInitWorkspaceNoOverwrite tests that existing files are not overwritten
func TestInitWorkspaceNoOverwrite(t *testing.T) {
	tmpDir := t.TempDir()

	// First initialization
	err := InitWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("first InitWorkspace failed: %v", err)
	}

	// Modify AGENTS.md
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	originalContent := []byte("MODIFIED")
	if err := os.WriteFile(agentsPath, originalContent, 0644); err != nil {
		t.Fatalf("failed to modify AGENTS.md: %v", err)
	}

	// Second initialization (should not overwrite)
	err = InitWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("second InitWorkspace failed: %v", err)
	}

	// Verify file was not overwritten
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}

	if string(content) != "MODIFIED" {
		t.Error("AGENTS.md was overwritten despite existing file")
	}
}

// TestInitWorkspaceSubdirs tests that subdirectories are created
func TestInitWorkspaceSubdirs(t *testing.T) {
	tmpDir := t.TempDir()

	err := InitWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("InitWorkspace failed: %v", err)
	}

	// Verify skills subdirectory exists
	skillsDir := filepath.Join(tmpDir, "skills")
	if info, err := os.Stat(skillsDir); err != nil || !info.IsDir() {
		t.Errorf("skills subdirectory not created or is not a directory")
	}

	// Verify skills files exist
	expectedSkills := []string{
		"coding.md",
		"research.md",
		"self-management.md",
		"opencode-ulw.md",
		"opencode-start-work.md",
		"opencode-handoff.md",
		"pinchtab-browsing.md",
		"opencode-commands.md",
	}
	for _, skill := range expectedSkills {
		path := filepath.Join(skillsDir, skill)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected skill file not found: %s", skill)
		}
	}
}

// TestListTemplates tests that template list is returned correctly
func TestListTemplates(t *testing.T) {
	templates := ListTemplates()

	if len(templates) == 0 {
		t.Fatal("ListTemplates returned empty list")
	}

	// Check for expected templates
	expectedCount := 11 // AGENTS.md, SOUL.md, MEMORY.md, and eight skill templates
	if len(templates) != expectedCount {
		t.Errorf("expected %d templates, got %d", expectedCount, len(templates))
	}

	// Verify specific templates are present
	templateMap := make(map[string]bool)
	for _, t := range templates {
		templateMap[t] = true
	}

	// Expected templates (using forward slashes as they come from embed.FS)
	expected := []string{
		"AGENTS.md",
		"SOUL.md",
		"MEMORY.md",
		"skills/coding.md",
		"skills/research.md",
		"skills/self-management.md",
		"skills/opencode-ulw.md",
		"skills/opencode-start-work.md",
		"skills/opencode-handoff.md",
		"skills/pinchtab-browsing.md",
		"skills/opencode-commands.md",
	}

	for _, exp := range expected {
		if !templateMap[exp] {
			t.Errorf("expected template not found: %s", exp)
		}
	}
}
