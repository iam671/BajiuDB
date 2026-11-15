package main

import (
	"bajiudb/internal/connections"
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
	"path/filepath"
)

type App struct {
	ctx        context.Context
	service    *connections.Service
	configPath string
	startupErr error
}

func NewApp() *App {
	dir, err := os.UserConfigDir()
	app := &App{startupErr: err}
	if err == nil {
		app.configPath = filepath.Join(dir, "BajiuDB", "connections.json")
		app.service = connections.New(connections.FileStore{Path: app.configPath}, connections.SystemSecrets{}, connections.MySQLDialer{})
	}
	return app
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) shutdown(context.Context) {
	if a.service != nil {
		a.service.Close()
	}
}
func (a *App) ListConnections() ([]connections.Profile, error) {
	if a.startupErr != nil {
		return nil, a.startupErr
	}
	return a.service.List()
}
func (a *App) SaveConnection(input connections.Input) (connections.Profile, error) {
	if a.startupErr != nil {
		return connections.Profile{}, a.startupErr
	}
	return a.service.Save(input)
}
func (a *App) DeleteConnection(id string, revision int) error {
	if a.startupErr != nil {
		return a.startupErr
	}
	return a.service.Delete(id, revision)
}
func (a *App) TestConnection(input connections.Input) (connections.TestResult, error) {
	if a.startupErr != nil {
		return connections.TestResult{}, a.startupErr
	}
	return a.service.Test(input)
}
func (a *App) Connect(input connections.Input) (connections.SessionInfo, error) {
	if a.startupErr != nil {
		return connections.SessionInfo{}, a.startupErr
	}
	return a.service.Connect(input)
}
func (a *App) Disconnect(id string) {
	if a.service != nil {
		a.service.Disconnect(id)
	}
}
func (a *App) ListSessions() []connections.SessionInfo {
	if a.service == nil {
		return []connections.SessionInfo{}
	}
	return a.service.Sessions()
}
func (a *App) RefreshDatabases(id string) (connections.SessionInfo, error) {
	if a.startupErr != nil {
		return connections.SessionInfo{}, a.startupErr
	}
	return a.service.Refresh(id)
}
func (a *App) ConfigPath() string { return a.configPath }
func (a *App) SelectCAFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择 CA 证书", Filters: []runtime.FileFilter{{DisplayName: "PEM 证书", Pattern: "*.pem;*.crt;*.cer"}}})
}
