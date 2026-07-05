package app

import (
	"fmt"
	"os"
	"os/exec"

	"Marcus/internal/executil"
	"Marcus/internal/model"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ─── Runtime Status ─────────────────────────────────────────

func (a *App) GetRuntimeStatus() map[string]model.RuntimeInfo {
	return a.rt.CheckAll()
}

// ─── Runtime Installation ───────────────────────────────────

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
