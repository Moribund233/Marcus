// Package llm 定义并实现了 Marcus LLM Agent 所需的 LLM 提供者抽象层。
//
// 该包屏蔽不同 LLM 供应商（OpenAI、Anthropic、Ollama 等）的 API 差异，
// 向上层 Agent Core 提供统一的消息、工具调用和流式响应接口。
package llm

import (
	"context"
	"fmt"

	"Marcus/internal/model"
)

// Provider 是 LLM 提供者的统一接口。
type Provider interface {
	// Name 返回提供者唯一标识，如 "openai"、"anthropic"、"ollama"。
	Name() string

	// Models 返回该提供者支持的模型列表。
	Models() []model.Model

	// Chat 发送非流式聊天请求并返回完整响应。
	Chat(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error)

	// ChatStream 发送流式聊天请求，返回按顺序产生的响应片段通道。
	// 调用方应持续读取该通道直到收到 Done==true 或 Error!=nil 的片段。
	ChatStream(ctx context.Context, req *model.ChatRequest) (<-chan *model.ChatChunk, error)

	// TestConnection 测试当前配置是否能成功访问提供者 API。
	TestConnection(ctx context.Context) error
}

// Config 表示单个 LLM 提供者的运行时配置。
type Config struct {
	Provider model.LLMProvider `json:"provider"`
	APIKey   string            `json:"api_key"`
	Model    string            `json:"model"`
	BaseURL  string            `json:"base_url"`
}

// NewProvider 根据配置创建对应的 LLM Provider 实例。
// 若 provider 不被支持，返回错误。
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case model.LLMProviderOpenAI:
		return NewOpenAI(cfg), nil
	case model.LLMProviderAnthropic:
		return NewAnthropic(cfg), nil
	case model.LLMProviderOllama:
		return NewOllama(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", cfg.Provider)
	}
}

// SupportedProviders 返回 Marcus 当前支持的所有 LLM 提供者标识。
func SupportedProviders() []model.LLMProvider {
	return []model.LLMProvider{
		model.LLMProviderOpenAI,
		model.LLMProviderAnthropic,
		model.LLMProviderOllama,
	}
}

// DefaultModelForProvider 返回指定提供者的默认推荐模型。
func DefaultModelForProvider(provider model.LLMProvider) string {
	switch provider {
	case model.LLMProviderOpenAI:
		return "gpt-4o"
	case model.LLMProviderAnthropic:
		return "claude-3-5-sonnet-latest"
	case model.LLMProviderOllama:
		return "llama3.1"
	default:
		return ""
	}
}

// ContextLengthForModel 返回常见模型的最大上下文长度。
// 对于未知模型返回 0，调用方应使用配置项或保守默认值。
func ContextLengthForModel(provider model.LLMProvider, modelID string) int {
	known := map[string]int{
		// OpenAI
		"gpt-4o":          128000,
		"gpt-4o-mini":     128000,
		"gpt-4-turbo":     128000,
		"gpt-4":           8192,
		"gpt-3.5-turbo":   16385,
		// Anthropic
		"claude-3-5-sonnet-latest": 200000,
		"claude-3-5-sonnet-20240620": 200000,
		"claude-3-opus-latest":     200000,
		"claude-3-haiku-latest":    200000,
		// Ollama common defaults
		"llama3.1": 128000,
		"qwen2.5":  128000,
		"mistral":  32768,
	}

	if v, ok := known[modelID]; ok {
		return v
	}
	// Ollama 本地模型通常支持较大上下文，但信息不可知时返回 0。
	if provider == model.LLMProviderOllama {
		return 0
	}
	return 0
}
