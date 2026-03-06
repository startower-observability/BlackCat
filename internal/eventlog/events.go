// Package eventlog provides structured JSON Lines event logging for BlackCat.
// Events are written to ~/.blackcat/events.log (configurable) for observability
// into tool calls, sessions, errors, and completions.
package eventlog

import "time"

// Event type constants.
const (
	EventTypeToolCall     = "tool_call"
	EventTypeToolResult   = "tool_result"
	EventTypeSessionStart = "session_start"
	EventTypeSessionEnd   = "session_end"
	EventTypeError        = "error"
	EventTypeTaskQueued   = "task_queued"
)

// EventRecord is a single structured event written as one JSON line.
type EventRecord struct {
	Timestamp  time.Time      `json:"timestamp"`
	SessionID  string         `json:"session_id,omitempty"`
	EventType  string         `json:"event_type"`
	ToolName   string         `json:"tool_name,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
	Channel    string         `json:"channel,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}
