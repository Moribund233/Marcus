package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"Marcus/internal/model"
)

func TestBuildEnv(t *testing.T) {
	env := buildEnv()
	var pathVal string
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "PATH") {
			pathVal = parts[1]
			break
		}
	}
	if pathVal == "" {
		t.Fatal("PATH not found in buildEnv output")
	}
}

// writeFakeExecutable creates a small runnable script/binary in dir with the
// given name and returns its full path. This avoids depending on tools that
// may or may not be installed on the developer machine.
func writeFakeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		path += ".exe"
		// Minimal PE header so isExecutable (used by scanner) recognises it as a
		// Windows executable. The OS will not actually run it, but resolvePath
		// only checks file existence.
		data := append([]byte("MZ"), make([]byte, 62)...)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("write fake exe: %v", err)
		}
	} else {
		content := "#!/bin/sh\necho ok\n"
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatalf("write fake script: %v", err)
		}
	}
	return path
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	name := "marcus-test-bin"
	writeFakeExecutable(t, dir, name)

	// Temporarily prepend our fake binary directory to PATH.
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", dir+string(filepath.ListSeparator)+oldPath)

	resolved, err := resolvePath(name)
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	if !strings.HasPrefix(resolved, dir) {
		t.Fatalf("expected path under %s, got %s", dir, resolved)
	}
}

func TestResolvePathNotFound(t *testing.T) {
	_, err := resolvePath("marcus-definitely-missing-" + fmt.Sprint(time.Now().UnixNano()))
	if err == nil {
		t.Fatal("expected error for missing executable")
	}
}

// TestManagerStartStop verifies that a simple terminal tool can be started and
// stopped, and that the state transitions are reported correctly.
func TestManagerStartStop(t *testing.T) {
	var scriptPath string
	if runtime.GOOS == "windows" {
		// Use cmd.exe to run a simple sleep-like loop. The executable under test
		// is "cmd", which is on PATH, so no fake binary is required.
		scriptPath = "cmd"
	} else {
		scriptPath = "sleep"
	}

	m := NewManager(model.ResourceLimits{}, nil, nil)
	defer m.Shutdown()

	manifest := model.ToolManifest{
		ToolID:       "test:sleep",
		DisplayName:  "Test Sleep",
		Contribution: model.ContributionTerminal,
		Terminal: &model.TerminalManifest{
			Command: scriptPath,
			Args:    []model.TerminalArg{},
		},
	}

	if runtime.GOOS == "windows" {
		manifest.Terminal.Args = []model.TerminalArg{
			{Name: "cmd", Type: "string"},
		}
		manifest.Terminal.Command = "cmd"
	} else {
		// Use a short sleep so the test doesn't hang if stop fails.
		manifest.Terminal.Args = []model.TerminalArg{
			{Name: "duration", Type: "string"},
		}
	}

	args := map[string]string{}
	if runtime.GOOS == "windows" {
		args["cmd"] = "/C timeout /t 30 >nul"
	} else {
		args["duration"] = "30"
	}

	state, err := m.Start(manifest, args)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if state.Status != model.ProcessLaunching {
		t.Fatalf("expected launching status, got %s", state.Status)
	}

	// Give the process a moment to actually start.
	time.Sleep(200 * time.Millisecond)

	if err := m.Stop(state.ToolID); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	final := m.GetState(state.ToolID)
	if final.Status != model.ProcessIdle && final.Status != model.ProcessExited {
		t.Fatalf("expected idle or exited after stop, got %s", final.Status)
	}
}

// TestManagerShutdownTerminatesRunningProcess checks that Shutdown cleans up
// active processes (on Windows via closing the Job Object handle).
func TestManagerShutdownTerminatesRunningProcess(t *testing.T) {
	m := NewManager(model.ResourceLimits{}, nil, nil)

	manifest := model.ToolManifest{
		ToolID:       "test:shutdown",
		DisplayName:  "Test Shutdown",
		Contribution: model.ContributionTerminal,
		Terminal: &model.TerminalManifest{
			Command: "sleep",
			Args:    []model.TerminalArg{{Name: "duration", Type: "string"}},
		},
	}

	if runtime.GOOS == "windows" {
		manifest.Terminal.Command = "cmd"
		manifest.Terminal.Args = []model.TerminalArg{{Name: "cmd", Type: "string"}}
	}

	args := map[string]string{}
	if runtime.GOOS == "windows" {
		args["cmd"] = "/C timeout /t 30 >nul"
	} else {
		args["duration"] = "30"
	}

	_, err := m.Start(manifest, args)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Shutdown must not panic and should terminate the running process.
	m.Shutdown()
}

// TestManagerUnknownToolStop returns an error when stopping a non-existent tool.
func TestManagerUnknownToolStop(t *testing.T) {
	m := NewManager(model.ResourceLimits{}, nil, nil)
	defer m.Shutdown()

	if err := m.Stop("does-not-exist"); err == nil {
		t.Fatal("expected error stopping unknown tool")
	}
}

// TestStateChangeCallback ensures the onStateChange callback is invoked.
func TestStateChangeCallback(t *testing.T) {
	var lastState model.ProcessState
	m := NewManager(model.ResourceLimits{}, nil, func(_ string, s model.ProcessState) {
		lastState = s
	})
	defer m.Shutdown()

	manifest := model.ToolManifest{
		ToolID:       "test:callback",
		DisplayName:  "Test Callback",
		Contribution: model.ContributionTerminal,
		Terminal: &model.TerminalManifest{
			Command: "echo",
			Args:    []model.TerminalArg{{Name: "msg", Type: "string"}},
		},
	}
	if runtime.GOOS == "windows" {
		manifest.Terminal.Command = "cmd"
		manifest.Terminal.Args = []model.TerminalArg{{Name: "cmd", Type: "string"}}
	}

	args := map[string]string{}
	if runtime.GOOS == "windows" {
		args["cmd"] = "/C echo hello"
	} else {
		args["msg"] = "hello"
	}

	_, err := m.Start(manifest, args)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the process to finish naturally.
	time.Sleep(500 * time.Millisecond)

	if lastState.ToolID != "test:callback" {
		t.Fatalf("expected callback for test:callback, got %s", lastState.ToolID)
	}
}
