package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"Marcus/internal/executil"
	"Marcus/internal/model"
)

// maxIDLen limits the readable portion of a tool ID to prevent absurdly long
// slugs from tools with very long names.
const maxIDLen = 48

type Scanner struct {
	registry     *Registry
	toolsDir     string
	uvCache      sync.Map
}

func NewScanner(registry *Registry, toolsDir string) *Scanner {
	return &Scanner{registry: registry, toolsDir: toolsDir}
}

func toolID(name, source string) string {
	slug := strings.ToLower(name)
	slug = strings.NewReplacer(" ", "_", "/", "_", ":", "_", "..", ".", "__", "_").Replace(slug)
	if len(slug) > maxIDLen {
		h := sha256.Sum256([]byte(slug))
		suffix := fmt.Sprintf("%x", h[:4])
		slug = slug[:maxIDLen-9] + ".." + suffix
	}
	return fmt.Sprintf("%s:%s", source, slug)
}

type scanResult struct {
	tools []model.ToolInfo
	err   error
}

func (s *Scanner) ScanAll() ([]model.ToolInfo, error) {
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		all  []model.ToolInfo
		errs []string
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		tools, err := s.ScanUV()
		mu.Lock()
		if err != nil {
			errs = append(errs, fmt.Sprintf("uv: %v", err))
		} else {
			all = append(all, tools...)
		}
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		tools, err := s.ScanBun()
		mu.Lock()
		if err != nil {
			errs = append(errs, fmt.Sprintf("bun: %v", err))
		} else {
			all = append(all, tools...)
		}
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		tools, err := s.ScanBinary()
		mu.Lock()
		if err != nil {
			errs = append(errs, fmt.Sprintf("binary: %v", err))
		} else {
			all = append(all, tools...)
		}
		mu.Unlock()
	}()
	wg.Wait()

	for _, t := range all {
		if err := s.registry.UpsertTool(t); err != nil {
			return nil, fmt.Errorf("upsert tool %s: %w", t.Name, err)
		}
	}

	if len(errs) > 0 {
		return all, fmt.Errorf("scan errors: %s", strings.Join(errs, "; "))
	}
	return all, nil
}

// ─── uv scanner ─────────────────────────────────────────────

type uvToolInfo struct {
	Name    string
	Version string
	EnvPath string
}

func (s *Scanner) ScanUV() ([]model.ToolInfo, error) {
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return nil, nil // runtime not available, skip silently
	}

	uvCmd := exec.Command(uvPath, "tool", "list", "--show-paths")
	executil.HideWindow(uvCmd)
	out, err := uvCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("uv tool list: %w", err)
	}

	tools := parseUVToolList(string(out))
	results := []model.ToolInfo{}

	var errs []error
	for _, uvTool := range tools {
		manifest, err := s.readUvManifest(uvTool)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", uvTool.Name, err))
			continue
		}

		t := model.ToolInfo{
			ID:      toolID(uvTool.Name, string(model.SourcePython)),
			Name:    uvTool.Name,
			Version: uvTool.Version,
			Source:  model.SourcePython,
			Enabled: true,
		}

		t.DisplayName = manifest.DisplayName
		if t.DisplayName == "" {
			t.DisplayName = uvTool.Name
		}
		t.Description = manifest.Description
		t.Icon = manifest.Icon
		t.Category = manifest.Category
		if t.Category == "" {
			t.Category = "other"
		}
		t.Contribution = manifest.Contribution
		t.Manifest = manifestJSON(manifest)

		results = append(results, t)
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("uv scan: %s", strings.Join(errorStrings(errs), "; "))
	}
	return results, nil
}

func errorStrings(errs []error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}

func parseUVToolList(output string) []uvToolInfo {
	var tools []uvToolInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		// format: "name v1.0.0 (C:\path\to\env)"
		if idx := strings.Index(line, " v"); idx > 0 {
			rest := line[idx+2:]
			name := line[:idx]

			version := ""
			envPath := ""
			if pIdx := strings.Index(rest, " ("); pIdx > 0 {
				version = rest[:pIdx]
				end := strings.LastIndex(rest, ")")
				if end > pIdx+2 {
					envPath = rest[pIdx+2 : end]
				}
			} else {
				version = rest
			}

			tools = append(tools, uvToolInfo{
				Name:    name,
				Version: version,
				EnvPath: envPath,
			})
		}
	}
	return tools
}

func (s *Scanner) readUvManifest(tool uvToolInfo) (*model.ToolManifest, error) {
	if tool.EnvPath == "" {
		return nil, fmt.Errorf("no env path for %s", tool.Name)
	}

	// Return cached manifest if available. The cache survives across ScanUV calls
	// so repeated scans after initial discovery avoid spawning Python for every tool.
	type cacheEntry struct {
		m   model.ToolManifest
		err error
	}
	cacheKey := tool.Name + "|" + tool.EnvPath
	if cached, ok := s.uvCache.Load(cacheKey); ok {
		entry := cached.(cacheEntry)
		if entry.err != nil {
			return nil, entry.err
		}
		return &entry.m, nil
	}

	python := filepath.Join(tool.EnvPath, "Scripts", "python.exe")
	if _, err := os.Stat(python); err != nil {
		python = filepath.Join(tool.EnvPath, "bin", "python")
		if _, err := os.Stat(python); err != nil {
			err := fmt.Errorf("python not found in env %s", tool.EnvPath)
			s.uvCache.Store(cacheKey, cacheEntry{err: err})
			return nil, err
		}
	}

	code := `
import importlib.metadata as md, json
eps = list(md.entry_points(group="marcus"))
if eps:
    print(json.dumps(eps[0].load()()))
else:
    print("NO_MARCUS_ENTRYPOINT")
`
	cmd := exec.Command(python, "-c", code)
	executil.HideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err := fmt.Errorf("execute entry point for %s: %w\noutput:\n%s", tool.Name, err, string(output))
		s.uvCache.Store(cacheKey, cacheEntry{err: err})
		return nil, err
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "NO_MARCUS_ENTRYPOINT" {
		err := fmt.Errorf("no marcus entry point for %s", tool.Name)
		s.uvCache.Store(cacheKey, cacheEntry{err: err})
		return nil, err
	}

	var m model.ToolManifest
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		err := fmt.Errorf("parse manifest for %s: %w", tool.Name, err)
		s.uvCache.Store(cacheKey, cacheEntry{err: err})
		return nil, err
	}
	if m.DisplayName == "" {
		err := fmt.Errorf("manifest missing display_name")
		s.uvCache.Store(cacheKey, cacheEntry{err: err})
		return nil, err
	}

	s.uvCache.Store(cacheKey, cacheEntry{m: m})
	return &m, nil
}

// ─── bun scanner ────────────────────────────────────────────

func (s *Scanner) ScanBun() ([]model.ToolInfo, error) {
	if _, err := exec.LookPath("bun"); err != nil {
		return nil, nil
	}

	home, _ := os.UserHomeDir()
	globalDir := filepath.Join(home, ".bun", "install", "global", "node_modules")

	entries, err := os.ReadDir(globalDir)
	if err != nil {
		return nil, nil
	}

	results := []model.ToolInfo{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		name := entry.Name()
		manifest, err := s.readBunManifest(name)
		if err != nil || manifest == nil {
			continue
		}

		t := model.ToolInfo{
			ID:           toolID(name, string(model.SourceJS)),
			Name:         name,
			DisplayName:  manifest.DisplayName,
			Description:  manifest.Description,
			Icon:         manifest.Icon,
			Category:     manifest.Category,
			Source:       model.SourceJS,
			Contribution: manifest.Contribution,
			Enabled:      true,
			Manifest:     manifestJSON(manifest),
		}
		if t.DisplayName == "" {
			t.DisplayName = name
		}

		// Validate tool health.
		health := s.checkBunToolHealth(name, manifest)
		t.Healthy = health.Healthy
		t.HealthError = health.Error
		t.HealthHint = health.Hint

		results = append(results, t)
	}

	return results, nil
}

// checkBunToolHealth validates that a discovered bun tool can actually run.
func (s *Scanner) checkBunToolHealth(name string, manifest *model.ToolManifest) model.ToolHealth {
	home, _ := os.UserHomeDir()
	bunBinDir := filepath.Join(home, ".bun", "bin")
	bunExe := filepath.Join(bunBinDir, "bun.exe")

	// Check if bun.exe is at the standard location.
	if _, err := os.Stat(bunExe); os.IsNotExist(err) {
		return model.ToolHealth{
			Healthy: false,
			Error:   fmt.Sprintf("%s 无法启动：bun 运行时不完整", name),
			Hint: "bun.exe 不在 ~/.bun/bin/ 目录中。\n" +
				"当前 bun 通过 npm 安装，其生成的二进制包装器无法正常运行。\n" +
				"解决方案：重新安装 bun\n" +
				"  powershell -c \"irm bun.sh/install.ps1 | iex\"",
		}
	}

	// Check the binary shim exists.
	bunExePath := filepath.Join(bunBinDir, name+".exe")
	if manifest.Contribution == model.ContributionTerminal || manifest.Contribution == model.ContributionFile {
		if _, err := os.Stat(bunExePath); os.IsNotExist(err) {
			return model.ToolHealth{
				Healthy: false,
				Error:   fmt.Sprintf("%s 的二进制包装器不存在", name),
				Hint:    fmt.Sprintf("未找到 %s。请尝试重新安装此工具：bun install -g %s", bunExePath, name),
			}
		}
	}

	// Check the entry JS file exists.
	targetJS := filepath.Join(home, ".bun", "install", "global", "node_modules", name, "index.js")
	if _, err := os.Stat(targetJS); os.IsNotExist(err) {
		targetJS = filepath.Join(home, ".bun", "install", "global", "node_modules", name, "main.js")
		if _, err := os.Stat(targetJS); os.IsNotExist(err) {
			return model.ToolHealth{
				Healthy: false,
				Error:   fmt.Sprintf("%s 的入口文件不存在", name),
				Hint:    "node_modules 可能已损坏。请运行：bun install --force",
			}
		}
	}

	// Quick smoke test: run --help or --version to verify the tool is responsive.
	if manifest.Terminal != nil && manifest.Terminal.Command != "" {
		cmdPath := filepath.Join(bunBinDir, manifest.Terminal.Command+".exe")
		if _, err := os.Stat(cmdPath); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			healthCmd := exec.CommandContext(ctx, cmdPath, "--help")
			executil.HideWindow(healthCmd)
			if err := healthCmd.Run(); err != nil {
				return model.ToolHealth{
					Healthy: false,
					Error:   fmt.Sprintf("%s 启动测试失败", name),
					Hint:    "工具包装器存在但无法正常执行。请尝试 bun install --force",
				}
			}
		}
	}

	return model.ToolHealth{Healthy: true}
}

func (s *Scanner) readBunManifest(name string) (*model.ToolManifest, error) {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".bun", "install", "global", "node_modules", name, "package.json"),
		filepath.Join(home, ".bun", "bin", "node_modules", name, "package.json"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var pkg struct {
			Marcus *model.ToolManifest `json:"marcus"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			return nil, err
		}
		if pkg.Marcus != nil {
			return pkg.Marcus, nil
		}
	}
	return nil, fmt.Errorf("no marcus manifest found for %s", name)
}

// ─── binary scanner ─────────────────────────────────────────

// isExecutable checks whether path looks like a runnable file by
// examining its leading magic bytes.  This avoids running arbitrary
// non‑executables (documents, scripts dropped by accident, etc.).
func isExecutable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return false
	}

	switch {
	// PE  (Windows)  — "MZ"
	case magic[0] == 'M' && magic[1] == 'Z':
		return true
	// ELF (Linux)
	case magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F':
		return true
	// shebang (Unix scripts)
	case magic[0] == '#' && magic[1] == '!':
		return true
	default:
		return false
	}
}

func (s *Scanner) ScanBinary() ([]model.ToolInfo, error) {
	entries, err := os.ReadDir(s.toolsDir)
	if err != nil {
		return nil, nil
	}

	var errs []error
	results := []model.ToolInfo{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		binaryPath := filepath.Join(s.toolsDir, entry.Name())

		if !isExecutable(binaryPath) {
			errs = append(errs, fmt.Errorf("skip non-executable: %s", entry.Name()))
			continue
		}

		manifest, err := s.readBinaryManifest(binaryPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}

		t := model.ToolInfo{
			ID:           toolID(entry.Name(), string(model.SourceBinary)),
			Name:         entry.Name(),
			DisplayName:  manifest.DisplayName,
			Icon:         manifest.Icon,
			Category:     manifest.Category,
			Contribution: manifest.Contribution,
			Source:       model.SourceBinary,
			PackagePath:  binaryPath,
			Manifest:     string(manifestJSON(manifest)),
			Enabled:      true,
		}
		if t.DisplayName == "" {
			t.DisplayName = entry.Name()
		}
		results = append(results, t)
	}

	if len(results) == 0 && len(errs) > 0 {
		return nil, errs[0]
	}
	return results, nil
}

func (s *Scanner) readBinaryManifest(path string) (*model.ToolManifest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--marcus-manifest")
	executil.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("exec manifest: %w", err)
	}
	var m model.ToolManifest
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

func manifestJSON(m *model.ToolManifest) string {
	data, _ := json.Marshal(m)
	return string(data)
}
