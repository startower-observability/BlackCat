package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a skill loaded from a markdown file
type Skill struct {
	Name     string
	Content  string
	Tags     []string
	FilePath string
	Requires Requirements
}

// IsEligible checks if a skill's requirements are met.
// Returns true if all required binaries are on PATH and all required env vars are set.
func (s *Skill) IsEligible() bool {
	for _, bin := range s.Requires.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			return false
		}
	}
	for _, env := range s.Requires.Env {
		if os.Getenv(env) == "" {
			return false
		}
	}
	return true
}

// LoadSkills loads all .md files from a directory and parses them as skills
func LoadSkills(dir string) ([]Skill, error) {
	// If dir doesn't exist, return empty slice (not error)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Skill{}, nil
		}
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []Skill

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Filter for .md files
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		skill := parseSkillFile(string(content), filePath, entry.Name())
		skills = append(skills, skill)
	}

	// Sort by Name (alphabetical)
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills, nil
}

// parseSkillFile parses a skill markdown file
func parseSkillFile(content string, filePath string, filename string) Skill {
	skill := Skill{
		FilePath: filePath,
	}

	// Try to parse YAML frontmatter first
	fm, body, hasFrontmatter := ParseFrontmatter(content)

	if hasFrontmatter {
		// If frontmatter present, use it for metadata
		if fm.Name != "" {
			skill.Name = fm.Name
		}
		if fm.Description != "" {
			// Description in frontmatter is optional metadata
		}
		if len(fm.Tags) > 0 {
			skill.Tags = fm.Tags
		}
		skill.Requires = fm.Requires
		skill.Content = body

		// If name not set from frontmatter, fall through to header parsing
		if skill.Name == "" {
			// Parse name from body content if not in frontmatter
			lines := strings.Split(body, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# ") {
					skill.Name = strings.TrimPrefix(line, "# ")
					skill.Name = strings.TrimSpace(skill.Name)
					break
				}
			}
		}

		// If still no name, use filename
		if skill.Name == "" {
			skill.Name = strings.TrimSuffix(filename, filepath.Ext(filename))
		}
		return skill
	}

	// Fall back to original parsing logic for non-frontmatter skills (G3)
	// This ensures perfect backward compatibility
	lines := strings.Split(content, "\n")

	// Extract name from first # heading line, or use filename
	nameFound := false
	contentStartIdx := 0

	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			skill.Name = strings.TrimPrefix(line, "# ")
			skill.Name = strings.TrimSpace(skill.Name)
			nameFound = true
			contentStartIdx = i + 1
			break
		}
	}

	// If no heading found, use filename without extension
	if !nameFound {
		skill.Name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	// Parse tags from "Tags:" or "**Tags**:" line
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Tags:") || strings.HasPrefix(trimmed, "**Tags**:") {
			// Extract tags after "Tags:" or "**Tags**:"
			var tagsStr string
			if strings.HasPrefix(trimmed, "**Tags**:") {
				tagsStr = strings.TrimPrefix(trimmed, "**Tags**:")
			} else {
				tagsStr = strings.TrimPrefix(trimmed, "Tags:")
			}
			tagsStr = strings.TrimSpace(tagsStr)

			// Parse comma-separated tags
			if tagsStr != "" {
				tagParts := strings.Split(tagsStr, ",")
				for _, tag := range tagParts {
					tag := strings.TrimSpace(tag)
					if tag != "" {
						skill.Tags = append(skill.Tags, tag)
					}
				}
			}
			break
		}
	}

	// Content is everything after the first heading line
	if contentStartIdx > 0 && contentStartIdx < len(lines) {
		skill.Content = strings.Join(lines[contentStartIdx:], "\n")
		skill.Content = strings.TrimSpace(skill.Content)
	} else if contentStartIdx == 0 && nameFound {
		// If we found a name but contentStartIdx is 0, take everything after first heading
		if len(lines) > 1 {
			skill.Content = strings.Join(lines[1:], "\n")
			skill.Content = strings.TrimSpace(skill.Content)
		}
	} else {
		// No heading found, use all content
		skill.Content = strings.TrimSpace(content)
	}

	return skill
}

// LoadSkillsFromMultipleSources loads skills from multiple directories with precedence.
// Earlier directories have higher priority — if the same skill name exists in multiple
// directories, only the first occurrence is kept.
// Returns eligible skills only (those with satisfied requirements).
func LoadSkillsFromMultipleSources(dirs []string) ([]Skill, error) {
	seen := make(map[string]bool)
	var all []Skill
	for _, dir := range dirs {
		skills, err := LoadSkills(dir)
		if err != nil {
			continue // skip missing directories
		}
		for _, s := range skills {
			if !seen[s.Name] && s.IsEligible() {
				seen[s.Name] = true
				all = append(all, s)
			}
		}
	}
	return all, nil
}

// FormatForInjection formats skills as context blocks for system prompt injection
func FormatForInjection(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var output strings.Builder

	for i, skill := range skills {
		output.WriteString(fmt.Sprintf("<skill name=\"%s\">\n", skill.Name))
		output.WriteString(skill.Content)
		output.WriteString("\n</skill>")

		// Add newline between skills, but not after the last one
		if i < len(skills)-1 {
			output.WriteString("\n\n")
		}
	}

	return output.String()
}

// FilterByFileSize returns skills whose raw content size is at or below maxBytes.
// Skills exceeding the limit are silently skipped.
func FilterByFileSize(skills []Skill, maxBytes int) []Skill {
	if maxBytes <= 0 {
		return skills
	}
	out := make([]Skill, 0, len(skills))
	for _, s := range skills {
		if len(s.Content) <= maxBytes {
			out = append(out, s)
		}
	}
	return out
}

// LimitSkillCount returns at most maxCount skills from the provided slice.
// If maxCount is 0 or negative, all skills are returned unchanged.
func LimitSkillCount(skills []Skill, maxCount int) []Skill {
	if maxCount <= 0 || len(skills) <= maxCount {
		return skills
	}
	return skills[:maxCount]
}
