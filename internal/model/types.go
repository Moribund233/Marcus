package model

import (
	"encoding/json"
)

// Contribution types
type ContributionType string

const (
	ContributionWeb      ContributionType = "web"
	ContributionTerminal ContributionType = "terminal"
	ContributionFile     ContributionType = "file"
)

// Tool source
type ToolSource string

const (
	SourcePython ToolSource = "python:uv"
	SourceJS     ToolSource = "js:bun"
	SourceBinary ToolSource = "binary:marcus"
	SourceManual ToolSource = "manual"
)

// Process status
type ProcessStatus string

const (
	ProcessIdle      ProcessStatus = "idle"
	ProcessLaunching ProcessStatus = "launching"
	ProcessRunning   ProcessStatus = "running"
	ProcessStopping  ProcessStatus = "stopping"
	ProcessCrashed   ProcessStatus = "crashed"
	ProcessExited    ProcessStatus = "exited"
)

// ─── Tool Manifest ──────────────────────────────────────────

type WebManifest struct {
	StartCommand string `json:"start_command"`
	Port         int    `json:"port"`
	HealthCheck  string `json:"health_check,omitempty"`
	AutoOpen     bool   `json:"auto_open,omitempty"`
}

type TerminalArg struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Type    string `json:"type"` // "string" | "number" | "directory" | "file"
	Default any    `json:"default,omitempty"`
}

type TerminalManifest struct {
	Command string        `json:"command"`
	Args    []TerminalArg `json:"args,omitempty"`
}

type FileManifest struct {
	Command         string        `json:"command"`
	InputType       string        `json:"input_type"` // "file" | "files" | "directory"
	InputExtensions []string      `json:"input_extensions,omitempty"`
	OutputType      string        `json:"output_type"` // "file" | "directory"
	Args            []TerminalArg `json:"args,omitempty"`
}

type UIInput struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // "text" | "number" | "select" | "file" | "directory"
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
}

type UIAction struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Style string `json:"style,omitempty"` // "primary" | "default" | "destructive"
}

type UIOutputField struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"type"` // "text" | "image" | "file" | "table"
}

type UISchema struct {
	Type    string          `json:"type"` // "form" | "none"
	Inputs  []UIInput       `json:"inputs,omitempty"`
	Actions []UIAction      `json:"actions,omitempty"`
	Output  []UIOutputField `json:"output,omitempty"`
}

type ToolManifest struct {
	ToolID string `json:"-"` // internal: set by Marcus, not from tool manifest

	APIVersion   string            `json:"api_version,omitempty"`
	ID           string            `json:"id,omitempty"`
	DisplayName  string            `json:"display_name"`
	Description  string            `json:"description,omitempty"`
	Icon         string            `json:"icon,omitempty"`
	Category     string            `json:"category,omitempty"`
	Contribution ContributionType  `json:"contribution"`
	Web          *WebManifest      `json:"web,omitempty"`
	Terminal     *TerminalManifest `json:"terminal,omitempty"`
	File         *FileManifest     `json:"file,omitempty"`
	UI           *UISchema         `json:"ui,omitempty"`
}

// ─── Tool Registry ──────────────────────────────────────────

type ToolInfo struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	DisplayName  string           `json:"display_name"`
	Description  string           `json:"description,omitempty"`
	Icon         string           `json:"icon,omitempty"`
	Category     string           `json:"category"`
	Version      string           `json:"version,omitempty"`
	Source       ToolSource       `json:"source"`
	Contribution ContributionType `json:"contribution"`
	PackagePath  string           `json:"package_path,omitempty"`
	Manifest     string           `json:"manifest"`
	EntryPoint   string           `json:"entry_point,omitempty"`
	Enabled      bool             `json:"enabled"`
	Healthy      bool             `json:"healthy,omitempty"`
	HealthError  string           `json:"health_error,omitempty"`
	HealthHint   string           `json:"health_hint,omitempty"`
	LastSeen     string           `json:"last_seen"`
	CreatedAt    string           `json:"created_at"`
}

func (t ToolInfo) ManifestAsManifest() (ToolManifest, error) {
	var m ToolManifest
	err := json.Unmarshal([]byte(t.Manifest), &m)
	return m, err
}

// ─── Runtime ────────────────────────────────────────────────

type RuntimeInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Error     string `json:"error,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

// ToolHealth describes whether a discovered tool is actually runnable.
type ToolHealth struct {
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// ─── Process & Sandbox ──────────────────────────────────────

type ProcessState struct {
	ToolID    string        `json:"tool_id"`
	PID       int           `json:"pid,omitempty"`
	Status    ProcessStatus `json:"status"`
	Port      int           `json:"port,omitempty"`
	ExitCode  int           `json:"exit_code,omitempty"`
	ErrorLog  string `json:"error_log,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	StoppedAt string `json:"stopped_at,omitempty"`
}

type ResourceLimits struct {
	CPULimitPercent int `json:"cpu_limit_percent,omitempty"`
	MemoryLimitMB   int `json:"memory_limit_mb,omitempty"`
	TimeoutSeconds  int `json:"timeout_seconds,omitempty"`
}

// ─── Plugin Store ────────────────────────────────────────────

type StoreVersion struct {
	PublishedAt      string   `json:"published_at"`
	DownloadURL      string   `json:"download_url"`
	DisplayName      string   `json:"display_name"`
	Description      string   `json:"description"`
	Categories       []string `json:"categories"`
	Contribution     string   `json:"contribution"`
	MinMarcusVersion string   `json:"min_marcus_version"`
}

type StorePlugin struct {
	ID               string                  `json:"id"`
	LatestVersion    string                  `json:"latest_version"`
	Deprecated       bool                    `json:"deprecated"`
	DeprecationMsg   string                  `json:"deprecation_message,omitempty"`
	Versions         map[string]StoreVersion `json:"versions"`
	InstalledVersion string                  `json:"installed_version,omitempty"`
	UpdateAvailable  bool                    `json:"update_available,omitempty"`
}

type StoreIndex struct {
	SchemaVersion int                    `json:"schema_version"`
	UpdatedAt     string                 `json:"updated_at"`
	Plugins       map[string]StorePlugin `json:"plugins"`
}

type InstallResult struct {
	PluginID string `json:"plugin_id"`
	Version  string `json:"version"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

type UpdateCheckResult struct {
	PluginID        string `json:"plugin_id"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
}
