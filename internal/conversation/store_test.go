package conversation

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"Marcus/internal/model"
)

// setupTestConversationStore 创建内存 SQLite 并运行迁移。
func setupTestConversationStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := migrateTestDB(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewStore(db)
}

// migrateTestDB 为测试运行集中迁移。
func migrateTestDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id          TEXT PRIMARY KEY,
			title       TEXT,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS messages (
			id              TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			role            TEXT NOT NULL,
			content         TEXT,
			tool_calls      TEXT,
			tool_results    TEXT,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id)
		);
		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at);
	`)
	return err
}

// TestCreateAndGetConversation 验证创建和获取对话。
func TestCreateAndGetConversation(t *testing.T) {
	store := setupTestConversationStore(t)

	conv, err := store.CreateConversation("Test Chat")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if conv.ID == "" {
		t.Fatal("conversation id should not be empty")
	}

	got, err := store.GetConversation(conv.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if got == nil || got.Title != "Test Chat" {
		t.Fatalf("conversation title = %q, want Test Chat", got.Title)
	}
}

// TestAddAndGetMessages 验证消息添加和查询。
func TestAddAndGetMessages(t *testing.T) {
	store := setupTestConversationStore(t)

	conv, _ := store.CreateConversation("Chat")
	_, _ = store.AddMessage(conv.ID, model.RoleUser, "hi")
	_, _ = store.AddAssistantMessage(conv.ID, "hello", []model.ToolCall{
		{ID: "call_1", Function: model.ToolCallFunction{Name: "marcus-img2ascii"}},
	})
	_, _ = store.AddToolResults(conv.ID, []model.ToolCallResult{
		{ToolCallID: "call_1", Name: "marcus-img2ascii", Content: "done"},
	})

	msgs, err := store.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("messages count = %d, want 3", len(msgs))
	}
	if len(msgs[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool calls = %d, want 1", len(msgs[1].ToolCalls))
	}
	if len(msgs[2].ToolResults) != 1 {
		t.Fatalf("tool results = %d, want 1", len(msgs[2].ToolResults))
	}
}

// TestDeleteConversation 验证删除对话级联删除消息。
func TestDeleteConversation(t *testing.T) {
	store := setupTestConversationStore(t)

	conv, _ := store.CreateConversation("ToDelete")
	_, _ = store.AddMessage(conv.ID, model.RoleUser, "hi")

	if err := store.DeleteConversation(conv.ID); err != nil {
		t.Fatalf("delete conversation: %v", err)
	}

	got, err := store.GetConversation(conv.ID)
	if err != nil {
		t.Fatalf("get conversation after delete: %v", err)
	}
	if got != nil {
		t.Fatal("conversation should be deleted")
	}

	msgs, err := store.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("get messages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("messages should be empty, got %d", len(msgs))
	}
}
