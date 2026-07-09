// Package memory 实现了 Marcus LLM Agent 的长期记忆存储与管理。
//
// 长期记忆以 SQLite 表为载体，包含用户偏好、环境事实、项目约定等稳定信息。
// 每次 Agent Run 启动时会生成冻结快照注入系统 Prompt，同时允许 Agent 通过
// memory 工具在会话中增删改记忆（变更会持久化，但当前会话的系统 Prompt 不变）。
package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"Marcus/internal/model"
)

// DefaultMemoryLimit 是长期记忆注入系统 Prompt 的默认字符上限。
const DefaultMemoryLimit = 8000

// SuggestedLimit 根据模型上下文 token 数计算建议的记忆字符上限。
// contextTokens 为模型最大上下文长度；若 <= 0 则返回 DefaultMemoryLimit。
// 规则：取 10% 上下文窗口，限制在 [2000, 32000] 范围。
func SuggestedLimit(contextTokens int) int {
	if contextTokens <= 0 {
		return DefaultMemoryLimit
	}
	limit := contextTokens * 4 / 10 // 近似：1 token ≈ 4 chars, 取 10%
	switch {
	case limit < 2000:
		return 2000
	case limit > 32000:
		return 32000
	default:
		return limit
	}
}

// Store 提供长期记忆的持久化能力。
type Store struct {
	db        *sql.DB
	limit     int
	separator string
}

// NewStore 创建一个新的记忆存储实例。
//
// 参数 db 必须是已初始化的 SQLite 连接；limit 为注入 Prompt 的字符上限，
// 若小于等于 0 则使用 DefaultMemoryLimit。
func NewStore(db *sql.DB, limit int) *Store {
	if limit <= 0 {
		limit = DefaultMemoryLimit
	}
	return &Store{
		db:        db,
		limit:     limit,
		separator: "\n§\n",
	}
}

// Add 添加一条记忆；若同 key 已存在则更新内容与 scope。
func (s *Store) Add(entry model.MemoryEntry) (*model.MemoryStats, error) {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Scope == "" {
		entry.Scope = model.MemoryScopeGlobal
	}
		now := time.Now().UTC().Format(time.RFC3339)
		entry.CreatedAt = now
		entry.UpdatedAt = now

	_, err := s.db.Exec(
		`INSERT INTO memories (id, scope, key, content, source, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
			 scope=excluded.scope,
			 content=excluded.content,
			 source=excluded.source,
			 updated_at=excluded.updated_at`,
		entry.ID, string(entry.Scope), entry.Key, entry.Content, entry.Source,
		entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert memory: %w", err)
	}

	return s.Stats(entry.Scope)
}

// GetByKey 通过 key 查找记忆。
func (s *Store) GetByKey(key string) (*model.MemoryEntry, error) {
	row := s.db.QueryRow(
		`SELECT id, scope, key, content, source, created_at, updated_at
		 FROM memories WHERE key = ?`, key)

	var entry model.MemoryEntry
	var scopeStr string
	err := row.Scan(&entry.ID, &scopeStr, &entry.Key, &entry.Content, &entry.Source, &entry.CreatedAt, &entry.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query memory by key: %w", err)
	}
	entry.Scope = model.MemoryScope(scopeStr)
	return &entry, nil
}

// Replace 通过 oldText 子字符串匹配替换已有记忆的内容。
// 若未找到匹配或存在多条匹配，返回错误。
func (s *Store) Replace(key, oldText, newText string) (*model.MemoryStats, error) {
	entry, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("memory with key %q not found", key)
	}

	count := strings.Count(entry.Content, oldText)
	if count == 0 {
		return nil, fmt.Errorf("old_text not found in memory %q", key)
	}
	if count > 1 {
		return nil, fmt.Errorf("old_text matches %d places in memory %q; please provide more specific text", count, key)
	}

	entry.Content = strings.Replace(entry.Content, oldText, newText, 1)
		entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		_, err = s.db.Exec(
			`UPDATE memories SET content = ?, updated_at = ? WHERE key = ?`,
			entry.Content, entry.UpdatedAt, key,
		)
		if err != nil {
			return nil, fmt.Errorf("update memory: %w", err)
		}

		return s.Stats(entry.Scope)
	}

	// Remove 删除整条记忆或通过 oldText 子字符串匹配删除部分内容。
func (s *Store) Remove(key, oldText string) (*model.MemoryStats, error) {
	entry, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("memory with key %q not found", key)
	}

	if oldText == "" {
		// 删除整条记忆
		_, err = s.db.Exec(`DELETE FROM memories WHERE key = ?`, key)
		if err != nil {
			return nil, fmt.Errorf("delete memory: %w", err)
		}
		return s.Stats(entry.Scope)
	}

	count := strings.Count(entry.Content, oldText)
	if count == 0 {
		return nil, fmt.Errorf("old_text not found in memory %q", key)
	}
	if count > 1 {
		return nil, fmt.Errorf("old_text matches %d places in memory %q; please provide more specific text", count, key)
	}

	entry.Content = strings.Replace(entry.Content, oldText, "", 1)
		entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if strings.TrimSpace(entry.Content) == "" {
		_, err = s.db.Exec(`DELETE FROM memories WHERE key = ?`, key)
		if err != nil {
			return nil, fmt.Errorf("delete empty memory: %w", err)
		}
	} else {
		_, err = s.db.Exec(
			`UPDATE memories SET content = ?, updated_at = ? WHERE key = ?`,
			entry.Content, entry.UpdatedAt, key,
		)
		if err != nil {
			return nil, fmt.Errorf("update memory: %w", err)
		}
	}

	return s.Stats(entry.Scope)
}

// List 返回符合条件的记忆列表。
func (s *Store) List(scope model.MemoryScope) ([]model.MemoryEntry, error) {
	query := `SELECT id, scope, key, content, source, created_at, updated_at FROM memories`
	var args []interface{}
	if scope != "" {
		query += ` WHERE scope = ?`
		args = append(args, string(scope))
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	return scanMemoryRows(rows)
}

// Stats 返回指定 scope 的记忆容量统计。
func (s *Store) Stats(scope model.MemoryScope) (*model.MemoryStats, error) {
	query := `SELECT COALESCE(SUM(LENGTH(content)), 0) FROM memories`
	var args []interface{}
	if scope != "" {
		query += ` WHERE scope = ?`
		args = append(args, string(scope))
	}

	var total int
	if err := s.db.QueryRow(query, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("calculate memory stats: %w", err)
	}

	return &model.MemoryStats{
		TotalChars: total,
		MaxChars:   s.limit,
		Usage:      float64(total) / float64(s.limit),
	}, nil
}

// BuildSnapshot 构建用于注入系统 Prompt 的记忆快照。
// 默认包含 user 和 global 两个 scope 的记忆，按最近更新排序。
func (s *Store) BuildSnapshot() (*model.MemorySnapshot, error) {
	entries, err := s.List("")
	if err != nil {
		return nil, err
	}

	var filtered []model.MemoryEntry
	total := 0
	for _, e := range entries {
		if e.Scope != model.MemoryScopeUser && e.Scope != model.MemoryScopeGlobal {
			continue
		}
		if total+len(e.Content) > s.limit {
			break
		}
		filtered = append(filtered, e)
		total += len(e.Content)
	}

	stats := &model.MemoryStats{
		TotalChars: total,
		MaxChars:   s.limit,
		Usage:      float64(total) / float64(s.limit),
	}

	return &model.MemorySnapshot{
		Entries: filtered,
		Stats:   *stats,
	}, nil
}

// RenderPrompt 将记忆快照渲染为 Prompt 文本。
func (s *Store) RenderPrompt(snapshot *model.MemorySnapshot) string {
	if snapshot == nil || len(snapshot.Entries) == 0 {
		return ""
	}

	var contents []string
	for _, e := range snapshot.Entries {
		contents = append(contents, fmt.Sprintf("[%s] %s", e.Key, e.Content))
	}

	usage := ""
	if snapshot.Stats.MaxChars > 0 {
		usage = fmt.Sprintf(" [%d%% — %d/%d chars]", int(snapshot.Stats.Usage*100), snapshot.Stats.TotalChars, snapshot.Stats.MaxChars)
	}

	return fmt.Sprintf(
		"══════════════════════════════════════════════════\n"+
			"MEMORY (long-term facts)%s\n"+
			"══════════════════════════════════════════════════\n"+
			"%s",
		usage,
		strings.Join(contents, s.separator),
	)
}

// ApplyToolCall 执行 memory 工具调用，并返回结果字符串。
func (s *Store) ApplyToolCall(call model.ToolCall) (string, error) {
	if call.Function.Name != "memory" {
		return "", fmt.Errorf("not a memory tool call")
	}

	var args struct {
		Action  string `json:"action"`
		Scope   string `json:"scope"`
		Key     string `json:"key"`
		Content string `json:"content"`
		OldText string `json:"old_text"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("parse memory tool arguments: %w", err)
	}

	if args.Key == "" {
		return "", fmt.Errorf("memory tool requires key")
	}
	if args.Scope == "" {
		args.Scope = string(model.MemoryScopeGlobal)
	}

	switch args.Action {
	case "add":
		if args.Content == "" {
			return "", fmt.Errorf("memory add requires content")
		}
		stats, err := s.Add(model.MemoryEntry{
			Scope:   model.MemoryScope(args.Scope),
			Key:     args.Key,
			Content: args.Content,
			Source:  "agent",
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Memory added. Usage: %d/%d chars (%d%%).", stats.TotalChars, stats.MaxChars, int(stats.Usage*100)), nil

	case "replace":
		stats, err := s.Replace(args.Key, args.OldText, args.Content)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Memory updated. Usage: %d/%d chars (%d%%).", stats.TotalChars, stats.MaxChars, int(stats.Usage*100)), nil

	case "remove":
		stats, err := s.Remove(args.Key, args.OldText)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Memory removed. Usage: %d/%d chars (%d%%).", stats.TotalChars, stats.MaxChars, int(stats.Usage*100)), nil

	default:
		return "", fmt.Errorf("unsupported memory action: %s", args.Action)
	}
}

func scanMemoryRows(rows *sql.Rows) ([]model.MemoryEntry, error) {
	var entries []model.MemoryEntry
	for rows.Next() {
		var entry model.MemoryEntry
		var scopeStr string
		if err := rows.Scan(&entry.ID, &scopeStr, &entry.Key, &entry.Content, &entry.Source, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory row: %w", err)
		}
		entry.Scope = model.MemoryScope(scopeStr)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
