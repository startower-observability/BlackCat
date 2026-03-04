package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

// ChannelHealthProvider exposes channel health information.
type ChannelHealthProvider interface {
	ChannelHealthList() []ChannelHealthStatus
}

// ChannelHealthStatus represents the health of a single channel.
type ChannelHealthStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Details string `json:"details"`
}

// subsystemStatus is the JSON representation of a subsystem's health.
type subsystemStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// healthResponse is the JSON response for the /health endpoint.
type healthResponse struct {
	Status     string             `json:"status"`
	Service    string             `json:"service"`
	Uptime     string             `json:"uptime"`
	StartedAt  time.Time          `json:"startedAt"`
	Subsystems []subsystemStatus  `json:"subsystems,omitempty"`
	Channels   []ChannelHealthStatus `json:"channels,omitempty"`
}

type HealthSubsystem struct {
	addr      string
	service   string
	startTime time.Time

	mu      sync.RWMutex
	server  *http.Server
	mux     *http.ServeMux
	errCh   chan error
	status  string
	message string

	registry SubsystemHealthLister
	channels ChannelHealthProvider
}

// SubsystemHealthLister returns health of all registered subsystems.
type SubsystemHealthLister interface {
	Healthz() []SubsystemHealth
}

// HealthOption configures the HealthSubsystem.
type HealthOption func(*HealthSubsystem)

// WithRegistry injects the subsystem registry for health reporting.
func WithRegistry(r SubsystemHealthLister) HealthOption {
	return func(h *HealthSubsystem) { h.registry = r }
}

// WithChannelHealth injects the channel health provider.
func WithChannelHealth(c ChannelHealthProvider) HealthOption {
	return func(h *HealthSubsystem) { h.channels = c }
}

func NewHealthSubsystem(addr, service string, opts ...HealthOption) *HealthSubsystem {
	h := &HealthSubsystem{
		addr:      addr,
		service:   service,
		startTime: time.Now(),
		status:    "stopped",
		message:   "not started",
		errCh:     make(chan error, 1),
	}

	for _, opt := range opts {
		opt(h)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	h.mux = mux
	h.server = &http.Server{Addr: addr, Handler: mux}

	return h
}

// HandleFunc registers an additional HTTP handler on the health server's mux.
// Must be called before Start().
func (h *HealthSubsystem) HandleFunc(pattern string, handler http.HandlerFunc) {
	h.mux.HandleFunc(pattern, handler)
}

func (h *HealthSubsystem) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:    "healthy",
		Service:   h.service,
		Uptime:    time.Since(h.startTime).Truncate(time.Second).String(),
		StartedAt: h.startTime,
	}

	// Collect subsystem health from registry
	if h.registry != nil {
		for _, sh := range h.registry.Healthz() {
			resp.Subsystems = append(resp.Subsystems, subsystemStatus{
				Name:    sh.Name,
				Status:  sh.Status,
				Message: sh.Message,
			})
			if sh.Status == "degraded" || sh.Status == "error" {
				resp.Status = "degraded"
			}
		}
	}

	// Collect channel health
	if h.channels != nil {
		for _, ch := range h.channels.ChannelHealthList() {
			resp.Channels = append(resp.Channels, ch)
			if !ch.Healthy {
				if resp.Status == "healthy" {
					resp.Status = "degraded"
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *HealthSubsystem) Name() string {
	return "health"
}

func (h *HealthSubsystem) Start(ctx context.Context) error {
	h.mu.Lock()
	if h.status == "running" {
		h.mu.Unlock()
		return nil
	}
	server := h.server
	h.status = "starting"
	h.message = "starting health server"
	h.mu.Unlock()

	go func() {
		if err := server.ListenAndServe(); err != nil {
			h.mu.Lock()
			defer h.mu.Unlock()
			if errors.Is(err, http.ErrServerClosed) {
				h.status = "stopped"
				h.message = "health server stopped"
				return
			}
			h.status = "degraded"
			h.message = err.Error()
			select {
			case h.errCh <- err:
			default:
			}
		}
	}()

	h.mu.Lock()
	h.status = "running"
	h.message = "listening on " + h.addr
	h.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

func (h *HealthSubsystem) Stop(ctx context.Context) error {
	h.mu.Lock()
	if h.status == "stopped" {
		h.mu.Unlock()
		return nil
	}
	server := h.server
	h.mu.Unlock()

	err := server.Shutdown(ctx)

	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		h.status = "degraded"
		h.message = err.Error()
		return err
	}
	h.status = "stopped"
	h.message = "stopped"

	return nil
}

func (h *HealthSubsystem) Health() SubsystemHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return SubsystemHealth{
		Name:    h.Name(),
		Status:  h.status,
		Message: h.message,
	}
}

func (h *HealthSubsystem) Errors() <-chan error {
	return h.errCh
}
