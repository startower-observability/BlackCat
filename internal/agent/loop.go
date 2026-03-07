package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	guardrailsPkg "github.com/startower-observability/blackcat/internal/guardrails"
	"github.com/startower-observability/blackcat/internal/hooks"
	"github.com/startower-observability/blackcat/internal/llm"
	"github.com/startower-observability/blackcat/internal/memory"
	"github.com/startower-observability/blackcat/internal/observability"
	"github.com/startower-observability/blackcat/internal/security"
	"github.com/startower-observability/blackcat/internal/skills"
	"github.com/startower-observability/blackcat/internal/tools"
	"github.com/startower-observability/blackcat/internal/types"
	"github.com/startower-observability/blackcat/internal/workspace"
)

const defaultMaxHistoryMessages = 20

type Loop struct {
	llm                types.LLMClient
	tools              *tools.Registry
	scrubber           *security.Scrubber
	memory             memory.Store
	skills             []skills.Skill
	hooks              *hooks.HookRegistry
	workspace          string
	maxTurns           int
	sessionMessages    []types.LLMMessage
	maxHistoryMessages int
	maxContextTokens   int
	compactor          *Compactor
	agentName          string
	sessionID          string
	agentLanguage      string
	agentTone          string
	modelName          string
	providerName       string
	channelType        string
	userID             string
	coreStore          *memory.CoreStore
	guardrails         *guardrailsPkg.Pipeline
	interruptMgr       *InterruptManager
	eventStream        chan<- AgentEvent
	traceID            string
	costTracker        *observability.CostTracker
	reflector          *Reflector
	prefMgr            *PreferenceManager
	planner            *Planner
	daemonStartedAt    time.Time
}

type LoopConfig struct {
	LLM                types.LLMClient
	Tools              *tools.Registry
	Scrubber           *security.Scrubber
	Memory             memory.Store
	Skills             []skills.Skill
	Hooks              *hooks.HookRegistry
	WorkspaceDir       string
	MaxTurns           int
	SessionMessages    []types.LLMMessage
	MaxHistoryMessages int
	MaxContextTokens   int
	MemoryFileStore    *memory.FileStore
	AgentName          string
	SessionID          string
	AgentLanguage      string
	AgentTone          string
	ModelName          string
	ProviderName       string
	ChannelType        string
	UserID             string
	CoreStore          *memory.CoreStore
	Guardrails         *guardrailsPkg.Pipeline
	EventStream        chan<- AgentEvent
	CostTracker        *observability.CostTracker // optional, nil skips recording
	Reflector          *Reflector                 // optional, nil disables self-reflection
	PrefManager        *PreferenceManager         // optional, nil disables adaptive preferences
	Planner            *Planner                   // optional, nil disables plan-and-execute
	DaemonStartedAt    time.Time                  // zero value acceptable; snapshot builder handles zero case
}

func NewLoop(cfg LoopConfig) *Loop {
	reg := cfg.Tools
	if reg == nil {
		reg = tools.NewRegistry()
	}

	scrubber := cfg.Scrubber
	if scrubber == nil {
		scrubber = security.NewScrubber()
	}

	maxTurns := cfg.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}

	maxHistoryMessages := cfg.MaxHistoryMessages
	if maxHistoryMessages <= 0 {
		maxHistoryMessages = defaultMaxHistoryMessages
	}

	if cfg.MaxContextTokens <= 0 {
		cfg.MaxContextTokens = 80000
	}

	var compactor *Compactor
	if cfg.LLM != nil {
		compactor = NewCompactor(CompactorConfig{
			LLM:         cfg.LLM,
			Memory:      cfg.MemoryFileStore,
			Threshold:   0.80,
			MaxTokens:   cfg.MaxContextTokens,
			MinMessages: 6,
		})
	}

	sessionMessages := cfg.SessionMessages
	if len(sessionMessages) > 0 {
		sessionMessages = append([]types.LLMMessage(nil), sessionMessages...)
	}

	return &Loop{
		llm:                cfg.LLM,
		tools:              reg,
		scrubber:           scrubber,
		memory:             cfg.Memory,
		skills:             cfg.Skills,
		hooks:              cfg.Hooks,
		workspace:          cfg.WorkspaceDir,
		maxTurns:           maxTurns,
		sessionMessages:    sessionMessages,
		maxHistoryMessages: maxHistoryMessages,
		maxContextTokens:   cfg.MaxContextTokens,
		compactor:          compactor,
		agentName:          cfg.AgentName,
		sessionID:          cfg.SessionID,
		agentLanguage:      cfg.AgentLanguage,
		agentTone:          cfg.AgentTone,
		modelName:          cfg.ModelName,
		providerName:       cfg.ProviderName,
		channelType:        cfg.ChannelType,
		userID:             cfg.UserID,
		coreStore:          cfg.CoreStore,
		guardrails:         cfg.Guardrails,
		interruptMgr:       NewInterruptManager(),
		eventStream:        cfg.EventStream,
		costTracker:        cfg.CostTracker,
		reflector:          cfg.Reflector,
		prefMgr:            cfg.PrefManager,
		planner:            cfg.Planner,
		daemonStartedAt:    cfg.DaemonStartedAt,
	}
}

// SelfKnowledgeProvider accessor methods — make Loop satisfy agentapi.SelfKnowledgeProvider.
func (l *Loop) GetAgentName() string                       { return l.agentName }
func (l *Loop) GetModelName() string                       { return l.modelName }
func (l *Loop) GetProviderName() string                    { return l.providerName }
func (l *Loop) GetChannelType() string                     { return l.channelType }
func (l *Loop) GetDaemonStartedAt() time.Time              { return l.daemonStartedAt }
func (l *Loop) GetActiveSkills() []skills.Skill            { return l.skills }
func (l *Loop) GetInactiveSkills() []skills.InactiveSkill  { return nil }
func (l *Loop) GetCostTracker() *observability.CostTracker { return l.costTracker }
func (l *Loop) GetUserID() string                          { return l.userID }

func (l *Loop) Run(ctx context.Context, userMessage string) (*Execution, error) {
	execution := NewExecution(l.maxTurns)
	l.traceID = observability.NewTraceID()

	systemPrompt, err := l.buildSystemPrompt(ctx)
	if err != nil {
		execution.Error = err
		execution.NextStep = Error
		return execution, err
	}

	execution.AddSystemMessage(systemPrompt)
	l.addSessionHistory(execution)

	// Plan-and-execute: for complex tasks, generate a structured plan and store
	// it on the execution. A plan summary is injected as a system message for the
	// LLM to follow step-by-step.
	if l.planner != nil && IsComplexTask(userMessage) {
		toolDefs := l.tools.List()
		plan, planErr := l.planner.GeneratePlan(ctx, userMessage, toolDefs)
		if planErr != nil {
			slog.Warn("plan generation failed, proceeding without plan", "err", planErr)
		} else if plan != nil {
			execution.Plan = plan
			l.emit(l.eventStream, AgentEvent{Kind: EventThinking, Message: "Generated execution plan: " + plan.Goal})
			planCtx := fmt.Sprintf("# Execution Plan\n%s\n\nFollow this plan step-by-step. Report progress after each step.", plan.Summary())
			execution.Messages = append(execution.Messages, types.LLMMessage{Role: "system", Content: planCtx})
		}
	}

	// Clarification: if the request is ambiguous, inject guidance asking the
	// agent to ask clarifying questions instead of acting blindly.
	if clarSection := ClarificationPromptSection(userMessage); clarSection != "" {
		execution.Messages = append(execution.Messages, types.LLMMessage{Role: "system", Content: clarSection})
		l.emit(l.eventStream, AgentEvent{Kind: EventThinking, Message: "Request appears ambiguous, will ask for clarification"})
	}

	execution.AddUserMessage(userMessage)

	// Proactive compaction: if messages approach context limit, compact first
	if l.compactor != nil && l.compactor.ShouldCompact(execution.Messages) {
		compacted, compactErr := l.compactor.Compact(ctx, execution.Messages)
		if compactErr != nil {
			slog.Warn("proactive compaction failed, proceeding with full context", "err", compactErr)
		} else {
			// Flush compacted-away facts to persistent memory
			if len(execution.Messages) > 1 && len(compacted) < len(execution.Messages) {
				toFlush := execution.Messages[1 : len(execution.Messages)-len(compacted)+1]
				if flushErr := l.compactor.FlushToMemory(ctx, toFlush); flushErr != nil {
					slog.Warn("memory flush failed", "err", flushErr)
				}
			}
			execution.Messages = compacted
			execution.Compacted = true
		}
	}

	toolDefs := l.tools.List()

	for {
		step := l.processOneTurn(ctx, execution, toolDefs)
		execution.NextStep = step

		switch step {
		case RunAgain:
			continue
		case FinalOutput, Interrupted, Handoff:
			return execution, nil
		case Error:
			return execution, execution.Error
		default:
			execution.Error = fmt.Errorf("unknown next step: %d", step)
			execution.NextStep = Error
			return execution, execution.Error
		}
	}
}

func (l *Loop) processOneTurn(ctx context.Context, execution *Execution, toolDefs []types.ToolDefinition) NextStep {
	span := observability.NewTurnSpan(l.traceID, execution.TurnCount, l.modelName, l.providerName)
	defer span.End(ctx)

	if l.hooks != nil {
		if err := l.fireHook(ctx, hooks.PreChat, &hooks.HookContext{Metadata: map[string]any{"messages": execution.Messages}}); err != nil {
			execution.Error = err
			span.Outcome = "error"
			span.ErrorMsg = err.Error()
			return Error
		}
	}

	if false {
		return Interrupted
	}

	if false {
		return Handoff
	}

	// Emit thinking event before LLM call
	l.emit(l.eventStream, AgentEvent{Kind: EventThinking, TurnNum: execution.TurnCount, Message: fmt.Sprintf("Thinking (turn %d)...", execution.TurnCount)})

	resp, err := llm.RetryChat(ctx, l.llm, execution.Messages, toolDefs, 3)
	if err != nil {
		execution.Error = err
		l.emit(l.eventStream, AgentEvent{Kind: EventError, TurnNum: execution.TurnCount, Error: err.Error()})
		span.Outcome = "error"
		span.ErrorMsg = err.Error()
		return Error
	}

	if l.hooks != nil {
		if err := l.fireHook(ctx, hooks.PostChat, &hooks.HookContext{LLMResponse: resp}); err != nil {
			execution.Error = err
			span.Outcome = "error"
			span.ErrorMsg = err.Error()
			return Error
		}
	}

	execution.TotalUsage.PromptTokens += resp.Usage.PromptTokens
	execution.TotalUsage.CompletionTokens += resp.Usage.CompletionTokens
	execution.TotalUsage.TotalTokens += resp.Usage.TotalTokens

	// Fallback: estimate tokens when the provider returns zero (e.g. GitHub Copilot)
	promptTokens := resp.Usage.PromptTokens
	completionTokens := resp.Usage.CompletionTokens
	if promptTokens == 0 && completionTokens == 0 {
		var inputText strings.Builder
		for _, m := range execution.Messages {
			inputText.WriteString(m.Content)
		}
		promptTokens = llm.EstimateTokens(inputText.String())
		completionTokens = llm.EstimateTokens(resp.Content)
		// Also update execution totals with estimates
		execution.TotalUsage.PromptTokens += promptTokens
		execution.TotalUsage.CompletionTokens += completionTokens
		execution.TotalUsage.TotalTokens += promptTokens + completionTokens
	}

	span.InputTokens = promptTokens
	span.OutputTokens = completionTokens

	// Record token cost if tracker is available
	if l.costTracker != nil {
		_ = l.costTracker.Record(ctx, l.userID, l.sessionID, l.modelName, l.providerName,
			promptTokens, completionTokens)
	}

	execution.TurnCount++

	if len(resp.ToolCalls) == 0 {
		execution.AddAssistantMessage(resp.Content, nil)
		execution.Response = resp.Content
		execution.Done = true
		l.emit(l.eventStream, AgentEvent{Kind: EventDone, TurnNum: execution.TurnCount, Result: execution.Response})
		span.Outcome = "final_output"
		return FinalOutput
	}

	execution.AddAssistantMessage(resp.Content, resp.ToolCalls)

	for _, call := range resp.ToolCalls {
		span.ToolCallCount++
		span.ToolNames = append(span.ToolNames, call.Name)
		// Emit tool call start event
		l.emit(l.eventStream, AgentEvent{Kind: EventToolCallStart, TurnNum: execution.TurnCount, ToolName: call.Name, ToolArgs: string(call.Arguments), Message: fmt.Sprintf("Calling %s...", call.Name)})

		if l.hooks != nil {
			hctx := &hooks.HookContext{ToolName: call.Name, Metadata: map[string]any{"args": call.Arguments}}
			if err := l.fireHook(ctx, hooks.PreToolExec, hctx); err != nil {
				continue
			}
		}

		if l.guardrails != nil {
			result := l.guardrails.CheckTool(call.Name, string(call.Arguments))
			if !result.Allow {
				if l.interruptMgr == nil {
					l.interruptMgr = NewInterruptManager()
				}
				pa := l.interruptMgr.CreateApproval(l.userID, call.Name, string(call.Arguments), result.Reason, defaultApprovalTimeout)
				execution.PendingApproval = pa
				l.emit(l.eventStream, AgentEvent{Kind: EventInterrupted, TurnNum: execution.TurnCount, Message: "Waiting for approval..."})
				span.Outcome = "interrupted"
				return Interrupted
			}
		}

		toolResult, err := l.tools.Execute(ctx, call.Name, call.Arguments)
		if err != nil {
			var valErr *tools.ValidationError
			if errors.As(err, &valErr) {
				execution.ToolRetryCount[call.ID]++
				if execution.ToolRetryCount[call.ID] < 2 {
					errorMsg := fmt.Sprintf("Validation error: %s. Please fix your arguments and try again.", valErr.Error())
					scrubbedMsg := l.scrubber.Scrub(errorMsg)
					execution.AddToolResult(call.ID, call.Name, scrubbedMsg)
					span.Outcome = "run_again"
					return RunAgain
				}
				execution.Error = fmt.Errorf("tool %q exceeded max retries: %w", call.Name, valErr)
				span.Outcome = "error"
				span.ErrorMsg = execution.Error.Error()
				return Error
			}
			slog.Warn("tool execution error, feeding back to LLM", "tool", call.Name, "err", err)
			toolResult = fmt.Sprintf("Tool error: %s", err.Error())
			// Self-reflection: critique the failure and store lesson
			if l.reflector != nil && execution.ReflectionCount < MaxReflections {
				lesson, reflErr := l.reflector.Reflect(ctx, l.userID, call.Name, string(call.Arguments), toolResult)
				if reflErr != nil {
					slog.WarnContext(ctx, "self-reflection failed", "tool", call.Name, "err", reflErr)
				} else if lesson != "" {
					slog.InfoContext(ctx, "self-reflection lesson", "tool", call.Name, "lesson", lesson)
				}
				execution.ReflectionCount++
			}
		}

		if l.hooks != nil {
			hctx := &hooks.HookContext{ToolName: call.Name, Metadata: map[string]any{"args": call.Arguments, "result": toolResult}}
			if err := l.fireHook(ctx, hooks.PostToolExec, hctx); err != nil {
				execution.Error = err
				span.Outcome = "error"
				span.ErrorMsg = err.Error()
				return Error
			}
		}

		scrubbedResult := l.scrubber.Scrub(toolResult)
		execution.AddToolResult(call.ID, call.Name, scrubbedResult)

		// Emit tool call result event
		l.emit(l.eventStream, AgentEvent{Kind: EventToolCallResult, TurnNum: execution.TurnCount, ToolName: call.Name, Result: truncate(scrubbedResult, 200)})

		if tool, getErr := l.tools.Get(call.Name); getErr == nil {
			execution.ToolMappings[call.ID] = tool
		}
	}

	if execution.TurnCount >= execution.MaxTurns {
		execution.Error = types.ErrMaxTurnsExceeded
		span.Outcome = "error"
		span.ErrorMsg = types.ErrMaxTurnsExceeded.Error()
		return Error
	}

	span.Outcome = "run_again"
	return RunAgain
}

func (l *Loop) addSessionHistory(execution *Execution) {
	if len(l.sessionMessages) == 0 {
		return
	}

	history := l.sessionMessages
	if len(history) > l.maxHistoryMessages {
		history = history[len(history)-l.maxHistoryMessages:]
	}

	for _, message := range history {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}

		execution.Messages = append(execution.Messages, types.LLMMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
}

func (l *Loop) fireHook(ctx context.Context, event hooks.HookEvent, hctx *hooks.HookContext) error {
	if l.hooks == nil {
		return nil
	}

	return l.hooks.Fire(ctx, event, hctx)
}

func (l *Loop) buildSystemPrompt(ctx context.Context) (string, error) {
	var sections []string

	// Persona section (first — sets agent identity)
	if l.agentName != "" {
		var persona strings.Builder
		persona.WriteString("# Agent Persona\n")
		persona.WriteString(fmt.Sprintf("Your name is %s.\n", l.agentName))
		if l.agentTone != "" {
			persona.WriteString(fmt.Sprintf("Your communication tone is %s.\n", l.agentTone))
		}
		if l.agentLanguage != "" {
			persona.WriteString(fmt.Sprintf("You communicate primarily in %s.\n", l.agentLanguage))
		}
		sections = append(sections, strings.TrimSpace(persona.String()))
	}

	agentsSections, err := l.loadAgentsMD()
	if err != nil {
		return "", err
	}
	if agentsSections != "" {
		sections = append(sections, "# AGENTS.md\n"+agentsSections)
	}

	if soulText, err := l.readWorkspaceFile("SOUL.md"); err != nil {
		return "", err
	} else if soulText != "" {
		sections = append(sections, "# SOUL.md\n"+soulText)
	}

	if identityText, err := l.readWorkspaceFile("IDENTITY.md"); err != nil {
		return "", err
	} else if identityText != "" {
		sections = append(sections, "# IDENTITY.md\n"+identityText)
	}

	if len(l.skills) > 0 {
		skillContext := skills.FormatForInjection(l.skills)
		if skillContext != "" {
			sections = append(sections, "# Skills\n"+skillContext)
		}
	}

	defs := l.tools.List()
	if len(defs) > 0 {
		var b strings.Builder
		b.WriteString("# Tools\n")
		for _, def := range defs {
			b.WriteString("- ")
			b.WriteString(def.Name)
			if def.Description != "" {
				b.WriteString(": ")
				b.WriteString(def.Description)
			}
			b.WriteByte('\n')
		}
		sections = append(sections, strings.TrimSpace(b.String()))
	}

	// Runtime context — agent self-awareness
	var runtime strings.Builder
	runtime.WriteString("# Runtime Context\n")
	runtime.WriteString(fmt.Sprintf("Current time: %s\n", time.Now().Format("2006-01-02 15:04:05 MST")))
	snap := BuildSelfKnowledgeSnapshot(ctx, l, false, nil)
	runtime.WriteString(snap.CompactSummary())
	if len(defs) > 0 {
		runtime.WriteString(fmt.Sprintf("\nAvailable tools: %d\n", len(defs)))
	}
	sections = append(sections, strings.TrimSpace(runtime.String()))

	if l.coreStore != nil {
		coreMemory, coreErr := l.coreStore.FormatForPrompt(ctx, l.userID)
		if coreErr == nil && coreMemory != "" {
			sections = append(sections, coreMemory)
		}
	}
	// Adaptive user preferences — injected after core memory.
	if l.prefMgr != nil {
		profile := l.prefMgr.LoadPreferences(ctx, l.userID)
		if prefBlock := FormatForPrompt(profile); prefBlock != "" {
			sections = append(sections, prefBlock)
		}
	}

	// Handling ambiguous requests — ask for clarification before executing.
	sections = append(sections, "# Handling Ambiguous Requests\nIf a user's request is ambiguous, incomplete, or could be interpreted in multiple ways, ask ONE concise clarifying question before taking action. Do NOT ask multiple questions. Do NOT ask for clarification on clear, straightforward requests.")

	// Final reinforcement — placed LAST for maximum LLM compliance.
	sections = append(sections, "# CRITICAL REMINDER\nYour responses MUST be 1-3 sentences. No lists. No menus. No capability dumps. Just answer the question directly.")

	if len(sections) == 0 {
		return "You are an autonomous AI agent. Do the work without asking questions. Use your tools proactively. Be concise and action-oriented.", nil
	}

	if l.maxContextTokens > 0 {
		// Remove sections in priority order until under budget.
		// Priority (remove first -> last): Core Memory, Skills, AGENTS.md
		// NEVER remove: Persona, SOUL.md, IDENTITY.md, Tools, Runtime Context, CRITICAL REMINDER.
		removablePrefixes := []string{"### Core Memory", "# Skills", "# AGENTS.md"}
		for _, prefix := range removablePrefixes {
			joined := strings.Join(sections, "\n\n")
			if llm.EstimateTokens(joined) <= l.maxContextTokens {
				break
			}

			for i, s := range sections {
				if strings.HasPrefix(s, prefix) {
					sections = append(sections[:i], sections[i+1:]...)
					slog.Warn("system prompt section removed for context window",
						"section", prefix,
						"estimated_tokens", llm.EstimateTokens(joined),
						"threshold", l.maxContextTokens,
					)
					break
				}
			}
		}
	}

	return strings.Join(sections, "\n\n"), nil
}

func (l *Loop) readWorkspaceFile(name string) (string, error) {
	if l.workspace == "" {
		return "", nil
	}

	path := filepath.Join(l.workspace, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", name, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// loadAgentsMD loads AGENTS.md content with hierarchical support and
// falls back to a single workspace AGENTS.md read for backward compatibility.
func (l *Loop) loadAgentsMD() (string, error) {
	if l.workspace == "" {
		return "", nil
	}

	workspaceDir := l.workspace
	entries, err := workspace.LoadHierarchicalAgents(workspaceDir, workspaceDir)
	if err != nil || len(entries) == 0 {
		return l.readWorkspaceFile("AGENTS.md")
	}

	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if cwdEntries, loadErr := workspace.LoadHierarchicalAgents(workspaceDir, cwd); loadErr == nil && len(cwdEntries) > len(entries) {
			entries = cwdEntries
		}
	}

	contents := make([]string, 0, len(entries))
	for _, entry := range entries {
		contents = append(contents, entry.Content)
	}

	merged := strings.Join(contents, "\n\n---\n\n")
	if strings.TrimSpace(merged) == "" {
		return l.readWorkspaceFile("AGENTS.md")
	}

	return merged, nil
}
