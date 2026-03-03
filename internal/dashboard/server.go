package dashboard

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/daemon"
	"github.com/startower-observability/blackcat/internal/scheduler"
)

type SubsystemManager interface {
	Healthz() []daemon.SubsystemHealth
}

type TaskLister interface {
	ListTasks() []string
}

type HeartbeatStore interface {
	Latest(n int) []scheduler.HeartbeatResult
}

type TaskDetailLister interface {
	ListTasks() []scheduler.TaskState
}

type ScheduleProvider interface {
	ListJobs() []CalendarJobInfo
}

type CalendarJobInfo struct {
	Name     string
	Schedule string
	Enabled  bool
}

type DashboardDeps struct {
	SubsystemManager SubsystemManager
	TaskLister       TaskLister
	HeartbeatStore   HeartbeatStore
	TaskDetailLister TaskDetailLister
	ScheduleProvider ScheduleProvider
}

type Server struct {
	router        *chi.Mux
	cfg           config.DashboardConfig
	deps          DashboardDeps
	httpServer    *http.Server
	broadcaster   *Broadcaster
	qrBroadcaster *QRBroadcaster
	sessionSecret []byte
	apiHandler    *APIHandler

	startupTime time.Time

	mu      sync.RWMutex
	started bool
	status  string
	message string
}

func NewServer(cfg config.DashboardConfig, deps DashboardDeps) *Server {
	if !cfg.Enabled {
		return nil
	}

	if cfg.Addr == "" {
		cfg.Addr = ":8081"
	}

	sessionSecret, err := generateSessionSecret()
	if err != nil {
		panic(fmt.Sprintf("dashboard session secret generation failed: %v", err))
	}

	s := &Server{
		cfg:           cfg,
		deps:          deps,
		broadcaster:   NewBroadcaster(),
		qrBroadcaster: NewQRBroadcaster(),
		sessionSecret: sessionSecret,
		startupTime:   time.Now(),
		status:        "stopped",
		message:       "not started",
	}

	r := chi.NewRouter()
	apiHandler := NewAPIHandler(deps, s.startupTime)
	s.apiHandler = apiHandler

	spaH := SPAHandler()

	// Login page: served by SPA without auth
	r.Get("/dashboard/login", spaH.ServeHTTP)
	r.Post("/dashboard/login", s.handleLogin)
	r.Get("/dashboard/logout", s.handleLogout)

	// Serve SPA assets WITHOUT auth (login page needs to load)
	r.Get("/dashboard/assets/*", spaH.ServeHTTP)
	r.Get("/dashboard/vite.svg", spaH.ServeHTTP)

	r.Route("/dashboard", func(r chi.Router) {
		r.Use(s.authMiddleware())
		r.Get("/events", s.handleEvents)
		r.Get("/qr/stream", s.handleQRStream)
		apiHandler.RegisterRoutes(r)
		// SPA fallback for all other routes under /dashboard
		r.Get("/*", spaH.ServeHTTP)
		r.Get("/", spaH.ServeHTTP)
	})

	s.router = r
	s.httpServer = &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	return s
}

func TokenAuthMiddleware(token string) func(http.Handler) http.Handler {
	expected := "Bearer " + token

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := strings.TrimSpace(r.Header.Get("Authorization"))
			if token == "" || authorization != expected {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.cfg.Token == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			if auth == "Bearer "+s.cfg.Token {
				next.ServeHTTP(w, r)
				return
			}

			if cookie, err := r.Cookie(sessionCookieName); err == nil {
				if validateSession(cookie.Value, s.cfg.Token, s.sessionSecret) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Browser requests: redirect to login page
			accept := strings.ToLower(r.Header.Get("Accept"))
			if strings.Contains(accept, "text/html") {
				nextPath := r.URL.Path
				if r.URL.RawQuery != "" {
					nextPath += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, "/dashboard/login?next="+nextPath, http.StatusSeeOther)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		})
	}
}

func (s *Server) Name() string {
	return "dashboard"
}

func (s *Server) Broadcaster() *Broadcaster {
	return s.broadcaster
}

func (s *Server) QRBroadcaster() *QRBroadcaster {
	return s.qrBroadcaster
}

func (s *Server) Start(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.status = "starting"
	s.message = "starting dashboard server"
	server := s.httpServer
	addr := s.cfg.Addr
	s.mu.Unlock()

	go func() {
		err := server.ListenAndServe()

		s.mu.Lock()
		defer s.mu.Unlock()
		s.started = false

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.status = "degraded"
			s.message = err.Error()
			return
		}

		s.status = "stopped"
		s.message = "stopped"
	}()

	s.mu.Lock()
	s.status = "running"
	s.message = "listening on " + addr
	s.mu.Unlock()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	s.mu.Lock()
	if !s.started {
		s.status = "stopped"
		if s.message == "" {
			s.message = "stopped"
		}
		s.mu.Unlock()
		return nil
	}
	server := s.httpServer
	s.mu.Unlock()

	err := server.Shutdown(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = false
	if err != nil {
		s.status = "degraded"
		s.message = err.Error()
		return err
	}

	s.status = "stopped"
	s.message = "stopped"
	return nil
}

func (s *Server) Health() daemon.SubsystemHealth {
	if s == nil {
		return daemon.SubsystemHealth{
			Name:    "dashboard",
			Status:  "stopped",
			Message: "disabled",
		}
	}

	s.mu.RLock()
	status := s.status
	message := s.message
	s.mu.RUnlock()

	return daemon.SubsystemHealth{
		Name:    s.Name(),
		Status:  status,
		Message: message,
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	next := r.FormValue("next")
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = "/dashboard/"
	}

	if !hmac.Equal([]byte(token), []byte(s.cfg.Token)) {
		http.Redirect(w, r, "/dashboard/login?error=invalid_token&next="+next, http.StatusSeeOther)
		return
	}

	setSessionCookie(w, s.cfg.Token, s.sessionSecret, isSecureRequest(r))
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/dashboard/login", http.StatusSeeOther)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsub := s.broadcaster.Subscribe()
	defer unsub()

	// Send initial keepalive comment
	fmt.Fprintf(w, ": keepalive\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event)
			flusher.Flush()
		}
	}
}
