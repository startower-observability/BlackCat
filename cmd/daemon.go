package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/startower-observability/blackcat/internal/agent"
	"github.com/startower-observability/blackcat/internal/channel"
	"github.com/startower-observability/blackcat/internal/channel/discord"
	"github.com/startower-observability/blackcat/internal/channel/telegram"
	"github.com/startower-observability/blackcat/internal/channel/whatsapp"
	"github.com/startower-observability/blackcat/internal/config"
	daemonruntime "github.com/startower-observability/blackcat/internal/daemon"
	"github.com/startower-observability/blackcat/internal/dashboard"
	"github.com/startower-observability/blackcat/internal/eventlog"
	guardrailsPkg "github.com/startower-observability/blackcat/internal/guardrails"
	"github.com/startower-observability/blackcat/internal/hooks"
	"github.com/startower-observability/blackcat/internal/llm"
	"github.com/startower-observability/blackcat/internal/llm/antigravity"
	"github.com/startower-observability/blackcat/internal/llm/copilot"
	"github.com/startower-observability/blackcat/internal/llm/gemini"
	"github.com/startower-observability/blackcat/internal/llm/zen"
	"github.com/startower-observability/blackcat/internal/mcp"
	"github.com/startower-observability/blackcat/internal/memory"
	"github.com/startower-observability/blackcat/internal/oauth"
	"github.com/startower-observability/blackcat/internal/observability"
	"github.com/startower-observability/blackcat/internal/opencode"
	"github.com/startower-observability/blackcat/internal/ratelimit"
	"github.com/startower-observability/blackcat/internal/rules"
	"github.com/startower-observability/blackcat/internal/scheduler"
	"github.com/startower-observability/blackcat/internal/security"
	"github.com/startower-observability/blackcat/internal/session"
	"github.com/startower-observability/blackcat/internal/skills"
	"github.com/startower-observability/blackcat/internal/taskqueue"
	"github.com/startower-observability/blackcat/internal/tools"
	"github.com/startower-observability/blackcat/internal/transcription"
	"github.com/startower-observability/blackcat/internal/types"
	"github.com/startower-observability/blackcat/internal/workspace"
)

var daemonWorkers int

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start BlackCat as a long-running daemon with all channels",
	Long: `daemon starts BlackCat as a long-lived process that listens on
configured messaging channels (Telegram, Discord, WhatsApp), routes messages
through the agent loop, and delegates coding tasks to OpenCode.`,
	RunE: runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().IntVar(&daemonWorkers, "workers", 3, "Maximum concurrent message processors")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg.Validate()
	if err := validateStartupConfig(cfg); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}
	if validationErr := config.ValidateDeep(cfg); validationErr != nil {
		return fmt.Errorf("config validation failed: %w", validationErr)
	}

	var rateLimiter *ratelimit.Limiter
	if cfg.RateLimit.Enabled {
		maxReq := cfg.RateLimit.MaxRequests
		if maxReq <= 0 {
			maxReq = 10
		}
		windowSec := cfg.RateLimit.WindowSeconds
		if windowSec <= 0 {
			windowSec = 60
		}
		rateLimiter = ratelimit.NewLimiter(maxReq, windowSec)
		slog.Info("rate limiter enabled", "max_requests", maxReq, "window_seconds", windowSec)
	}

	setupLogger(cfg.Logging)

	denyList, err := buildDenyList(cfg.Security.DenyPatterns)
	if err != nil {
		return err
	}
	scrubber := security.NewScrubber()
	if len(cfg.Security.DenyPatterns) > 0 {
		slog.Info("security deny patterns loaded", "count", len(cfg.Security.DenyPatterns))
	}

	// Build guardrails pipeline from config
	guardrailsCfg := guardrailsPkg.GuardrailsConfig{
		InputEnabled:            cfg.Security.Guardrails.Input.Enabled,
		ToolEnabled:             cfg.Security.Guardrails.Tool.Enabled,
		OutputEnabled:           cfg.Security.Guardrails.Output.Enabled,
		CustomInputPatterns:     cfg.Security.Guardrails.Input.CustomPatterns,
		RequireApprovalPatterns: cfg.Security.Guardrails.Tool.RequireApprovalPatterns,
	}
	guardrailsPipeline := guardrailsPkg.NewPipeline(guardrailsCfg)
	slog.Info("guardrails pipeline initialized",
		"input_enabled", guardrailsCfg.InputEnabled,
		"tool_enabled", guardrailsCfg.ToolEnabled,
		"output_enabled", guardrailsCfg.OutputEnabled,
	)

	fileMemStore := memory.NewFileStore(cfg.Memory.FilePath)
	var memStore memory.Store = fileMemStore
	var sqliteMemStore *memory.SQLiteStore
	{
		homeDir, _ := os.UserHomeDir()
		dbPath := filepath.Join(homeDir, ".blackcat", "memory.db")
		if sqlStore, sqlErr := memory.NewSQLiteStore(dbPath); sqlErr != nil {
			slog.Warn("sqlite memory store unavailable, using file store", "err", sqlErr)
		} else {
			sqliteMemStore = sqlStore
			memStore = sqlStore
			if count, _ := sqlStore.Count(context.Background()); count == 0 {
				if imported, migrateErr := sqlStore.MigrateFromFileStore(context.Background(), fileMemStore); migrateErr != nil {
					slog.Warn("memory migration failed", "err", migrateErr)
				} else if imported > 0 {
					slog.Info("migrated memories from file to sqlite", "count", imported)
				}
			}
		}
	}

	// Create embedding client (nil if no API key configured)
	var embedClient *memory.EmbeddingClient
	if cfg.Memory.Embedding.APIKey != "" {
		embedClient = memory.NewEmbeddingClient(cfg.Memory.Embedding.APIKey, cfg.Memory.Embedding.BaseURL, cfg.Memory.Embedding.Model)
		slog.Info("embedding client initialized", "model", cfg.Memory.Embedding.Model)
	}

	// Create core memory store (shares the SQLite DB)
	var coreStore *memory.CoreStore
	if sqliteMemStore != nil {
		coreStore = memory.NewCoreStore(sqliteMemStore.DB())
	}

	// Create cost tracker (shares the SQLite DB)
	var costTracker *observability.CostTracker
	if sqliteMemStore != nil {
		var costErr error
		costTracker, costErr = observability.NewCostTracker(sqliteMemStore.DB())
		if costErr != nil {
			slog.Warn("cost tracker unavailable", "error", costErr)
		} else {
			slog.Info("cost tracking enabled")
		}
	}

	// Migrate MEMORY.md to archival (idempotent)
	if sqliteMemStore != nil && cfg.Memory.FilePath != "" {
		if migrated, migrErr := memory.MigrateFromMemoryMD(ctx, cfg.Memory.FilePath, sqliteMemStore, embedClient, "default"); migrErr != nil {
			slog.Warn("MEMORY.md archival migration failed", "err", migrErr)
		} else if migrated > 0 {
			slog.Info("migrated MEMORY.md entries to archival", "count", migrated)
		}
	}

	// Create task queue and event logger for background task processing
	var tq *taskqueue.TaskQueue
	{
		homeDir, _ := os.UserHomeDir()
		tqDBPath := filepath.Join(homeDir, ".blackcat", "tasks.db")
		evLogPath := filepath.Join(homeDir, ".blackcat", "events.log")

		var eventLogger *eventlog.EventLogger
		if el, elErr := eventlog.New(evLogPath); elErr != nil {
			slog.Warn("event logger unavailable", "err", elErr)
		} else {
			eventLogger = el
			defer eventLogger.Close()
		}

		if q, tqErr := taskqueue.New(tqDBPath); tqErr != nil {
			slog.Warn("task queue unavailable", "err", tqErr)
		} else {
			tq = q
			if eventLogger != nil {
				tq.SetEventLogger(eventLogger)
			}
			defer tq.Shutdown()
		}
	}

	// Create session store
	sessionStoreDir := cfg.Session.StoreDir
	if sessionStoreDir == "" {
		homeDir, _ := os.UserHomeDir()
		sessionStoreDir = filepath.Join(homeDir, ".blackcat", "sessions")
	}
	var sessionStore *session.FileStore
	if cfg.Session.Enabled {
		var sessionErr error
		sessionStore, sessionErr = session.NewFileStore(sessionStoreDir, cfg.Session.MaxHistory)
		if sessionErr != nil {
			slog.Warn("session store init failed, sessions disabled", "err", sessionErr)
			sessionStore = nil
		}
	}
	llmClient := llm.NewClient(
		cfg.LLM.APIKey,
		cfg.LLM.BaseURL,
		cfg.LLM.Model,
		cfg.LLM.Temperature,
		cfg.LLM.MaxTokens,
	)

	// --- Phase 2: Wire new LLM providers based on config ---
	// Register all built-in backend factories
	llm.RegisterBackend("openai", llm.NewOpenAIBackend)
	llm.RegisterBackend("copilot", func(bc llm.BackendConfig) (llm.Backend, error) {
		return copilot.NewCopilotBackend(bc)
	})
	llm.RegisterBackend("gemini", func(bc llm.BackendConfig) (llm.Backend, error) {
		return gemini.NewGeminiBackend(bc)
	})
	llm.RegisterBackend("zen", func(bc llm.BackendConfig) (llm.Backend, error) {
		return zen.NewZenBackend(bc)
	})

	// Try to create a Phase 2 backend if any new provider is enabled
	var activeBackend llm.Backend
	var activeProviderName, activeModelName string
	if backend, name := createActiveBackend(cfg); backend != nil {
		activeBackend = backend
		activeProviderName = name
		switch name {
		case "openai":
			activeModelName = cfg.Providers.OpenAI.Model
		case "copilot":
			activeModelName = cfg.Providers.Copilot.Model
		case "antigravity":
			activeModelName = cfg.Providers.Antigravity.Model
		case "gemini":
			activeModelName = cfg.Providers.Gemini.Model
		case "zen":
			activeModelName = cfg.Providers.Zen.Model
		}
		slog.Info("phase 2 backend activated", "provider", name, "model", activeModelName)
	}
	if activeProviderName == "" {
		activeProviderName = cfg.LLM.Provider
		activeModelName = cfg.LLM.Model
	}

	workspaceDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine workspace dir: %w", err)
	}
	slog.Info("workspace initialized", "dir", workspaceDir, "templates", len(workspace.ListTemplates()))
	// Hot reload: AGENTS.md, SOUL.md, IDENTITY.md are re-read from disk on every
	// message via readWorkspaceFile(). No restart needed for workspace file changes.

	registry := tools.NewRegistry()
	registry.Register(tools.NewExecTool(denyList, workspaceDir, 0))
	registry.Register(tools.NewFilesystemTool(workspaceDir))
	registry.Register(tools.NewWebTool(0))
	if cfgFile != "" {
		registry.Register(tools.NewConfigUpdateTool(cfgFile))
	}

	var openCodeOpts []opencode.ClientOption
	if cfg.OpenCode.Password != "" {
		openCodeOpts = append(openCodeOpts, opencode.WithPassword(cfg.OpenCode.Password))
	}
	openCodeClient := opencode.NewClient(cfg.OpenCode.Addr, openCodeOpts...)
	registry.Register(tools.NewOpenCodeTool(openCodeClient, cfg.Security.AutoPermit, cfg.OpenCode.Timeout.Duration))
	registry.Register(tools.NewOpenCodeStatusTool(openCodeClient))
	if tq != nil {
		registry.Register(tools.NewOpenCodeTaskAsyncTool(openCodeClient, tq, cfg.Security.AutoPermit, cfg.OpenCode.Timeout.Duration))
	}
	if sqliteMemStore != nil {
		registry.Register(tools.NewMemoryTool(sqliteMemStore))
	}
	// Register three-tier memory tools (core + archival)
	if sqliteMemStore != nil {
		tools.RegisterMemoryTools(registry, coreStore, sqliteMemStore, embedClient, "default")
		slog.Info("memory tools registered (core + archival)")
	}
	if costTracker != nil {
		registry.Register(tools.NewUsageTool(costTracker))
	}
	if cfgFile != "" {
		registry.Register(tools.NewSchedulerTool(cfgFile))
	}

	mcpClient := mcp.NewClient()
	for _, serverCfg := range cfg.MCP.Servers {
		before := toolNameSet(mcpClient.Tools())
		if connectErr := mcpClient.Connect(ctx, serverCfg); connectErr != nil {
			slog.Warn("failed to connect mcp server", "server", serverCfg.Name, "err", connectErr)
			continue
		}

		registered := 0
		for _, def := range mcpClient.Tools() {
			if _, exists := before[def.Name]; exists {
				continue
			}

			proxyName := fmt.Sprintf("mcp_%s_%s", serverCfg.Name, def.Name)
			registry.Register(&mcpProxyTool{
				name:        proxyName,
				description: fmt.Sprintf("Proxy to MCP tool %s on server %s", def.Name, serverCfg.Name),
				parameters:  def.Parameters,
				serverName:  serverCfg.Name,
				toolName:    def.Name,
				client:      mcpClient,
			})
			registered++
		}

		slog.Info("mcp server connected", "server", serverCfg.Name, "proxy_tools", registered)
	}

	skillDirs := []string{cfg.Skills.Dir}
	if cfg.Skills.MarketplaceDir != "" {
		marketplaceAbsDir := filepath.Join(workspaceDir, cfg.Skills.MarketplaceDir)
		skillDirs = append(skillDirs, marketplaceAbsDir)
	}
	loadedSkills, skillErr := skills.LoadSkillsFromMultipleSources(skillDirs)
	if skillErr != nil {
		return fmt.Errorf("load skills: %w", skillErr)
	}
	loadedSkills = skills.FilterByFileSize(loadedSkills, cfg.Skills.MaxSkillFileBytes)
	loadedSkills = skills.LimitSkillCount(loadedSkills, cfg.Skills.MaxSkillsInPrompt)
	slog.Info("skills loaded", "count", len(loadedSkills), "dirs", skillDirs)

	// Phase 3 (T15): Wire rules engine into hook registry for PostFileRead injection
	hookRegistry := hooks.NewHookRegistry()
	if cfg.Rules.Dir != "" {
		rulesEngine := rules.NewEngine()
		if loadErr := rulesEngine.LoadRules(cfg.Rules.Dir); loadErr != nil {
			slog.Warn("rules engine: failed to load rules, proceeding without rules", "dir", cfg.Rules.Dir, "err", loadErr)
		} else {
			rulesHandler := rules.NewRulesHookHandler(rulesEngine)
			hookRegistry.Register(hooks.PostFileRead, rulesHandler.HandlePostFileRead)
			slog.Info("rules engine loaded", "dir", cfg.Rules.Dir)
		}
	}
	// Choose LLM client for agent: prefer Phase 2 backend if available
	var agentLLM types.LLMClient
	if activeBackend != nil {
		agentLLM = backendAdapter{activeBackend}
	} else {
		agentLLM = llmClient
	}
	var atomicBackend atomic.Value
	if activeBackend != nil {
		atomicBackend.Store(&llm.BackendHolder{Backend: activeBackend})
	}

	var skillsMu sync.RWMutex
	baseLoopCfg := agent.LoopConfig{
		LLM:              agentLLM,
		Tools:            registry,
		Scrubber:         scrubber,
		Memory:           memStore,
		Skills:           loadedSkills,
		Hooks:            hookRegistry,
		WorkspaceDir:     workspaceDir,
		MaxTurns:         50,
		AgentName:        cfg.Agent.Name,
		AgentLanguage:    cfg.Agent.Language,
		AgentTone:        cfg.Agent.Tone,
		ModelName:        activeModelName,
		ProviderName:     activeProviderName,
		MaxContextTokens: cfg.LLM.MaxContextTokens,
		MemoryFileStore:  fileMemStore,
		CoreStore:        coreStore,
		Guardrails:       guardrailsPipeline,
		CostTracker:      costTracker,
		Reflector:        agent.NewReflector(agentLLM, sqliteMemStore),
		PrefManager:      agent.NewPreferenceManager(coreStore),
		Planner:          agent.NewPlanner(agentLLM),
	}
	supervisor := agent.NewSupervisor(baseLoopCfg)

	bus := channel.NewMessageBus(256)
	var whatsAppChannel *whatsapp.WhatsAppChannel
	if cfg.Channels.Telegram.Enabled {
		if cfg.Channels.Telegram.Token == "" {
			slog.Warn("telegram enabled but token is empty")
		} else if registerErr := bus.Register(telegram.NewTelegramChannel(cfg.Channels.Telegram.Token)); registerErr != nil {
			return fmt.Errorf("register telegram channel: %w", registerErr)
		}
	}
	if cfg.Channels.Discord.Enabled {
		if cfg.Channels.Discord.Token == "" {
			slog.Warn("discord enabled but token is empty")
		} else if registerErr := bus.Register(discord.NewDiscordChannel(cfg.Channels.Discord.Token)); registerErr != nil {
			return fmt.Errorf("register discord channel: %w", registerErr)
		}
	}
	if cfg.Channels.WhatsApp.Enabled {
		if cfg.Channels.WhatsApp.Token == "" {
			slog.Warn("whatsapp enabled but token is empty")
		} else {
			whatsAppChannel = whatsapp.NewWhatsAppChannel(cfg.Channels.WhatsApp.Token, cfg.Channels.WhatsApp.AllowFrom)
			if registerErr := bus.Register(whatsAppChannel); registerErr != nil {
				return fmt.Errorf("register whatsapp channel: %w", registerErr)
			}
		}
	}

	daemonRegistry := daemonruntime.NewSubsystemRegistry()
	channelHealthAdapter := &busChannelHealth{bus: bus}
	healthSubsystem := daemonruntime.NewHealthSubsystem(cfg.Server.Addr, "blackcat",
		daemonruntime.WithRegistry(daemonRegistry),
		daemonruntime.WithChannelHealth(channelHealthAdapter),
	)
	daemonRegistry.Register(healthSubsystem)
	schedulerSubsystem := scheduler.NewSchedulerSubsystem(cfg.Scheduler)
	schedulerSubsystem.WithChecker(daemonRegistryChecker{reg: daemonRegistry, bus: bus})
	schedulerSubsystem.WithExecutor(&scheduler.ChannelExecutor{Sender: bus, Shell: &scheduler.ShellExecutor{}})
	schedulerSubsystem.WithReconnector(&busReconnector{bus: bus})
	daemonRegistry.Register(schedulerSubsystem)
	dashboardSubsystem := dashboard.NewServer(cfg.Dashboard, dashboard.DashboardDeps{
		SubsystemManager: daemonRegistry,
		TaskLister:       dashboardConfigTaskLister{jobs: cfg.Scheduler.Jobs},
		HeartbeatStore:   schedulerSubsystem.HeartbeatStore(),
		TaskDetailLister: dashboardScheduleDetailLister{subsystem: schedulerSubsystem},
		ScheduleProvider: dashboardScheduleProvider{jobs: cfg.Scheduler.Jobs},
	})
	if dashboardSubsystem != nil {
		daemonRegistry.Register(dashboardSubsystem)
		// Wire broadcaster: fire "schedule" SSE event after each task completes
		schedulerSubsystem.SetOnTaskComplete(func(name string, _ error) {
			dashboardSubsystem.Broadcaster().Send("schedule")
		})
		if whatsAppChannel != nil {
			qrCh := make(chan string, 8)
			type qrChannelSetter interface {
				SetQRChannel(ch chan<- string)
			}
			if setter, ok := any(whatsAppChannel).(qrChannelSetter); ok {
				setter.SetQRChannel(qrCh)
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case qrCode, ok := <-qrCh:
							if !ok {
								return
							}
							dashboardSubsystem.QRBroadcaster().Send(qrCode)
						}
					}
				}()
			}
		}
	}
	phase3Lifecycle := daemonruntime.NewLifecycleManager(schedulerSubsystem, dashboardSubsystem)

	// Register /api/channels/send endpoint on the health server for CLI and agent use.
	healthSubsystem.HandleFunc("/api/channels/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Channel   string `json:"channel"`
			ChannelID string `json:"channelId"`
			Message   string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if req.Channel == "" || req.ChannelID == "" || req.Message == "" {
			http.Error(w, `{"error":"channel, channelId, and message are required"}`, http.StatusBadRequest)
			return
		}
		channelType := types.ChannelType(req.Channel)
		msg := types.Message{
			ID:          fmt.Sprintf("api-send-%d", time.Now().UnixMilli()),
			ChannelType: channelType,
			ChannelID:   req.ChannelID,
			Content:     req.Message,
			Timestamp:   time.Now(),
		}
		if err := bus.Send(r.Context(), channelType, msg); err != nil {
			slog.Error("api channels send failed", "channel", req.Channel, "err", err)
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		slog.Info("api channels send ok", "channel", req.Channel, "channelId", req.ChannelID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	// Wire notification sender and start task queue before bus.Start
	if tq != nil {
		tq.SetNotificationSender(busNotificationAdapter{bus: bus})
		tq.Start(ctx)

		// Recover tasks that were pending/running when the daemon last stopped.
		if recovered := tq.RecoverInterruptedTasks(ctx); len(recovered) > 0 {
			slog.Info("recovered interrupted tasks", "count", len(recovered))
		}
	}

	if err := bus.Start(ctx); err != nil {
		return fmt.Errorf("start message bus: %w", err)
	}

	if err := healthSubsystem.Start(ctx); err != nil {
		return fmt.Errorf("start daemon subsystems: %w", err)
	}
	if err := phase3Lifecycle.StartAll(ctx); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		cleanupErr := phase3Lifecycle.StopAll(cleanupCtx)
		if healthErr := healthSubsystem.Stop(cleanupCtx); healthErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("subsystem %q failed to stop: %w", healthSubsystem.Name(), healthErr))
		}
		if cleanupErr != nil {
			return fmt.Errorf("start daemon subsystems: %w", errors.Join(err, cleanupErr))
		}

		return fmt.Errorf("start daemon subsystems: %w", err)
	}

	var watcher *config.Watcher
	if cfgFile != "" {
		watcher, err = config.Watch(cfgFile, func(newCfg *config.Config) {
			slog.Info("config changed", "logging_level", newCfg.Logging.Level)
			if newBackend, name := createActiveBackend(newCfg); newBackend != nil {
				atomicBackend.Store(&llm.BackendHolder{Backend: newBackend})
				slog.Info("backend hot-swapped", "provider", name)
			}
		})
		if err != nil {
			slog.Warn("config watcher disabled", "err", err)
		} else {
			slog.Info("config watcher started", "path", cfgFile)
		}
	}

	// Phase 4 (T4): Skill hot-reload via fsnotify
	var skillWatcher *skills.SkillWatcher
	if cfg.Skills.Dir != "" {
		watchDirs := []string{cfg.Skills.Dir}
		if cfg.Skills.MarketplaceDir != "" {
			marketplaceAbsDir := filepath.Join(workspaceDir, cfg.Skills.MarketplaceDir)
			watchDirs = append(watchDirs, marketplaceAbsDir)
		}
		skillWatcher, err = skills.NewSkillWatcher(watchDirs, 500*time.Millisecond, func(newSkills []skills.Skill) {
			newSkills = skills.FilterByFileSize(newSkills, cfg.Skills.MaxSkillFileBytes)
			newSkills = skills.LimitSkillCount(newSkills, cfg.Skills.MaxSkillsInPrompt)
			skillsMu.Lock()
			baseLoopCfg.Skills = newSkills
			skillsMu.Unlock()
		})
		if err != nil {
			slog.Warn("skill watcher disabled", "err", err)
		} else {
			go skillWatcher.Run()
			slog.Info("skill watcher started", "dirs", watchDirs)
		}
	}

	// Initialize Whisper transcription client if enabled
	var groqClient *transcription.GroqClient
	if cfg.Whisper.Enabled && cfg.Whisper.GroqAPIKey != "" {
		groqClient = transcription.NewGroqClient(
			cfg.Whisper.GroqAPIKey,
			transcription.WithModel(cfg.Whisper.Model),
			transcription.WithMaxFileSizeMB(cfg.Whisper.MaxFileSizeMB),
		)
		slog.Info("Whisper transcription enabled", "model", cfg.Whisper.Model)
	}

	workers, _ := cmd.Flags().GetInt("workers")
	if workers <= 0 {
		workers = 1
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	const historyMessageLimit = 20

	go func() {
		for msg := range bus.Messages() {
			sem <- struct{}{}
			wg.Add(1)
			go func(m types.Message) {
				defer func() {
					<-sem
					wg.Done()
				}()

				loopCfg := baseLoopCfg
				skillsMu.RLock()
				loopCfg.Skills = baseLoopCfg.Skills
				skillsMu.RUnlock()
				if holder, ok := atomicBackend.Load().(*llm.BackendHolder); ok && holder != nil && holder.Backend != nil {
					loopCfg.LLM = backendAdapter{holder.Backend}
				}
				loopCfg.MaxHistoryMessages = historyMessageLimit
				loopCfg.ChannelType = string(m.ChannelType)

				// Set per-user ID for memory isolation
				if m.UserID != "" {
					loopCfg.UserID = m.UserID
				} else {
					loopCfg.UserID = "default"
				}

				if rateLimiter != nil {
					rateLimitKey := string(m.ChannelType) + ":" + m.UserID
					if !rateLimiter.Allow(rateLimitKey) {
						slog.Warn("rate limit exceeded", "key", rateLimitKey)
						reply := types.Message{
							ID:          fmt.Sprintf("ratelimit-%s", m.ID),
							ChannelType: m.ChannelType,
							ChannelID:   m.ChannelID,
							Content:     "Maaf, Anda mengirim pesan terlalu cepat. Coba lagi nanti.",
							ReplyTo:     m.ID,
							Timestamp:   time.Now(),
						}
						sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
						defer cancel()
						_ = bus.Send(sendCtx, m.ChannelType, reply)
						return
					}
				}

				// Load session history
				key := session.SessionKey{
					ChannelType: string(m.ChannelType),
					ChannelID:   m.ChannelID,
					UserID:      m.UserID, // E3: empty UserID → channelID-only key handled by SessionKey.String()
				}
				if sessionStore != nil {
					sess, sessErr := sessionStore.Get(key)
					if sessErr != nil {
						slog.Warn("session load failed", "err", sessErr, "key", key.String())
					} else if sess != nil {
						history := make([]types.LLMMessage, 0, len(sess.Messages))
						for _, msg := range sess.Messages {
							if msg.Role != "user" && msg.Role != "assistant" {
								continue
							}

							history = append(history, types.LLMMessage{Role: msg.Role, Content: msg.Content})
						}
						loopCfg.SessionMessages = history
					}
				}

				// Acknowledgment: typing indicator is sent by the WhatsApp channel
				// on message receipt (whatsapp.go event handler). No text ack needed.

				// Create event stream for this message
				eventCh := make(chan agent.AgentEvent, 32)
				loopCfg.EventStream = eventCh

				// Forward select events to the user as interim progress messages
				go func() {
					for ev := range eventCh {
						switch ev.Kind {
						case agent.EventToolCallStart:
							progressMsg := types.Message{
								ID:          fmt.Sprintf("progress-%s-%d", m.ID, ev.TurnNum),
								ChannelType: m.ChannelType,
								ChannelID:   m.ChannelID,
								Content:     fmt.Sprintf("⚙️ %s", ev.Message),
								ReplyTo:     m.ID,
								Timestamp:   time.Now(),
							}
							sendCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
							_ = bus.Send(sendCtx, m.ChannelType, progressMsg)
							cancel()
						case agent.EventInterrupted:
							// Approval request is handled by HITL — do not double-send
						case agent.EventError:
							// Error messages are handled by the Run() return value
						default:
							// EventThinking, EventDone, EventHandoff — skip (noisy or duplicated)
						}
					}
				}()

				// Transcribe voice/audio messages via Groq Whisper
				if groqClient != nil && m.MediaType != "" {
					transcribeCtx, transcribeCancel := context.WithTimeout(ctx, 30*time.Second)
					var transcribed string
					var transcribeErr error

					if m.Metadata != nil && m.Metadata["wa_media_requires_download"] == "true" {
						// WhatsApp: can't use URL directly — skip for now, log warning
						slog.Warn("WhatsApp voice transcription requires whatsmeow download — skipping",
							"channel", m.ChannelType, "user", m.UserID)
					} else if m.MediaURL != "" {
						transcribed, transcribeErr = groqClient.TranscribeURL(transcribeCtx, m.MediaURL)
					}
					transcribeCancel()

					if transcribeErr != nil {
						slog.Warn("Whisper transcription failed", "error", transcribeErr,
							"channel", m.ChannelType, "mediaType", m.MediaType)
						// Don't fail — continue with fallback message
						m.Content = "[Voice message — transcription failed]"
					} else if transcribed != "" {
						m.Content = transcribed
						if m.Metadata == nil {
							m.Metadata = make(map[string]string)
						}
						m.Metadata["whisper_transcribed"] = "true"
						m.Metadata["whisper_media_type"] = m.MediaType
						slog.Info("Whisper transcription complete",
							"channel", m.ChannelType, "chars", len(transcribed))
					}
				}

				execution, runErr := supervisor.RouteWithCfg(ctx, m.Content, loopCfg)
				close(eventCh)
				response := ""
				if runErr != nil {
					slog.Error("agent error", "err", runErr, "channel", m.ChannelType, "user", m.UserID)
					switch {
					case errors.Is(runErr, types.ErrMaxTurnsExceeded):
						response = "Maaf, permintaan ini terlalu kompleks. Coba pecah jadi beberapa pertanyaan."
					case errors.Is(runErr, context.DeadlineExceeded), errors.Is(runErr, context.Canceled), errors.Is(runErr, llm.ErrTimeout):
						response = "Maaf, permintaan timeout. Coba lagi."
					case errors.Is(runErr, llm.ErrAuthFailure):
						response = "Ada masalah koneksi ke AI backend. Hubungi admin."
					case errors.Is(runErr, llm.ErrRateLimit):
						response = "AI provider sedang sibuk. Coba lagi dalam satu menit."
					case errors.Is(runErr, llm.ErrContextLength):
						// Retry once: compact session history, then re-run
						slog.Warn("context length exceeded, compacting and retrying", "channel", m.ChannelType)
						if sessionStore != nil {
							if sess, getErr := sessionStore.Get(key); getErr == nil && sess != nil {
								// Trim session to last 6 messages (3 exchanges)
								if len(sess.Messages) > 6 {
									sess.Messages = sess.Messages[len(sess.Messages)-6:]
									_ = sessionStore.Save(sess)
								}
								loopCfg.SessionMessages = sess.Messages
							}
						}
						// Retry with trimmed context
						loopCfg.EventStream = nil // eventCh is closed; disable streaming for retry
						retryExecution, retryErr := supervisor.RouteWithCfg(ctx, m.Content, loopCfg)
						if retryErr != nil {
							slog.Error("retry after compaction failed", "err", retryErr)
							response = "Percakapan sangat panjang. Saya mulai sesi baru untuk Anda."
							// Clear session entirely for next time
							if sessionStore != nil {
								_ = sessionStore.Delete(key)
							}
						} else {
							response = retryExecution.Response
							execution = retryExecution
						}
					default:
						response = "Maaf, terjadi kesalahan. Coba lagi."
					}
				} else {
					response = execution.Response
					slog.Info("llm usage",
						"prompt_tokens", execution.TotalUsage.PromptTokens,
						"completion_tokens", execution.TotalUsage.CompletionTokens,
						"total_tokens", execution.TotalUsage.TotalTokens,
						"turns", execution.TurnCount,
						"channel", m.ChannelType,
						"user", m.UserID,
					)
				}
				response = scrubber.Scrub(response)

				// Save interaction to memory
				if sqliteMemStore != nil {
					_ = sqliteMemStore.Add(ctx, m.Content, []string{string(m.ChannelType), "user"}, string(m.ChannelType))
					_ = sqliteMemStore.Add(ctx, response, []string{string(m.ChannelType), "assistant"}, string(m.ChannelType))
				}

				// Save session
				if sessionStore != nil {
					sess := &session.Session{Key: key, CreatedAt: time.Now()}
					if existing, getErr := sessionStore.Get(key); getErr == nil && existing != nil {
						sess = existing
					}
					if execution != nil && execution.Compacted {
						// Replace with compacted messages (excluding system prompt)
						sess.Messages = filterUserAssistant(execution.Messages)
					} else {
						sess.Messages = append(sess.Messages, types.LLMMessage{Role: "user", Content: m.Content})
						sess.Messages = append(sess.Messages, types.LLMMessage{Role: "assistant", Content: response})
					}
					sess.UpdatedAt = time.Now()
					if saveErr := sessionStore.Save(sess); saveErr != nil {
						slog.Warn("session save failed", "err", saveErr, "key", key.String())
					}
				}

				reply := types.Message{
					ID:          fmt.Sprintf("reply-%s", m.ID),
					ChannelType: m.ChannelType,
					ChannelID:   m.ChannelID,
					Content:     response,
					ReplyTo:     m.ID,
					Timestamp:   time.Now(),
				}

				sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if sendErr := bus.Send(sendCtx, m.ChannelType, reply); sendErr != nil {
					slog.Error("send reply failed", "err", sendErr, "channel", m.ChannelType, "msg_id", m.ID)
				}
			}(msg)
		}
	}()

	slog.Info("daemon started", "workers", workers, "health_addr", cfg.Server.Addr)

	select {
	case serveErr := <-healthSubsystem.Errors():
		stop()
		return fmt.Errorf("health server failed: %w", serveErr)
	case <-ctx.Done():
	}

	slog.Info("daemon shutting down")

	if watcher != nil {
		if watchErr := watcher.Stop(); watchErr != nil {
			slog.Warn("config watcher stop failed", "err", watchErr)
		}
	}
	if skillWatcher != nil {
		skillWatcher.Stop()
	}
	shutdownErr := bus.Stop()
	wg.Wait()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	subsystemShutdownErr := phase3Lifecycle.StopAll(shutdownCtx)
	if healthErr := healthSubsystem.Stop(shutdownCtx); healthErr != nil {
		subsystemShutdownErr = errors.Join(subsystemShutdownErr, fmt.Errorf("subsystem %q failed to stop: %w", healthSubsystem.Name(), healthErr))
	}
	if subsystemShutdownErr != nil {
		slog.Warn("daemon subsystem shutdown failed", "err", subsystemShutdownErr)
	}

	if err := memStore.Consolidate(shutdownCtx); err != nil {
		slog.Warn("memory flush failed", "err", err)
	}
	if sqliteMemStore != nil {
		if closeErr := sqliteMemStore.Close(); closeErr != nil {
			slog.Warn("sqlite memory close failed", "err", closeErr)
		}
	}

	if err := mcpClient.Close(); err != nil {
		slog.Warn("mcp close failed", "err", err)
	}

	return shutdownErr
}

func setupLogger(cfg config.LoggingConfig) {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "json") {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	slog.SetDefault(slog.New(handler))
}

// validateStartupConfig performs startup validation of critical config fields.
// It logs warnings for non-critical issues (no channels, no LLM provider) but
// returns errors for fatal conditions (enabled channel with no token).
func validateStartupConfig(cfg *config.Config) error {
	// Check if any channel is enabled
	anyChannelEnabled := cfg.Channels.Telegram.Enabled || cfg.Channels.Discord.Enabled || cfg.Channels.WhatsApp.Enabled
	if !anyChannelEnabled {
		slog.Warn("no messaging channels enabled; daemon running in headless mode")
	}

	// Check for enabled channels without tokens
	if cfg.Channels.Telegram.Enabled && cfg.Channels.Telegram.Token == "" {
		return fmt.Errorf("telegram channel enabled but token is empty")
	}
	if cfg.Channels.Discord.Enabled && cfg.Channels.Discord.Token == "" {
		return fmt.Errorf("discord channel enabled but token is empty")
	}
	if cfg.Channels.WhatsApp.Enabled && cfg.Channels.WhatsApp.Token == "" {
		return fmt.Errorf("whatsapp channel enabled but token is empty")
	}

	// Check if LLM provider is configured (non-fatal warning)
	if cfg.LLM.Provider == "" {
		slog.Warn("llm provider not configured", "hint", "set BLACKCAT_LLM_PROVIDER env var or llm.provider in config")
	}

	return nil
}

func buildDenyList(patterns []string) (denyList *security.DenyList, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compile deny patterns: %v", r)
		}
	}()

	return security.NewDenyList(patterns...), nil
}

type dashboardConfigTaskLister struct {
	jobs []config.ScheduledJob
}

func (l dashboardConfigTaskLister) ListTasks() []string {
	out := make([]string, 0, len(l.jobs))
	for _, job := range l.jobs {
		if !job.Enabled {
			continue
		}
		out = append(out, job.Name)
	}

	return out
}

type dashboardScheduleDetailLister struct {
	subsystem *scheduler.SchedulerSubsystem
}

func (l dashboardScheduleDetailLister) ListTasks() []scheduler.TaskState {
	return l.subsystem.ListTasks()
}

type dashboardScheduleProvider struct {
	jobs []config.ScheduledJob
}

func (p dashboardScheduleProvider) ListJobs() []dashboard.CalendarJobInfo {
	out := make([]dashboard.CalendarJobInfo, 0, len(p.jobs))
	for _, job := range p.jobs {
		out = append(out, dashboard.CalendarJobInfo{
			Name:     job.Name,
			Schedule: job.Schedule,
			Enabled:  job.Enabled,
		})
	}
	return out
}

func toolNameSet(defs []types.ToolDefinition) map[string]struct{} {
	out := make(map[string]struct{}, len(defs))
	for _, def := range defs {
		out[def.Name] = struct{}{}
	}
	return out
}

type mcpProxyTool struct {
	name        string
	description string
	parameters  json.RawMessage
	serverName  string
	toolName    string
	client      *mcp.Client
}

func (t *mcpProxyTool) Name() string {
	return t.name
}

func (t *mcpProxyTool) Description() string {
	return t.description
}

func (t *mcpProxyTool) Parameters() json.RawMessage {
	return t.parameters
}

func (t *mcpProxyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return t.client.Execute(ctx, t.serverName, t.toolName, args)
}

// --- Phase 2 Backend Wiring Helpers ---

// backendAdapter wraps an llm.Backend to satisfy types.LLMClient.
type backendAdapter struct {
	backend llm.Backend
}

func (a backendAdapter) Chat(ctx context.Context, messages []types.LLMMessage, tools []types.ToolDefinition) (*types.LLMResponse, error) {
	return a.backend.Chat(ctx, messages, tools)
}

func (a backendAdapter) Stream(ctx context.Context, messages []types.LLMMessage, tools []types.ToolDefinition) (<-chan types.Chunk, error) {
	return a.backend.Stream(ctx, messages, tools)
}

// createActiveBackend returns the first enabled Phase 2 backend from config.
// Returns (nil, "") if no Phase 2 providers are enabled.
func createActiveBackend(cfg *config.Config) (llm.Backend, string) {
	// OpenAI / Codex (API key from config or vault)
	if cfg.Providers.OpenAI.Enabled {
		apiKey := cfg.Providers.OpenAI.APIKey
		if apiKey == "" {
			// Try reading from vault as fallback
			if src := vaultTokenSource("provider.openai.apikey"); src != nil {
				if key, err := src(); err == nil && key != "" {
					apiKey = key
				}
			}
		}
		backend, err := llm.CreateBackend("openai", llm.BackendConfig{
			APIKey:  apiKey,
			BaseURL: cfg.Providers.OpenAI.BaseURL,
			Model:   cfg.Providers.OpenAI.Model,
		})
		if err == nil {
			return backend, "openai"
		}
		slog.Warn("openai backend failed, skipping", "err", err)
	}

	// Copilot (OAuth token from vault)
	if cfg.Providers.Copilot.Enabled {
		tokenSource := vaultTokenSource("oauth.copilot")
		backend, err := llm.CreateBackend("copilot", llm.BackendConfig{
			TokenSource: tokenSource,
			Model:       cfg.Providers.Copilot.Model,
		})
		if err == nil {
			return backend, "copilot"
		}
		slog.Warn("copilot backend failed, skipping", "err", err)
	}

	// Antigravity (OAuth token from vault)
	if cfg.Providers.Antigravity.Enabled && cfg.OAuth.Antigravity.AcceptedToS {
		tokenSource := vaultTokenSource("oauth.antigravity")
		backend, err := antigravity.NewAntigravityBackend(antigravity.AntigravityConfig{
			BackendConfig: llm.BackendConfig{
				TokenSource: tokenSource,
				Model:       cfg.Providers.Antigravity.Model,
			},
			AcceptedToS: true,
		})
		if err == nil {
			return backend, "antigravity"
		}
		slog.Warn("antigravity backend failed, skipping", "err", err)
	}

	// Gemini (API key)
	if cfg.Providers.Gemini.Enabled {
		backend, err := llm.CreateBackend("gemini", llm.BackendConfig{
			APIKey: cfg.Providers.Gemini.APIKey,
			Model:  cfg.Providers.Gemini.Model,
		})
		if err == nil {
			return backend, "gemini"
		}
		slog.Warn("gemini backend failed, skipping", "err", err)
	}

	// Zen (API key)
	if cfg.Providers.Zen.Enabled || cfg.Zen.Enabled {
		apiKey := cfg.Zen.APIKey
		model := cfg.Providers.Zen.Model
		if model == "" && len(cfg.Zen.Models) > 0 {
			model = cfg.Zen.Models[0]
		}
		baseURL := cfg.Zen.BaseURL
		backend, err := llm.CreateBackend("zen", llm.BackendConfig{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		})
		if err == nil {
			return backend, "zen"
		}
		slog.Warn("zen backend failed, skipping", "err", err)
	}

	return nil, ""
}

// vaultTokenSource creates a TokenSource that reads an OAuth token from the vault.
// It extracts the access_token from the stored JSON TokenSet.
func vaultTokenSource(vaultKey string) func() (string, error) {
	return func() (string, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}

		vaultPath := filepath.Join(home, ".blackcat", "vault.json")
		passphrase := os.Getenv("BLACKCAT_VAULT_PASSPHRASE")
		if passphrase == "" {
			return "", fmt.Errorf("BLACKCAT_VAULT_PASSPHRASE not set (needed for %s)", vaultKey)
		}

		vault, err := security.NewVault(vaultPath, passphrase)
		if err != nil {
			return "", fmt.Errorf("open vault: %w", err)
		}

		tokenJSON, err := vault.Get(vaultKey)
		if err != nil {
			return "", fmt.Errorf("get %s from vault: %w", vaultKey, err)
		}

		var ts oauth.TokenSet
		if err := json.Unmarshal([]byte(tokenJSON), &ts); err != nil {
			return "", fmt.Errorf("parse %s token: %w", vaultKey, err)
		}

		if ts.AccessToken == "" {
			return "", fmt.Errorf("%s: no access_token in stored token set", vaultKey)
		}

		return ts.AccessToken, nil
	}
}

// filterUserAssistant filters messages to only user/assistant roles (strips system prompt).
func filterUserAssistant(messages []types.LLMMessage) []types.LLMMessage {
	result := make([]types.LLMMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "user" || msg.Role == "assistant" {
			result = append(result, msg)
		}
	}
	return result
}

// daemonRegistryChecker adapts *daemonruntime.SubsystemRegistry to scheduler.SubsystemChecker.
type daemonRegistryChecker struct {
	reg *daemonruntime.SubsystemRegistry
	bus *channel.MessageBus
}

func (c daemonRegistryChecker) ListHealthy() []scheduler.SubsystemHealthInfo {
	healths := c.reg.Healthz()
	out := make([]scheduler.SubsystemHealthInfo, 0, len(healths))
	for _, h := range healths {
		out = append(out, scheduler.SubsystemHealthInfo{
			Name:    h.Name,
			Healthy: h.Status == "running" || h.Status == "healthy" || h.Status == "ok",
			Details: h.Message,
		})
	}

	// Add channel health checks
	if c.bus != nil {
		for _, info := range c.bus.Channels() {
			ch := c.bus.GetChannel(info.Type)
			if ch != nil {
				health := ch.Health()
				out = append(out, scheduler.SubsystemHealthInfo{
					Name:    health.Name,
					Healthy: health.Healthy,
					Details: health.Details,
				})
			}
		}
	}

	return out
}

// busChannelHealth adapts *channel.MessageBus to daemon.ChannelHealthProvider.
type busChannelHealth struct {
	bus *channel.MessageBus
}

func (b *busChannelHealth) ChannelHealthList() []daemonruntime.ChannelHealthStatus {
	if b.bus == nil {
		return nil
	}
	var out []daemonruntime.ChannelHealthStatus
	for _, info := range b.bus.Channels() {
		ch := b.bus.GetChannel(info.Type)
		if ch != nil {
			h := ch.Health()
			out = append(out, daemonruntime.ChannelHealthStatus{
				Name:    h.Name,
				Healthy: h.Healthy,
				Details: h.Details,
			})
		}
	}
	return out
}

// busNotificationAdapter adapts *channel.MessageBus to taskqueue.NotificationSender.
type busNotificationAdapter struct {
	bus *channel.MessageBus
}

func (a busNotificationAdapter) Send(ctx context.Context, recipientID, message string) error {
	msg := types.Message{
		ID:          fmt.Sprintf("notif-%d", time.Now().UnixNano()),
		ChannelType: types.ChannelWhatsApp,
		ChannelID:   recipientID,
		Content:     message,
		Timestamp:   time.Now(),
	}
	return a.bus.Send(ctx, types.ChannelWhatsApp, msg)
}

// busReconnector adapts *channel.MessageBus to scheduler.ChannelReconnector.
// It finds unhealthy channels that implement types.Reconnectable.
type busReconnector struct {
	bus *channel.MessageBus
}

func (b *busReconnector) UnhealthyReconnectable() []scheduler.ReconnectableChannel {
	if b.bus == nil {
		return nil
	}
	var out []scheduler.ReconnectableChannel
	for _, info := range b.bus.Channels() {
		ch := b.bus.GetChannel(info.Type)
		if ch == nil {
			continue
		}
		h := ch.Health()
		if h.Healthy {
			continue // only unhealthy channels
		}
		if r, ok := ch.(types.Reconnectable); ok {
			out = append(out, scheduler.ReconnectableChannel{
				Name:      h.Name,
				Reconnect: r.Reconnect,
			})
		}
	}
	return out
}
