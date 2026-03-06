package taskqueue

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/startower-observability/blackcat/internal/eventlog"
)

// worker pulls tasks from the work channel and executes the registered handler.
type worker struct {
	id    int
	queue *TaskQueue
	wg    *sync.WaitGroup
}

// run is the main loop for a worker goroutine. It reads tasks from the work
// channel until the channel is closed or the context is cancelled.
func (w *worker) run(ctx context.Context) {
	defer w.wg.Done()

	slog.Debug("taskqueue: worker started", "worker_id", w.id)
	for {
		select {
		case <-ctx.Done():
			slog.Debug("taskqueue: worker stopping (ctx cancelled)", "worker_id", w.id)
			return
		case task, ok := <-w.queue.work:
			if !ok {
				slog.Debug("taskqueue: worker stopping (channel closed)", "worker_id", w.id)
				return
			}
			w.execute(ctx, task)
		}
	}
}

// execute runs the handler for the task's type and updates the DB.
func (w *worker) execute(ctx context.Context, task *Task) {
	logger := slog.With("worker_id", w.id, "task_id", task.ID, "task_type", task.TaskType)
	logger.Info("taskqueue: executing task", "retry", task.RetryCount, "max_retries", task.MaxRetries)

	handler, ok := w.queue.handler(task.TaskType)
	if !ok {
		logger.Error("taskqueue: no handler registered")
		w.failPermanent(task, fmt.Errorf("no handler registered for task type: %s", task.TaskType))
		return
	}

	// Mark in-progress.
	w.queue.updateStatus(task.ID, StatusInProgress, "", "")

	sessionID := fmt.Sprintf("task-%d", task.ID)
	startTime := time.Now()

	// Log task start event (best-effort).
	if w.queue.logger != nil {
		_ = w.queue.logger.LogEvent(ctx, eventlog.EventRecord{
			SessionID: sessionID,
			EventType: eventlog.EventTypeToolCall,
			ToolName:  task.TaskType,
		})
	}

	// Apply per-task timeout.
	timeoutSecs := task.TimeoutSecs
	if timeoutSecs <= 0 {
		timeoutSecs = 1800 // default 30 minutes
	}
	taskCtx, taskCancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer taskCancel()

	// Execute handler with panic recovery.
	result, err := w.safeExecute(taskCtx, handler, task.Payload)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		logger.Warn("taskqueue: task failed", "error", err, "duration_ms", durationMs)

		// Log failure events (best-effort).
		if w.queue.logger != nil {
			_ = w.queue.logger.LogEvent(ctx, eventlog.EventRecord{
				SessionID:  sessionID,
				EventType:  eventlog.EventTypeToolResult,
				ToolName:   task.TaskType,
				Success:    false,
				DurationMs: durationMs,
				Error:      err.Error(),
			})
			_ = w.queue.logger.LogEvent(ctx, eventlog.EventRecord{
				SessionID: sessionID,
				EventType: eventlog.EventTypeError,
				Error:     err.Error(),
			})
		}

		w.handleFailure(task, err)
		return
	}

	w.complete(task, result)
	logger.Info("taskqueue: task completed", "duration_ms", durationMs)

	// Log success event (best-effort).
	if w.queue.logger != nil {
		_ = w.queue.logger.LogEvent(ctx, eventlog.EventRecord{
			SessionID:  sessionID,
			EventType:  eventlog.EventTypeToolResult,
			ToolName:   task.TaskType,
			Success:    true,
			DurationMs: durationMs,
		})
	}
}

// safeExecute runs the handler with panic recovery.
// Panics are caught and returned as permanent errors.
func (w *worker) safeExecute(ctx context.Context, handler TaskHandler, payload string) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error occurred (panic: %v)", r)
		}
	}()
	return handler(ctx, payload)
}

// handleFailure classifies the error and either retries (transient) or fails permanently.
func (w *worker) handleFailure(task *Task, err error) {
	kind := ClassifyError(err)

	logger := slog.With("worker_id", w.id, "task_id", task.ID, "error_kind", kind.String(),
		"retry_count", task.RetryCount, "max_retries", task.MaxRetries)

	switch kind {
	case ErrorKindTransient:
		if task.RetryCount < task.MaxRetries {
			w.retry(task, err)
			return
		}
		logger.Warn("taskqueue: max retries exhausted for transient error")
		w.failWithKind(task, kind, err)

	case ErrorKindPermanent:
		logger.Warn("taskqueue: permanent error, no retry")
		w.failWithKind(task, kind, err)

	case ErrorKindUserActionNeeded:
		logger.Warn("taskqueue: user action needed, no retry")
		w.failWithKind(task, kind, err)
	}
}

// retry increments the task's retry count, resets it to pending status,
// and schedules it for re-execution after exponential backoff.
func (w *worker) retry(task *Task, err error) {
	task.RetryCount++
	backoff := time.Duration(1<<uint(task.RetryCount)) * time.Second * 2 // 2s, 4s, 8s, 16s, ...

	logger := slog.With("worker_id", w.id, "task_id", task.ID,
		"retry_count", task.RetryCount, "backoff", backoff)
	logger.Info("taskqueue: scheduling retry")

	// Update DB: increment retry_count and reset to pending.
	w.queue.mu.Lock()
	now := time.Now().UTC()
	_, dbErr := w.queue.db.Exec(
		`UPDATE tasks SET status = ?, retry_count = ?, error = ?, updated_at = ? WHERE id = ?`,
		StatusPending, task.RetryCount, err.Error(), now, task.ID,
	)
	w.queue.mu.Unlock()

	if dbErr != nil {
		slog.Error("taskqueue: failed to schedule retry", "task_id", task.ID, "error", dbErr)
	}
	// The dispatcher will pick it up on its next poll cycle (every 2s).
	// The backoff is effective because we sleep here before returning,
	// giving the system breathing room.
	time.Sleep(backoff)
}

// failWithKind marks a task as permanently failed and notifies with error-kind-specific messaging.
func (w *worker) failWithKind(task *Task, kind ErrorKind, err error) {
	now := time.Now().UTC()
	errMsg := ErrorKindMessage(kind, err)

	w.queue.mu.Lock()
	defer w.queue.mu.Unlock()

	_, dbErr := w.queue.db.Exec(
		`UPDATE tasks SET status = ?, error = ?, updated_at = ?, completed_at = ? WHERE id = ?`,
		StatusFailed, errMsg, now, now, task.ID,
	)
	if dbErr != nil {
		slog.Error("taskqueue: failed to mark task failed", "task_id", task.ID, "error", dbErr)
		return
	}

	task.Status = StatusFailed
	task.Error = errMsg
	task.CompletedAt = &now

	if w.queue.notificationSender != nil && task.RecipientID != "" {
		go func() {
			if sendErr := w.queue.notificationSender.Send(context.Background(), task.RecipientID, errMsg); sendErr != nil {
				slog.Warn("taskqueue: failed to send failure notification", "task_id", task.ID, "error", sendErr)
			}
		}()
	}
}

// failPermanent is a convenience for immediate permanent failures (e.g. no handler).
func (w *worker) failPermanent(task *Task, err error) {
	w.failWithKind(task, ErrorKindPermanent, err)
}

// complete marks a task as completed in the DB and sends a notification
// if a NotificationSender is registered and the task has a RecipientID.
func (w *worker) complete(task *Task, result string) {
	now := time.Now().UTC()
	w.queue.mu.Lock()
	defer w.queue.mu.Unlock()

	_, err := w.queue.db.Exec(
		`UPDATE tasks SET status = ?, result = ?, updated_at = ?, completed_at = ? WHERE id = ?`,
		StatusCompleted, result, now, now, task.ID,
	)
	if err != nil {
		slog.Error("taskqueue: failed to mark task completed", "task_id", task.ID, "error", err)
		return
	}

	// Populate task fields so the formatter has full context.
	task.Status = StatusCompleted
	task.Result = result
	task.CompletedAt = &now

	if w.queue.notificationSender != nil && task.RecipientID != "" {
		msg := FormatCompletion(task)
		go func() {
			if err := w.queue.notificationSender.Send(context.Background(), task.RecipientID, msg); err != nil {
				slog.Warn("taskqueue: failed to send completion notification", "task_id", task.ID, "error", err)
			}
		}()
	}
}

// updateStatus is a helper on TaskQueue used by workers to set in_progress.
func (q *TaskQueue) updateStatus(id int64, status, result, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now().UTC()
	var completedAt sql.NullTime
	if status == StatusCompleted || status == StatusFailed {
		completedAt = sql.NullTime{Time: now, Valid: true}
	}

	_, err := q.db.Exec(
		`UPDATE tasks SET status = ?, result = ?, error = ?, updated_at = ?, completed_at = ? WHERE id = ?`,
		status, result, errMsg, now, completedAt, id,
	)
	if err != nil {
		slog.Error("taskqueue: update status failed", "task_id", id, "error", err)
	}
}
