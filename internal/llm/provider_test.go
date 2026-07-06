package llm

import (
	"testing"

	"Marcus/internal/model"
)

// TestNewProvider 验证根据配置创建对应 Provider 实例。
func TestNewProvider(t *testing.T) {
	tests := []struct {
		provider model.LLMProvider
		wantName string
	}{
		{model.LLMProviderOpenAI, "openai"},
		{model.LLMProviderAnthropic, "anthropic"},
		{model.LLMProviderOllama, "ollama"},
	}

	for _, tt := range tests {
		p, err := NewProvider(Config{Provider: tt.provider})
		if err != nil {
			t.Fatalf("NewProvider(%s) error: %v", tt.provider, err)
		}
		if p.Name() != tt.wantName {
			t.Fatalf("Name() = %q, want %q", p.Name(), tt.wantName)
		}
	}
}

// TestNewProviderUnsupported 验证不支持的提供者返回错误。
func TestNewProviderUnsupported(t *testing.T) {
	_, err := NewProvider(Config{Provider: "unknown"})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

// TestDefaultModelForProvider 验证默认模型映射。
func TestDefaultModelForProvider(t *testing.T) {
	if got := DefaultModelForProvider(model.LLMProviderOpenAI); got != "gpt-4o" {
		t.Fatalf("OpenAI default model = %q, want gpt-4o", got)
	}
	if got := DefaultModelForProvider(model.LLMProviderAnthropic); got != "claude-3-5-sonnet-latest" {
		t.Fatalf("Anthropic default model = %q, want claude-3-5-sonnet-latest", got)
	}
	if got := DefaultModelForProvider(model.LLMProviderOllama); got != "llama3.1" {
		t.Fatalf("Ollama default model = %q, want llama3.1", got)
	}
}

// TestContextLengthForModel 验证已知模型的上下文长度。
func TestContextLengthForModel(t *testing.T) {
	if got := ContextLengthForModel(model.LLMProviderOpenAI, "gpt-4o"); got != 128000 {
		t.Fatalf("gpt-4o context = %d, want 128000", got)
	}
	if got := ContextLengthForModel(model.LLMProviderAnthropic, "claude-3-5-sonnet-latest"); got != 200000 {
		t.Fatalf("claude sonnet context = %d, want 200000", got)
	}
	if got := ContextLengthForModel(model.LLMProviderOllama, "unknown"); got != 0 {
		t.Fatalf("unknown ollama model context should be 0, got %d", got)
	}
}
