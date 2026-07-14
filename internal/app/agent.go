// Package app 实现了 Marcus 的 Wails 后端业务层。
package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

	if provider == nil {
		provider = llm.NewOpenAI(llm.Config{
			Provider: model.LLMProviderOpenAI,
			Model:    llm.DefaultModelForProvider(model.LLMProviderOpenAI),
		})
	}

	contextTokens := getModelContext(provider, llmCfg.Model)
	memStore := memory.NewStore(a.db, memory.SuggestedLimit(contextTokens))

	// 种子基础记忆：LLM 角色设定
	if _, err := memStore.Add(model.MemoryEntry{
		Scope:   model.MemoryScopeGlobal,
		Key:     "self-introduction",
		Content: "I am Marcus, a field agent from the St. Pavlov Foundation. When I read, even an eternity seems like an instant.",
		Source:  "system",
	}); err != nil {
		a.writeLog("WARN", "seed base memory: "+err.Error())
	}

	skStore := skill.NewStore(a.db)

	agentCfg := agent.Config{
		LLM:              provider,
		Runner:           a.sandbox,
		ConvStore:        convStore,
		MemoryStore:      memStore,
		SkillStore:       skStore,
		MaxContextTokens: contextTokens,
		Language:         a.cfg.LLMLanguage,
		OnPhase:          a.onAgentPhase,
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
		cfg.APIKey = strings.TrimSpace(key)
	}
	return &cfg, nil
}

// saveLLMConfig 将 LLM 配置持久化到 SQLite，并对 API Key 加密。
func (a *App) saveLLMConfig(cfg llm.Config) error {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
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
// 每次调用会先清除旧的用户工具定义，确保与数据库状态一致。
func (a *App) registerTools(ag *agent.Agent) {
	ag.Registry().ClearUserTools()

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
		if manifest.ID == "" {
			manifest.ID = sanitizeToolFnName(t.ID)
		} else if !isValidFnName(manifest.ID) {
			manifest.ID = sanitizeToolFnName(manifest.ID)
		}
		if err := ag.Registry().RegisterFromManifest(&manifest); err != nil {
			a.writeLog("WARN", fmt.Sprintf("register tool %s to agent: %v", t.ID, err))
		}
	}
}

// syncAgentTools re-registers all DB tools with the running agent.
// Used after any tool mutation (add, delete, install, uninstall, scan).
func (a *App) syncAgentTools() {
	a.agentMu.RLock()
	ag := a.agent
	a.agentMu.RUnlock()
	if ag != nil {
		a.registerTools(ag)
	}
}

// sanitizeToolFnName 将工具 ID 转化为符合 LLM API 要求的函数名。
// LLM 要求 function name 须匹配 ^[a-zA-Z0-9_-]+$。
func sanitizeToolFnName(id string) string {
	r := strings.NewReplacer(":", "_", ".", "_", "/", "_", " ", "_", "..", ".")
	return r.Replace(id)
}

// isValidFnName 检查名称是否满足 LLM function name 的字符要求。
func isValidFnName(name string) bool {
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
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
	contextTokens := getModelContext(provider, llmCfg.Model)
	memStore := memory.NewStore(a.db, memory.SuggestedLimit(contextTokens))
	ag := agent.NewAgent(agent.Config{
		LLM:              provider,
		Runner:           a.sandbox,
		ConvStore:        a.convStore,
		MemoryStore:      memStore,
		SkillStore:       skStore,
		MaxContextTokens: contextTokens,
		Language:         a.cfg.LLMLanguage,
		OnPhase:          a.onAgentPhase,
	})
	a.registerTools(ag)

	a.agentMu.Lock()
	a.agent = ag
	a.memStore = memStore
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
	resp, err := ag.Run(context.Background(), conversationID, userMessage)
	if err == nil {
		ag.ConsolidateMemory(context.Background(), conversationID)
		a.autoRenameConversation(conversationID)
	}
	return resp, err
}

// EditMessage 编辑指定消息的内容。
func (a *App) EditMessage(messageID string, content string) error {
	if a.convStore == nil {
		return fmt.Errorf("conversation store not initialized")
	}
	return a.convStore.UpdateMessage(messageID, content)
}

// DeleteMessage 删除单条消息。
func (a *App) DeleteMessage(messageID string) error {
	if a.convStore == nil {
		return fmt.Errorf("conversation store not initialized")
	}
	return a.convStore.DeleteMessage(messageID)
}

// RecallMessages 撤回指定消息及其之后的所有消息（从 LLM 上下文中移除）。
func (a *App) RecallMessages(messageID string) error {
	if a.convStore == nil {
		return fmt.Errorf("conversation store not initialized")
	}
	return a.convStore.RecallMessages(messageID)
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

		ag.ConsolidateMemory(context.Background(), conversationID)
		a.autoRenameConversation(conversationID)

		// 将最终内容以打字机效果分片推送（使用 rune 分割避免 UTF-8 字符被截断）。
		content := resp.Content
		runes := []rune(content)
		chunkSize := 4
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			wailsRuntime.EventsEmit(a.ctx, "chat:stream:chunk", map[string]string{
				"conversation_id": conversationID,
				"content":         string(runes[i:end]),
			})
			time.Sleep(12 * time.Millisecond)
		}

		wailsRuntime.EventsEmit(a.ctx, "chat:stream:end", map[string]string{
			"conversation_id": conversationID,
		})
	}()

	return nil
}

// onAgentPhase 是 agent.PhaseCallback 的实现，将 Agent 阶段事件转发为 Wails 事件。
func (a *App) onAgentPhase(conversationID string, phase agent.PhaseType, content string, metadata map[string]string) {
	if a.ctx == nil {
		return
	}
	event := map[string]string{
		"conversation_id": conversationID,
		"type":            string(phase),
		"content":         content,
	}
	for k, v := range metadata {
		event[k] = v
	}
	wailsRuntime.EventsEmit(a.ctx, "chat:stream:phase", event)
}

// EmitPhase 向前端发送 LLM 执行阶段事件。
func (a *App) EmitPhase(conversationID string, phaseType string, content string, metadata map[string]string) {
	if a.ctx == nil {
		return
	}
	event := map[string]string{
		"conversation_id": conversationID,
		"type":            phaseType,
		"content":         content,
	}
	for k, v := range metadata {
		event[k] = v
	}
	wailsRuntime.EventsEmit(a.ctx, "chat:stream:phase", event)
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

// GetProviderConfig 返回指定供应商的存储配置（API Key 已解密）。
func (a *App) GetProviderConfig(provider string) (*llm.Config, error) {
	row := a.db.QueryRow(
		`SELECT provider, api_key, model, base_url FROM llm_config WHERE provider = ?`,
		provider,
	)

	var cfg llm.Config
	var providerStr string
	var encryptedKey string
	err := row.Scan(&providerStr, &encryptedKey, &cfg.Model, &cfg.BaseURL)
	if err == sql.ErrNoRows {
		p := model.LLMProvider(provider)
		entry := llm.DefaultRegistry().Lookup(p)
		if entry != nil {
			return &llm.Config{
				Provider: p,
				Model:    entry.DefaultModel,
				BaseURL:  entry.DefaultBaseURL,
			}, nil
		}
		return &llm.Config{Provider: p}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query provider config: %w", err)
	}

	cfg.Provider = model.LLMProvider(providerStr)
	if encryptedKey != "" {
		key, err := config.DecryptAPIKey(encryptedKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
		cfg.APIKey = strings.TrimSpace(key)
	}
	return &cfg, nil
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

// RefreshLLMModels 清除当前 Provider 的模型缓存并从 API 重新拉取。
func (a *App) RefreshLLMModels() ([]model.Model, error) {
	a.agentMu.RLock()
	cfg := a.llmConfig
	a.agentMu.RUnlock()

	if cfg == nil {
		return nil, fmt.Errorf("llm config not initialized")
	}

	if store := llm.GetModelStore(); store != nil {
		if err := store.ClearProvider(cfg.Provider); err != nil {
			a.writeLog("WARN", "clear model cache: "+err.Error())
		}
	}

	provider, err := llm.NewProvider(*cfg)
	if err != nil {
		return nil, err
	}
	return provider.Models(), nil
}

// GetSupportedProviders 返回注册表中所有支持的 LLM 供应商列表。
func (a *App) GetSupportedProviders() []model.ProviderInfo {
	return llm.DefaultRegistry().List()
}

// autoRenameConversation 在首轮对话后自动生成会话标题。
// 仅在消息数 ≤5 时执行（即尚处于首轮对话阶段），避免重复重命名。
// 优先使用 LLM 基于首轮对话内容生成简洁标题；若 LLM 不可用则回退到时间戳+短哈希。
func (a *App) autoRenameConversation(conversationID string) {
	if a.convStore == nil {
		return
	}

	msgs, err := a.convStore.GetMessages(conversationID)
	if err != nil || len(msgs) < 2 || len(msgs) > 5 {
		return
	}

	// 提取首条用户消息和最后一条助手回复作为标题生成上下文
	var firstUserMsg, lastAssistantContent string
	for _, m := range msgs {
		if m.Role == model.RoleUser && m.Content != "" && firstUserMsg == "" {
			firstUserMsg = m.Content
		}
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == model.RoleAssistant && msgs[i].Content != "" {
			lastAssistantContent = msgs[i].Content
			break
		}
	}

	if firstUserMsg == "" {
		return
	}

	title := a.generateTitleWithLLM(firstUserMsg, lastAssistantContent)
	if title == "" {
		title = a.fallbackTitle(conversationID)
	}
	_ = a.convStore.UpdateConversationTitle(conversationID, title)
}

// generateTitleWithLLM 调用 LLM 根据首轮对话生成简短标题。
func (a *App) generateTitleWithLLM(userContent, assistantContent string) string {
	a.agentMu.RLock()
	provider := a.agent.LLMProvider()
	a.agentMu.RUnlock()

	if provider == nil {
		return ""
	}

	prompt := fmt.Sprintf(`Generate a concise title (5 words or less, in the same language as the conversation) based on this conversation:

User: %s
Assistant: %s

Title:`, userContent, assistantContent)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := provider.Chat(ctx, &model.ChatRequest{
		Messages: []model.Message{
			{Role: model.RoleUser, Content: prompt},
		},
		MaxTokens: 20,
	})
	if err != nil {
		return ""
	}

	title := strings.TrimSpace(resp.Content)
	title = strings.Trim(title, "\"'「」")
	if title == "" || len([]rune(title)) > 40 {
		return ""
	}
	return title
}

// fallbackTitle 当 LLM 不可用时使用时间戳+短哈希生成标题。
func (a *App) fallbackTitle(conversationID string) string {
	now := time.Now().Format("2006-01-02")
	hash := conversationID
	if len(hash) > 8 {
		hash = hash[len(hash)-8:]
	}
	return fmt.Sprintf("%s %s", now, hash)
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
