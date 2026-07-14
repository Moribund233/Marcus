// Package agent 实现了 Marcus LLM Agent 的核心逻辑。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"Marcus/internal/conversation"
	"Marcus/internal/llm"
	"Marcus/internal/memory"
	"Marcus/internal/model"
	"Marcus/internal/skill"
)

// PhaseType 表示 Agent 执行阶段类型。
type PhaseType string

const (
	PhaseThinking  PhaseType = "thinking"
	PhaseToolCall  PhaseType = "tool_call"
	PhaseToolDone  PhaseType = "tool_done"
	PhaseCode      PhaseType = "code"
	PhaseFetch     PhaseType = "fetch"
	PhaseText      PhaseType = "text"
)

// PhaseCallback 在 Agent 执行各阶段被调用，用于通知前端展示实时状态。
// conversationID 为当前对话 ID，便于回调方发射 Wails 事件。
type PhaseCallback func(conversationID string, phase PhaseType, content string, metadata map[string]string)

// Agent 是 Marcus LLM Agent 的核心，负责运行 TAO（Thought-Action-Observation）循环。
type Agent struct {
	llm           llm.Provider
	registry      *Registry
	promptMgr     *PromptManager
	executor      *Executor
	convStore     *conversation.Store
	memoryStore   *memory.Store
	skillStore    *skill.Store
	maxIterations int
	compressor    *ContextCompressor
	lang          string
	onPhase       PhaseCallback
}

// Config 用于创建 Agent 实例。
type Config struct {
	LLM              llm.Provider
	Runner           SyncRunner
	ConvStore        *conversation.Store
	MemoryStore      *memory.Store
	SkillStore       *skill.Store
	MaxIterations    int
	MaxContextTokens int
	Language         string
	OnPhase          PhaseCallback
}

// NewAgent 创建一个新的 Agent 实例。
func NewAgent(cfg Config) *Agent {
	registry := NewRegistry()
	registry.RegisterMemoryTool()

	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}

	maxTokens := cfg.MaxContextTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	return &Agent{
		llm:           cfg.LLM,
		registry:      registry,
		promptMgr:     NewPromptManager(registry),
		executor:      NewExecutor(registry, cfg.Runner, cfg.MemoryStore),
		convStore:     cfg.ConvStore,
		memoryStore:   cfg.MemoryStore,
		skillStore:    cfg.SkillStore,
		maxIterations: cfg.MaxIterations,
		compressor:    NewContextCompressor(maxTokens, 0.85),
		lang:          cfg.Language,
		onPhase:       cfg.OnPhase,
	}
}

// Registry 返回 Agent 的工具注册表，供外部注入 Marcus 工具。
func (a *Agent) Registry() *Registry {
	return a.registry
}

// LLMProvider 返回 Agent 使用的 LLM Provider。
func (a *Agent) LLMProvider() llm.Provider {
	return a.llm
}

// Run 执行 Agent 主循环，处理用户消息并返回最终助手回复。
//
// 流程：
//  1. 加载长期记忆快照并构建系统提示词。
//  2. 将用户消息保存到对话历史。
//  3. 进入 TAO 循环：调用 LLM → 解析响应 → 执行工具调用 → 保存结果 → 继续。
//  4. 当 LLM 不再请求工具或达到最大迭代次数时，保存最终回复并返回。
func (a *Agent) Run(ctx context.Context, conversationID string, userMessage string) (*model.ChatResponse, error) {
	if err := a.ensureConversation(conversationID); err != nil {
		return nil, fmt.Errorf("ensure conversation: %w", err)
	}

	if _, err := a.convStore.AddMessage(conversationID, model.RoleUser, userMessage); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	systemPrompt, err := a.buildSystemPrompt(userMessage)
	if err != nil {
		return nil, fmt.Errorf("build system prompt: %w", err)
	}

	var finalResponse *model.ChatResponse
	for i := 0; i < a.maxIterations; i++ {
		history, err := a.buildMessageHistory(conversationID, systemPrompt)
		if err != nil {
			return nil, fmt.Errorf("build message history: %w", err)
		}

		a.emitPhase(conversationID, PhaseThinking, "", nil)

		resp, err := a.llm.Chat(ctx, &model.ChatRequest{
			Model:    "", // Provider 使用配置中的默认模型
			Messages: history,
			Tools:    a.registry.GetUserToolDefinitions(),
		})
		if err != nil {
			return nil, fmt.Errorf("llm chat: %w", err)
		}

		if _, err := a.convStore.AddAssistantMessage(conversationID, resp.Content, resp.ToolCalls); err != nil {
			return nil, fmt.Errorf("save assistant message: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			finalResponse = resp
			break
		}

		results := make([]model.ToolCallResult, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			a.emitPhase(conversationID, PhaseToolCall, tc.Function.Arguments, map[string]string{
				"tool_name": tc.Function.Name,
			})

			result := a.executor.Execute(ctx, tc)

			a.emitPhase(conversationID, PhaseToolDone, result.Content, map[string]string{
				"tool_name": tc.Function.Name,
			})

			results = append(results, result)
		}

		if _, err := a.convStore.AddToolResults(conversationID, results); err != nil {
			return nil, fmt.Errorf("save tool results: %w", err)
		}
	}

	if finalResponse == nil {
		return nil, fmt.Errorf("agent reached maximum iterations (%d) without final response", a.maxIterations)
	}

	return finalResponse, nil
}

// ensureConversation 确保指定 ID 的对话存在。
func (a *Agent) ensureConversation(id string) error {
	if id == "" {
		return fmt.Errorf("conversation id is empty")
	}
	conv, err := a.convStore.GetConversation(id)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found: %s", id)
	}
	return nil
}

// langDirective 返回 LLM 输出语言的指令。
func (a *Agent) langDirective() string {
	switch a.lang {
	case "zh-CN":
		return "\nIMPORTANT: Always reply in Chinese (简体中文).\n"
	case "en-US":
		return "\nIMPORTANT: Always reply in English.\n"
	default: // "auto" or empty
		return ""
	}
}

// buildSystemPrompt 构建包含记忆快照和技能匹配的系统提示词。
func (a *Agent) buildSystemPrompt(userMessage string) (string, error) {
	var memoryPrompt string
	if a.memoryStore != nil {
		snapshot, err := a.memoryStore.BuildSnapshot()
		if err != nil {
			return "", fmt.Errorf("build memory snapshot: %w", err)
		}
		memoryPrompt = a.memoryStore.RenderPrompt(snapshot)
	}

	var skillsPrompt string
	if a.skillStore != nil && userMessage != "" {
		keywords := skill.ExtractKeywords(userMessage)
		if len(keywords) > 0 {
			matched, err := a.skillStore.Match(keywords)
			if err != nil {
				return "", fmt.Errorf("match skills: %w", err)
			}
			skillsPrompt = skill.RenderSkillsPrompt(matched)
		}
	}

	prompt := a.promptMgr.BuildSystemPrompt(memoryPrompt, skillsPrompt)

	prompt += a.langDirective()
	return prompt, nil
}

// ConsolidateMemory 对已完成对话进行归纳：调用 LLM 提取关键事实并写入 L2 长期记忆。
// 仅在对话有足够内容且 memoryStore 可用时执行；失败不影响主流程。
func (a *Agent) ConsolidateMemory(ctx context.Context, conversationID string) {
	if a.memoryStore == nil || a.llm == nil {
		return
	}

	msgs, err := a.convStore.GetMessages(conversationID)
	if err != nil || len(msgs) < 4 {
		return
	}

	var b strings.Builder
	for _, msg := range msgs {
		switch msg.Role {
		case model.RoleUser:
			if msg.Content != "" {
				b.WriteString("User: ")
				b.WriteString(msg.Content)
				b.WriteString("\n")
			}
		case model.RoleAssistant:
			if msg.Content != "" {
				b.WriteString("Assistant: ")
				b.WriteString(msg.Content)
				b.WriteString("\n")
			}
		}
	}

	conversationText := b.String()
	if len(conversationText) < 20 {
		return
	}
	if len(conversationText) > 6000 {
		conversationText = conversationText[len(conversationText)-6000:]
	}

	prompt := fmt.Sprintf(`Extract key facts from the conversation below. Only extract stable, factual information about the user (preferences, environment, project details). Skip greetings, thanks, and transient chit-chat.

Return a JSON array of objects, each with "key" (short kebab-case identifier) and "content" (brief factual statement). Example:
[{"key":"preferred-language","content":"User prefers Chinese responses"}]

If no facts to extract, return an empty array [].

Conversation:
%s

JSON facts:`, conversationText)

	resp, err := a.llm.Chat(ctx, &model.ChatRequest{
		Messages: []model.Message{
			{Role: model.RoleSystem, Content: "You are a precise fact extraction assistant. Return only valid JSON."},
			{Role: model.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return
	}

	raw := resp.Content
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "["); idx >= 0 {
		raw = raw[idx:]
	}
	if idx := strings.LastIndex(raw, "]"); idx >= 0 {
		raw = raw[:idx+1]
	}

	var facts []struct {
		Key     string `json:"key"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &facts); err != nil || len(facts) == 0 {
		return
	}

	for _, f := range facts {
		f.Key = strings.TrimSpace(f.Key)
		f.Content = strings.TrimSpace(f.Content)
		if f.Key == "" || f.Content == "" {
			continue
		}
		_, _ = a.memoryStore.Add(model.MemoryEntry{
			Scope:   model.MemoryScopeUser,
			Key:     f.Key,
			Content: f.Content,
			Source:  "consolidation",
		})
	}
}

// buildMessageHistory 从对话存储加载历史并组装为 LLM 消息列表。
// 当历史消息估算 token 数超过阈值时，会自动压缩历史以控制上下文长度。
func (a *Agent) buildMessageHistory(conversationID, systemPrompt string) ([]model.Message, error) {
	history := []model.Message{
		{Role: model.RoleSystem, Content: systemPrompt},
	}

	msgs, err := a.convStore.GetMessages(conversationID)
	if err != nil {
		return nil, err
	}

	for _, msg := range msgs {
		switch msg.Role {
		case model.RoleUser:
			history = append(history, model.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		case model.RoleAssistant:
			history = append(history, model.Message{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: msg.ToolCalls,
			})
		case model.RoleTool:
			for _, r := range msg.ToolResults {
				history = append(history, model.Message{
					Role:       model.RoleTool,
					Name:       r.Name,
					Content:    r.Content,
					ToolCallID: r.ToolCallID,
				})
			}
		}
	}

	if a.compressor != nil && a.compressor.ShouldCompress(history) {
		history = a.compressor.Compress(history)
	}

	return history, nil
}

func (a *Agent) emitPhase(conversationID string, phase PhaseType, content string, metadata map[string]string) {
	if a.onPhase != nil {
		a.onPhase(conversationID, phase, content, metadata)
	}
}
