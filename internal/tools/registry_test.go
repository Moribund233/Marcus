package tools

import (
	"os"
	"path/filepath"
	"testing"

	"Marcus/internal/model"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	reg, err := NewRegistry(dbPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

func TestRegistryUpsertAndGet(t *testing.T) {
	reg := newTestRegistry(t)

	tool := model.ToolInfo{
		ID:           "python:test_tool",
		Name:         "test_tool",
		DisplayName:  "Test Tool",
		Description:  "A test tool",
		Category:     "dev",
		Version:      "1.0.0",
		Source:       model.SourcePython,
		Contribution: model.ContributionTerminal,
		Enabled:      true,
	}

	if err := reg.UpsertTool(tool); err != nil {
		t.Fatalf("UpsertTool: %v", err)
	}

	got, err := reg.GetTool("python:test_tool")
	if err != nil {
		t.Fatalf("GetTool: %v", err)
	}

	if got.ID != "python:test_tool" {
		t.Errorf("expected ID 'python:test_tool', got %q", got.ID)
	}
	if got.DisplayName != "Test Tool" {
		t.Errorf("expected DisplayName 'Test Tool', got %q", got.DisplayName)
	}
	if got.Enabled != true {
		t.Errorf("expected enabled true")
	}
	if got.Source != model.SourcePython {
		t.Errorf("expected Source 'python:uv', got %q", got.Source)
	}

	// Ensure manifest was auto-populated.
	if got.Manifest == "" {
		t.Error("expected non-empty Manifest after upsert")
	}
}

func TestRegistryGetNonExistent(t *testing.T) {
	reg := newTestRegistry(t)
	_, err := reg.GetTool("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent tool")
	}
}

func TestRegistryListToolsAll(t *testing.T) {
	reg := newTestRegistry(t)

	tools := []model.ToolInfo{
		{ID: "python:a", Name: "a", DisplayName: "Tool A", Source: model.SourcePython, Contribution: model.ContributionTerminal, Enabled: true, Category: "dev"},
		{ID: "js:b", Name: "b", DisplayName: "Tool B", Source: model.SourceJS, Contribution: model.ContributionWeb, Enabled: true, Category: "image"},
		{ID: "python:c", Name: "c", DisplayName: "Tool C", Source: model.SourcePython, Contribution: model.ContributionFile, Enabled: true, Category: "other"},
	}

	for _, tool := range tools {
		if err := reg.UpsertTool(tool); err != nil {
			t.Fatalf("UpsertTool %s: %v", tool.ID, err)
		}
	}

	// List all tools.
	all, err := reg.ListTools("all")
	if err != nil {
		t.Fatalf("ListTools('all'): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 tools, got %d", len(all))
	}

	// List by category.
	dev, err := reg.ListTools("dev")
	if err != nil {
		t.Fatalf("ListTools('dev'): %v", err)
	}
	if len(dev) != 1 || dev[0].ID != "python:a" {
		t.Errorf("expected 1 dev tool, got %d", len(dev))
	}
}

func TestRegistryListToolsDisabled(t *testing.T) {
	reg := newTestRegistry(t)

	if err := reg.UpsertTool(model.ToolInfo{
		ID: "python:disabled", Name: "disabled", DisplayName: "Disabled Tool",
		Source: model.SourcePython, Contribution: model.ContributionTerminal,
		Enabled: false, Category: "other",
	}); err != nil {
		t.Fatalf("UpsertTool: %v", err)
	}

	all, err := reg.ListTools("all")
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	// Disabled tools should not appear in standard listing
	for _, tool := range all {
		if tool.ID == "python:disabled" {
			t.Error("disabled tool was listed")
		}
	}
}

func TestRegistryUpsertUpdatesExisting(t *testing.T) {
	reg := newTestRegistry(t)

	original := model.ToolInfo{
		ID: "python:updatable", Name: "updatable", DisplayName: "Original",
		Source: model.SourcePython, Contribution: model.ContributionTerminal,
		Enabled: true, Version: "1.0.0", Category: "dev",
	}
	if err := reg.UpsertTool(original); err != nil {
		t.Fatalf("first UpsertTool: %v", err)
	}

	updated := original
	updated.DisplayName = "Updated"
	updated.Version = "2.0.0"
	updated.Description = "Updated description"
	if err := reg.UpsertTool(updated); err != nil {
		t.Fatalf("second UpsertTool: %v", err)
	}

	got, err := reg.GetTool("python:updatable")
	if err != nil {
		t.Fatalf("GetTool: %v", err)
	}
	if got.DisplayName != "Updated" {
		t.Errorf("expected DisplayName 'Updated', got %q", got.DisplayName)
	}
	if got.Version != "2.0.0" {
		t.Errorf("expected Version '2.0.0', got %q", got.Version)
	}
	if got.Description != "Updated description" {
		t.Errorf("expected Description updated, got %q", got.Description)
	}
}

func TestRegistryDeleteTool(t *testing.T) {
	reg := newTestRegistry(t)

	tool := model.ToolInfo{
		ID: "python:deletable", Name: "deletable", DisplayName: "Deletable",
		Source: model.SourcePython, Contribution: model.ContributionTerminal,
		Enabled: true, Category: "other",
	}
	reg.UpsertTool(tool)

	if err := reg.DeleteTool("python:deletable"); err != nil {
		t.Fatalf("DeleteTool: %v", err)
	}

	_, err := reg.GetTool("python:deletable")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRegistryClose(t *testing.T) {
	reg := newTestRegistry(t)
	if err := reg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDBExposed(t *testing.T) {
	reg := newTestRegistry(t)
	db := reg.DB()
	if db == nil {
		t.Fatal("DB() returned nil")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
}

func TestRegistryDBFileCleanup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "marcus.db")

	reg, err := NewRegistry(dbPath)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// Write a tool and close.
	reg.UpsertTool(model.ToolInfo{
		ID: "python:persist", Name: "persist", DisplayName: "Persist Test",
		Source: model.SourcePython, Contribution: model.ContributionTerminal,
		Enabled: true, Category: "other",
	})
	reg.Close()

	// Verify the DB file exists and has content.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("DB file was not created on disk")
	}

	// Re-open and verify data persisted.
	reg2, err := NewRegistry(dbPath)
	if err != nil {
		t.Fatalf("NewRegistry (reopen): %v", err)
	}
	defer reg2.Close()

	tool, err := reg2.GetTool("python:persist")
	if err != nil {
		t.Fatalf("GetTool after reopen: %v", err)
	}
	if tool.DisplayName != "Persist Test" {
		t.Errorf("expected 'Persist Test', got %q", tool.DisplayName)
	}
}
