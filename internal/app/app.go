package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"Marcus/internal/config"
	"Marcus/internal/executil"
	"Marcus/internal/model"
	"Marcus/internal/runtime"
	"Marcus/internal/sandbox"
	"Marcus/internal/store"
	"Marcus/internal/tools"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	cfg     *config.Config
	cfgPath string

	tools   ToolStore
	logs    LogStore
	scanner ToolScanner
	sandbox ProcessManager
	rt      RuntimeChecker

	storeClient    *store.Client
	storeInstaller *store.Installer
	storeUpdater   *store.Updater

	logFile   *os.File
	logFileMu sync.Mutex
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
	a.cfg = cfg
	a.cfgPath = cfgPath

	reg, err := tools.NewRegistry(a.cfg.DBPath)
	if err != nil {
		a.writeLog("ERROR", "registry init: "+err.Error())
		wailsRuntime.LogError(a.ctx, "registry init: "+err.Error())
		return
	}
	a.tools = reg

	logs, err := tools.NewLogStore(reg.DB())
	if err != nil {
		a.writeLog("ERROR", "logstore init: "+err.Error())
		wailsRuntime.LogError(a.ctx, "logstore init: "+err.Error())
		return
	}
	a.logs = logs

	a.scanner = tools.NewScanner(reg, a.cfg.ToolsDir)

	storeClient, err := store.NewClient(reg.DB())
	if err != nil {
		a.writeLog("ERROR", "store client init: "+err.Error())
		wailsRuntime.LogError(a.ctx, "store client init: "+err.Error())
	} else {
		a.storeClient = storeClient
		a.storeInstaller = store.NewInstaller(reg.DB(), reg, a.cfg.ToolsDir)
		a.storeUpdater = store.NewUpdater(reg.DB())

		// background sync on startup — fail silently, user can refresh manually
		go func() {
			if _, err := a.storeClient.Sync(); err != nil {
				a.writeLog("DEBUG", "store initial sync skipped: "+err.Error())
				wailsRuntime.LogDebug(a.ctx, "store initial sync skipped: "+err.Error())
			}
		}()
	}

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
}

func (a *App) Shutdown(ctx context.Context) {
	if a.sandbox != nil {
		a.sandbox.Shutdown()
	}
	if a.tools != nil {
		a.tools.Close()
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
	err := a.tools.UpsertTool(t)
	return t, err
}

func (a *App) DeleteTool(toolID string) error {
	return a.tools.DeleteTool(toolID)
}

func (a *App) GetToolLogs(toolID string, limit int) ([]model.ProcessState, error) {
	return a.logs.GetLogs(toolID, limit)
}

func (a *App) GetConfig() *config.Config {
	return a.cfg
}

func (a *App) SaveConfig(cfg config.Config) error {
	a.cfg = &cfg
	return a.cfg.Save(a.cfgPath)
}

// ─── Install Tool Package ────────────────────────────────────

// InstallToolPackage installs a .whl or .tgz package synchronously. Prefer
// InstallToolPackageAsync for UI feedback.
func (a *App) InstallToolPackage(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	switch {
	case ext == ".whl":
		if _, err := exec.LookPath("uv"); err != nil {
			return fmt.Errorf("uv 未安装，无法安装 Python 包。\n请先安装 uv：\n  powershell -c \"irm https://astral.sh/uv/install.ps1 | iex\"")
		}
		cmd := exec.Command("uv", "tool", "install", path)
		executil.HideWindow(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case ext == ".tgz" || strings.HasSuffix(base, ".tar.gz"):
		if _, err := exec.LookPath("bun"); err != nil {
			return fmt.Errorf("bun 未安装，无法安装 Node.js 包。\n请先安装 bun：\n  powershell -c \"irm bun.sh/install.ps1 | iex\"")
		}
		cmd := exec.Command("bun", "install", "-g", path)
		executil.HideWindow(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf("unsupported package type: %s (supported: .whl, .tgz)", ext)
	}
}

// InstallToolPackageAsync starts package installation in the background and
// pushes progress/completion events to the frontend.
func (a *App) InstallToolPackageAsync(path string) error {
	if a.ctx == nil {
		return fmt.Errorf("app not started")
	}
	go a.runPackageInstall(path)
	return nil
}

func (a *App) runPackageInstall(path string) {
	emit := func(status, message string, progress int) {
		wailsRuntime.EventsEmit(a.ctx, "install:progress", map[string]any{
			"path":     path,
			"status":   status,
			"message":  message,
			"progress": progress,
		})
	}
	emitComplete := func(success bool, errMsg string) {
		wailsRuntime.EventsEmit(a.ctx, "install:complete", map[string]any{
			"path":    path,
			"success": success,
			"error":   errMsg,
		})
	}

	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	emit("starting", "准备安装...", 0)

	var cmd *exec.Cmd
	switch {
	case ext == ".whl":
		if _, err := exec.LookPath("uv"); err != nil {
			msg := "uv 未安装。\n请先安装 uv：\n  powershell -c \"irm https://astral.sh/uv/install.ps1 | iex\""
			emit("error", msg, 0)
			emitComplete(false, msg)
			return
		}
		cmd = exec.Command("uv", "tool", "install", path)
	case ext == ".tgz" || strings.HasSuffix(base, ".tar.gz"):
		if _, err := exec.LookPath("bun"); err != nil {
			msg := "bun 未安装。\n请先安装 bun：\n  powershell -c \"irm bun.sh/install.ps1 | iex\""
			emit("error", msg, 0)
			emitComplete(false, msg)
			return
		}
		cmd = exec.Command("bun", "install", "-g", path)
	default:
		emit("error", fmt.Sprintf("不支持的包类型: %s", ext), 0)
		emitComplete(false, fmt.Sprintf("unsupported package type: %s", ext))
		return
	}
	executil.HideWindow(cmd)

	emit("running", "正在执行安装命令...", 30)

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(output)
		if msg == "" {
			msg = err.Error()
		}
		emit("error", msg, 0)
		emitComplete(false, fmt.Sprintf("%v: %s", err, msg))
		return
	}

	emit("running", "安装完成，正在刷新工具列表...", 80)

	if a.scanner != nil {
		if _, scanErr := a.scanner.ScanAll(); scanErr != nil {
			emit("warning", "安装成功但刷新工具列表失败: "+scanErr.Error(), 90)
		}
	}

	emit("running", "工具列表已刷新", 100)
	emitComplete(true, "")
}

// ─── Store (Plugin Marketplace) ─────────────────────────────

func (a *App) StoreSync() (*model.StoreIndex, error) {
	if a.storeClient == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeClient.Sync()
}

func (a *App) StoreListPlugins() ([]model.StorePlugin, error) {
	if a.storeClient == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeClient.ListPlugins()
}

func (a *App) StoreSearchPlugins(query string) ([]model.StorePlugin, error) {
	if a.storeClient == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeClient.SearchPlugins(query)
}

func (a *App) StoreInstall(pluginID, version string) (*model.InstallResult, error) {
	if a.storeInstaller == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	plugin, err := a.storeClient.GetCachedPlugin(pluginID)
	if err != nil {
		return nil, fmt.Errorf("lookup plugin: %w", err)
	}

	ver, ok := plugin.Versions[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found for plugin %s", version, pluginID)
	}

	return a.storeInstaller.Install(pluginID, version, ver.DownloadURL)
}

func (a *App) StoreCheckUpdates() ([]model.UpdateCheckResult, error) {
	if a.storeUpdater == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeUpdater.CheckUpdates()
}

func (a *App) StoreHasUpdates() (bool, error) {
	if a.storeUpdater == nil {
		return false, fmt.Errorf("store not initialized")
	}
	return a.storeUpdater.HasUpdates()
}

// ─── Runtime Installation ────────────────────────────────────

// InstallRuntime installs a runtime (uv or bun) synchronously using the
// language-native package manager (pip for uv, npm for bun).
func (a *App) InstallRuntime(runtimeName string) error {
	switch runtimeName {
	case "uv":
		if _, err := exec.LookPath("uv"); err == nil {
			return nil
		}
		if _, err := exec.LookPath("pip"); err != nil {
			return fmt.Errorf("pip 未安装，无法安装 uv。请先安装 Python pip")
		}
		cmd := exec.Command("pip", "install", "uv")
		executil.HideWindow(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "bun":
		if _, err := exec.LookPath("bun"); err == nil {
			return nil
		}
		if _, err := exec.LookPath("npm"); err != nil {
			return fmt.Errorf("npm 未安装，无法安装 bun。请先安装 Node.js npm")
		}
		cmd := exec.Command("npm", "install", "-g", "bun")
		executil.HideWindow(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf("unknown runtime: %s (supported: uv, bun)", runtimeName)
	}
}

// InstallRuntimeAsync starts runtime installation in the background and pushes
// progress/completion events to the frontend.
func (a *App) InstallRuntimeAsync(runtimeName string) error {
	if a.ctx == nil {
		return fmt.Errorf("app not started")
	}
	go a.runRuntimeInstall(runtimeName)
	return nil
}

func (a *App) runRuntimeInstall(runtimeName string) {
	emit := func(status, message string, progress int) {
		wailsRuntime.EventsEmit(a.ctx, "runtime:install-progress", map[string]any{
			"runtime":  runtimeName,
			"status":   status,
			"message":  message,
			"progress": progress,
		})
	}
	emitComplete := func(success bool, errMsg string) {
		wailsRuntime.EventsEmit(a.ctx, "runtime:install-complete", map[string]any{
			"runtime": runtimeName,
			"success": success,
			"error":   errMsg,
		})
	}

	// Check if already installed.
	switch runtimeName {
	case "uv":
		if _, err := exec.LookPath("uv"); err == nil {
			emit("done", "uv 已安装", 100)
			emitComplete(true, "")
			return
		}
	case "bun":
		if _, err := exec.LookPath("bun"); err == nil {
			emit("done", "bun 已安装", 100)
			emitComplete(true, "")
			return
		}
	default:
		emit("error", fmt.Sprintf("不支持运行时: %s", runtimeName), 0)
		emitComplete(false, fmt.Sprintf("unknown runtime: %s", runtimeName))
		return
	}

	emit("starting", "正在准备安装...", 0)

	var cmd *exec.Cmd
	var label string
	switch runtimeName {
	case "uv":
		if _, err := exec.LookPath("pip"); err != nil {
			msg := "pip 未安装，无法安装 uv。请先安装 Python pip"
			emit("error", msg, 0)
			emitComplete(false, msg)
			return
		}
		cmd = exec.Command("pip", "install", "uv")
		label = "uv"
	case "bun":
		if _, err := exec.LookPath("npm"); err != nil {
			msg := "npm 未安装，无法安装 bun。请先安装 Node.js npm"
			emit("error", msg, 0)
			emitComplete(false, msg)
			return
		}
		cmd = exec.Command("npm", "install", "-g", "bun")
		label = "bun"
	}
	executil.HideWindow(cmd)

	emit("running", fmt.Sprintf("正在通过 %s 安装 %s...",
		map[string]string{"uv": "pip", "bun": "npm"}[runtimeName], label), 30)

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(output)
		if msg == "" {
			msg = err.Error()
		}
		emit("error", msg, 0)
		emitComplete(false, msg)
		return
	}

	emit("done", fmt.Sprintf("%s 安装成功", label), 100)
	emitComplete(true, "")
}

// ─── App Logging ─────────────────────────────────────────────

func (a *App) writeLog(level, msg string) {
	a.logFileMu.Lock()
	defer a.logFileMu.Unlock()
	if a.logFile != nil {
		fmt.Fprintf(a.logFile, "[%s] %s: %s\n", time.Now().Format(time.DateTime), level, msg)
	}
}

func (a *App) GetAppLogs(count int) ([]string, error) {
	if a.cfgPath == "" {
		return nil, nil
	}
	logPath := filepath.Join(filepath.Dir(a.cfgPath), "app.log")
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return lines, nil
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
