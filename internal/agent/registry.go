// Package agent 实现了 Marcus LLM Agent 的核心逻辑。
//
// 包括工具注册表、Prompt 管理、工具执行器、TAO 循环以及长期记忆集成。
package agent

import (
	"fmt"
	"sort"
	"strings"

	"Marcus/internal/model"
)

// Registry 维护从 Marcus 工具到 LLM ToolDefinition 的映射。
// 区分系统工具（内置，LLM 内部使用，不暴露给用户）和用户工具（工具箱安装注册）。
type Registry struct {
	// definitions 保存用户工具（工具箱安装）的定义，key 为 tool ID。
	definitions map[string]model.ToolDefinition
	// systemDefs 保存系统工具（如 memory）的定义，仅用于 LLM 调用，不写入 system prompt。
	systemDefs map[string]model.ToolDefinition
	// manifests 保存原始 ToolManifest，key 为 tool ID，供执行器使用。
	manifests map[string]*model.ToolManifest
}

// NewRegistry 创建一个新的工具注册表。
func NewRegistry() *Registry {
	return &Registry{
		definitions: make(map[string]model.ToolDefinition),
		systemDefs:  make(map[string]model.ToolDefinition),
		manifests:   make(map[string]*model.ToolManifest),
	}
}

// RegisterFromManifest 从 ToolManifest 自动生成并注册 LLM ToolDefinition。
func (r *Registry) RegisterFromManifest(manifest *model.ToolManifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	// 优先用 ID，为空时回退到 DisplayName（必填字段）
	id := manifest.ID
	if id == "" {
		id = manifest.DisplayName
	}
	if id == "" {
		return fmt.Errorf("manifest id and display_name are both empty")
	}

	params, err := convertParams(manifest)
	if err != nil {
		return fmt.Errorf("convert params for %s: %w", id, err)
	}

	desc := manifest.Description
	if desc == "" {
		desc = fmt.Sprintf("Execute the %s tool.", manifest.DisplayName)
	}

	def := model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunctionDefinition{
			Name:        id,
			Description: desc,
			Parameters:  params,
		},
	}

	r.definitions[id] = def
	r.manifests[id] = manifest
	return nil
}

// ClearUserTools clears all user-registered tools (leaves system tools intact).
// Used before re-registering after a re-scan so stale tools are removed.
func (r *Registry) ClearUserTools() {
	r.definitions = make(map[string]model.ToolDefinition)
	r.manifests = make(map[string]*model.ToolManifest)
}

// RegisterMemoryTool 注册 Agent 用于自我管理长期记忆的特殊工具。
func (r *Registry) RegisterMemoryTool() {
	def := model.ToolDefinition{
		Type: "function",
		Function: model.ToolFunctionDefinition{
			Name:        "memory",
			Description: "Manage long-term memory: add, replace, or remove stable facts and user preferences.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "string",
						"enum": []string{"add", "replace", "remove"},
						"description": "Action to perform on memory.",
					},
					"scope": map[string]interface{}{
						"type": "string",
						"enum": []string{"user", "project", "global"},
						"description": "Memory scope. Default: global",
					},
					"key": map[string]interface{}{
						"type": "string",
						"description": "Short unique key for substring matching",
					},
					"content": map[string]interface{}{
						"type": "string",
						"description": "Full memory content (required for add/replace)",
					},
					"old_text": map[string]interface{}{
						"type": "string",
						"description": "Substring of the existing entry to replace/remove",
					},
				},
				"required": []string{"action", "key"},
			},
		},
	}
	r.systemDefs["memory"] = def
}

// GetToolDefinitions 返回所有工具定义（系统工具 + 用户工具），按名称排序。
// 用于 LLM function calling API，LLM 可以看到并调用所有工具。
func (r *Registry) GetToolDefinitions() []model.ToolDefinition {
	ids := make([]string, 0, len(r.definitions)+len(r.systemDefs))
	for id := range r.definitions {
		ids = append(ids, id)
	}
	for id := range r.systemDefs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	defs := make([]model.ToolDefinition, 0, len(ids))
	for _, id := range ids {
		if d, ok := r.definitions[id]; ok {
			defs = append(defs, d)
		} else if d, ok := r.systemDefs[id]; ok {
			defs = append(defs, d)
		}
	}
	return defs
}

// GetUserToolDefinitions 仅返回用户安装的工具定义（不包含系统工具），按名称排序。
// 用于 system prompt 文本描述，避免 LLM 将系统工具暴露给用户。
func (r *Registry) GetUserToolDefinitions() []model.ToolDefinition {
	ids := make([]string, 0, len(r.definitions))
	for id := range r.definitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	defs := make([]model.ToolDefinition, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, r.definitions[id])
	}
	return defs
}

// GetManifest 返回指定 tool ID 的原始 ToolManifest。
func (r *Registry) GetManifest(toolID string) (*model.ToolManifest, bool) {
	m, ok := r.manifests[toolID]
	return m, ok
}

// convertParams 根据 contribution 类型转换参数为 JSON Schema。
func convertParams(manifest *model.ToolManifest) (map[string]interface{}, error) {
	switch manifest.Contribution {
	case model.ContributionTerminal:
		return convertTerminalParams(manifest.Terminal), nil
	case model.ContributionWeb:
		return convertWebParams(manifest.Web), nil
	case model.ContributionFile:
		return convertFileParams(manifest.File), nil
	default:
		return nil, fmt.Errorf("unsupported contribution type: %s", manifest.Contribution)
	}
}

// convertTerminalParams 将 TerminalManifest 转换为 JSON Schema。
func convertTerminalParams(tm *model.TerminalManifest) map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	if tm != nil {
		for _, arg := range tm.Args {
			prop := terminalArgToProperty(arg)
			properties[arg.Name] = prop
			if arg.Default == nil {
				required = append(required, arg.Name)
			}
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

// convertFileParams 将 FileManifest 转换为 JSON Schema。
func convertFileParams(fm *model.FileManifest) map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	if fm != nil {
		inputType := "string"
		switch fm.InputType {
		case "files":
			inputType = "array"
		case "directory":
			inputType = "string"
		default:
			inputType = "string"
		}

		inputProp := map[string]interface{}{
			"type":        inputType,
			"description": fmt.Sprintf("Input %s for the tool.", fm.InputType),
		}
		if len(fm.InputExtensions) > 0 {
			inputProp["description"] = fmt.Sprintf("Input %s. Supported extensions: %s.",
				fm.InputType, strings.Join(fm.InputExtensions, ", "))
		}
		properties["input"] = inputProp
		required = append(required, "input")

		for _, arg := range fm.Args {
			properties[arg.Name] = terminalArgToProperty(arg)
			if arg.Default == nil {
				required = append(required, arg.Name)
			}
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

// convertWebParams 将 WebManifest 转换为 JSON Schema。
func convertWebParams(wm *model.WebManifest) map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	if wm != nil {
		if wm.Port > 0 {
			properties["port"] = map[string]interface{}{
				"type":        "integer",
				"description": "Port to run the web service on.",
				"default":     wm.Port,
			}
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

// terminalArgToProperty 将 TerminalArg 转换为 JSON Schema 属性。
func terminalArgToProperty(arg model.TerminalArg) map[string]interface{} {
	prop := map[string]interface{}{
		"type":        argTypeToJSONSchema(arg.Type),
		"description": arg.Label,
	}
	if arg.Default != nil {
		prop["default"] = arg.Default
	}
	return prop
}

// argTypeToJSONSchema 将 Marcus 参数类型映射为 JSON Schema 类型。
func argTypeToJSONSchema(t string) string {
	switch t {
	case "number":
		return "number"
	case "integer":
		return "integer"
	case "boolean":
		return "boolean"
	case "array":
		return "array"
	default:
		return "string"
	}
}
