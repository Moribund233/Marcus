package llm

import (
	"database/sql"

	"Marcus/internal/model"
)

// ModelStore 管理模型列表的 SQLite 缓存，避免每次启动都依赖 API 获取。
// 模型列表在用户手动刷新（RefreshLLMModels）时更新。
type ModelStore struct {
	db *sql.DB
}

// NewModelStore 创建 ModelStore 实例。
func NewModelStore(db *sql.DB) *ModelStore {
	return &ModelStore{db: db}
}

// GetModels 返回指定供应商的缓存模型列表。
func (s *ModelStore) GetModels(provider model.LLMProvider) ([]model.Model, error) {
	rows, err := s.db.Query(
		`SELECT model_id, model_name, context_length FROM provider_models WHERE provider = ? ORDER BY model_id`,
		string(provider),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []model.Model
	for rows.Next() {
		var m model.Model
		if err := rows.Scan(&m.ID, &m.Name, &m.Context); err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, nil
	}
	return models, nil
}

// SetModels 替换指定供应商的完整模型缓存。
func (s *ModelStore) SetModels(provider model.LLMProvider, models []model.Model) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM provider_models WHERE provider = ?`, string(provider)); err != nil {
		return err
	}

	for _, m := range models {
		if _, err := tx.Exec(
			`INSERT INTO provider_models (provider, model_id, model_name, context_length, updated_at)
			 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			string(provider), m.ID, m.Name, m.Context,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ClearProvider 删除指定供应商的模型缓存。
func (s *ModelStore) ClearProvider(provider model.LLMProvider) error {
	_, err := s.db.Exec(`DELETE FROM provider_models WHERE provider = ?`, string(provider))
	return err
}

// globalModelStore 是包级别的全局 ModelStore，由 App 启动时设置。
var globalModelStore *ModelStore

// SetModelStore 设置全局 ModelStore。
func SetModelStore(store *ModelStore) {
	globalModelStore = store
}

// GetModelStore 返回全局 ModelStore（可能为 nil）。
func GetModelStore() *ModelStore {
	return globalModelStore
}
