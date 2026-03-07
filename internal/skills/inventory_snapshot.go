package skills

// InactiveReason is a safe reason category (no raw env var names).
type InactiveReason string

const (
	ReasonMissingEnv    InactiveReason = "missing_env"
	ReasonMissingBinary InactiveReason = "missing_binary"
	ReasonOther         InactiveReason = "other"
)

// InactiveSkillSummary is the LLM-safe summary of an inactive skill.
type InactiveSkillSummary struct {
	Name    string           `json:"name"`
	Reasons []InactiveReason `json:"reasons"`
}

// SkillInventorySnapshot is the runtime self-knowledge view of the skill system.
type SkillInventorySnapshot struct {
	ActiveCount    int                    `json:"active_count"`
	ActiveNames    []string               `json:"active_names"`
	InactiveCount  int                    `json:"inactive_count"`
	InactiveSkills []InactiveSkillSummary `json:"inactive_skills,omitempty"`
}

// BuildSkillInventorySnapshot builds a redacted, LLM-safe skill inventory snapshot.
// It redacts raw env var names and binary names, exposing only reason categories.
func BuildSkillInventorySnapshot(inv *Inventory) SkillInventorySnapshot {
	if inv == nil {
		return SkillInventorySnapshot{}
	}

	activeNames := make([]string, 0, len(inv.Active))
	for _, s := range inv.Active {
		activeNames = append(activeNames, s.Name)
	}

	inactiveSummaries := make([]InactiveSkillSummary, 0, len(inv.Inactive))
	for _, is := range inv.Inactive {
		reasons := buildReasons(is)
		inactiveSummaries = append(inactiveSummaries, InactiveSkillSummary{
			Name:    is.Name,
			Reasons: reasons,
		})
	}

	return SkillInventorySnapshot{
		ActiveCount:    len(inv.Active),
		ActiveNames:    activeNames,
		InactiveCount:  len(inv.Inactive),
		InactiveSkills: inactiveSummaries,
	}
}

// buildReasons infers safe reason categories from an InactiveSkill.
// Raw env var names and binary names are intentionally NOT exposed.
func buildReasons(is InactiveSkill) []InactiveReason {
	seen := map[InactiveReason]bool{}
	var reasons []InactiveReason

	if len(is.MissingEnv) > 0 {
		if !seen[ReasonMissingEnv] {
			seen[ReasonMissingEnv] = true
			reasons = append(reasons, ReasonMissingEnv)
		}
	}
	if len(is.MissingBins) > 0 {
		if !seen[ReasonMissingBinary] {
			seen[ReasonMissingBinary] = true
			reasons = append(reasons, ReasonMissingBinary)
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, ReasonOther)
	}

	return reasons
}
