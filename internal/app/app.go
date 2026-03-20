package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"miniclaw-go/internal/agent"
	"miniclaw-go/internal/config"
	"miniclaw-go/internal/core"
	"miniclaw-go/internal/llm"
	"miniclaw-go/internal/mcp"
	"miniclaw-go/internal/memory"
	"miniclaw-go/internal/skills"
	"miniclaw-go/internal/tools"
	"miniclaw-go/internal/trace"
)

type App struct {
	cfg             *config.Config
	projectRoot     string
	llmClient       *llm.Client
	registry        *tools.Registry
	sessionStore    *memory.SessionStore
	memoryStore     *memory.Store
	skillLoader     *skills.Loader
	traceStore      *trace.Store
	loop            *agent.Loop
	mcpClients      []*mcp.Client
	startupWarnings []string

	lockMu       sync.Mutex
	sessionLocks map[string]*sync.Mutex
}

func New(configPath string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(configPath), ".."))
	registry := tools.NewRegistry()
	filePolicy := tools.FilePolicy{
		ProjectRoot: projectRoot,
		AllowedRoots: []string{
			projectRoot,
			cfg.Workspace.Root,
			cfg.Memory.Root,
			cfg.Skills.Root,
		},
	}

	tools.RegisterFileTools(registry, filePolicy)
	tools.RegisterCommandTool(registry, filePolicy, cfg.Tools.CommandTimeout)
	tools.RegisterFetchTool(registry)
	tools.RegisterSearchTool(registry)
	tools.RegisterChartTool(registry, filePolicy)

	app := &App{
		cfg:          cfg,
		projectRoot:  projectRoot,
		llmClient:    llm.NewClient(cfg.LLM),
		registry:     registry,
		sessionStore: memory.NewSessionStore(cfg.Sessions.Root),
		memoryStore:  memory.NewStore(cfg.Memory.Root),
		skillLoader:  skills.NewLoader(cfg.Skills.Root),
		traceStore:   trace.NewStore(cfg.Traces.Root),
		sessionLocks: map[string]*sync.Mutex{},
	}

	if err := app.ensureDirs(); err != nil {
		return nil, err
	}
	if err := app.initMCP(); err != nil {
		return nil, err
	}

	app.loop = agent.NewLoop(app.llmClient, app.registry, cfg.Agent.MaxSteps, cfg.Skills.Root)
	return app, nil
}

func (a *App) Close() {
	for _, client := range a.mcpClients {
		_ = client.Close()
	}
}

func (a *App) StartupWarnings() []string {
	return append([]string(nil), a.startupWarnings...)
}

func (a *App) Run(ctx context.Context, sessionID, userInput string) (core.RunResult, error) {
	lock := a.sessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	runCtx := core.RunContext{
		SessionID: sessionID,
		RunID:     core.NewID("run"),
		UserInput: userInput,
		StartTime: time.Now().UTC(),
	}

	history, err := a.sessionStore.Load(sessionID)
	if err != nil {
		return core.RunResult{}, err
	}
	coreDocs, err := a.loadCorePromptSections()
	if err != nil {
		return core.RunResult{}, err
	}
	memoryContext, err := a.memoryStore.GetMemoryContext(3)
	if err != nil {
		return core.RunResult{}, err
	}
	sessionSummary, err := a.memoryStore.LoadSessionSummary(sessionID)
	if err != nil {
		return core.RunResult{}, err
	}
	skillDescs, err := a.skillLoader.Load()
	if err != nil {
		return core.RunResult{}, err
	}
	toolDescs := a.registry.List()
	promptInput := agent.PromptInput{
		CoreDocs:       coreDocs,
		MemoryContext:  memoryContext,
		SessionSummary: sessionSummary,
		RecentHistory:  history,
		Tools:          toolDescs,
		Skills:         skillDescs,
	}
	systemPrompt := agent.BuildSystemPrompt(promptInput)
	contextSummary := agent.ContextSummary(promptInput)

	newMessages, runTrace, result, err := a.loop.Run(ctx, runCtx, systemPrompt, history, toolDescs)
	if err != nil {
		return core.RunResult{}, err
	}

	rememberFilename, rememberContent, shouldRemember := detectRememberRequest(userInput)
	if shouldRemember {
		if memErr := a.memoryStore.AppendLongTerm(rememberFilename, rememberContent); memErr == nil {
			result.Output = strings.TrimSpace(result.Output + "\n\n已记录到长期记忆。")
			if len(newMessages) > 0 && newMessages[len(newMessages)-1].Role == "assistant" {
				newMessages[len(newMessages)-1].Content = result.Output
			}
		}
	}

	if err := a.sessionStore.Append(sessionID, newMessages); err != nil {
		return core.RunResult{}, err
	}

	combinedHistory := append(history, newMessages...)
	compressedHistory, compressed, err := a.maybeCompressSession(ctx, sessionID, sessionSummary, combinedHistory)
	if err != nil {
		return core.RunResult{}, err
	}
	if compressed {
		runTrace.Metadata = map[string]any{
			"session_compressed":     true,
			"history_after_compress": len(compressedHistory),
		}
	}

	runTrace.ContextSummary = contextSummary
	runTrace.Skills = skillNames(skillDescs)
	tracePath, err := a.traceStore.Write(runTrace)
	if err != nil {
		return core.RunResult{}, err
	}
	result.TracePath = tracePath
	return result, nil
}

func (a *App) ListTools() []core.ToolDescriptor {
	return a.registry.List()
}

func (a *App) ListSkills() ([]core.SkillDescriptor, error) {
	return a.skillLoader.Load()
}

func (a *App) ListMemoryFiles() ([]string, error) {
	return a.memoryStore.ListFiles()
}

func (a *App) SessionStats(sessionID string) (core.SessionStats, error) {
	stats, err := a.sessionStore.Stats(sessionID)
	if err != nil {
		return core.SessionStats{}, err
	}
	summaryExists, err := a.memoryStore.SessionSummaryExists(sessionID)
	if err != nil {
		return core.SessionStats{}, err
	}
	stats.SummaryExists = summaryExists
	return stats, nil
}

func (a *App) ensureDirs() error {
	dirs := []string{
		a.cfg.Workspace.Root,
		a.cfg.Memory.Root,
		a.cfg.Skills.Root,
		a.cfg.Sessions.Root,
		a.cfg.Traces.Root,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return a.memoryStore.Ensure()
}

func (a *App) initMCP() error {
	for _, server := range a.cfg.MCP.Servers {
		client := mcp.NewClient(server)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		if err := client.Start(ctx); err != nil {
			cancel()
			a.startupWarnings = append(a.startupWarnings, fmt.Sprintf("MCP 服务 %s 不可用：%v", server.Name, err))
			continue
		}
		toolDescs, err := client.ListTools(ctx)
		cancel()
		if err != nil {
			a.startupWarnings = append(a.startupWarnings, fmt.Sprintf("MCP 服务 %s 获取工具列表失败：%v", server.Name, err))
			_ = client.Close()
			continue
		}

		for _, desc := range toolDescs {
			toolName := desc.Name
			a.registry.Register(desc, func(ctx context.Context, args json.RawMessage) (string, error) {
				return client.CallTool(ctx, toolName, args)
			})
		}
		a.mcpClients = append(a.mcpClients, client)
	}
	return nil
}

func (a *App) sessionLock(sessionID string) *sync.Mutex {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if lock, ok := a.sessionLocks[sessionID]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	a.sessionLocks[sessionID] = lock
	return lock
}

func (a *App) loadCorePromptSections() ([]agent.PromptSection, error) {
	docs, err := a.memoryStore.LoadCoreDocuments()
	if err != nil {
		return nil, err
	}

	sections := make([]agent.PromptSection, 0, len(docs))
	for _, doc := range docs {
		sections = append(sections, agent.PromptSection{
			Title:   doc.Title,
			Content: doc.Content,
		})
	}
	return sections, nil
}

func detectRememberRequest(input string) (filename string, content string, ok bool) {
	text := strings.TrimSpace(input)
	lower := strings.ToLower(text)
	if strings.Contains(text, "记住") || strings.Contains(lower, "remember") {
		filename = "MEMORY.md"
		if strings.Contains(text, "我") || strings.Contains(text, "我的") || strings.Contains(lower, " i ") || strings.Contains(lower, " my ") {
			filename = "USER.md"
		}
		return filename, text, true
	}
	return "", "", false
}

func skillNames(skills []core.SkillDescriptor) []string {
	out := make([]string, 0, len(skills))
	for _, skill := range skills {
		out = append(out, skill.Name)
	}
	return out
}
