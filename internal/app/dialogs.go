package app

import (
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ─── File Dialogs ────────────────────────────────────────────

func (a *App) OpenFileDialog(filter string) (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) OpenDirectoryDialog() (string, error) {
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择目录",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}
