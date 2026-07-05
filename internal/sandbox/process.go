package sandbox

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"Marcus/internal/executil"
	"Marcus/internal/model"
)

type OutputHandler func(toolID string, line string)

type StateChangeHandler func(toolID string, state model.ProcessState)

type Manager struct {
	mu            sync.Mutex
	processes     map[string]*Process
	limits        model.ResourceLimits
	onOutput      OutputHandler
	onStateChange StateChangeHandler
}

type Process struct {
	Cmd       *exec.Cmd
	stateMu   sync.Mutex
	State     model.ProcessState
	Cancel    context.CancelFunc
	Stopped   chan struct{}
	jobHandle uintptr // Windows Job Object handle; closed on stop/shutdown to kill child tree
}

func NewManager(limits model.ResourceLimits, onOutput OutputHandler, onStateChange StateChangeHandler) *Manager {
	return &Manager{
		processes:     make(map[string]*Process),
		limits:        limits,
		onOutput:      onOutput,
		onStateChange: onStateChange,
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

	// Closing the Job Object handle triggers JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
	// terminating the whole child process tree on Windows before we wait.
	closeJobHandle(p.jobHandle)
	p.jobHandle = 0

	p.Cancel()
	select {
	case <-p.Stopped:
	case <-time.After(5 * time.Second):
		p.stateMu.Lock()
		if p.Cmd != nil && p.Cmd.Process != nil {
			p.Cmd.Process.Kill()
		}
		p.stateMu.Unlock()
	}
	return nil
}

func (m *Manager) GetState(toolID string) *model.ProcessState {
	m.mu.Lock()
	p, ok := m.processes[toolID]
	m.mu.Unlock()
	if !ok {
		return &model.ProcessState{ToolID: toolID, Status: model.ProcessIdle}
	}
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	copy := p.State
	return &copy
}

func (m *Manager) RunningProcesses() []model.ProcessState {
	m.mu.Lock()
	defer m.mu.Unlock()
	var states []model.ProcessState
	for _, p := range m.processes {
		p.stateMu.Lock()
		states = append(states, p.State)
		p.stateMu.Unlock()
	}
	return states
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.processes {
		closeJobHandle(p.jobHandle)
		p.jobHandle = 0
		p.Cancel()
		if p.Cmd != nil && p.Cmd.Process != nil {
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
		p.stateMu.Lock()
		done := p.State.Status == model.ProcessExited || p.State.Status == model.ProcessCrashed
		p.stateMu.Unlock()
		if done {
			delete(m.processes, id)
		}
	}
}

// setState safely updates a process's state under its own lock and notifies
// the manager's state-change callback.
func (m *Manager) setState(p *Process, updater func(s *model.ProcessState)) {
	p.stateMu.Lock()
	updater(&p.State)
	copy := p.State
	p.stateMu.Unlock()

	if m.onStateChange != nil {
		m.onStateChange(copy.ToolID, copy)
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
	executil.HideWindow(cmd)
	cmd.Env = append(buildEnv(), fmt.Sprintf("MARCUS_PORT=%d", port))

	proc := &Process{
		Cmd: cmd,
		State: model.ProcessState{
			ToolID:    manifest.ToolID,
			Status:    model.ProcessLaunching,
			Port:      port,
			StartedAt: time.Now().Format(time.DateTime),
		},
		Cancel:  cancel,
		Stopped: make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start web process: %w", err)
	}

	m.track(proc)
	m.setState(proc, func(s *model.ProcessState) {
		s.PID = cmd.Process.Pid
	})

	if job, err := assignToJob(cmd.Process.Pid, m.limits); err != nil {
		log.Printf("sandbox: assignToJob(%d): %v", cmd.Process.Pid, err)
	} else {
		proc.jobHandle = job
	}

	go func() {
		defer close(proc.Stopped)
		defer closeJobHandle(proc.jobHandle)

		healthURL := fmt.Sprintf("http://127.0.0.1:%d%s", port, manifest.Web.HealthCheck)
		if manifest.Web.HealthCheck == "" {
			healthURL = fmt.Sprintf("http://127.0.0.1:%d/", port)
		}

		maxRetries := 15
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

		m.setState(proc, func(s *model.ProcessState) {
			if healthy {
				s.Status = model.ProcessRunning
			} else {
				s.Status = model.ProcessCrashed
				s.ErrorLog = "health check timed out"
			}
		})

		cmd.Wait()
		m.setState(proc, func(s *model.ProcessState) {
			s.Status = model.ProcessExited
			if cmd.ProcessState != nil {
				s.ExitCode = cmd.ProcessState.ExitCode()
			}
			s.StoppedAt = time.Now().Format(time.DateTime)
		})
	}()

	proc.stateMu.Lock()
	copy := proc.State
	proc.stateMu.Unlock()
	return &copy, nil
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
	executil.HideWindow(cmd)
	cmd.Env = buildEnv()

	proc := &Process{
		Cmd: cmd,
		State: model.ProcessState{
			ToolID:    manifest.ToolID,
			Status:    model.ProcessLaunching,
			StartedAt: time.Now().Format(time.DateTime),
		},
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
		return nil, fmt.Errorf("start process: %w\n%s", err, diagnoseStartError(fullPath, err))
	}

	m.track(proc)
	m.setState(proc, func(s *model.ProcessState) {
		s.PID = cmd.Process.Pid
	})

	if job, err := assignToJob(cmd.Process.Pid, m.limits); err != nil {
		log.Printf("sandbox: assignToJob(%d): %v", cmd.Process.Pid, err)
	} else {
		proc.jobHandle = job
	}

	go m.streamOutput(proc, stdout, manifest.ToolID)
	go m.streamOutput(proc, stderr, manifest.ToolID)

	go func() {
		defer close(proc.Stopped)
		defer closeJobHandle(proc.jobHandle)

		err := cmd.Wait()
		m.setState(proc, func(s *model.ProcessState) {
			s.Status = model.ProcessExited
			if cmd.ProcessState != nil {
				s.ExitCode = cmd.ProcessState.ExitCode()
			}
			if err != nil && ctx.Err() == context.DeadlineExceeded {
				s.Status = model.ProcessCrashed
				s.ErrorLog = "timeout"
			}
			s.StoppedAt = time.Now().Format(time.DateTime)
		})
	}()
 
	proc.stateMu.Lock()
	copy := proc.State
	proc.stateMu.Unlock()
	return &copy, nil
}

func (m *Manager) streamOutput(_ *Process, r interface{ Read([]byte) (int, error) }, toolID string) {
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

// diagnoseStartError examines an exec start error and returns a user-friendly
// diagnostic hint with suggested fix steps.
func diagnoseStartError(fullPath string, err error) string {
	errStr := err.Error()
	home, _ := os.UserHomeDir()

	// bun binary wrapper failure patterns (from marcus-img2ascii.exe etc.).
	bunErrPatterns := []struct {
		pattern string
		hint    string
	}{
		{
			"Bun failed to remap this bin",
			"bun 生成的二进制包装器无法找到运行环境。\n" +
				"原因：bun.exe 不在 ~/.bun/bin/ 目录中。\n" +
				"解决方案：\n" +
				"  1. 如果 bun 通过 npm 安装，请重新使用官方脚本安装：\n" +
				"     powershell -c \"irm bun.sh/install.ps1 | iex\"\n" +
				"  2. 或运行 bun install --force 修复 node_modules",
		},
		{
			"could not find node_modules path",
			"bun 无法定位 node_modules 路径。\n" +
				"请运行：bun install --force",
		},
		{
			"bun is not installed in %PATH%",
			"bun 不在 PATH 环境变量中。\n" +
				"请将以下目录添加到 PATH：\n" +
				"  " + filepath.Join(home, ".bun", "bin") + "\n" +
				"或重新安装 bun：\n" +
				"  powershell -c \"irm bun.sh/install.ps1 | iex\"",
		},
	}

	// Check for bun wrapper errors embedded in the executable output.
	if strings.Contains(errStr, "cannot run executable") ||
		strings.Contains(errStr, "is not recognized") ||
		strings.Contains(errStr, "could not create process") {
		return "\n⚠️ 提示：可执行文件无法运行。"
	}

	for _, bp := range bunErrPatterns {
		if strings.Contains(errStr, bp.pattern) {
			return "\n⚠️ " + bp.hint
		}
	}

	// Generic diagnostics based on file type.
	if strings.HasSuffix(strings.ToLower(fullPath), ".exe") {
		if _, statErr := os.Stat(fullPath); statErr != nil {
			return "\n⚠️ 文件不存在或无法访问：" + fullPath
		}
	}

	return ""
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
	// Provide diagnostic hints for common bun tool scenarios.
	bunHint := ""
	bunExe := filepath.Join(home, ".bun", "bin", "bun.exe")
	if _, err := os.Stat(bunExe); os.IsNotExist(err) {
		bunHint = "\n⚠️ bun.exe 不在 ~/.bun/bin/ 中（bun 可能通过 npm 安装）。\n" +
			"bun 二进制包装器需要 bun.exe 在同一目录才能运行。\n" +
			"解决方案：使用官方安装脚本重新安装 bun：\n" +
			"  powershell -c \"irm bun.sh/install.ps1 | iex\""
	} else {
		// bun.exe exists but the tool shim doesn't — suggest reinstall.
		toolShim := filepath.Join(home, ".bun", "bin", name+".exe")
		if _, err := os.Stat(toolShim); os.IsNotExist(err) {
			bunHint = "\n⚠️ 未找到工具包装器 " + name + ".exe。\n" +
				"请尝试重新安装此工具：bun install -g " + name
		}
	}
	return "", fmt.Errorf("find %s: not found in PATH or runtime directories%s", name, bunHint)
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

	for i, e := range env {
		key, _, ok := strings.Cut(e, "=")
		if ok && strings.EqualFold(key, "PATH") {
			env[i] = key + "=" + newPath
			return env
		}
	}
	return append(env, "PATH="+newPath)
}
