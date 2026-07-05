package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ─── App Logging ─────────────────────────────────────────────

func (a *App) writeLog(level, msg string) {
	a.logFileMu.Lock()
	defer a.logFileMu.Unlock()
	if a.logFile != nil {
		fmt.Fprintf(a.logFile, "[%s] %s: %s\n", time.Now().Format(time.DateTime), level, msg)
	}
}

func (a *App) GetAppLogs(count int) ([]string, error) {
	if a.cfgPath == "" {
		return nil, nil
	}
	logPath := filepath.Join(filepath.Dir(a.cfgPath), "app.log")
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return lines, nil
}
