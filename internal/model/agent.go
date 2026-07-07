package model

// LLMProvider 表示支持的 LLM 提供者类型。
type LLMProvider string

const (
	LLMProviderOpenAI    LLMProvider = "openai"
	LLMProviderAnthropic LLMProvider = "anthropic"
	LLMProviderOllama    LLMProvider = "ollama"
)

// MessageRole 表示对话消息的角色。
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Message 表示发送给 LLM 的单条消息。
type Message struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
	// Name 用于 tool 角色的消息，标识该消息来自哪个工具调用。
	Name string `json:"name,omitempty"`
	// ToolCallID 用于 tool 角色的消息，与 assistant 消息中的 tool_calls 一一对应（OpenAI 必需）。
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall 表示 LLM 请求调用某个工具。
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
	// Extra 保留provider特定的原始字段。
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// ToolCallFunction 表示工具调用的函数部分。
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCallResult 表示工具调用的执行结果。
type ToolCallResult struct {
	ToolCallID string `json:"tool_call_id"`
	Role       string `json:"role"`
	Name       string `json:"name"`
	Content    string `json:"content"`
}

// ToolDefinition 表示暴露给 LLM 的工具定义（JSON Schema 格式）。
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
	// Extra 保留provider特定的扩展字段。
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// ToolFunctionDefinition 表示工具函数定义。
type ToolFunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Usage 表示一次 LLM 调用的 token 使用量。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Model 表示某个 LLM 提供者支持的模型信息。
type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Context int    `json:"context"` // 最大上下文长度（token 数）
}

// ChatRequest 表示发送给 LLM 提供者的聊天请求。
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

// ChatResponse 表示 LLM 提供者的非流式响应。
type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     Usage      `json:"usage"`
	// Raw 保留provider原始的响应数据，便于调试。
	Raw map[string]interface{} `json:"raw,omitempty"`
}

// ChatChunk 表示流式响应中的单个片段。
type ChatChunk struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Done      bool       `json:"done"`
	Usage     *Usage     `json:"usage,omitempty"`
	Error     error      `json:"-"`
}

// LLMConfig 表示单个 LLM 提供者的配置，对应数据库 llm_config 表。
type LLMConfig struct {
	Provider  LLMProvider `json:"provider"`
	APIKey    string      `json:"api_key,omitempty"`
	Model     string      `json:"model"`
	BaseURL   string      `json:"base_url,omitempty"` // 用于本地模型或自定义端点
	Enabled   bool        `json:"enabled"`
	UpdatedAt string      `json:"updated_at"`
}

// Conversation 表示一个对话会话，对应数据库 conversations 表。
type Conversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ConversationMessage 表示对话中的一条消息，对应数据库 messages 表。
type ConversationMessage struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversation_id"`
	Role           MessageRole      `json:"role"`
	Content        string           `json:"content"`
	ToolCalls      []ToolCall       `json:"tool_calls,omitempty"`
	ToolResults    []ToolCallResult `json:"tool_results,omitempty"`
	CreatedAt      string           `json:"created_at"`
}

// MemoryScope 表示长期记忆的作用域。
type MemoryScope string

const (
	MemoryScopeUser    MemoryScope = "user"
	MemoryScopeProject MemoryScope = "project"
	MemoryScopeGlobal  MemoryScope = "global"
)

// MemoryEntry 表示一条长期记忆条目。
type MemoryEntry struct {
	ID        string      `json:"id"`
	Scope     MemoryScope `json:"scope"`
	Key       string      `json:"key"`
	Content   string      `json:"content"`
	Source    string      `json:"source"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

// MemoryStats 表示记忆的容量统计。
type MemoryStats struct {
	TotalChars int     `json:"total_chars"`
	MaxChars   int     `json:"max_chars"`
	Usage      float64 `json:"usage"`
}

// MemorySnapshot 表示注入系统 Prompt 的记忆快照。
type MemorySnapshot struct {
	Entries []MemoryEntry `json:"entries"`
	Stats   MemoryStats   `json:"stats"`
}
