// Package app 实现了 Marcus 的 Wails 后端业务层。
package app

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Marcus/internal/agent"
	"Marcus/internal/config"
	"Marcus/internal/conversation"
	"Marcus/internal/llm"
	"Marcus/internal/memory"
	"Marcus/internal/model"
	"Marcus/internal/skill"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// getModelContext 返回指定模型 ID 的上下文长度；若找不到则返回 0，由调用方决定默认值。
func getModelContext(provider llm.Provider, modelID string) int {
	if provider == nil {
		return 0
	}
	for _, m := range provider.Models() {
		if m.ID == modelID {
			return m.Context
		}
	}
	return 0
}

// initAgent 初始化 Agent、LLM Provider、对话存储、记忆存储和技能存储。
// 该方法在 Startup 中调用，使用与 tools registry 同一个 SQLite 连接。
func (a *App) initAgent() error {
	if a.tools == nil {
		return fmt.Errorf("tools registry not initialized")
	}

	convStore := conversation.NewStore(a.db)
	memStore := memory.NewStore(a.db, memory.DefaultMemoryLimit)
	skStore := skill.NewStore(a.db)

	// LLM Provider 配置
	llmCfg, err := a.loadLLMConfig()
	if err != nil {
		return fmt.Errorf("load llm config: %w", err)
	}

	var provider llm.Provider
	if llmCfg.Provider != "" {
		provider, err = llm.NewProvider(*llmCfg)
		if err != nil {
			a.writeLog("WARN", "llm provider init failed: "+err.Error())
		}
	}

	// 如果未配置或初始化失败，默认使用 OpenAI 占位（无 API Key），等待用户配置。
	if provider == nil {
		provider = llm.NewOpenAI(llm.Config{
			Provider: model.LLMProviderOpenAI,
			Model:    llm.DefaultModelForProvider(model.LLMProviderOpenAI),
		})
	}

	agentCfg := agent.Config{
		LLM:              provider,
		Runner:           a.sandbox,
		ConvStore:        convStore,
		MemoryStore:      memStore,
		SkillStore:       skStore,
		MaxContextTokens: getModelContext(provider, llmCfg.Model),
	}
	ag := agent.NewAgent(agentCfg)
	a.registerTools(ag)
	a.agent = ag
	a.llmConfig = llmCfg
	a.convStore = convStore
	a.memStore = memStore
	return nil
}

// loadLLMConfig 从 SQLite 加载 LLM 配置，若不存在则返回默认值。
func (a *App) loadLLMConfig() (*llm.Config, error) {
	row := a.db.QueryRow(
		`SELECT provider, api_key, model, base_url FROM llm_config WHERE provider = ?`,
		a.cfg.DefaultLLMProvider,
	)

	var cfg llm.Config
	var providerStr string
	var encryptedKey string
	err := row.Scan(&providerStr, &encryptedKey, &cfg.Model, &cfg.BaseURL)
	if err == sql.ErrNoRows {
		provider := model.LLMProvider(a.cfg.DefaultLLMProvider)
		if provider == "" {
			provider = model.LLMProviderOpenAI
		}
		cfg.Provider = provider
		cfg.Model = a.cfg.DefaultLLMModel
		if cfg.Model == "" {
			cfg.Model = llm.DefaultModelForProvider(provider)
		}
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query llm config: %w", err)
	}

	cfg.Provider = model.LLMProvider(providerStr)
	if encryptedKey != "" {
		key, err := config.DecryptAPIKey(encryptedKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
		cfg.APIKey = key
	}
	return &cfg, nil
}

// saveLLMConfig 将 LLM 配置持久化到 SQLite，并对 API Key 加密。
func (a *App) saveLLMConfig(cfg llm.Config) error {
	encryptedKey, err := config.EncryptAPIKey(cfg.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	_, err = a.db.Exec(
		`INSERT INTO llm_config (provider, api_key, model, base_url, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(provider) DO UPDATE SET
			 api_key=excluded.api_key,
			 model=excluded.model,
			 base_url=excluded.base_url,
			 updated_at=CURRENT_TIMESTAMP`,
		string(cfg.Provider), encryptedKey, cfg.Model, cfg.BaseURL,
	)
	if err != nil {
		return fmt.Errorf("upsert llm config: %w", err)
	}

	a.configMu.Lock()
	a.cfg.DefaultLLMProvider = string(cfg.Provider)
	a.cfg.DefaultLLMModel = cfg.Model
	_ = a.cfg.Save(a.cfgPath)
	a.configMu.Unlock()

	a.agentMu.Lock()
	a.llmConfig = &cfg
	a.agentMu.Unlock()
	return nil
}

// refreshAgentProvider 根据当前 llmConfig 重新创建 LLM Provider。
// registerTools 从数据库加载工具，注册到 agent 的注册表。
func (a *App) registerTools(ag *agent.Agent) {
	toolList, err := a.tools.ListTools("")
	if err != nil {
		a.writeLog("WARN", "list tools for agent: "+err.Error())
		return
	}
	for _, t := range toolList {
		manifest, err := t.ManifestAsManifest()
		if err != nil {
			a.writeLog("WARN", fmt.Sprintf("parse manifest for %s: %v", t.ID, err))
			continue
		}
		manifest.ToolID = t.ID
		if err := ag.Registry().RegisterFromManifest(&manifest); err != nil {
			a.writeLog("WARN", fmt.Sprintf("register tool %s to agent: %v", t.ID, err))
		}
	}
}

func (a *App) refreshAgentProvider() error {
	a.agentMu.RLock()
	llmCfg := a.llmConfig
	a.agentMu.RUnlock()

	if llmCfg == nil {
		return fmt.Errorf("llm config not loaded")
	}
	provider, err := llm.NewProvider(*llmCfg)
	if err != nil {
		return err
	}
	skStore := skill.NewStore(a.db)
	ag := agent.NewAgent(agent.Config{
		LLM:              provider,
		Runner:           a.sandbox,
		ConvStore:        a.convStore,
		MemoryStore:      a.memStore,
		SkillStore:       skStore,
		MaxContextTokens: getModelContext(provider, llmCfg.Model),
	})
	a.registerTools(ag)

	a.agentMu.Lock()
	a.agent = ag
	a.agentMu.Unlock()
	return nil
}

// ─── Agent / Chat Wails Bind Methods ─────────────────────────

// CreateConversation 创建新对话。
func (a *App) CreateConversation(title string) (*model.Conversation, error) {
	if a.convStore == nil {
		return nil, fmt.Errorf("conversation store not initialized")
	}
	return a.convStore.CreateConversation(title)
}

// ListConversations 返回最近更新的对话列表。
func (a *App) ListConversations(limit int) ([]model.Conversation, error) {
	if a.convStore == nil {
		return nil, fmt.Errorf("conversation store not initialized")
	}
	return a.convStore.ListConversations(limit)
}

// GetConversationMessages 返回指定对话的所有消息。
func (a *App) GetConversationMessages(conversationID string) ([]model.ConversationMessage, error) {
	if a.convStore == nil {
		return nil, fmt.Errorf("conversation store not initialized")
	}
	return a.convStore.GetMessages(conversationID)
}

// DeleteConversation 删除对话及其消息。
func (a *App) DeleteConversation(conversationID string) error {
	if a.convStore == nil {
		return fmt.Errorf("conversation store not initialized")
	}
	return a.convStore.DeleteConversation(conversationID)
}

// SendMessage 向 Agent 发送用户消息并返回最终助手回复。
func (a *App) SendMessage(conversationID string, userMessage string) (*model.ChatResponse, error) {
	a.agentMu.RLock()
	ag := a.agent
	llmCfg := a.llmConfig
	a.agentMu.RUnlock()

	if ag == nil {
		return nil, fmt.Errorf("agent not initialized")
	}
	if llmCfg == nil || llmCfg.APIKey == "" {
		return nil, fmt.Errorf("llm not configured")
	}
	return ag.Run(context.Background(), conversationID, userMessage)
}

// SendMessageStream 以流式方式向 Agent 发送用户消息。
// 该方法先完成完整的 Agent TAO 循环（包括工具调用），然后将最终助手回复
// 通过 Wails 事件以打字机效果逐步推送到前端。
func (a *App) SendMessageStream(conversationID string, userMessage string) error {
	a.agentMu.RLock()
	ag := a.agent
	llmCfg := a.llmConfig
	a.agentMu.RUnlock()

	if ag == nil {
		return fmt.Errorf("agent not initialized")
	}
	if llmCfg == nil || llmCfg.APIKey == "" {
		return fmt.Errorf("llm not configured")
	}
	if a.ctx == nil {
		return fmt.Errorf("app context not initialized")
	}

	go func() {
		wailsRuntime.EventsEmit(a.ctx, "chat:stream:start", map[string]string{
			"conversation_id": conversationID,
		})

		resp, err := ag.Run(context.Background(), conversationID, userMessage)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "chat:stream:error", map[string]string{
				"conversation_id": conversationID,
				"error":           err.Error(),
			})
			return
		}

		// 将最终内容以打字机效果分片推送。
		content := resp.Content
		chunkSize := 4
		for i := 0; i < len(content); i += chunkSize {
			end := i + chunkSize
			if end > len(content) {
				end = len(content)
			}
			wailsRuntime.EventsEmit(a.ctx, "chat:stream:chunk", map[string]string{
				"conversation_id": conversationID,
				"content":         content[i:end],
			})
			time.Sleep(12 * time.Millisecond)
		}

		wailsRuntime.EventsEmit(a.ctx, "chat:stream:end", map[string]string{
			"conversation_id": conversationID,
		})
	}()

	return nil
}

// GetLLMConfig 返回当前 LLM 配置（API Key 已解密）。
func (a *App) GetLLMConfig() (*llm.Config, error) {
	a.agentMu.RLock()
	llmCfg := a.llmConfig
	a.agentMu.RUnlock()

	if llmCfg == nil {
		return nil, fmt.Errorf("llm config not initialized")
	}
	// 返回副本，避免外部修改内部状态。
	cfg := *llmCfg
	return &cfg, nil
}

// SaveLLMConfig 保存 LLM 配置并重新初始化 Provider。
func (a *App) SaveLLMConfig(cfg llm.Config) error {
	if a.tools == nil {
		return fmt.Errorf("tools registry not initialized")
	}
	if err := a.saveLLMConfig(cfg); err != nil {
		return err
	}
	return a.refreshAgentProvider()
}

// TestLLMConnection 测试当前 LLM 配置是否能连通。
func (a *App) TestLLMConnection() error {
	a.agentMu.RLock()
	llmCfg := a.llmConfig
	a.agentMu.RUnlock()

	if llmCfg == nil {
		return fmt.Errorf("llm config not initialized")
	}
	provider, err := llm.NewProvider(*llmCfg)
	if err != nil {
		return err
	}
	return provider.TestConnection(context.Background())
}

// GetSupportedProviders 返回注册表中所有支持的 LLM 供应商列表。
func (a *App) GetSupportedProviders() []model.ProviderInfo {
	return llm.DefaultRegistry().List()
}

// GetLLMModels 返回当前 Provider 支持的模型列表。
func (a *App) GetLLMModels() ([]model.Model, error) {
	a.agentMu.RLock()
	ag := a.agent
	a.agentMu.RUnlock()

	if ag == nil || ag.LLMProvider() == nil {
		return nil, fmt.Errorf("agent or llm provider not initialized")
	}
	return ag.LLMProvider().Models(), nil
}
