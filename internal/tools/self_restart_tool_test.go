package tools

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/service"
)

// mockServiceManager is a test double for service.Manager.
type mockServiceManager struct {
	installed    bool
	restartErr   error
	restartCalls int
	restartCh    chan struct{}
}

func (m *mockServiceManager) Install(_ service.ServiceConfig) error { return nil }
func (m *mockServiceManager) Uninstall() error                      { return nil }
func (m *mockServiceManager) Start() error                          { return nil }
func (m *mockServiceManager) Stop() error                           { return nil }
func (m *mockServiceManager) Restart() error {
	m.restartCalls++
	if m.restartCh != nil {
		select {
		case m.restartCh <- struct{}{}:
		default:
		}
	}
	return m.restartErr
}
func (m *mockServiceManager) Status() (service.ServiceStatus, error) {
	return service.ServiceStatus{Installed: m.installed, Running: m.installed}, nil
}
func (m *mockServiceManager) IsInstalled() bool { return m.installed }

func TestSelfRestartRequiresConfirm(t *testing.T) {
	mock := &mockServiceManager{installed: true}
	tool := NewSelfRestartToolWithFactory(func() service.Manager { return mock })

	cases := []struct {
		name string
		args string
	}{
		{"confirm_false", `{"confirm": false}`},
		{"confirm_missing", `{}`},
		{"empty_args", ``},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rawArgs json.RawMessage
			if tc.args != "" {
				rawArgs = json.RawMessage(tc.args)
			}

			result, err := tool.Execute(context.Background(), rawArgs)
			if err != nil {
				t.Fatalf("Execute() returned unexpected error: %v", err)
			}
			if !strings.Contains(result, "Restart cancelled: confirm parameter must be true") {
				t.Fatalf("expected cancellation message, got: %q", result)
			}
			if mock.restartCalls > 0 {
				t.Fatal("Restart() should not have been called without confirmation")
			}
		})
	}
}

func TestSelfRestartRegistered(t *testing.T) {
	tool := NewSelfRestartTool()

	if got := tool.Name(); got != "self_restart" {
		t.Fatalf("Name() = %q, want %q", got, "self_restart")
	}
	if !strings.Contains(strings.ToLower(tool.Description()), "restart") {
		t.Fatalf("Description() should mention restart, got %q", tool.Description())
	}
	if len(tool.Parameters()) == 0 {
		t.Fatal("Parameters() returned empty schema")
	}

	// Verify tool can be registered and retrieved from a registry.
	reg := NewRegistry()
	reg.Register(tool)

	found, err := reg.Get("self_restart")
	if err != nil {
		t.Fatalf("registry.Get(self_restart) returned error: %v", err)
	}
	if found.Name() != "self_restart" {
		t.Fatalf("registry returned tool with wrong name: %q", found.Name())
	}
}

func TestSelfRestartPlatformGuard(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("platform guard test skipped on Linux (would attempt actual restart)")
	}

	mock := &mockServiceManager{installed: true}
	tool := NewSelfRestartToolWithFactory(func() service.Manager { return mock })

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"confirm": true}`))
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if !strings.Contains(result, "only supported on Linux") {
		t.Fatalf("expected platform guard message, got: %q", result)
	}
	if mock.restartCalls > 0 {
		t.Fatal("Restart() should not have been called on non-Linux")
	}
}

func TestSelfRestartNotInstalled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("not-installed test only meaningful on Linux")
	}

	mock := &mockServiceManager{installed: false}
	tool := NewSelfRestartToolWithFactory(func() service.Manager { return mock })

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"confirm": true}`))
	if err == nil {
		t.Fatal("expected error when daemon not installed, got nil")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("expected 'not installed' error, got: %v", err)
	}
}

func TestSelfRestartAsyncRestartOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("async restart test only meaningful on Linux")
	}

	mock := &mockServiceManager{installed: true, restartCh: make(chan struct{}, 1)}
	tool := NewSelfRestartToolWithFactory(func() service.Manager { return mock })

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"confirm": true}`))
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if !strings.Contains(result, "Daemon restart initiated") {
		t.Fatalf("expected success message, got: %q", result)
	}

	if mock.restartCalls != 0 {
		t.Fatalf("Restart() should not be called synchronously, got %d calls", mock.restartCalls)
	}

	select {
	case <-mock.restartCh:
		if mock.restartCalls == 0 {
			t.Fatal("expected Restart() to be called asynchronously")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for asynchronous Restart() call")
	}
}

func TestSelfRestartParametersSchema(t *testing.T) {
	tool := NewSelfRestartTool()

	var schema struct {
		Type       string `json:"type"`
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}

	if err := json.Unmarshal(tool.Parameters(), &schema); err != nil {
		t.Fatalf("failed to parse parameters schema: %v", err)
	}

	if schema.Type != "object" {
		t.Fatalf("schema type = %q, want object", schema.Type)
	}

	confirmProp, ok := schema.Properties["confirm"]
	if !ok {
		t.Fatal("schema missing 'confirm' property")
	}
	if confirmProp.Type != "boolean" {
		t.Fatalf("confirm type = %q, want boolean", confirmProp.Type)
	}

	found := false
	for _, r := range schema.Required {
		if r == "confirm" {
			found = true
		}
	}
	if !found {
		t.Fatal("'confirm' should be in required fields")
	}
}
