package main

import (
	"bajiudb/internal/datafiles"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) TransferFormats() []datafiles.Format { return datafiles.Formats(a.ctx) }
func (a *App) SelectTransferFile(format string) (string, error) {
	if !datafiles.ValidFormat(format) || format == "odbc" {
		return "", errors.New("请选择文件格式")
	}
	return a.desktop.Dialog.OpenFile().SetTitle("选择导入文件").AddFilter(strings.ToUpper(format)+" 文件", "*"+datafiles.Extension(format)).AttachToWindow(a.desktop.Window.Current()).PromptForSingleSelection()
}
func (a *App) InspectTransferSource(task string, options datafiles.Options) ([]string, error) {
	ctx, finish, err := a.adminTaskContext(task)
	if err != nil {
		return nil, err
	}
	defer finish()
	return datafiles.Inspect(ctx, options)
}
func (a *App) ReadTransferSource(task string, options datafiles.Options) (datafiles.Data, error) {
	ctx, finish, err := a.adminTaskContext(task)
	if err != nil {
		return datafiles.Data{}, err
	}
	defer finish()
	return datafiles.Read(ctx, options)
}
func (a *App) SaveTransferData(task string, options datafiles.Options, data datafiles.Data) (string, error) {
	ctx, finish, err := a.adminTaskContext(task)
	if err != nil {
		return "", err
	}
	defer finish()
	if err = datafiles.Validate(data); err != nil {
		return "", err
	}
	if !datafiles.ValidFormat(options.Format) {
		return "", errors.New("数据格式无效")
	}
	if options.Format == "odbc" {
		if err = datafiles.Write(ctx, options, data); err != nil {
			return "", err
		}
		return "ODBC / " + options.Table, nil
	}
	path, err := a.desktop.Dialog.SaveFile().SetFilename(datafiles.SafeFilename(options.Table, options.Format)).AddFilter(strings.ToUpper(options.Format)+" 文件", "*"+datafiles.Extension(options.Format)).AttachToWindow(a.desktop.Window.Current()).PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(path), datafiles.Extension(options.Format)) {
		return "", errors.New("文件扩展名与所选格式不一致")
	}
	return saveTransferFile(ctx, path, options, data)
}

type TransferExportResult struct {
	Path string `json:"path"`
	Rows int64  `json:"rows"`
}

// ExportTransferTable reads and writes in Go; no table data crosses the WebView.
func (a *App) ExportTransferTable(task, id, database, table string, options datafiles.Options) (TransferExportResult, error) {
	if a.startupErr != nil {
		return TransferExportResult{}, a.startupErr
	}
	ctx, finish, err := a.adminTaskContext(task)
	if err != nil {
		return TransferExportResult{}, err
	}
	defer finish()
	choose := func() (string, error) {
		return a.desktop.Dialog.SaveFile().SetFilename(datafiles.SafeFilename(options.Table, options.Format)).AddFilter(strings.ToUpper(options.Format)+" 文件", "*"+datafiles.Extension(options.Format)).AttachToWindow(a.desktop.Window.Current()).PromptForSingleSelection()
	}
	return exportTransferTable(ctx, options, choose, func(header func([]string) error, row func([]*string) error) error {
		return a.service.ExportTable(ctx, id, database, table, header, row)
	})
}
func exportTransferTable(ctx context.Context, options datafiles.Options, choose func() (string, error), source datafiles.Source) (TransferExportResult, error) {
	result := TransferExportResult{}
	if !datafiles.ValidFormat(options.Format) {
		return result, errors.New("数据格式无效")
	}
	if options.Format == "odbc" {
		count, err := datafiles.WriteStream(ctx, options, source)
		if err != nil {
			return result, err
		}
		return TransferExportResult{Path: "ODBC / " + options.Table, Rows: count}, nil
	}
	path, err := choose()
	if err != nil || path == "" {
		return result, err
	}
	if !strings.EqualFold(filepath.Ext(path), datafiles.Extension(options.Format)) {
		return result, errors.New("文件扩展名与所选格式不一致")
	}
	var count int64
	saved, err := publishTransferFile(ctx, path, options, func(o datafiles.Options) error {
		var e error
		count, e = datafiles.WriteStream(ctx, o, source)
		return e
	})
	if err != nil {
		return result, err
	}
	return TransferExportResult{Path: saved, Rows: count}, nil
}
func saveTransferFile(ctx context.Context, path string, options datafiles.Options, data datafiles.Data) (string, error) {
	return publishTransferFile(ctx, path, options, func(o datafiles.Options) error { return datafiles.Write(ctx, o, data) })
}
func publishTransferFile(ctx context.Context, path string, options datafiles.Options, write func(datafiles.Options) error) (string, error) {
	stage, err := os.MkdirTemp(filepath.Dir(path), ".bajiu-transfer-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	options.Path = filepath.Join(stage, "data"+datafiles.Extension(options.Format))
	if err = write(options); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	f, err := os.OpenFile(options.Path, os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err = os.Rename(options.Path, path); err != nil {
		return "", err
	}
	return path, nil
}
