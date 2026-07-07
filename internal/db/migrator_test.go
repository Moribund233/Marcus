package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrator_RunFresh(t *testing.T) {
	db := openTestDB(t)
	m := NewMigrator(db)
	m.Register(All()...)

	if err := m.Run(); err != nil {
		t.Fatalf("Run on fresh DB: %v", err)
	}

	// All migrations should now be applied.
	ids, err := getAppliedIDs(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(All()) {
		t.Errorf("applied %d, want %d", len(ids), len(All()))
	}
}

func TestMigrator_RunIdempotent(t *testing.T) {
	db := openTestDB(t)
	m := NewMigrator(db)
	m.Register(All()...)

	if err := m.Run(); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Second run should succeed and apply nothing new.
	m2 := NewMigrator(db)
	m2.Register(All()...)
	if err := m2.Run(); err != nil {
		t.Fatalf("second run: %v", err)
	}

	ids, err := getAppliedIDs(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(All()) {
		t.Errorf("applied %d (second run added dupes), want %d", len(ids), len(All()))
	}
}

func TestMigrator_Pending(t *testing.T) {
	db := openTestDB(t)
	m := NewMigrator(db)
	m.Register(All()...)

	pending, err := m.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != len(All()) {
		t.Errorf("pending on fresh DB: %d, want %d", len(pending), len(All()))
	}

	m.Run()

	pending, err = m.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("pending after run: %d, want 0", len(pending))
	}
}

func TestMigrator_MigrationFails(t *testing.T) {
	db := openTestDB(t)
	m := NewMigrator(db)

	bad := Migration{ID: "9999_bad", Desc: "always fails", Up: func(tx *sql.Tx) error {
		_, err := tx.Exec("CREATE TABLE this_will_fail (id INTEGER PRIMARY KEY)")
		if err != nil {
			return err
		}
		_, err = tx.Exec("CREATE TABLE this_will_fail (id INTEGER PRIMARY KEY)")
		return err
	}}
	m.Register(bad)

	err := m.Run()
	if err == nil {
		t.Fatal("expected error for bad migration")
	}

	// The failed migration's ID should NOT be recorded.
	ids, err := getAppliedIDs(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if id == bad.ID {
			t.Fatal("failed migration was recorded in schema_migrations")
		}
	}
}

// TestMigrator_LLMConfigAndSkillsSchema 验证 llm_config 扩展列与 skills 表已创建。
func TestMigrator_LLMConfigAndSkillsSchema(t *testing.T) {
	db := openTestDB(t)
	m := NewMigrator(db)
	m.Register(All()...)

	if err := m.Run(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	columns := map[string]bool{}
	rows, err := db.Query("SELECT name FROM pragma_table_info('llm_config')")
	if err != nil {
		t.Fatalf("query llm_config columns: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}

	for _, col := range []string{"provider", "api_key", "model", "base_url", "enabled", "updated_at"} {
		if !columns[col] {
			t.Errorf("llm_config missing column: %s", col)
		}
	}

	var tableCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='skills'").Scan(&tableCount); err != nil {
		t.Fatalf("check skills table: %v", err)
	}
	if tableCount != 1 {
		t.Errorf("skills table not created")
	}
}

func getAppliedIDs(db *sql.DB) ([]string, error) {
	// schema_migrations might not exist yet.
	rows, err := db.Query("SELECT id FROM schema_migrations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
