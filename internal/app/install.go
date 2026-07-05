package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"Marcus/internal/executil"
	"Marcus/internal/model"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// InstallToolPackage installs a .whl or .tgz package synchronously. Prefer
// InstallToolPackageAsync for UI feedback.
func (a *App) InstallToolPackage(path string) error {
	cmd, _, err := a.buildPackageInstallCmd(path)
	if err != nil {
		return err
	}
	executil.HideWindow(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	_, err = a.registerLocalPackage(path)
	return err
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

	emit("starting", "准备安装...", 0)

	cmd, _, err := a.buildPackageInstallCmd(path)
	if err != nil {
		emit("error", err.Error(), 0)
		emitComplete(false, err.Error())
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

	emit("running", "安装完成，正在注册工具...", 80)

	newTools, err := a.registerLocalPackage(path)
	if err != nil {
		emit("error", err.Error(), 90)
		emitComplete(false, err.Error())
		return
	}

	emit("running", fmt.Sprintf("已注册 %d 个新工具", len(newTools)), 100)
	emitComplete(true, "")
}

// buildPackageInstallCmd constructs the platform-native install command for a
// local .whl or .tgz package. It returns the command, a package type label, and
// any validation error.
func (a *App) buildPackageInstallCmd(path string) (*exec.Cmd, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	switch {
	case ext == ".whl":
		if _, err := exec.LookPath("uv"); err != nil {
			return nil, "", fmt.Errorf("uv 未安装，无法安装 Python 包。\n请先安装 uv：\n  powershell -c \"irm https://astral.sh/uv/install.ps1 | iex\"")
		}
		return exec.Command("uv", "tool", "install", path), "whl", nil

	case ext == ".tgz" || strings.HasSuffix(base, ".tar.gz"):
		if _, err := exec.LookPath("bun"); err != nil {
			return nil, "", fmt.Errorf("bun 未安装，无法安装 Node.js 包。\n请先安装 bun：\n  powershell -c \"irm bun.sh/install.ps1 | iex\"")
		}
		return exec.Command("bun", "install", "-g", path), "tgz", nil

	default:
		return nil, "", fmt.Errorf("unsupported package type: %s (supported: .whl, .tgz)", ext)
	}
}

// registerLocalPackage scans for tools after a local package install, records
// any newly discovered tools, and returns them. If no new tool is found it
// returns an error so the caller can surface a clear failure message.
func (a *App) registerLocalPackage(path string) ([]model.ToolInfo, error) {
	if a.scanner == nil {
		return nil, fmt.Errorf("scanner not initialized")
	}
	if a.tools == nil {
		return nil, fmt.Errorf("tool store not initialized")
	}

	before, err := a.toolIDsSnapshot()
	if err != nil {
		return nil, fmt.Errorf("snapshot tools before install: %w", err)
	}

	if _, err := a.scanner.ScanAll(); err != nil {
		return nil, fmt.Errorf("scan tools after install: %w", err)
	}

	after, err := a.tools.ListTools("all")
	if err != nil {
		return nil, fmt.Errorf("list tools after install: %w", err)
	}

	var newTools []model.ToolInfo
	for _, t := range after {
		if _, exists := before[t.ID]; !exists {
			newTools = append(newTools, t)
		}
	}

	if len(newTools) == 0 {
		return nil, fmt.Errorf("安装成功，但未找到可注册的 Marcus 工具。请确认包包含 marcus entry point 或 marcus 配置")
	}

	if a.storeInstaller != nil {
		for _, t := range newTools {
			if err := a.storeInstaller.RecordInstalled(t.ID, t.Version); err != nil {
				a.writeLog("WARNING", fmt.Sprintf("record local install %s: %v", t.ID, err))
			}
		}
	}

	return newTools, nil
}

func (a *App) toolIDsSnapshot() (map[string]struct{}, error) {
	tools, err := a.tools.ListTools("all")
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		ids[t.ID] = struct{}{}
	}
	return ids, nil
}
