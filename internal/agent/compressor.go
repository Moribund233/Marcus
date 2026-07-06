// Package agent 实现了 Marcus LLM Agent 的核心逻辑。
package agent

import (
	"fmt"

	"Marcus/internal/model"
)

// ContextCompressor 根据模型上下文长度限制对消息历史进行压缩决策。
type ContextCompressor struct {
	maxTokens int
	threshold float64
}

// NewContextCompressor 创建一个新的上下文压缩器。
//
// maxTokens 为模型支持的最大 token 数；threshold 为触发压缩的阈值比例（0.0-1.0）。
// 当历史消息估算 token 数超过 maxTokens*threshold 时，ShouldCompress 返回 true。
func NewContextCompressor(maxTokens int, threshold float64) *ContextCompressor {
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	if threshold <= 0 || threshold > 1 {
		threshold = 0.85
	}
	return &ContextCompressor{
		maxTokens: maxTokens,
		threshold: threshold,
	}
}

// ShouldCompress 判断当前消息历史是否需要进行压缩。
func (c *ContextCompressor) ShouldCompress(messages []model.Message) bool {
	total := c.EstimateTokens(messages)
	return float64(total) > float64(c.maxTokens)*c.threshold
}

// EstimateTokens 使用简单启发式估算消息列表的 token 数：
// 每个消息固定 4 个 token 开销，内容按 1 token / 4 字符估算（英文平均）。
// 该估算不精确，但足够用于压缩决策；未来可替换为 tiktoken-go 等精确计数库。
func (c *ContextCompressor) EstimateTokens(messages []model.Message) int {
	const overhead = 4
	const charsPerToken = 4

	total := 0
	for _, m := range messages {
		total += overhead
		if len(m.Content) > 0 {
			total += len(m.Content) / charsPerToken
		}
	}
	return total
}

// Compress 对消息历史进行压缩：保留系统提示和最近的用户/助手消息，
// 对较早的消息进行摘要合并。
//
// 当前实现为简化版：直接丢弃系统提示之后的旧消息，仅保留最近的 N 条。
// 后续可引入 LLM 摘要生成更智能的压缩结果。
func (c *ContextCompressor) Compress(messages []model.Message) []model.Message {
	if len(messages) == 0 {
		return messages
	}

	// 保留系统提示（如果有）。
	var systemMsg *model.Message
	startIdx := 0
	if messages[0].Role == model.RoleSystem {
		systemMsg = &messages[0]
		startIdx = 1
	}

	// 计算在阈值内最多能保留多少条消息。
	maxTokens := int(float64(c.maxTokens) * c.threshold)
	keep := 0
	current := 0
	for i := len(messages) - 1; i >= startIdx; i-- {
		tokens := c.EstimateTokens([]model.Message{messages[i]})
		if current+tokens > maxTokens && keep > 0 {
			break
		}
		current += tokens
		keep++
	}

	result := make([]model.Message, 0, keep+1)
	if systemMsg != nil {
		result = append(result, *systemMsg)
	}
	if keep > 0 {
		start := len(messages) - keep
		if start < startIdx {
			start = startIdx
		}
		result = append(result, messages[start:]...)
	}

	return result
}

// Summary 返回当前压缩器的配置信息。
func (c *ContextCompressor) Summary() string {
	return fmt.Sprintf("maxTokens=%d threshold=%.0f%%", c.maxTokens, c.threshold*100)
}
