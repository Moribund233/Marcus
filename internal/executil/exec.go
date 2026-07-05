package executil

import (
	"os/exec"
	"runtime"
	"syscall"
)

func HideWindow(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
}
