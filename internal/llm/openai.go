// Package llm 实现了 OpenAI 兼容的 LLM Provider。
//
// 该文件仅提供 NewOpenAI 向后兼容别名。实际实现在 openai_compat.go。
package llm

// NewOpenAI 是 NewOpenAICompatible 的别名，保持向后兼容。
func NewOpenAI(cfg Config) *OpenAICompatible {
	return NewOpenAICompatible(cfg)
}
