package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"Marcus/internal/model"

	_ "modernc.org/sqlite"
)

type Client struct {
	db       *sql.DB
	indexURL string
	hc       *http.Client
}

func NewClient(db *sql.DB, indexURL ...string) (*Client, error) {
	url := "https://raw.githubusercontent.com/Moribund233/Marcus-plugins/master/index.json"
	if len(indexURL) > 0 && indexURL[0] != "" {
		url = indexURL[0]
	}
	c := &Client{
		db:       db,
		indexURL: url,
		hc:       &http.Client{Timeout: 30 * time.Second},
	}
	if err := c.migrate(); err != nil {
		return nil, fmt.Errorf("store migrate: %w", err)
	}
	return c, nil
}

func (c *Client) migrate() error {
	_, err := c.db.Exec(`
		CREATE TABLE IF NOT EXISTS store_cache (
			id              TEXT PRIMARY KEY,
			display_name    TEXT NOT NULL,
			description     TEXT DEFAULT '',
			categories      TEXT DEFAULT '[]',
			latest_version  TEXT NOT NULL,
			versions        TEXT NOT NULL,
			deprecated      INTEGER DEFAULT 0,
			deprecation_msg TEXT DEFAULT '',
			synced_at       DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS store_installed (
			plugin_id    TEXT PRIMARY KEY,
			version      TEXT NOT NULL,
			installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (plugin_id) REFERENCES store_cache(id)
		);
		CREATE TABLE IF NOT EXISTS pending_store_install (
			plugin_id   TEXT PRIMARY KEY,
			version     TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

// RecordPendingInstall records a plugin whose download+install succeeded but
// whose scanner discovery failed. The entry is retried on the next scan.
func (c *Client) RecordPendingInstall(pluginID, version string) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO pending_store_install (plugin_id, version) VALUES (?, ?)`,
		pluginID, version,
	)
	return err
}

// ClearPendingInstall removes a pending entry after a successful scan.
func (c *Client) ClearPendingInstall(pluginID string) error {
	_, err := c.db.Exec(`DELETE FROM pending_store_install WHERE plugin_id = ?`, pluginID)
	return err
}

// ListPendingInstalls returns all pending installs that should be retried.
func (c *Client) ListPendingInstalls() ([]struct{ PluginID, Version string }, error) {
	rows, err := c.db.Query(`SELECT plugin_id, version FROM pending_store_install`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct{ PluginID, Version string }
	for rows.Next() {
		var r struct{ PluginID, Version string }
		if err := rows.Scan(&r.PluginID, &r.Version); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Sync fetches the remote index and updates the local cache.
func (c *Client) Sync() (*model.StoreIndex, error) {
	req, err := http.NewRequest(http.MethodGet, c.indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index returned status %d", resp.StatusCode)
	}

	var idx model.StoreIndex
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}

	if err := c.saveToCache(&idx); err != nil {
		return nil, fmt.Errorf("save cache: %w", err)
	}

	return &idx, nil
}

func (c *Client) saveToCache(idx *model.StoreIndex) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM store_cache"); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO store_cache (id, display_name, description, categories, latest_version, versions, deprecated, deprecation_msg, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for id, plugin := range idx.Plugins {
		versionsJSON, _ := json.Marshal(plugin.Versions)

		latestVer, ok := plugin.Versions[plugin.LatestVersion]
		displayName := id
		description := ""
		categories := "[]"
		if ok {
			if latestVer.DisplayName != "" {
				displayName = latestVer.DisplayName
			}
			description = latestVer.Description
			if cats, err := json.Marshal(latestVer.Categories); err == nil {
				categories = string(cats)
			}
		}

		deprecated := 0
		if plugin.Deprecated {
			deprecated = 1
		}

		if _, err := stmt.Exec(
			id, displayName, description, categories,
			plugin.LatestVersion, string(versionsJSON),
			deprecated, plugin.DeprecationMsg,
		); err != nil {
			return fmt.Errorf("insert cache %s: %w", id, err)
		}
	}

	return tx.Commit()
}

// ListPlugins returns all cached plugins from the local DB.
func (c *Client) ListPlugins() ([]model.StorePlugin, error) {
	rows, err := c.db.Query(`
		SELECT c.id, c.display_name, c.description, c.categories,
		       c.latest_version, c.versions, c.deprecated, c.deprecation_msg,
		       COALESCE(i.version, '') as installed_version
		FROM store_cache c
		LEFT JOIN store_installed i ON i.plugin_id = c.id
		ORDER BY c.display_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []model.StorePlugin
	for rows.Next() {
		var p model.StorePlugin
		var displayName, description, categoriesStr, versionsStr, installedVersion string
		var deprecated int

		if err := rows.Scan(
			&p.ID, &displayName, &description, &categoriesStr,
			&p.LatestVersion, &versionsStr, &deprecated, &p.DeprecationMsg,
			&installedVersion,
		); err != nil {
			return nil, err
		}

		p.Deprecated = deprecated != 0
		p.InstalledVersion = installedVersion
		p.UpdateAvailable = installedVersion != "" && installedVersion != p.LatestVersion

		if err := json.Unmarshal([]byte(versionsStr), &p.Versions); err != nil {
			p.Versions = nil
		}

		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}

// SearchPlugins filters cached plugins by query text.
func (c *Client) SearchPlugins(query string) ([]model.StorePlugin, error) {
	all, err := c.ListPlugins()
	if err != nil {
		return nil, err
	}
	if query == "" {
		return all, nil
	}

	var filtered []model.StorePlugin
	for _, p := range all {
		latestVer, ok := p.Versions[p.LatestVersion]
		if !ok {
			continue
		}
		if containsFold(p.ID, query) || containsFold(latestVer.DisplayName, query) || containsFold(latestVer.Description, query) {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func containsFold(s, substr string) bool {
	s = toUpper(s)
	substr = toUpper(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			b[i] = s[i] - 32
		} else {
			b[i] = s[i]
		}
	}
	return string(b)
}

// GetCachedPlugin returns a single plugin from cache by ID.
func (c *Client) GetCachedPlugin(id string) (*model.StorePlugin, error) {
	plugins, err := c.ListPlugins()
	if err != nil {
		return nil, err
	}
	for _, p := range plugins {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("plugin %s not found in cache", id)
}

func (c *Client) Close() error {
	return c.db.Close()
}
