package agent

import (
	"sort"

	"github.com/startower-observability/blackcat/internal/agentapi"
	"github.com/startower-observability/blackcat/internal/config"
)

// RoleRegistry is a read model over the configured roles.
type RoleRegistry struct {
	roles []config.RoleConfig
}

// NewRoleRegistry builds a RoleRegistry from the configured role list.
func NewRoleRegistry(roles []config.RoleConfig) *RoleRegistry {
	return &RoleRegistry{roles: roles}
}

// Views returns an LLM-safe []RoleView sorted ascending by priority.
func (r *RoleRegistry) Views() []agentapi.RoleView {
	sorted := make([]config.RoleConfig, len(r.roles))
	copy(sorted, r.roles)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	views := make([]agentapi.RoleView, 0, len(sorted))
	for _, rc := range sorted {
		views = append(views, agentapi.RoleView{
			Name:         rc.Name,
			Priority:     rc.Priority,
			KeywordCount: len(rc.Keywords),
		})
	}
	return views
}

// FallbackRole returns the highest-priority fallback (lowest int priority that has no keywords),
// i.e. the catch-all "oracle" role. Returns nil if none found.
func (r *RoleRegistry) FallbackRole() *agentapi.RoleView {
	for _, rc := range r.roles {
		if len(rc.Keywords) == 0 {
			v := agentapi.RoleView{
				Name:         rc.Name,
				Priority:     rc.Priority,
				KeywordCount: 0,
			}
			return &v
		}
	}
	return nil
}
