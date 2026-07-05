package store

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"Marcus/internal/model"

	_ "modernc.org/sqlite"
)

func newTestClient(t *testing.T, indexURL string) *Client {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	c, err := NewClient(db, indexURL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestClientSync(t *testing.T) {
	idx := &model.StoreIndex{
		SchemaVersion: 1,
		Plugins: map[string]model.StorePlugin{
			"test-plugin": {
				ID:            "test-plugin",
				LatestVersion: "1.0.0",
				Versions: map[string]model.StoreVersion{
					"1.0.0": {
						PublishedAt:  "2024-01-01",
						DownloadURL:  "https://example.com/test-plugin-1.0.0.marcus-plugin",
						DisplayName:  "Test Plugin",
						Description:  "A test plugin",
						Categories:   []string{"dev"},
						Contribution: "terminal",
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(idx)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	// First sync.
	got, err := c.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(got.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(got.Plugins))
	}

	// Verify cache.
	plugins, err := c.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 cached plugin, got %d", len(plugins))
	}
	if plugins[0].ID != "test-plugin" {
		t.Errorf("expected ID 'test-plugin', got %q", plugins[0].ID)
	}
	if plugins[0].LatestVersion != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", plugins[0].LatestVersion)
	}
}

func TestClientSyncIncremental(t *testing.T) {
	idx1 := &model.StoreIndex{
		SchemaVersion: 1,
		Plugins: map[string]model.StorePlugin{
			"plugin-a": {
				ID: "plugin-a", LatestVersion: "1.0.0",
				Versions: map[string]model.StoreVersion{
					"1.0.0": {DisplayName: "Plugin A", DownloadURL: "https://example.com/a"},
				},
			},
			"plugin-b": {
				ID: "plugin-b", LatestVersion: "2.0.0",
				Versions: map[string]model.StoreVersion{
					"2.0.0": {DisplayName: "Plugin B", DownloadURL: "https://example.com/b"},
				},
			},
		},
	}

	idx2 := &model.StoreIndex{
		SchemaVersion: 1,
		Plugins: map[string]model.StorePlugin{
			"plugin-a": {
				ID: "plugin-a", LatestVersion: "1.1.0",
				Versions: map[string]model.StoreVersion{
					"1.1.0": {DisplayName: "Plugin A Updated", DownloadURL: "https://example.com/a-v2"},
				},
			},
			"plugin-c": {
				ID: "plugin-c", LatestVersion: "3.0.0",
				Versions: map[string]model.StoreVersion{
					"3.0.0": {DisplayName: "Plugin C", DownloadURL: "https://example.com/c"},
				},
			},
		},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			json.NewEncoder(w).Encode(idx1)
		} else {
			json.NewEncoder(w).Encode(idx2)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	// First sync: plugin-a and plugin-b.
	if _, err := c.Sync(); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	plugins, _ := c.ListPlugins()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins after first sync, got %d", len(plugins))
	}

	// Second sync: plugin-a updated, plugin-b removed, plugin-c added.
	if _, err := c.Sync(); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	plugins, _ = c.ListPlugins()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins after second sync, got %d", len(plugins))
	}

	// Verify plugin-a was updated.
	pa, err := c.GetCachedPlugin("plugin-a")
	if err != nil {
		t.Fatalf("GetCachedPlugin(plugin-a): %v", err)
	}
	if pa.LatestVersion != "1.1.0" {
		t.Errorf("expected plugin-a version '1.1.0', got %q", pa.LatestVersion)
	}

	// Verify plugin-b was removed.
	_, err = c.GetCachedPlugin("plugin-b")
	if err == nil {
		t.Error("expected plugin-b to be removed from cache")
	}

	// Verify plugin-c was added.
	pc, err := c.GetCachedPlugin("plugin-c")
	if err != nil {
		t.Fatalf("GetCachedPlugin(plugin-c): %v", err)
	}
	if pc.LatestVersion != "3.0.0" {
		t.Errorf("expected plugin-c version '3.0.0', got %q", pc.LatestVersion)
	}
}

func TestClientSearchPlugins(t *testing.T) {
	idx := &model.StoreIndex{
		SchemaVersion: 1,
		Plugins: map[string]model.StorePlugin{
			"image-tools": {
				ID: "image-tools", LatestVersion: "1.0.0",
				Versions: map[string]model.StoreVersion{
					"1.0.0": {DisplayName: "Image Tools", Description: "Tools for image processing"},
				},
			},
			"text-tools": {
				ID: "text-tools", LatestVersion: "1.0.0",
				Versions: map[string]model.StoreVersion{
					"1.0.0": {DisplayName: "Text Tools", Description: "Tools for text processing"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(idx)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.Sync()

	// Search by name.
	results, err := c.SearchPlugins("image")
	if err != nil {
		t.Fatalf("SearchPlugins: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'image', got %d", len(results))
	}
	if results[0].ID != "image-tools" {
		t.Errorf("expected 'image-tools', got %q", results[0].ID)
	}

	// Search by description.
	results, err = c.SearchPlugins("text")
	if err != nil {
		t.Fatalf("SearchPlugins: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'text', got %d", len(results))
	}

	// Empty search returns all.
	results, err = c.SearchPlugins("")
	if err != nil {
		t.Fatalf("SearchPlugins(''): %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for empty query, got %d", len(results))
	}
}

func TestClientPendingInstalls(t *testing.T) {
	c := newTestClient(t, "https://example.com/unused")

	// Record pending install.
	if err := c.RecordPendingInstall("test-plugin", "1.0.0"); err != nil {
		t.Fatalf("RecordPendingInstall: %v", err)
	}

	// List pending.
	pending, err := c.ListPendingInstalls()
	if err != nil {
		t.Fatalf("ListPendingInstalls: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].PluginID != "test-plugin" {
		t.Errorf("expected 'test-plugin', got %q", pending[0].PluginID)
	}

	// Clear pending.
	if err := c.ClearPendingInstall("test-plugin"); err != nil {
		t.Fatalf("ClearPendingInstall: %v", err)
	}

	pending, _ = c.ListPendingInstalls()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after clear, got %d", len(pending))
	}
}
