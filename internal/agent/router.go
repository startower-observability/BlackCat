package agent

import (
	"sort"
	"strings"

	"github.com/startower-observability/blackcat/internal/config"
)

// RoleType represents the classification of an incoming message by agent role.
type RoleType = string

const (
	RolePhantom   RoleType = "phantom"
	RoleAstrology RoleType = "astrology"
	RoleWizard    RoleType = "wizard"
	RoleArtist    RoleType = "artist"
	RoleScribe    RoleType = "scribe"
	RoleExplorer  RoleType = "explorer"
	RoleOracle    RoleType = "oracle"
)

// Backward-compatible aliases for the old TaskType constants.
type TaskType = RoleType

const (
	TaskCoding   = RoleWizard
	TaskResearch = RoleExplorer
	TaskAdmin    = RolePhantom
	TaskGeneral  = RoleOracle
)

// Legacy aliases matching original constant names.
const (
	TaskTypeCoding   = RoleWizard
	TaskTypeResearch = RoleExplorer
	TaskTypeAdmin    = RolePhantom
	TaskTypeGeneral  = RoleOracle
)

// defaultRoles mirrors the 7 defaults from config.Validate().
var defaultRoles = []config.RoleConfig{
	{Name: "phantom", Keywords: []string{"restart", "deploy", "server", "status", "docker", "systemctl", "health", "infra", "devops", "service", "nginx", "ssl"}, Priority: 10},
	{Name: "astrology", Keywords: []string{"crypto", "bitcoin", "btc", "eth", "ethereum", "trading", "token", "defi", "nft", "wallet", "market", "portfolio", "investment", "stock", "forex", "chart", "candlestick", "pump", "whale"}, Priority: 20},
	{Name: "wizard", Keywords: []string{"code", "implement", "function", "bug", "fix", "test", "build", "compile", "git", "deploy", "opencode", "typescript", "golang", "python", "javascript", "refactor", "debug", "api", "endpoint", "database", "sql", "migration"}, Priority: 30},
	{Name: "artist", Keywords: []string{"instagram", "tiktok", "twitter", "linkedin", "facebook", "threads", "post", "caption", "hashtag", "reel", "story", "content", "social", "viral", "engagement", "schedule", "publish"}, Priority: 40},
	{Name: "scribe", Keywords: []string{"write", "draft", "article", "blog", "email", "document", "copy", "copywriting", "proofread", "translate", "summarize", "report", "newsletter", "pitch", "proposal"}, Priority: 50},
	{Name: "explorer", Keywords: []string{"search", "find", "look up", "what is", "explain", "research", "summarize", "web", "browse", "read", "compare", "analyze", "review", "investigate"}, Priority: 60},
	{Name: "oracle", Keywords: nil, Priority: 100},
}

// ClassifyMessage classifies a message into a RoleType using keyword heuristics.
// Roles are checked in ascending Priority order (lowest number = highest precedence).
// If roles is nil or empty, hardcoded defaults are used.
// Falls back to the highest-priority-number role (oracle-style) if no keywords match.
func ClassifyMessage(msg string, roles []config.RoleConfig) RoleType {
	if len(roles) == 0 {
		roles = defaultRoles
	}

	// Sort by Priority ascending (lowest number checked first = highest precedence).
	sorted := make([]config.RoleConfig, len(roles))
	copy(sorted, roles)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	lower := strings.ToLower(msg)

	for _, role := range sorted {
		for _, kw := range role.Keywords {
			if strings.Contains(lower, kw) {
				return RoleType(role.Name)
			}
		}
	}

	// Fallback: return the role with the highest Priority number (last in sorted order).
	return RoleType(sorted[len(sorted)-1].Name)
}
