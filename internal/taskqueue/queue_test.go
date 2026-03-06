//go:build cgo

package taskqueue

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestQueue creates a TaskQueue backed by a temp SQLite file.
func newTestQueue(t *testing.T) *TaskQueue {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tasks.db")

	q, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() {
		// Shutdown only if Start was called (cancel != nil).
		if q.cancel != nil {
			q.Shutdown()
		} else {
			q.db.Close()
		}
	})
	return q
}

func TestEnqueue(t *testing.T) {
	q := newTestQueue(t)

	id, err := q.Enqueue(Task{
		TaskType:    "test_task",
		Payload:     `{"key":"value"}`,
		RecipientID: "user123",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected ID > 0, got %d", id)
	}

	// Verify status is pending in DB.
	task, err := q.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask(%d) error: %v", id, err)
	}
	if task.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, task.Status)
	}
}

func TestGetTask(t *testing.T) {
	q := newTestQueue(t)

	id, err := q.Enqueue(Task{
		TaskType:    "opencode_task",
		Payload:     `{"prompt":"hello"}`,
		RecipientID: "wa_12345",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	task, err := q.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask() error: %v", err)
	}

	if task.ID != id {
		t.Errorf("ID: got %d, want %d", task.ID, id)
	}
	if task.TaskType != "opencode_task" {
		t.Errorf("TaskType: got %q, want %q", task.TaskType, "opencode_task")
	}
	if task.Status != StatusPending {
		t.Errorf("Status: got %q, want %q", task.Status, StatusPending)
	}
	if task.Payload != `{"prompt":"hello"}` {
		t.Errorf("Payload: got %q, want %q", task.Payload, `{"prompt":"hello"}`)
	}
	if task.RecipientID != "wa_12345" {
		t.Errorf("RecipientID: got %q, want %q", task.RecipientID, "wa_12345")
	}
	if task.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if task.CompletedAt != nil {
		t.Error("CompletedAt should be nil for pending task")
	}
}

func TestGetTask_NotFound(t *testing.T) {
	q := newTestQueue(t)

	_, err := q.GetTask(9999)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestListTasks(t *testing.T) {
	q := newTestQueue(t)

	for i := range 3 {
		_, err := q.Enqueue(Task{
			TaskType: "batch_task",
			Payload:  `{"i":` + string(rune('0'+i)) + `}`,
		})
		if err != nil {
			t.Fatalf("Enqueue() error: %v", err)
		}
	}

	tasks, err := q.ListTasks(StatusPending)
	if err != nil {
		t.Fatalf("ListTasks() error: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("expected 3 pending tasks, got %d", len(tasks))
	}

	// No completed tasks should exist.
	completed, err := q.ListTasks(StatusCompleted)
	if err != nil {
		t.Fatalf("ListTasks(completed) error: %v", err)
	}
	if len(completed) != 0 {
		t.Errorf("expected 0 completed tasks, got %d", len(completed))
	}
}

func TestWorker_Execute(t *testing.T) {
	q := newTestQueue(t)

	// Register a handler that returns a known result.
	q.RegisterHandler("echo_task", func(ctx context.Context, payload string) (string, error) {
		return "echoed: " + payload, nil
	})

	id, err := q.Enqueue(Task{
		TaskType: "echo_task",
		Payload:  "hello world",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	// Wait up to 3 seconds for the task to complete.
	deadline := time.Now().Add(3 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusCompleted || task.Status == StatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != StatusCompleted {
		t.Fatalf("expected status %q, got %q (error: %s)", StatusCompleted, task.Status, task.Error)
	}
	if task.Result != "echoed: hello world" {
		t.Errorf("expected result %q, got %q", "echoed: hello world", task.Result)
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set for completed task")
	}
}

func TestWorker_ExecuteFailure(t *testing.T) {
	q := newTestQueue(t)

	q.RegisterHandler("fail_task", func(ctx context.Context, payload string) (string, error) {
		return "", os.ErrPermission
	})

	id, err := q.Enqueue(Task{
		TaskType: "fail_task",
		Payload:  "data",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	deadline := time.Now().Add(3 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusCompleted || task.Status == StatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q", StatusFailed, task.Status)
	}
	if task.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestCleanup(t *testing.T) {
	q := newTestQueue(t)

	// Manually insert a completed task with completed_at 8 days ago.
	eightDaysAgo := time.Now().UTC().Add(-8 * 24 * time.Hour)
	q.mu.Lock()
	_, err := q.db.Exec(
		`INSERT INTO tasks (task_type, status, payload, result, error, recipient_id,
		                     created_at, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, '', '', ?, ?, ?)`,
		"old_task", StatusCompleted, "{}", "done",
		eightDaysAgo, eightDaysAgo, eightDaysAgo,
	)
	q.mu.Unlock()
	if err != nil {
		t.Fatalf("insert old task: %v", err)
	}

	// Also insert a recent completed task that should NOT be deleted.
	recentTime := time.Now().UTC().Add(-1 * time.Hour)
	q.mu.Lock()
	_, err = q.db.Exec(
		`INSERT INTO tasks (task_type, status, payload, result, error, recipient_id,
		                     created_at, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, '', '', ?, ?, ?)`,
		"recent_task", StatusCompleted, "{}", "done",
		recentTime, recentTime, recentTime,
	)
	q.mu.Unlock()
	if err != nil {
		t.Fatalf("insert recent task: %v", err)
	}

	// Run cleanup.
	cleanupOldTasks(q.db)

	// The old task should be gone.
	tasks, err := q.ListTasks(StatusCompleted)
	if err != nil {
		t.Fatalf("ListTasks() error: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 remaining completed task, got %d", len(tasks))
	}
	if tasks[0].TaskType != "recent_task" {
		t.Errorf("expected recent_task to remain, got %q", tasks[0].TaskType)
	}
}

func TestRecoverInterruptedTasks(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	// Insert a pending task.
	id1, err := q.Enqueue(Task{
		TaskType:    "opencode_task",
		Payload:     `{"prompt":"build something"}`,
		RecipientID: "wa_111",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	// Insert a task and manually set it to in_progress (simulating a running task).
	id2, err := q.Enqueue(Task{
		TaskType:    "opencode_task",
		Payload:     `{"prompt":"deploy"}`,
		RecipientID: "wa_222",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}
	q.updateStatus(id2, StatusInProgress, "", "")

	// Insert a completed task (should NOT be recovered).
	id3, err := q.Enqueue(Task{
		TaskType: "opencode_task",
		Payload:  `{"prompt":"done"}`,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}
	q.updateStatus(id3, StatusCompleted, "all good", "")

	// Recover interrupted tasks.
	recovered := q.RecoverInterruptedTasks(ctx)

	if len(recovered) != 2 {
		t.Fatalf("expected 2 recovered tasks, got %d", len(recovered))
	}

	// All recovered tasks should now be failed.
	for _, task := range recovered {
		if task.Status != StatusFailed {
			t.Errorf("task %d: expected status %q, got %q", task.ID, StatusFailed, task.Status)
		}
		if task.Error != "interrupted by daemon restart" {
			t.Errorf("task %d: expected error %q, got %q", task.ID, "interrupted by daemon restart", task.Error)
		}
		if task.CompletedAt == nil {
			t.Errorf("task %d: expected CompletedAt to be set", task.ID)
		}
	}

	// Verify the recovered task IDs match what we expect.
	recoveredIDs := map[int64]bool{}
	for _, task := range recovered {
		recoveredIDs[task.ID] = true
	}
	if !recoveredIDs[id1] {
		t.Errorf("expected pending task %d to be recovered", id1)
	}
	if !recoveredIDs[id2] {
		t.Errorf("expected in_progress task %d to be recovered", id2)
	}
	if recoveredIDs[id3] {
		t.Errorf("completed task %d should NOT be recovered", id3)
	}

	// Verify DB state: no pending or in_progress tasks should remain.
	pendingTasks, err := q.ListTasks(StatusPending)
	if err != nil {
		t.Fatalf("ListTasks(pending) error: %v", err)
	}
	if len(pendingTasks) != 0 {
		t.Errorf("expected 0 pending tasks after recovery, got %d", len(pendingTasks))
	}

	inProgressTasks, err := q.ListTasks(StatusInProgress)
	if err != nil {
		t.Fatalf("ListTasks(in_progress) error: %v", err)
	}
	if len(inProgressTasks) != 0 {
		t.Errorf("expected 0 in_progress tasks after recovery, got %d", len(inProgressTasks))
	}
}

func TestRecoverInterruptedTasks_Empty(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	// No tasks at all — should return nil/empty.
	recovered := q.RecoverInterruptedTasks(ctx)
	if len(recovered) != 0 {
		t.Errorf("expected 0 recovered tasks, got %d", len(recovered))
	}
}
