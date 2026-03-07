package scheduler

import (
	"github.com/startower-observability/blackcat/internal/daemon"
)

// SchedulerTaskView is the LLM-safe view of a single scheduled task's runtime state.
type SchedulerTaskView struct {
	Name       string `json:"name"`
	LastStatus string `json:"last_status"` // "ok", "failed", "skipped", "running", or "" if never run
	RunCount   int    `json:"run_count"`
}

// SchedulerStatusSnapshot holds a point-in-time view of the scheduler subsystem's
// runtime state. It reads from ListTasks() and Health(), NOT from YAML config.
type SchedulerStatusSnapshot struct {
	Enabled   bool                `json:"enabled"`
	Status    string              `json:"status"` // from Health(): "running", "stopped", "degraded", etc.
	Message   string              `json:"message,omitempty"`
	TaskCount int                 `json:"task_count"`
	Tasks     []SchedulerTaskView `json:"tasks,omitempty"`
}

// BuildSchedulerStatusSnapshot constructs a snapshot from the scheduler subsystem's
// runtime state. If sub is nil, the snapshot reflects a disabled scheduler.
func BuildSchedulerStatusSnapshot(sub *SchedulerSubsystem) SchedulerStatusSnapshot {
	if sub == nil {
		return SchedulerStatusSnapshot{
			Enabled: false,
			Status:  "stopped",
			Message: "scheduler subsystem not available",
		}
	}

	health := sub.Health()
	tasks := sub.ListTasks()

	enabled := health.Status == "running" || health.Status == "starting" || health.Status == "degraded"

	views := make([]SchedulerTaskView, len(tasks))
	for i, t := range tasks {
		views[i] = SchedulerTaskView{
			Name:       t.Name,
			LastStatus: t.LastStatus,
			RunCount:   t.RunCount,
		}
	}

	return SchedulerStatusSnapshot{
		Enabled:   enabled,
		Status:    health.Status,
		Message:   health.Message,
		TaskCount: len(tasks),
		Tasks:     views,
	}
}

// HealthStatus is a convenience type alias for daemon.SubsystemHealth,
// used when only the health portion is needed without importing daemon.
type HealthStatus = daemon.SubsystemHealth
