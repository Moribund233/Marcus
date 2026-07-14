package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"Marcus/internal/agent"
	"Marcus/internal/config"
	"Marcus/internal/conversation"
	marcusdb "Marcus/internal/db"
	"Marcus/internal/llm"
	"Marcus/internal/memory"
	"Marcus/internal/model"
	"Marcus/internal/runtime"
	"Marcus/internal/sandbox"
	"Marcus/internal/store"
	"Marcus/internal/tools"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	_ "modernc.org/sqlite"
)

type App struct {
	ctx     context.Context
	cfg     *config.Config
	cfgPath string

	db      *sql.DB
	tools   ToolStore
	logs    LogStore
	scanner ToolScanner
	sandbox ProcessManager
	rt      RuntimeChecker

	storeClient    *store.Client
	storeInstaller *store.Installer
	storeUpdater   *store.Updater
	uninstaller    *tools.Uninstaller

	// Agent 相关依赖
	agent     *agent.Agent
	llmConfig *llm.Config
	convStore *conversation.Store
	memStore  *memory.Store

	agentMu sync.RWMutex

	logFile   *os.File
	logFileMu sync.Mutex
	configMu  sync.RWMutex
}

func New() *App {
	return &App{
		rt: runtime.New(),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".marcus")
	os.MkdirAll(dataDir, 0755)

	logPath := filepath.Join(dataDir, "app.log")
	a.logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

	cfgPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.Default()
	}
	cfg.Save(cfgPath)
	a.setConfig(cfg)
	a.cfgPath = cfgPath

	db, err := sql.Open("sqlite", a.cfg.DBPath)
	if err != nil {
		a.writeLog("ERROR", "open db: "+err.Error())
		wailsRuntime.LogError(a.ctx, "open db: "+err.Error())
		return
	}
	a.db = db

	migrator := marcusdb.NewMigrator(db)
	migrator.Register(marcusdb.All()...)
	if err := migrator.Run(); err != nil {
		a.writeLog("ERROR", "migrate: "+err.Error())
		wailsRuntime.LogError(a.ctx, "migrate: "+err.Error())
		return
	}

	modelStore := llm.NewModelStore(db)
	llm.SetModelStore(modelStore)

	reg := tools.NewRegistry(db)
	a.tools = reg
	a.logs = tools.NewLogStore(db)
	a.scanner = tools.NewScanner(reg, a.cfg.ToolsDir)

	a.storeClient = store.NewClient(db, a.cfg.StoreIndexURL)
	a.storeInstaller = store.NewInstaller(db, a.cfg.ToolsDir)
	a.storeUpdater = store.NewUpdater(db)

	go func() {
		if _, err := a.storeClient.Sync(); err != nil {
			a.writeLog("DEBUG", "store initial sync skipped: "+err.Error())
			wailsRuntime.LogDebug(a.ctx, "store initial sync skipped: "+err.Error())
		}
	}()

	// Retry any pending store installs that failed during previous scanning.
	go a.retryPendingInstalls()

	a.uninstaller = tools.NewUninstaller(reg, db, a.cfg.ToolsDir)
	a.uninstaller.SetLanguage(a.cfg.Language)

	a.sandbox = sandbox.NewManager(
		model.ResourceLimits{
			CPULimitPercent: a.cfg.DefaultCPU,
			MemoryLimitMB:   a.cfg.DefaultMemory,
			TimeoutSeconds:  a.cfg.DefaultTimeout,
		},
		func(toolID string, line string) {
			wailsRuntime.EventsEmit(a.ctx, "tool:output", map[string]string{
				"tool_id": toolID,
				"line":    line,
			})
		},
		func(toolID string, state model.ProcessState) {
			wailsRuntime.EventsEmit(a.ctx, "tool:state-changed", map[string]any{
				"tool_id":   toolID,
				"status":    state.Status,
				"pid":       state.PID,
				"port":      state.Port,
				"exit_code": state.ExitCode,
				"error":     state.ErrorLog,
			})
		},
	)

	if err := a.initAgent(); err != nil {
		a.writeLog("ERROR", "agent init: "+err.Error())
		wailsRuntime.LogError(a.ctx, "agent init: "+err.Error())
	}

	// Initial tool discovery: scan runtimes (uv, bun, binary dir) for any
	// already-installed tools, then register them with the agent.
	if _, err := a.scanner.ScanAll(); err != nil {
		a.writeLog("WARN", "initial tool scan: "+err.Error())
	}
	a.registerTools(a.agent)
}

func (a *App) Shutdown(ctx context.Context) {
	if a.sandbox != nil {
		a.sandbox.Shutdown()
	}
	if a.db != nil {
		a.db.Close()
	}
}

// ─── Wails Bind Methods ─────────────────────────────────────

func (a *App) GetTools(category string) ([]model.ToolInfo, error) {
	return a.tools.ListTools(category)
}

func (a *App) LaunchTool(toolID string, args map[string]string) (*model.ProcessState, error) {
	tool, err := a.tools.GetTool(toolID)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %w", err)
	}

	manifest, err := tool.ManifestAsManifest()
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	manifest.ToolID = toolID
	state, err := a.sandbox.Start(manifest, args)
	if err != nil {
		return nil, err
	}

	a.logs.AddLog(*state)
	_ = a.tools.UpdateLastUsed(toolID)
	return state, nil
}

func (a *App) StopTool(toolID string) error {
	return a.sandbox.Stop(toolID)
}

func (a *App) GetToolState(toolID string) *model.ProcessState {
	return a.sandbox.GetState(toolID)
}

func (a *App) RefreshTools() ([]model.ToolInfo, error) {
	tools, err := a.scanner.ScanAll()
	if err != nil {
		return tools, err
	}
	a.syncAgentTools()
	return tools, nil
}

func (a *App) AddManualTool(name, command, argType string) (model.ToolInfo, error) {
	t := tools.ParseManual(name, command, argType)
	if err := a.tools.UpsertTool(t); err != nil {
		return t, err
	}
	a.syncAgentTools()
	return t, nil
}

func (a *App) DeleteTool(toolID string) error {
	if err := a.tools.DeleteTool(toolID); err != nil {
		return err
	}
	// Clean up store_installed in case this was a store-installed tool.
	a.db.Exec(
		`DELETE FROM store_installed WHERE ? LIKE '%' || plugin_id`, toolID,
	)
	a.syncAgentTools()
	return nil
}

func (a *App) UninstallTool(toolID string) (*model.UninstallResult, error) {
	if a.uninstaller == nil {
		return nil, fmt.Errorf("uninstaller not initialized")
	}

	err := a.sandbox.Stop(toolID)
	if err != nil && !errors.Is(err, sandbox.ErrProcessNotFound) {
		return nil, fmt.Errorf("stop running process: %w", err)
	}

	result, err := a.uninstaller.Uninstall(toolID)
	if err != nil {
		return result, err
	}

	if result.Success {
		wailsRuntime.EventsEmit(a.ctx, "tool-uninstalled", toolID)
		a.syncAgentTools()
	}

	return result, nil
}

func (a *App) GetToolLogs(toolID string, limit int) ([]model.ProcessState, error) {
	return a.logs.GetLogs(toolID, limit)
}

func (a *App) GetRecentTools(limit int) ([]model.ToolInfo, error) {
	if a.tools == nil {
		return nil, fmt.Errorf("tools registry not initialized")
	}
	return a.tools.ListRecentTools(limit)
}

func (a *App) GetConfig() *config.Config {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.cfg
}

func (a *App) SaveConfig(cfg config.Config) error {
	a.configMu.Lock()
	a.cfg = &cfg
	err := a.cfg.Save(a.cfgPath)
	a.configMu.Unlock()
	return err
}

// setConfig is an internal helper that replaces the config pointer while holding
// the write lock. Used during startup where external callers are not yet active.
func (a *App) setConfig(cfg *config.Config) {
	a.configMu.Lock()
	a.cfg = cfg
	a.configMu.Unlock()
}
