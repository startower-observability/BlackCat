package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/daemon"
	"github.com/startower-observability/blackcat/internal/scheduler"
)

const testDashboardToken = "test-token"

type mockSubsystemManager struct {
	health []daemon.SubsystemHealth
}

func (m mockSubsystemManager) Healthz() []daemon.SubsystemHealth {
	out := make([]daemon.SubsystemHealth, len(m.health))
	copy(out, m.health)
	return out
}

type mockTaskLister struct {
	tasks []string
}

func (m mockTaskLister) ListTasks() []string {
	out := make([]string, len(m.tasks))
	copy(out, m.tasks)
	return out
}

type mockHeartbeatStore struct {
	results []scheduler.HeartbeatResult
}

func (m mockHeartbeatStore) Latest(n int) []scheduler.HeartbeatResult {
	if n <= 0 {
		return []scheduler.HeartbeatResult{}
	}

	if n > len(m.results) {
		n = len(m.results)
	}

	out := make([]scheduler.HeartbeatResult, n)
	copy(out, m.results[:n])
	return out
}

func TestAPIStatus(t *testing.T) {
	server := newAPITestServer(t, nil, nil)
	rr := performAPIRequest(t, server, "/dashboard/api/status", true, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}

	if _, ok := response["healthy"]; !ok {
		t.Fatalf("expected healthy field in response: %s", rr.Body.String())
	}
}

func TestAPIAgents(t *testing.T) {
	server := newAPITestServer(t, nil, nil)
	rr := performAPIRequest(t, server, "/dashboard/api/agents", true, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var agents []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &agents); err != nil {
		t.Fatalf("failed to decode agents response: %v", err)
	}

	if len(agents) == 0 {
		t.Fatalf("expected non-empty agents array, got %s", rr.Body.String())
	}
}

func TestAPITasksPagination(t *testing.T) {
	tasks := make([]string, 0, 50)
	for i := 1; i <= 50; i++ {
		tasks = append(tasks, fmt.Sprintf("task-%d", i))
	}

	server := newAPITestServer(t, tasks, nil)
	rr := performAPIRequest(t, server, "/dashboard/api/tasks?page=2&limit=20", true, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response struct {
		Tasks []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"tasks"`
		Total int `json:"total"`
		Page  int `json:"page"`
		Limit int `json:"limit"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode tasks response: %v", err)
	}

	if response.Total != 50 {
		t.Fatalf("expected total 50, got %d", response.Total)
	}
	if response.Page != 2 {
		t.Fatalf("expected page 2, got %d", response.Page)
	}
	if response.Limit != 20 {
		t.Fatalf("expected limit 20, got %d", response.Limit)
	}
	if len(response.Tasks) != 20 {
		t.Fatalf("expected 20 tasks, got %d", len(response.Tasks))
	}
	if response.Tasks[0].ID != "21" || response.Tasks[0].Name != "task-21" {
		t.Fatalf("expected first task to be task-21, got id=%q name=%q", response.Tasks[0].ID, response.Tasks[0].Name)
	}
	if response.Tasks[19].ID != "40" || response.Tasks[19].Name != "task-40" {
		t.Fatalf("expected last task to be task-40, got id=%q name=%q", response.Tasks[19].ID, response.Tasks[19].Name)
	}
}

func TestAPITaskDetail(t *testing.T) {
	server := newAPITestServer(t, []string{"task-1", "task-2", "task-3"}, nil)

	valid := performAPIRequest(t, server, "/dashboard/api/tasks/2", true, "")
	if valid.Code != http.StatusOK {
		t.Fatalf("expected status %d for valid task id, got %d", http.StatusOK, valid.Code)
	}

	var task struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(valid.Body.Bytes(), &task); err != nil {
		t.Fatalf("failed to decode valid task response: %v", err)
	}

	if task.ID != "2" || task.Name != "task-2" {
		t.Fatalf("expected task id=2 name=task-2, got id=%q name=%q", task.ID, task.Name)
	}

	invalid := performAPIRequest(t, server, "/dashboard/api/tasks/999", true, "")
	if invalid.Code != http.StatusNotFound {
		t.Fatalf("expected status %d for invalid task id, got %d", http.StatusNotFound, invalid.Code)
	}
}

func TestAPIHealth(t *testing.T) {
	heartbeats := []scheduler.HeartbeatResult{
		{
			Timestamp:      time.Now().Add(-1 * time.Minute),
			OverallHealthy: true,
		},
		{
			Timestamp:      time.Now().Add(-2 * time.Minute),
			OverallHealthy: false,
		},
	}

	server := newAPITestServer(t, nil, heartbeats)
	rr := performAPIRequest(t, server, "/dashboard/api/health", true, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response []scheduler.HeartbeatResult
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if len(response) != 2 {
		t.Fatalf("expected 2 heartbeat results, got %d", len(response))
	}
}

func TestAPIUnauthorized(t *testing.T) {
	server := newAPITestServer(t, []string{"task-1"}, nil)
	endpoints := []string{
		"/dashboard/api/status",
		"/dashboard/api/agents",
		"/dashboard/api/tasks",
		"/dashboard/api/tasks/1",
		"/dashboard/api/health",
	}

	for _, endpoint := range endpoints {
		rr := performAPIRequest(t, server, endpoint, false, "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d for %s, got %d", http.StatusUnauthorized, endpoint, rr.Code)
		}
	}
}

func TestAPIAlwaysReturnsJSON(t *testing.T) {
	server := newAPITestServer(t, []string{"task-1"}, nil)
	rr := performAPIRequest(t, server, "/dashboard/api/agents", true, "text/html")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected application/json content type even with Accept: text/html, got %q", contentType)
	}

	var payload interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON body, decode error: %v", err)
	}
}

func newAPITestServer(t *testing.T, tasks []string, heartbeats []scheduler.HeartbeatResult) *Server {
	t.Helper()

	deps := DashboardDeps{
		SubsystemManager: mockSubsystemManager{health: []daemon.SubsystemHealth{
			{Name: "scheduler", Status: "running", Message: "running; tasks=2"},
			{Name: "health", Status: "running", Message: "listening on :9090"},
		}},
		TaskLister:     mockTaskLister{tasks: tasks},
		HeartbeatStore: mockHeartbeatStore{results: heartbeats},
	}

	server := NewServer(config.DashboardConfig{
		Enabled: true,
		Addr:    ":8081",
		Token:   testDashboardToken,
	}, deps)

	if server == nil {
		t.Fatal("expected dashboard server instance")
	}

	return server
}

func performAPIRequest(t *testing.T, server *Server, path string, withToken bool, accept string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if withToken {
		req.Header.Set("Authorization", "Bearer "+testDashboardToken)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	return rr
}
