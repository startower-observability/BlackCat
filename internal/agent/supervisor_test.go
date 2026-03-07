package agent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/security"
	"github.com/startower-observability/blackcat/internal/tools"
	"github.com/startower-observability/blackcat/internal/types"
)

// supervisorMockLLM is a mock LLM client for supervisor tests.
type supervisorMockLLM struct {
	responses []*types.LLMResponse
	idx       int
}

func (m *supervisorMockLLM) Chat(_ context.Context, _ []types.LLMMessage, _ []types.ToolDefinition) (*types.LLMResponse, error) {
	if m.idx >= len(m.responses) {
		return &types.LLMResponse{Content: "done"}, nil
	}
	r := m.responses[m.idx]
	m.idx++
	return r, nil
}

func (m *supervisorMockLLM) Stream(_ context.Context, _ []types.LLMMessage, _ []types.ToolDefinition) (<-chan types.Chunk, error) {
	ch := make(chan types.Chunk, 1)
	close(ch)
	return ch, nil
}

// supervisorMockTool is a mock tool for supervisor tests.
type supervisorMockTool struct {
	name   string
	result string
	params json.RawMessage
}

func (m *supervisorMockTool) Name() string                { return m.name }
func (m *supervisorMockTool) Description() string         { return "supervisor mock tool" }
func (m *supervisorMockTool) Parameters() json.RawMessage { return m.params }
func (m *supervisorMockTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return m.result, nil
}

func TestClassifyMessage_Coding(t *testing.T) {
	tests := []struct {
		msg  string
		want TaskType
	}{
		{"fix the bug in my code", TaskTypeCoding},
		{"implement a new function", TaskTypeCoding},
		{"write some tests for the module", TaskTypeCoding},
		{"help me build this project", TaskTypeCoding},
		{"deploy the application", TaskTypeAdmin}, // phantom has "deploy" at priority 10
	}
	for _, tc := range tests {
		got := ClassifyMessage(tc.msg, nil)
		if got != tc.want {
			t.Errorf("ClassifyMessage(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

func TestClassifyMessage_Research(t *testing.T) {
	tests := []struct {
		msg  string
		want TaskType
	}{
		{"search for recent Go news", TaskTypeResearch},
		{"what is the meaning of life", TaskTypeResearch},
		{"explain how goroutines work", TaskTypeResearch},
		{"summarize that article", RoleScribe}, // scribe(50) has "summarize" before explorer(60)
		{"browse the web for info", TaskTypeResearch},
	}
	for _, tc := range tests {
		got := ClassifyMessage(tc.msg, nil)
		if got != tc.want {
			t.Errorf("ClassifyMessage(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

func TestClassifyMessage_Admin(t *testing.T) {
	tests := []struct {
		msg  string
		want TaskType
	}{
		{"restart the service", TaskTypeAdmin},
		{"check the health endpoint", TaskTypeAdmin},
		{"update the config file", TaskTypeGeneral}, // no keyword match → oracle fallback
		{"check blackcat status", TaskTypeAdmin},
		{"stop the server now", TaskTypeAdmin},
	}
	for _, tc := range tests {
		got := ClassifyMessage(tc.msg, nil)
		if got != tc.want {
			t.Errorf("ClassifyMessage(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

func TestClassifyMessage_General(t *testing.T) {
	tests := []struct {
		msg  string
		want TaskType
	}{
		{"hello how are you", TaskTypeGeneral},
		{"tell me a joke", TaskTypeGeneral},
		{"good morning", TaskTypeGeneral},
	}
	for _, tc := range tests {
		got := ClassifyMessage(tc.msg, nil)
		if got != tc.want {
			t.Errorf("ClassifyMessage(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

func TestClassifyMessage_AdminPriorityOverCoding(t *testing.T) {
	// "restart" is admin, "build" is coding — admin should win.
	got := ClassifyMessage("restart the build server", nil)
	if got != TaskTypeAdmin {
		t.Errorf("ClassifyMessage(admin+coding overlap) = %q, want %q", got, TaskTypeAdmin)
	}
}

func TestClassifyMessage_CodingPriorityOverResearch(t *testing.T) {
	// "test" is coding, "search" is research — coding should win.
	got := ClassifyMessage("search and test the code", nil)
	if got != TaskTypeCoding {
		t.Errorf("ClassifyMessage(coding+research overlap) = %q, want %q", got, TaskTypeCoding)
	}
}

func TestSupervisor_NilReturnsError(t *testing.T) {
	var s *Supervisor
	_, err := s.RouteWithCfg(context.Background(), "hello", LoopConfig{})
	if err == nil {
		t.Fatal("expected error from nil supervisor, got nil")
	}
	if err.Error() != "supervisor not initialized" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestSupervisor_RouteWithCfg_CodingMessage(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&supervisorMockTool{
		name:   "echo",
		result: "ok",
		params: json.RawMessage(`{"type":"object"}`),
	})

	llm := &supervisorMockLLM{responses: []*types.LLMResponse{
		{Content: "I fixed the bug"},
	}}

	baseCfg := LoopConfig{
		LLM:       llm,
		Tools:     registry,
		Scrubber:  security.NewScrubber(),
		MaxTurns:  10,
		AgentName: "TestBot",
	}

	supervisor := NewSupervisor(baseCfg, nil, nil)

	execution, err := supervisor.RouteWithCfg(context.Background(), "fix the bug in my code", baseCfg)
	if err != nil {
		t.Fatalf("RouteWithCfg() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Errorf("expected FinalOutput, got %v", execution.NextStep)
	}
	if execution.Response != "I fixed the bug" {
		t.Errorf("expected response 'I fixed the bug', got %q", execution.Response)
	}
}

func TestSupervisor_Route_UsesBaseConfig(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&supervisorMockTool{
		name:   "echo",
		result: "ok",
		params: json.RawMessage(`{"type":"object"}`),
	})

	llm := &supervisorMockLLM{responses: []*types.LLMResponse{
		{Content: "hello there"},
	}}

	baseCfg := LoopConfig{
		LLM:       llm,
		Tools:     registry,
		Scrubber:  security.NewScrubber(),
		MaxTurns:  10,
		AgentName: "TestBot",
	}

	supervisor := NewSupervisor(baseCfg, nil, nil)

	execution, err := supervisor.Route(context.Background(), "hello how are you")
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Errorf("expected FinalOutput, got %v", execution.NextStep)
	}
}

func TestNewSupervisor_DefaultConfigs(t *testing.T) {
	supervisor := NewSupervisor(LoopConfig{}, nil, nil)

	// Verify all four task types have configs
	for _, tt := range []TaskType{TaskTypeCoding, TaskTypeResearch, TaskTypeAdmin, TaskTypeGeneral} {
		if _, ok := supervisor.subAgentConfigs[tt]; !ok {
			t.Errorf("missing sub-agent config for task type %q", tt)
		}
	}

	// Research should have restricted tools
	researchCfg := supervisor.subAgentConfigs[TaskTypeResearch]
	if researchCfg.AllowedTools == nil {
		t.Error("research sub-agent should have AllowedTools set")
	}
	if len(researchCfg.AllowedTools) != 5 {
		t.Errorf("research AllowedTools count = %d, want 5", len(researchCfg.AllowedTools))
	}

	// Coding should have nil AllowedTools (all tools)
	codingCfg := supervisor.subAgentConfigs[TaskTypeCoding]
	if codingCfg.AllowedTools != nil {
		t.Error("coding sub-agent should have nil AllowedTools (all tools)")
	}

	// General should have empty overlay
	generalCfg := supervisor.subAgentConfigs[TaskTypeGeneral]
	if generalCfg.SystemPromptOverlay != "" {
		t.Errorf("general overlay should be empty, got %q", generalCfg.SystemPromptOverlay)
	}
}

// ---------------------------------------------------------------------------
// trackingMockLLM records whether Chat was called. Used to verify backend swap.
// ---------------------------------------------------------------------------
type trackingMockLLM struct {
	chatCalled atomic.Bool
}

func (m *trackingMockLLM) Chat(_ context.Context, _ []types.LLMMessage, _ []types.ToolDefinition) (*types.LLMResponse, error) {
	m.chatCalled.Store(true)
	return &types.LLMResponse{Content: "tracking-mock-response"}, nil
}

func (m *trackingMockLLM) Stream(_ context.Context, _ []types.LLMMessage, _ []types.ToolDefinition) (<-chan types.Chunk, error) {
	ch := make(chan types.Chunk, 1)
	close(ch)
	return ch, nil
}

// ---------------------------------------------------------------------------
// Phase 4: role-routing and backend-swapping tests
// ---------------------------------------------------------------------------

func TestNewSupervisorWithRoles(t *testing.T) {
	roles := []config.RoleConfig{
		{
			Name:         "alpha",
			SystemPrompt: "You are alpha.",
			Keywords:     []string{"hello"},
			AllowedTools: []string{"tool_a"},
			Model:        "model-a",
			Provider:     "prov-a",
			Temperature:  0.5,
		},
		{
			Name:         "beta",
			SystemPrompt: "You are beta.",
			Keywords:     []string{"world"},
			AllowedTools: nil,
			Model:        "model-b",
			Provider:     "prov-b",
			Temperature:  0.9,
		},
	}

	supervisor := NewSupervisor(LoopConfig{}, roles, nil)

	if len(supervisor.subAgentConfigs) != 2 {
		t.Fatalf("expected 2 subAgentConfigs, got %d", len(supervisor.subAgentConfigs))
	}

	alpha, ok := supervisor.subAgentConfigs["alpha"]
	if !ok {
		t.Fatal("missing subAgentConfig for role 'alpha'")
	}
	if alpha.Name != "alpha" {
		t.Errorf("alpha.Name = %q, want %q", alpha.Name, "alpha")
	}
	if alpha.SystemPromptOverlay != "You are alpha." {
		t.Errorf("alpha.SystemPromptOverlay = %q, want %q", alpha.SystemPromptOverlay, "You are alpha.")
	}
	if len(alpha.AllowedTools) != 1 || alpha.AllowedTools[0] != "tool_a" {
		t.Errorf("alpha.AllowedTools = %v, want [tool_a]", alpha.AllowedTools)
	}
	if alpha.Model != "model-a" {
		t.Errorf("alpha.Model = %q, want %q", alpha.Model, "model-a")
	}
	if alpha.Provider != "prov-a" {
		t.Errorf("alpha.Provider = %q, want %q", alpha.Provider, "prov-a")
	}
	if alpha.Temperature != 0.5 {
		t.Errorf("alpha.Temperature = %f, want 0.5", alpha.Temperature)
	}

	beta, ok := supervisor.subAgentConfigs["beta"]
	if !ok {
		t.Fatal("missing subAgentConfig for role 'beta'")
	}
	if beta.Model != "model-b" {
		t.Errorf("beta.Model = %q, want %q", beta.Model, "model-b")
	}
	if beta.AllowedTools != nil {
		t.Errorf("beta.AllowedTools should be nil, got %v", beta.AllowedTools)
	}
}

func TestNewSupervisorWithNilRoles(t *testing.T) {
	supervisor := NewSupervisor(LoopConfig{}, nil, nil)

	// Must fall back to 4 hardcoded defaults.
	if len(supervisor.subAgentConfigs) != 4 {
		t.Fatalf("expected 4 default subAgentConfigs, got %d", len(supervisor.subAgentConfigs))
	}
	for _, tt := range []RoleType{TaskTypeCoding, TaskTypeResearch, TaskTypeAdmin, TaskTypeGeneral} {
		if _, ok := supervisor.subAgentConfigs[tt]; !ok {
			t.Errorf("missing default subAgentConfig for %q", tt)
		}
	}
}

func TestNewSupervisorWithEmptyRoles(t *testing.T) {
	supervisor := NewSupervisor(LoopConfig{}, []config.RoleConfig{}, nil)

	// Empty slice also falls back to 4 hardcoded defaults.
	if len(supervisor.subAgentConfigs) != 4 {
		t.Fatalf("expected 4 default subAgentConfigs, got %d", len(supervisor.subAgentConfigs))
	}
	for _, tt := range []RoleType{TaskTypeCoding, TaskTypeResearch, TaskTypeAdmin, TaskTypeGeneral} {
		if _, ok := supervisor.subAgentConfigs[tt]; !ok {
			t.Errorf("missing default subAgentConfig for %q", tt)
		}
	}
}

func TestRouteWithCfgAppliesModelOverlay(t *testing.T) {
	// Role "wizard" with Model="gpt-4" — after construction the config should
	// carry Model="gpt-4" which RouteWithCfg writes into cfg.ModelName.
	roles := []config.RoleConfig{
		{
			Name:     "wizard",
			Model:    "gpt-4",
			Keywords: []string{"code", "fix", "bug"},
			Priority: 30,
		},
		{
			Name:     "oracle",
			Keywords: nil,
			Priority: 100,
		},
	}

	llm := &supervisorMockLLM{responses: []*types.LLMResponse{
		{Content: "model overlay ok"},
	}}

	baseCfg := LoopConfig{
		LLM:       llm,
		Tools:     tools.NewRegistry(),
		Scrubber:  security.NewScrubber(),
		MaxTurns:  5,
		ModelName: "default-model",
	}

	supervisor := NewSupervisor(baseCfg, roles, nil)

	// Verify the subAgentConfig stores Model correctly.
	wizardCfg, ok := supervisor.subAgentConfigs["wizard"]
	if !ok {
		t.Fatal("missing subAgentConfig for 'wizard'")
	}
	if wizardCfg.Model != "gpt-4" {
		t.Fatalf("wizardCfg.Model = %q, want %q", wizardCfg.Model, "gpt-4")
	}

	// Route a coding message — "fix the bug" should match "wizard".
	exec, err := supervisor.RouteWithCfg(context.Background(), "fix the bug", baseCfg)
	if err != nil {
		t.Fatalf("RouteWithCfg() error = %v", err)
	}
	if exec.Response != "model overlay ok" {
		t.Errorf("unexpected response %q", exec.Response)
	}
}

func TestRouteWithCfgAppliesProviderOverlay(t *testing.T) {
	roles := []config.RoleConfig{
		{
			Name:     "wizard",
			Provider: "openai",
			Keywords: []string{"code", "implement"},
			Priority: 30,
		},
		{
			Name:     "oracle",
			Keywords: nil,
			Priority: 100,
		},
	}

	llm := &supervisorMockLLM{responses: []*types.LLMResponse{
		{Content: "provider overlay ok"},
	}}

	baseCfg := LoopConfig{
		LLM:          llm,
		Tools:        tools.NewRegistry(),
		Scrubber:     security.NewScrubber(),
		MaxTurns:     5,
		ProviderName: "default-provider",
	}

	supervisor := NewSupervisor(baseCfg, roles, nil)

	wizardCfg, ok := supervisor.subAgentConfigs["wizard"]
	if !ok {
		t.Fatal("missing subAgentConfig for 'wizard'")
	}
	if wizardCfg.Provider != "openai" {
		t.Fatalf("wizardCfg.Provider = %q, want %q", wizardCfg.Provider, "openai")
	}

	// Route a coding message — "implement a feature" should match "wizard".
	exec, err := supervisor.RouteWithCfg(context.Background(), "implement a feature", baseCfg)
	if err != nil {
		t.Fatalf("RouteWithCfg() error = %v", err)
	}
	if exec.Response != "provider overlay ok" {
		t.Errorf("unexpected response %q", exec.Response)
	}
}

func TestRouteWithCfgSwapsBackend(t *testing.T) {
	roles := []config.RoleConfig{
		{
			Name:     "wizard",
			Keywords: []string{"code", "fix", "bug"},
			Priority: 30,
		},
		{
			Name:     "oracle",
			Keywords: nil,
			Priority: 100,
		},
	}

	// This is the base LLM that should NOT be called for "wizard" routing.
	baseLLM := &supervisorMockLLM{responses: []*types.LLMResponse{
		{Content: "base-llm-should-not-be-used"},
	}}

	// This is the swapped backend that SHOULD be called.
	swappedBackend := &trackingMockLLM{}

	backends := map[RoleType]types.LLMClient{
		"wizard": swappedBackend,
	}

	baseCfg := LoopConfig{
		LLM:      baseLLM,
		Tools:    tools.NewRegistry(),
		Scrubber: security.NewScrubber(),
		MaxTurns: 5,
	}

	supervisor := NewSupervisor(baseCfg, roles, backends)

	exec, err := supervisor.RouteWithCfg(context.Background(), "fix the bug in my code", baseCfg)
	if err != nil {
		t.Fatalf("RouteWithCfg() error = %v", err)
	}

	// Verify swapped backend was called.
	if !swappedBackend.chatCalled.Load() {
		t.Error("expected swapped backend Chat() to be called, but it was not")
	}
	if exec.Response != "tracking-mock-response" {
		t.Errorf("expected response from swapped backend, got %q", exec.Response)
	}
}

func TestRouteWithCfgBackwardCompat(t *testing.T) {
	// nil roles + nil backends — should not panic and should work with defaults.
	llm := &supervisorMockLLM{responses: []*types.LLMResponse{
		{Content: "backward compat ok"},
	}}

	baseCfg := LoopConfig{
		LLM:      llm,
		Tools:    tools.NewRegistry(),
		Scrubber: security.NewScrubber(),
		MaxTurns: 5,
	}

	supervisor := NewSupervisor(baseCfg, nil, nil)

	// General message — should route to oracle (default fallback).
	exec, err := supervisor.RouteWithCfg(context.Background(), "hello how are you", baseCfg)
	if err != nil {
		t.Fatalf("RouteWithCfg() error = %v", err)
	}
	if exec.NextStep != FinalOutput {
		t.Errorf("expected FinalOutput, got %v", exec.NextStep)
	}
	if exec.Response != "backward compat ok" {
		t.Errorf("expected 'backward compat ok', got %q", exec.Response)
	}
}
