package runtime

import (
	"os/exec"
	"strings"
	"sync"

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
			out, err := exec.Command(t.Command, t.Args...).Output()
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
	wg.Wait()

	return results
}

func parseVersion(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "Python ")
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "uv ")
	v = strings.TrimPrefix(v, "bun ")
	return strings.TrimSpace(v)
}
