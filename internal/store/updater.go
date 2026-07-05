package store

import (
	"database/sql"

	"Marcus/internal/model"
)

type Updater struct {
	db *sql.DB
}

func NewUpdater(db *sql.DB) *Updater {
	return &Updater{db: db}
}

// CheckUpdates compares installed plugin versions against the cache.
func (u *Updater) CheckUpdates() ([]model.UpdateCheckResult, error) {
	rows, err := u.db.Query(`
		SELECT i.plugin_id, i.version, c.latest_version
		FROM store_installed i
		JOIN store_cache c ON c.id = i.plugin_id
		WHERE i.version != c.latest_version
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.UpdateCheckResult
	for rows.Next() {
		var r model.UpdateCheckResult
		if err := rows.Scan(&r.PluginID, &r.CurrentVersion, &r.LatestVersion); err != nil {
			return nil, err
		}
		r.UpdateAvailable = true
		results = append(results, r)
	}
	return results, rows.Err()
}

// HasUpdates returns true if any installed plugin has an available update.
func (u *Updater) HasUpdates() (bool, error) {
	var count int
	err := u.db.QueryRow(`
		SELECT COUNT(*) FROM store_installed i
		JOIN store_cache c ON c.id = i.plugin_id
		WHERE i.version != c.latest_version
	`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
