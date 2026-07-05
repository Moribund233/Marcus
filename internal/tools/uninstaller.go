package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"Marcus/internal/executil"
	"Marcus/internal/model"
)

type Uninstaller struct {
	registry *Registry
	toolsDir string
	lang     string
}

func NewUninstaller(registry *Registry, toolsDir string) *Uninstaller {
	return &Uninstaller{registry: registry, toolsDir: toolsDir, lang: "zh-CN"}
}

// SetLanguage updates the language used for user-facing messages.
func (u *Uninstaller) SetLanguage(lang string) {
	u.lang = lang
}

func (u *Uninstaller) Uninstall(toolID string) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: toolID}

	tool, err := u.registry.GetTool(toolID)
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
	result.Message = model.Localize(u.lang, "uninstall.store.unregistered", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallUV(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if _, err := exec.LookPath("uv"); err != nil {
		result.Error = model.Localize(u.lang, "uninstall.uv.notFound")
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
	result.Message = model.Localize(u.lang, "uninstall.uv.success", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallBun(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if _, err := exec.LookPath("bun"); err != nil {
		result.Error = model.Localize(u.lang, "uninstall.bun.notFound")
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
	result.Message = model.Localize(u.lang, "uninstall.bun.success", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallBinary(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = model.Localize(u.lang, "uninstall.binary.success", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallManual(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = model.Localize(u.lang, "uninstall.manual.success", tool.DisplayName)
	return result, nil
}

func (u *Uninstaller) uninstallGeneric(tool *model.ToolInfo) (*model.UninstallResult, error) {
	result := &model.UninstallResult{ToolID: tool.ID}

	if err := u.deleteFromRegistry(tool.ID); err != nil {
		result.Error = fmt.Sprintf("remove registry: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = model.Localize(u.lang, "uninstall.generic.success", tool.DisplayName)
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
	return u.registry.DeleteTool(toolID)
}

func (u *Uninstaller) deleteFromStoreInstalled(toolID string) error {
	_, err := u.registry.DB().Exec("DELETE FROM store_installed WHERE plugin_id = ?", toolID)
	return err
}
