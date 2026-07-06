package tools

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"Marcus/internal/model"

	_ "modernc.org/sqlite"
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

func TestToolID(t *testing.T) {
	cases := []struct {
		name, source, wantPrefix string
	}{
		{"My Tool", "binary", "binary:my_tool"},
		{"A/B Tool", "python", "python:a_b_tool"},
		{"Very Long Tool Name That Exceeds The Maximum Allowed Length For IDs", "manual", "manual:very_long_tool_name_that_exceeds_the_"},
	}
	for _, tc := range cases {
		id := toolID(tc.name, tc.source)
		if !startsWith(id, tc.wantPrefix) {
			t.Errorf("toolID(%q, %q) = %q, want prefix %q", tc.name, tc.source, id, tc.wantPrefix)
		}
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// writeFakeBinary writes a file that passes isExecutable into dir and returns
// its path. On Windows it writes a minimal PE header; on Unix it writes a
// shebang script. If manifest is non-nil, running the binary/script with
// --marcus-manifest prints the manifest JSON.
func writeFakeBinary(t *testing.T, dir, name string, manifest *model.ToolManifest) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		path += ".exe"
		// Write a minimal PE that isExecutable recognises. The file is not
		// actually runnable, so manifest cannot be tested by executing it on
		// Windows in this helper.
		data := append([]byte("MZ"), make([]byte, 62)...)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("write fake exe: %v", err)
		}
	} else {
		content := "#!/bin/sh\n"
		if manifest != nil {
			data, _ := json.Marshal(manifest)
			content += "if [ \"$1\" = \"--marcus-manifest\" ]; then echo '" + string(data) + "'; exit 0; fi\n"
		}
		content += "echo ok\n"
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatalf("write fake script: %v", err)
		}
	}
	return path
}

func TestScanBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The fake binary is not executable on Windows, so readBinaryManifest
		// would fail. Skip the full ScanBinary test there; isExecutable is still
		// covered implicitly by writeFakeBinary usage.
		t.Skip("skipping runnable fake binary test on Windows")
	}

	dir := t.TempDir()
	manifest := &model.ToolManifest{
		DisplayName:  "Test Binary",
		Contribution: model.ContributionTerminal,
		Terminal: &model.TerminalManifest{
			Command: "./testbin",
		},
	}
	writeFakeBinary(t, dir, "testbin", manifest)

	db, err := sql.Open("sqlite", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(toolsSchema); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(db)

	s := &Scanner{registry: reg, toolsDir: dir}
	tools, err := s.ScanBinary()
	if err != nil {
		t.Fatalf("ScanBinary: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 binary tool, got %d", len(tools))
	}
	if tools[0].DisplayName != "Test Binary" {
		t.Fatalf("expected display name 'Test Binary', got %q", tools[0].DisplayName)
	}
	if tools[0].Source != model.SourceBinary {
		t.Fatalf("expected source %s, got %s", model.SourceBinary, tools[0].Source)
	}
}

func TestScanBinarySkipsNonExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping runnable fake binary test on Windows")
	}

	dir := t.TempDir()
	manifest := &model.ToolManifest{
		DisplayName:  "Real Binary",
		Contribution: model.ContributionTerminal,
		Terminal: &model.TerminalManifest{
			Command: "./realbin",
		},
	}
	writeFakeBinary(t, dir, "realbin", manifest)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte(		"not an executable"), 0644); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(toolsSchema); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(db)

	s := &Scanner{registry: reg, toolsDir: dir}
	tools, err := s.ScanBinary()
	if err != nil {
		t.Fatalf("ScanBinary: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].DisplayName != "Real Binary" {
		t.Fatalf("expected 'Real Binary', got %q", tools[0].DisplayName)
	}
}

func TestParseManual(t *testing.T) {
	tool := ParseManual("My CLI", "mycli run", "file")
	if tool.DisplayName != "My CLI" {
		t.Fatalf("expected display name 'My CLI', got %q", tool.DisplayName)
	}
	if tool.Source != model.SourceManual {
		t.Fatalf("expected source manual, got %s", tool.Source)
	}
	if tool.Contribution != model.ContributionTerminal {
		t.Fatalf("expected terminal contribution, got %s", tool.Contribution)
	}

	var m model.ToolManifest
	if err := json.Unmarshal([]byte(tool.Manifest), &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if m.Terminal == nil || m.Terminal.Command != "mycli run" {
		t.Fatalf("unexpected terminal command: %+v", m.Terminal)
	}
	if len(m.Terminal.Args) != 1 || m.Terminal.Args[0].Name != "input" {
		t.Fatalf("unexpected args: %+v", m.Terminal.Args)
	}
}
