package main

import (
	"context"

	"Marcus/internal/app"
)

type App struct {
	impl *app.App
}

func NewApp() *App {
	return &App{impl: app.New()}
}

func (a *App) startup(ctx context.Context) {
	a.impl.Startup(ctx)
}

func (a *App) shutdown(ctx context.Context) {
	a.impl.Shutdown(ctx)
}

func (a *App) OpenFileDialog(filter string) (string, error) {
	return a.impl.OpenFileDialog(filter)
}

func (a *App) OpenDirectoryDialog() (string, error) {
	return a.impl.OpenDirectoryDialog()
}

func (a *App) GetAppLogs(count int) ([]string, error) {
	return a.impl.GetAppLogs(count)
}
