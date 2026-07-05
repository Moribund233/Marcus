package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DBPath          string `json:"db_path"`
	ToolsDir        string `json:"tools_dir"`
	WindowWidth     int    `json:"window_width"`
	WindowHeight    int    `json:"window_height"`
	HealthTimeout   int    `json:"health_timeout_seconds"`
	DefaultCPU      int    `json:"default_cpu_percent"`
	DefaultMemory   int    `json:"default_memory_mb"`
	DefaultTimeout  int    `json:"default_timeout_seconds"`
	AutoScan        bool   `json:"auto_scan"`
	TerminateOnExit bool   `json:"terminate_on_exit"`
	Theme           string `json:"theme"`
	Language        string `json:"language"`
	StoreIndexURL   string `json:"store_index_url"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".marcus")

	return &Config{
		DBPath:          filepath.Join(dataDir, "marcus.db"),
		ToolsDir:        filepath.Join(dataDir, "bin"),
		WindowWidth:     1024,
		WindowHeight:    768,
		HealthTimeout:   15,
		DefaultCPU:      0,
		DefaultMemory:   0,
		DefaultTimeout:  0,
		AutoScan:        true,
		TerminateOnExit: true,
		Theme:           "dark",
		Language:        "zh-CN",
		StoreIndexURL:   "https://raw.githubusercontent.com/Moribund233/Marcus-plugins/master/index.json",
	}
}

func (c *Config) ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	if err := c.ensureDir(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
