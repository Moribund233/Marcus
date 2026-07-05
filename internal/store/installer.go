package store

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"Marcus/internal/model"
)

type Installer struct {
	db       *sql.DB
	registry interface {
		UpsertTool(model.ToolInfo) error
	}
	toolsDir string
	hc       *http.Client
}

func NewInstaller(db *sql.DB, registry interface{ UpsertTool(model.ToolInfo) error }, toolsDir string) *Installer {
	return &Installer{
		db:       db,
		registry: registry,
		toolsDir: toolsDir,
		hc:       &http.Client{Timeout: 120 * time.Second},
	}
}

// Install downloads a .marcus-plugin from the given URL, extracts it, and
// registers the tool in Marcus.
func (inst *Installer) Install(pluginID, version, downloadURL string) (*model.InstallResult, error) {
	result := &model.InstallResult{PluginID: pluginID, Version: version}

	pkgData, err := inst.downloadPackage(downloadURL)
	if err != nil {
		result.Error = fmt.Sprintf("download failed: %v", err)
		return result, fmt.Errorf("download %s: %w", pluginID, err)
	}

	manifest, err := extractManifest(pkgData)
	if err != nil {
		result.Error = fmt.Sprintf("extract manifest: %v", err)
		return result, fmt.Errorf("extract manifest from %s: %w", pluginID, err)
	}

	if manifest.ID == "" {
		manifest.ID = pluginID
	}

	installPath := filepath.Join(inst.toolsDir, pluginID)
	if err := os.RemoveAll(installPath); err != nil {
		result.Error = fmt.Sprintf("clean install dir: %v", err)
		return result, fmt.Errorf("clean %s: %w", installPath, err)
	}
	if err := os.MkdirAll(installPath, 0755); err != nil {
		result.Error = fmt.Sprintf("create install dir: %v", err)
		return result, fmt.Errorf("mkdir %s: %w", installPath, err)
	}

	if err := extractPackage(pkgData, installPath); err != nil {
		result.Error = fmt.Sprintf("extract package: %v", err)
		os.RemoveAll(installPath)
		return result, fmt.Errorf("extract %s: %w", pluginID, err)
	}

	if err := inst.installDeps(pluginID, manifest); err != nil {
		result.Error = fmt.Sprintf("install deps: %v", err)
		return result, fmt.Errorf("deps %s: %w", pluginID, err)
	}

	manifestJSON, _ := json.Marshal(manifest)
	toolInfo := model.ToolInfo{
		ID:           pluginID,
		Name:         pluginID,
		DisplayName:  manifest.DisplayName,
		Description:  manifest.Description,
		Icon:         manifest.Icon,
		Category:     firstNonEmpty(manifest.Category, "other"),
		Version:      version,
		Source:       model.SourceManual,
		Contribution: manifest.Contribution,
		PackagePath:  installPath,
		Manifest:     string(manifestJSON),
		EntryPoint:   filepath.Join(installPath, "dist"),
		Enabled:      true,
	}
	if err := inst.registry.UpsertTool(toolInfo); err != nil {
		result.Error = fmt.Sprintf("register tool: %v", err)
		return result, fmt.Errorf("register %s: %w", pluginID, err)
	}

	_, err = inst.db.Exec(
		`INSERT OR REPLACE INTO store_installed (plugin_id, version, installed_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		pluginID, version,
	)
	if err != nil {
		result.Error = fmt.Sprintf("record install: %v", err)
		return result, fmt.Errorf("record install %s: %w", pluginID, err)
	}

	result.Success = true
	return result, nil
}

func (inst *Installer) downloadPackage(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := inst.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func extractManifest(data []byte) (*model.ToolManifest, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, f := range zr.File {
		if f.Name == "marcus-plugin.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			var m model.ToolManifest
			if err := json.NewDecoder(rc).Decode(&m); err != nil {
				return nil, fmt.Errorf("decode manifest: %w", err)
			}
			return &m, nil
		}
	}

	return nil, fmt.Errorf("marcus-plugin.json not found in package")
}

func extractPackage(data []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(fpath), 0755)

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(fpath)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (inst *Installer) installDeps(pluginID string, manifest *model.ToolManifest) error {
	distDir := filepath.Join(inst.toolsDir, pluginID, "dist")

	switch manifest.Contribution {
	case model.ContributionWeb, model.ContributionTerminal:
		if hasPackageJSON(distDir) {
			cmd := exec.Command("bun", "install")
			cmd.Dir = distDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("bun install: %w", err)
			}
		}
		if hasRequirementsTXT(distDir) || hasPyprojectTOML(distDir) {
			cmd := exec.Command("uv", "pip", "install", "-r", "requirements.txt")
			cmd.Dir = distDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("uv pip install: %w", err)
			}
		}
	}

	return nil
}

func hasPackageJSON(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "package.json"))
	return err == nil
}

func hasRequirementsTXT(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "requirements.txt"))
	return err == nil
}

func hasPyprojectTOML(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "pyproject.toml"))
	return err == nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
