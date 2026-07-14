package llm

import "Marcus/internal/model"

// AdapterType 标识 Provider 的实现适配器类型。
type AdapterType string

const (
	AdapterOpenAICompat AdapterType = "openai-compat"
	AdapterAnthropic    AdapterType = "anthropic"
	AdapterGemini       AdapterType = "gemini"
)

// ProviderEntry 描述一个 LLM 供应商的注册信息。
type ProviderEntry struct {
	Adapter     AdapterType
	Name        string
	DefaultModel string
	DefaultBaseURL string
	NeedAPIKey  bool
	Models      []model.Model
}

// Registry 是 LLM 供应商的注册表，管理供应商到适配器的映射。
type Registry struct {
	entries map[model.LLMProvider]*ProviderEntry
}

var defaultRegistry *Registry

func init() {
	defaultRegistry = NewRegistry()
	defaultRegistry.Register(model.LLMProviderOpenAI, &ProviderEntry{
		Adapter:      AdapterOpenAICompat,
		Name:         "OpenAI",
		DefaultModel: "gpt-4o",
		NeedAPIKey:   true,
		Models: []model.Model{
			{ID: "gpt-4o", Name: "GPT-4o", Context: 128000},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Context: 128000},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Context: 128000},
			{ID: "gpt-4", Name: "GPT-4", Context: 8192},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Context: 16385},
		},
	})
	defaultRegistry.Register(model.LLMProviderAnthropic, &ProviderEntry{
		Adapter:      AdapterAnthropic,
		Name:         "Anthropic",
		DefaultModel: "claude-3-5-sonnet-latest",
		NeedAPIKey:   true,
		Models: []model.Model{
			{ID: "claude-3-5-sonnet-latest", Name: "Claude 3.5 Sonnet", Context: 200000},
			{ID: "claude-3-opus-latest", Name: "Claude 3 Opus", Context: 200000},
			{ID: "claude-3-haiku-latest", Name: "Claude 3 Haiku", Context: 200000},
		},
	})
	defaultRegistry.Register(model.LLMProviderDeepSeek, &ProviderEntry{
		Adapter:        AdapterOpenAICompat,
		Name:           "DeepSeek",
		DefaultModel:   "deepseek-v4-flash",
		DefaultBaseURL: "https://api.deepseek.com",
		NeedAPIKey:     true,
		Models: []model.Model{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Context: 1000000},
			{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", Context: 1000000},
			{ID: "deepseek-chat", Name: "DeepSeek Chat (legacy)", Context: 128000},
			{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner (legacy)", Context: 128000},
		},
	})
	defaultRegistry.Register(model.LLMProviderGroq, &ProviderEntry{
		Adapter:      AdapterOpenAICompat,
		Name:         "Groq",
		DefaultModel: "llama-3.3-70b-versatile",
		DefaultBaseURL: "https://api.groq.com/openai/v1",
		NeedAPIKey:   true,
		Models: []model.Model{
			{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Context: 131072},
			{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B", Context: 131072},
			{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Context: 32768},
			{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Context: 8192},
		},
	})
	defaultRegistry.Register(model.LLMProviderOpenRouter, &ProviderEntry{
		Adapter:      AdapterOpenAICompat,
		Name:         "OpenRouter",
		DefaultModel: "anthropic/claude-3.5-sonnet",
		DefaultBaseURL: "https://openrouter.ai/api/v1",
		NeedAPIKey:   true,
	})
	defaultRegistry.Register(model.LLMProviderTogether, &ProviderEntry{
		Adapter:      AdapterOpenAICompat,
		Name:         "Together AI",
		DefaultModel: "mistralai/Mixtral-8x22B-Instruct-v0.1",
		DefaultBaseURL: "https://api.together.xyz/v1",
		NeedAPIKey:   true,
	})
	defaultRegistry.Register(model.LLMProviderOllama, &ProviderEntry{
		Adapter:      AdapterOpenAICompat,
		Name:         "Ollama",
		DefaultModel: "llama3.1",
		DefaultBaseURL: "http://localhost:11434/v1",
		NeedAPIKey:   false,
	})
}

// NewRegistry 创建一个空的供应商注册表。
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[model.LLMProvider]*ProviderEntry),
	}
}

// Register 注册一个供应商到注册表。
func (r *Registry) Register(provider model.LLMProvider, entry *ProviderEntry) {
	r.entries[provider] = entry
}

// Lookup 查询供应商的注册信息。
func (r *Registry) Lookup(provider model.LLMProvider) *ProviderEntry {
	return r.entries[provider]
}

// List 返回所有已注册的供应商列表。
func (r *Registry) List() []model.ProviderInfo {
	result := make([]model.ProviderInfo, 0, len(r.entries))
	for id, entry := range r.entries {
		info := model.ProviderInfo{
			Provider:    id,
			Name:        entry.Name,
			Adapter:     string(entry.Adapter),
			DefaultModel: entry.DefaultModel,
			DefaultBaseURL: entry.DefaultBaseURL,
			NeedAPIKey:  entry.NeedAPIKey,
		}
		result = append(result, info)
	}
	return result
}

// DefaultRegistry 返回全局默认注册表。
func DefaultRegistry() *Registry {
	return defaultRegistry
}
