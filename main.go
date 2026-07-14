package main

import (
	"embed"

	"Marcus/internal/config"
	"Marcus/internal/llm"
	"Marcus/internal/model"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:           "Marcus",
		Width:           1024,
		Height:          768,
		Frameless:       true,
		CSSDragProperty: "--wails-draggable",
		CSSDragValue:    "drag",
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// ─── Wails Bind Delegates ───────────────────────────────────

func (a *App) GetTools(category string) ([]model.ToolInfo, error) {
	return a.impl.GetTools(category)
}

func (a *App) LaunchTool(toolID string, args map[string]string) (*model.ProcessState, error) {
	return a.impl.LaunchTool(toolID, args)
}

func (a *App) StopTool(toolID string) error {
	return a.impl.StopTool(toolID)
}

func (a *App) GetToolState(toolID string) *model.ProcessState {
	return a.impl.GetToolState(toolID)
}

func (a *App) GetRuntimeStatus() map[string]model.RuntimeInfo {
	return a.impl.GetRuntimeStatus()
}

func (a *App) RefreshTools() ([]model.ToolInfo, error) {
	return a.impl.RefreshTools()
}

func (a *App) AddManualTool(name, command, argType string) (model.ToolInfo, error) {
	return a.impl.AddManualTool(name, command, argType)
}

func (a *App) DeleteTool(toolID string) error {
	return a.impl.DeleteTool(toolID)
}

func (a *App) UninstallTool(toolID string) (*model.UninstallResult, error) {
	return a.impl.UninstallTool(toolID)
}

func (a *App) GetToolLogs(toolID string, limit int) ([]model.ProcessState, error) {
	return a.impl.GetToolLogs(toolID, limit)
}

func (a *App) GetRecentTools(limit int) ([]model.ToolInfo, error) {
	return a.impl.GetRecentTools(limit)
}

func (a *App) InstallToolPackage(path string) error {
	return a.impl.InstallToolPackage(path)
}

func (a *App) InstallToolPackageAsync(path string) error {
	return a.impl.InstallToolPackageAsync(path)
}

func (a *App) GetConfig() *config.Config {
	return a.impl.GetConfig()
}

func (a *App) SaveConfig(cfg config.Config) error {
	return a.impl.SaveConfig(cfg)
}

// ─── Store (Plugin Marketplace) ──────────────────────────────

func (a *App) StoreSync() (*model.StoreIndex, error) {
	return a.impl.StoreSync()
}

func (a *App) StoreListPlugins() ([]model.StorePlugin, error) {
	return a.impl.StoreListPlugins()
}

func (a *App) StoreSearchPlugins(query string) ([]model.StorePlugin, error) {
	return a.impl.StoreSearchPlugins(query)
}

func (a *App) StoreInstall(pluginID, version string) (*model.InstallResult, error) {
	return a.impl.StoreInstall(pluginID, version)
}

func (a *App) StoreCheckUpdates() ([]model.UpdateCheckResult, error) {
	return a.impl.StoreCheckUpdates()
}

func (a *App) StoreHasUpdates() (bool, error) {
	return a.impl.StoreHasUpdates()
}

func (a *App) InstallRuntime(runtimeName string) error {
	return a.impl.InstallRuntime(runtimeName)
}

func (a *App) InstallRuntimeAsync(runtimeName string) error {
	return a.impl.InstallRuntimeAsync(runtimeName)
}

// ─── Agent / Chat ───────────────────────────────────────────

func (a *App) CreateConversation(title string) (*model.Conversation, error) {
	return a.impl.CreateConversation(title)
}

func (a *App) ListConversations(limit int) ([]model.Conversation, error) {
	return a.impl.ListConversations(limit)
}

func (a *App) GetConversationMessages(conversationID string) ([]model.ConversationMessage, error) {
	return a.impl.GetConversationMessages(conversationID)
}

func (a *App) DeleteConversation(conversationID string) error {
	return a.impl.DeleteConversation(conversationID)
}

func (a *App) SendMessage(conversationID string, userMessage string) (*model.ChatResponse, error) {
	return a.impl.SendMessage(conversationID, userMessage)
}

func (a *App) SendMessageStream(conversationID string, userMessage string) error {
	return a.impl.SendMessageStream(conversationID, userMessage)
}

func (a *App) EditMessage(messageID string, content string) error {
	return a.impl.EditMessage(messageID, content)
}

func (a *App) DeleteMessage(messageID string) error {
	return a.impl.DeleteMessage(messageID)
}

func (a *App) RecallMessages(messageID string) error {
	return a.impl.RecallMessages(messageID)
}

func (a *App) GetLLMConfig() (*llm.Config, error) {
	return a.impl.GetLLMConfig()
}

func (a *App) GetProviderConfig(provider string) (*llm.Config, error) {
	return a.impl.GetProviderConfig(provider)
}

func (a *App) SaveLLMConfig(cfg llm.Config) error {
	return a.impl.SaveLLMConfig(cfg)
}

func (a *App) TestLLMConnection() error {
	return a.impl.TestLLMConnection()
}

func (a *App) GetLLMModels() ([]model.Model, error) {
	return a.impl.GetLLMModels()
}

func (a *App) RefreshLLMModels() ([]model.Model, error) {
	return a.impl.RefreshLLMModels()
}

func (a *App) GetSupportedProviders() []model.ProviderInfo {
	return a.impl.GetSupportedProviders()
}
