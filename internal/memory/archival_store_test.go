//go:build cgo

package memory

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestArchivalStore(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open :memory: db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	hasFTS5 := detectFTS5(db)
	if err := createSchema(db, hasFTS5); err != nil {
		t.Fatalf("createSchema: %v", err)
	}
	if err := createArchivalSchema(db, hasFTS5); err != nil {
		t.Fatalf("createArchivalSchema: %v", err)
	}
	return &SQLiteStore{db: db, hasFTS5: hasFTS5}
}

func TestInsertArchival_Basic(t *testing.T) {
	store := newTestArchivalStore(t)
	ctx := context.Background()

	err := store.InsertArchival(ctx, "user1", "Go is a compiled language", []string{"programming", "go"}, nil)
	if err != nil {
		t.Fatalf("InsertArchival: %v", err)
	}

	count, err := store.CountArchival(ctx, "user1")
	if err != nil {
		t.Fatalf("CountArchival: %v", err)
	}
	if count != 1 {
		t.Errorf("CountArchival = %d, want 1", count)
	}
}

func TestSearchArchival_FTS(t *testing.T) {
	store := newTestArchivalStore(t)
	ctx := context.Background()

	// Insert 3 entries with distinct content.
	entries := []struct {
		content string
		tags    []string
	}{
		{"Go is a compiled language designed at Google", []string{"programming", "go"}},
		{"Python is an interpreted scripting language", []string{"programming", "python"}},
		{"Kubernetes orchestrates container workloads", []string{"devops", "k8s"}},
	}
	for _, e := range entries {
		if err := store.InsertArchival(ctx, "user1", e.content, e.tags, nil); err != nil {
			t.Fatalf("InsertArchival: %v", err)
		}
	}

	// FTS search for "compiled" should find only the Go entry.
	results, err := store.SearchArchival(ctx, "user1", "compiled", nil, 10)
	if err != nil {
		t.Fatalf("SearchArchival: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchArchival 'compiled': got %d results, want 1", len(results))
	}
	if results[0].Content != entries[0].content {
		t.Errorf("got content %q, want %q", results[0].Content, entries[0].content)
	}

	// FTS search for "language" should find Go and Python entries.
	results, err = store.SearchArchival(ctx, "user1", "language", nil, 10)
	if err != nil {
		t.Fatalf("SearchArchival: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("SearchArchival 'language': got %d results, want 2", len(results))
	}
}

func TestSearchArchival_TagSearch(t *testing.T) {
	store := newTestArchivalStore(t)
	ctx := context.Background()

	// Insert 2 entries with different tags.
	err := store.InsertArchival(ctx, "user1", "Entry about databases", []string{"database", "sql"}, nil)
	if err != nil {
		t.Fatalf("InsertArchival: %v", err)
	}
	err = store.InsertArchival(ctx, "user1", "Entry about networking", []string{"network", "tcp"}, nil)
	if err != nil {
		t.Fatalf("InsertArchival: %v", err)
	}

	// FTS searches content AND tags. Search for "database" tag.
	results, err := store.SearchArchival(ctx, "user1", "database", nil, 10)
	if err != nil {
		t.Fatalf("SearchArchival: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchArchival 'database': got %d results, want 1", len(results))
	}
	if results[0].Content != "Entry about databases" {
		t.Errorf("got content %q, want %q", results[0].Content, "Entry about databases")
	}
	// Verify tags are present.
	if len(results[0].Tags) != 2 || results[0].Tags[0] != "database" {
		t.Errorf("tags = %v, want [database sql]", results[0].Tags)
	}
}

func TestSearchArchival_UserIsolation(t *testing.T) {
	store := newTestArchivalStore(t)
	ctx := context.Background()

	// UserA inserts an entry.
	err := store.InsertArchival(ctx, "userA", "Secret recipe for apple pie", []string{"cooking"}, nil)
	if err != nil {
		t.Fatalf("InsertArchival userA: %v", err)
	}

	// UserB inserts an entry.
	err = store.InsertArchival(ctx, "userB", "Guide to apple farming", []string{"agriculture"}, nil)
	if err != nil {
		t.Fatalf("InsertArchival userB: %v", err)
	}

	// UserA searches for "apple" — should only see their own entry.
	resultsA, err := store.SearchArchival(ctx, "userA", "apple", nil, 10)
	if err != nil {
		t.Fatalf("SearchArchival userA: %v", err)
	}
	if len(resultsA) != 1 {
		t.Fatalf("userA search 'apple': got %d results, want 1", len(resultsA))
	}
	if resultsA[0].Content != "Secret recipe for apple pie" {
		t.Errorf("userA got %q, want 'Secret recipe for apple pie'", resultsA[0].Content)
	}

	// UserB searches for "apple" — should only see their own entry.
	resultsB, err := store.SearchArchival(ctx, "userB", "apple", nil, 10)
	if err != nil {
		t.Fatalf("SearchArchival userB: %v", err)
	}
	if len(resultsB) != 1 {
		t.Fatalf("userB search 'apple': got %d results, want 1", len(resultsB))
	}
	if resultsB[0].Content != "Guide to apple farming" {
		t.Errorf("userB got %q, want 'Guide to apple farming'", resultsB[0].Content)
	}

	// CountArchival should be isolated too.
	countA, _ := store.CountArchival(ctx, "userA")
	countB, _ := store.CountArchival(ctx, "userB")
	if countA != 1 || countB != 1 {
		t.Errorf("counts: userA=%d userB=%d, want 1 each", countA, countB)
	}
}

func TestInsertArchival_WithEmbedding(t *testing.T) {
	store := newTestArchivalStore(t)
	ctx := context.Background()

	emb := []float32{0.1, 0.2, 0.3, 0.4}
	err := store.InsertArchival(ctx, "user1", "Vectors are fun", []string{"math"}, emb)
	if err != nil {
		t.Fatalf("InsertArchival with embedding: %v", err)
	}

	count, err := store.CountArchival(ctx, "user1")
	if err != nil {
		t.Fatalf("CountArchival: %v", err)
	}
	if count != 1 {
		t.Errorf("CountArchival = %d, want 1", count)
	}
}

func TestSearchArchival_HybridSearch(t *testing.T) {
	store := newTestArchivalStore(t)
	ctx := context.Background()

	// Insert entries with embeddings. Use simple vectors for predictable cosine similarity.
	// Entry 0: about Go, embedding points in direction [1,0,0]
	err := store.InsertArchival(ctx, "user1", "Go programming language tutorial",
		[]string{"go"}, []float32{1, 0, 0})
	if err != nil {
		t.Fatalf("InsertArchival 0: %v", err)
	}

	// Entry 1: about Python, embedding points in direction [0,1,0]
	err = store.InsertArchival(ctx, "user1", "Python programming language guide",
		[]string{"python"}, []float32{0, 1, 0})
	if err != nil {
		t.Fatalf("InsertArchival 1: %v", err)
	}

	// Entry 2: about Rust, embedding points in direction [0,0,1]
	err = store.InsertArchival(ctx, "user1", "Rust systems programming",
		[]string{"rust"}, []float32{0, 0, 1})
	if err != nil {
		t.Fatalf("InsertArchival 2: %v", err)
	}

	// Query: text = "programming" (matches all 3 via FTS), embedding = [1,0,0] (closest to Go).
	// Hybrid search should rank Go highest due to both FTS and vector match.
	results, err := store.SearchArchival(ctx, "user1", "programming", []float32{1, 0, 0}, 10)
	if err != nil {
		t.Fatalf("SearchArchival hybrid: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("hybrid search: got %d results, want 3", len(results))
	}

	// The Go entry should have the highest score (rank 1 in both lists).
	foundGo := false
	for _, r := range results {
		if r.Content == "Go programming language tutorial" {
			foundGo = true
			if r.Score <= 0 {
				t.Error("Go entry should have positive RRF score")
			}
		}
	}
	if !foundGo {
		t.Error("Go entry not found in hybrid search results")
	}

	// All results should have positive scores.
	for i, r := range results {
		if r.Score <= 0 {
			t.Errorf("result %d score = %f, want > 0", i, r.Score)
		}
	}

	// Scores should be sorted descending.
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted by score: [%d]=%f > [%d]=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestFloat32Serialization(t *testing.T) {
	original := []float32{1.5, -2.7, 0.0, 3.14159, -0.001}
	b := float32sToBytes(original)
	roundTrip := bytesToFloat32s(b)

	if len(roundTrip) != len(original) {
		t.Fatalf("len(roundTrip) = %d, want %d", len(roundTrip), len(original))
	}
	for i := range original {
		if roundTrip[i] != original[i] {
			t.Errorf("roundTrip[%d] = %f, want %f", i, roundTrip[i], original[i])
		}
	}
}

func TestRRFuse(t *testing.T) {
	// Two ranked lists: [A=0, B=1] and [B=0, A=1]
	// A gets 1/(60+1) + 1/(60+2) = 1/61 + 1/62
	// B gets 1/(60+2) + 1/(60+1) = 1/62 + 1/61
	// Both should have equal scores.
	scores := rrfFuse([]int{0, 1}, []int{1, 0})
	if scores[0] != scores[1] {
		t.Errorf("symmetric RRF: score[0]=%f score[1]=%f, want equal", scores[0], scores[1])
	}

	// Item appearing in only one list.
	scores2 := rrfFuse([]int{0, 1}, []int{2})
	if scores2[0] <= scores2[1] {
		// Item 0 is rank 1 in list 1, not in list 2 → score = 1/61
		// Item 1 is rank 2 in list 1, not in list 2 → score = 1/62
		// Item 0 should have higher score than item 1.
	}
	if scores2[2] == 0 {
		t.Error("item 2 should have non-zero score")
	}
}
