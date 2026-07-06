package tools

import (
	"database/sql"
	"encoding/json"

	"Marcus/internal/model"
)

// Compile-time check: *Registry implements RegistryReader.
var _ RegistryReader = (*Registry)(nil)

// RegistryReader is the subset of Registry methods needed by Uninstaller.
type RegistryReader interface {
	GetTool(id string) (*model.ToolInfo, error)
	DeleteTool(id string) error
}

type Registry struct {
	db *sql.DB
}

func NewRegistry(db *sql.DB) *Registry {
	return &Registry{db: db}
}

func (r *Registry) Close() error {
	return r.db.Close()
}

func rowToTool(row scanner) (model.ToolInfo, error) {
	var t model.ToolInfo
	var enabled int
	var lastSeen, createdAt string
	var lastUsed sql.NullString

	err := row.Scan(
		&t.ID, &t.Name, &t.DisplayName, &t.Description,
		&t.Icon, &t.Category, &t.Version, &t.Source,
		&t.Contribution, &t.PackagePath, &t.Manifest,
		&t.EntryPoint, &enabled, &lastSeen, &lastUsed, &createdAt,
	)
	if err != nil {
		return t, err
	}
	t.Enabled = enabled != 0
	t.LastSeen = lastSeen
	t.LastUsed = lastUsed.String
	t.CreatedAt = createdAt
	return t, nil
}

type scanner interface{ Scan(dest ...any) error }

// toolColumns is the single source of truth for column order.
// Must match the scan order in rowToTool.
var toolColumns = []string{
	"id", "name", "display_name", "description", "icon", "category",
	"version", "source", "contribution", "package_path", "manifest",
	"entry_point", "enabled", "last_seen", "last_used", "created_at",
}

func cols() string {
	s := ""
	for i, c := range toolColumns {
		if i > 0 {
			s += ", "
		}
		s += c
	}
	return s
}

func (r *Registry) ListTools(category string) ([]model.ToolInfo, error) {
	query := "SELECT " + cols() + " FROM tools WHERE enabled = 1"
	args := []any{}
	if category != "" && category != "all" {
		query += " AND category = ?"
		args = append(args, category)
	}
	query += " ORDER BY display_name ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tools := []model.ToolInfo{}
	for rows.Next() {
		t, err := rowToTool(rows)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func (r *Registry) GetTool(id string) (*model.ToolInfo, error) {
	row := r.db.QueryRow("SELECT "+cols()+" FROM tools WHERE id = ?", id)
	t, err := rowToTool(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Registry) UpsertTool(t model.ToolInfo) error {
	return upsertTool(r.db, t)
}

func upsertTool(execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, t model.ToolInfo) error {
	manifestJSON := t.Manifest
	if manifestJSON == "" {
		data, _ := json.Marshal(map[string]string{"display_name": t.DisplayName})
		manifestJSON = string(data)
	}

	enabled := 0
	if t.Enabled {
		enabled = 1
	}

	_, err := execer.Exec(`
		INSERT INTO tools (`+cols()+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, NULL, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			display_name=excluded.display_name,
			description=excluded.description,
			icon=excluded.icon,
			category=excluded.category,
			version=excluded.version,
			source=excluded.source,
			contribution=excluded.contribution,
			package_path=excluded.package_path,
			manifest=excluded.manifest,
			entry_point=excluded.entry_point,
			enabled=excluded.enabled,
			last_seen=CURRENT_TIMESTAMP
	`,
		t.ID, t.Name, t.DisplayName, t.Description,
		t.Icon, t.Category, t.Version, t.Source,
		t.Contribution, t.PackagePath, manifestJSON,
		t.EntryPoint, enabled,
	)
	return err
}

func (r *Registry) DeleteTool(id string) error {
	_, err := r.db.Exec("DELETE FROM tools WHERE id = ?", id)
	return err
}

func (r *Registry) UpdateLastUsed(toolID string) error {
	_, err := r.db.Exec(
		"UPDATE tools SET last_used = CURRENT_TIMESTAMP WHERE id = ?", toolID,
	)
	return err
}

func (r *Registry) ListRecentTools(limit int) ([]model.ToolInfo, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Query(
		"SELECT "+cols()+" FROM tools WHERE enabled = 1 AND last_used IS NOT NULL ORDER BY last_used DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tools := []model.ToolInfo{}
	for rows.Next() {
		t, err := rowToTool(rows)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}


