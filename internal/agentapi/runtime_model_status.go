package agentapi

import (
	"sync"
)

// RuntimeModelRef is the normalized model identity captured at runtime.
// It mirrors llm.CanonicalModelRef shape without importing internal/llm
// (agentapi is imported by llm; importing llm here would create an import cycle).
type RuntimeModelRef struct {
	CanonicalID     string
	Vendor          string
	RawModel        string
	DisplayName     string
	BackendProvider string
	SourceProvider  string
}

// RuntimeModelStatus captures the actual runtime state of the active model.
// This is the single source of truth for what model is CURRENTLY running,
// distinct from what's persisted in config.
type RuntimeModelStatus struct {
	// ConfiguredModel is what's written in blackcat.yaml (may differ from applied)
	ConfiguredModel RuntimeModelRef

	// AppliedModel is the model actually loaded into the active backend
	AppliedModel RuntimeModelRef

	// BackendProvider is the active backend key (e.g. "openai", "copilot", "zen")
	BackendProvider string

	// LastReloadError is non-empty if the last config reload attempt failed
	LastReloadError string

	// ReloadCount is incremented each time a successful reload occurs
	ReloadCount int
}

// RuntimeModelHolder provides threadsafe access to the current RuntimeModelStatus.
type RuntimeModelHolder struct {
	mu     sync.RWMutex
	status RuntimeModelStatus
}

// NewRuntimeModelHolder creates a new holder with an empty status.
func NewRuntimeModelHolder() *RuntimeModelHolder {
	return &RuntimeModelHolder{}
}

// Set atomically replaces the current status.
func (h *RuntimeModelHolder) Set(s RuntimeModelStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status = s
}

// Get returns a copy of the current status.
func (h *RuntimeModelHolder) Get() RuntimeModelStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

// UpdateApplied updates only the AppliedModel and BackendProvider fields.
func (h *RuntimeModelHolder) UpdateApplied(applied RuntimeModelRef, backend string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status.AppliedModel = applied
	h.status.BackendProvider = backend
}

// SetReloadError sets the LastReloadError field.
func (h *RuntimeModelHolder) SetReloadError(errMsg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.status.LastReloadError = errMsg
}
