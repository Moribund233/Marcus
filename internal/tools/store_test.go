package tools

import (
	"database/sql"
	"path/filepath"
	"testing"

	"Marcus/internal/model"

	_ "modernc.org/sqlite"
)

func newTestLogStore(t *testing.T) (*LogStore, *Registry) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(logSchema); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	reg := NewRegistry(db)
	logs := NewLogStore(db)
	return logs, reg
}

const logSchema = `
CREATE TABLE IF NOT EXISTS tools (
	id TEXT PRIMARY KEY, name TEXT NOT NULL, display_name TEXT NOT NULL,
	description TEXT, icon TEXT, category TEXT DEFAULT 'other', version TEXT,
	source TEXT NOT NULL, contribution TEXT NOT NULL, package_path TEXT,
	manifest TEXT NOT NULL, entry_point TEXT, enabled INTEGER DEFAULT 1,
	last_seen DATETIME DEFAULT CURRENT_TIMESTAMP, last_used DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS tool_runtime_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT, tool_id TEXT NOT NULL, pid INTEGER,
	status TEXT, started_at DATETIME, stopped_at DATETIME, exit_code INTEGER,
	port INTEGER, error_log TEXT
)`

func TestLogStoreAddAndGet(t *testing.T) {
	logs, _ := newTestLogStore(t)

	entry := model.ProcessState{
		ToolID:    "python:test",
		PID:       12345,
		Status:    model.ProcessRunning,
		StartedAt: "2024-01-01 12:00:00",
	}

	if err := logs.AddLog(entry); err != nil {
		t.Fatalf("AddLog: %v", err)
	}

	result, err := logs.GetLogs("python:test", 10)
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 log, got %d", len(result))
	}
	if result[0].ToolID != "python:test" {
		t.Errorf("expected tool ID 'python:test', got %q", result[0].ToolID)
	}
	if result[0].PID != 12345 {
		t.Errorf("expected PID 12345, got %d", result[0].PID)
	}
}

func TestLogStoreGetLogsLimit(t *testing.T) {
	logs, _ := newTestLogStore(t)

	// Add multiple logs.
	for i := 0; i < 5; i++ {
		logs.AddLog(model.ProcessState{
			ToolID: "python:test",
			PID:    i,
			Status: model.ProcessExited,
		})
	}

	// Get with limit.
	result, err := logs.GetLogs("python:test", 3)
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(result))
	}

	// Should be ordered by ID DESC (most recent first).
	if result[0].PID != 4 {
		t.Errorf("expected most recent PID 4, got %d", result[0].PID)
	}
}

func TestLogStoreGetLogsEmpty(t *testing.T) {
	logs, _ := newTestLogStore(t)

	result, err := logs.GetLogs("nonexistent", 10)
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(result))
	}
}
