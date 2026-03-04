package memory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// MigrateFromMemoryMD reads a MEMORY.md file and inserts each non-empty line
// as an archival memory entry. It is idempotent: if the migration has already
// been run (tracked via the metadata table), it returns (0, nil) immediately.
func MigrateFromMemoryMD(ctx context.Context, path string, store *SQLiteStore, embed *EmbeddingClient, userID string) (int, error) {
	if store == nil {
		return 0, nil
	}

	const migrationKey = "memory_md_migrated"

	// Check if already migrated
	var val string
	row := store.db.QueryRowContext(ctx, `SELECT value FROM metadata WHERE key = ?`, migrationKey)
	if err := row.Scan(&val); err == nil && val == "1" {
		return 0, nil
	}

	// Read MEMORY.md
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Mark as migrated (nothing to migrate)
			_, _ = store.db.ExecContext(ctx, `INSERT OR REPLACE INTO metadata(key, value) VALUES (?, ?)`, migrationKey, "1")
			return 0, nil
		}
		return 0, fmt.Errorf("MigrateFromMemoryMD: open %s: %w", path, err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var embedding []float32
		if embed != nil {
			if emb, embedErr := embed.EmbedSingle(ctx, line); embedErr == nil {
				embedding = emb
			}
		}

		if err := store.InsertArchival(ctx, userID, line, []string{"migrated", "memory_md"}, embedding); err != nil {
			return count, fmt.Errorf("MigrateFromMemoryMD: insert line: %w", err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("MigrateFromMemoryMD: scan: %w", err)
	}

	// Mark as migrated
	_, _ = store.db.ExecContext(ctx, `INSERT OR REPLACE INTO metadata(key, value) VALUES (?, ?)`, migrationKey, "1")

	return count, nil
}
