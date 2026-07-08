// Package agent 实现了 Marcus LLM Agent 的核心逻辑。
package agent

import (
	"encoding/json"
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

	b.WriteString("You are Marcus, an intelligent agent that helps users by selecting and executing tools from the Marcus toolbox.\n")
	b.WriteString("Follow these rules:\n")
	b.WriteString("1. Analyze the user's request and choose the most appropriate tool(s).\n")
	b.WriteString("2. When you need a tool, respond with a tool call using the provided function schema.\n")
	b.WriteString("3. After each tool result, decide if you need another tool or if you can provide the final answer.\n")
	b.WriteString("4. Keep your final response concise and helpful.\n")
	b.WriteString("5. You can manage long-term memory using the 'memory' tool to remember user preferences or environment facts.\n")
	b.WriteString("6. Below are relevant workflows (SKILLS) that match the current request. Consider them as reference patterns.\n\n")

	if memoryPrompt != "" {
		b.WriteString(memoryPrompt)
		b.WriteString("\n\n")
	}

	if skillsPrompt != "" {
		b.WriteString(skillsPrompt)
		b.WriteString("\n\n")
	}

	tools := pm.registry.GetToolDefinitions()
	if len(tools) > 0 {
		b.WriteString("Available tools:\n")
		for _, tool := range tools {
			b.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
			if len(tool.Function.Parameters) > 0 {
				schemaJSON, err := json.MarshalIndent(tool.Function.Parameters, "  ", "  ")
				if err == nil {
					b.WriteString(fmt.Sprintf("  Parameters schema:\n%s\n", schemaJSON))
				}
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("When calling tools, always use valid JSON arguments matching the tool's parameters schema.\n")
	b.WriteString("If no tool is needed, reply directly to the user.\n")

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
