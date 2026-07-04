package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"Marcus/internal/model"

	_ "modernc.org/sqlite"
)

type Registry struct {
	db *sql.DB
}

func NewRegistry(dbPath string) (*Registry, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	r := &Registry{db: db}
	if err := r.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return r, nil
}

func (r *Registry) Close() error {
	return r.db.Close()
}

func (r *Registry) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tools (
		id           TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		display_name TEXT NOT NULL,
		description  TEXT,
		icon         TEXT,
		category     TEXT DEFAULT 'other',
		version      TEXT,
		source       TEXT NOT NULL,
		contribution TEXT NOT NULL,
		package_path TEXT,
		manifest     TEXT NOT NULL,
		entry_point  TEXT,
		enabled      INTEGER DEFAULT 1,
		last_seen    DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tool_runtime_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		tool_id     TEXT REFERENCES tools(id),
		pid         INTEGER,
		status      TEXT,
		started_at  DATETIME,
		stopped_at  DATETIME,
		exit_code   INTEGER,
		port        INTEGER,
		error_log   TEXT
	);
	`
	_, err := r.db.Exec(schema)
	return err
}

func rowToTool(row scanner) (model.ToolInfo, error) {
	var t model.ToolInfo
	var enabled int
	var lastSeen, createdAt string

	err := row.Scan(
		&t.ID, &t.Name, &t.DisplayName, &t.Description,
		&t.Icon, &t.Category, &t.Version, &t.Source,
		&t.Contribution, &t.PackagePath, &t.Manifest,
		&t.EntryPoint, &enabled, &lastSeen, &createdAt,
	)
	if err != nil {
		return t, err
	}
	t.Enabled = enabled != 0
	t.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)
	t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return t, nil
}

type scanner interface{ Scan(dest ...any) error }

func (r *Registry) ListTools(category string) ([]model.ToolInfo, error) {
	query := "SELECT * FROM tools WHERE enabled = 1"
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
	row := r.db.QueryRow("SELECT * FROM tools WHERE id = ?", id)
	t, err := rowToTool(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Registry) UpsertTool(t model.ToolInfo) error {
	manifestJSON := t.Manifest
	if manifestJSON == "" {
		data, _ := json.Marshal(map[string]string{"display_name": t.DisplayName})
		manifestJSON = string(data)
	}

	enabled := 0
	if t.Enabled {
		enabled = 1
	}

	_, err := r.db.Exec(`
		INSERT INTO tools (id, name, display_name, description, icon, category, version, source, contribution, package_path, manifest, entry_point, enabled, last_seen, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
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

func (r *Registry) AddLog(entry model.ProcessState) error {
	var started, stopped any
	if entry.StartedAt != nil {
		started = entry.StartedAt.Format("2006-01-02 15:04:05")
	}
	if entry.StoppedAt != nil {
		stopped = entry.StoppedAt.Format("2006-01-02 15:04:05")
	}
	_, err := r.db.Exec(`
		INSERT INTO tool_runtime_log (tool_id, pid, status, started_at, stopped_at, exit_code, port, error_log)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ToolID, entry.PID, entry.Status, started, stopped, entry.ExitCode, entry.Port, entry.ErrorLog)
	return err
}

func (r *Registry) GetLogs(toolID string, limit int) ([]model.ProcessState, error) {
	rows, err := r.db.Query(`
		SELECT tool_id, pid, status, COALESCE(port,0), COALESCE(exit_code,0), COALESCE(error_log,''), started_at, stopped_at
		FROM tool_runtime_log WHERE tool_id = ? ORDER BY id DESC LIMIT ?
	`, toolID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []model.ProcessState{}
	for rows.Next() {
		var l model.ProcessState
		var started, stopped *string
		if err := rows.Scan(&l.ToolID, &l.PID, &l.Status, &l.Port, &l.ExitCode, &l.ErrorLog, &started, &stopped); err != nil {
			return nil, err
		}
		if started != nil {
			t, _ := time.Parse("2006-01-02 15:04:05", *started)
			l.StartedAt = &t
		}
		if stopped != nil {
			t, _ := time.Parse("2006-01-02 15:04:05", *stopped)
			l.StoppedAt = &t
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
