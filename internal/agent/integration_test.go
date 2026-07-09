// Package agent 集成测试，验证 Agent 与对话、记忆存储的端到端流程。
package agent

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"Marcus/internal/conversation"
	"Marcus/internal/memory"
	"Marcus/internal/model"

	_ "modernc.org/sqlite"
)

// noopRunner 是一个不执行真实进程的 SyncRunner，仅返回空结果。
type noopRunner struct{}

func (n *noopRunner) RunSync(_ context.Context, _ model.ToolManifest, _ map[string]string) (string, int, error) {
	return "", 0, nil
}

// mockProvider 是一个不访问外部网络的 LLM Provider，用于端到端测试。
type mockProvider struct {
	responses []*model.ChatResponse
	index     int
}

func newMockProvider(responses []*model.ChatResponse) *mockProvider {
	return &mockProvider{responses: responses}
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Models() []model.Model {
	return []model.Model{{ID: "mock-model", Name: "Mock Model"}}
}

func (m *mockProvider) Chat(_ context.Context, _ *model.ChatRequest) (*model.ChatResponse, error) {
	if m.index >= len(m.responses) {
		return &model.ChatResponse{Content: "done", Usage: model.Usage{}}, nil
	}
	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

func (m *mockProvider) ChatStream(_ context.Context, _ *model.ChatRequest) (<-chan *model.ChatChunk, error) {
	ch := make(chan *model.ChatChunk, 1)
	ch <- &model.ChatChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockProvider) TestConnection(_ context.Context) error { return nil }

// newTestDB 创建一个内存 SQLite 数据库，调用方负责关闭。
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// newTestAgent 使用 mock provider 创建测试用 Agent。
func newTestAgent(t *testing.T, responses []*model.ChatResponse) (*Agent, *sql.DB) {
	t.Helper()
	db := newTestDB(t)
	if err := migrateTestDB(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := Config{
		LLM:           newMockProvider(responses),
		Runner:        &noopRunner{},
		ConvStore:     conversation.NewStore(db),
		MemoryStore:   memory.NewStore(db, memory.DefaultMemoryLimit),
		MaxIterations: 5,
	}
	return NewAgent(cfg), db
}

// migrateTestDB 为测试创建所需的表。
func migrateTestDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY, title TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, role TEXT NOT NULL,
			content TEXT, tool_calls TEXT, tool_results TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id)
		);
		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at);
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY, scope TEXT NOT NULL DEFAULT 'global', key TEXT NOT NULL UNIQUE,
			content TEXT NOT NULL, source TEXT NOT NULL DEFAULT 'agent',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_memories_scope ON memories(scope);
		CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(key);
	`)
	return err
}

// TestAgentRun_SimpleReply 验证 Agent 能在单轮对话中返回助手回复并持久化。
func TestAgentRun_SimpleReply(t *testing.T) {
	responses := []*model.ChatResponse{
		{Content: "Hello from mock", Usage: model.Usage{TotalTokens: 10}},
	}
	ag, db := newTestAgent(t, responses)
	defer db.Close()

	conv, err := ag.convStore.CreateConversation("Test")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	resp, err := ag.Run(context.Background(), conv.ID, "hi")
	if err != nil {
		t.Fatalf("agent run: %v", err)
	}
	if resp.Content != "Hello from mock" {
		t.Errorf("unexpected content: %s", resp.Content)
	}

	msgs, err := ag.convStore.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != model.RoleUser || msgs[0].Content != "hi" {
		t.Errorf("first message mismatch: %v", msgs[0])
	}
	if msgs[1].Role != model.RoleAssistant || msgs[1].Content != "Hello from mock" {
		t.Errorf("second message mismatch: %v", msgs[1])
	}
}

// TestAgentRun_ToolCallLoop 验证 Agent 能执行工具调用循环并保存结果。
func TestAgentRun_ToolCallLoop(t *testing.T) {
	responses := []*model.ChatResponse{
		{
			Content: "",
			ToolCalls: []model.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: model.ToolCallFunction{
						Name:      "memory",
						Arguments: `{"action":"add","key":"greeting","content":"user likes tests"}`,
					},
				},
			},
		},
		{Content: "Memory updated", Usage: model.Usage{TotalTokens: 20}},
	}
	ag, db := newTestAgent(t, responses)
	defer db.Close()

	conv, err := ag.convStore.CreateConversation("Tool Test")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	resp, err := ag.Run(context.Background(), conv.ID, "remember this")
	if err != nil {
		t.Fatalf("agent run: %v", err)
	}
	if resp.Content != "Memory updated" {
		t.Errorf("unexpected final content: %s", resp.Content)
	}

	msgs, err := ag.convStore.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages (user, assistant tool call, tool result, final assistant), got %d", len(msgs))
	}
	if msgs[2].Role != model.RoleTool {
		t.Errorf("expected tool result message, got %s", msgs[2].Role)
	}

	// 验证记忆已写入。
	entry, err := ag.memoryStore.GetByKey("greeting")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if entry == nil {
		t.Fatal("expected memory entry to exist")
	}
	if entry.Content != "user likes tests" {
		t.Errorf("unexpected memory value: %s", entry.Content)
	}
}

// TestAgentRun_ConversationNotFound 验证传入不存在会话 ID 时返回错误。
func TestAgentRun_ConversationNotFound(t *testing.T) {
	ag, db := newTestAgent(t, nil)
	defer db.Close()

	_, err := ag.Run(context.Background(), "non-existent-id", "hi")
	if err == nil {
		t.Fatal("expected error for missing conversation")
	}
}

// TestBuildMessageHistory_ToolCallID 验证 tool 结果消息会携带 ToolCallID。
func TestBuildMessageHistory_ToolCallID(t *testing.T) {
	ag, db := newTestAgent(t, nil)
	defer db.Close()

	conv, err := ag.convStore.CreateConversation("ToolCallID Test")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	if _, err := ag.convStore.AddAssistantMessage(conv.ID, "", []model.ToolCall{
		{ID: "call_abc", Type: "function", Function: model.ToolCallFunction{Name: "test_tool"}},
	}); err != nil {
		t.Fatalf("add assistant message: %v", err)
	}
	if _, err := ag.convStore.AddToolResults(conv.ID, []model.ToolCallResult{
		{ToolCallID: "call_abc", Name: "test_tool", Role: "tool", Content: "result"},
	}); err != nil {
		t.Fatalf("add tool results: %v", err)
	}

	history, err := ag.buildMessageHistory(conv.ID, "system prompt")
	if err != nil {
		t.Fatalf("build message history: %v", err)
	}

	var found bool
	for _, m := range history {
		if m.Role == model.RoleTool && m.ToolCallID == "call_abc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected tool message with ToolCallID 'call_abc' in history")
	}
}

// TestBuildSystemPrompt_IncludesSchema 验证系统提示词中包含工具参数 Schema。
func TestBuildSystemPrompt_IncludesSchema(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterMemoryTool()
	pm := NewPromptManager(registry)

	prompt := pm.BuildSystemPrompt("", "")
	if !strings.Contains(prompt, "Parameters schema") {
		t.Errorf("expected system prompt to contain 'Parameters schema', got:\n%s", prompt)
	}
}

// TestBuildMessageHistory_Compression 验证超过 token 阈值时历史消息会被压缩。
func TestBuildMessageHistory_Compression(t *testing.T) {
	ag, db := newTestAgent(t, nil)
	defer db.Close()

	// 强制压缩器只能保留极少消息。
	ag.compressor = NewContextCompressor(32, 0.5)

	conv, err := ag.convStore.CreateConversation("Compression Test")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	for i := 0; i < 10; i++ {
		if _, err := ag.convStore.AddMessage(conv.ID, model.RoleUser, "this is a relatively long message content to exceed the tiny limit"); err != nil {
			t.Fatalf("add message: %v", err)
		}
	}

	history, err := ag.buildMessageHistory(conv.ID, "system prompt")
	if err != nil {
		t.Fatalf("build message history: %v", err)
	}

	if len(history) >= 12 { // system + 10 user messages
		t.Errorf("expected history to be compressed, got %d messages", len(history))
	}
}

// TestConsolidateMemory 验证 Agent 能将对话内容归纳为 L2 记忆。
func TestConsolidateMemory(t *testing.T) {
	factJSON := `[{"key":"preferred-framework","content":"User prefers Gin framework"}]`
	responses := []*model.ChatResponse{
		{Content: factJSON, Usage: model.Usage{TotalTokens: 15}},
	}
	ag, db := newTestAgent(t, responses)
	defer db.Close()

	conv, err := ag.convStore.CreateConversation("Consolidation Test")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	if _, err := ag.convStore.AddMessage(conv.ID, model.RoleUser, "hi, I use Gin for my Go projects"); err != nil {
		t.Fatalf("add message: %v", err)
	}
	if _, err := ag.convStore.AddAssistantMessage(conv.ID, "Great choice! Gin is fast.", nil); err != nil {
		t.Fatalf("add assistant message: %v", err)
	}
	if _, err := ag.convStore.AddMessage(conv.ID, model.RoleUser, "and I prefer PostgreSQL"); err != nil {
		t.Fatalf("add message: %v", err)
	}
	if _, err := ag.convStore.AddAssistantMessage(conv.ID, "PostgreSQL is solid.", nil); err != nil {
		t.Fatalf("add assistant message: %v", err)
	}

	ag.ConsolidateMemory(context.Background(), conv.ID)

	entry, err := ag.memoryStore.GetByKey("preferred-framework")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if entry == nil {
		t.Fatal("expected consolidated memory to exist")
	}
	if !strings.Contains(entry.Content, "Gin") {
		t.Errorf("memory content = %q, want it to mention Gin", entry.Content)
	}
}

// TestConsolidateMemory_SkipsShortConversation 验证短对话不会触发归纳。
func TestConsolidateMemory_SkipsShortConversation(t *testing.T) {
	ag, db := newTestAgent(t, nil)
	defer db.Close()

	conv, err := ag.convStore.CreateConversation("Short Test")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	// 只有一条消息，不应触发归纳
	if _, err := ag.convStore.AddMessage(conv.ID, model.RoleUser, "hi"); err != nil {
		t.Fatalf("add message: %v", err)
	}

	ag.ConsolidateMemory(context.Background(), conv.ID)

	stats, err := ag.memoryStore.Stats("")
	if err != nil {
		t.Fatalf("memory stats: %v", err)
	}
	if stats.TotalChars != 0 {
		t.Errorf("expected empty memory, got %d chars", stats.TotalChars)
	}
}


