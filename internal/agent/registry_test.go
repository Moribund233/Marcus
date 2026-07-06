package agent

import (
	"testing"

	"Marcus/internal/model"
)

// TestRegisterFromManifestTerminal 验证 terminal 类型工具注册。
func TestRegisterFromManifestTerminal(t *testing.T) {
	reg := NewRegistry()
	manifest := &model.ToolManifest{
		ID:           "marcus-hello",
		DisplayName:  "Hello Tool",
		Description:  "Say hello",
		Contribution: model.ContributionTerminal,
		Terminal: &model.TerminalManifest{
			Command: "hello",
			Args: []model.TerminalArg{
				{Name: "name", Label: "Name", Type: "string"},
			},
		},
	}

	if err := reg.RegisterFromManifest(manifest); err != nil {
		t.Fatalf("register manifest: %v", err)
	}

	defs := reg.GetToolDefinitions()
	if len(defs) != 1 {
		t.Fatalf("definitions count = %d, want 1", len(defs))
	}
	if defs[0].Function.Name != "marcus-hello" {
		t.Fatalf("tool name = %q, want marcus-hello", defs[0].Function.Name)
	}
}

// TestRegisterFromManifestFile 验证 file 类型工具注册。
func TestRegisterFromManifestFile(t *testing.T) {
	reg := NewRegistry()
	manifest := &model.ToolManifest{
		ID:           "marcus-img2ascii",
		DisplayName:  "Image to ASCII",
		Description:  "Convert image to ASCII",
		Contribution: model.ContributionFile,
		File: &model.FileManifest{
			Command:         "img2ascii",
			InputType:       "file",
			InputExtensions: []string{".png", ".jpg"},
			OutputType:      "file",
			Args: []model.TerminalArg{
				{Name: "width", Label: "Width", Type: "integer", Default: 80},
			},
		},
	}

	if err := reg.RegisterFromManifest(manifest); err != nil {
		t.Fatalf("register manifest: %v", err)
	}

	defs := reg.GetToolDefinitions()
	if len(defs) != 1 {
		t.Fatalf("definitions count = %d, want 1", len(defs))
	}
	params := defs[0].Function.Parameters
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters properties missing")
	}
	if _, ok := props["input"]; !ok {
		t.Fatal("file tool should have input property")
	}
}

// TestRegisterMemoryTool 验证 memory 工具注册。
func TestRegisterMemoryTool(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterMemoryTool()

	defs := reg.GetToolDefinitions()
	if len(defs) != 1 {
		t.Fatalf("definitions count = %d, want 1", len(defs))
	}
	if defs[0].Function.Name != "memory" {
		t.Fatalf("tool name = %q, want memory", defs[0].Function.Name)
	}
}
