package tools

import (
	"path/filepath"
	"testing"

	"Marcus/internal/model"
)

func newTestLogStore(t *testing.T) (*LogStore, *Registry) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	reg, err := NewRegistry(dbPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { reg.Close() })

	logs, err := NewLogStore(reg.DB())
	if err != nil {
		t.Fatalf("NewLogStore: %v", err)
	}
	return logs, reg
}

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
