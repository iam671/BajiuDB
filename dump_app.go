package main

import (
	"bajiudb/internal/connections"
	"errors"
	"fmt"
	"os"
	"strings"
)

// DumpDatabaseToDirectory asks for the destination before reading any database data.
func (a *App) DumpDatabaseToDirectory(task, id, database string, includeData bool) (string, error) {
	if a.startupErr != nil {
		return "", a.startupErr
	}
	if a.desktop == nil {
		return "", errors.New("请在桌面程序中选择转储目录")
	}
	directory, err := a.desktop.Dialog.OpenFile().SetTitle("选择 SQL 转储保存目录").CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true).AttachToWindow(a.mainWindow).PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return dumpToDirectory(directory, database, includeData, func() (connections.AdminResult, error) {
		return a.DatabaseAdmin(task, id, database, connections.AdminRequest{Operation: "dump", IncludeData: includeData})
	})
}

func dumpToDirectory(directory, database string, includeData bool, read func() (connections.AdminResult, error)) (string, error) {
	if directory == "" {
		return "", nil
	}
	info, err := os.Stat(directory)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("请选择保存目录")
	}
	result, err := read()
	if err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", errors.New(result.Error)
	}
	if len(result.Text) > 64<<20 {
		return "", errors.New("转储文件超过 64 MB")
	}
	name := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, database)
	mode := "structure"
	if includeData {
		mode = "structure-data"
	}
	// A unique file prevents silently overwriting an earlier backup.
	file, err := os.CreateTemp(directory, "BajiuDB-"+name+"-"+mode+"-*.sql")
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(file.Name())
		}
	}()
	if _, err = file.WriteString(result.Text); err != nil {
		return "", fmt.Errorf("写入转储文件失败：%w", err)
	}
	if err = file.Sync(); err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	complete = true
	return file.Name(), nil
}
