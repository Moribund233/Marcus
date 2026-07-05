package app

import (
	"fmt"

	"Marcus/internal/model"
)

// ─── Store (Plugin Marketplace) ─────────────────────────────

func (a *App) StoreSync() (*model.StoreIndex, error) {
	if a.storeClient == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeClient.Sync()
}

func (a *App) StoreListPlugins() ([]model.StorePlugin, error) {
	if a.storeClient == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeClient.ListPlugins()
}

func (a *App) StoreSearchPlugins(query string) ([]model.StorePlugin, error) {
	if a.storeClient == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeClient.SearchPlugins(query)
}

func (a *App) StoreInstall(pluginID, version string) (*model.InstallResult, error) {
	if a.storeInstaller == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	plugin, err := a.storeClient.GetCachedPlugin(pluginID)
	if err != nil {
		return nil, fmt.Errorf("lookup plugin: %w", err)
	}

	ver, ok := plugin.Versions[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found for plugin %s", version, pluginID)
	}

	before, err := a.toolIDsSnapshot()
	if err != nil {
		return nil, fmt.Errorf("snapshot tools before install: %w", err)
	}

	result, err := a.storeInstaller.Install(pluginID, version, ver.DownloadURL)
	if err != nil || !result.Success {
		return result, err
	}

	if a.scanner != nil {
		if _, scanErr := a.scanner.ScanAll(); scanErr != nil {
			// Record as pending so the next startup/scan can retry discovery.
			if a.storeClient != nil {
				_ = a.storeClient.RecordPendingInstall(pluginID, version)
			}
			result.Success = false
			result.Error = fmt.Sprintf("安装包已下载，但扫描工具时失败：%v", scanErr)
			return result, nil
		}
	}

	after, err := a.tools.ListTools("all")
	if err != nil {
		return nil, fmt.Errorf("list tools after install: %w", err)
	}

	var newTools []model.ToolInfo
	for _, t := range after {
		if _, exists := before[t.ID]; !exists {
			newTools = append(newTools, t)
		}
	}

	if len(newTools) == 0 {
		if a.storeClient != nil {
			_ = a.storeClient.RecordPendingInstall(pluginID, version)
		}
		result.Success = false
		result.Error = "安装包已下载，但未找到可注册的 Marcus 工具。请检查插件包是否包含正确的 marcus entry point。"
		return result, nil
	}

	// Clear any pending install entry since discovery succeeded.
	if a.storeClient != nil {
		_ = a.storeClient.ClearPendingInstall(pluginID)
	}

	return result, nil
}

func (a *App) StoreCheckUpdates() ([]model.UpdateCheckResult, error) {
	if a.storeUpdater == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return a.storeUpdater.CheckUpdates()
}

func (a *App) StoreHasUpdates() (bool, error) {
	if a.storeUpdater == nil {
		return false, fmt.Errorf("store not initialized")
	}
	return a.storeUpdater.HasUpdates()
}

// retryPendingInstalls re-scans for tools from any store installs that
// previously succeeded in download but failed in discovery. This is called
// once on startup in a background goroutine.
func (a *App) retryPendingInstalls() {
	if a.storeClient == nil || a.scanner == nil {
		return
	}

	pending, err := a.storeClient.ListPendingInstalls()
	if err != nil || len(pending) == 0 {
		return
	}

	a.writeLog("INFO", fmt.Sprintf("retrying %d pending store installs", len(pending)))

	// Trigger a full scan; if tools are discovered, the pending entries
	// will be cleared by the normal install flow on the next successful scan.
	if _, scanErr := a.scanner.ScanAll(); scanErr != nil {
		a.writeLog("WARNING", fmt.Sprintf("pending install scan retry failed: %v", scanErr))
		return
	}

	// Clear all pending entries after a successful scan.
	for _, p := range pending {
		_ = a.storeClient.ClearPendingInstall(p.PluginID)
	}
}
