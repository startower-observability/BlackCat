package taskqueue

import (
	"fmt"
	"strings"
)

// ErrorKind classifies task errors for retry decisions and user messaging.
type ErrorKind int

const (
	// ErrorKindTransient indicates a temporary failure that may succeed on retry.
	ErrorKindTransient ErrorKind = iota

	// ErrorKindPermanent indicates a failure that will not recover with retries.
	ErrorKindPermanent

	// ErrorKindUserActionNeeded indicates the user must take action (e.g. re-authenticate).
	ErrorKindUserActionNeeded
)

// String returns a human-readable label for the error kind.
func (k ErrorKind) String() string {
	switch k {
	case ErrorKindTransient:
		return "transient"
	case ErrorKindPermanent:
		return "permanent"
	case ErrorKindUserActionNeeded:
		return "user_action_needed"
	default:
		return "unknown"
	}
}

// ClassifyError inspects the error message and returns the appropriate ErrorKind.
// nil errors are classified as Permanent (nothing to retry).
func ClassifyError(err error) ErrorKind {
	if err == nil {
		return ErrorKindPermanent
	}

	msg := strings.ToLower(err.Error())

	switch {
	// Transient: network / timeout / temporary glitches.
	case strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "timeout"),
		strings.Contains(msg, "temporary"),
		strings.Contains(msg, "unavailable"),
		strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "try again"),
		strings.Contains(msg, "too many requests"),
		strings.Contains(msg, "rate limit"):
		return ErrorKindTransient

	// Permanent: bad input, missing resources.
	case strings.Contains(msg, "invalid dir"),
		strings.Contains(msg, "not found"),
		strings.Contains(msg, "no such file"),
		strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "invalid argument"),
		strings.Contains(msg, "bad request"):
		return ErrorKindPermanent

	// User action needed: authentication issues.
	case strings.Contains(msg, "auth"),
		strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "not authenticated"),
		strings.Contains(msg, "requires login"),
		strings.Contains(msg, "token expired"):
		return ErrorKindUserActionNeeded

	default:
		// Default to transient — better to retry than to silently fail.
		return ErrorKindTransient
	}
}

// ErrorKindMessage returns a user-friendly WhatsApp notification message
// appropriate for the error kind and the original error.
func ErrorKindMessage(kind ErrorKind, err error) string {
	errText := "unknown error"
	if err != nil {
		errText = err.Error()
	}

	switch kind {
	case ErrorKindTransient:
		return fmt.Sprintf("⚠️ Task failed after retries (temporary issue): %s", truncate(errText, 200))
	case ErrorKindPermanent:
		return fmt.Sprintf("❌ Task failed (permanent error): %s", truncate(errText, 200))
	case ErrorKindUserActionNeeded:
		return fmt.Sprintf("🔑 Task requires your action: %s\n\nPlease check your authentication and try again.", truncate(errText, 200))
	default:
		return fmt.Sprintf("❌ Task failed: %s", truncate(errText, 200))
	}
}
