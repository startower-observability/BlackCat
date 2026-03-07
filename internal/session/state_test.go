package session

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/types"
)

func TestGetWithStateReturnsStaleSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, 100)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	key := SessionKey{ChannelType: "telegram", ChannelID: "stale-chat", UserID: "stale-user"}
	now := time.Now()
	sess := &Session{
		Key:          key,
		Messages:     []types.LLMMessage{{Role: "user", Content: "old message"}},
		CreatedAt:    now.Add(-48 * time.Hour),
		UpdatedAt:    now.Add(-48 * time.Hour),
		LastActivity: now.Add(-25 * time.Hour), // 25h ago → stale
		MessageCount: 1,
		// ExpiresAt left zero to trigger fallback stale logic
	}

	// Write directly to bypass Save() which resets metadata
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := store.filePath(key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	lookup, err := store.GetWithState(key)
	if err != nil {
		t.Fatalf("GetWithState: %v", err)
	}

	if lookup.State != SessionStateStale {
		t.Errorf("State = %q, want %q", lookup.State, SessionStateStale)
	}
	if lookup.Session == nil {
		t.Fatal("Session should be non-nil for stale sessions")
	}
	if len(lookup.Session.Messages) != 1 {
		t.Errorf("Messages count = %d, want 1", len(lookup.Session.Messages))
	}
	if lookup.Session.Messages[0].Content != "old message" {
		t.Errorf("Message content = %q, want %q", lookup.Session.Messages[0].Content, "old message")
	}
}

func TestGetWithStateReturnsExpiredSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, 100)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	key := SessionKey{ChannelType: "discord", ChannelID: "exp-chat", UserID: "exp-user"}
	now := time.Now()
	sess := &Session{
		Key:          key,
		Messages:     []types.LLMMessage{{Role: "assistant", Content: "expired reply"}},
		CreatedAt:    now.Add(-72 * time.Hour),
		UpdatedAt:    now.Add(-72 * time.Hour),
		LastActivity: now.Add(-72 * time.Hour),
		MessageCount: 1,
		ExpiresAt:    now.Add(-1 * time.Hour), // expired 1h ago
	}

	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := store.filePath(key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	lookup, err := store.GetWithState(key)
	if err != nil {
		t.Fatalf("GetWithState: %v", err)
	}

	if lookup.State != SessionStateExpired {
		t.Errorf("State = %q, want %q", lookup.State, SessionStateExpired)
	}
	if lookup.Session == nil {
		t.Fatal("Session should be non-nil for expired sessions")
	}
	if lookup.Session.Messages[0].Content != "expired reply" {
		t.Errorf("Message content = %q, want %q", lookup.Session.Messages[0].Content, "expired reply")
	}
}

func TestGetWithStateReturnsMissingForAbsent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, 100)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	key := SessionKey{ChannelType: "telegram", ChannelID: "nonexistent", UserID: "ghost"}

	lookup, err := store.GetWithState(key)
	if err != nil {
		t.Fatalf("GetWithState: %v", err)
	}

	if lookup.State != SessionStateMissing {
		t.Errorf("State = %q, want %q", lookup.State, SessionStateMissing)
	}
	if lookup.Session != nil {
		t.Error("Session should be nil for missing sessions")
	}
}

func TestGetWithStateReturnsActiveSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, 100)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	key := SessionKey{ChannelType: "telegram", ChannelID: "active-chat", UserID: "active-user"}
	sess := &Session{
		Key:      key,
		Messages: []types.LLMMessage{{Role: "user", Content: "hello"}},
	}

	// Use Save() which sets LastActivity and ExpiresAt to now+24h
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	lookup, err := store.GetWithState(key)
	if err != nil {
		t.Fatalf("GetWithState: %v", err)
	}

	if lookup.State != SessionStateActive {
		t.Errorf("State = %q, want %q", lookup.State, SessionStateActive)
	}
	if lookup.Session == nil {
		t.Fatal("Session should be non-nil for active sessions")
	}
}

// TestGetReturnsNilForStaleSession verifies that existing Get() behavior is unchanged:
// stale sessions still return (nil, nil).
func TestGetReturnsNilForStaleSession(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, 100)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	key := SessionKey{ChannelType: "telegram", ChannelID: "compat-chat", UserID: "compat-user"}
	now := time.Now()
	sess := &Session{
		Key:          key,
		Messages:     []types.LLMMessage{{Role: "user", Content: "stale compat test"}},
		CreatedAt:    now.Add(-48 * time.Hour),
		UpdatedAt:    now.Add(-48 * time.Hour),
		LastActivity: now.Add(-25 * time.Hour), // 25h ago → stale
		MessageCount: 1,
		// ExpiresAt left zero
	}

	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := store.filePath(key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Existing Get() must still return nil for stale sessions
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("Get() returned non-nil for stale session, want nil (backward compat)")
	}

	// But GetWithState() returns the session with stale state
	lookup, err := store.GetWithState(key)
	if err != nil {
		t.Fatalf("GetWithState: %v", err)
	}
	if lookup.State != SessionStateStale {
		t.Errorf("GetWithState State = %q, want %q", lookup.State, SessionStateStale)
	}
	if lookup.Session == nil {
		t.Fatal("GetWithState Session should be non-nil for stale sessions")
	}
}
