package store

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractManifest(t *testing.T) {
	manifest := map[string]string{
		"display_name": "Test Plugin",
		"description":  "A test plugin",
		"id":           "test-plugin",
	}
	manifestData, _ := json.Marshal(manifest)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("marcus-plugin.json")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	f.Write(manifestData)
	w.Close()

	got, err := extractManifest(buf.Bytes())
	if err != nil {
		t.Fatalf("extractManifest: %v", err)
	}

	if got.DisplayName != "Test Plugin" {
		t.Errorf("expected 'Test Plugin', got %q", got.DisplayName)
	}
	if got.ID != "test-plugin" {
		t.Errorf("expected 'test-plugin', got %q", got.ID)
	}
}

func TestExtractManifestMissingFile(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("some-other-file.json")
	f.Write([]byte(`{}`))
	w.Close()

	_, err := extractManifest(buf.Bytes())
	if err == nil {
		t.Fatal("expected error for missing marcus-plugin.json")
	}
}

func TestExtractManifestInvalidZip(t *testing.T) {
	_, err := extractManifest([]byte("not a zip file"))
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestExtractManifestInvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("marcus-plugin.json")
	f.Write([]byte("{invalid json}"))
	w.Close()

	_, err := extractManifest(buf.Bytes())
	if err == nil {
		t.Fatal("expected error for invalid JSON in manifest")
	}
}

func TestExtractPackage(t *testing.T) {
	manifestContent := `{"display_name":"Test","id":"test"}`
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	entries := []struct {
		name    string
		content string
	}{
		{"marcus-plugin.json", manifestContent},
		{"dist/index.js", "console.log('hello');"},
		{"dist/package.json", `{"name":"test","version":"1.0.0"}`},
	}
	for _, e := range entries {
		f, _ := w.Create(e.name)
		f.Write([]byte(e.content))
	}
	w.Close()

	dir := t.TempDir()
	if err := extractPackage(buf.Bytes(), dir); err != nil {
		t.Fatalf("extractPackage: %v", err)
	}

	// Verify extracted files.
	for _, e := range entries {
		path := filepath.Join(dir, e.name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after extraction", e.name)
		}
	}
}

func TestExtractPackageZipSlip(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("../../../etc/passwd")
	f.Write([]byte("root:x:0:0:"))
	w.Close()

	dir := t.TempDir()
	if err := extractPackage(buf.Bytes(), dir); err != nil {
		t.Fatalf("extractPackage: %v", err)
	}

	// Verify the zip slip path was sanitized and didn't escape.
	expectedPath := filepath.Join(dir, "..", "..", "..", "etc", "passwd")
	if _, err := os.Stat(expectedPath); err == nil {
		t.Error("zip slip file was written outside destination")
	}
}

func TestHasPackageJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	if !hasPackageJSON(dir) {
		t.Error("expected hasPackageJSON true")
	}
}

func TestHasPackageJSONMissing(t *testing.T) {
	dir := t.TempDir()
	if hasPackageJSON(dir) {
		t.Error("expected hasPackageJSON false")
	}
}

func TestHasRequirementsTXT(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask"), 0644)
	if !hasRequirementsTXT(dir) {
		t.Error("expected hasRequirementsTXT true")
	}
}

func TestHasPyprojectTOML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)
	if !hasPyprojectTOML(dir) {
		t.Error("expected hasPyprojectTOML true")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "bar", "baz"); got != "bar" {
		t.Errorf("expected 'bar', got %q", got)
	}
	if got := firstNonEmpty("", "", ""); got != "" {
		t.Errorf("expected '', got %q", got)
	}
	if got := firstNonEmpty("first", "second"); got != "first" {
		t.Errorf("expected 'first', got %q", got)
	}
}
