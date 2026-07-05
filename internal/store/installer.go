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

	"Marcus/internal/executil"
	"Marcus/internal/model"
)

type Installer struct {
	db       *sql.DB
	registry interface {
		UpsertTool(model.ToolInfo) error
		UpsertToolTx(*sql.Tx, model.ToolInfo) error
	}
	toolsDir string
	hc       *http.Client
}

func NewInstaller(db *sql.DB, registry interface {
	UpsertTool(model.ToolInfo) error
	UpsertToolTx(*sql.Tx, model.ToolInfo) error
}, toolsDir string) *Installer {
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

	// Record the store installation for update checks. The actual tool entry in
	// the tools table is created by the runtime scanner (ScanUV/ScanBun) after
	// the package has been installed into the runtime environment.
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

// RecordInstalled records a locally installed package in store_installed so it
// can participate in update checks.
func (inst *Installer) RecordInstalled(pluginID, version string) error {
	_, err := inst.db.Exec(
		`INSERT OR REPLACE INTO store_installed (plugin_id, version, installed_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		pluginID, version,
	)
	return err
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

	// Determine runtime from manifest or fall back to file detection.
	runtime := strings.ToLower(manifest.Runtime)
	if runtime == "" {
		switch {
		case hasPyprojectTOML(distDir), hasRequirementsTXT(distDir):
			runtime = "python"
		case hasPackageJSON(distDir):
			runtime = "bun"
		}
	}

	switch runtime {
	case "python":
		if _, err := exec.LookPath("uv"); err != nil {
			return fmt.Errorf("uv not found: uv 运行时未安装，无法安装 Python 工具。请在设置中安装 uv")
		}
		// Install the package as a uv-managed tool so the scanner can discover it.
		cmd := exec.Command("uv", "tool", "install", "--force", distDir)
		executil.HideWindow(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("uv tool install failed: %w", err)
		}
	case "bun":
		if _, err := exec.LookPath("bun"); err != nil {
			return fmt.Errorf("bun not found: bun 运行时未安装，无法安装 JS 工具。请在设置中安装 bun")
		}
		// Install dependencies into the package directory first. A subsequent
		// bun link only registers the directory in the global node_modules and
		// does not fetch dependencies on its own.
		localInstall := exec.Command("bun", "install", "--production")
		localInstall.Dir = distDir
		executil.HideWindow(localInstall)
		localInstall.Stdout = os.Stdout
		localInstall.Stderr = os.Stderr
		if err := localInstall.Run(); err != nil {
			return fmt.Errorf("bun install dependencies failed: %w", err)
		}

		// Link the package globally so the scanner can discover it via
		// ~/.bun/install/global/node_modules and a shim is created in ~/.bun/bin.
		// bun install -g <local dir> on Windows may try to copy files and fail
		// with EPERM; bun link creates a junction instead.
		link := exec.Command("bun", "link")
		link.Dir = distDir
		executil.HideWindow(link)
		link.Stdout = os.Stdout
		link.Stderr = os.Stderr
		if err := link.Run(); err != nil {
			return fmt.Errorf("bun link failed: %w", err)
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
