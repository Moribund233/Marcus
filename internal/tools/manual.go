package tools

import (
	"encoding/json"
	"strings"

	"Marcus/internal/model"
)

func ParseManual(name, command, argType string) model.ToolInfo {
	manifest := model.ToolManifest{
		DisplayName:  name,
		Contribution: model.ContributionTerminal,
		Terminal: &model.TerminalManifest{
			Command: command,
			Args: []model.TerminalArg{
				{Name: "input", Label: "参数", Type: argType},
			},
		},
	}
	manifestData, _ := json.Marshal(manifest)

	id := toolID(name, string(model.SourceManual))
	return model.ToolInfo{
		ID:           id,
		Name:         strings.ToLower(strings.ReplaceAll(name, " ", "_")),
		DisplayName:  name,
		Source:       model.SourceManual,
		Contribution: model.ContributionTerminal,
		Manifest:     string(manifestData),
		Enabled:      true,
	}
}
