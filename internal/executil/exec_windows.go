//go:build windows

package executil

import (
	"os/exec"
	"syscall"
)

// HideWindow 为 Windows 子进程设置隐藏窗口属性。
// 在打包后的 Wails 应用中启动后台命令时，可避免弹出终端窗口。
func HideWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
