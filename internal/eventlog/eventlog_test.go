package eventlog

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestEventRecordMarshal(t *testing.T) {
	ts := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	rec := EventRecord{
		Timestamp:  ts,
		SessionID:  "sess-001",
		EventType:  EventTypeToolCall,
		ToolName:   "exec",
		UserID:     "u42",
		Channel:    "telegram",
		DurationMs: 150,
		Success:    true,
		Extra:      map[string]any{"key": "value"},
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got EventRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.EventType != EventTypeToolCall {
		t.Errorf("event_type = %q, want %q", got.EventType, EventTypeToolCall)
	}
	if got.SessionID != "sess-001" {
		t.Errorf("session_id = %q, want %q", got.SessionID, "sess-001")
	}
	if got.ToolName != "exec" {
		t.Errorf("tool_name = %q, want %q", got.ToolName, "exec")
	}
	if !got.Success {
		t.Error("success = false, want true")
	}
	if got.DurationMs != 150 {
		t.Errorf("duration_ms = %d, want 150", got.DurationMs)
	}
	if got.Extra["key"] != "value" {
		t.Errorf("extra[key] = %v, want %q", got.Extra["key"], "value")
	}
}

func TestEventRecordOmitEmpty(t *testing.T) {
	rec := EventRecord{
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EventType: EventTypeError,
		Success:   false,
		Error:     "something broke",
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	// Fields with omitempty and zero values should be absent.
	for _, field := range []string{"session_id", "tool_name", "user_id", "channel", "duration_ms", "extra"} {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when zero/empty", field)
		}
	}

	// success should always be present (no omitempty).
	if _, ok := raw["success"]; !ok {
		t.Error("field \"success\" should always be present")
	}
}

func TestLogEventWritesValidJSONLine(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test-events.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer logger.Close()

	rec := EventRecord{
		Timestamp: time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC),
		SessionID: "s1",
		EventType: EventTypeSessionStart,
		Channel:   "discord",
		Success:   true,
	}

	if err := logger.LogEvent(context.Background(), rec); err != nil {
		t.Fatalf("LogEvent: %v", err)
	}

	// Write a second event.
	rec2 := EventRecord{
		EventType: EventTypeToolCall,
		ToolName:  "read_file",
		Success:   true,
	}
	if err := logger.LogEvent(context.Background(), rec2); err != nil {
		t.Fatalf("LogEvent #2: %v", err)
	}

	// Read back and verify each line is valid JSON.
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var parsed EventRecord
		if err := json.Unmarshal(scanner.Bytes(), &parsed); err != nil {
			t.Errorf("line %d: invalid JSON: %v\nraw: %s", lineNum, err, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner: %v", err)
	}

	if lineNum != 2 {
		t.Errorf("got %d lines, want 2", lineNum)
	}
}

func TestLogEventAutoTimestamp(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ts-events.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer logger.Close()

	before := time.Now()
	rec := EventRecord{
		EventType: EventTypeError,
		Error:     "test error",
	}
	if err := logger.LogEvent(context.Background(), rec); err != nil {
		t.Fatalf("LogEvent: %v", err)
	}
	after := time.Now()

	// Read back.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var got EventRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Timestamp.Before(before) || got.Timestamp.After(after) {
		t.Errorf("auto timestamp %v not between %v and %v", got.Timestamp, before, after)
	}
}

func TestCloseClosesFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "close-events.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Logging after close should fail.
	err = logger.LogEvent(context.Background(), EventRecord{EventType: EventTypeError})
	if err == nil {
		t.Fatal("expected error logging after Close, got nil")
	}

	// Double close should be safe.
	if err := logger.Close(); err != nil {
		t.Fatalf("double Close: %v", err)
	}
}

func TestNewDefaultPath(t *testing.T) {
	// We don't actually want to write to ~/.blackcat, so just verify
	// that passing an explicit path works and empty string would resolve.
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sub", "dir", "events.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("New with nested path: %v", err)
	}
	defer logger.Close()

	// Verify directory was created.
	if _, err := os.Stat(filepath.Dir(logPath)); err != nil {
		t.Errorf("parent dir not created: %v", err)
	}
}

func TestConcurrentLogEvent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "concurrent-events.log")

	logger, err := New(logPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer logger.Close()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			rec := EventRecord{
				EventType:  EventTypeToolCall,
				ToolName:   "tool",
				DurationMs: int64(idx),
				Success:    true,
			}
			if err := logger.LogEvent(context.Background(), rec); err != nil {
				t.Errorf("LogEvent(%d): %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	// Verify all lines are valid JSON.
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var rec EventRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Errorf("line %d: bad JSON: %v", lineNum, err)
		}
	}

	if lineNum != n {
		t.Errorf("got %d lines, want %d", lineNum, n)
	}
}

func TestEventTypeConstants(t *testing.T) {
	// Verify constants have expected values.
	tests := []struct {
		got, want string
	}{
		{EventTypeToolCall, "tool_call"},
		{EventTypeToolResult, "tool_result"},
		{EventTypeSessionStart, "session_start"},
		{EventTypeSessionEnd, "session_end"},
		{EventTypeError, "error"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("constant = %q, want %q", tt.got, tt.want)
		}
	}
}
