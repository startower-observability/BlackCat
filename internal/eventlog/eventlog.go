package eventlog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultLogPath is the default location for event logs.
const DefaultLogPath = ".blackcat/events.log"

// EventLogger writes structured JSON Lines events to a file.
// It is safe for concurrent use.
type EventLogger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	logPath string
}

// New creates a new EventLogger that appends to the given file path.
// If logPath is empty, it defaults to ~/.blackcat/events.log.
// Parent directories are created automatically.
func New(logPath string) (*EventLogger, error) {
	if logPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("eventlog: user home dir: %w", err)
		}
		logPath = filepath.Join(home, DefaultLogPath)
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("eventlog: create dir: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("eventlog: open file: %w", err)
	}

	slog.Debug("eventlog: opened event log", "path", logPath)

	return &EventLogger{
		file:    f,
		encoder: json.NewEncoder(f),
		logPath: logPath,
	}, nil
}

// LogEvent writes a single EventRecord as a JSON line to the log file.
// If the record has a zero Timestamp, it is set to time.Now().
func (l *EventLogger) LogEvent(_ context.Context, record EventRecord) error {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return fmt.Errorf("eventlog: logger is closed")
	}

	if err := l.encoder.Encode(record); err != nil {
		return fmt.Errorf("eventlog: encode event: %w", err)
	}

	return nil
}

// Close flushes and closes the underlying log file.
func (l *EventLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}

	err := l.file.Close()
	l.file = nil
	l.encoder = nil
	if err != nil {
		return fmt.Errorf("eventlog: close file: %w", err)
	}
	return nil
}

// TODO: Implement daily rotation.
// When the date changes, close the current file and open events.YYYY-MM-DD.log.
// Keep the last 7 days of rotated files, deleting older ones.
// This is deferred to a follow-up task due to complexity (file naming,
// cleanup goroutine, date tracking, etc.).
