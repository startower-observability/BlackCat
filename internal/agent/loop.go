package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/startower-observability/blackcat/internal/hooks"
	"github.com/startower-observability/blackcat/internal/llm"
	"github.com/startower-observability/blackcat/internal/memory"
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
	agentLanguage      string
	agentTone          string
	modelName          string
	providerName       string
	channelType        string
	userID    string
	coreStore *memory.CoreStore
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
	AgentLanguage      string
	AgentTone          string
	ModelName          string
	ProviderName       string
	ChannelType        string
	UserID    string
	CoreStore *memory.CoreStore
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
		agentLanguage:      cfg.AgentLanguage,
		agentTone:          cfg.AgentTone,
		modelName:          cfg.ModelName,
		providerName:       cfg.ProviderName,
		channelType:        cfg.ChannelType,
		userID:    cfg.UserID,
		coreStore: cfg.CoreStore,
	}
}

func (l *Loop) Run(ctx context.Context, userMessage string) (*Execution, error) {
	execution := NewExecution(l.maxTurns)

	systemPrompt, err := l.buildSystemPrompt(ctx)
	if err != nil {
		execution.Error = err
		return execution, err
	}

	execution.AddSystemMessage(systemPrompt)
	l.addSessionHistory(execution)
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
		if l.hooks != nil {
			if err := l.fireHook(ctx, hooks.PreChat, &hooks.HookContext{Metadata: map[string]any{"messages": execution.Messages}}); err != nil {
				execution.Error = err
				return execution, err
			}
		}

		resp, err := llm.RetryChat(ctx, l.llm, execution.Messages, toolDefs, 3)
		if err != nil {
			execution.Error = err
			return execution, err
		}

		if l.hooks != nil {
			if err := l.fireHook(ctx, hooks.PostChat, &hooks.HookContext{LLMResponse: resp}); err != nil {
				execution.Error = err
				return execution, err
			}
		}

		execution.TotalUsage.PromptTokens += resp.Usage.PromptTokens
		execution.TotalUsage.CompletionTokens += resp.Usage.CompletionTokens
		execution.TotalUsage.TotalTokens += resp.Usage.TotalTokens

		execution.TurnCount++

		if len(resp.ToolCalls) == 0 {
			execution.AddAssistantMessage(resp.Content, nil)
			execution.Response = resp.Content
			execution.Done = true
			return execution, nil
		}

		execution.AddAssistantMessage(resp.Content, resp.ToolCalls)

		for _, call := range resp.ToolCalls {
			if l.hooks != nil {
				hctx := &hooks.HookContext{ToolName: call.Name, Metadata: map[string]any{"args": call.Arguments}}
				if err := l.fireHook(ctx, hooks.PreToolExec, hctx); err != nil {
					continue
				}
			}

			toolResult, err := l.tools.Execute(ctx, call.Name, call.Arguments)
			if err != nil {
				slog.Warn("tool execution error, feeding back to LLM", "tool", call.Name, "err", err)
				toolResult = fmt.Sprintf("Tool error: %s", err.Error())
			}

			if l.hooks != nil {
				hctx := &hooks.HookContext{ToolName: call.Name, Metadata: map[string]any{"args": call.Arguments, "result": toolResult}}
				if err := l.fireHook(ctx, hooks.PostToolExec, hctx); err != nil {
					execution.Error = err
					return execution, err
				}
			}

			scrubbedResult := l.scrubber.Scrub(toolResult)
			execution.AddToolResult(call.ID, call.Name, scrubbedResult)

			if tool, getErr := l.tools.Get(call.Name); getErr == nil {
				execution.ToolMappings[call.ID] = tool
			}
		}

		if execution.TurnCount >= execution.MaxTurns {
			execution.Error = types.ErrMaxTurnsExceeded
			return execution, types.ErrMaxTurnsExceeded
		}
	}
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
	if l.agentName != "" {
		runtime.WriteString(fmt.Sprintf("Agent: %s\n", l.agentName))
	}
	if l.modelName != "" {
		runtime.WriteString(fmt.Sprintf("LLM Model: %s\n", l.modelName))
	}
	if l.providerName != "" {
		runtime.WriteString(fmt.Sprintf("LLM Provider: %s\n", l.providerName))
	}
	if l.channelType != "" {
		runtime.WriteString(fmt.Sprintf("Channel: %s\n", l.channelType))
	}
	if len(l.skills) > 0 {
		skillNames := make([]string, len(l.skills))
		for i, s := range l.skills {
			skillNames[i] = s.Name
		}
		runtime.WriteString(fmt.Sprintf("Active skills: %s\n", strings.Join(skillNames, ", ")))
	}
	if len(defs) > 0 {
		runtime.WriteString(fmt.Sprintf("Available tools: %d\n", len(defs)))
	}
	sections = append(sections, strings.TrimSpace(runtime.String()))

	if l.coreStore != nil {
		coreMemory, coreErr := l.coreStore.FormatForPrompt(ctx, l.userID)
		if coreErr == nil && coreMemory != "" {
			sections = append(sections, coreMemory)
		}
	}

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
