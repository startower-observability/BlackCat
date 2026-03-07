package session

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SessionState describes the lifecycle state of a session.
type SessionState string

const (
	// SessionStateActive means the session is valid and usable.
	SessionStateActive SessionState = "active"
	// SessionStateStale means the session exists but LastActivity exceeded 24h with no ExpiresAt.
	SessionStateStale SessionState = "stale"
	// SessionStateExpired means the session's ExpiresAt has passed.
	SessionStateExpired SessionState = "expired"
	// SessionStateMissing means no session file was found for the key.
	SessionStateMissing SessionState = "missing"
)

// SessionLookup pairs a session with its resolved lifecycle state.
// Session is non-nil for active, stale, and expired states (only nil for missing).
type SessionLookup struct {
	Session *Session
	State   SessionState
}

// GetWithState retrieves a session by key and returns it with its lifecycle state.
// Unlike Get(), this method returns the session even when stale or expired,
// allowing callers (e.g. rollover reflection) to read the prior transcript.
// Returns SessionStateMissing with nil Session when no file exists.
func (fs *FileStore) GetWithState(key SessionKey) (*SessionLookup, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.filePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SessionLookup{Session: nil, State: SessionStateMissing}, nil
		}
		return nil, fmt.Errorf("session: read %s: %w", path, err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("session: unmarshal %s: %w", path, err)
	}

	// Check if session is expired (ExpiresAt set and past)
	if !sess.ExpiresAt.IsZero() && time.Now().After(sess.ExpiresAt) {
		return &SessionLookup{Session: &sess, State: SessionStateExpired}, nil
	}

	// Fallback: if ExpiresAt is zero but LastActivity older than 24h, treat as stale
	if sess.ExpiresAt.IsZero() && !sess.LastActivity.IsZero() && time.Since(sess.LastActivity) > 24*time.Hour {
		return &SessionLookup{Session: &sess, State: SessionStateStale}, nil
	}

	return &SessionLookup{Session: &sess, State: SessionStateActive}, nil
}
