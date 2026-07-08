package skill

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"Marcus/internal/model"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := migrateTestDB(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewStore(db)
}

func migrateTestDB(db *sql.DB) error {
	_, err := db.Exec(`
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
}

func TestSkillAddAndGetByID(t *testing.T) {
	store := setupTestStore(t)

	entry, err := store.Add(model.SkillEntry{
		Name:        "image-to-ascii",
		Description: "Convert images to ASCII art",
		Tags:        `["image","ascii","convert"]`,
		Content:     "Use marcus-img2ascii with input file and optional width/height",
	})
	if err != nil {
		t.Fatalf("add skill: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("skill id should not be empty")
	}

	got, err := store.GetByID(entry.ID)
	if err != nil {
		t.Fatalf("get skill by id: %v", err)
	}
	if got == nil {
		t.Fatal("skill not found")
	}
	if got.Name != "image-to-ascii" {
		t.Fatalf("name = %q, want %q", got.Name, "image-to-ascii")
	}
}

func TestSkillGetByName(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.Add(model.SkillEntry{
		Name:        "batch-rename",
		Description: "Batch rename files with pattern",
		Tags:        `["file","rename","batch"]`,
		Content:     "Use marcus-rename with pattern and directory",
	})
	if err != nil {
		t.Fatalf("add skill: %v", err)
	}

	got, err := store.GetByName("batch-rename")
	if err != nil {
		t.Fatalf("get skill by name: %v", err)
	}
	if got == nil {
		t.Fatal("skill not found by name")
	}
}

func TestSkillMatch(t *testing.T) {
	store := setupTestStore(t)

	_, _ = store.Add(model.SkillEntry{
		Name:        "image-to-ascii",
		Description: "Convert images to ASCII art",
		Tags:        `["image","ascii","convert"]`,
		Content:     "Use marcus-img2ascii with input file",
	})
	_, _ = store.Add(model.SkillEntry{
		Name:        "batch-rename",
		Description: "Batch rename files with pattern",
		Tags:        `["file","rename","batch"]`,
		Content:     "Use marcus-rename with pattern and directory",
	})

	// Match by keyword in name
	matched, err := store.Match([]string{"ascii"})
	if err != nil {
		t.Fatalf("match skills: %v", err)
	}
	if len(matched) != 1 {
		t.Fatalf("matched %d skills for 'ascii', want 1", len(matched))
	}

	// Match by keyword in tags — "file" appears in both: tag "file" + content "input file"
	matched, err = store.Match([]string{"file"})
	if err != nil {
		t.Fatalf("match skills: %v", err)
	}
	if len(matched) != 2 {
		t.Fatalf("matched %d skills for 'file', want 2", len(matched))
	}

	// No match
	matched, err = store.Match([]string{"nonexistent"})
	if err != nil {
		t.Fatalf("match skills: %v", err)
	}
	if len(matched) != 0 {
		t.Fatalf("matched %d skills, want 0", len(matched))
	}

	// Empty keywords
	matched, err = store.Match(nil)
	if err != nil {
		t.Fatalf("match skills with nil: %v", err)
	}
	if len(matched) != 0 {
		t.Fatalf("matched %d skills with nil, want 0", len(matched))
	}
}

func TestSkillIncrementUsage(t *testing.T) {
	store := setupTestStore(t)

	entry, err := store.Add(model.SkillEntry{
		Name:    "test-skill",
		Content: "test content",
	})
	if err != nil {
		t.Fatalf("add skill: %v", err)
	}

	if err := store.IncrementUsage(entry.ID); err != nil {
		t.Fatalf("increment usage: %v", err)
	}

	got, err := store.GetByID(entry.ID)
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if got.UseCount != 1 {
		t.Fatalf("use_count = %d, want 1", got.UseCount)
	}
}

func TestSkillDelete(t *testing.T) {
	store := setupTestStore(t)

	entry, err := store.Add(model.SkillEntry{
		Name:    "to-delete",
		Content: "will be deleted",
	})
	if err != nil {
		t.Fatalf("add skill: %v", err)
	}

	if err := store.Delete(entry.ID); err != nil {
		t.Fatalf("delete skill: %v", err)
	}

	got, err := store.GetByID(entry.ID)
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if got != nil {
		t.Fatal("skill should be deleted")
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
	{"convert this image to ascii", []string{"convert", "image", "ascii"}},
	{"帮我重命名文件", []string{"帮我重命名文件"}},
		{"the quick brown fox", []string{"quick", "brown", "fox"}},
		{"a", nil},
		{"", nil},
	}

	for _, tt := range tests {
		got := ExtractKeywords(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("ExtractKeywords(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ExtractKeywords(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestRenderSkillsPrompt(t *testing.T) {
	skills := []model.SkillEntry{
		{
			Name:        "image-to-ascii",
			Description: "Convert images to ASCII art",
			Content:     "Use marcus-img2ascii with input file",
		},
	}

	prompt := RenderSkillsPrompt(skills)
	if !strings.Contains(prompt, "SKILLS") {
		t.Fatal("prompt should contain SKILLS header")
	}
	if !strings.Contains(prompt, "image-to-ascii") {
		t.Fatal("prompt should contain skill name")
	}
	if !strings.Contains(prompt, "marcus-img2ascii") {
		t.Fatal("prompt should contain skill content")
	}

	// Empty skills
	empty := RenderSkillsPrompt(nil)
	if empty != "" {
		t.Fatal("empty skills should return empty string")
	}
	empty = RenderSkillsPrompt([]model.SkillEntry{})
	if empty != "" {
		t.Fatal("empty skills slice should return empty string")
	}
}
