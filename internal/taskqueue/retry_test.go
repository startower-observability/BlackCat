//go:build cgo

package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// ---------- Error Classification Tests ----------

func TestClassifyError_Nil(t *testing.T) {
	if got := ClassifyError(nil); got != ErrorKindPermanent {
		t.Errorf("ClassifyError(nil) = %v, want %v", got, ErrorKindPermanent)
	}
}

func TestClassifyError_Transient(t *testing.T) {
	cases := []string{
		"connection refused",
		"dial tcp: connection refused",
		"context deadline exceeded",
		"temporary failure in name resolution",
		"service unavailable",
		"timeout waiting for response",
		"try again later",
		"too many requests",
		"rate limit exceeded",
		"connection reset by peer",
	}
	for _, msg := range cases {
		err := errors.New(msg)
		if got := ClassifyError(err); got != ErrorKindTransient {
			t.Errorf("ClassifyError(%q) = %v, want %v", msg, got, ErrorKindTransient)
		}
	}
}

func TestClassifyError_Permanent(t *testing.T) {
	cases := []string{
		"invalid dir: /foo/bar",
		"file not found",
		"no such file or directory",
		"permission denied",
		"invalid argument provided",
		"bad request: missing field",
	}
	for _, msg := range cases {
		err := errors.New(msg)
		if got := ClassifyError(err); got != ErrorKindPermanent {
			t.Errorf("ClassifyError(%q) = %v, want %v", msg, got, ErrorKindPermanent)
		}
	}
}

func TestClassifyError_UserActionNeeded(t *testing.T) {
	cases := []string{
		"requires auth token",
		"unauthorized access",
		"not authenticated",
		"requires login to continue",
		"token expired",
	}
	for _, msg := range cases {
		err := errors.New(msg)
		if got := ClassifyError(err); got != ErrorKindUserActionNeeded {
			t.Errorf("ClassifyError(%q) = %v, want %v", msg, got, ErrorKindUserActionNeeded)
		}
	}
}

func TestClassifyError_DefaultTransient(t *testing.T) {
	// Unknown errors default to transient (better to retry than silently fail).
	err := errors.New("some unknown error xyz")
	if got := ClassifyError(err); got != ErrorKindTransient {
		t.Errorf("ClassifyError(unknown) = %v, want %v", got, ErrorKindTransient)
	}
}

func TestErrorKind_String(t *testing.T) {
	cases := []struct {
		kind ErrorKind
		want string
	}{
		{ErrorKindTransient, "transient"},
		{ErrorKindPermanent, "permanent"},
		{ErrorKindUserActionNeeded, "user_action_needed"},
		{ErrorKind(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("ErrorKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestErrorKindMessage(t *testing.T) {
	err := errors.New("connection refused")

	msg := ErrorKindMessage(ErrorKindTransient, err)
	if msg == "" {
		t.Error("ErrorKindMessage(Transient) returned empty string")
	}
	if !contains(msg, "⚠️") {
		t.Errorf("Transient message should contain warning emoji, got: %s", msg)
	}

	msg = ErrorKindMessage(ErrorKindPermanent, err)
	if !contains(msg, "❌") {
		t.Errorf("Permanent message should contain error emoji, got: %s", msg)
	}

	msg = ErrorKindMessage(ErrorKindUserActionNeeded, err)
	if !contains(msg, "🔑") {
		t.Errorf("UserAction message should contain key emoji, got: %s", msg)
	}

	msg = ErrorKindMessage(ErrorKindTransient, nil)
	if !contains(msg, "unknown error") {
		t.Errorf("nil error message should contain 'unknown error', got: %s", msg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------- Enqueue Default Tests ----------

func TestEnqueue_DefaultRetryValues(t *testing.T) {
	q := newTestQueue(t)

	id, err := q.Enqueue(Task{
		TaskType: "test_defaults",
		Payload:  "{}",
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	task, err := q.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask() error: %v", err)
	}

	if task.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", task.MaxRetries)
	}
	if task.TimeoutSecs != 1800 {
		t.Errorf("TimeoutSecs: got %d, want 1800", task.TimeoutSecs)
	}
	if task.RetryCount != 0 {
		t.Errorf("RetryCount: got %d, want 0", task.RetryCount)
	}
}

func TestEnqueue_CustomRetryValues(t *testing.T) {
	q := newTestQueue(t)

	id, err := q.Enqueue(Task{
		TaskType:    "test_custom",
		Payload:     "{}",
		MaxRetries:  5,
		TimeoutSecs: 60,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	task, err := q.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask() error: %v", err)
	}

	if task.MaxRetries != 5 {
		t.Errorf("MaxRetries: got %d, want 5", task.MaxRetries)
	}
	if task.TimeoutSecs != 60 {
		t.Errorf("TimeoutSecs: got %d, want 60", task.TimeoutSecs)
	}
}

// ---------- Retry Logic Tests ----------

func TestWorker_RetryOnTransientError(t *testing.T) {
	q := newTestQueue(t)

	attempt := 0
	q.RegisterHandler("retry_task", func(ctx context.Context, payload string) (string, error) {
		attempt++
		if attempt < 3 {
			return "", errors.New("connection refused")
		}
		return "success on attempt 3", nil
	})

	id, err := q.Enqueue(Task{
		TaskType:    "retry_task",
		Payload:     "{}",
		MaxRetries:  3,
		TimeoutSecs: 10,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	// Wait for task to complete (retries add backoff time).
	deadline := time.Now().Add(30 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusCompleted || task.Status == StatusFailed {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if task.Status != StatusCompleted {
		t.Fatalf("expected status %q, got %q (error: %s, retries: %d)", StatusCompleted, task.Status, task.Error, task.RetryCount)
	}
	if task.Result != "success on attempt 3" {
		t.Errorf("expected result %q, got %q", "success on attempt 3", task.Result)
	}
	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

func TestWorker_MaxRetriesExhausted(t *testing.T) {
	q := newTestQueue(t)

	q.RegisterHandler("always_fail", func(ctx context.Context, payload string) (string, error) {
		return "", errors.New("connection refused")
	})

	id, err := q.Enqueue(Task{
		TaskType:    "always_fail",
		Payload:     "{}",
		MaxRetries:  2,
		TimeoutSecs: 10,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	deadline := time.Now().Add(30 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusFailed {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if task.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q", StatusFailed, task.Status)
	}
	if task.Error == "" {
		t.Error("expected non-empty error after max retries")
	}
}

func TestWorker_PermanentError_NoRetry(t *testing.T) {
	q := newTestQueue(t)

	attempts := 0
	q.RegisterHandler("perm_fail", func(ctx context.Context, payload string) (string, error) {
		attempts++
		return "", errors.New("file not found: /nonexistent")
	})

	id, err := q.Enqueue(Task{
		TaskType:    "perm_fail",
		Payload:     "{}",
		MaxRetries:  3,
		TimeoutSecs: 10,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q", StatusFailed, task.Status)
	}
	// Permanent errors should not retry.
	if attempts != 1 {
		t.Errorf("permanent error should execute only once, got %d attempts", attempts)
	}
}

func TestWorker_UserActionNeeded_NoRetry(t *testing.T) {
	q := newTestQueue(t)

	attempts := 0
	q.RegisterHandler("auth_fail", func(ctx context.Context, payload string) (string, error) {
		attempts++
		return "", errors.New("unauthorized: token expired")
	})

	id, err := q.Enqueue(Task{
		TaskType:    "auth_fail",
		Payload:     "{}",
		MaxRetries:  3,
		TimeoutSecs: 10,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q", StatusFailed, task.Status)
	}
	if attempts != 1 {
		t.Errorf("auth error should execute only once, got %d attempts", attempts)
	}
	// Check error message contains the key emoji for user action.
	if !containsStr(task.Error, "🔑") {
		t.Errorf("expected user action message with key emoji, got: %s", task.Error)
	}
}

// ---------- Panic Recovery Tests ----------

func TestWorker_PanicRecovery(t *testing.T) {
	q := newTestQueue(t)

	q.RegisterHandler("panic_task", func(ctx context.Context, payload string) (string, error) {
		panic("unexpected nil pointer")
	})

	id, err := q.Enqueue(Task{
		TaskType:    "panic_task",
		Payload:     "{}",
		MaxRetries:  0, // no retries — should fail immediately
		TimeoutSecs: 10,
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q", StatusFailed, task.Status)
	}
	if !containsStr(task.Error, "internal error") {
		t.Errorf("expected panic error to contain 'internal error', got: %s", task.Error)
	}
}

// ---------- Timeout Tests ----------

func TestWorker_Timeout(t *testing.T) {
	q := newTestQueue(t)

	q.RegisterHandler("slow_task", func(ctx context.Context, payload string) (string, error) {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("task timed out: %w", ctx.Err())
		case <-time.After(30 * time.Second):
			return "should not reach", nil
		}
	})

	id, err := q.Enqueue(Task{
		TaskType:    "slow_task",
		Payload:     "{}",
		MaxRetries:  0, // no retries
		TimeoutSecs: 1, // 1 second timeout
	})
	if err != nil {
		t.Fatalf("Enqueue() error: %v", err)
	}

	ctx := context.Background()
	q.Start(ctx)

	deadline := time.Now().Add(10 * time.Second)
	var task *Task
	for time.Now().Before(deadline) {
		task, err = q.GetTask(id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if task.Status == StatusFailed || task.Status == StatusCompleted {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if task.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q (result: %s)", StatusFailed, task.Status, task.Result)
	}
}
