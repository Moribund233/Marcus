package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseUVToolList(t *testing.T) {
	output := `marcus-textstats v0.1.0 (C:\Users\test\AppData\Roaming\uv\tools\marcus-textstats)
- marcus-textstats (C:\Users\test\.local\bin\marcus-textstats.exe)
`

	tools := parseUVToolList(output)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "marcus-textstats" {
		t.Fatalf("expected marcus-textstats, got %s", tools[0].Name)
	}
	if tools[0].Version != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", tools[0].Version)
	}
	if tools[0].EnvPath != `C:\Users\test\AppData\Roaming\uv\tools\marcus-textstats` {
		t.Fatalf("unexpected env path: %s", tools[0].EnvPath)
	}
}

func TestScanUV(t *testing.T) {
	dir, _ := os.MkdirTemp("", "marcus-test-*")
	defer os.RemoveAll(dir)

	reg, err := NewRegistry(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()

	s := &Scanner{registry: reg, toolsDir: dir}
	tools, err := s.ScanUV()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("discovered %d uv tools", len(tools))
	for _, tool := range tools {
		t.Logf("  - %s (%s): %s [%s]", tool.DisplayName, tool.Name, tool.Contribution, tool.Category)
	}
}
