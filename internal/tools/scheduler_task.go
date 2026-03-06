package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/types"
	"gopkg.in/yaml.v3"
)

const (
	schedulerToolName        = "scheduler_task"
	schedulerToolDescription = "Manage scheduled tasks: list, get, create, update, or delete cron jobs"
)

var schedulerToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"operation": {
			"type": "string",
			"enum": ["list", "get", "create", "update", "delete"],
			"description": "CRUD operation to perform"
		},
		"name": {
			"type": "string",
			"description": "Task name (required for get, create, update, delete)"
		},
		"schedule": {
			"type": "string",
			"description": "Cron expression, e.g. '0 9 * * *'"
		},
		"command": {
			"type": "string",
			"description": "Shell command to execute"
		},
		"enabled": {
			"type": "boolean",
			"description": "Whether the task is enabled"
		}
	},
	"required": ["operation"]
}`)

// SchedulerTool manages scheduled task CRUD via config YAML editing.
type SchedulerTool struct {
	configPath string
}

var _ types.Tool = (*SchedulerTool)(nil)

// NewSchedulerTool creates a new SchedulerTool bound to the given config file.
func NewSchedulerTool(configPath string) *SchedulerTool {
	return &SchedulerTool{configPath: configPath}
}

func (t *SchedulerTool) Name() string                { return schedulerToolName }
func (t *SchedulerTool) Description() string         { return schedulerToolDescription }
func (t *SchedulerTool) Parameters() json.RawMessage { return schedulerToolParameters }

func (t *SchedulerTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Operation string `json:"operation"`
		Name      string `json:"name"`
		Schedule  string `json:"schedule"`
		Command   string `json:"command"`
		Enabled   *bool  `json:"enabled"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("scheduler_task: invalid arguments: %w", err)
	}

	params.Operation = strings.TrimSpace(params.Operation)
	params.Name = strings.TrimSpace(params.Name)

	switch params.Operation {
	case "list":
		return t.list()
	case "get":
		if params.Name == "" {
			return "", fmt.Errorf("scheduler_task: 'name' is required for get")
		}
		return t.get(params.Name)
	case "create":
		if params.Name == "" {
			return "", fmt.Errorf("scheduler_task: 'name' is required for create")
		}
		if strings.TrimSpace(params.Schedule) == "" {
			return "", fmt.Errorf("scheduler_task: 'schedule' is required for create")
		}
		if strings.TrimSpace(params.Command) == "" {
			return "", fmt.Errorf("scheduler_task: 'command' is required for create")
		}
		enabled := true
		if params.Enabled != nil {
			enabled = *params.Enabled
		}
		return t.create(params.Name, params.Schedule, params.Command, enabled)
	case "update":
		if params.Name == "" {
			return "", fmt.Errorf("scheduler_task: 'name' is required for update")
		}
		return t.update(params.Name, params.Schedule, params.Command, params.Enabled)
	case "delete":
		if params.Name == "" {
			return "", fmt.Errorf("scheduler_task: 'name' is required for delete")
		}
		return t.delete(params.Name)
	default:
		return "", fmt.Errorf("scheduler_task: unknown operation %q (use list, get, create, update, delete)", params.Operation)
	}
}

// readConfig loads and parses the YAML config file.
func (t *SchedulerTool) readConfig() (*config.Config, error) {
	data, err := os.ReadFile(t.configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// writeConfig marshals the config and writes it back to disk.
func (t *SchedulerTool) writeConfig(cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(t.configPath); statErr == nil {
		mode = info.Mode()
	}

	if err := os.WriteFile(t.configPath, data, mode); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// findJob returns the index and pointer to the job with the given name, or -1 if not found.
func findJob(jobs []config.ScheduledJob, name string) (int, *config.ScheduledJob) {
	for i := range jobs {
		if jobs[i].Name == name {
			return i, &jobs[i]
		}
	}
	return -1, nil
}

func (t *SchedulerTool) list() (string, error) {
	cfg, err := t.readConfig()
	if err != nil {
		return "", fmt.Errorf("scheduler_task list: %w", err)
	}

	jobs := cfg.Scheduler.Jobs
	if len(jobs) == 0 {
		return "📋 Scheduled Tasks:\n\n(none configured)\n\n💡 Use create operation to add a new task.", nil
	}

	var sb strings.Builder
	sb.WriteString("📋 Scheduled Tasks:\n\n")
	for _, j := range jobs {
		status := "✅ Enabled"
		if !j.Enabled {
			status = "⏸️ Disabled"
		}
		sb.WriteString(fmt.Sprintf("• %s: %s (%s)\n", j.Name, j.Schedule, status))
		sb.WriteString(fmt.Sprintf("  → Command: %s\n", j.Command))
		if j.Deliver != nil {
			sb.WriteString(fmt.Sprintf("  → Deliver: %s → %s\n", j.Deliver.Channel, j.Deliver.ChannelID))
		}
	}
	sb.WriteString(fmt.Sprintf("\n📊 Total: %d task(s)", len(jobs)))
	return sb.String(), nil
}

func (t *SchedulerTool) get(name string) (string, error) {
	cfg, err := t.readConfig()
	if err != nil {
		return "", fmt.Errorf("scheduler_task get: %w", err)
	}

	_, job := findJob(cfg.Scheduler.Jobs, name)
	if job == nil {
		return "", fmt.Errorf("scheduler_task get: task %q not found", name)
	}

	status := "✅ Enabled"
	if !job.Enabled {
		status = "⏸️ Disabled"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📌 Task: %s\n\n", job.Name))
	sb.WriteString(fmt.Sprintf("• Schedule: %s\n", job.Schedule))
	sb.WriteString(fmt.Sprintf("• Command: %s\n", job.Command))
	sb.WriteString(fmt.Sprintf("• Status: %s\n", status))
	if job.Deliver != nil {
		sb.WriteString(fmt.Sprintf("• Deliver: %s → %s\n", job.Deliver.Channel, job.Deliver.ChannelID))
		if job.Deliver.Message != "" {
			sb.WriteString(fmt.Sprintf("• Message: %s\n", job.Deliver.Message))
		}
	}
	return sb.String(), nil
}

func (t *SchedulerTool) create(name, schedule, command string, enabled bool) (string, error) {
	cfg, err := t.readConfig()
	if err != nil {
		return "", fmt.Errorf("scheduler_task create: %w", err)
	}

	if idx, _ := findJob(cfg.Scheduler.Jobs, name); idx >= 0 {
		return "", fmt.Errorf("scheduler_task create: task %q already exists", name)
	}

	job := config.ScheduledJob{
		Name:     name,
		Schedule: schedule,
		Command:  command,
		Enabled:  enabled,
	}
	cfg.Scheduler.Jobs = append(cfg.Scheduler.Jobs, job)

	if err := t.writeConfig(cfg); err != nil {
		return "", fmt.Errorf("scheduler_task create: %w", err)
	}

	status := "✅ Enabled"
	if !enabled {
		status = "⏸️ Disabled"
	}

	return fmt.Sprintf("✅ Task Created:\n\n"+
		"• Name: %s\n"+
		"• Schedule: %s\n"+
		"• Command: %s\n"+
		"• Status: %s\n\n"+
		"⚠️ Note: Restart daemon required for changes to take effect.",
		name, schedule, command, status), nil
}

func (t *SchedulerTool) update(name, schedule, command string, enabled *bool) (string, error) {
	cfg, err := t.readConfig()
	if err != nil {
		return "", fmt.Errorf("scheduler_task update: %w", err)
	}

	idx, job := findJob(cfg.Scheduler.Jobs, name)
	if idx < 0 {
		return "", fmt.Errorf("scheduler_task update: task %q not found", name)
	}

	var changes []string
	if s := strings.TrimSpace(schedule); s != "" {
		changes = append(changes, fmt.Sprintf("schedule: %s → %s", job.Schedule, s))
		job.Schedule = s
	}
	if c := strings.TrimSpace(command); c != "" {
		changes = append(changes, fmt.Sprintf("command: %s → %s", job.Command, c))
		job.Command = c
	}
	if enabled != nil {
		oldStatus := "enabled"
		if !job.Enabled {
			oldStatus = "disabled"
		}
		newStatus := "enabled"
		if !*enabled {
			newStatus = "disabled"
		}
		changes = append(changes, fmt.Sprintf("status: %s → %s", oldStatus, newStatus))
		job.Enabled = *enabled
	}

	if len(changes) == 0 {
		return "ℹ️ No changes specified for update.", nil
	}

	cfg.Scheduler.Jobs[idx] = *job

	if err := t.writeConfig(cfg); err != nil {
		return "", fmt.Errorf("scheduler_task update: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✏️ Task Updated: %s\n\n", name))
	for _, c := range changes {
		sb.WriteString(fmt.Sprintf("• %s\n", c))
	}
	sb.WriteString("\n⚠️ Note: Restart daemon required for changes to take effect.")
	return sb.String(), nil
}

func (t *SchedulerTool) delete(name string) (string, error) {
	cfg, err := t.readConfig()
	if err != nil {
		return "", fmt.Errorf("scheduler_task delete: %w", err)
	}

	idx, _ := findJob(cfg.Scheduler.Jobs, name)
	if idx < 0 {
		return "", fmt.Errorf("scheduler_task delete: task %q not found", name)
	}

	cfg.Scheduler.Jobs = append(cfg.Scheduler.Jobs[:idx], cfg.Scheduler.Jobs[idx+1:]...)

	if err := t.writeConfig(cfg); err != nil {
		return "", fmt.Errorf("scheduler_task delete: %w", err)
	}

	return fmt.Sprintf("🗑️ Task Deleted: %s\n\n"+
		"⚠️ Note: Restart daemon required for changes to take effect.", name), nil
}
