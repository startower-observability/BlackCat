package taskqueue

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/startower-observability/blackcat/internal/eventlog"
)

// TaskQueue is a persistent background task queue backed by SQLite.
type TaskQueue struct {
	db       *sql.DB
	mu       sync.Mutex // guards DB writes
	work     chan *Task
	handlers map[string]TaskHandler
	logger   *eventlog.EventLogger // optional event logger; nil = no logging

	notificationSender NotificationSender // optional; sends completion/failure messages

	numWorkers int
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// New creates a TaskQueue with its own SQLite database at dbPath.
func New(dbPath string) (*TaskQueue, error) {
	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}

	numWorkers := runtime.NumCPU() / 2
	if numWorkers < 1 {
		numWorkers = 1
	}
	if numWorkers > 4 {
		numWorkers = 4
	}

	return &TaskQueue{
		db:         db,
		work:       make(chan *Task, numWorkers*2),
		handlers:   make(map[string]TaskHandler),
		numWorkers: numWorkers,
	}, nil
}

// NewWithEventLog creates a TaskQueue with event lifecycle logging enabled.
func NewWithEventLog(dbPath string, logger *eventlog.EventLogger) (*TaskQueue, error) {
	q, err := New(dbPath)
	if err != nil {
		return nil, err
	}
	q.logger = logger
	return q, nil
}

// SetEventLogger attaches an optional EventLogger for task lifecycle events.
// Pass nil to disable logging. Must be called before Start.
func (q *TaskQueue) SetEventLogger(logger *eventlog.EventLogger) {
	q.logger = logger
}

// RegisterHandler registers a handler function for a given task type.
// Must be called before Start.
func (q *TaskQueue) RegisterHandler(taskType string, fn TaskHandler) {
	q.handlers[taskType] = fn
}

// handler returns the registered handler for a task type (thread-safe read
// because handlers are registered before Start and never mutated after).
func (q *TaskQueue) handler(taskType string) (TaskHandler, bool) {
	fn, ok := q.handlers[taskType]
	return fn, ok
}

// Enqueue inserts a new task into the database and returns its ID.
func (q *TaskQueue) Enqueue(task Task) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Apply defaults for retry/timeout if not set by caller.
	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}
	if task.TimeoutSecs == 0 {
		task.TimeoutSecs = 1800
	}

	now := time.Now().UTC()
	res, err := q.db.Exec(
		`INSERT INTO tasks (task_type, status, payload, result, error, recipient_id,
		                     retry_count, max_retries, timeout_secs, created_at, updated_at)
		 VALUES (?, ?, ?, '', '', ?, ?, ?, ?, ?, ?)`,
		task.TaskType, StatusPending, task.Payload, task.RecipientID,
		task.RetryCount, task.MaxRetries, task.TimeoutSecs, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("taskqueue: enqueue: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("taskqueue: last insert id: %w", err)
	}

	// Best-effort lifecycle event logging.
	if q.logger != nil {
		_ = q.logger.LogEvent(context.Background(), eventlog.EventRecord{
			SessionID: fmt.Sprintf("task-%d", id),
			EventType: eventlog.EventTypeTaskQueued,
			Extra: map[string]any{
				"task_type":    task.TaskType,
				"recipient_id": task.RecipientID,
			},
		})
	}

	return id, nil
}

// GetTask retrieves a single task by ID.
func (q *TaskQueue) GetTask(id int64) (*Task, error) {
	row := q.db.QueryRow(
		`SELECT id, task_type, status, payload, result, error, recipient_id,
		        retry_count, max_retries, timeout_secs,
		        created_at, updated_at, completed_at
		 FROM tasks WHERE id = ?`, id,
	)
	return scanTask(row)
}

// ListTasks returns tasks filtered by status, limited to 50.
func (q *TaskQueue) ListTasks(status string) ([]Task, error) {
	rows, err := q.db.Query(
		`SELECT id, task_type, status, payload, result, error, recipient_id,
		        retry_count, max_retries, timeout_secs,
		        created_at, updated_at, completed_at
		 FROM tasks WHERE status = ?
		 ORDER BY created_at DESC LIMIT 50`, status,
	)
	if err != nil {
		return nil, fmt.Errorf("taskqueue: list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var completedAt sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.TaskType, &t.Status, &t.Payload, &t.Result, &t.Error,
			&t.RecipientID, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs,
			&t.CreatedAt, &t.UpdatedAt, &completedAt,
		); err != nil {
			return nil, fmt.Errorf("taskqueue: scan task: %w", err)
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Start launches the worker pool, dispatcher, and cleanup goroutines.
func (q *TaskQueue) Start(ctx context.Context) {
	ctx, q.cancel = context.WithCancel(ctx)

	// Start workers.
	for i := range q.numWorkers {
		w := &worker{id: i, queue: q, wg: &q.wg}
		q.wg.Add(1)
		go w.run(ctx)
	}

	// Start dispatcher — polls DB for pending tasks every 2 seconds.
	q.wg.Add(1)
	go q.dispatch(ctx)

	// Start cleanup — removes old completed/failed tasks every hour.
	q.wg.Add(1)
	go q.cleanupLoop(ctx)

	slog.Info("taskqueue: started", "workers", q.numWorkers)
}

// Shutdown cancels the context, drains work channel, and waits for workers.
func (q *TaskQueue) Shutdown() {
	if q.cancel != nil {
		q.cancel()
	}
	// Close work channel so workers drain remaining items and exit.
	close(q.work)
	q.wg.Wait()

	if q.db != nil {
		q.db.Close()
	}
	slog.Info("taskqueue: shut down")
}

// dispatch polls the DB every 2 seconds for pending tasks and pushes them
// to the work channel for workers to pick up.
func (q *TaskQueue) dispatch(ctx context.Context) {
	defer q.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.dispatchPending(ctx)
		}
	}
}

// dispatchPending fetches pending tasks from the DB and enqueues them.
func (q *TaskQueue) dispatchPending(ctx context.Context) {
	rows, err := q.db.Query(
		`SELECT id, task_type, status, payload, result, error, recipient_id,
		        retry_count, max_retries, timeout_secs,
		        created_at, updated_at, completed_at
		 FROM tasks WHERE status = ?
		 ORDER BY created_at ASC LIMIT ?`, StatusPending, q.numWorkers*2,
	)
	if err != nil {
		slog.Warn("taskqueue: dispatch query failed", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var t Task
		var completedAt sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.TaskType, &t.Status, &t.Payload, &t.Result, &t.Error,
			&t.RecipientID, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs,
			&t.CreatedAt, &t.UpdatedAt, &completedAt,
		); err != nil {
			slog.Warn("taskqueue: dispatch scan failed", "error", err)
			continue
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}

		// Atomically claim the task so other dispatchers (or restarts) don't pick it up.
		q.mu.Lock()
		res, err := q.db.Exec(
			`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
			StatusInProgress, time.Now().UTC(), t.ID, StatusPending,
		)
		q.mu.Unlock()
		if err != nil {
			slog.Warn("taskqueue: dispatch claim failed", "task_id", t.ID, "error", err)
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			// Another goroutine already claimed this task.
			continue
		}

		select {
		case q.work <- &t:
		case <-ctx.Done():
			return
		}
	}
}

// cleanupLoop runs cleanupOldTasks every hour.
func (q *TaskQueue) cleanupLoop(ctx context.Context) {
	defer q.wg.Done()

	// Run once on start.
	cleanupOldTasks(q.db)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupOldTasks(q.db)
		}
	}
}

// RecoverInterruptedTasks finds tasks that were pending or in_progress when the
// daemon was last stopped, marks them as failed with "interrupted by daemon restart",
// logs error events, and sends notifications. It returns the list of recovered tasks.
// Call this once after Start to recover state from a previous run.
func (q *TaskQueue) RecoverInterruptedTasks(ctx context.Context) []Task {
	rows, err := q.db.Query(
		`SELECT id, task_type, status, payload, result, error, recipient_id,
		        retry_count, max_retries, timeout_secs,
		        created_at, updated_at, completed_at
		 FROM tasks WHERE status IN (?, ?)
		 ORDER BY created_at ASC`,
		StatusPending, StatusInProgress,
	)
	if err != nil {
		slog.Warn("taskqueue: recover query failed", "error", err)
		return nil
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var completedAt sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.TaskType, &t.Status, &t.Payload, &t.Result, &t.Error,
			&t.RecipientID, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs,
			&t.CreatedAt, &t.UpdatedAt, &completedAt,
		); err != nil {
			slog.Warn("taskqueue: recover scan failed", "error", err)
			continue
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		slog.Warn("taskqueue: recover rows iteration error", "error", err)
	}

	const errMsg = "interrupted by daemon restart"
	now := time.Now().UTC()

	for i := range tasks {
		t := &tasks[i]
		prevStatus := t.Status

		// Mark as failed in DB.
		q.mu.Lock()
		_, dbErr := q.db.Exec(
			`UPDATE tasks SET status = ?, error = ?, updated_at = ?, completed_at = ? WHERE id = ?`,
			StatusFailed, errMsg, now, now, t.ID,
		)
		q.mu.Unlock()
		if dbErr != nil {
			slog.Error("taskqueue: recover update failed", "task_id", t.ID, "error", dbErr)
			continue
		}

		t.Status = StatusFailed
		t.Error = errMsg
		t.CompletedAt = &now

		// Log error event (best-effort).
		if q.logger != nil {
			_ = q.logger.LogEvent(ctx, eventlog.EventRecord{
				SessionID: fmt.Sprintf("task-%d", t.ID),
				EventType: eventlog.EventTypeError,
				Error:     errMsg,
				Extra: map[string]any{
					"task_type":    t.TaskType,
					"recipient_id": t.RecipientID,
					"recovery":     true,
				},
			})
		}

		// Notify recipient (best-effort).
		if q.notificationSender != nil && t.RecipientID != "" {
			notifMsg := fmt.Sprintf("⚠️ Task %d (%s) was interrupted by a restart. Please re-submit if needed.", t.ID, t.TaskType)
			go func(recipientID string) {
				if sendErr := q.notificationSender.Send(context.Background(), recipientID, notifMsg); sendErr != nil {
					slog.Warn("taskqueue: recover notification failed", "task_id", t.ID, "error", sendErr)
				}
			}(t.RecipientID)
		}

		slog.Info("taskqueue: recovered interrupted task", "task_id", t.ID, "task_type", t.TaskType, "previous_status", prevStatus)
	}

	return tasks
}

// scanTask scans a single task row.
func scanTask(row *sql.Row) (*Task, error) {
	var t Task
	var completedAt sql.NullTime
	if err := row.Scan(
		&t.ID, &t.TaskType, &t.Status, &t.Payload, &t.Result, &t.Error,
		&t.RecipientID, &t.RetryCount, &t.MaxRetries, &t.TimeoutSecs,
		&t.CreatedAt, &t.UpdatedAt, &completedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("taskqueue: task not found")
		}
		return nil, fmt.Errorf("taskqueue: scan task: %w", err)
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return &t, nil
}
