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
- [x] 创建 `internal/llm/` 模块
- [x] 实现 Provider 接口
- [x] 添加 OpenAI 提供者实现

### Phase 2: Agent Core (2-3 天)
- [x] 创建 `internal/agent/` 模块
- [x] 实现 TAO 循环
- [x] 工具注册表
- [x] 创建 `internal/memory/` 模块（长期记忆）
- [x] 实现记忆 CRUD、容量检查与 `memory` 工具
- [x] 修改 PromptManager 注入记忆冻结快照

### Phase 3: 对话存储 (1 天)
- [x] 创建 `internal/conversation/` 模块
- [x] SQLite Schema

### Phase 4: 前端 (3-4 天)
- [x] 侧边栏重构
- [x] 聊天界面
- [x] 设置页面

### Phase 5: 集成测试 (1-2 天)
- [x] 端到端测试
- [x] 错误处理

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

---

## 11. 记忆系统（补充设计）

> 参考 Hermes Agent 的核心设计：其记忆系统并非简单的 "remember this" 字段，而是由**持久化记忆（事实/偏好）**、**程序性技能（Skills）**和**会话搜索**组成的分层架构。Hermes 通过 `MEMORY.md`、`USER.md`、`SOUL.md` 等文件在会话开始时以**冻结快照**注入系统 Prompt，从而保证前缀缓存命中并降低上下文成本。
>
> 对于 Marcus，我们不需要复刻 Hermes 完整的多层记忆与 Skills 自进化能力，但需要实现一个**轻量、可扩展的记忆层**，让 Agent 能够记住用户偏好、常用路径、项目约定等稳定事实，并在后续会话中自动应用。

### 11.1 设计目标

- 跨会话持久化稳定事实与用户偏好
- 让 Agent 在系统 Prompt 中自动携带相关记忆
- 允许 Agent 通过工具自我管理记忆（添加/替换/删除）
- 保持实现简单，不引入外部向量数据库或复杂检索

### 11.2 记忆分层

| 层级 | 名称 | 用途 | 存储方式 | 注入方式 |
|------|------|------|----------|----------|
| L1 | 会话记忆 | 当前对话上下文 | `conversations`/`messages` 表 | 作为 Messages 传入 LLM |
| L2 | 长期记忆 | 用户偏好、环境事实、项目约定 | `memories` 表 + 可选 MEMORY.md | 冻结快照注入系统 Prompt |
| L3（可选） | 技能记忆 | 常用工具组合或工作流 | `skills` 表 / Markdown 文件 | 按关键词匹配后注入 |

**初期实现范围**：完成 L1（已有）和 L2；L3 作为后续扩展预留。

### 11.3 数据模型

```go
// internal/memory/model.go
type MemoryEntry struct {
    ID        string    // 唯一标识
    Scope     string    // "user" | "project" | "global"
    Key       string    // 简短键名，用于子字符串匹配
    Content   string    // 记忆内容
    Source    string    // 来源："agent" | "manual" | "imported"
    CreatedAt time.Time
    UpdatedAt time.Time
}

type MemoryStats struct {
    TotalChars int
    MaxChars   int
    Usage      float64 // 0.0 - 1.0
}
```

### 11.4 数据库 Schema

```sql
-- 长期记忆表
CREATE TABLE IF NOT EXISTS memories (
    id          TEXT PRIMARY KEY,
    scope       TEXT NOT NULL DEFAULT 'global',
    key         TEXT NOT NULL,
    content     TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT 'agent',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 可选：技能记忆表（预留）
CREATE TABLE IF NOT EXISTS skills (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    tags        TEXT,  -- JSON 数组
    content     TEXT NOT NULL,
    use_count   INTEGER DEFAULT 0,
    last_used   DATETIME,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 11.5 系统 Prompt 注入格式

每次 `Agent.Run` 启动时，从 `memory store` 加载冻结快照，按以下格式追加到系统 Prompt 末尾：

```text
══════════════════════════════════════════════════
MEMORY (long-term facts) [45% — 990/2,200 chars]
══════════════════════════════════════════════════
用户偏好使用中文回复
§
常用项目路径：D:\Project\go\Marcus
§
图片处理工具默认输出到桌面
```

注入规则：
- 仅注入 `scope='user'` 或 `scope='global'` 的条目
- 总字符上限默认 2,200（与 Hermes 对齐），可配置
- 采用**冻结快照**：本次会话中记忆变更会持久化，但已构建的系统 Prompt 不变，直到下次 Run 才刷新

### 11.6 memory 工具设计

Agent 通过调用 `memory` 工具管理长期记忆，无需用户手动编辑。

```go
// internal/agent/tools/memory.go
var MemoryTool = ToolDefinition{
    Name:        "memory",
    Description: "Manage long-term memory: add, replace, or remove stable facts and user preferences.",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "action": map[string]interface{}{
                "type": "string",
                "enum": []string{"add", "replace", "remove"},
            },
            "scope": map[string]interface{}{
                "type": "string",
                "enum": []string{"user", "project", "global"},
                "description": "Memory scope. Default: global",
            },
            "key": map[string]interface{}{
                "type": "string",
                "description": "Short unique key for substring matching",
            },
            "content": map[string]interface{}{
                "type": "string",
                "description": "Full memory content (required for add/replace)",
            },
            "old_text": map[string]interface{}{
                "type": "string",
                "description": "Substring of the existing entry to replace/remove",
            },
        },
        "required": []string{"action", "key"},
    },
}
```

操作语义：
- `add`：新增一条记忆；若 `key` 已存在则更新
- `replace`：通过 `old_text` 子字符串匹配并替换内容
- `remove`：通过 `old_text` 子字符串匹配并删除条目

### 11.7 容量管理

```go
// internal/memory/store.go
const DefaultMemoryLimit = 2200 // 字符

func (s *Store) CheckLimit(scope string) (*MemoryStats, error) {
    // 统计 scope 下所有 content 字符数
    // 返回使用百分比和建议
}

func (s *Store) Add(entry MemoryEntry) error {
    // 1. 检查容量
    // 2. 若超出，返回错误并让 LLM 决定替换/删除哪些条目
    // 3. 插入或更新
}
```

当记忆接近上限时，Agent 应收到系统提示：

```text
记忆容量接近上限（90%）。请考虑删除或合并不再相关的条目。
```

### 11.8 新增模块

```
internal/
├── memory/
│   ├── store.go          # 记忆持久化
│   ├── model.go          # 数据模型
│   ├── tool.go           # memory 工具定义
│   └── prompt.go         # 记忆快照渲染
```

### 11.9 与 Agent Core 集成

```go
// internal/agent/core.go
func (a *Agent) Run(ctx context.Context, sessionID string, userMessage string) error {
    // 1. 加载长期记忆快照
    memorySnapshot := a.memoryStore.BuildSnapshot()
    
    // 2. 构建系统提示词（包含记忆快照）
    systemPrompt := a.promptMgr.BuildSystemPrompt(memorySnapshot)
    
    // 3. 加载会话历史
    history := a.store.GetMessages(sessionID)
    
    // 4. TAO 循环（原有逻辑）
    // ...
}
```

### 11.10 实现计划补充

在原有 Phase 2（Agent Core）中增加以下任务：

- [x] 创建 `internal/memory/` 模块与 SQLite Schema
- [x] 实现记忆 CRUD 与容量检查
- [x] 实现 `memory` 工具并注册到 Agent
- [x] 修改 `PromptManager`，注入记忆冻结快照
- [x] 为记忆系统编写单元测试

---

## 12. 参考资源补充

- [Hermes Agent 持久化记忆](https://hermes-agent.nousresearch.com/docs/zh-Hans/user-guide/features/memory)
- [Hermes Agent Memory Providers](https://hermes-agent.nousresearch.com/docs/zh-Hans/user-guide/features/memory-providers)
- [Inside Hermes' Three-Layer Memory](https://hermes-agent.ai/blog/hermes-agent-memory-system)

---

## 13. 体验优化项

在核心功能实现之后，针对聊天交互与状态感知完成以下优化：

- [x] **ChatPage 错误提示**：发送消息失败时在输入框上方显示错误横幅，支持一键清除
  - 相关文件：`frontend/src/components/chat/ChatPage.tsx`
- [x] **侧边栏会话删除确认**：删除历史会话前弹出二次确认对话框，防止误操作
  - 相关文件：`frontend/src/components/sidebar.tsx`
- [x] **状态栏 LLM 状态显示**：在底部状态栏右侧展示当前配置的 LLM 提供者与模型名称
  - 相关文件：`frontend/src/components/status-bar.tsx`、`frontend/src/App.tsx`
- [x] **流式聊天响应**：后端通过 Wails 事件逐步推送助手回复片段，前端以打字机效果实时渲染
  - 相关文件：`internal/app/agent.go`、`frontend/src/hooks/useChat.ts`、`frontend/src/components/chat/ChatPage.tsx`
