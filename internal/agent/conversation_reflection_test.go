package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/startower-observability/blackcat/internal/types"
)

// --- Mock types unique to conversation reflection tests ---

// convReflectMockLLM implements types.LLMClient for conversation reflection tests.
type convReflectMockLLM struct {
	response string
	err      error
	called   bool
}

func (m *convReflectMockLLM) Chat(_ context.Context, _ []types.LLMMessage, _ []types.ToolDefinition) (*types.LLMResponse, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return &types.LLMResponse{Content: m.response}, nil
}

func (m *convReflectMockLLM) Stream(_ context.Context, _ []types.LLMMessage, _ []types.ToolDefinition) (<-chan types.Chunk, error) {
	ch := make(chan types.Chunk)
	close(ch)
	return ch, nil
}

// convReflectMockStore implements ArchivalStoreIface for conversation reflection tests.
type convReflectMockStore struct {
	insertedContent string
	insertedTags    []string
	insertedUserID  string
	err             error
	called          bool
}

func (m *convReflectMockStore) InsertArchival(_ context.Context, userID, content string, tags []string, _ []float32) error {
	m.called = true
	m.insertedUserID = userID
	m.insertedContent = content
	m.insertedTags = tags
	return m.err
}

// --- Tests ---

func TestStaleSessionTriggersConversationReflection(t *testing.T) {
	// Simulate: a stale session has messages, ConversationReflector.Reflect
	// should call the LLM and store the insight in archival.
	insight := "User prefers concise answers. Timezone is UTC+7."
	mockLLM := &convReflectMockLLM{response: insight}
	store := &convReflectMockStore{}
	cr := NewConversationReflector(mockLLM, store)

	messages := []types.LLMMessage{
		{Role: "user", Content: "My name is Alex and I'm in UTC+7"},
		{Role: "assistant", Content: "Got it, Alex!"},
		{Role: "user", Content: "I prefer concise answers"},
	}

	err := cr.Reflect(context.Background(), "user-alex", messages)
	if err != nil {
		t.Fatalf("Reflect returned unexpected error: %v", err)
	}

	if !mockLLM.called {
		t.Fatal("LLM should have been called for stale session reflection")
	}
	if !store.called {
		t.Fatal("archival store InsertArchival should have been called")
	}
	if store.insertedUserID != "user-alex" {
		t.Fatalf("userID = %q, want %q", store.insertedUserID, "user-alex")
	}
	if store.insertedContent != insight {
		t.Fatalf("content = %q, want %q", store.insertedContent, insight)
	}
	if len(store.insertedTags) != 1 || store.insertedTags[0] != "conversation_reflection" {
		t.Fatalf("tags = %v, want [conversation_reflection]", store.insertedTags)
	}
}

func TestRolloverDeletesHandledSession(t *testing.T) {
	// This test verifies the ConversationReflector returns nil on success,
	// allowing the daemon wiring to proceed to session deletion.
	// The actual Delete call is in daemon.go; here we verify the reflector
	// completes cleanly so the delete path is reachable.
	mockLLM := &convReflectMockLLM{response: "Some insight"}
	store := &convReflectMockStore{}
	cr := NewConversationReflector(mockLLM, store)

	messages := []types.LLMMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	err := cr.Reflect(context.Background(), "user1", messages)
	if err != nil {
		t.Fatalf("Reflect should succeed to allow session deletion, got: %v", err)
	}

	// Verify the reflection completed (store was called)
	if !store.called {
		t.Fatal("store should have been called — reflection must complete for delete to proceed")
	}
}

func TestConversationReflectionFailureIsBestEffort(t *testing.T) {
	// If the LLM fails, Reflect returns an error but the daemon wiring
	// should log and continue (not abort). Verify the error is returned
	// so the daemon can log it, but that it's a wrapped error (not a panic).
	mockLLM := &convReflectMockLLM{err: errors.New("llm temporarily unavailable")}
	store := &convReflectMockStore{}
	cr := NewConversationReflector(mockLLM, store)

	messages := []types.LLMMessage{
		{Role: "user", Content: "some conversation"},
	}

	err := cr.Reflect(context.Background(), "user1", messages)
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
	if !mockLLM.called {
		t.Fatal("LLM should have been called even if it returns error")
	}
	if store.called {
		t.Fatal("store should NOT be called when LLM fails")
	}

	// Verify nil receiver is safe (best-effort, no panic)
	var nilReflector *ConversationReflector
	err = nilReflector.Reflect(context.Background(), "user1", messages)
	if err != nil {
		t.Fatalf("nil receiver should return nil error, got: %v", err)
	}

	// Verify empty messages is a no-op
	cr2 := NewConversationReflector(mockLLM, store)
	store.called = false // reset
	mockLLM.called = false
	err = cr2.Reflect(context.Background(), "user1", nil)
	if err != nil {
		t.Fatalf("empty messages should return nil error, got: %v", err)
	}
	if mockLLM.called {
		t.Fatal("LLM should not be called for empty messages")
	}
}

func TestToolFailureReflectionUnchanged(t *testing.T) {
	// Verify the original Reflector from reflection.go still works correctly —
	// conversation reflection must not interfere with tool-failure reflection.
	expectedLesson := "Validate URLs before passing to http_fetch."
	mockLLM := &reflectionMockLLM{response: expectedLesson}
	store := &reflectionMockStore{}
	r := NewReflector(mockLLM, store)

	lesson, err := r.Reflect(context.Background(), "user42", "http_fetch", `{"url":"bad"}`, "Tool error: invalid URL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lesson != expectedLesson {
		t.Fatalf("lesson = %q, want %q", lesson, expectedLesson)
	}
	if !mockLLM.called {
		t.Fatal("LLM should have been called")
	}
	if !store.called {
		t.Fatal("store should have been called")
	}
	if store.insertedUserID != "user42" {
		t.Fatalf("userID = %q, want %q", store.insertedUserID, "user42")
	}
	// Verify tags are tool-failure reflection tags, NOT conversation_reflection
	if len(store.insertedTags) != 2 || store.insertedTags[0] != "reflection" || store.insertedTags[1] != "http_fetch" {
		t.Fatalf("tags = %v, want [reflection, http_fetch]", store.insertedTags)
	}
}
