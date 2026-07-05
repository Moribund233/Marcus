//go:build !windows

package executil

import "os/exec"

// HideWindow 在非 Windows 平台上不执行任何操作。
func HideWindow(cmd *exec.Cmd) {}
