package db

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration 定义一次不可变的 schema 变更。
// ID 使用时间戳前缀避免多人开发冲突："20240501_short_description"
type Migration struct {
	ID   string
	Desc string
	Up   func(*sql.Tx) error
}

// Migrator 管理所有 schema 迁移。
// 每条迁移在自己的事务中运行，原子提交或回滚。
type Migrator struct {
	db         *sql.DB
	migrations []Migration
}

func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Register(migs ...Migration) {
	m.migrations = append(m.migrations, migs...)
}

func (m *Migrator) Run() error {
	if err := m.ensureTracker(); err != nil {
		return err
	}
	applied, err := m.fetchApplied()
	if err != nil {
		return err
	}

	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].ID < m.migrations[j].ID
	})

	for _, mig := range m.migrations {
		if applied[mig.ID] {
			continue
		}
		if err := m.runOne(mig); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) ensureTracker() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id         TEXT PRIMARY KEY,
			desc       TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	return err
}

func (m *Migrator) fetchApplied() (map[string]bool, error) {
	rows, err := m.db.Query("SELECT id FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		applied[id] = true
	}
	return applied, rows.Err()
}

// Pending 返回尚未应用的迁移列表。
func (m *Migrator) Pending() ([]Migration, error) {
	if err := m.ensureTracker(); err != nil {
		return nil, err
	}
	applied, err := m.fetchApplied()
	if err != nil {
		return nil, err
	}
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].ID < m.migrations[j].ID
	})
	var pending []Migration
	for _, mig := range m.migrations {
		if !applied[mig.ID] {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

func (m *Migrator) runOne(mig Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := mig.Up(tx); err != nil {
		return fmt.Errorf("migration %s (%s): %w", mig.ID, mig.Desc, err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (id, desc, applied_at) VALUES (?, ?, ?)",
		mig.ID, mig.Desc, now,
	); err != nil {
		return fmt.Errorf("record %s: %w", mig.ID, err)
	}
	return tx.Commit()
}
