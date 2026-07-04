package tools

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"Marcus/internal/model"
)

type Scanner struct {
	registry *Registry
	toolsDir string
}

func NewScanner(registry *Registry, toolsDir string) *Scanner {
	return &Scanner{registry: registry, toolsDir: toolsDir}
}

func toolID(name, source string) string {
	raw := fmt.Sprintf("%s:%s", source, name)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:16])
}

func (s *Scanner) ScanAll() ([]model.ToolInfo, error) {
	all := []model.ToolInfo{}

	uvTools, _ := s.ScanUV()
	all = append(all, uvTools...)

	bunTools, _ := s.ScanBun()
	all = append(all, bunTools...)

	binTools, _ := s.ScanBinary()
	all = append(all, binTools...)

	for _, t := range all {
		if err := s.registry.UpsertTool(t); err != nil {
			return nil, fmt.Errorf("upsert tool %s: %w", t.Name, err)
		}
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
		return nil, nil
	}

	out, err := exec.Command(uvPath, "tool", "list", "--show-paths").Output()
	if err != nil {
		return nil, fmt.Errorf("uv tool list: %w", err)
	}

	tools := parseUVToolList(string(out))
	results := []model.ToolInfo{}

	for _, uvTool := range tools {
		manifest, err := s.readUvManifest(uvTool)
		if err != nil {
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

	return results, nil
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

	python := filepath.Join(tool.EnvPath, "Scripts", "python.exe")
	if _, err := os.Stat(python); err != nil {
		python = filepath.Join(tool.EnvPath, "bin", "python")
		if _, err := os.Stat(python); err != nil {
			return nil, fmt.Errorf("python not found in env %s", tool.EnvPath)
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
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("execute entry point for %s: %w", tool.Name, err)
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "NO_MARCUS_ENTRYPOINT" {
		return nil, fmt.Errorf("no marcus entry point for %s", tool.Name)
	}

	var m model.ToolManifest
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		return nil, fmt.Errorf("parse manifest for %s: %w", tool.Name, err)
	}
	if m.DisplayName == "" {
		return nil, fmt.Errorf("manifest missing display_name")
	}
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
		results = append(results, t)
	}

	return results, nil
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

func (s *Scanner) ScanBinary() ([]model.ToolInfo, error) {
	entries, err := os.ReadDir(s.toolsDir)
	if err != nil {
		return nil, nil
	}

	results := []model.ToolInfo{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		binaryPath := filepath.Join(s.toolsDir, entry.Name())
		manifest, err := s.readBinaryManifest(binaryPath)
		if err != nil {
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
	return results, nil
}

func (s *Scanner) readBinaryManifest(path string) (*model.ToolManifest, error) {
	out, err := exec.Command(path, "--marcus-manifest").Output()
	if err != nil {
		return nil, err
	}
	var m model.ToolManifest
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func manifestJSON(m *model.ToolManifest) string {
	data, _ := json.Marshal(m)
	return string(data)
}


