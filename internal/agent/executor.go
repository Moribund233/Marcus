// Package agent 实现了 Marcus LLM Agent 的核心逻辑。
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"Marcus/internal/memory"
	"Marcus/internal/model"
)

// SyncRunner 是同步执行 Marcus 工具的接口。
type SyncRunner interface {
	RunSync(ctx context.Context, manifest model.ToolManifest, args map[string]string) (string, int, error)
}

// Executor 负责将 LLM 的 ToolCall 转换为实际工具执行。
type Executor struct {
	registry *Registry
	runner   SyncRunner
	memory   *memory.Store
}

// NewExecutor 创建一个新的工具执行器。
func NewExecutor(registry *Registry, runner SyncRunner, mem *memory.Store) *Executor {
	return &Executor{
		registry: registry,
		runner:   runner,
		memory:   mem,
	}
}

// Execute 执行单个 ToolCall 并返回 ToolCallResult。
// 支持 memory 工具（自我管理长期记忆）和 Marcus 注册工具。
func (e *Executor) Execute(ctx context.Context, call model.ToolCall) model.ToolCallResult {
	result := model.ToolCallResult{
		ToolCallID: call.ID,
		Name:       call.Function.Name,
		Role:       "tool",
	}

	if call.Function.Name == "memory" && e.memory != nil {
		out, err := e.memory.ApplyToolCall(call)
		if err != nil {
			result.Content = fmt.Sprintf("memory tool failed: %v", err)
		} else {
			result.Content = out
		}
		return result
	}

	manifest, ok := e.registry.GetManifest(call.Function.Name)
	if !ok {
		result.Content = fmt.Sprintf("tool %q not found in registry", call.Function.Name)
		return result
	}

	args, err := parseToolArguments(call.Function.Arguments)
	if err != nil {
		result.Content = fmt.Sprintf("parse tool arguments: %v", err)
		return result
	}

	output, exitCode, err := e.runner.RunSync(ctx, *manifest, args)
	if err != nil {
		result.Content = fmt.Sprintf("execute tool %q failed (exit %d): %v\nOutput:\n%s", call.Function.Name, exitCode, err, truncateOutput(output))
		return result
	}

	result.Content = truncateOutput(output)
	return result
}

// maxToolOutputLen 是工具输出注入 LLM 上下文的最大字节数。
// 超出部分被截断，防止撑爆对话窗口。
const maxToolOutputLen = 10 * 1024

// truncateOutput 截断工具输出至 maxToolOutputLen。
func truncateOutput(s string) string {
	if len(s) <= maxToolOutputLen {
		return s
	}
	return s[:maxToolOutputLen] + fmt.Sprintf("\n\n[...output truncated at %d bytes, total %d bytes]", maxToolOutputLen, len(s))
}

// parseToolArguments 将工具调用的 JSON 参数字符串解析为字符串映射。
func parseToolArguments(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("invalid json arguments: %w", err)
	}

	result := make(map[string]string, len(data))
	for k, v := range data {
		switch val := v.(type) {
		case string:
			result[k] = val
		case nil:
			result[k] = ""
		default:
			b, _ := json.Marshal(val)
			result[k] = string(b)
		}
	}
	return result, nil
}
