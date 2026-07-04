package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"Marcus/internal/config"
	"Marcus/internal/model"
	"Marcus/internal/runtime"
	"Marcus/internal/sandbox"
	"Marcus/internal/tools"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	cfg     *config.Config
	cfgPath string
	reg     *tools.Registry
	scanner *tools.Scanner
	rt      *runtime.Checker
	sandbox *sandbox.Manager
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

	cfgPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.Default()
	}
	cfg.Save(cfgPath)
	a.cfg = cfg
	a.cfgPath = cfgPath

	reg, err := tools.NewRegistry(a.cfg.DBPath)
	if err != nil {
		wailsRuntime.LogError(a.ctx, "registry init: "+err.Error())
		return
	}
	a.reg = reg
	a.scanner = tools.NewScanner(reg, a.cfg.ToolsDir)
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
	)
}

func (a *App) Shutdown(ctx context.Context) {
	if a.sandbox != nil {
		a.sandbox.Shutdown()
	}
	if a.reg != nil {
		a.reg.Close()
	}
}

// ─── Wails Bind Methods ─────────────────────────────────────

func (a *App) GetTools(category string) ([]model.ToolInfo, error) {
	return a.reg.ListTools(category)
}

func (a *App) LaunchTool(toolID string, args map[string]string) (*model.ProcessState, error) {
	tool, err := a.reg.GetTool(toolID)
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

	a.reg.AddLog(*state)
	return state, nil
}

func (a *App) StopTool(toolID string) error {
	return a.sandbox.Stop(toolID)
}

func (a *App) GetToolState(toolID string) *model.ProcessState {
	return a.sandbox.GetState(toolID)
}

func (a *App) GetRuntimeStatus() map[string]model.RuntimeInfo {
	return a.rt.CheckAll()
}

func (a *App) RefreshTools() ([]model.ToolInfo, error) {
	return a.scanner.ScanAll()
}

func (a *App) AddManualTool(name, command, argType string) (model.ToolInfo, error) {
	t := tools.ParseManual(name, command, argType)
	err := a.reg.UpsertTool(t)
	return t, err
}

func (a *App) DeleteTool(toolID string) error {
	return a.reg.DeleteTool(toolID)
}

func (a *App) GetToolLogs(toolID string, limit int) ([]model.ProcessState, error) {
	return a.reg.GetLogs(toolID, limit)
}

func (a *App) GetConfig() *config.Config {
	return a.cfg
}

func (a *App) SaveConfig(cfg config.Config) error {
	a.cfg = &cfg
	return a.cfg.Save(a.cfgPath)
}

// ─── Install Tool Package ────────────────────────────────────

func (a *App) InstallToolPackage(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	switch {
	case ext == ".whl":
		cmd := exec.Command("uv", "tool", "install", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case ext == ".tgz" || strings.HasSuffix(base, ".tar.gz"):
		cmd := exec.Command("bun", "install", "-g", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf("unsupported package type: %s (supported: .whl, .tgz)", ext)
	}
}

// ─── File Dialogs ────────────────────────────────────────────

func (a *App) OpenFileDialog(filter string) (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) OpenDirectoryDialog() (string, error) {
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择目录",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}
