package agent

import (
	"context"
	"fmt"
	"testing"
)

// --- Mock implementations ---

type mockCoreMemoryWriter struct {
	data     map[string]string // key → value (single user)
	setCalls []setCalled
	getErr   error
	setErr   error
}

type setCalled struct {
	UserID, Key, Value string
}

func newMockCoreWriter() *mockCoreMemoryWriter {
	return &mockCoreMemoryWriter{data: make(map[string]string)}
}

func (m *mockCoreMemoryWriter) Get(_ context.Context, userID, key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.data[key], nil
}

func (m *mockCoreMemoryWriter) Set(_ context.Context, userID, key, value string) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	m.setCalls = append(m.setCalls, setCalled{userID, key, value})
	return nil
}

type mockPrefWriter struct {
	calls  []setCalled
	updErr error
}

func newMockPrefWriter() *mockPrefWriter {
	return &mockPrefWriter{}
}

func (m *mockPrefWriter) UpdatePreference(_ context.Context, userID, key, value string) error {
	if m.updErr != nil {
		return m.updErr
	}
	m.calls = append(m.calls, setCalled{userID, key, value})
	return nil
}

// --- TestExtractExplicitUserKnowledge ---

func TestExtractExplicitUserKnowledge(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
	}{
		// user:name patterns
		{"name: my name is", "my name is Alice", "user:name", "Alice"},
		{"name: My name is multi", "My name is John Doe", "user:name", "John Doe"},
		{"name: i am Name", "I am Alice", "user:name", "Alice"},
		{"name: call me X (name fallback)", "call me Bob", "user:name", "Bob"},

		// user:nickname patterns
		{"nickname: my nickname is", "my nickname is Ace", "user:nickname", "Ace"},
		{"nickname: you can call me", "you can call me Buddy", "user:nickname", "Buddy"},

		// user:timezone patterns
		{"timezone: my timezone is", "my timezone is UTC+7", "user:timezone", "Utc+7"},
		{"timezone: i'm in timezone", "i'm in timezone Asia/Jakarta", "user:timezone", "Asia/jakarta"},
		{"timezone: i am in timezone", "i am in timezone PST", "user:timezone", "Pst"},

		// user:locale patterns
		{"locale: i am from", "I am from Indonesia", "user:locale", "Indonesia"},
		{"locale: my locale is", "my locale is en-US", "user:locale", "En-us"},

		// pref:language patterns
		{"lang: reply in", "reply in English", "pref:language", "english"},
		{"lang: speak in", "please speak in Spanish", "pref:language", "spanish"},
		{"lang: use X language", "use French language", "pref:language", "french"},
		{"lang: respond in", "respond in Japanese", "pref:language", "japanese"},

		// pref:style patterns
		{"style: use X style", "use casual style", "pref:style", "casual"},
		{"style: write in X style", "write in formal style", "pref:style", "formal"},

		// pref:verbosity patterns
		{"verbosity: be verbose", "be verbose", "pref:verbosity", "verbose"},
		{"verbosity: be concise", "be concise", "pref:verbosity", "concise"},
		{"verbosity: keep it brief", "keep it brief", "pref:verbosity", "brief"},
		{"verbosity: be detailed", "be detailed", "pref:verbosity", "detailed"},

		// pref:technical_depth patterns
		{"depth: explain at X level", "explain at beginner level", "pref:technical_depth", "beginner"},
		{"depth: X level explanations", "expert level explanations", "pref:technical_depth", "expert"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			facts := ExtractExplicitUserKnowledge(tt.input)
			found := false
			for _, f := range facts {
				if f.Key == tt.wantKey {
					if f.Value != tt.wantVal {
						t.Errorf("key %q: got value %q, want %q", tt.wantKey, f.Value, tt.wantVal)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("key %q not found in facts: %v", tt.wantKey, facts)
			}
		})
	}
}

func TestExtractExplicitUserKnowledge_MultipleKeys(t *testing.T) {
	msg := "My name is Alice and reply in Spanish"
	facts := ExtractExplicitUserKnowledge(msg)
	if len(facts) < 2 {
		t.Fatalf("expected at least 2 facts, got %d: %v", len(facts), facts)
	}
	keys := make(map[string]string)
	for _, f := range facts {
		keys[f.Key] = f.Value
	}
	if keys["user:name"] != "Alice" {
		t.Errorf("user:name = %q, want Alice", keys["user:name"])
	}
	if keys["pref:language"] != "spanish" {
		t.Errorf("pref:language = %q, want spanish", keys["pref:language"])
	}
}

func TestExtractExplicitUserKnowledge_NicknameOverName(t *testing.T) {
	msg := "you can call me Ace"
	facts := ExtractExplicitUserKnowledge(msg)
	// "you can call me" → nickname. "call me" also matches → should NOT duplicate.
	keys := make(map[string]string)
	for _, f := range facts {
		keys[f.Key] = f.Value
	}
	if keys["user:nickname"] != "Ace" {
		t.Errorf("user:nickname = %q, want Ace", keys["user:nickname"])
	}
}

func TestExtractExplicitUserKnowledge_TruncateLongValue(t *testing.T) {
	// Build a message with a very long "name"
	longName := "My name is " + "Abcdefghij"
	// 100+ chars constructed
	for len(longName) < 120 {
		longName += "abcdefghij"
	}
	// This won't match the regex anyway (regex captures 1-3 capitalized words).
	// Test truncation via a simpler approach: long locale.
	longLocale := "my locale is " + "Abcdefghijklmnopqrstuvwxyz Abcdefghijklmnopqrstuvwxyz Abcdefghijklmnopqrstuvwxyz Abcdefghijklmnopqrstuvwxyz Abcdefghijklmnopqrstuvwxyz"
	facts := ExtractExplicitUserKnowledge(longLocale)
	for _, f := range facts {
		if f.Key == "user:locale" && len(f.Value) > maxFactValueLen {
			t.Errorf("value length %d exceeds max %d", len(f.Value), maxFactValueLen)
		}
	}
}

func TestExtractExplicitUserKnowledge_EmptyMessage(t *testing.T) {
	facts := ExtractExplicitUserKnowledge("")
	if len(facts) != 0 {
		t.Errorf("expected no facts from empty message, got %v", facts)
	}
}

// --- TestRejectsInferredPreferences ---

func TestRejectsInferredPreferences(t *testing.T) {
	rejections := []struct {
		name string
		msg  string
	}{
		{"happy state", "i am happy"},
		{"tired state", "i am tired"},
		{"confused state", "i am confused"},
		{"fine state", "i am fine"},
		{"emotion", "i love this!"},
		{"feeling", "i'm feeling stressed"},
		{"tone comment", "you sound smart"},
		{"opinion", "this is great work"},
		{"greeting", "hello, how are you?"},
		{"question", "what do you think?"},
		{"lowercase non-name", "i am going to the store"},
		{"working state", "i am working on it"},
	}

	for _, tt := range rejections {
		t.Run(tt.name, func(t *testing.T) {
			facts := ExtractExplicitUserKnowledge(tt.msg)
			if len(facts) > 0 {
				t.Errorf("expected no facts from %q, but got %v", tt.msg, facts)
			}
		})
	}
}

// --- TestDoesNotPersistToolDerivedFacts ---

func TestDoesNotPersistToolDerivedFacts(t *testing.T) {
	toolOutputs := []struct {
		name string
		msg  string
	}{
		{"result output", "result: 42"},
		{"error output", "error: connection refused"},
		{"json output", `{"status": "ok", "count": 5}`},
		{"code block", "```go\nfmt.Println(\"hello\")\n```"},
		{"tool response", "The function returned 200 OK"},
		{"generic message", "can you help me debug this?"},
		{"weather", "the weather is nice today"},
	}

	for _, tt := range toolOutputs {
		t.Run(tt.name, func(t *testing.T) {
			facts := ExtractExplicitUserKnowledge(tt.msg)
			if len(facts) > 0 {
				t.Errorf("expected no facts from tool-like output %q, but got %v", tt.msg, facts)
			}
		})
	}
}

// --- TestSuccessfulTurnPersistsExplicitKnowledge ---

func TestSuccessfulTurnPersistsExplicitKnowledge(t *testing.T) {
	ctx := context.Background()

	t.Run("persists name via coreStore", func(t *testing.T) {
		coreWriter := newMockCoreWriter()
		prefWriter := newMockPrefWriter()

		err := ApplyExplicitLearning(ctx, "user-1", "my name is Alice", coreWriter, prefWriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(coreWriter.setCalls) != 1 {
			t.Fatalf("expected 1 Set call, got %d", len(coreWriter.setCalls))
		}
		c := coreWriter.setCalls[0]
		if c.Key != "user:name" || c.Value != "Alice" {
			t.Errorf("Set call = {%q, %q}, want {user:name, Alice}", c.Key, c.Value)
		}
		if c.UserID != "user-1" {
			t.Errorf("UserID = %q, want user-1", c.UserID)
		}
	})

	t.Run("persists language via prefMgr", func(t *testing.T) {
		coreWriter := newMockCoreWriter()
		prefWriter := newMockPrefWriter()

		err := ApplyExplicitLearning(ctx, "user-2", "reply in French", coreWriter, prefWriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(prefWriter.calls) != 1 {
			t.Fatalf("expected 1 UpdatePreference call, got %d", len(prefWriter.calls))
		}
		c := prefWriter.calls[0]
		if c.Key != "pref:language" || c.Value != "french" {
			t.Errorf("UpdatePreference call = {%q, %q}, want {pref:language, french}", c.Key, c.Value)
		}
	})

	t.Run("skips write when value unchanged", func(t *testing.T) {
		coreWriter := newMockCoreWriter()
		coreWriter.data["user:name"] = "Alice" // Already stored
		prefWriter := newMockPrefWriter()

		err := ApplyExplicitLearning(ctx, "user-1", "my name is Alice", coreWriter, prefWriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(coreWriter.setCalls) != 0 {
			t.Errorf("expected 0 Set calls (value unchanged), got %d", len(coreWriter.setCalls))
		}
	})

	t.Run("no-op on nil stores", func(t *testing.T) {
		err := ApplyExplicitLearning(ctx, "user-1", "my name is Alice", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no-op on empty message", func(t *testing.T) {
		coreWriter := newMockCoreWriter()
		prefWriter := newMockPrefWriter()

		err := ApplyExplicitLearning(ctx, "user-1", "", coreWriter, prefWriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(coreWriter.setCalls) != 0 {
			t.Errorf("expected 0 Set calls, got %d", len(coreWriter.setCalls))
		}
	})

	t.Run("continues on error and returns first error", func(t *testing.T) {
		coreWriter := newMockCoreWriter()
		coreWriter.setErr = fmt.Errorf("db write failed")
		prefWriter := newMockPrefWriter()

		// Message with both user:name and pref:language
		err := ApplyExplicitLearning(ctx, "user-1", "my name is Alice and reply in French", coreWriter, prefWriter)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "db write failed" {
			t.Errorf("error = %q, want 'db write failed'", err.Error())
		}
		// pref writer should still have been called
		if len(prefWriter.calls) != 1 {
			t.Errorf("expected 1 pref call (continue on error), got %d", len(prefWriter.calls))
		}
	})

	t.Run("persists multiple facts from one message", func(t *testing.T) {
		coreWriter := newMockCoreWriter()
		prefWriter := newMockPrefWriter()

		err := ApplyExplicitLearning(ctx, "user-1", "My name is Alice, reply in Spanish, be concise", coreWriter, prefWriter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(coreWriter.setCalls) != 1 {
			t.Fatalf("expected 1 core Set call, got %d", len(coreWriter.setCalls))
		}
		if coreWriter.setCalls[0].Key != "user:name" || coreWriter.setCalls[0].Value != "Alice" {
			t.Errorf("core Set = %v, want user:name=Alice", coreWriter.setCalls[0])
		}
		if len(prefWriter.calls) != 2 {
			t.Fatalf("expected 2 pref calls, got %d: %v", len(prefWriter.calls), prefWriter.calls)
		}
	})
}
