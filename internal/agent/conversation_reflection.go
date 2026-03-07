package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/startower-observability/blackcat/internal/types"
)

// maxConversationReflectionLength is the maximum character length for a
// conversation reflection insight.
const maxConversationReflectionLength = 500

// ConversationReflector archives key learnings from a stale/expired session
// transcript. This is SEPARATE from Reflector (which handles tool-failure critique).
type ConversationReflector struct {
	llm   types.LLMClient
	store ArchivalStoreIface // reuse the same interface from reflection.go
}

// NewConversationReflector creates a new ConversationReflector.
// Returns nil if llm or store is nil, making all Reflect calls safe no-ops.
func NewConversationReflector(llm types.LLMClient, store ArchivalStoreIface) *ConversationReflector {
	if llm == nil || store == nil {
		return nil
	}
	return &ConversationReflector{llm: llm, store: store}
}

// Reflect analyzes a prior session transcript and archives durable insights.
// Best-effort: always returns nil if r is nil; logs errors but never blocks.
func (r *ConversationReflector) Reflect(ctx context.Context, userID string, messages []types.LLMMessage) error {
	if r == nil || r.llm == nil || r.store == nil {
		return nil
	}
	if len(messages) == 0 {
		return nil
	}

	// Build transcript from non-system messages only
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, truncateReflection(msg.Content, 300)))
	}
	transcript := sb.String()
	if strings.TrimSpace(transcript) == "" {
		return nil
	}

	prompt := "You are reviewing a conversation that has ended. Extract any explicit user facts, preferences, or unresolved follow-ups worth remembering for next time.\n\nRules:\n- Only extract EXPLICITLY stated information (no inferences)\n- Focus on: user name/timezone/preferences, pending tasks, context for next session\n- Ignore tool calls and system messages\n- Output 2-4 concise sentences max\n\nConversation:\n" + truncateReflection(transcript, 2000)

	resp, err := r.llm.Chat(ctx, []types.LLMMessage{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		slog.WarnContext(ctx, "conversation reflection LLM call failed", "err", err)
		return fmt.Errorf("conversation reflection: llm chat: %w", err)
	}

	lesson := resp.Content
	if len(lesson) > maxConversationReflectionLength {
		lesson = lesson[:maxConversationReflectionLength]
	}
	if strings.TrimSpace(lesson) == "" {
		return nil
	}

	if err := r.store.InsertArchival(ctx, userID, lesson, []string{"conversation_reflection"}, nil); err != nil {
		slog.WarnContext(ctx, "conversation reflection archival insert failed", "err", err)
		return fmt.Errorf("conversation reflection: archival insert: %w", err)
	}

	slog.Info("conversation reflection archived", "user", userID, "insight_len", len(lesson))
	return nil
}
