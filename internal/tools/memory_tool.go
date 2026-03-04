package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/startower-observability/blackcat/internal/memory"
)

const (
	memoryToolName        = "memory_search"
	memoryToolDescription = "[DEPRECATED: use archival_memory_search] Search agent memory for relevant past interactions and knowledge"
)

var memoryToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "Search query to find relevant memories"
		},
		"limit": {
			"type": "number",
			"description": "Maximum number of results (default 5)"
		}
	},
	"required": ["query"]
}`)

// MemoryTool allows the LLM to search agent memory via FTS5.
type MemoryTool struct {
	store *memory.SQLiteStore
}

// NewMemoryTool creates a new memory search tool backed by SQLiteStore.
func NewMemoryTool(store *memory.SQLiteStore) *MemoryTool {
	return &MemoryTool{store: store}
}

func (t *MemoryTool) Name() string                { return memoryToolName }
func (t *MemoryTool) Description() string         { return memoryToolDescription }
func (t *MemoryTool) Parameters() json.RawMessage { return memoryToolParameters }

func (t *MemoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("memory_search: parse args: %w", err)
	}

	if params.Query == "" {
		return "Error: query parameter is required", nil
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 5
	}

	entries, err := t.store.SearchWithLimit(ctx, params.Query, limit)
	if err != nil {
		return fmt.Sprintf("Error searching memory: %s", err), nil
	}

	if len(entries) == 0 {
		return "No matching memories found.", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d matching memories:\n\n", len(entries)))
	for i, e := range entries {
		b.WriteString(fmt.Sprintf("%d. [%s] %s",
			i+1,
			e.Timestamp.UTC().Format("2006-01-02 15:04"),
			strings.TrimSpace(e.Content),
		))
		if len(e.Tags) > 0 {
			b.WriteString(fmt.Sprintf(" (tags: %s)", strings.Join(e.Tags, ", ")))
		}
		b.WriteByte('\n')
	}

	return b.String(), nil
}
