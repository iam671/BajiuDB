package datafiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxRows = 10000
const MaxColumns = 1024
const MaxBytes = 8 << 20
const MaxFileBytes = 64 << 20

type Data struct {
	Columns []string    `json:"columns"`
	Rows    [][]*string `json:"rows"`
}
type Options struct {
	Format           string `json:"format"`
	Path             string `json:"path"`
	Table            string `json:"table"`
	Delimiter        string `json:"delimiter"`
	ConnectionString string `json:"connectionString"`
	ProtectFormulas  bool   `json:"protectFormulas"`
}
type Format struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	Note      string `json:"note"`
}

func Formats(ctx context.Context) []Format {
	legacy, odbc, note := driverCapabilities(ctx)
	odbcNote := "使用已安装的 ODBC 驱动及连接字符串；导出只创建新表，不覆盖已有表"
	if !odbc {
		odbcNote = note
	}
	return []Format{
		{"txt", "TXT 文本", true, "UTF-8，支持制表符、逗号、分号或竖线分隔"},
		{"csv", "CSV", true, "UTF-8，首行为字段名；\\N 表示 NULL"},
		{"json", "JSON", true, "对象数组，或包含 columns/rows 的数据对象；数字按原文读取"},
		{"xml", "XML", true, "行/字段 XML；支持本工具格式及简单 row/record 元素"},
		{"xls", "Excel 97–2003 (.xls)", legacy, note + "；\\N 表示 NULL，\\E 表示空字符串，与标记相同的字面值前添加一个反斜杠"},
		{"xlsx", "Excel 工作簿 (.xlsx)", true, "可选工作表，首行为字段名；导出文本单元格，避免精度丢失和公式执行"},
		{"mdb", "Access (.mdb)", legacy, note},
		{"accdb", "Access (.accdb)", legacy, note},
		{"odbc", "ODBC 数据源", odbc, odbcNote},
		{"dbf", "dBASE (.dbf)", legacy, note + "；导出字段名为 ASCII 且不超过 10 字节、值使用系统代码页且不超过 254 字节，不含 NULL"},
		{"db", "SQLite 数据库 (.db)", true, "可选数据表；导出为新数据库，使用 TEXT/NULL 保留原值"},
	}
}
func native(format string) bool {
	switch format {
	case "txt", "csv", "json", "xml", "xlsx", "db":
		return true
	}
	return false
}
func ValidFormat(format string) bool {
	return native(format) || format == "xls" || format == "mdb" || format == "accdb" || format == "odbc" || format == "dbf"
}
func validateSource(o Options) error {
	if !ValidFormat(o.Format) {
		return errors.New("请选择有效的数据格式")
	}
	if o.Format == "odbc" {
		return nil
	}
	info, err := os.Stat(o.Path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxFileBytes {
		return errors.New("请选择 64 MB 以内的普通数据文件")
	}
	return nil
}
func Inspect(ctx context.Context, o Options) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateSource(o); err != nil {
		return nil, err
	}
	switch o.Format {
	case "xlsx":
		return workbookSheets(o.Path)
	case "db":
		return sqliteTables(ctx, o.Path)
	case "txt", "csv", "json", "xml":
		return []string{"数据"}, nil
	default:
		return driverInspect(ctx, o)
	}
}
func Read(ctx context.Context, o Options) (Data, error) {
	if err := ctx.Err(); err != nil {
		return Data{}, err
	}
	if err := validateSource(o); err != nil {
		return Data{}, err
	}
	var data Data
	var err error
	switch o.Format {
	case "xlsx":
		data, err = readWorkbook(o)
	case "db":
		data, err = readSQLite(ctx, o)
	case "txt", "csv", "json", "xml":
		var raw []byte
		raw, err = readTextFile(o.Path)
		if err == nil {
			data, err = decodeText(raw, o)
		}
	default:
		data, err = driverRead(ctx, o)
	}
	if err != nil {
		return Data{}, err
	}
	if err = Validate(data); err != nil {
		return Data{}, err
	}
	return data, nil
}

// Write writes only to a new staging file. The caller publishes it after success.
func Write(ctx context.Context, o Options, data Data) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ValidFormat(o.Format) {
		return errors.New("数据格式无效")
	}
	if err := Validate(data); err != nil {
		return err
	}
	switch o.Format {
	case "xlsx":
		return writeWorkbook(o.Path, data)
	case "db":
		return writeSQLite(ctx, o, data)
	case "txt", "csv", "json", "xml":
		raw, err := encodeText(data, o)
		if err != nil {
			return err
		}
		if len(raw) > MaxBytes {
			return errors.New("导出内容超过 8 MB，请分批导出")
		}
		return os.WriteFile(o.Path, raw, 0600)
	default:
		return driverWrite(ctx, o, data)
	}
}
func Validate(d Data) error {
	if len(d.Columns) == 0 || len(d.Columns) > MaxColumns || len(d.Rows) > MaxRows || len(d.Rows)*len(d.Columns) > 200000 {
		return errors.New("数据超限：最多 10,000 行、1,024 列、200,000 个单元格")
	}
	seen := map[string]bool{}
	bytes := 0
	for _, col := range d.Columns {
		if col == "" || !utf8.ValidString(col) || strings.ContainsRune(col, 0) || seen[col] {
			return errors.New("字段名不能为空、包含 NUL 或重复")
		}
		seen[col] = true
		bytes += len(col)
		if bytes > MaxBytes {
			return errors.New("字段名内容超过 8 MB")
		}
	}
	for _, row := range d.Rows {
		if len(row) != len(d.Columns) {
			return errors.New("每行列数必须与字段名一致")
		}
		for _, cell := range row {
			if cell != nil {
				if !utf8.ValidString(*cell) {
					return errors.New("数据包含无效 UTF-8 文本")
				}
				bytes += len(*cell)
			}
		}
		if bytes > MaxBytes {
			return errors.New("数据内容超过 8 MB，请拆分文件")
		}
	}
	return nil
}
func readTextFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err == nil && len(data) > MaxBytes {
		err = errors.New("文本数据超过 8 MB")
	}
	return data, err
}
func text(value string) *string { return &value }
func cellText(cell *string) string {
	if cell == nil {
		return `\N`
	}
	if strings.HasPrefix(*cell, `\`) && strings.TrimLeft(*cell, `\`) == "N" {
		return `\` + *cell
	}
	return *cell
}
func textCell(value string) *string {
	if value == `\N` {
		return nil
	}
	if strings.HasPrefix(value, `\\`) && strings.TrimLeft(value, `\`) == "N" {
		value = value[1:]
	}
	return text(value)
}
func sourceTable(o Options, names []string) (string, error) {
	if o.Table == "" && len(names) > 0 {
		return names[0], nil
	}
	for _, name := range names {
		if name == o.Table {
			return name, nil
		}
	}
	return "", fmt.Errorf("找不到所选表或工作表：%s", o.Table)
}
func Extension(format string) string {
	if !ValidFormat(format) || format == "odbc" {
		return ""
	}
	return "." + format
}
func SafeFilename(name, format string) string {
	name = strings.TrimRight(strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name), ". ")
	if name == "" {
		name = "data"
	}
	return "BajiuDB-" + filepath.Base(name) + Extension(format)
}
