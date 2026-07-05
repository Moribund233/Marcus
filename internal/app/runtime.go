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
	lang := a.cfg.Language
	switch runtimeName {
	case "uv":
		if _, err := exec.LookPath("uv"); err == nil {
			return nil
		}
		if _, err := exec.LookPath("pip"); err != nil {
			return fmt.Errorf("%s", model.Localize(lang, "runtime.uv.pipNotFound"))
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
			return fmt.Errorf("%s", model.Localize(lang, "runtime.bun.npmNotFound"))
		}
		cmd := exec.Command("npm", "install", "-g", "bun")
		executil.HideWindow(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf("%s", fmt.Sprintf(model.Localize(lang, "runtime.unknown"), runtimeName))
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
	lang := a.cfg.Language

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
			emit("done", model.Localize(lang, "runtime.uv.alreadyInstalled"), 100)
			emitComplete(true, "")
			return
		}
	case "bun":
		if _, err := exec.LookPath("bun"); err == nil {
			emit("done", model.Localize(lang, "runtime.bun.alreadyInstalled"), 100)
			emitComplete(true, "")
			return
		}
	default:
		msg := fmt.Sprintf(model.Localize(lang, "runtime.unknown"), runtimeName)
		emit("error", msg, 0)
		emitComplete(false, msg)
		return
	}

	emit("starting", model.Localize(lang, "install.preparing"), 0)

	var cmd *exec.Cmd
	var label string
	switch runtimeName {
	case "uv":
		if _, err := exec.LookPath("pip"); err != nil {
			msg := model.Localize(lang, "runtime.uv.pipNotFound")
			emit("error", msg, 0)
			emitComplete(false, msg)
			return
		}
		cmd = exec.Command("pip", "install", "uv")
		label = "uv"
	case "bun":
		if _, err := exec.LookPath("npm"); err != nil {
			msg := model.Localize(lang, "runtime.bun.npmNotFound")
			emit("error", msg, 0)
			emitComplete(false, msg)
			return
		}
		cmd = exec.Command("npm", "install", "-g", "bun")
		label = "bun"
	}
	executil.HideWindow(cmd)

	emit("running", fmt.Sprintf(model.Localize(lang, "runtime.%s.installing"), label), 30)

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

	emit("done", fmt.Sprintf(model.Localize(lang, "runtime.%s.installSuccess"), label), 100)
	emitComplete(true, "")
}
