package agentapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/startower-observability/blackcat/internal/observability"
	"github.com/startower-observability/blackcat/internal/scheduler"
	"github.com/startower-observability/blackcat/internal/skills"
	"github.com/startower-observability/blackcat/internal/version"
)

// ProcessStartedAt records when this process was loaded. Used for ProcessUptime.
var ProcessStartedAt = time.Now()

// SelfKnowledgeProvider is the narrow interface for snapshot building.
// Loop implements this interface; tests use a stub.
type SelfKnowledgeProvider interface {
	GetAgentName() string
	GetModelName() string
	GetProviderName() string
	GetChannelType() string
	GetDaemonStartedAt() time.Time
	GetActiveSkills() []skills.Skill
	GetInactiveSkills() []skills.InactiveSkill // empty for compact mode
	GetCostTracker() *observability.CostTracker
	GetUserID() string
}

// SelfKnowledgeSnapshot holds a point-in-time view of the agent's self-knowledge.
// Used by both the compact Runtime Context injection and the full self-status tool.
type SelfKnowledgeSnapshot struct {
	// Static identity
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`

	// Runtime identity
	AgentName    string `json:"agent_name"`
	ModelName    string `json:"model_name"`
	ProviderName string `json:"provider_name"`
	ChannelType  string `json:"channel_type"`

	// Uptime
	DaemonUptime  string `json:"daemon_uptime"`  // human-readable, e.g. "2h 15m" or "unavailable" if zero time
	ProcessUptime string `json:"process_uptime"` // time.Since(ProcessStartedAt)

	// Skills
	ActiveSkillCount   int                    `json:"active_skill_count"`
	ActiveSkillNames   []string               `json:"active_skill_names"`
	InactiveSkillCount int                    `json:"inactive_skill_count"`
	InactiveSkills     []skills.InactiveSkill `json:"inactive_skills,omitempty"` // populated in full mode only

	// Token usage
	TokenUsage24h string `json:"token_usage_24h"` // e.g. "1234 in / 567 out" or "unavailable"
	CacheUsage    string `json:"cache_usage"`     // always "unavailable"

	// Phase 5 extensions
	Roles              []RoleView     `json:"roles,omitempty"`
	SchedulerEnabled   bool           `json:"scheduler_enabled"`
	SchedulerTaskCount int            `json:"scheduler_task_count,omitempty"`
	ProviderCatalog    []CatalogEntry `json:"provider_catalog,omitempty"`

	// Mode
	FullMode bool `json:"full_mode"`

	// Flags for absent fields
	UnavailableFields []string `json:"unavailable_fields,omitempty"`
}

// SelfKnowledgeExtras holds optional runtime subsystems for Phase 5 snapshot enrichment.
// All fields are nil-safe — missing fields degrade gracefully.
// Callers that do not need Phase 5 enrichment should pass nil.
type SelfKnowledgeExtras struct {
	// Roles is the pre-computed list of role views (avoids cycle with internal/agent).
	Roles []RoleView
	// SkillInventory is the full skill inventory (active + inactive) for enriched counts.
	SkillInventory *skills.Inventory
	// SchedulerSubsystem provides runtime scheduler state for enabled/task-count fields.
	SchedulerSubsystem *scheduler.SchedulerSubsystem
	// ProviderCatalog is the pre-fetched list of provider catalog entries.
	// Use this instead of *llm.ProviderCatalogCache to avoid an import cycle
	// (llm imports agentapi).
	ProviderCatalog []CatalogEntry
}

// BuildSelfKnowledgeSnapshot constructs a snapshot from the given provider.
// If fullMode is true, inactive skill details are populated.
// extras may be nil; when non-nil its fields enrich the Phase 5 snapshot sections.
func BuildSelfKnowledgeSnapshot(ctx context.Context, p SelfKnowledgeProvider, fullMode bool, extras *SelfKnowledgeExtras) SelfKnowledgeSnapshot {
	snap := SelfKnowledgeSnapshot{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,

		AgentName:    p.GetAgentName(),
		ModelName:    p.GetModelName(),
		ProviderName: p.GetProviderName(),
		ChannelType:  p.GetChannelType(),

		CacheUsage: "unavailable",
		FullMode:   fullMode,
	}

	// Daemon uptime
	daemonStart := p.GetDaemonStartedAt()
	if daemonStart.IsZero() {
		snap.DaemonUptime = "unavailable"
		snap.UnavailableFields = append(snap.UnavailableFields, "DaemonUptime")
	} else {
		snap.DaemonUptime = FormatDuration(time.Since(daemonStart))
	}

	// Process uptime
	snap.ProcessUptime = FormatDuration(time.Since(ProcessStartedAt))

	// Skills
	activeSkills := p.GetActiveSkills()
	snap.ActiveSkillCount = len(activeSkills)
	snap.ActiveSkillNames = make([]string, len(activeSkills))
	for i, s := range activeSkills {
		snap.ActiveSkillNames[i] = s.Name
	}

	inactiveSkills := p.GetInactiveSkills()
	snap.InactiveSkillCount = len(inactiveSkills)
	if fullMode {
		snap.InactiveSkills = inactiveSkills
	}

	// Token usage (24h)
	snap.TokenUsage24h = BuildTokenUsage24h(ctx, p.GetCostTracker())

	// CacheUsage always unavailable
	snap.UnavailableFields = append(snap.UnavailableFields, "CacheUsage")

	// Phase 5 enrichment from extras
	if extras != nil {
		// Roles
		if len(extras.Roles) > 0 {
			snap.Roles = extras.Roles
		}
		// Skills (full inventory with redacted inactive details)
		if extras.SkillInventory != nil {
			invSnap := skills.BuildSkillInventorySnapshot(extras.SkillInventory)
			snap.ActiveSkillCount = invSnap.ActiveCount
			snap.ActiveSkillNames = invSnap.ActiveNames
			snap.InactiveSkillCount = invSnap.InactiveCount
		}
		// Scheduler
		if extras.SchedulerSubsystem != nil {
			schedSnap := scheduler.BuildSchedulerStatusSnapshot(extras.SchedulerSubsystem)
			snap.SchedulerEnabled = schedSnap.Enabled
			snap.SchedulerTaskCount = schedSnap.TaskCount
		}
		// Provider catalog
		if len(extras.ProviderCatalog) > 0 {
			snap.ProviderCatalog = extras.ProviderCatalog
		}
	}

	return snap
}

// BuildTokenUsage24h queries the cost tracker for aggregate token counts.
func BuildTokenUsage24h(ctx context.Context, ct *observability.CostTracker) string {
	if ct == nil {
		return "unavailable"
	}
	usages, err := ct.AllSummarySince(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return "unavailable"
	}
	if len(usages) == 0 {
		return "0 in / 0 out"
	}
	var totalIn, totalOut int64
	for _, u := range usages {
		totalIn += u.TotalInputTokens
		totalOut += u.TotalOutputTokens
	}
	return fmt.Sprintf("%d in / %d out", totalIn, totalOut)
}

// CompactSummary returns a multi-line string suitable for injecting into the
// Runtime Context section of the system prompt.
func (s *SelfKnowledgeSnapshot) CompactSummary() string {
	var b strings.Builder

	// Version line
	commitShort := s.Commit
	if len(commitShort) > 12 {
		commitShort = commitShort[:12]
	}
	b.WriteString(fmt.Sprintf("Version: %s (%s)\n", s.Version, commitShort))

	// Uptime line
	b.WriteString(fmt.Sprintf("Uptime: %s daemon / %s process\n", s.DaemonUptime, s.ProcessUptime))

	// Active skills line
	if s.ActiveSkillCount > 0 {
		b.WriteString(fmt.Sprintf("Active skills: %d (%s)\n", s.ActiveSkillCount, strings.Join(s.ActiveSkillNames, ", ")))
	} else {
		b.WriteString("Active skills: 0\n")
	}

	// Inactive skills line
	b.WriteString(fmt.Sprintf("Inactive skills: %d\n", s.InactiveSkillCount))

	// Token usage
	b.WriteString(fmt.Sprintf("Token usage (24h): %s\n", s.TokenUsage24h))

	// Cache usage
	b.WriteString(fmt.Sprintf("Cache usage: %s", s.CacheUsage))

	return b.String()
}

// FormatDuration converts a duration to a human-readable string.
// Examples: "2h 15m", "45m", "3s", "0s".
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	seconds := int(d.Seconds())
	return fmt.Sprintf("%ds", seconds)
}
