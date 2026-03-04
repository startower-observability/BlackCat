package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/startower-observability/blackcat/internal/memory"
)

const (
	archivalMemoryInsertName        = "archival_memory_insert"
	archivalMemoryInsertDescription = "Save important information to long-term archival memory. Use this for significant facts, decisions, events, or anything worth remembering for future sessions."

	archivalMemorySearchName        = "archival_memory_search"
	archivalMemorySearchDescription = "Search archival long-term memory using semantic and keyword search. Use this to recall past conversations, facts, or decisions."

	archivalSearchDefaultLimit = 5
	archivalSearchMaxLimit     = 20
)

var archivalMemoryInsertParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"content": {
			"type": "string",
			"description": "The information to save to archival memory"
		},
		"tags": {
			"type": "string",
			"description": "Comma-separated tags for categorization"
		}
	},
	"required": ["content"]
}`)

var archivalMemorySearchParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "Search query to find relevant memories"
		},
		"limit": {
			"type": "integer",
			"description": "Max results to return (default: 5, max: 20)"
		}
	},
	"required": ["query"]
}`)

// ArchivalMemoryInsertTool saves information to long-term archival memory.
type ArchivalMemoryInsertTool struct {
	store  *memory.SQLiteStore
	embed  *memory.EmbeddingClient
	userID string
}

// ArchivalMemorySearchTool searches archival memory using semantic+keyword search.
type ArchivalMemorySearchTool struct {
	store  *memory.SQLiteStore
	embed  *memory.EmbeddingClient
	userID string
}

// ArchivalMemoryToolHandler constructs both archival memory tools for a given user.
type ArchivalMemoryToolHandler struct {
	store  *memory.SQLiteStore
	embed  *memory.EmbeddingClient
	userID string
}

// NewArchivalMemoryToolHandler creates a handler that produces archival memory tools.
func NewArchivalMemoryToolHandler(store *memory.SQLiteStore, embed *memory.EmbeddingClient, userID string) *ArchivalMemoryToolHandler {
	return &ArchivalMemoryToolHandler{store: store, embed: embed, userID: userID}
}

// RegisterTools registers both archival memory tools into the given registry.
func (h *ArchivalMemoryToolHandler) RegisterTools(r *Registry) {
	r.Register(&ArchivalMemoryInsertTool{store: h.store, embed: h.embed, userID: h.userID})
	r.Register(&ArchivalMemorySearchTool{store: h.store, embed: h.embed, userID: h.userID})
}

// --- ArchivalMemoryInsertTool implements types.Tool ---

func (t *ArchivalMemoryInsertTool) Name() string        { return archivalMemoryInsertName }
func (t *ArchivalMemoryInsertTool) Description() string { return archivalMemoryInsertDescription }
func (t *ArchivalMemoryInsertTool) Parameters() json.RawMessage {
	return archivalMemoryInsertParameters
}

func (t *ArchivalMemoryInsertTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Content string `json:"content"`
		Tags    string `json:"tags"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("archival_memory_insert: parse args: %w", err)
	}

	content := strings.TrimSpace(params.Content)
	if content == "" {
		return "Error: content parameter is required", nil
	}

	// Parse comma-separated tags.
	var tags []string
	if params.Tags != "" {
		for _, tag := range strings.Split(params.Tags, ",") {
			if t := strings.TrimSpace(tag); t != "" {
				tags = append(tags, t)
			}
		}
	}

	// Generate embedding if client is available.
	var embedding []float32
	if t.embed != nil {
		vec, err := t.embed.EmbedSingle(ctx, content)
		if err == nil {
			embedding = vec
		}
		// If embedding fails, we still insert without it.
	}

	if err := t.store.InsertArchival(ctx, t.userID, content, tags, embedding); err != nil {
		return fmt.Sprintf("Error saving to archival memory: %s", err), nil
	}

	tagStr := ""
	if len(tags) > 0 {
		tagStr = fmt.Sprintf(" (tags: %s)", strings.Join(tags, ", "))
	}
	return fmt.Sprintf("Saved to archival memory%s: %s", tagStr, truncate(content, 100)), nil
}

// --- ArchivalMemorySearchTool implements types.Tool ---

func (t *ArchivalMemorySearchTool) Name() string        { return archivalMemorySearchName }
func (t *ArchivalMemorySearchTool) Description() string { return archivalMemorySearchDescription }
func (t *ArchivalMemorySearchTool) Parameters() json.RawMessage {
	return archivalMemorySearchParameters
}

func (t *ArchivalMemorySearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("archival_memory_search: parse args: %w", err)
	}

	query := strings.TrimSpace(params.Query)
	if query == "" {
		return "Error: query parameter is required", nil
	}

	limit := params.Limit
	if limit <= 0 {
		limit = archivalSearchDefaultLimit
	}
	if limit > archivalSearchMaxLimit {
		limit = archivalSearchMaxLimit
	}

	// Generate embedding for semantic search if client is available.
	var embedding []float32
	if t.embed != nil {
		vec, err := t.embed.EmbedSingle(ctx, query)
		if err == nil {
			embedding = vec
		}
		// If embedding fails, SearchArchival falls back to keyword-only search.
	}

	results, err := t.store.SearchArchival(ctx, t.userID, query, embedding, limit)
	if err != nil {
		return fmt.Sprintf("Error searching archival memory: %s", err), nil
	}

	if len(results) == 0 {
		return "No matching archival memories found.", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d archival memories:\n\n", len(results)))
	for i, r := range results {
		b.WriteString(fmt.Sprintf("%d. [%s] %s",
			i+1,
			r.CreatedAt.UTC().Format("2006-01-02 15:04"),
			strings.TrimSpace(r.Content),
		))
		if len(r.Tags) > 0 {
			b.WriteString(fmt.Sprintf(" (tags: %s)", strings.Join(r.Tags, ", ")))
		}
		if r.Score > 0 {
			b.WriteString(fmt.Sprintf(" [score: %.2f]", r.Score))
		}
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// truncate returns the first n characters of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
