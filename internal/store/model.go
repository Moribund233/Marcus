package store

import (
	"time"
)

// CacheRow mirrors the store_cache SQLite table.
type CacheRow struct {
	ID             string    `json:"id"`
	DisplayName    string    `json:"display_name"`
	Description    string    `json:"description"`
	Categories     string    `json:"categories"`
	LatestVersion  string    `json:"latest_version"`
	Versions       string    `json:"versions"`
	Deprecated     bool      `json:"deprecated"`
	DeprecationMsg string    `json:"deprecation_message"`
	SyncedAt       time.Time `json:"synced_at"`
}
