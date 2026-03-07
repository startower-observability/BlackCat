package scheduler

import (
	"context"
	"testing"

	"github.com/startower-observability/blackcat/internal/config"
)

func TestSchedulerStatusSnapshotEnabled(t *testing.T) {
	sub := NewSchedulerSubsystem(config.SchedulerConfig{
		Enabled: true,
		Jobs: []config.ScheduledJob{
			{
				Name:     "test-job-1",
				Schedule: "@every 1h",
				Enabled:  true,
			},
			{
				Name:     "test-job-2",
				Schedule: "@every 30m",
				Enabled:  true,
			},
		},
	})

	if err := sub.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = sub.Stop(context.Background()) })

	snap := BuildSchedulerStatusSnapshot(sub)

	if !snap.Enabled {
		t.Fatal("expected Enabled=true for running scheduler")
	}
	if snap.Status != "running" {
		t.Fatalf("Status = %q, want %q", snap.Status, "running")
	}
	// heartbeat (auto-registered) + 2 configured jobs = 3 tasks
	if snap.TaskCount != 3 {
		t.Fatalf("TaskCount = %d, want 3", snap.TaskCount)
	}
	if len(snap.Tasks) != 3 {
		t.Fatalf("len(Tasks) = %d, want 3", len(snap.Tasks))
	}

	// Verify task names are present
	names := make(map[string]bool)
	for _, task := range snap.Tasks {
		names[task.Name] = true
	}
	for _, want := range []string{"test-job-1", "test-job-2"} {
		if !names[want] {
			t.Fatalf("task %q not found in snapshot, got names: %v", want, names)
		}
	}
}

func TestSchedulerStatusSnapshotDisabled(t *testing.T) {
	snap := BuildSchedulerStatusSnapshot(nil)

	if snap.Enabled {
		t.Fatal("expected Enabled=false for nil subsystem")
	}
	if snap.Status != "stopped" {
		t.Fatalf("Status = %q, want %q", snap.Status, "stopped")
	}
	if snap.TaskCount != 0 {
		t.Fatalf("TaskCount = %d, want 0", snap.TaskCount)
	}
	if len(snap.Tasks) != 0 {
		t.Fatalf("len(Tasks) = %d, want 0", len(snap.Tasks))
	}
}

func TestSchedulerStatusSnapshotDisabledSubsystem(t *testing.T) {
	// Subsystem exists but scheduler is disabled in config
	sub := NewSchedulerSubsystem(config.SchedulerConfig{Enabled: false})

	if err := sub.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = sub.Stop(context.Background()) })

	snap := BuildSchedulerStatusSnapshot(sub)

	if snap.Enabled {
		t.Fatal("expected Enabled=false for disabled scheduler config")
	}
	if snap.Status != "stopped" {
		t.Fatalf("Status = %q, want %q", snap.Status, "stopped")
	}
	if snap.TaskCount != 0 {
		t.Fatalf("TaskCount = %d, want 0", snap.TaskCount)
	}
}
