package agent

import (
	"context"
	"fmt"

	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/types"
)

// SubAgentConfig holds the configuration overlay for a specialized sub-agent.
type SubAgentConfig struct {
	// Name is an optional human-readable role name (e.g. "wizard", "explorer").
	Name string
	// SystemPromptOverlay is prepended to the agent name for task specialization.
	SystemPromptOverlay string
	// AllowedTools lists the tools this sub-agent can use. nil means all tools allowed.
	AllowedTools []string
	// Model overrides the LLM model for this sub-agent (empty = use base config).
	Model string
	// Provider overrides the LLM provider for this sub-agent (empty = use base config).
	Provider string
	// Temperature overrides the sampling temperature for this sub-agent (0 = use base config).
	Temperature float64
}

// Supervisor is a lightweight router that classifies incoming messages and
// routes them to specialized sub-agent Loop instances with tailored system
// prompt overlays and tool subsets.
type Supervisor struct {
	baseLoopCfg     LoopConfig
	subAgentConfigs map[RoleType]SubAgentConfig
	roles           []config.RoleConfig
	roleBackends    map[RoleType]types.LLMClient
}

// NewSupervisor creates a Supervisor with sub-agent configurations.
// When roles is non-nil and non-empty, configs are built from the supplied
// RoleConfig slice; otherwise the hardcoded 4-type defaults are used for
// backward compatibility.  backends maps each RoleType to an LLM client that
// should be used when routing to that role (nil entries fall back to the base
// config LLM).
func NewSupervisor(baseCfg LoopConfig, roles []config.RoleConfig, backends map[RoleType]types.LLMClient) *Supervisor {
	s := &Supervisor{
		baseLoopCfg:  baseCfg,
		roles:        roles,
		roleBackends: backends,
	}

	if len(roles) > 0 {
		s.subAgentConfigs = make(map[RoleType]SubAgentConfig, len(roles))
		for _, r := range roles {
			s.subAgentConfigs[RoleType(r.Name)] = SubAgentConfig{
				Name:                r.Name,
				SystemPromptOverlay: r.SystemPrompt,
				AllowedTools:        r.AllowedTools,
				Model:               r.Model,
				Provider:            r.Provider,
				Temperature:         r.Temperature,
			}
		}
	} else {
		s.subAgentConfigs = map[RoleType]SubAgentConfig{
			TaskTypeCoding: {
				SystemPromptOverlay: "You are a specialized coding agent. Focus on code quality, testing, and best practices.",
				AllowedTools:        nil, // all tools
			},
			TaskTypeResearch: {
				SystemPromptOverlay: "You are a specialized research agent. Focus on finding accurate information. Do not execute system commands.",
				AllowedTools:        []string{"memory_search", "web_search", "archival_memory_search", "archival_memory_insert", "core_memory_get"},
			},
			TaskTypeAdmin: {
				SystemPromptOverlay: "You are an admin agent. Handle system configuration and service management carefully.",
				AllowedTools:        nil, // all tools
			},
			TaskTypeGeneral: {
				SystemPromptOverlay: "",
				AllowedTools:        nil, // all tools
			},
		}
	}

	return s
}

// Route classifies a message and runs it through a specialized Loop using the
// base configuration stored in the Supervisor.
func (s *Supervisor) Route(ctx context.Context, msg string) (*Execution, error) {
	return s.RouteWithCfg(ctx, msg, s.baseLoopCfg)
}

// RouteWithCfg classifies a message, selects the appropriate sub-agent config,
// creates a specialized Loop with tailored system prompt overlay and tool subset,
// then runs the message through it. The caller provides a per-message LoopConfig
// (with EventStream, SessionMessages, UserID, etc. already set).
func (s *Supervisor) RouteWithCfg(ctx context.Context, msg string, cfg LoopConfig) (*Execution, error) {
	if s == nil {
		return nil, fmt.Errorf("supervisor not initialized")
	}

	taskType := ClassifyMessage(msg, s.roles)
	subCfg, ok := s.subAgentConfigs[taskType]
	if !ok {
		subCfg = s.subAgentConfigs[TaskTypeGeneral]
	}

	// Apply agent name suffix for task specialization
	if cfg.AgentName != "" {
		cfg.AgentName = fmt.Sprintf("%s [%s]", cfg.AgentName, string(taskType))
	}

	// Apply system prompt overlay via a Reflector-style approach:
	// We prepend the overlay to the AgentTone field which gets injected into the
	// system prompt persona section. This is a lightweight approach that doesn't
	// require modifying Loop internals.
	if subCfg.SystemPromptOverlay != "" {
		if cfg.AgentTone != "" {
			cfg.AgentTone = subCfg.SystemPromptOverlay + " " + cfg.AgentTone
		} else {
			cfg.AgentTone = subCfg.SystemPromptOverlay
		}
	}

	// Apply per-role model/provider overrides from SubAgentConfig.
	if subCfg.Model != "" {
		cfg.ModelName = subCfg.Model
	}
	if subCfg.Provider != "" {
		cfg.ProviderName = subCfg.Provider
	}

	// Apply per-role LLM backend override.
	if s.roleBackends != nil {
		if backend, exists := s.roleBackends[taskType]; exists && backend != nil {
			cfg.LLM = backend
		}
	}

	// Tool filtering: if AllowedTools is specified, filter the registry.
	if subCfg.AllowedTools != nil && cfg.Tools != nil {
		cfg.Tools = cfg.Tools.Filter(subCfg.AllowedTools)
	}

	return NewLoop(cfg).Run(ctx, msg)
}
