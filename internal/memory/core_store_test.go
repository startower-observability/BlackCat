//go:build cgo

package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestCoreStore(t *testing.T) *CoreStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open :memory: db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewCoreStore(db)
}

func TestCoreStore_SetGet(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	// Set a value and read it back.
	if err := cs.Set(ctx, "user1", "name", "John"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := cs.Get(ctx, "user1", "name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "John" {
		t.Errorf("Get = %q, want %q", val, "John")
	}
}

func TestCoreStore_GetNotFound(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	val, err := cs.Get(ctx, "user1", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "" {
		t.Errorf("Get nonexistent = %q, want empty string", val)
	}
}

func TestCoreStore_SetOverwrite(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	if err := cs.Set(ctx, "user1", "lang", "English"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cs.Set(ctx, "user1", "lang", "Indonesian"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}

	val, err := cs.Get(ctx, "user1", "lang")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "Indonesian" {
		t.Errorf("Get after overwrite = %q, want %q", val, "Indonesian")
	}
}

func TestCoreStore_GetAll_Isolation(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	// User1 entries.
	if err := cs.Set(ctx, "user1", "name", "Alice"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cs.Set(ctx, "user1", "lang", "Go"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// User2 entries.
	if err := cs.Set(ctx, "user2", "name", "Bob"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	pairs, err := cs.GetAll(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("GetAll user1 count = %d, want 2", len(pairs))
	}
	if pairs["name"] != "Alice" {
		t.Errorf("user1 name = %q, want %q", pairs["name"], "Alice")
	}
	if pairs["lang"] != "Go" {
		t.Errorf("user1 lang = %q, want %q", pairs["lang"], "Go")
	}

	// Verify user2 is isolated.
	pairs2, err := cs.GetAll(ctx, "user2")
	if err != nil {
		t.Fatalf("GetAll user2: %v", err)
	}
	if len(pairs2) != 1 {
		t.Fatalf("GetAll user2 count = %d, want 1", len(pairs2))
	}
	if pairs2["name"] != "Bob" {
		t.Errorf("user2 name = %q, want %q", pairs2["name"], "Bob")
	}
}

func TestCoreStore_Delete(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	if err := cs.Set(ctx, "user1", "name", "John"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cs.Delete(ctx, "user1", "name"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	val, err := cs.Get(ctx, "user1", "name")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if val != "" {
		t.Errorf("Get after delete = %q, want empty", val)
	}
}

func TestCoreStore_DeleteNonexistent(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	// Should not error.
	if err := cs.Delete(ctx, "user1", "nonexistent"); err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}
}

func TestCoreStore_MaxEntries(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	// Insert 21 entries. The oldest should be evicted.
	for i := 1; i <= 21; i++ {
		key := fmt.Sprintf("key%02d", i)
		val := fmt.Sprintf("val%02d", i)
		if err := cs.Set(ctx, "user1", key, val); err != nil {
			t.Fatalf("Set key%02d: %v", i, err)
		}
	}

	pairs, err := cs.GetAll(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(pairs) != 20 {
		t.Fatalf("entries after 21 inserts = %d, want 20", len(pairs))
	}

	// key01 should have been evicted (oldest by updated_at).
	if _, ok := pairs["key01"]; ok {
		t.Error("key01 should have been evicted")
	}
	// key02..key21 should exist.
	for i := 2; i <= 21; i++ {
		key := fmt.Sprintf("key%02d", i)
		if _, ok := pairs[key]; !ok {
			t.Errorf("%s should exist but was evicted", key)
		}
	}
}

func TestCoreStore_MaxEntries_UpdateDoesNotEvict(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	// Fill to max.
	for i := 1; i <= 20; i++ {
		key := fmt.Sprintf("key%02d", i)
		if err := cs.Set(ctx, "user1", key, "v1"); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	// Update an existing key — should NOT evict anything.
	if err := cs.Set(ctx, "user1", "key01", "updated"); err != nil {
		t.Fatalf("Set update: %v", err)
	}

	pairs, err := cs.GetAll(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(pairs) != 20 {
		t.Fatalf("entries after update = %d, want 20", len(pairs))
	}
	if pairs["key01"] != "updated" {
		t.Errorf("key01 = %q, want %q", pairs["key01"], "updated")
	}
}

func TestCoreStore_ValueTruncation(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	longVal := strings.Repeat("x", 600)
	if err := cs.Set(ctx, "user1", "bio", longVal); err != nil {
		t.Fatalf("Set long value: %v", err)
	}

	val, err := cs.Get(ctx, "user1", "bio")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(val) != 500 {
		t.Errorf("value length = %d, want 500", len(val))
	}
}

func TestCoreStore_FormatForPrompt_Empty(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	out, err := cs.FormatForPrompt(ctx, "user1")
	if err != nil {
		t.Fatalf("FormatForPrompt: %v", err)
	}
	if out != "" {
		t.Errorf("FormatForPrompt empty user = %q, want empty string", out)
	}
}

func TestCoreStore_FormatForPrompt_WithEntries(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	if err := cs.Set(ctx, "user1", "name", "John"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cs.Set(ctx, "user1", "language", "Indonesian"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cs.Set(ctx, "user1", "project", "BlackCat"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	out, err := cs.FormatForPrompt(ctx, "user1")
	if err != nil {
		t.Fatalf("FormatForPrompt: %v", err)
	}

	want := "### Core Memory\n- language: Indonesian\n- name: John\n- project: BlackCat\n"
	if out != want {
		t.Errorf("FormatForPrompt:\ngot:  %q\nwant: %q", out, want)
	}
}

func TestCoreStore_FormatForPrompt_Isolation(t *testing.T) {
	cs := newTestCoreStore(t)
	ctx := context.Background()

	if err := cs.Set(ctx, "user1", "name", "Alice"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := cs.Set(ctx, "user2", "name", "Bob"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	out, err := cs.FormatForPrompt(ctx, "user1")
	if err != nil {
		t.Fatalf("FormatForPrompt: %v", err)
	}
	if !strings.Contains(out, "Alice") {
		t.Error("user1 prompt should contain Alice")
	}
	if strings.Contains(out, "Bob") {
		t.Error("user1 prompt should NOT contain Bob")
	}
}
