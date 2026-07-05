package tools

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"Marcus/internal/executil"
	"Marcus/internal/model"
)

type Uninstaller struct {
	db       *sql.DB
	toolsDir string
}

func NewUninstaller(db *sql.DB, toolsDir string) *Uninstaller {
	return &Uninstaller{db: db, toolsDir: toolsDir}
}

func (u *Uninstaller) Uninstall(toolID string) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: toolID}

	tool, err := getToolInfo(u.db, toolID)
	if err != nil {
		result.Error = fmt.Sprintf("tool not found: %v", err)
		return result, fmt.Errorf("find tool %s: %w", toolID, err)
	}

	switch tool.Source {
	case model.SourceStore:
		return u.uninstallStore(tool)
	case model.SourcePython:
		return u.uninstallUV(tool)
	case model.SourceJS:
		return u.uninstallBun(tool)
	case model.SourceBinary:
		return u.uninstallBinary(tool)
	case model.SourceManual:
		return u.uninstallManual(tool)
	default:
		return u.uninstallGeneric(tool)
	}
}

func (u *Uninstaller) uninstallStore(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if tool.PackagePath != "" {
		if err := os.RemoveAll(tool.PackagePath); err != nil {
			result.Error = fmt.Sprintf("clean install dir: %v", err)
			return result, err
		}
	}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	if err := u.deleteFromStoreInstalled(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove store record: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("已卸载工具 %s", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallUV(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if _, err := exec.LookPath("uv"); err != nil {
		result.Error = "uv 未安装，无法通过包管理器卸载。已从工具列表移除，建议手动清理 uv 环境"
		return u.deleteFromRegistryOnly(tool, result)
	}

	cmd := exec.Command("uv", "tool", "uninstall", tool.Name)
	executil.HideWindow(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		result.Error = fmt.Sprintf("uv uninstall failed: %v", err)
		return u.deleteFromRegistryOnly(tool, result)
	}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	if err := u.deleteFromStoreInstalled(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove store record: %v", err)
		return result, err
	}

	if tool.PackagePath != "" {
		_ = os.RemoveAll(tool.PackagePath)
	} else {
		_ = os.RemoveAll(filepath.Join(u.toolsDir, tool.Name))
	}

	result.Success = true
	result.Message = fmt.Sprintf("已通过 uv 卸载工具 %s", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallBun(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if _, err := exec.LookPath("bun"); err != nil {
		result.Error = "bun 未安装，无法通过包管理器卸载。已从工具列表移除，建议手动清理 bun 环境"
		return u.deleteFromRegistryOnly(tool, result)
	}

	cmd := exec.Command("bun", "uninstall", "-g", tool.Name)
	executil.HideWindow(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		result.Error = fmt.Sprintf("bun uninstall failed: %v", err)
		return u.deleteFromRegistryOnly(tool, result)
	}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	if err := u.deleteFromStoreInstalled(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove store record: %v", err)
		return result, err
	}

	if tool.PackagePath != "" {
		_ = os.RemoveAll(tool.PackagePath)
	} else {
		_ = os.RemoveAll(filepath.Join(u.toolsDir, tool.Name))
	}

	result.Success = true
	result.Message = fmt.Sprintf("已通过 bun 卸载工具 %s", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallBinary(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("已从工具列表移除 %s。原始文件未删除，请手动清理", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallManual(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("已移除手动添加的工具 %s", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallGeneric(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("已从工具列表移除 %s", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) deleteFromRegistryOnly(tool *model.ToolInfo, result *model.UninstallResult) (*model.UninstallResult, error) {
	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	if err := u.deleteFromStoreInstalled(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove store record: %v", err)
		return result, err
	}

	result.Success = true
	return result, nil
}

func (u *Uninstaller) deleteFromRegistry(toolID string) error {
	_, err := u.db.Exec("DELETE FROM tools WHERE id = ?", toolID)
	return err
}

func (u *Uninstaller) deleteFromStoreInstalled(toolID string) error {
	_, err := u.db.Exec("DELETE FROM store_installed WHERE plugin_id = ?", toolID)
	return err
}

func getToolInfo(db *sql.DB, toolID string) (*model.ToolInfo, error) {
	var t model.ToolInfo
	var enabled int

	err := db.QueryRow(`
		SELECT id, name, display_name, description, icon, category, version, source,
		       contribution, package_path, manifest, entry_point, enabled
		FROM tools WHERE id = ?
	`, toolID).Scan(
		&t.ID, &t.Name, &t.DisplayName, &t.Description,
		&t.Icon, &t.Category, &t.Version, &t.Source,
		&t.Contribution, &t.PackagePath, &t.Manifest,
		&t.EntryPoint, &enabled,
	)
	if err != nil {
		return nil, err
	}
	t.Enabled = enabled != 0
	return &t, nil
}
