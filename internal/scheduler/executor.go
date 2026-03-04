package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/types"
)

// JobExecutor defines how scheduled jobs execute their commands.
type JobExecutor interface {
	Execute(ctx context.Context, job config.ScheduledJob) error
}

// ChannelSender sends a message to a specific channel type.
// Implemented by channel.MessageBus.
type ChannelSender interface {
	Send(ctx context.Context, channelType types.ChannelType, msg types.Message) error
}

// ShellExecutor runs job commands as shell subprocesses.
type ShellExecutor struct{}

// Execute runs the job's Command as a shell subprocess.
func (e *ShellExecutor) Execute(ctx context.Context, job config.ScheduledJob) error {
	if job.Command == "" {
		return fmt.Errorf("job %q: command is empty", job.Name)
	}

	cmd := shellCommand(ctx, job.Command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	slog.Info("executing scheduled job", "job", job.Name, "command", job.Command)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("job %q command failed: %w", job.Name, err)
	}

	return nil
}

// ChannelExecutor handles jobs with Deliver config by sending messages
// directly through the channel bus. For jobs without Deliver, it falls
// back to ShellExecutor. This replaces the need for a `blackcat channels send`
// CLI subcommand in scheduled jobs.
type ChannelExecutor struct {
	Sender ChannelSender
	Shell  *ShellExecutor
}

// Execute handles the job: if Deliver is configured, sends the message
// directly via the channel bus. If Command is also set, it runs the command
// first and delivers its output. If only Command is set, delegates to ShellExecutor.
func (e *ChannelExecutor) Execute(ctx context.Context, job config.ScheduledJob) error {
	if job.Deliver == nil {
		// No deliver config — fall back to plain shell execution.
		return e.Shell.Execute(ctx, job)
	}

	channelType := types.ChannelType(job.Deliver.Channel)
	if channelType == "" {
		return fmt.Errorf("job %q: deliver.channel is required", job.Name)
	}
	if job.Deliver.ChannelID == "" {
		return fmt.Errorf("job %q: deliver.channelId is required", job.Name)
	}

	// Determine message content.
	content := job.Deliver.Message
	if content == "" && job.Command != "" {
		// Run the command and capture stdout as the message.
		cmd := shellCommand(ctx, job.Command)
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("job %q command failed: %w", job.Name, err)
		}
		content = string(out)
	}
	if content == "" {
		return fmt.Errorf("job %q: no message to deliver (set deliver.message or command)", job.Name)
	}

	slog.Info("delivering scheduled message",
		"job", job.Name,
		"channel", job.Deliver.Channel,
		"channelId", job.Deliver.ChannelID,
	)

	msg := types.Message{
		ID:          fmt.Sprintf("sched-%s-%d", job.Name, time.Now().UnixMilli()),
		ChannelType: channelType,
		ChannelID:   job.Deliver.ChannelID,
		Content:     content,
		Timestamp:   time.Now(),
	}

	if err := e.Sender.Send(ctx, channelType, msg); err != nil {
		return fmt.Errorf("job %q deliver failed: %w", job.Name, err)
	}

	slog.Info("scheduled message delivered", "job", job.Name, "channel", job.Deliver.Channel)
	return nil
}

// shellCommand returns an exec.Cmd appropriate for the current OS.
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}
