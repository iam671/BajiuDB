package main

import (
	"bajiudb/internal/connections"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// DumpDatabaseToFile asks for the filename before reading any database data.
func (a *App) DumpDatabaseToFile(task, id, database string, includeData bool) (string, error) {
	if a.startupErr != nil {
		return "", a.startupErr
	}
	if a.desktop == nil {
		return "", errors.New("请在桌面程序中选择转储文件")
	}
	path, err := a.desktop.Dialog.SaveFile().SetFilename(dumpFilename(database)).AddFilter("SQL 脚本文件 (*.sql)", "*.sql").CanCreateDirectories(true).AttachToWindow(a.mainWindow).PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return dumpToFile(path, func() (connections.AdminResult, error) {
		return a.DatabaseAdmin(task, id, database, connections.AdminRequest{Operation: "dump", IncludeData: includeData})
	})
}

func dumpToFile(path string, read func() (connections.AdminResult, error)) (string, error) {
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("请选择有效的保存位置")
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return "", errors.New("请选择 SQL 文件名，不能选择目录")
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
	return writeDatabaseReport(path, result.Text)
}

func dumpFilename(database string) string {
	name := strings.TrimRight(strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, database), ". ")
	if name == "" {
		name = "database"
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
		name = "_" + name
	}
	return name + ".sql"
}
