//go:build cgo

package memory

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// testSQLiteStore creates a file-backed SQLiteStore in a temp directory.
// Use this instead of :memory: when tests need the full schema including metadata table.
func testSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewSQLiteStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// TestIntegration_CoreMemoryPipeline verifies the full core-memory flow:
// set keys for a user, format them into a prompt, and confirm per-user isolation.
func TestIntegration_CoreMemoryPipeline(t *testing.T) {
	// Use an in-memory DB shared between SQLiteStore schema and CoreStore.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open :memory: db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Bootstrap schemas so both layers work.
	if err := createSchema(db); err != nil {
		t.Fatalf("createSchema: %v", err)
	}
	if err := createArchivalSchema(db); err != nil {
		t.Fatalf("createArchivalSchema: %v", err)
	}

	cs := NewCoreStore(db)
	ctx := context.Background()

	// Set 3 keys for alice.
	if err := cs.Set(ctx, "alice", "name", "Alice"); err != nil {
		t.Fatalf("Set name: %v", err)
	}
	if err := cs.Set(ctx, "alice", "language", "Go"); err != nil {
		t.Fatalf("Set language: %v", err)
	}
	if err := cs.Set(ctx, "alice", "project", "BlackCat"); err != nil {
		t.Fatalf("Set project: %v", err)
	}

	// FormatForPrompt should contain all 3 keys.
	out, err := cs.FormatForPrompt(ctx, "alice")
	if err != nil {
		t.Fatalf("FormatForPrompt alice: %v", err)
	}
	for _, want := range []string{"name: Alice", "language: Go", "project: BlackCat"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatForPrompt missing %q in output:\n%s", want, out)
		}
	}

	// Isolation: bob has no entries → empty string.
	bobOut, err := cs.FormatForPrompt(ctx, "bob")
	if err != nil {
		t.Fatalf("FormatForPrompt bob: %v", err)
	}
	if bobOut != "" {
		t.Errorf("FormatForPrompt bob = %q, want empty string", bobOut)
	}
}

// TestIntegration_ArchivalInsertAndSearch verifies the archival pipeline:
// insert entries, search via FTS, and confirm per-user isolation.
func TestIntegration_ArchivalInsertAndSearch(t *testing.T) {
	store := testSQLiteStore(t)
	ctx := context.Background()

	// Insert 3 entries for alice.
	entries := []struct {
		content string
		tags    []string
	}{
		{"Go is a compiled language designed at Google", []string{"programming", "go"}},
		{"Python is an interpreted scripting language", []string{"programming", "python"}},
		{"Kubernetes orchestrates container workloads", []string{"devops", "k8s"}},
	}
	for _, e := range entries {
		if err := store.InsertArchival(ctx, "alice", e.content, e.tags, nil); err != nil {
			t.Fatalf("InsertArchival: %v", err)
		}
	}

	// Search should return results matching inserted content.
	results, err := store.SearchArchival(ctx, "alice", "programming", nil, 10)
	if err != nil {
		t.Fatalf("SearchArchival: %v", err)
	}
	if len(results) < 1 {
		t.Fatal("SearchArchival returned 0 results, expected at least 1")
	}

	// Verify at least one result contains expected content.
	foundGo := false
	for _, r := range results {
		if strings.Contains(r.Content, "Go is a compiled") {
			foundGo = true
		}
	}
	if !foundGo {
		t.Error("SearchArchival did not return the Go entry for query 'programming'")
	}

	// Count should be 3 for alice.
	count, err := store.CountArchival(ctx, "alice")
	if err != nil {
		t.Fatalf("CountArchival alice: %v", err)
	}
	if count != 3 {
		t.Errorf("CountArchival alice = %d, want 3", count)
	}

	// Isolation: bob should have 0 results.
	bobResults, err := store.SearchArchival(ctx, "bob", "programming", nil, 10)
	if err != nil {
		t.Fatalf("SearchArchival bob: %v", err)
	}
	if len(bobResults) != 0 {
		t.Errorf("SearchArchival bob = %d results, want 0", len(bobResults))
	}

	bobCount, err := store.CountArchival(ctx, "bob")
	if err != nil {
		t.Fatalf("CountArchival bob: %v", err)
	}
	if bobCount != 0 {
		t.Errorf("CountArchival bob = %d, want 0", bobCount)
	}
}

// TestIntegration_MigrateFromMemoryMD verifies that MEMORY.md migration
// imports content lines, skips blanks and comments, and is idempotent.
func TestIntegration_MigrateFromMemoryMD(t *testing.T) {
	// Write a temp MEMORY.md with 3 content lines, 1 blank, 1 comment.
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "MEMORY.md")
	mdContent := "User prefers dark mode\n\nProject uses Go 1.25\n# This is a comment\nDeploy target is Linux arm64\n"
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatalf("WriteFile MEMORY.md: %v", err)
	}

	// Use file-backed SQLiteStore (NewSQLiteStore creates metadata table).
	store := testSQLiteStore(t)
	ctx := context.Background()

	// First migration: should import 3 lines.
	count, err := MigrateFromMemoryMD(ctx, mdPath, store, nil, "default")
	if err != nil {
		t.Fatalf("MigrateFromMemoryMD (first): %v", err)
	}
	if count != 3 {
		t.Errorf("MigrateFromMemoryMD count = %d, want 3", count)
	}

	// Second migration: idempotent → 0 new entries.
	count2, err := MigrateFromMemoryMD(ctx, mdPath, store, nil, "default")
	if err != nil {
		t.Fatalf("MigrateFromMemoryMD (second): %v", err)
	}
	if count2 != 0 {
		t.Errorf("MigrateFromMemoryMD idempotent count = %d, want 0", count2)
	}

	// CountArchival should still be 3.
	total, err := store.CountArchival(ctx, "default")
	if err != nil {
		t.Fatalf("CountArchival: %v", err)
	}
	if total != 3 {
		t.Errorf("CountArchival after migration = %d, want 3", total)
	}
}
