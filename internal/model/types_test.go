package model

import (
	"encoding/json"
	"testing"
)

func TestToolInfoManifestAsManifest(t *testing.T) {
	manifest := ToolManifest{
		DisplayName:  "Test Tool",
		Description:  "A test",
		Contribution: ContributionWeb,
		Web: &WebManifest{
			StartCommand: "node server.js",
			Port:         3000,
		},
	}
	data, _ := json.Marshal(manifest)

	tool := ToolInfo{
		ID:      "test:tool",
		Name:    "tool",
		Version: "1.0.0",
		Source:  SourcePython,
		Manifest: string(data),
	}

	got, err := tool.ManifestAsManifest()
	if err != nil {
		t.Fatalf("ManifestAsManifest: %v", err)
	}

	if got.DisplayName != "Test Tool" {
		t.Errorf("expected DisplayName 'Test Tool', got %q", got.DisplayName)
	}
	if got.Contribution != ContributionWeb {
		t.Errorf("expected Contribution 'web', got %q", got.Contribution)
	}
	if got.Web == nil || got.Web.Port != 3000 {
		t.Errorf("expected Web.Port 3000, got %+v", got.Web)
	}
}

func TestToolInfoManifestAsManifestInvalidJSON(t *testing.T) {
	tool := ToolInfo{
		ID:       "test:bad",
		Name:     "bad",
		Manifest: "{invalid json}",
	}
	_, err := tool.ManifestAsManifest()
	if err == nil {
		t.Fatal("expected error for invalid manifest JSON")
	}
}

func TestContributionTypeConstants(t *testing.T) {
	tests := []struct {
		ct  ContributionType
		exp string
	}{
		{ContributionWeb, "web"},
		{ContributionTerminal, "terminal"},
		{ContributionFile, "file"},
	}
	for _, tc := range tests {
		if string(tc.ct) != tc.exp {
			t.Errorf("expected %q, got %q", tc.exp, string(tc.ct))
		}
	}
}

func TestToolSourceConstants(t *testing.T) {
	tests := []struct {
		ts  ToolSource
		exp string
	}{
		{SourcePython, "python:uv"},
		{SourceJS, "js:bun"},
		{SourceBinary, "binary:marcus"},
		{SourceManual, "manual"},
		{SourceStore, "store"},
	}
	for _, tc := range tests {
		if string(tc.ts) != tc.exp {
			t.Errorf("expected %q, got %q", tc.exp, string(tc.ts))
		}
	}
}

func TestProcessStatusConstants(t *testing.T) {
	tests := []struct {
		ps  ProcessStatus
		exp string
	}{
		{ProcessIdle, "idle"},
		{ProcessLaunching, "launching"},
		{ProcessRunning, "running"},
		{ProcessStopping, "stopping"},
		{ProcessCrashed, "crashed"},
		{ProcessExited, "exited"},
	}
	for _, tc := range tests {
		if string(tc.ps) != tc.exp {
			t.Errorf("expected %q, got %q", tc.exp, string(tc.ps))
		}
	}
}

func TestInstallResultSerialization(t *testing.T) {
	r := InstallResult{
		PluginID: "test-plugin",
		Version:  "1.0.0",
		Success:  true,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded InstallResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.PluginID != "test-plugin" {
		t.Errorf("expected test-plugin, got %q", decoded.PluginID)
	}
	if decoded.Success != true {
		t.Errorf("expected success true")
	}
}

func TestUninstallResultSerialization(t *testing.T) {
	r := UninstallResult{
		ToolID:  "python:my_tool",
		Success: true,
		Message: "已卸载工具 My Tool",
	}
	data, _ := json.Marshal(r)
	var decoded UninstallResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ToolID != "python:my_tool" {
		t.Errorf("expected python:my_tool, got %q", decoded.ToolID)
	}
	if decoded.Message != "已卸载工具 My Tool" {
		t.Errorf("expected Chinese message, got %q", decoded.Message)
	}
}
