# Marcus LLM Agent 集成设计文档

**日期**: 2026-07-06  
**状态**: 已批准  
**作者**: Marcus Team

---

## 1. 概述

将 Marcus 从工具管理平台演进为支持 LLM Agent 的智能代理系统。用户可以通过自然语言对话，让 Agent 自动选择和执行 Marcus 工具。

### 1.1 设计目标

- 保留现有工具管理功能
- 添加 LLM Agent 对话能力
- 支持多种 LLM 提供者（OpenAI/Anthropic/本地模型）
- 复用现有架构模式（参考 Hermes Agent）

### 1.2 核心设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 交互入口 | 侧边栏上下分区 | 保持工具可见性，对话独立 |
| 对话存储 | SQLite | 复用现有数据库，FTS5 支持 |
| LLM 提供者 | 用户配置 | 灵活性高，支持多种场景 |
| 工具描述 | 自动生成 | 从 manifest 转换，无需额外维护 |

---

## 2. 侧边栏重构

### 2.1 布局设计

```
┌─────────────────────────┐
│ ▼ 对话                  │  ← 可折叠区域
│   + 新建对话            │
│   📝 图片转ASCII        │  ← 历史对话列表
│   📝 批量重命名文件      │
│   📝 ...                │
├─────────────────────────┤
│ ▼ 工具                  │  ← 可折叠区域
│   🌐 Web 工具           │
│   💻 Terminal 工具      │
│   📁 File 工具          │
└─────────────────────────┘
```

### 2.2 组件结构

```typescript
// 新增组件
components/
├── sidebar/
│   ├── Sidebar.tsx          # 主侧边栏容器
│   ├── ConversationList.tsx  # 对话列表
│   └── ToolSection.tsx      # 工具区域（现有）
```

### 2.3 状态管理

```typescript
// 侧边栏状态
interface SidebarState {
  conversationExpanded: boolean  // 对话区域展开状态
  toolExpanded: boolean         // 工具区域展开状态
  activeConversationId: string | null
}
```

---

## 3. 聊天界面

### 3.1 界面布局

```
┌─────────────────────────────────────────────┐
│  💬 对话标题                                │
├─────────────────────────────────────────────┤
│                                             │
│  👤 用户: 把这张图片转成字符画              │
│                                             │
│  🤖 Agent: 我来帮你处理。正在调用...        │
│     ┌─────────────────────────┐            │
│     │ 🔧 marcus-img2ascii     │            │
│     │ 参数: input=/path/img   │            │
│     │ 状态: ⏳ 执行中...      │            │
│     └─────────────────────────┘            │
│                                             │
│  🤖 Agent: 完成！字符画已保存到...          │
│                                             │
├─────────────────────────────────────────────┤
│  [📎] 输入消息...                    [发送] │
└─────────────────────────────────────────────┘
```

### 3.2 消息类型

```typescript
interface Message {
  id: string
  role: 'user' | 'assistant' | 'system' | 'tool'
  content: string
  toolCalls?: ToolCall[]
  timestamp: string
}

interface ToolCall {
  id: string
  toolId: string
  toolName: string
  params: Record<string, any>
  status: 'pending' | 'running' | 'completed' | 'error'
  result?: string
  error?: string
}
```

### 3.3 Agent 执行流程

```
用户输入 → Agent Core → LLM API → 解析响应
                                    ↓
                              工具调用? → 执行工具 → 返回结果 → 继续对话
                                    ↓
                              最终回答 → 保存对话 → 显示给用户
```

---

## 4. LLM 提供者配置

### 4.1 设置界面

```
┌─────────────────────────────────────────────┐
│  ⚙️ 设置 > AI 助手                          │
├─────────────────────────────────────────────┤
│                                             │
│  LLM 提供者: [OpenAI ▼]                    │
│                                             │
│  API Key: [sk-••••••••••••]                │
│                                             │
│  模型: [gpt-4o ▼]                           │
│                                             │
│  [测试连接]                                 │
│                                             │
│  ─────────────────────────────────────────  │
│                                             │
│  支持的提供者:                              │
│  • OpenAI (GPT-4o, GPT-4-turbo)            │
│  • Anthropic (Claude 3.5 Sonnet)           │
│  • 本地模型 (Ollama, LM Studio)            │
│                                             │
└─────────────────────────────────────────────┘
```

### 4.2 提供者接口

```go
// internal/llm/provider.go
type Provider interface {
    // Name 返回提供者名称
    Name() string
    
    // Models 返回支持的模型列表
    Models() []Model
    
    // Chat 发送聊天请求
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    
    // ChatStream 流式聊天
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan *ChatChunk, error)
}

type ChatRequest struct {
    Model       string
    Messages    []Message
    Tools       []ToolDefinition
    Temperature float64
    MaxTokens   int
}

type ChatResponse struct {
    Content   string
    ToolCalls []ToolCall
    Usage     Usage
}
```

### 4.3 支持的提供者

| 提供者 | 模型 | API Key 格式 |
|--------|------|--------------|
| OpenAI | gpt-4o, gpt-4-turbo | sk-... |
| Anthropic | claude-3-5-sonnet | sk-ant-... |
| Ollama | 本地模型 | 无需 API Key |

---

## 5. 后端架构

### 5.1 新增模块

```
internal/
├── agent/
│   ├── core.go           # Agent 核心 (TAO 循环)
│   ├── registry.go       # 工具注册表
│   ├── prompt.go         # Prompt 模板
│   └── executor.go       # 工具执行器
├── llm/
│   ├── provider.go       # 提供者接口
│   ├── openai.go         # OpenAI 实现
│   ├── anthropic.go      # Anthropic 实现
│   └── ollama.go         # Ollama 实现
└── conversation/
    ├── store.go          # 对话存储 (SQLite)
    └── model.go          # 数据模型
```

### 5.2 Agent Core (TAO 循环)

```go
// internal/agent/core.go
type Agent struct {
    registry   *ToolRegistry
    llm        llm.Provider
    store      *conversation.Store
    promptMgr  *PromptManager
}

// Run 执行 Agent 主循环
func (a *Agent) Run(ctx context.Context, sessionID string, userMessage string) error {
    // 1. 加载会话历史
    history := a.store.GetMessages(sessionID)
    
    // 2. 构建系统提示词
    systemPrompt := a.promptMgr.BuildSystemPrompt()
    
    // 3. TAO 循环
    for {
        // Thought: 调用 LLM
        response, err := a.llm.Chat(ctx, &llm.ChatRequest{
            Model:    a.config.Model,
            Messages: append([]llm.Message{{Role: "system", Content: systemPrompt}}, history...),
            Tools:    a.registry.GetToolDefinitions(),
        })
        if err != nil {
            return err
        }
        
        // 保存助手回复
        a.store.AddMessage(sessionID, "assistant", response.Content)
        
        // Action: 检查是否有工具调用
        if len(response.ToolCalls) == 0 {
            // 无工具调用，对话结束
            return nil
        }
        
        // 执行工具调用
        for _, toolCall := range response.ToolCalls {
            result := a.executor.Execute(toolCall)
            a.store.AddToolResult(sessionID, toolCall.ID, result)
        }
        
        // Observation: 将工具结果添加到历史
        history = a.store.GetMessages(sessionID)
    }
}
```

### 5.3 工具注册表

```go
// internal/agent/registry.go
type ToolRegistry struct {
    tools map[string]*ToolDefinition
}

// 从 Marcus 工具自动生成 LLM 工具定义
func (r *ToolRegistry) RegisterFromManifest(manifest *model.ToolManifest) {
    tool := &ToolDefinition{
        Name:        manifest.ID,
        Description: manifest.Description,
        Parameters:  convertParams(manifest),
    }
    r.tools[manifest.ID] = tool
}

// 转换参数格式为 JSON Schema
func convertParams(manifest *model.ToolManifest) map[string]interface{} {
    // 根据 contribution 类型转换
    switch manifest.Contribution {
    case model.ContributionTerminal:
        return convertTerminalParams(manifest.Terminal)
    case model.ContributionWeb:
        return convertWebParams(manifest.Web)
    case model.ContributionFile:
        return convertFileParams(manifest.File)
    }
    return nil
}
```

### 5.4 对话存储

```go
// internal/conversation/store.go
type Store struct {
    db *sql.DB
}

func (s *Store) Migrate() error {
    schema := `
    CREATE TABLE IF NOT EXISTS conversations (
        id TEXT PRIMARY KEY,
        title TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE TABLE IF NOT EXISTS messages (
        id TEXT PRIMARY KEY,
        conversation_id TEXT NOT NULL,
        role TEXT NOT NULL,
        content TEXT,
        tool_calls TEXT,  -- JSON
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (conversation_id) REFERENCES conversations(id)
    );
    `
    _, err := s.db.Exec(schema)
    return err
}
```

---

## 6. 前端架构

### 6.1 新增页面/组件

```
frontend/src/
├── components/
│   ├── chat/
│   │   ├── ChatPage.tsx        # 聊天主页面
│   │   ├── MessageList.tsx     # 消息列表
│   │   ├── MessageInput.tsx    # 输入框
│   │   └── ToolCallCard.tsx    # 工具调用卡片
│   └── settings/
│       └── AISettings.tsx      # AI 设置页面
├── hooks/
│   ├── useChat.ts              # 聊天逻辑
│   └── useLLMConfig.ts        # LLM 配置
```

### 6.2 路由结构

```typescript
// 新增路由
/routes
├── /chat/:conversationId  # 聊天页面
└── /settings/ai           # AI 设置
```

---

## 7. 数据库 Schema

### 7.1 新增表

```sql
-- 对话表
CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 消息表
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    role TEXT NOT NULL,  -- user, assistant, system, tool
    content TEXT,
    tool_calls TEXT,     -- JSON 格式的工具调用
    tool_result TEXT,    -- JSON 格式的工具结果
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id)
);

-- LLM 配置表
CREATE TABLE llm_config (
    provider TEXT PRIMARY KEY,
    api_key TEXT,
    model TEXT,
    base_url TEXT,  -- 用于本地模型
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 8. 实现计划

### Phase 1: 基础设施 (1-2 天)
- [ ] 创建 `internal/llm/` 模块
- [ ] 实现 Provider 接口
- [ ] 添加 OpenAI 提供者实现

### Phase 2: Agent Core (2-3 天)
- [ ] 创建 `internal/agent/` 模块
- [ ] 实现 TAO 循环
- [ ] 工具注册表

### Phase 3: 对话存储 (1 天)
- [ ] 创建 `internal/conversation/` 模块
- [ ] SQLite Schema

### Phase 4: 前端 (3-4 天)
- [ ] 侧边栏重构
- [ ] 聊天界面
- [ ] 设置页面

### Phase 5: 集成测试 (1-2 天)
- [ ] 端到端测试
- [ ] 错误处理

---

## 9. 参考资源

- [Hermes Agent Architecture](https://hermes-agent.nousresearch.com/docs/developer-guide/architecture/)
- [OpenAI Function Calling](https://platform.openai.com/docs/guides/function-calling)
- [Anthropic Tool Use](https://docs.anthropic.com/claude/docs/tool-use)

---

## 10. 确认项

| 项目 | 决策 | 实现方式 |
|------|------|----------|
| API Key 存储 | 后端加密后存储 | 使用 AES-256-GCM 加密后存入 `llm_config` 表 |
| 工具调用超时 | 由 LLM 决定 | Agent 根据 LLM 响应动态设置，不硬编码 |
| 上下文长度限制 | 根据模型上下文决定 | 达到 85% 时自动压缩历史消息 |

### 10.1 API Key 加密存储

```go
// internal/config/crypto.go
func EncryptAPIKey(key string) (string, error) {
    // 使用 AES-256-GCM 加密
    // 密钥从环境变量或配置文件获取
}

func DecryptAPIKey(encrypted string) (string, error) {
    // 解密 API Key
}
```

### 10.2 上下文压缩策略

```go
// internal/agent/compressor.go
type ContextCompressor struct {
    maxTokens int
    threshold float64  // 0.85 = 85%
}

func (c *ContextCompressor) ShouldCompress(messages []Message) bool {
    totalTokens := countTokens(messages)
    return float64(totalTokens) > float64(c.maxTokens) * c.threshold
}

func (c *ContextCompressor) Compress(messages []Message) []Message {
    // 保留最近的 N 条消息 + 系统提示
    // 对旧消息进行摘要
}
```
