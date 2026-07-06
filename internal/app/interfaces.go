package app

import (
	"context"

	"Marcus/internal/model"
)

// ToolStore manages tool metadata.
type ToolStore interface {
	ListTools(category string) ([]model.ToolInfo, error)
	GetTool(id string) (*model.ToolInfo, error)
	UpsertTool(t model.ToolInfo) error
	DeleteTool(id string) error
	UpdateLastUsed(toolID string) error
	ListRecentTools(limit int) ([]model.ToolInfo, error)
	Close() error
}

// LogStore persists execution log entries.
type LogStore interface {
	AddLog(entry model.ProcessState) error
	GetLogs(toolID string, limit int) ([]model.ProcessState, error)
}

// ToolScanner discovers tools from installed runtimes.
type ToolScanner interface {
	ScanAll() ([]model.ToolInfo, error)
}

// ProcessManager controls tool subprocess lifecycle.
type ProcessManager interface {
	Start(manifest model.ToolManifest, args map[string]string) (*model.ProcessState, error)
	Stop(toolID string) error
	GetState(toolID string) *model.ProcessState
	Shutdown()
	RunSync(ctx context.Context, manifest model.ToolManifest, args map[string]string) (string, int, error)
}

// RuntimeChecker probes whether required runtimes are installed.
type RuntimeChecker interface {
	CheckAll() map[string]model.RuntimeInfo
}

// ManualToolParser creates a ToolInfo from user-supplied CLI fields.
type ManualToolParser interface {
	ParseManual(name, command, argType string) model.ToolInfo
}
