package model

import "fmt"

// ToolMessages holds user-facing messages for tool operations.
// Keys are in English for internal use; values can be localized.
var ToolMessages = map[string]map[string]string{
	"zh-CN": {
		// Uninstall messages
		"uninstall.store.unregistered":    "已卸载工具 %s",
		"uninstall.uv.notFound":           "uv 未安装，无法通过包管理器卸载。已从工具列表移除，建议手动清理 uv 环境",
		"uninstall.uv.success":            "已通过 uv 卸载工具 %s",
		"uninstall.bun.notFound":          "bun 未安装，无法通过包管理器卸载。已从工具列表移除，建议手动清理 bun 环境",
		"uninstall.bun.success":           "已通过 bun 卸载工具 %s",
		"uninstall.binary.success":        "已从工具列表移除 %s。原始文件未删除，请手动清理",
		"uninstall.manual.success":        "已移除手动添加的工具 %s",
		"uninstall.generic.success":       "已从工具列表移除 %s",

		// Runtime installation messages
		"runtime.uv.alreadyInstalled":     "uv 已安装",
		"runtime.bun.alreadyInstalled":    "bun 已安装",
		"runtime.uv.pipNotFound":          "pip 未安装，无法安装 uv。请先安装 Python pip",
		"runtime.bun.npmNotFound":         "npm 未安装，无法安装 bun。请先安装 Node.js npm",
		"runtime.uv.installing":           "正在通过 pip 安装 uv...",
		"runtime.bun.installing":          "正在通过 npm 安装 bun...",
		"runtime.uv.installSuccess":       "uv 安装成功",
		"runtime.bun.installSuccess":      "bun 安装成功",
		"runtime.unknown":                 "不支持的运行时: %s",

		// Package installation messages
		"install.preparing":               "准备安装...",
		"install.running":                 "正在执行安装命令...",
		"install.registering":             "安装完成，正在注册工具...",
		"install.registered":              "已注册 %d 个新工具",
		"install.uv.notFound":             "uv 未安装，无法安装 Python 包。请先安装 uv：\n  powershell -c \"irm https://astral.sh/uv/install.ps1 | iex\"",
		"install.bun.notFound":            "bun 未安装，无法安装 Node.js 包。请先安装 bun：\n  powershell -c \"irm bun.sh/install.ps1 | iex\"",
		"install.unsupportedType":         "不支持的包类型: %s（支持: .whl, .tgz）",
		"install.noToolsFound":            "安装成功，但未找到可注册的 Marcus 工具。请确认包包含 marcus entry point 或 marcus 配置",

		// Scanner messages
		"scanner.uv.error":                "uv 扫描错误: %s",
		"scanner.bun.error":               "bun 扫描错误: %s",
		"scanner.binary.error":            "binary 扫描错误: %s",

		// Sandbox messages
		"sandbox.timeout":                 "进程超时",
		"sandbox.healthCheckFailed":       "健康检查超时",
		"sandbox.processNotFound":         "未找到进程: %s",
	},
	"en-US": {
		// Uninstall messages
		"uninstall.store.unregistered":    "Uninstalled tool %s",
		"uninstall.uv.notFound":           "uv is not installed; cannot uninstall via package manager. Removed from tool list. Please clean up uv environment manually.",
		"uninstall.uv.success":            "Uninstalled tool %s via uv",
		"uninstall.bun.notFound":          "bun is not installed; cannot uninstall via package manager. Removed from tool list. Please clean up bun environment manually.",
		"uninstall.bun.success":           "Uninstalled tool %s via bun",
		"uninstall.binary.success":        "Removed %s from tool list. Original files were not deleted; please clean up manually.",
		"uninstall.manual.success":        "Removed manually added tool %s",
		"uninstall.generic.success":       "Removed %s from tool list",

		// Runtime installation messages
		"runtime.uv.alreadyInstalled":     "uv is already installed",
		"runtime.bun.alreadyInstalled":    "bun is already installed",
		"runtime.uv.pipNotFound":          "pip is not installed; cannot install uv. Please install Python pip first.",
		"runtime.bun.npmNotFound":         "npm is not installed; cannot install bun. Please install Node.js npm first.",
		"runtime.uv.installing":           "Installing uv via pip...",
		"runtime.bun.installing":          "Installing bun via npm...",
		"runtime.uv.installSuccess":       "uv installed successfully",
		"runtime.bun.installSuccess":      "bun installed successfully",
		"runtime.unknown":                 "Unknown runtime: %s",

		// Package installation messages
		"install.preparing":               "Preparing installation...",
		"install.running":                 "Running install command...",
		"install.registering":             "Installation complete, registering tools...",
		"install.registered":              "Registered %d new tools",
		"install.uv.notFound":             "uv is not installed; cannot install Python packages. Please install uv first:\n  powershell -c \"irm https://astral.sh/uv/install.ps1 | iex\"",
		"install.bun.notFound":            "bun is not installed; cannot install Node.js packages. Please install bun first:\n  powershell -c \"irm bun.sh/install.ps1 | iex\"",
		"install.unsupportedType":         "Unsupported package type: %s (supported: .whl, .tgz)",
		"install.noToolsFound":            "Installation succeeded, but no registerable Marcus tools were found. Please ensure the package contains a marcus entry point or marcus config.",

		// Scanner messages
		"scanner.uv.error":                "uv scan error: %s",
		"scanner.bun.error":               "bun scan error: %s",
		"scanner.binary.error":            "binary scan error: %s",

		// Sandbox messages
		"sandbox.timeout":                 "Process timed out",
		"sandbox.healthCheckFailed":       "Health check timed out",
		"sandbox.processNotFound":         "Process not found: %s",
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
