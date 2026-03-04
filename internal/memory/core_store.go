package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	maxCoreEntries  = 20
	maxCoreValueLen = 500
)

// CoreStore is a per-user key-value store backed by SQLite.
// It provides "core memory" — persistent facts about a user
// that should be injected into every prompt.
type CoreStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewCoreStore creates a CoreStore using an existing SQLite DB connection.
// It auto-creates the core_memory table if it does not exist.
func NewCoreStore(db *sql.DB) *CoreStore {
	_ = createCoreSchema(db)
	return &CoreStore{db: db}
}

func createCoreSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS core_memory (
		user_id    TEXT NOT NULL,
		key        TEXT NOT NULL,
		value      TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, key)
	);`
	_, err := db.Exec(schema)
	return err
}

// Get returns the value for a key, or ("", nil) if not found.
func (s *CoreStore) Get(ctx context.Context, userID, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM core_memory WHERE user_id = ? AND key = ?`,
		userID, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("core_memory: get: %w", err)
	}
	return value, nil
}

// Set upserts a key-value pair for a user.
// Values longer than 500 chars are silently truncated.
// If the user already has 20 entries and the key is new,
// the oldest entry (by updated_at) is deleted first.
func (s *CoreStore) Set(ctx context.Context, userID, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Truncate value if too long.
	if len(value) > maxCoreValueLen {
		value = value[:maxCoreValueLen]
	}

	// Check if this is a new key (not an update).
	var exists int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM core_memory WHERE user_id = ? AND key = ?`,
		userID, key,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("core_memory: check exists: %w", err)
	}

	// If key is new, enforce max entries limit.
	if exists == 0 {
		var count int
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM core_memory WHERE user_id = ?`,
			userID,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("core_memory: count: %w", err)
		}
		if count >= maxCoreEntries {
			_, err := s.db.ExecContext(ctx,
				`DELETE FROM core_memory WHERE user_id = ? AND key = (
					SELECT key FROM core_memory WHERE user_id = ? ORDER BY updated_at ASC LIMIT 1
				)`,
				userID, userID,
			)
			if err != nil {
				return fmt.Errorf("core_memory: evict oldest: %w", err)
			}
		}
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO core_memory (user_id, key, value, updated_at)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		userID, key, value,
	)
	if err != nil {
		return fmt.Errorf("core_memory: set: %w", err)
	}
	return nil
}

// Delete removes a key-value pair for a user.
func (s *CoreStore) Delete(ctx context.Context, userID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`DELETE FROM core_memory WHERE user_id = ? AND key = ?`,
		userID, key,
	)
	if err != nil {
		return fmt.Errorf("core_memory: delete: %w", err)
	}
	return nil
}

// GetAll returns all key-value pairs for a user.
func (s *CoreStore) GetAll(ctx context.Context, userID string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM core_memory WHERE user_id = ? ORDER BY key`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("core_memory: get all: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("core_memory: scan: %w", err)
		}
		result[k] = v
	}
	return result, rows.Err()
}

// FormatForPrompt renders all core memory entries as markdown.
// Returns "" if the user has no entries.
func (s *CoreStore) FormatForPrompt(ctx context.Context, userID string) (string, error) {
	pairs, err := s.GetAll(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(pairs) == 0 {
		return "", nil
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("### Core Memory\n")
	for _, k := range keys {
		sb.WriteString("- ")
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(pairs[k])
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
