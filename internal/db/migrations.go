package db

import (
	"database/sql"
	"fmt"
)

// All 返回所有 schema 迁移定义。
// 每条迁移按时间戳排序，新迁移追加到末尾。
//
// 规范：
//   - ID 格式 "20060102_description"，便于排序
//   - 已发布的迁移不可修改（只能追加新的）
//   - 迁移函数必须是幂等的（CREATE IF NOT EXISTS 等）
func All() []Migration {
	return []Migration{
		{
			ID:   "20240401_initial_tools",
			Desc: "Create tools table with initial columns",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
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
						last_used    DATETIME,
						created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
					)
				`)
				return err
			},
		},
		{
			ID:   "20240401_initial_conversations",
			Desc: "Create conversations and messages tables",
			Up: func(tx *sql.Tx) error {
				if _, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS conversations (
						id          TEXT PRIMARY KEY,
						title       TEXT,
						created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
						updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
					)
				`); err != nil {
					return err
				}
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS messages (
						id              TEXT PRIMARY KEY,
						conversation_id TEXT NOT NULL,
						role            TEXT NOT NULL,
						content         TEXT,
						tool_calls      TEXT,
						tool_results    TEXT,
						created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
						FOREIGN KEY (conversation_id) REFERENCES conversations(id)
					)
				`)
				return err
			},
		},
		{
			ID:   "20240401_initial_memories",
			Desc: "Create memories table",
			Up: func(tx *sql.Tx) error {
				if _, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS memories (
						id          TEXT PRIMARY KEY,
						scope       TEXT NOT NULL DEFAULT 'global',
						key         TEXT NOT NULL UNIQUE,
						content     TEXT NOT NULL,
						source      TEXT NOT NULL DEFAULT 'agent',
						created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
						updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
					)
				`); err != nil {
					return err
				}
				if _, err := tx.Exec(`
					CREATE INDEX IF NOT EXISTS idx_memories_scope ON memories(scope)
				`); err != nil {
					return err
				}
				_, err := tx.Exec(`
					CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(key)
				`)
				return err
			},
		},
		{
			ID:   "20240401_initial_llm_config",
			Desc: "Create llm_config table",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS llm_config (
						provider TEXT PRIMARY KEY,
						api_key  TEXT,
						model    TEXT,
						base_url TEXT
					)
				`)
				return err
			},
		},
		{
			ID:   "20240401_initial_store_cache",
			Desc: "Create store_cache, store_installed, pending_store_install tables",
			Up: func(tx *sql.Tx) error {
				if _, err := tx.Exec(`
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
					)
				`); err != nil {
					return err
				}
				if _, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS store_installed (
						plugin_id    TEXT PRIMARY KEY,
						version      TEXT NOT NULL,
						installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						FOREIGN KEY (plugin_id) REFERENCES store_cache(id)
					)
				`); err != nil {
					return err
				}
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS pending_store_install (
						plugin_id   TEXT PRIMARY KEY,
						version     TEXT NOT NULL,
						created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
					)
				`)
				return err
			},
		},
		{
			ID:   "20240401_initial_runtime_log",
			Desc: "Create tool_runtime_log table",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS tool_runtime_log (
						id          INTEGER PRIMARY KEY AUTOINCREMENT,
						tool_id     TEXT NOT NULL,
						pid         INTEGER,
						status      TEXT,
						started_at  DATETIME,
						stopped_at  DATETIME,
						exit_code   INTEGER,
						port        INTEGER,
						error_log   TEXT
					)
				`)
				return err
			},
		},
		{
			ID:   "20240402_add_message_idx",
			Desc: "Create index on messages(conversation_id, created_at)",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE INDEX IF NOT EXISTS idx_messages_conversation
					ON messages(conversation_id, created_at)
				`)
				return err
			},
		},
		{
			ID:   "20240707_update_llm_config",
			Desc: "Add enabled and updated_at columns to llm_config",
			Up: func(tx *sql.Tx) error {
				columns := []struct {
					name string
					def  string
				}{
					{"enabled", "INTEGER DEFAULT 1"},
					{"updated_at", "DATETIME DEFAULT CURRENT_TIMESTAMP"},
				}
				for _, col := range columns {
					var count int
					err := tx.QueryRow(
						"SELECT COUNT(*) FROM pragma_table_info('llm_config') WHERE name = ?",
						col.name,
					).Scan(&count)
					if err != nil {
						return fmt.Errorf("check column %s: %w", col.name, err)
					}
					if count == 0 {
						if _, err := tx.Exec(fmt.Sprintf(
							"ALTER TABLE llm_config ADD COLUMN %s %s",
							col.name, col.def,
						)); err != nil {
							return fmt.Errorf("add column %s: %w", col.name, err)
						}
					}
				}
				return nil
			},
		},
		{
			ID:   "20240707_create_skills",
			Desc: "Create skills table for reusable tool workflows",
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
					CREATE TABLE IF NOT EXISTS skills (
						id          TEXT PRIMARY KEY,
						name        TEXT NOT NULL,
						description TEXT,
						tags        TEXT,
						content     TEXT NOT NULL,
						use_count   INTEGER DEFAULT 0,
						last_used   DATETIME,
						created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
						updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
					)
				`)
				return err
			},
		},
	}
}
