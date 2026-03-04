// Package tools provides the tool registry and built-in tools for the agent.
package tools

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/startower-observability/blackcat/internal/memory"
	"github.com/startower-observability/blackcat/internal/types"
)

// Registry holds registered tools and dispatches execution requests.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]types.Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]types.Tool),
	}
}

// Register adds a tool to the registry, keyed by its Name().
func (r *Registry) Register(tool types.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get returns a registered tool by name, or types.ErrToolNotFound.
func (r *Registry) Get(name string) (types.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, types.ErrToolNotFound
	}
	return t, nil
}

// List returns tool definitions for all registered tools (for LLM consumption).
func (r *Registry) List() []types.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]types.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, types.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// Execute finds a tool by name and runs it with the given arguments.
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	t, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return t.Execute(ctx, args)
}

// RegisterMemoryTools registers the core memory and archival memory tools
// into the given registry. The userID is baked into each tool handler at
// construction time for security — it is NOT passed via tool parameters.
func RegisterMemoryTools(r *Registry, core *memory.CoreStore, archival *memory.SQLiteStore, embed *memory.EmbeddingClient, userID string) {
	if core != nil {
		h := NewCoreMemoryToolHandler(core, userID)
		h.RegisterTools(r)
	}
	if archival != nil {
		h := NewArchivalMemoryToolHandler(archival, embed, userID)
		h.RegisterTools(r)
	}
}
