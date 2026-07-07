# Marcus LLM Agent 剩余开发工作计划

## 1. 背景与目标

根据设计文档 `docs/superpowers/specs/2026-07-06-llm-agent-design.md`，Marcus LLM Agent 的前端界面、Agent Core、记忆系统、LLM Provider 等主体功能已实现，且 `go test ./internal/...` 与前端 `npm run build` 均通过。但仍存在若干**影响实际运行**的问题与文档/Schema 不一致项，需要补齐。

本次计划聚焦修复高优先级的功能性缺陷，并补齐数据库 Schema，确保 Agent 在多轮工具调用、长上下文、模型连接测试等场景下稳定工作。

## 2. 范围

### 2.1 本次必须完成（高优先级）

1. **修复 LLM Provider `TestConnection` HTTP 方法错误**
   - `newRequest` 当前一律使用 `POST`，但 `/models`、`/api/tags` 应为 `GET`。
2. **补齐 OpenAI tool 结果消息所需的 `tool_call_id`**
   - `model.Message` 缺少该字段，导致多轮工具调用时 OpenAI API 无法匹配结果。
3. **接入上下文压缩器 `ContextCompressor`**
   - 已实现但未在 `Agent.Run` 中使用，需按设计文档在达到 85% token 阈值时自动压缩历史消息。
4. **在系统 Prompt 中注入完整工具参数 Schema**
   - 当前只列出工具名与描述，需把 `ToolFunctionDefinition.Parameters` 一并写入 Prompt。
5. **补齐数据库 Schema**
   - `llm_config` 表增加 `enabled`、`updated_at` 字段。
   - 新增 `skills` 表（L3 技能记忆预留）。

### 2.2 可选完成（中/低优先级）

- 前端路由 `/chat/:conversationId`、`/settings/ai`（桌面端收益有限，当前状态机已覆盖）。
- 前端组件/Hook 命名对齐（`MessageList.tsx`、`MessageInput.tsx`、`ToolCallCard.tsx`、`AISettings.tsx`、`useLLMConfig.ts`）。
- 生产环境加密密钥移除硬编码 fallback（需在部署文档中强制要求 `MARCUS_MASTER_KEY`）。

## 3. 实现方案

### 3.1 修复 `TestConnection` HTTP 方法

给各 Provider 的 `newRequest` 增加 `method string` 参数：

- `internal/llm/openai.go`
  - `newRequest(ctx, method, path, body)`
  - `Chat`、`ChatStream` 传 `http.MethodPost` + `/chat/completions`
  - `TestConnection` 传 `http.MethodGet` + `/models`
- `internal/llm/anthropic.go`
  - `/messages` 用 `POST`，`/models` 用 `GET`
- `internal/llm/ollama.go`
  - `/api/chat` 用 `POST`，`/api/tags` 用 `GET`

### 3.2 补齐 `tool_call_id`

1. `internal/model/agent.go`
   - `Message` 增加 `ToolCallID string `json:"tool_call_id,omitempty"``。
2. `internal/agent/core.go`
   - `buildMessageHistory` 在 `case model.RoleTool` 分支中，把 `r.ToolCallID` 赋给 `model.Message.ToolCallID`。
3. `internal/llm/openai.go`
   - `buildRequestBody` 构造 `openAIMessage` 时，把 `m.ToolCallID` 填入 `ToolCallID`。
4. `internal/agent/prompt.go`
   - `BuildToolResultMessage` 同样填充 `ToolCallID`。

### 3.3 接入 `ContextCompressor`

1. `internal/agent/core.go`
   - `Config` 增加 `MaxContextTokens int`（默认值按模型设定，如 OpenAI gpt-4o 取 128k）。
   - `Agent` 结构体增加 `compressor *ContextCompressor`。
   - `NewAgent` 中根据 `Config.MaxContextTokens` 初始化压缩器（阈值 0.85）。
   - `buildMessageHistory` 组装完 `history` 后调用：
     ```go
     if a.compressor != nil && a.compressor.ShouldCompress(history) {
         history = a.compressor.Compress(history)
     }
     ```
2. 控制系统提示长度
   - 在 `memory.Store.BuildSnapshot` 渲染记忆时保持 2,200 字符上限（已存在，检查是否生效）。

### 3.4 Prompt 注入工具参数 Schema

修改 `internal/agent/prompt.go` 的 `BuildSystemPrompt`：

```go
for _, tool := range tools {
    b.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
    if len(tool.Function.Parameters) > 0 {
        schemaJSON, _ := json.MarshalIndent(tool.Function.Parameters, "  ", "  ")
        b.WriteString(fmt.Sprintf("  Parameters schema:\n%s\n", schemaJSON))
    }
}
```

### 3.5 数据库迁移

在 `internal/db/migrations.go` 的 `All()` 切片末尾追加两条新迁移：

1. 给 `llm_config` 增加列：
   - `enabled INTEGER DEFAULT 1`
   - `updated_at DATETIME DEFAULT CURRENT_TIMESTAMP`
2. 创建 `skills` 表（字段同设计文档第 551–561 行）。

迁移函数中通过 `PRAGMA table_info` 或 `ALTER TABLE ... ADD COLUMN` 配合错误处理实现幂等，避免重复执行报错。

## 4. 关键修改文件

- `internal/llm/openai.go`
- `internal/llm/anthropic.go`
- `internal/llm/ollama.go`
- `internal/model/agent.go`
- `internal/agent/core.go`
- `internal/agent/prompt.go`
- `internal/db/migrations.go`
- 相关单元测试文件：`internal/llm/*_test.go`、`internal/agent/*_test.go`、`internal/db/*_test.go`（按需补充或更新）

## 5. 验证方式

1. 后端测试：
   ```powershell
   go test ./internal/...
   ```
2. 前端构建：
   ```powershell
   cd frontend
   npm run build
   ```
3. Wails 编译：
   ```powershell
   wails build
   ```
4. 手动验证（用户自行执行）：
   - 在 AI 设置中点击「测试连接」，确认 OpenAI/Anthropic/Ollama 连接成功。
   - 创建会话，发送需要调用工具的消息，确认多轮工具调用后模型仍能正确返回结果。
   - 发送超长会话历史，触发上下文压缩（可临时调低 `MaxContextTokens` 测试）。
