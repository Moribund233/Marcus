package sandbox

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"Marcus/internal/model"
)

type OutputHandler func(toolID string, line string)

type Manager struct {
	mu        sync.Mutex
	processes map[string]*Process
	limits    model.ResourceLimits
	onOutput  OutputHandler
}

type Process struct {
	Cmd     *exec.Cmd
	State   model.ProcessState
	Cancel  context.CancelFunc
	Stopped chan struct{}
}

func NewManager(limits model.ResourceLimits, onOutput OutputHandler) *Manager {
	return &Manager{
		processes: make(map[string]*Process),
		limits:    limits,
		onOutput:  onOutput,
	}
}

func (m *Manager) Start(manifest model.ToolManifest, args map[string]string) (*model.ProcessState, error) {
	m.cleanup()
	switch manifest.Contribution {
	case model.ContributionWeb:
		return m.startWeb(manifest)
	case model.ContributionTerminal:
		return m.startTerminal(manifest, args)
	case model.ContributionFile:
		return m.startTerminal(manifest, args)
	default:
		return nil, fmt.Errorf("unknown contribution type: %s", manifest.Contribution)
	}
}

func (m *Manager) Stop(toolID string) error {
	m.mu.Lock()
	p, ok := m.processes[toolID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("process not found: %s", toolID)
	}
	p.Cancel()
	select {
	case <-p.Stopped:
	case <-time.After(5 * time.Second):
		if p.Cmd.Process != nil {
			p.Cmd.Process.Kill()
		}
	}
	return nil
}

func (m *Manager) GetState(toolID string) *model.ProcessState {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.processes[toolID]
	if !ok {
		return &model.ProcessState{ToolID: toolID, Status: model.ProcessIdle}
	}
	return &p.State
}

func (m *Manager) RunningProcesses() []model.ProcessState {
	m.mu.Lock()
	defer m.mu.Unlock()
	var states []model.ProcessState
	for _, p := range m.processes {
		states = append(states, p.State)
	}
	return states
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.processes {
		p.Cancel()
		if p.Cmd.Process != nil {
			p.Cmd.Process.Kill()
		}
		delete(m.processes, id)
	}
}

func (m *Manager) track(proc *Process) {
	m.mu.Lock()
	m.processes[proc.State.ToolID] = proc
	m.mu.Unlock()
}

func (m *Manager) untrack(toolID string) {
	m.mu.Lock()
	delete(m.processes, toolID)
	m.mu.Unlock()
}

// cleanup removes exited/crashed processes from tracking.
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.processes {
		if p.State.Status == model.ProcessExited || p.State.Status == model.ProcessCrashed {
			delete(m.processes, id)
		}
	}
}

func freePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// ─── web mode ───────────────────────────────────────────────

func (m *Manager) startWeb(manifest model.ToolManifest) (*model.ProcessState, error) {
	if manifest.Web == nil {
		return nil, fmt.Errorf("web manifest is nil")
	}

	port := manifest.Web.Port
	if port == 0 {
		port = freePort()
		if port == 0 {
			return nil, fmt.Errorf("failed to allocate port")
		}
	}

	webPath, err := resolvePath(manifest.Web.StartCommand)
	if err != nil {
		return nil, fmt.Errorf("resolve web executable: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutOrDefault(m.limits.TimeoutSeconds))
	cmd := exec.CommandContext(ctx, webPath)
	cmd.Env = append(buildEnv(), fmt.Sprintf("MARCUS_PORT=%d", port))

	state := model.ProcessState{
		ToolID: manifest.ToolID,
		Status: model.ProcessLaunching,
		Port:   port,
	}
	now := time.Now()
	state.StartedAt = &now

	proc := &Process{
		Cmd:     cmd,
		State:   state,
		Cancel:  cancel,
		Stopped: make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start web process: %w", err)
	}
	state.PID = cmd.Process.Pid

	m.track(proc)

	go func() {
		defer close(proc.Stopped)

		// health check loop
		healthURL := fmt.Sprintf("http://127.0.0.1:%d%s", port, manifest.Web.HealthCheck)
		if manifest.Web.HealthCheck == "" {
			healthURL = fmt.Sprintf("http://127.0.0.1:%d/", port)
		}

		maxRetries := 15 // ~15s total
		healthy := false
		for range maxRetries {
			resp, err := http.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				healthy = true
				break
			}
			time.Sleep(1 * time.Second)
		}

		m.mu.Lock()
		if healthy {
			proc.State.Status = model.ProcessRunning
		} else {
			proc.State.Status = model.ProcessCrashed
			proc.State.ErrorLog = "health check timed out"
		}
		m.mu.Unlock()

		cmd.Wait()
		m.mu.Lock()
		proc.State.Status = model.ProcessExited
		if cmd.ProcessState != nil {
			proc.State.ExitCode = cmd.ProcessState.ExitCode()
		}
		now := time.Now()
		proc.State.StoppedAt = &now
		m.mu.Unlock()
	}()

	return &proc.State, nil
}

// ─── terminal / file mode ───────────────────────────────────

func (m *Manager) startTerminal(manifest model.ToolManifest, args map[string]string) (*model.ProcessState, error) {
	var cmdStr string
	var cmdArgs []string

	switch manifest.Contribution {
	case model.ContributionTerminal:
		if manifest.Terminal == nil {
			return nil, fmt.Errorf("terminal manifest is nil")
		}
		cmdStr = manifest.Terminal.Command
		for _, a := range manifest.Terminal.Args {
			if v, ok := args[a.Name]; ok {
				cmdArgs = append(cmdArgs, v)
			}
		}
	case model.ContributionFile:
		if manifest.File == nil {
			return nil, fmt.Errorf("file manifest is nil")
		}
		cmdStr = manifest.File.Command
		for _, a := range manifest.File.Args {
			if v, ok := args[a.Name]; ok {
				cmdArgs = append(cmdArgs, v)
			}
		}
	}

	fullPath, err := resolvePath(cmdStr)
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}
	timeout := timeoutOrDefault(m.limits.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, fullPath, cmdArgs...)
	cmd.Env = buildEnv()

	state := model.ProcessState{
		ToolID: manifest.ToolID,
		Status: model.ProcessLaunching,
	}
	now := time.Now()
	state.StartedAt = &now

	proc := &Process{
		Cmd:     cmd,
		State:   state,
		Cancel:  cancel,
		Stopped: make(chan struct{}),
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}
	state.PID = cmd.Process.Pid

	// assign to job object (windows only)
	if err := assignToJob(cmd.Process.Pid); err != nil {
		// non-fatal on Windows
	}

	m.track(proc)
	go m.streamOutput(proc, stdout, manifest.ToolID)
	go m.streamOutput(proc, stderr, manifest.ToolID)

	go func() {
		defer close(proc.Stopped)

		err := cmd.Wait()
		m.mu.Lock()
		proc.State.Status = model.ProcessExited
		if cmd.ProcessState != nil {
			proc.State.ExitCode = cmd.ProcessState.ExitCode()
		}
		if err != nil && ctx.Err() == context.DeadlineExceeded {
			proc.State.Status = model.ProcessCrashed
			proc.State.ErrorLog = "timeout"
		}
		now := time.Now()
		proc.State.StoppedAt = &now
		m.mu.Unlock()
	}()

	return &proc.State, nil
}

func (m *Manager) streamOutput(proc *Process, r interface{ Read([]byte) (int, error) }, toolID string) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			line := string(buf[:n])
			if m.onOutput != nil {
				m.onOutput(toolID, line)
			}
		}
		if err != nil {
			break
		}
	}
}

func timeoutOrDefault(t int) time.Duration {
	if t <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(t) * time.Second
}

// resolvePath finds an executable by name, searching system PATH first,
// then falling back to common runtime bin directories (uv, bun, etc.).
func resolvePath(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find %s: %w", name, err)
	}

	dirs := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, "AppData", "Roaming", "npm"),
	}
	if runtime.GOOS == "windows" {
		if up := os.Getenv("USERPROFILE"); up != "" {
			dirs = append(dirs, filepath.Join(up, ".local", "bin"))
		}
	}

	exts := []string{""}
	if runtime.GOOS == "windows" {
		exts = []string{".exe", ".cmd", ".bat", ""}
	}

	for _, dir := range dirs {
		for _, ext := range exts {
			full := filepath.Join(dir, name+ext)
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				return full, nil
			}
		}
	}
	return "", fmt.Errorf("find %s: not found in PATH or runtime directories", name)
}

// buildEnv returns the full environment for subprocesses, with common runtime
// bin directories (uv, bun, etc.) added to PATH.
func buildEnv() []string {
	env := os.Environ()
	home, err := os.UserHomeDir()
	if err != nil {
		return env
	}

	var extraDirs []string
	extraDirs = append(extraDirs,
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, "AppData", "Roaming", "npm"),
	)

	if runtime.GOOS == "windows" {
		if up := os.Getenv("USERPROFILE"); up != "" {
			extraDirs = append(extraDirs, filepath.Join(up, ".local", "bin"))
		}
	}

	var existing []string
	for _, d := range extraDirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			existing = append(existing, d)
		}
	}
	if len(existing) == 0 {
		return env
	}

	currentPath := os.Getenv("PATH")
	newPath := strings.Join(append(existing, currentPath), string(filepath.ListSeparator))

	// find PATH entry (case-insensitive on Windows) and replace it
	for i, e := range env {
		key, _, ok := strings.Cut(e, "=")
		if ok && strings.EqualFold(key, "PATH") {
			env[i] = key + "=" + newPath
			return env
		}
	}
	// no PATH found, append it
	return append(env, "PATH="+newPath)
}


