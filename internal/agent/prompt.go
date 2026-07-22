// Package agent 实现了 Marcus LLM Agent 的核心逻辑。
package agent

import (
	"fmt"
	"strings"

	"Marcus/internal/model"
)

// PromptManager 负责构建和维护 Agent 系统提示词。
type PromptManager struct {
	registry *Registry
}

// NewPromptManager 创建一个新的 PromptManager。
func NewPromptManager(registry *Registry) *PromptManager {
	return &PromptManager{registry: registry}
}

// BuildSystemPrompt 构建完整的系统提示词，包含记忆快照、技能提示和可用工具说明。
func (pm *PromptManager) BuildSystemPrompt(memoryPrompt, skillsPrompt string) string {
	var b strings.Builder

	b.WriteString("You are Marcus, a tool-use assistant. Your job:\n")
	b.WriteString("1. Analyze the user's request.\n")
	b.WriteString("2. When a tool is needed, call it via the provided function schema.\n")
	b.WriteString("3. After each tool result, decide: call another tool or give the final answer.\n")
	b.WriteString("4. If a tool fails, try correcting the parameters; if still failing, explain to the user.\n")
	b.WriteString("5. Keep final responses clear and concise.\n\n")

	if memoryPrompt != "" {
		b.WriteString(memoryPrompt)
		b.WriteString("\n\n")
	}

	if skillsPrompt != "" {
		b.WriteString(skillsPrompt)
		b.WriteString("\n\n")
	}

	tools := pm.registry.GetUserToolDefinitions()
	if len(tools) > 0 {
		b.WriteString(fmt.Sprintf("You have %d tool(s) available in your toolbox. They are listed in the functions parameter above — use them as needed.\n\n", len(tools)))
	} else {
		b.WriteString("Your toolbox is currently empty. Do not fabricate tools; help the user directly.\n\n")
	}

	return b.String()
}

// BuildToolResultMessage 将 ToolCallResult 构建为适合 LLM 的消息。
func (pm *PromptManager) BuildToolResultMessage(result model.ToolCallResult) model.Message {
	return model.Message{
		Role:       model.RoleTool,
		Name:       result.Name,
		Content:    result.Content,
		ToolCallID: result.ToolCallID,
	}
}
