// Package skill 实现了 Marcus LLM Agent 的 L3 技能记忆存储与管理。
//
// L3 技能记忆存储可复用的工具工作流，按关键词匹配后注入系统 Prompt，
// 帮助 Agent 快速定位适合特定任务的工具组合。
package skill

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"Marcus/internal/model"
)

// Store 提供技能记忆的持久化与关键词匹配能力。
type Store struct {
	db *sql.DB
}

// NewStore 创建一个新的技能记忆存储实例。
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Add 添加一条技能；若同名已存在则更新。
func (s *Store) Add(entry model.SkillEntry) (*model.SkillEntry, error) {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	entry.CreatedAt = now
	entry.UpdatedAt = now

	// 确保 tags 为合法 JSON
	if entry.Tags != "" {
		var dummy []string
		if err := json.Unmarshal([]byte(entry.Tags), &dummy); err != nil {
			return nil, fmt.Errorf("tags must be a JSON array: %w", err)
		}
	}

	_, err := s.db.Exec(
		`INSERT INTO skills (id, name, description, tags, content, use_count, last_used, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, NULL, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			 name=excluded.name,
			 description=excluded.description,
			 tags=excluded.tags,
			 content=excluded.content,
			 updated_at=excluded.updated_at`,
		entry.ID, entry.Name, entry.Description, entry.Tags, entry.Content,
		entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert skill: %w", err)
	}

	return s.GetByID(entry.ID)
}

// GetByID 通过 ID 查询技能。
func (s *Store) GetByID(id string) (*model.SkillEntry, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, tags, content, use_count, last_used, created_at, updated_at
		 FROM skills WHERE id = ?`, id)

	var entry model.SkillEntry
	var lastUsed sql.NullString
	err := row.Scan(&entry.ID, &entry.Name, &entry.Description, &entry.Tags, &entry.Content,
		&entry.UseCount, &lastUsed, &entry.CreatedAt, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query skill by id: %w", err)
	}
	if lastUsed.Valid {
		entry.LastUsed = lastUsed.String
	}
	return &entry, nil
}

// GetByName 通过 name 查询技能（唯一约束由外部保证）。
func (s *Store) GetByName(name string) (*model.SkillEntry, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, tags, content, use_count, last_used, created_at, updated_at
		 FROM skills WHERE name = ?`, name)

	var entry model.SkillEntry
	var lastUsed sql.NullString
	err := row.Scan(&entry.ID, &entry.Name, &entry.Description, &entry.Tags, &entry.Content,
		&entry.UseCount, &lastUsed, &entry.CreatedAt, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query skill by name: %w", err)
	}
	if lastUsed.Valid {
		entry.LastUsed = lastUsed.String
	}
	return &entry, nil
}

// List 返回所有技能，按 use_count DESC, updated_at DESC 排序。
func (s *Store) List() ([]model.SkillEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, tags, content, use_count, last_used, created_at, updated_at
		 FROM skills ORDER BY use_count DESC, updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	return scanSkillRows(rows)
}

// Match 根据关键词列表匹配技能。
// 匹配规则：关键词出现在 name / description / tags / content 任一字段即视为匹配。
// 返回匹配的技能列表，按匹配字段数量和 use_count 综合排序。
func (s *Store) Match(keywords []string) ([]model.SkillEntry, error) {
	if len(keywords) == 0 {
		return nil, nil
	}

	// 去重关键词
	seen := map[string]bool{}
	unique := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" || len(kw) < 2 {
			continue
		}
		lower := strings.ToLower(kw)
		if !seen[lower] {
			seen[lower] = true
			unique = append(unique, kw)
		}
	}

	if len(unique) == 0 {
		return nil, nil
	}

	// 构建 WHERE 子句：对每个关键词，检查四个字段
	var conditions []string
	var args []interface{}
	for _, kw := range unique {
		pattern := "%" + kw + "%"
		conditions = append(conditions,
			`(LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(tags) LIKE ? OR LOWER(content) LIKE ?)`)
		args = append(args, pattern, pattern, pattern, pattern)
	}

	query := fmt.Sprintf(
		`SELECT id, name, description, tags, content, use_count, last_used, created_at, updated_at
		 FROM skills WHERE %s
		 ORDER BY use_count DESC, updated_at DESC`,
		strings.Join(conditions, " OR "))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("match skills: %w", err)
	}
	defer rows.Close()

	// 去重（同一技能可能匹配多个关键词）
	entries, err := scanSkillRows(rows)
	if err != nil {
		return nil, err
	}

	dedup := map[string]bool{}
	result := make([]model.SkillEntry, 0, len(entries))
	for _, e := range entries {
		if !dedup[e.ID] {
			dedup[e.ID] = true
			result = append(result, e)
		}
	}
	return result, nil
}

// IncrementUsage 递增技能使用计数并更新 last_used。
func (s *Store) IncrementUsage(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE skills SET use_count = use_count + 1, last_used = ? WHERE id = ?`,
		now, id)
	if err != nil {
		return fmt.Errorf("increment skill usage: %w", err)
	}
	return nil
}

// Delete 删除一条技能。
func (s *Store) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM skills WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	return nil
}

// ExtractKeywords 从文本中提取关键词用于技能匹配。
// 按空格/标点分割，过滤短词（<2字符）和常见停用词。
func ExtractKeywords(text string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "and": true, "but": true,
		"or": true, "nor": true, "not": true, "so": true, "yet": true,
		"both": true, "either": true, "neither": true, "this": true, "that": true,
		"these": true, "those": true, "i": true, "me": true, "my": true,
		"you": true, "your": true, "it": true, "its": true, "we": true,
		"they": true, "them": true, "what": true, "which": true, "who": true,
		"how": true, "when": true, "where": true, "why": true, "please": true,
		"help": true,
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "他": true, "她": true,
		"它": true, "们": true, "把": true, "被": true, "让": true,
		"给": true, "对": true, "从": true, "与": true, "以": true,
	}

	delimiters := func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == ',' || r == '.' || r == '!' || r == '?' ||
			r == ';' || r == ':' || r == '"' || r == '\'' ||
			r == '、' || r == '，' || r == '。' || r == '！' ||
			r == '？' || r == '；' || r == '：'
	}

	raw := strings.FieldsFunc(text, delimiters)
	seen := map[string]bool{}
	result := make([]string, 0, len(raw))
	for _, w := range raw {
		lower := strings.ToLower(strings.TrimSpace(w))
		if len(lower) < 2 || stopWords[lower] || seen[lower] {
			continue
		}
		seen[lower] = true
		result = append(result, lower)
	}
	return result
}

// RenderSkillsPrompt 将匹配的技能列表渲染为 Prompt 片段。
func RenderSkillsPrompt(skills []model.SkillEntry) string {
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("══════════════════════════════════════════════════\n")
	b.WriteString("SKILLS (relevant workflows)\n")
	b.WriteString("══════════════════════════════════════════════════\n")

	for _, s := range skills {
		b.WriteString(fmt.Sprintf("- %s", s.Name))
		if s.Description != "" {
			b.WriteString(fmt.Sprintf(": %s", s.Description))
		}
		b.WriteString("\n")
		if s.Content != "" {
			b.WriteString(fmt.Sprintf("  └─ %s\n", s.Content))
		}
	}

	return b.String()
}

func scanSkillRows(rows *sql.Rows) ([]model.SkillEntry, error) {
	var entries []model.SkillEntry
	for rows.Next() {
		var entry model.SkillEntry
		var lastUsed sql.NullString
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.Description, &entry.Tags, &entry.Content,
			&entry.UseCount, &lastUsed, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan skill row: %w", err)
		}
		if lastUsed.Valid {
			entry.LastUsed = lastUsed.String
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
