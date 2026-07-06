// Package conversation 实现 Marcus LLM Agent 的对话存储。
//
// 使用 SQLite 持久化 conversations 和 messages 表，支持工具调用与结果的 JSON 序列化。
package conversation

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"Marcus/internal/model"
)

// Store 提供对话与会话的持久化能力。
type Store struct {
	db *sql.DB
}

// NewStore 创建一个新的对话存储实例。
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateConversation 创建新对话。
func (s *Store) CreateConversation(title string) (*model.Conversation, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	conv := &model.Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(
		`INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		conv.ID, conv.Title, conv.CreatedAt, conv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert conversation: %w", err)
	}
	return conv, nil
}

// ListConversations 返回最近更新的对话列表。
func (s *Store) ListConversations(limit int) ([]model.Conversation, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var convs []model.Conversation
	for rows.Next() {
		var c model.Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

// GetConversation 按 ID 获取对话。
func (s *Store) GetConversation(id string) (*model.Conversation, error) {
	row := s.db.QueryRow(
		`SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?`, id)
	var c model.Conversation
	if err := row.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return &c, nil
}

// UpdateConversationTitle 更新对话标题。
func (s *Store) UpdateConversationTitle(id, title string) error {
	_, err := s.db.Exec(
		`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`,
		title, time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("update conversation title: %w", err)
	}
	return nil
}

// DeleteConversation 删除对话及其消息。
func (s *Store) DeleteConversation(id string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE conversation_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	_, err = s.db.Exec(`DELETE FROM conversations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

// AddMessage 添加一条消息到对话。
func (s *Store) AddMessage(conversationID string, role model.MessageRole, content string) (*model.ConversationMessage, error) {
	msg := &model.ConversationMessage{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.Exec(
		`INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		msg.ID, msg.ConversationID, string(msg.Role), msg.Content, msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	s.touchConversation(conversationID)
	return msg, nil
}

// AddAssistantMessage 添加助手消息，包含工具调用。
func (s *Store) AddAssistantMessage(conversationID string, content string, toolCalls []model.ToolCall) (*model.ConversationMessage, error) {
	tcJSON, err := json.Marshal(toolCalls)
	if err != nil {
		return nil, fmt.Errorf("marshal tool calls: %w", err)
	}

	msg := &model.ConversationMessage{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           model.RoleAssistant,
		Content:        content,
		ToolCalls:      toolCalls,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err = s.db.Exec(
		`INSERT INTO messages (id, conversation_id, role, content, tool_calls, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ConversationID, string(msg.Role), msg.Content, string(tcJSON), msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert assistant message: %w", err)
	}

	s.touchConversation(conversationID)
	return msg, nil
}

// AddToolResults 添加工具结果消息。
func (s *Store) AddToolResults(conversationID string, results []model.ToolCallResult) (*model.ConversationMessage, error) {
	trJSON, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("marshal tool results: %w", err)
	}

	msg := &model.ConversationMessage{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           model.RoleTool,
		Content:        "",
		ToolResults:    results,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err = s.db.Exec(
		`INSERT INTO messages (id, conversation_id, role, content, tool_results, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ConversationID, string(msg.Role), msg.Content, string(trJSON), msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert tool results: %w", err)
	}

	s.touchConversation(conversationID)
	return msg, nil
}

// GetMessages 返回指定对话的所有消息，按时间排序。
func (s *Store) GetMessages(conversationID string) ([]model.ConversationMessage, error) {
	rows, err := s.db.Query(
		`SELECT id, conversation_id, role, content, tool_calls, tool_results, created_at
		 FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// touchConversation 更新对话的 updated_at。
func (s *Store) touchConversation(id string) {
	_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id)
}

func scanMessages(rows *sql.Rows) ([]model.ConversationMessage, error) {
	var msgs []model.ConversationMessage
	for rows.Next() {
		var msg model.ConversationMessage
		var roleStr string
		var tcJSON, trJSON sql.NullString
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &roleStr, &msg.Content, &tcJSON, &trJSON, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msg.Role = model.MessageRole(roleStr)
		if tcJSON.Valid && tcJSON.String != "" {
			_ = json.Unmarshal([]byte(tcJSON.String), &msg.ToolCalls)
		}
		if trJSON.Valid && trJSON.String != "" {
			_ = json.Unmarshal([]byte(trJSON.String), &msg.ToolResults)
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}
