package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements Store with SQLite and FTS5 full-text search.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-backed memory store at the given path.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("memory: create dir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("memory: open sqlite: %w", err)
	}

	if err := createSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("memory: create schema: %w", err)
	}

	if err := createArchivalSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("memory: create archival schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		content    TEXT    NOT NULL,
		tags       TEXT    DEFAULT '',
		source     TEXT    DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		content, tags,
		content='memories',
		content_rowid='id'
	);

	CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
		INSERT INTO memories_fts(rowid, content, tags)
		VALUES (new.id, new.content, new.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, content, tags)
		VALUES ('delete', old.id, old.content, old.tags);
	END;
	CREATE TABLE IF NOT EXISTS metadata (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	_, err := db.Exec(schema)
	return err
}

// Read returns all entries (up to 100, in chronological order).
func (s *SQLiteStore) Read(ctx context.Context) ([]Entry, error) {
	return s.Recent(ctx, 100)
}

// Write appends a new memory entry.
func (s *SQLiteStore) Write(ctx context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tags := strings.Join(entry.Tags, ",")
	ts := entry.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memories (content, tags, source, created_at) VALUES (?, ?, ?, ?)`,
		entry.Content, tags, tagSource(entry.Tags), ts.UTC(),
	)
	if err != nil {
		return fmt.Errorf("memory: sqlite write: %w", err)
	}
	return nil
}

// Search returns entries matching the query using FTS5.
func (s *SQLiteStore) Search(ctx context.Context, query string) ([]Entry, error) {
	return s.SearchWithLimit(ctx, query, 10)
}

// SearchWithLimit performs FTS5 search with a configurable result limit.
func (s *SQLiteStore) SearchWithLimit(ctx context.Context, query string, limit int) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT m.content, m.tags, m.created_at
		 FROM memories m
		 JOIN memories_fts fts ON m.id = fts.rowid
		 WHERE memories_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		ftsQuery(query), limit,
	)
	if err != nil {
		// Fallback to LIKE if FTS fails (e.g. invalid query syntax)
		return s.searchLike(ctx, query, limit)
	}
	defer rows.Close()

	return scanRows(rows)
}

func (s *SQLiteStore) searchLike(ctx context.Context, query string, limit int) ([]Entry, error) {
	like := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT content, tags, created_at FROM memories
		 WHERE content LIKE ? OR tags LIKE ?
		 ORDER BY created_at DESC LIMIT ?`,
		like, like, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: sqlite like search: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// Recent returns the most recent entries in chronological order.
func (s *SQLiteStore) Recent(ctx context.Context, limit int) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT content, tags, created_at FROM memories ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: sqlite recent: %w", err)
	}
	defer rows.Close()

	entries, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	// Reverse to chronological order (oldest first)
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, nil
}

// Add is a convenience method for adding a memory entry.
func (s *SQLiteStore) Add(ctx context.Context, content string, tags []string, source string) error {
	return s.Write(ctx, Entry{
		Timestamp: time.Now(),
		Content:   content,
		Tags:      tags,
	})
}

// Count returns the total number of stored memories.
func (s *SQLiteStore) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`).Scan(&n); err != nil {
		return 0, fmt.Errorf("memory: sqlite count: %w", err)
	}
	return n, nil
}

// Close closes the SQLite database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for shared use.
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// Consolidate is a no-op for SQLiteStore.
func (s *SQLiteStore) Consolidate(ctx context.Context) error {
	return nil
}

// MigrateFromFileStore imports entries from a FileStore into SQLite.
func (s *SQLiteStore) MigrateFromFileStore(ctx context.Context, fs *FileStore) (int, error) {
	entries, err := fs.Read(ctx)
	if err != nil {
		return 0, fmt.Errorf("memory: migrate read: %w", err)
	}

	imported := 0
	for _, e := range entries {
		if err := s.Write(ctx, e); err != nil {
			continue
		}
		imported++
	}
	return imported, nil
}

func scanRows(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry
	for rows.Next() {
		var content, tags string
		var createdAt time.Time
		if err := rows.Scan(&content, &tags, &createdAt); err != nil {
			return nil, fmt.Errorf("memory: sqlite scan: %w", err)
		}

		var tagSlice []string
		if tags != "" {
			for _, t := range strings.Split(tags, ",") {
				if s := strings.TrimSpace(t); s != "" {
					tagSlice = append(tagSlice, s)
				}
			}
		}

		entries = append(entries, Entry{
			Timestamp: createdAt,
			Content:   content,
			Tags:      tagSlice,
		})
	}
	return entries, rows.Err()
}

// ftsQuery builds an FTS5 MATCH query from user input.
func ftsQuery(query string) string {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return `""`
	}
	escaped := make([]string, len(terms))
	for i, t := range terms {
		t = strings.NewReplacer(`"`, "", "*", "", "(", "", ")", "", ":", "").Replace(t)
		escaped[i] = `"` + t + `"` + "*"
	}
	return strings.Join(escaped, " ")
}

// tagSource extracts the first tag as source, or returns empty string.
func tagSource(tags []string) string {
	if len(tags) > 0 {
		return tags[0]
	}
	return ""
}

// ---------------------------------------------------------------------------
// Archival Memory
// ---------------------------------------------------------------------------

func createArchivalSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS archival_memory (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    TEXT    NOT NULL,
		content    TEXT    NOT NULL,
		embedding  BLOB,
		tags       TEXT    DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS archival_memory_fts USING fts5(
		content, tags,
		content='archival_memory',
		content_rowid='id'
	);

	CREATE TRIGGER IF NOT EXISTS archival_memory_ai AFTER INSERT ON archival_memory BEGIN
		INSERT INTO archival_memory_fts(rowid, content, tags)
		VALUES (new.id, new.content, new.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS archival_memory_ad AFTER DELETE ON archival_memory BEGIN
		INSERT INTO archival_memory_fts(archival_memory_fts, rowid, content, tags)
		VALUES ('delete', old.id, old.content, old.tags);
	END;
	`
	_, err := db.Exec(schema)
	return err
}

// float32sToBytes serializes a float32 slice to little-endian bytes.
func float32sToBytes(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// bytesToFloat32s deserializes little-endian bytes to a float32 slice.
func bytesToFloat32s(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// InsertArchival adds a new entry to archival memory for a user.
func (s *SQLiteStore) InsertArchival(ctx context.Context, userID, content string, tags []string, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tagStr := strings.Join(tags, ",")
	var embBlob []byte
	if len(embedding) > 0 {
		embBlob = float32sToBytes(embedding)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO archival_memory (user_id, content, embedding, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		userID, content, embBlob, tagStr,
	)
	if err != nil {
		return fmt.Errorf("memory: archival insert: %w", err)
	}
	return nil
}

// SearchArchival performs hybrid search (FTS5 + vector cosine similarity)
// across archival memory for a user. If embedding is nil, only FTS5 is used.
// Results are fused using Reciprocal Rank Fusion (RRF) with k=60.
func (s *SQLiteStore) SearchArchival(ctx context.Context, userID, query string, embedding []float32, limit int) ([]ArchivalResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// --- FTS5 ranked list ---
	ftsRanked, ftsResults, err := s.archivalFTSSearch(ctx, userID, query)
	if err != nil {
		return nil, err
	}

	// If no embedding provided, return FTS-only results.
	if embedding == nil {
		if len(ftsResults) > limit {
			ftsResults = ftsResults[:limit]
		}
		return ftsResults, nil
	}

	// --- Vector ranked list ---
	vecRanked, vecResults, err := s.archivalVectorSearch(ctx, userID, embedding)
	if err != nil {
		return nil, err
	}

	// --- RRF Fusion ---
	// Build a combined pool of all unique results keyed by archival_memory.id.
	// We use the row content as the canonical result.
	type indexedResult struct {
		result ArchivalResult
		rowID  int64
	}

	pool := make(map[int64]*indexedResult)
	for i, id := range ftsRanked {
		pool[id] = &indexedResult{result: ftsResults[i], rowID: id}
	}
	for i, id := range vecRanked {
		if _, ok := pool[id]; !ok {
			pool[id] = &indexedResult{result: vecResults[i], rowID: id}
		}
	}

	// Build int-indexed lists for RRF
	idToIdx := make(map[int64]int)
	idxToResult := make([]ArchivalResult, 0, len(pool))
	for id, ir := range pool {
		idToIdx[id] = len(idxToResult)
		idxToResult = append(idxToResult, ir.result)
	}

	ftsIntRanked := make([]int, len(ftsRanked))
	for i, id := range ftsRanked {
		ftsIntRanked[i] = idToIdx[id]
	}
	vecIntRanked := make([]int, len(vecRanked))
	for i, id := range vecRanked {
		vecIntRanked[i] = idToIdx[id]
	}

	scores := rrfFuse(ftsIntRanked, vecIntRanked)
	for idx, score := range scores {
		idxToResult[idx].Score = score
	}

	sortByScoreDesc(idxToResult)

	if len(idxToResult) > limit {
		idxToResult = idxToResult[:limit]
	}
	return idxToResult, nil
}

// archivalFTSSearch returns FTS5-ranked archival results for a user.
// Returns (rowIDs, results, error) where rowIDs are ordered by FTS5 rank.
func (s *SQLiteStore) archivalFTSSearch(ctx context.Context, userID, query string) ([]int64, []ArchivalResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.content, m.tags, m.created_at
		 FROM archival_memory m
		 JOIN archival_memory_fts fts ON m.id = fts.rowid
		 WHERE m.user_id = ? AND archival_memory_fts MATCH ?
		 ORDER BY rank`,
		userID, ftsQuery(query),
	)
	if err != nil {
		// Return empty on FTS parse error.
		return nil, nil, nil
	}
	defer rows.Close()

	var ids []int64
	var results []ArchivalResult
	for rows.Next() {
		var id int64
		var content, tags string
		var createdAt time.Time
		if err := rows.Scan(&id, &content, &tags, &createdAt); err != nil {
			return nil, nil, fmt.Errorf("memory: archival fts scan: %w", err)
		}
		ids = append(ids, id)
		results = append(results, ArchivalResult{
			Content:   content,
			Tags:      splitTags(tags),
			CreatedAt: createdAt,
		})
	}
	return ids, results, rows.Err()
}

// archivalVectorSearch returns vector-similarity-ranked archival results for a user.
// Returns (rowIDs, results, error) where rowIDs are ordered by cosine similarity descending.
func (s *SQLiteStore) archivalVectorSearch(ctx context.Context, userID string, queryEmb []float32) ([]int64, []ArchivalResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, content, embedding, tags, created_at
		 FROM archival_memory
		 WHERE user_id = ? AND embedding IS NOT NULL`,
		userID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("memory: archival vector query: %w", err)
	}
	defer rows.Close()

	type vecRow struct {
		id        int64
		result    ArchivalResult
		similarity float32
	}

	var vrows []vecRow
	for rows.Next() {
		var id int64
		var content string
		var embBlob []byte
		var tags string
		var createdAt time.Time
		if err := rows.Scan(&id, &content, &embBlob, &tags, &createdAt); err != nil {
			return nil, nil, fmt.Errorf("memory: archival vector scan: %w", err)
		}
		sim := CosineSimilarity(queryEmb, bytesToFloat32s(embBlob))
		vrows = append(vrows, vecRow{
			id: id,
			result: ArchivalResult{
				Content:   content,
				Tags:      splitTags(tags),
				CreatedAt: createdAt,
			},
			similarity: sim,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Sort by similarity descending.
	sort.Slice(vrows, func(i, j int) bool {
		return vrows[i].similarity > vrows[j].similarity
	})

	ids := make([]int64, len(vrows))
	results := make([]ArchivalResult, len(vrows))
	for i, vr := range vrows {
		ids[i] = vr.id
		results[i] = vr.result
	}
	return ids, results, nil
}

// CountArchival returns the total number of archival entries for a user.
func (s *SQLiteStore) CountArchival(ctx context.Context, userID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM archival_memory WHERE user_id = ?`, userID,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("memory: archival count: %w", err)
	}
	return n, nil
}

// splitTags splits a comma-separated tag string into a trimmed slice.
func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}
	var result []string
	for _, t := range strings.Split(tags, ",") {
		if s := strings.TrimSpace(t); s != "" {
			result = append(result, s)
		}
	}
	return result
}
