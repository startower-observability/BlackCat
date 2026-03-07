package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/startower-observability/blackcat/internal/security"
	"github.com/startower-observability/blackcat/internal/tools"
	"github.com/startower-observability/blackcat/internal/types"
)

// wave4MockLLM is a mock LLM client for Wave 4 integration tests.
type wave4MockLLM struct {
	responses []*types.LLMResponse
	idx       int
	captured  [][]types.LLMMessage // captures messages sent to Chat for inspection
}

func (m *wave4MockLLM) Chat(ctx context.Context, msgs []types.LLMMessage, toolDefs []types.ToolDefinition) (*types.LLMResponse, error) {
	m.captured = append(m.captured, msgs)
	if m.idx >= len(m.responses) {
		return &types.LLMResponse{Content: "done"}, nil
	}
	r := m.responses[m.idx]
	m.idx++
	return r, nil
}

func (m *wave4MockLLM) Stream(ctx context.Context, msgs []types.LLMMessage, toolDefs []types.ToolDefinition) (<-chan types.Chunk, error) {
	ch := make(chan types.Chunk, 1)
	close(ch)
	return ch, nil
}

// wave4MockTool is a mock tool for Wave 4 integration tests.
type wave4MockTool struct {
	name      string
	result    string
	err       error
	params    json.RawMessage
	callCount int
}

func (m *wave4MockTool) Name() string                { return m.name }
func (m *wave4MockTool) Description() string         { return "wave4 mock tool" }
func (m *wave4MockTool) Parameters() json.RawMessage { return m.params }
func (m *wave4MockTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}

// wave4MockStore implements ArchivalStoreIface for testing reflection.
type wave4MockStore struct {
	inserted []string
}

func (s *wave4MockStore) InsertArchival(ctx context.Context, userID, content string, tags []string, embedding []float32) error {
	s.inserted = append(s.inserted, content)
	return nil
}

// ---------------------------------------------------------------------------
// Existing tests (T1-T5 era)
// ---------------------------------------------------------------------------

// TestWave4_Reflection_NilLLM_NoOp verifies that when the Reflector has a nil
// LLM, the agent loop runs without panic and produces final output normally.
func TestWave4_Reflection_NilLLM_NoOp(t *testing.T) {
	ctx := context.Background()

	llm := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "reflection test done"},
	}}

	// Reflector with nil LLM — should no-op gracefully
	reflector := NewReflector(nil, nil)

	loop := NewLoop(LoopConfig{
		LLM:       llm,
		Scrubber:  security.NewScrubber(),
		MaxTurns:  10,
		Reflector: reflector,
	})

	execution, err := loop.Run(ctx, "hello with reflection")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Fatalf("NextStep = %d, want FinalOutput (%d)", execution.NextStep, FinalOutput)
	}
	if execution.Response != "reflection test done" {
		t.Fatalf("Response = %q, want %q", execution.Response, "reflection test done")
	}
}

// TestWave4_Supervisor_ClassifiesCoding verifies that ClassifyMessage correctly
// classifies a coding-related message as TaskTypeCoding.
func TestWave4_Supervisor_ClassifiesCoding(t *testing.T) {
	got := ClassifyMessage("fix the bug in my code", nil)
	if got != TaskTypeCoding {
		t.Fatalf("ClassifyMessage('fix the bug in my code') = %q, want %q", got, TaskTypeCoding)
	}
}

// TestWave4_Supervisor_ClassifiesAdmin verifies that ClassifyMessage correctly
// classifies an admin-related message as TaskTypeAdmin.
func TestWave4_Supervisor_ClassifiesAdmin(t *testing.T) {
	got := ClassifyMessage("restart the service", nil)
	if got != TaskTypeAdmin {
		t.Fatalf("ClassifyMessage('restart the service') = %q, want %q", got, TaskTypeAdmin)
	}
}

// TestWave4_Planner_ShouldPlan_MultiStep verifies that Planner.ShouldPlan
// returns true for a multi-step message containing sequential keywords.
func TestWave4_Planner_ShouldPlan_MultiStep(t *testing.T) {
	planner := NewPlanner(nil) // nil LLM is fine — ShouldPlan doesn't call LLM
	got := planner.ShouldPlan("First do A. Then do B. Finally do C.")
	if !got {
		t.Fatal("ShouldPlan('First do A. Then do B. Finally do C.') = false, want true")
	}
}

// TestWave4_AdaptivePrefs_NilManager_NoOp verifies that NewPreferenceManager(nil)
// followed by LoadPreferences returns a default AdaptiveProfile with all fields
// set to "auto", without panicking.
func TestWave4_AdaptivePrefs_NilManager_NoOp(t *testing.T) {
	ctx := context.Background()
	pm := NewPreferenceManager(nil)

	profile := pm.LoadPreferences(ctx, "user-123")

	if profile.Language != "auto" {
		t.Fatalf("Language = %q, want %q", profile.Language, "auto")
	}
	if profile.Style != "auto" {
		t.Fatalf("Style = %q, want %q", profile.Style, "auto")
	}
	if profile.Verbosity != "auto" {
		t.Fatalf("Verbosity = %q, want %q", profile.Verbosity, "auto")
	}
	if profile.TechnicalDepth != "auto" {
		t.Fatalf("TechnicalDepth = %q, want %q", profile.TechnicalDepth, "auto")
	}
}

// ---------------------------------------------------------------------------
// T6 integration tests — comprehensive Wave 4 verification
// ---------------------------------------------------------------------------

// TestWave4_Planner_LoopInjectsPlan verifies that when a complex message is
// sent through the loop with a Planner that has a working LLM, a plan is
// generated and stored on the execution, and a plan system message appears
// in the messages sent to the LLM.
func TestWave4_Planner_LoopInjectsPlan(t *testing.T) {
	ctx := context.Background()

	planJSON := `{"goal":"multi-step task","steps":[{"index":1,"description":"step one"},{"index":2,"description":"step two"}]}`

	// First call generates the plan, second call is the actual agent response
	plannerLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: planJSON},               // plan generation call
		{Content: "executing multi-step"}, // agent loop call
	}}

	planner := NewPlanner(plannerLLM)

	// Need a separate LLM for the main loop (planner uses its own)
	loopLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "completed the multi-step task"},
	}}

	loop := NewLoop(LoopConfig{
		LLM:      loopLLM,
		Scrubber: security.NewScrubber(),
		MaxTurns: 10,
		Planner:  planner,
	})

	// Use a message that triggers IsComplexTask (multiple sentences with sequence keywords)
	execution, err := loop.Run(ctx, "First create the database schema. Then seed test data. Finally run the migration.")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Fatalf("NextStep = %d, want FinalOutput (%d)", execution.NextStep, FinalOutput)
	}
	if execution.Plan == nil {
		t.Fatal("execution.Plan is nil, want non-nil plan")
	}
	if execution.Plan.Goal != "multi-step task" {
		t.Fatalf("Plan.Goal = %q, want %q", execution.Plan.Goal, "multi-step task")
	}
	if len(execution.Plan.Steps) != 2 {
		t.Fatalf("Plan.Steps count = %d, want 2", len(execution.Plan.Steps))
	}

	// Verify plan context was injected as system message to the loop LLM
	if len(loopLLM.captured) == 0 {
		t.Fatal("loopLLM.captured is empty, expected at least one Chat call")
	}
	found := false
	for _, msg := range loopLLM.captured[0] {
		if msg.Role == "system" && strings.Contains(msg.Content, "Execution Plan") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no system message with 'Execution Plan' found in loop LLM messages")
	}
}

// TestWave4_Planner_SimpleMessage_NoPlan verifies that a simple message does
// not trigger plan generation even when a Planner is configured.
func TestWave4_Planner_SimpleMessage_NoPlan(t *testing.T) {
	ctx := context.Background()

	plannerLLM := &wave4MockLLM{responses: []*types.LLMResponse{}}
	planner := NewPlanner(plannerLLM)

	loopLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "hello back"},
	}}

	loop := NewLoop(LoopConfig{
		LLM:      loopLLM,
		Scrubber: security.NewScrubber(),
		MaxTurns: 10,
		Planner:  planner,
	})

	execution, err := loop.Run(ctx, "What is the weather today?")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.Plan != nil {
		t.Fatalf("execution.Plan = %+v, want nil for simple message", execution.Plan)
	}
	// Planner LLM should not have been called
	if len(plannerLLM.captured) != 0 {
		t.Fatalf("plannerLLM.captured has %d calls, want 0", len(plannerLLM.captured))
	}
}

// TestWave4_Clarification_AmbiguousInjectsSystemMessage verifies that sending
// an ambiguous message through the loop causes a clarification system message
// to be injected into the LLM context.
func TestWave4_Clarification_AmbiguousInjectsSystemMessage(t *testing.T) {
	ctx := context.Background()

	llm := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "What would you like me to fix?"},
	}}

	events := make(chan AgentEvent, 16)

	loop := NewLoop(LoopConfig{
		LLM:         llm,
		Scrubber:    security.NewScrubber(),
		MaxTurns:    10,
		EventStream: events,
	})

	execution, err := loop.Run(ctx, "fix it")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Fatalf("NextStep = %d, want FinalOutput", execution.NextStep)
	}

	// Verify clarification system message was injected
	if len(llm.captured) == 0 {
		t.Fatal("llm.captured is empty")
	}
	found := false
	for _, msg := range llm.captured[0] {
		if msg.Role == "system" && strings.Contains(msg.Content, "Clarification Required") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no system message with 'Clarification Required' found in LLM messages")
	}

	// Verify thinking event was emitted
	close(events)
	foundEvent := false
	for ev := range events {
		if ev.Kind == EventThinking && strings.Contains(ev.Message, "ambiguous") {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Fatal("no EventThinking with 'ambiguous' message found in events")
	}
}

// TestWave4_Clarification_ClearMessage_NoClarification verifies that a clear,
// specific request does not trigger clarification injection.
func TestWave4_Clarification_ClearMessage_NoClarification(t *testing.T) {
	ctx := context.Background()

	llm := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "Done, I fixed the login bug."},
	}}

	loop := NewLoop(LoopConfig{
		LLM:      llm,
		Scrubber: security.NewScrubber(),
		MaxTurns: 10,
	})

	execution, err := loop.Run(ctx, "Fix the authentication bug in the login handler at line 42")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Fatalf("NextStep = %d, want FinalOutput", execution.NextStep)
	}

	// Verify NO clarification system message was injected
	if len(llm.captured) == 0 {
		t.Fatal("llm.captured is empty")
	}
	for _, msg := range llm.captured[0] {
		if msg.Role == "system" && strings.Contains(msg.Content, "Clarification Required") {
			t.Fatal("found 'Clarification Required' system message for a clear request")
		}
	}
}

// TestWave4_Reflector_ToolFailure_StoresLesson verifies that when a tool fails,
// the reflector runs and stores a lesson in the mock archival store, and the
// reflection count is incremented on the execution.
func TestWave4_Reflector_ToolFailure_StoresLesson(t *testing.T) {
	ctx := context.Background()

	registry := tools.NewRegistry()
	registry.Register(&wave4MockTool{
		name:   "failing_tool",
		result: "",
		err:    fmt.Errorf("connection refused"),
		params: json.RawMessage(`{"type":"object"}`),
	})

	// LLM calls: 1st returns tool call, 2nd returns final text
	llm := &wave4MockLLM{responses: []*types.LLMResponse{
		{ToolCalls: []types.ToolCall{{ID: "call-1", Name: "failing_tool", Arguments: json.RawMessage(`{}`)}}},
		{Content: "I see the tool failed, let me try another approach"},
	}}

	// Reflector LLM returns a lesson
	reflectorLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "Always verify network connectivity before calling remote tools."},
	}}
	store := &wave4MockStore{}
	reflector := NewReflector(reflectorLLM, store)

	loop := NewLoop(LoopConfig{
		LLM:       llm,
		Tools:     registry,
		Scrubber:  security.NewScrubber(),
		MaxTurns:  10,
		Reflector: reflector,
		UserID:    "test-user",
	})

	execution, err := loop.Run(ctx, "run the failing tool")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Fatalf("NextStep = %d, want FinalOutput", execution.NextStep)
	}
	if execution.ReflectionCount != 1 {
		t.Fatalf("ReflectionCount = %d, want 1", execution.ReflectionCount)
	}
	// Verify lesson was stored
	if len(store.inserted) == 0 {
		t.Fatal("store.inserted is empty, expected at least one reflection lesson")
	}
	if !strings.Contains(store.inserted[0], "network connectivity") {
		t.Fatalf("stored lesson = %q, want substring 'network connectivity'", store.inserted[0])
	}
}

// TestWave4_AllComponentsWired_FullPipeline verifies the full Wave 4 pipeline:
// Planner + Clarification + Reflector + AdaptivePrefs all wired into one loop.
// Uses a complex AND ambiguous message — both planner and clarification fire.
func TestWave4_AllComponentsWired_FullPipeline(t *testing.T) {
	ctx := context.Background()

	// This message is complex (multi-sentence with sequence keywords) AND ambiguous (contains "stuff")
	msg := "First do something with the stuff. Then update it. Finally fix everything."

	// Verify our assumptions about the detection functions
	if !IsComplexTask(msg) {
		t.Fatal("IsComplexTask should return true for the test message")
	}
	if !IsAmbiguous(msg) {
		t.Fatal("IsAmbiguous should return true for the test message")
	}

	// Planner LLM returns a valid plan
	planJSON := `{"goal":"do stuff","steps":[{"index":1,"description":"handle stuff"}]}`
	plannerLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: planJSON},
	}}
	planner := NewPlanner(plannerLLM)

	// Reflector with nil LLM — will no-op gracefully
	reflector := NewReflector(nil, nil)

	// PrefManager with nil store — returns defaults
	prefMgr := NewPreferenceManager(nil)

	// Main loop LLM
	loopLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "I have some questions about what you need..."},
	}}

	events := make(chan AgentEvent, 32)

	loop := NewLoop(LoopConfig{
		LLM:         loopLLM,
		Scrubber:    security.NewScrubber(),
		MaxTurns:    10,
		Planner:     planner,
		Reflector:   reflector,
		PrefManager: prefMgr,
		UserID:      "user-full-pipeline",
		EventStream: events,
	})

	execution, err := loop.Run(ctx, msg)
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Fatalf("NextStep = %d, want FinalOutput", execution.NextStep)
	}

	// Plan should be set (complex task triggers planner)
	if execution.Plan == nil {
		t.Fatal("execution.Plan is nil, want non-nil")
	}

	// Verify both plan and clarification system messages in LLM context
	if len(loopLLM.captured) == 0 {
		t.Fatal("loopLLM.captured is empty")
	}
	var hasPlan, hasClarification bool
	for _, msg := range loopLLM.captured[0] {
		if msg.Role == "system" && strings.Contains(msg.Content, "Execution Plan") {
			hasPlan = true
		}
		if msg.Role == "system" && strings.Contains(msg.Content, "Clarification Required") {
			hasClarification = true
		}
	}
	if !hasPlan {
		t.Fatal("missing 'Execution Plan' system message")
	}
	if !hasClarification {
		t.Fatal("missing 'Clarification Required' system message")
	}

	// Verify events include both thinking events
	close(events)
	var planEvent, clarEvent bool
	for ev := range events {
		if ev.Kind == EventThinking && strings.Contains(ev.Message, "execution plan") {
			planEvent = true
		}
		if ev.Kind == EventThinking && strings.Contains(ev.Message, "ambiguous") {
			clarEvent = true
		}
	}
	if !planEvent {
		t.Fatal("missing EventThinking for plan generation")
	}
	if !clarEvent {
		t.Fatal("missing EventThinking for ambiguity detection")
	}
}

// TestWave4_AllNil_LoopStillWorks verifies that when ALL optional Wave 4
// components are nil (Planner, Reflector, PrefManager, EventStream), the loop
// still runs and returns the expected response.
func TestWave4_AllNil_LoopStillWorks(t *testing.T) {
	ctx := context.Background()

	llm := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "all nil, still works"},
	}}

	loop := NewLoop(LoopConfig{
		LLM:         llm,
		Scrubber:    security.NewScrubber(),
		MaxTurns:    10,
		Planner:     nil,
		Reflector:   nil,
		PrefManager: nil,
		EventStream: nil,
	})

	execution, err := loop.Run(ctx, "First do A. Then do B. Finally do C.")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}
	if execution.NextStep != FinalOutput {
		t.Fatalf("NextStep = %d, want FinalOutput", execution.NextStep)
	}
	if execution.Response != "all nil, still works" {
		t.Fatalf("Response = %q, want %q", execution.Response, "all nil, still works")
	}
	if execution.Plan != nil {
		t.Fatalf("Plan = %+v, want nil when Planner is nil", execution.Plan)
	}
}

// TestWave4_EventStream_PlanAndClarificationEvents verifies that the event
// stream receives EventThinking events for both plan generation and clarification.
func TestWave4_EventStream_PlanAndClarificationEvents(t *testing.T) {
	ctx := context.Background()

	planJSON := `{"goal":"test events","steps":[{"index":1,"description":"only step"}]}`
	plannerLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: planJSON},
	}}

	loopLLM := &wave4MockLLM{responses: []*types.LLMResponse{
		{Content: "event test done"},
	}}

	events := make(chan AgentEvent, 32)

	loop := NewLoop(LoopConfig{
		LLM:         loopLLM,
		Scrubber:    security.NewScrubber(),
		MaxTurns:    10,
		Planner:     NewPlanner(plannerLLM),
		EventStream: events,
	})

	// Complex + ambiguous: "First do something. Then fix it."
	_, err := loop.Run(ctx, "First do something with stuff. Then fix it.")
	if err != nil {
		t.Fatalf("loop.Run() error = %v", err)
	}

	close(events)
	var kinds []EventKind
	for ev := range events {
		kinds = append(kinds, ev.Kind)
	}

	// Should have at least: thinking (plan), thinking (clarification), thinking (LLM call), done
	thinkingCount := 0
	for _, k := range kinds {
		if k == EventThinking {
			thinkingCount++
		}
	}
	if thinkingCount < 2 {
		t.Fatalf("EventThinking count = %d, want >= 2 (plan + clarification)", thinkingCount)
	}
}

// TestWave4_Supervisor_AllTaskTypes verifies that ClassifyMessage correctly
// classifies messages for all four task types.
func TestWave4_Supervisor_AllTaskTypes(t *testing.T) {
	tests := []struct {
		msg  string
		want TaskType
	}{
		{"write a function to parse JSON", TaskTypeCoding},
		{"search for information about Go generics", TaskTypeResearch},
		{"restart the server and check the health status", TaskTypeAdmin},
		{"hello, how are you today?", TaskTypeGeneral},
	}
	for _, tt := range tests {
		got := ClassifyMessage(tt.msg, nil)
		if got != tt.want {
			t.Errorf("ClassifyMessage(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
}

// TestWave4_FormatPlanForPrompt_IntegrationWithParse verifies the roundtrip:
// parse JSON -> format for prompt -> contains expected content.
func TestWave4_FormatPlanForPrompt_IntegrationWithParse(t *testing.T) {
	jsonStr := `{"goal":"build feature","steps":[{"index":1,"description":"create module","status":"pending"},{"index":2,"description":"write tests","status":"pending"}]}`
	plan, err := parsePlanJSON(jsonStr)
	if err != nil {
		t.Fatalf("parsePlanJSON error = %v", err)
	}
	formatted := FormatPlanForPrompt(plan)
	if !strings.Contains(formatted, "build feature") {
		t.Fatalf("formatted plan missing goal, got: %s", formatted)
	}
	if !strings.Contains(formatted, "create module") {
		t.Fatalf("formatted plan missing step 1, got: %s", formatted)
	}
	if !strings.Contains(formatted, "write tests") {
		t.Fatalf("formatted plan missing step 2, got: %s", formatted)
	}
}
