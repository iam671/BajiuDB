package main

import (
	"bajiudb/internal/connections"
	"context"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

var adminTasks = struct {
	sync.Mutex
	cancel map[string]context.CancelFunc
}{cancel: map[string]context.CancelFunc{}}

func (a *App) DatabaseAdmin(task, id, database string, request connections.AdminRequest) (connections.AdminResult, error) {
	if a.startupErr != nil {
		return connections.AdminResult{}, a.startupErr
	}
	if task == "" || len(task) > 128 {
		return connections.AdminResult{}, errors.New("操作标识无效")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	adminTasks.Lock()
	if _, exists := adminTasks.cancel[task]; exists {
		adminTasks.Unlock()
		cancel()
		return connections.AdminResult{}, errors.New("操作标识重复")
	}
	adminTasks.cancel[task] = cancel
	adminTasks.Unlock()
	defer func() { cancel(); adminTasks.Lock(); delete(adminTasks.cancel, task); adminTasks.Unlock() }()
	return a.service.Admin(ctx, id, database, request)
}
func (a *App) CancelDatabaseTask(task string) {
	adminTasks.Lock()
	cancel := adminTasks.cancel[task]
	adminTasks.Unlock()
	if cancel != nil {
		cancel()
	}
}

type SQLFile struct {
	Path       string `json:"path"`
	Text       string `json:"text"`
	Statements int    `json:"statements"`
}

func (a *App) OpenSQLFile() (SQLFile, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择要运行的 SQL 文件", Filters: []runtime.FileFilter{{DisplayName: "SQL 文件（UTF-8）", Pattern: "*.sql"}}})
	if err != nil || path == "" {
		return SQLFile{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return SQLFile{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if err != nil {
		return SQLFile{}, err
	}
	if len(data) > 8<<20 || !utf8.Valid(data) {
		return SQLFile{}, errors.New("请选择 8 MB 以内的 UTF-8 SQL 文件")
	}
	statements, err := connections.SplitSQL(string(data))
	if err != nil {
		return SQLFile{}, err
	}
	return SQLFile{path, string(data), len(statements)}, nil
}
func (a *App) SaveDatabaseReport(name, text string) (string, error) {
	if len(text) > 64<<20 {
		return "", errors.New("文件超过 64 MB")
	}
	name = filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".sql" && ext != ".html" && ext != ".svg" {
		return "", errors.New("不支持的导出格式")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "保存数据库文件", DefaultFilename: name, Filters: []runtime.FileFilter{{DisplayName: "数据库文件", Pattern: "*" + ext}}})
	if err != nil || path == "" {
		return "", err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".bajiu-export-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.WriteString(text); err != nil {
		temp.Close()
		return "", err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return "", err
	}
	if err = temp.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(temp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}
