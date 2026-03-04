package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/startower-observability/blackcat/internal/memory"
)

const (
	coreMemoryGetName        = "core_memory_get"
	coreMemoryGetDescription = "Get a value from your core memory. Core memory contains important facts about the user that persist across conversations. Use keys like: name, language, preferences, project, style, location."

	coreMemoryUpdateName        = "core_memory_update"
	coreMemoryUpdateDescription = "Store or update a key-value fact in your core memory. Use this to remember important information about the user that should persist across conversations. Suggested keys: name, language, preferences, project, style, location, notes."
)

var coreMemoryGetParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"key": {
			"type": "string",
			"description": "The key to retrieve"
		}
	},
	"required": ["key"]
}`)

var coreMemoryUpdateParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"key": {
			"type": "string",
			"description": "The key to store"
		},
		"value": {
			"type": "string",
			"description": "The value to store"
		}
	},
	"required": ["key", "value"]
}`)

// CoreMemoryGetTool retrieves a value from core memory by key.
type CoreMemoryGetTool struct {
	store  *memory.CoreStore
	userID string
}

// CoreMemoryUpdateTool stores or updates a key-value pair in core memory.
type CoreMemoryUpdateTool struct {
	store  *memory.CoreStore
	userID string
}

// CoreMemoryToolHandler constructs both core memory tools for a given user.
type CoreMemoryToolHandler struct {
	store  *memory.CoreStore
	userID string
}

// NewCoreMemoryToolHandler creates a handler that produces core memory tools.
func NewCoreMemoryToolHandler(store *memory.CoreStore, userID string) *CoreMemoryToolHandler {
	return &CoreMemoryToolHandler{store: store, userID: userID}
}

// Tools returns the core_memory_get and core_memory_update tools.
func (h *CoreMemoryToolHandler) Tools() []interface{ Name() string } {
	return []interface{ Name() string }{
		&CoreMemoryGetTool{store: h.store, userID: h.userID},
		&CoreMemoryUpdateTool{store: h.store, userID: h.userID},
	}
}

// RegisterTools registers both core memory tools into the given registry.
func (h *CoreMemoryToolHandler) RegisterTools(r *Registry) {
	r.Register(&CoreMemoryGetTool{store: h.store, userID: h.userID})
	r.Register(&CoreMemoryUpdateTool{store: h.store, userID: h.userID})
}

// --- CoreMemoryGetTool implements types.Tool ---

func (t *CoreMemoryGetTool) Name() string                { return coreMemoryGetName }
func (t *CoreMemoryGetTool) Description() string         { return coreMemoryGetDescription }
func (t *CoreMemoryGetTool) Parameters() json.RawMessage { return coreMemoryGetParameters }

func (t *CoreMemoryGetTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("core_memory_get: parse args: %w", err)
	}

	key := strings.TrimSpace(params.Key)
	if key == "" {
		return "Error: key parameter is required", nil
	}

	value, err := t.store.Get(ctx, t.userID, key)
	if err != nil {
		return fmt.Sprintf("Error reading core memory: %s", err), nil
	}

	if value == "" {
		return fmt.Sprintf("No value found for key %q in core memory.", key), nil
	}

	return fmt.Sprintf("Core memory [%s]: %s", key, value), nil
}

// --- CoreMemoryUpdateTool implements types.Tool ---

func (t *CoreMemoryUpdateTool) Name() string                { return coreMemoryUpdateName }
func (t *CoreMemoryUpdateTool) Description() string         { return coreMemoryUpdateDescription }
func (t *CoreMemoryUpdateTool) Parameters() json.RawMessage { return coreMemoryUpdateParameters }

func (t *CoreMemoryUpdateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("core_memory_update: parse args: %w", err)
	}

	key := strings.TrimSpace(params.Key)
	if key == "" {
		return "Error: key parameter is required", nil
	}
	value := strings.TrimSpace(params.Value)
	if value == "" {
		return "Error: value parameter is required", nil
	}

	if err := t.store.Set(ctx, t.userID, key, value); err != nil {
		return fmt.Sprintf("Error updating core memory: %s", err), nil
	}

	return fmt.Sprintf("Core memory updated: %s = %s", key, value), nil
}
