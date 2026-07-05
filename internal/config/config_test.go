package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Theme != "dark" {
		t.Errorf("expected dark theme, got %q", cfg.Theme)
	}
	if cfg.Language != "zh-CN" {
		t.Errorf("expected zh-CN language, got %q", cfg.Language)
	}
	if cfg.AutoScan != true {
		t.Errorf("expected auto scan enabled")
	}
	if cfg.TerminateOnExit != true {
		t.Errorf("expected terminate on exit enabled")
	}
	if cfg.StoreIndexURL == "" {
		t.Error("expected non-empty store index URL")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Default()
	cfg.Theme = "light"
	cfg.Language = "en-US"
	cfg.DefaultTimeout = 60

	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Theme != "light" {
		t.Errorf("expected light theme after reload, got %q", loaded.Theme)
	}
	if loaded.Language != "en-US" {
		t.Errorf("expected en-US language after reload, got %q", loaded.Language)
	}
	if loaded.DefaultTimeout != 60 {
		t.Errorf("expected timeout 60 after reload, got %d", loaded.DefaultTimeout)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load non-existent should not error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected default config, got nil")
	}
}

func TestLoadCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.json")
	os.WriteFile(path, []byte("{not json}"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for corrupted JSON")
	}
}

func TestRoundTripPreservesUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Default()
	cfg.Theme = "marcus"
	cfg.StoreIndexURL = "https://example.com/custom-index.json"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Theme != "marcus" {
		t.Errorf("expected marcus theme, got %q", loaded.Theme)
	}
	if loaded.StoreIndexURL != "https://example.com/custom-index.json" {
		t.Errorf("expected custom store URL, got %q", loaded.StoreIndexURL)
	}
}
