package model

import "fmt"

// ToolMessages holds user-facing messages for tool operations.
// Keys are in English for internal use; values can be localized.
var ToolMessages = map[string]map[string]string{
	"zh-CN": {
		"uninstall.store.unregistered":    "已卸载工具 %s",
		"uninstall.uv.notFound":           "uv 未安装，无法通过包管理器卸载。已从工具列表移除，建议手动清理 uv 环境",
		"uninstall.uv.success":            "已通过 uv 卸载工具 %s",
		"uninstall.bun.notFound":          "bun 未安装，无法通过包管理器卸载。已从工具列表移除，建议手动清理 bun 环境",
		"uninstall.bun.success":           "已通过 bun 卸载工具 %s",
		"uninstall.binary.success":        "已从工具列表移除 %s。原始文件未删除，请手动清理",
		"uninstall.manual.success":        "已移除手动添加的工具 %s",
		"uninstall.generic.success":       "已从工具列表移除 %s",
	},
	"en-US": {
		"uninstall.store.unregistered":    "Uninstalled tool %s",
		"uninstall.uv.notFound":           "uv is not installed; cannot uninstall via package manager. Removed from tool list. Please clean up uv environment manually.",
		"uninstall.uv.success":            "Uninstalled tool %s via uv",
		"uninstall.bun.notFound":          "bun is not installed; cannot uninstall via package manager. Removed from tool list. Please clean up bun environment manually.",
		"uninstall.bun.success":           "Uninstalled tool %s via bun",
		"uninstall.binary.success":        "Removed %s from tool list. Original files were not deleted; please clean up manually.",
		"uninstall.manual.success":        "Removed manually added tool %s",
		"uninstall.generic.success":       "Removed %s from tool list",
	},
}

// Localize returns a localized message for the given key and language.
// Falls back to zh-CN if the language is unknown.
func Localize(lang, key string, args ...any) string {
	langMap, ok := ToolMessages[lang]
	if !ok {
		langMap = ToolMessages["zh-CN"]
	}
	tmpl, ok := langMap[key]
	if !ok {
		tmpl = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(tmpl, args...)
	}
	return tmpl
}
