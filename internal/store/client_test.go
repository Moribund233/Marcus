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

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "store.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestClientSyncAndList(t *testing.T) {
	db := openTestDB(t)

	idx := model.StoreIndex{
		SchemaVersion: 1,
		UpdatedAt:     "2024-01-01T00:00:00Z",
		Plugins: map[string]model.StorePlugin{
			"plugin-a": {
				ID:            "plugin-a",
				LatestVersion: "1.0.0",
				Versions: map[string]model.StoreVersion{
					"1.0.0": {
						PublishedAt:      "2024-01-01T00:00:00Z",
						DownloadURL:      "https://example.com/a.zip",
						DisplayName:      "Plugin A",
						Description:      "Description A",
						Categories:       []string{"dev"},
						Contribution:     "terminal",
						MinMarcusVersion: "1.0.0",
					},
				},
			},
			"plugin-b": {
				ID:            "plugin-b",
				LatestVersion: "2.0.0",
				Versions: map[string]model.StoreVersion{
					"2.0.0": {
						PublishedAt:      "2024-02-01T00:00:00Z",
						DownloadURL:      "https://example.com/b.zip",
						DisplayName:      "Plugin B",
						Description:      "Description B",
						Categories:       []string{"image"},
						Contribution:     "web",
						MinMarcusVersion: "1.0.0",
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(idx)
	}))
	defer server.Close()

	c, err := NewClient(db, server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Sync.
	synced, err := c.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if synced.SchemaVersion != 1 {
		t.Errorf("expected schema version 1, got %d", synced.SchemaVersion)
	}

	// ListPlugins.
	plugins, err := c.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	pluginMap := make(map[string]model.StorePlugin)
	for _, p := range plugins {
		pluginMap[p.ID] = p
	}

	if a, ok := pluginMap["plugin-a"]; !ok {
		t.Error("plugin-a not found")
	} else if a.LatestVersion != "1.0.0" {
		t.Errorf("plugin-a latest version expected 1.0.0, got %s", a.LatestVersion)
	}

	// Search.
	results, err := c.SearchPlugins("Plugin B")
	if err != nil {
		t.Fatalf("SearchPlugins: %v", err)
	}
	if len(results) != 1 || results[0].ID != "plugin-b" {
		t.Errorf("expected 1 result (plugin-b), got %d", len(results))
	}
}

func TestClientSyncServerError(t *testing.T) {
	db := openTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c, err := NewClient(db, server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = c.Sync()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestClientSearchEmpty(t *testing.T) {
	db := openTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.StoreIndex{
			SchemaVersion: 1,
			UpdatedAt:     "2024-01-01T00:00:00Z",
			Plugins:       map[string]model.StorePlugin{},
		})
	}))
	defer server.Close()

	c, err := NewClient(db, server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.Sync()

	all, err := c.SearchPlugins("")
	if err != nil {
		t.Fatalf("SearchPlugins(''): %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(all))
	}
}

func TestGetCachedPlugin(t *testing.T) {
	db := openTestDB(t)

	idx := model.StoreIndex{
		SchemaVersion: 1,
		Plugins: map[string]model.StorePlugin{
			"my-plugin": {
				ID:            "my-plugin",
				LatestVersion: "1.0.0",
				Versions: map[string]model.StoreVersion{
					"1.0.0": {
						DownloadURL:      "https://example.com/p.zip",
						DisplayName:      "My Plugin",
						PublishedAt:      "2024-01-01T00:00:00Z",
						Categories:       []string{"dev"},
						Contribution:     "terminal",
						MinMarcusVersion: "1.0.0",
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(idx)
	}))
	defer server.Close()

	c, err := NewClient(db, server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.Sync()

	plugin, err := c.GetCachedPlugin("my-plugin")
	if err != nil {
		t.Fatalf("GetCachedPlugin: %v", err)
	}
	if plugin.LatestVersion != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", plugin.LatestVersion)
	}

	_, err = c.GetCachedPlugin("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestContainsFold(t *testing.T) {
	tests := []struct {
		s, substr string
		expected  bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "HELLO", true},
		{"Hello World", "xyz", false},
		{"Golang", "go", true},
		{"", "", true},
		{"abc", "", true},
	}
	for _, tc := range tests {
		got := containsFold(tc.s, tc.substr)
		if got != tc.expected {
			t.Errorf("containsFold(%q, %q) = %v, want %v", tc.s, tc.substr, got, tc.expected)
		}
	}
}

func TestToUpper(t *testing.T) {
	if got := toUpper("hello"); got != "HELLO" {
		t.Errorf("expected 'HELLO', got %q", got)
	}
	if got := toUpper(""); got != "" {
		t.Errorf("expected '', got %q", got)
	}
	if got := toUpper("ALREADY"); got != "ALREADY" {
		t.Errorf("expected 'ALREADY', got %q", got)
	}
}
