package tools

import (
	"database/sql"

	"Marcus/internal/model"
)

// LogStore persists process execution logs independently of tool metadata.
type LogStore struct {
	db *sql.DB
}

func NewLogStore(db *sql.DB) *LogStore {
	return &LogStore{db: db}
}

func (s *LogStore) AddLog(entry model.ProcessState) error {
	_, err := s.db.Exec(`
		INSERT INTO tool_runtime_log (tool_id, pid, status, started_at, stopped_at, exit_code, port, error_log)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ToolID, entry.PID, entry.Status, entry.StartedAt, entry.StoppedAt, entry.ExitCode, entry.Port, entry.ErrorLog)
	return err
}

func (s *LogStore) GetLogs(toolID string, limit int) ([]model.ProcessState, error) {
	rows, err := s.db.Query(`
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
			l.StartedAt = *started
		}
		if stopped != nil {
			l.StoppedAt = *stopped
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
