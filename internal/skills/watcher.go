package skills

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// SkillWatcher watches one or more skills directories for file changes and
// triggers a reload callback with debouncing.
type SkillWatcher struct {
	dirs     []string
	debounce time.Duration
	onReload func([]Skill)
	watcher  *fsnotify.Watcher
	done     chan struct{}
	mu       sync.Mutex
}

// NewSkillWatcher creates a new watcher for the given skills directories.
// The onReload callback receives the freshly loaded skills slice whenever
// a file change is detected (after debouncing). Non-existent directories
// are skipped with a warning.
func NewSkillWatcher(dirs []string, debounce time.Duration, onReload func([]Skill)) (*SkillWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			slog.Warn("skill watcher: skipping non-existent dir", "dir", dir)
			continue
		}
		if err := w.Add(dir); err != nil {
			slog.Warn("skill watcher: failed to watch dir", "dir", dir, "error", err)
		}
	}

	if debounce <= 0 {
		debounce = 500 * time.Millisecond
	}

	return &SkillWatcher{
		dirs:     dirs,
		debounce: debounce,
		onReload: onReload,
		watcher:  w,
		done:     make(chan struct{}),
	}, nil
}

// Run starts watching for file changes. It blocks until Stop is called
// or the done channel is closed.
func (sw *SkillWatcher) Run() {
	debounceTimer := time.NewTimer(0)
	<-debounceTimer.C // drain initial fire

	for {
		select {
		case <-sw.done:
			sw.watcher.Close()
			return

		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}

			// Only care about writes, creates, removes, and renames
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			debounceTimer.Reset(sw.debounce)

		case <-debounceTimer.C:
			skills, err := LoadSkillsFromMultipleSources(sw.dirs)
			if err != nil {
				slog.Warn("skill hot-reload failed", "dirs", sw.dirs, "err", err)
				continue
			}

			slog.Info("skills hot-reloaded", "dirs", sw.dirs, "count", len(skills))

			sw.mu.Lock()
			cb := sw.onReload
			sw.mu.Unlock()

			if cb != nil {
				cb(skills)
			}

		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("skill watcher error", "err", err)
		}
	}
}

// Stop closes the watcher and stops monitoring.
func (sw *SkillWatcher) Stop() {
	select {
	case <-sw.done:
		// Already stopped
	default:
		close(sw.done)
	}
}
