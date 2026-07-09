package memory

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"Marcus/internal/model"
)

// setupTestStore 创建内存 SQLite 并运行迁移。
func setupTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := migrateTestDB(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewStore(db, 200)
}

// migrateTestDB 为测试运行集中迁移。
func migrateTestDB(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id          TEXT PRIMARY KEY,
			scope       TEXT NOT NULL DEFAULT 'global',
			key         TEXT NOT NULL UNIQUE,
			content     TEXT NOT NULL,
			source      TEXT NOT NULL DEFAULT 'agent',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_memories_scope ON memories(scope);
		CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(key);
	`)
	return err
}

// TestMemoryAddAndGet 验证添加和查询记忆。
func TestMemoryAddAndGet(t *testing.T) {
	store := setupTestStore(t)

	stats, err := store.Add(model.MemoryEntry{
		Scope:   model.MemoryScopeUser,
		Key:     "language",
		Content: "用户偏好使用中文回复",
		Source:  "agent",
	})
	if err != nil {
		t.Fatalf("add memory: %v", err)
	}
	if stats.TotalChars != 10 {
		t.Fatalf("total chars = %d, want %d", stats.TotalChars, 10)
	}

	entry, err := store.GetByKey("language")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if entry == nil {
		t.Fatal("memory not found")
	}
	if entry.Content != "用户偏好使用中文回复" {
		t.Fatalf("content = %q, want %q", entry.Content, "用户偏好使用中文回复")
	}
}

// TestMemoryReplace 验证替换记忆内容。
func TestMemoryReplace(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.Add(model.MemoryEntry{
		Scope:   model.MemoryScopeGlobal,
		Key:     "project_path",
		Content: "常用项目路径：D:\\Project\\go\\Marcus",
	})
	if err != nil {
		t.Fatalf("add memory: %v", err)
	}

	_, err = store.Replace("project_path", "Marcus", "Marcus-v2")
	if err != nil {
		t.Fatalf("replace memory: %v", err)
	}

	entry, err := store.GetByKey("project_path")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if !strings.Contains(entry.Content, "Marcus-v2") {
		t.Fatalf("replaced content = %q", entry.Content)
	}
}

// TestMemoryRemove 验证删除整条记忆。
func TestMemoryRemove(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.Add(model.MemoryEntry{
		Scope:   model.MemoryScopeGlobal,
		Key:     "temp",
		Content: "temporary fact",
	})
	if err != nil {
		t.Fatalf("add memory: %v", err)
	}

	_, err = store.Remove("temp", "")
	if err != nil {
		t.Fatalf("remove memory: %v", err)
	}

	entry, err := store.GetByKey("temp")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if entry != nil {
		t.Fatal("memory should be removed")
	}
}

// TestMemorySnapshotAndPrompt 验证快照构建与 Prompt 渲染。
func TestMemorySnapshotAndPrompt(t *testing.T) {
	store := setupTestStore(t)

	_, _ = store.Add(model.MemoryEntry{
		Scope:   model.MemoryScopeUser,
		Key:     "lang",
		Content: "prefer Chinese",
	})
	_, _ = store.Add(model.MemoryEntry{
		Scope:   model.MemoryScopeProject,
		Key:     "project",
		Content: "Marcus",
	})

	snapshot, err := store.BuildSnapshot()
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if len(snapshot.Entries) != 1 {
		t.Fatalf("snapshot entries = %d, want 1 (project scope excluded)", len(snapshot.Entries))
	}

	prompt := store.RenderPrompt(snapshot)
	if !strings.Contains(prompt, "MEMORY") {
		t.Fatal("prompt should contain MEMORY header")
	}
	if !strings.Contains(prompt, "prefer Chinese") {
		t.Fatal("prompt should contain memory content")
	}
}

// TestMemoryToolCall 验证 memory 工具调用执行。
func TestMemoryToolCall(t *testing.T) {
	store := setupTestStore(t)

	result, err := store.ApplyToolCall(model.ToolCall{
		Function: model.ToolCallFunction{
			Name:      "memory",
			Arguments: `{"action":"add","key":"lang","content":"prefer Chinese"}`,
		},
	})
	if err != nil {
		t.Fatalf("apply memory tool: %v", err)
	}
	if !strings.Contains(result, "Memory added") {
		t.Fatalf("result = %q, want Memory added", result)
	}

	entry, err := store.GetByKey("lang")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if entry.Content != "prefer Chinese" {
		t.Fatalf("content = %q", entry.Content)
	}
}

func TestSuggestedLimit(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, DefaultMemoryLimit},
		{-1, DefaultMemoryLimit},
		{2000, 2000},
		{8000, 3200},
		{128000, 32000},
		{200000, 32000},
		{1000000, 32000},
	}
	for _, tc := range tests {
		got := SuggestedLimit(tc.input)
		if got != tc.expected {
			t.Errorf("SuggestedLimit(%d) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}
