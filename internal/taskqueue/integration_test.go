//go:build cgo
// +build cgo

package taskqueue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// e2eMockSender captures notification calls for E2E test assertions.
// Separate from mockSender in notification_test.go to avoid redefinition.
type e2eMockSender struct {
	mu         sync.Mutex
	messages   []string
	recipients []string
}

func (m *e2eMockSender) Send(_ context.Context, recipientID, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
	m.recipients = append(m.recipients, recipientID)
	return nil
}

func (m *e2eMockSender) getMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	dst := make([]string, len(m.messages))
	copy(dst, m.messages)
	return dst
}

func (m *e2eMockSender) getRecipients() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	dst := make([]string, len(m.recipients))
	copy(dst, m.recipients)
	return dst
}

// pollTask polls GetTask until the status matches one of the terminal statuses
// or the timeout expires. Returns the last-fetched task.
func pollTask(t *testing.T, q *TaskQueue, id int64, timeout time.Duration, statuses ...string) *Task {
	t.Helper()
	want := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		want[s] = true
	}

	deadline := time.Now().Add(timeout)
	var task *Task
	var err error
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask(%d) error: %v", id, err)
		}
		if want[task.Status] {
			return task
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("pollTask(%d): timed out after %v; last status=%q, wanted one of %v",
		id, timeout, task.Status, statuses)
	return nil // unreachable
}

// TestE2E_HappyPath enqueues a task, the handler succeeds, and we verify
// status=completed with the expected result.
func TestE2E_HappyPath(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	q.RegisterHandler("e2e_echo", func(_ context.Context, payload string) (string, error) {
		return "result:" + payload, nil
	})

	id, err := q.Enqueue(Task{
		TaskType: "e2e_echo",
		Payload:  "hello",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	q.Start(ctx)

	task := pollTask(t, q, id, 5*time.Second, StatusCompleted, StatusFailed)

	if task.Status != StatusCompleted {
		t.Fatalf("expected status %q, got %q (error: %s)", StatusCompleted, task.Status, task.Error)
	}
	if task.Result != "result:hello" {
		t.Errorf("expected result %q, got %q", "result:hello", task.Result)
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set for completed task")
	}
}

// TestE2E_ErrorPath enqueues a task with MaxRetries=2, the handler always fails
// with a transient error (defaults to retry), and we verify the task reaches
// status=failed after retries are exhausted with the error populated.
func TestE2E_ErrorPath(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	var attempts int32
	var mu sync.Mutex

	q.RegisterHandler("e2e_fail", func(_ context.Context, _ string) (string, error) {
		mu.Lock()
		attempts++
		mu.Unlock()
		return "", errors.New("always fails")
	})

	id, err := q.Enqueue(Task{
		TaskType:   "e2e_fail",
		Payload:    "data",
		MaxRetries: 2,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	q.Start(ctx)

	// "always fails" classifies as transient → retried with exponential backoff.
	// MaxRetries=2 means: initial attempt + 2 retries with backoff (4s + 8s).
	// Need generous timeout to accommodate backoff sleeps + dispatcher polls.
	task := pollTask(t, q, id, 30*time.Second, StatusFailed)

	if task.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q", StatusFailed, task.Status)
	}
	if task.Error == "" {
		t.Error("expected non-empty error message")
	}

	// Verify we actually retried: initial attempt (0) + 2 retries = 3 total.
	mu.Lock()
	got := attempts
	mu.Unlock()
	if got < 2 {
		t.Errorf("expected at least 2 handler invocations (initial + retries), got %d", got)
	}
}

// TestE2E_RecoveryPath inserts a task with status=running directly into the DB
// (simulating a daemon crash), calls RecoverInterruptedTasks, and verifies it
// gets marked failed with "interrupted" message.
func TestE2E_RecoveryPath(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	// Directly insert a task with status='in_progress' to simulate interruption.
	now := time.Now().UTC()
	q.mu.Lock()
	res, err := q.db.Exec(
		`INSERT INTO tasks (task_type, status, payload, result, error, recipient_id,
		                     retry_count, max_retries, timeout_secs, created_at, updated_at)
		 VALUES (?, ?, ?, '', '', ?, 0, 3, 1800, ?, ?)`,
		"e2e_interrupted", StatusInProgress, `{"prompt":"build"}`, "user-recover", now, now,
	)
	q.mu.Unlock()
	if err != nil {
		t.Fatalf("insert running task: %v", err)
	}

	taskID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}

	// Recover interrupted tasks.
	recovered := q.RecoverInterruptedTasks(ctx)

	if len(recovered) != 1 {
		t.Fatalf("expected 1 recovered task, got %d", len(recovered))
	}

	rt := recovered[0]
	if rt.ID != taskID {
		t.Errorf("expected recovered task ID %d, got %d", taskID, rt.ID)
	}
	if rt.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, rt.Status)
	}
	if rt.Error != "interrupted by daemon restart" {
		t.Errorf("expected error %q, got %q", "interrupted by daemon restart", rt.Error)
	}
	if rt.CompletedAt == nil {
		t.Error("CompletedAt should be set for recovered task")
	}

	// Verify DB state via GetTask.
	dbTask, err := q.GetTask(taskID)
	if err != nil {
		t.Fatalf("GetTask(%d) error: %v", taskID, err)
	}
	if dbTask.Status != StatusFailed {
		t.Errorf("DB status: expected %q, got %q", StatusFailed, dbTask.Status)
	}
}

// TestE2E_NotificationOnComplete enqueues a task with a recipient_id, the handler
// succeeds, and we verify that a notification was sent to the correct recipient.
func TestE2E_NotificationOnComplete(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	sender := &e2eMockSender{}
	q.SetNotificationSender(sender)

	q.RegisterHandler("e2e_notify", func(_ context.Context, payload string) (string, error) {
		return "done:" + payload, nil
	})

	id, err := q.Enqueue(Task{
		TaskType:    "e2e_notify",
		Payload:     "work",
		RecipientID: "user-e2e-123",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	q.Start(ctx)

	// Wait for task completion.
	task := pollTask(t, q, id, 5*time.Second, StatusCompleted, StatusFailed)
	if task.Status != StatusCompleted {
		t.Fatalf("expected status %q, got %q (error: %s)", StatusCompleted, task.Status, task.Error)
	}

	// Allow goroutine to deliver the notification.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgs := sender.getMessages()
		if len(msgs) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	msgs := sender.getMessages()
	recipients := sender.getRecipients()

	if len(msgs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(msgs))
	}
	if recipients[0] != "user-e2e-123" {
		t.Errorf("recipient: got %q, want %q", recipients[0], "user-e2e-123")
	}
	if len(msgs[0]) == 0 {
		t.Error("notification message should not be empty")
	}
}
