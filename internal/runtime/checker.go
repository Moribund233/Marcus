package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"Marcus/internal/executil"
	"Marcus/internal/model"
)

type Checker struct {
	mu sync.Mutex
}

func New() *Checker {
	return &Checker{}
}

type checkTask struct {
	Name    string
	Command string
	Args    []string
}

var runtimes = []checkTask{
	{Name: "Python 3", Command: "python3", Args: []string{"--version"}},
	{Name: "Python 3", Command: "python", Args: []string{"--version"}},
	{Name: "Node.js", Command: "node", Args: []string{"--version"}},
	{Name: "uv", Command: "uv", Args: []string{"--version"}},
	{Name: "bun", Command: "bun", Args: []string{"--version"}},
}

func (c *Checker) CheckAll() map[string]model.RuntimeInfo {
	var mu sync.Mutex
	var wg sync.WaitGroup

	results := map[string]model.RuntimeInfo{
		"python": {Name: "Python 3"},
		"node":   {Name: "Node.js"},
		"uv":     {Name: "uv"},
		"bun":    {Name: "bun"},
	}

	check := func(key string, tasks []checkTask) {
		defer wg.Done()
		info := model.RuntimeInfo{Name: tasks[0].Name}
		for _, t := range tasks {
			path, err := exec.LookPath(t.Command)
			if err != nil {
				continue
			}
				cmd := exec.Command(t.Command, t.Args...)
				executil.HideWindow(cmd)
				out, err := cmd.Output()
				if err != nil {
					continue
				}
			info.Available = true
			info.Path = path
			info.Version = parseVersion(string(out))
			break
		}
		mu.Lock()
		results[key] = info
		mu.Unlock()
	}

	wg.Add(4)
	go check("python", []checkTask{runtimes[0], runtimes[1]})
	go check("node", []checkTask{runtimes[2]})
	go check("uv", []checkTask{runtimes[3]})
	go check("bun", []checkTask{runtimes[4]})

	// Add installation hints for missing runtimes.
	wg.Wait()

	if !results["uv"].Available {
		r := results["uv"]
		r.Hint = "uv 未安装。点击右侧「安装」按钮通过 pip 一键安装。"
		results["uv"] = r
	}
	if !results["bun"].Available {
		r := results["bun"]
		r.Hint = "bun 未安装。点击右侧「安装」按钮通过 npm 一键安装。"
		results["bun"] = r
	}

	// Run deeper diagnostics in parallel.
	go c.checkBunIntegrity(results)

	return results
}

// checkBunIntegrity runs additional diagnostics when bun is found.
func (c *Checker) checkBunIntegrity(results map[string]model.RuntimeInfo) {
	info := results["bun"]

	// Only check if bun was found via LookPath and we have a path.
	if !info.Available || info.Path == "" {
		return
	}

	home, _ := os.UserHomeDir()
	bunBinDir := filepath.Join(home, ".bun", "bin")
	expectedBunExe := filepath.Join(bunBinDir, "bun.exe")

	// Check if bun.exe exists at the standard location.
	if _, err := os.Stat(expectedBunExe); os.IsNotExist(err) {
		// bun is available via PATH (e.g. npm shim) but not at ~/.bun/bin/bun.exe.
		// This means bun-installed binary wrappers (marcus-img2ascii.exe etc.)
		// will fail because they expect bun.exe next to them.
		info.Hint = "bun 已安装，但 bun.exe 不在标准位置 (~/.bun/bin/)。\n通过 npm 安装的 bun 生成的二进制包装器可能无法运行。\n建议重新安装 bun：\n  powershell -c \"irm bun.sh/install.ps1 | iex\""
		results["bun"] = info
		return
	}

	// Check global node_modules directory.
	globalDir := filepath.Join(home, ".bun", "install", "global", "node_modules")
	if _, err := os.Stat(globalDir); os.IsNotExist(err) {
		info.Hint = "bun 全局 node_modules 目录不存在。\n请运行 'bun install -g <package>' 安装工具包。"
		results["bun"] = info
		return
	}
}

func CheckBunHealth() model.ToolHealth {
	home, _ := os.UserHomeDir()
	bunBin := filepath.Join(home, ".bun", "bin")
	bunExe := filepath.Join(bunBin, "bun.exe")

	if _, err := os.Stat(bunExe); os.IsNotExist(err) {
		return model.ToolHealth{
			Healthy: false,
			Error:   "bun 运行时未正确安装",
			Hint: "bun.exe 不在 ~/.bun/bin/ 目录中。\n" +
				"通过 npm 安装的 bun 生成的二进制包装器无法在此环境下运行。\n" +
				"解决方案：\n" +
				"  1. 卸载 npm 安装的 bun: npm uninstall -g bun\n" +
				"  2. 使用官方安装脚本重新安装:\n" +
				"     powershell -c \"irm bun.sh/install.ps1 | iex\"",
		}
	}

	// Check if bun version can be queried.
	out, err := exec.Command(bunExe, "--version").Output()
	if err != nil {
		return model.ToolHealth{
			Healthy: false,
			Error:   "bun 无法正常执行",
			Hint:    "bun.exe 存在但无法运行。请尝试重新安装 bun。",
		}
	}

	_ = strings.TrimSpace(string(out))

	// Check global node_modules.
	globalDir := filepath.Join(home, ".bun", "install", "global", "node_modules")
	if _, err := os.Stat(globalDir); os.IsNotExist(err) {
		return model.ToolHealth{
			Healthy: false,
			Error:   "bun 全局 node_modules 不存在",
			Hint:    "请运行 'bun install -g <package>' 来安装全局包。",
		}
	}

	return model.ToolHealth{Healthy: true}
}

func parseVersion(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "Python ")
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "uv ")
	v = strings.TrimPrefix(v, "bun ")
	return strings.TrimSpace(v)
}
