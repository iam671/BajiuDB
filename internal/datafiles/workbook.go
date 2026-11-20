package datafiles

import (
	"errors"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func openWorkbook(path string) (*excelize.File, error) {
	return excelize.OpenFile(path, excelize.Options{RawCellValue: true, UnzipSizeLimit: 64 << 20, UnzipXMLSizeLimit: 8 << 20})
}
func workbookSheets(path string) ([]string, error) {
	f, err := openWorkbook(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names := f.GetSheetList()
	if len(names) > 1000 {
		return nil, errors.New("工作表超过 1,000 张")
	}
	return names, nil
}
func readWorkbook(o Options) (Data, error) {
	f, err := openWorkbook(o.Path)
	if err != nil {
		return Data{}, err
	}
	defer f.Close()
	sheet, err := sourceTable(o, f.GetSheetList())
	if err != nil {
		return Data{}, err
	}
	rows, err := f.Rows(sheet)
	if err != nil {
		return Data{}, err
	}
	defer rows.Close()
	d := Data{Rows: [][]*string{}}
	number := 0
	bytes := 0
	for rows.Next() {
		number++
		values, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return Data{}, err
		}
		if len(values) > MaxColumns {
			return Data{}, errors.New("Excel 列数超过 1,024")
		}
		if number == 1 {
			d.Columns = values
			continue
		}
		if number > MaxRows+1 {
			return Data{}, errors.New("Excel 超过 10,000 行，已停止而非截断")
		}
		if len(values) > len(d.Columns) {
			return Data{}, errors.New("Excel 存在没有字段名的数据列")
		}
		row := make([]*string, len(d.Columns))
		for i := range row {
			value := ""
			if i < len(values) {
				value = values[i]
			}
			axis, _ := excelize.CoordinatesToCellName(i+1, number)
			value, err = workbookValue(f, sheet, axis, value)
			if err != nil {
				return Data{}, err
			}
			row[i] = textCell(value)
			bytes += len(value)
		}
		d.Rows = append(d.Rows, row)
		if bytes > MaxBytes || len(d.Rows)*len(d.Columns) > 200000 {
			return Data{}, errors.New("Excel 数据内容超过导入上限")
		}
	}
	return d, rows.Error()
}
func workbookValue(f *excelize.File, sheet, axis, raw string) (string, error) {
	if raw == "" {
		return raw, nil
	}
	kind, err := f.GetCellType(sheet, axis)
	if err != nil {
		return "", err
	}
	if kind == excelize.CellTypeError {
		return "", errors.New("Excel 含错误单元格：" + axis)
	}
	if kind == excelize.CellTypeBool {
		if raw == "1" {
			return "true", nil
		}
		return "false", nil
	}
	if kind == excelize.CellTypeSharedString || kind == excelize.CellTypeInlineString {
		return raw, nil
	}
	index, err := f.GetCellStyle(sheet, axis)
	if err != nil {
		return "", err
	}
	style, err := f.GetStyle(index)
	if err != nil {
		return "", err
	}
	date := style.NumFmt >= 14 && style.NumFmt <= 22 || style.NumFmt >= 27 && style.NumFmt <= 36 || style.NumFmt >= 45 && style.NumFmt <= 47 || style.NumFmt >= 50 && style.NumFmt <= 58
	if style.CustomNumFmt != nil {
		format := dateFormatCode(*style.CustomNumFmt)
		date = date || strings.ContainsAny(format, "ydhs")
	}
	if !date {
		return raw, nil
	}
	serial, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return raw, nil
	}
	props, err := f.GetWorkbookProps()
	if err != nil {
		return "", err
	}
	date1904 := props.Date1904 != nil && *props.Date1904
	value, err := excelize.ExcelDateToTime(serial, date1904)
	if err != nil {
		return "", err
	}
	if style.NumFmt >= 18 && style.NumFmt <= 21 || style.NumFmt >= 45 && style.NumFmt <= 47 {
		return value.Format("15:04:05.999999"), nil
	}
	if style.NumFmt >= 14 && style.NumFmt <= 17 {
		return value.Format("2006-01-02"), nil
	}
	return value.Format("2006-01-02 15:04:05.999999"), nil
}
func writeWorkbook(path string, d Data) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	for i, col := range d.Columns {
		axis, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(sheet, axis, col); err != nil {
			return err
		}
	}
	for n, row := range d.Rows {
		for i, cell := range row {
			axis, _ := excelize.CoordinatesToCellName(i+1, n+2)
			if err := f.SetCellStr(sheet, axis, cellText(cell)); err != nil {
				return err
			}
		}
	}
	return f.SaveAs(path)
}

func dateFormatCode(format string) string {
	var result strings.Builder
	quoted, bracket, escaped := false, false, false
	for _, r := range strings.ToLower(format) {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if r == '[' {
			bracket = true
			continue
		}
		if r == ']' {
			bracket = false
			continue
		}
		if !bracket {
			result.WriteRune(r)
		}
	}
	return result.String()
}
