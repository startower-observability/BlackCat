package taskqueue

import "context"

// NotificationSender is implemented by any channel that can send messages to a user.
// The WhatsApp channel implements this interface.
type NotificationSender interface {
	Send(ctx context.Context, recipientID string, message string) error
}

// SetNotificationSender registers a sender used to deliver task completion
// and failure notifications. If a sender is set and a task's RecipientID is
// non-empty, the worker will asynchronously notify the recipient when the
// task reaches a terminal state. Must be called before Start.
func (q *TaskQueue) SetNotificationSender(sender NotificationSender) {
	q.notificationSender = sender
}
