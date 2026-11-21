package datafiles

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// Source calls header once, then yields one complete row at a time. It must not
// retain yielded rows. Export limits are determined by the destination format,
// never by the import/preview limits in Validate.
type Source func(header func([]string) error, row func([]*string) error) error

type streamSink interface {
	row([]*string) error
	finish() error
	close() error
}

func WriteStream(ctx context.Context, o Options, source Source) (count int64, err error) {
	if !ValidFormat(o.Format) {
		return 0, errors.New("数据格式无效")
	}
	var columns []string
	checked := func(header func([]string) error, row func([]*string) error) error {
		return source(func(names []string) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if columns != nil || len(names) == 0 {
				return errors.New("导出字段列表无效")
			}
			seen := map[string]bool{}
			for _, name := range names {
				if name == "" || seen[name] || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
					return errors.New("导出字段名无效或重复")
				}
				seen[name] = true
			}
			columns = append([]string{}, names...)
			return header(columns)
		}, func(values []*string) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if columns == nil || len(values) != len(columns) {
				return errors.New("导出行与字段数不一致")
			}
			for _, v := range values {
				if v != nil && !utf8.ValidString(*v) {
					return errors.New("导出值包含无效 UTF-8")
				}
			}
			if e := row(values); e != nil {
				return e
			}
			count++
			return nil
		})
	}
	if !native(o.Format) {
		err = driverWriteStream(ctx, o, checked)
		return count, err
	}
	var sink streamSink
	defer func() {
		if sink != nil {
			if e := sink.close(); err == nil {
				err = e
			}
		}
	}()
	err = checked(func(names []string) error {
		var e error
		sink, e = newStreamSink(ctx, o, names)
		return e
	}, func(values []*string) error { return sink.row(values) })
	if err != nil {
		return count, err
	}
	if sink == nil {
		return count, errors.New("导出没有返回字段列表")
	}
	if err = ctx.Err(); err != nil {
		return count, err
	}
	err = sink.finish()
	return count, err
}

func newStreamSink(ctx context.Context, o Options, columns []string) (streamSink, error) {
	switch o.Format {
	case "xlsx":
		return newWorkbookStream(o.Path, columns)
	case "db":
		return newSQLiteStream(ctx, o, columns)
	default:
		return newTextStream(o, columns)
	}
}

type textStream struct {
	file      *os.File
	out       *bufio.Writer
	encoder   *xml.Encoder
	format    string
	separator rune
	protect   bool
	first     bool
}

func newTextStream(o Options, columns []string) (sink streamSink, err error) {
	f, err := os.OpenFile(o.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	s := &textStream{file: f, out: bufio.NewWriter(f), format: o.Format, protect: o.ProtectFormulas, first: true}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	switch o.Format {
	case "csv", "txt":
		s.separator, err = delimiter(o)
		if err != nil {
			return nil, err
		}
		if _, err = s.out.WriteString("\ufeff"); err != nil {
			return nil, err
		}
		err = s.delimited(columns)
	case "json":
		var raw []byte
		raw, err = json.Marshal(columns)
		if err != nil {
			return nil, err
		}
		_, err = s.out.WriteString(`{"columns":` + string(raw) + `,"rows":[`)
	case "xml":
		s.encoder = xml.NewEncoder(s.out)
		for _, token := range []xml.Token{xml.StartElement{Name: xml.Name{Local: "data"}}, xml.StartElement{Name: xml.Name{Local: "columns"}}} {
			if err = s.encoder.EncodeToken(token); err != nil {
				return nil, err
			}
		}
		for _, col := range columns {
			if err = s.encoder.EncodeElement(col, xml.StartElement{Name: xml.Name{Local: "column"}}); err != nil {
				return nil, err
			}
		}
		if err = s.encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "columns"}}); err != nil {
			return nil, err
		}
		err = s.encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "rows"}})
	}
	return s, err
}
func (s *textStream) delimited(values []string) error {
	for i, value := range values {
		if i > 0 {
			if _, e := s.out.WriteRune(s.separator); e != nil {
				return e
			}
		}
		if s.protect && formulaPrefix(value) {
			value = "'" + value
		}
		if _, e := s.out.WriteString(`"` + strings.ReplaceAll(value, `"`, `""`) + `"`); e != nil {
			return e
		}
	}
	_, err := s.out.WriteString("\r\n")
	return err
}
func formulaPrefix(value string) bool {
	v := strings.TrimLeft(value, " \t\r\n")
	return len(v) > 0 && strings.ContainsRune("=+-@", rune(v[0]))
}
func (s *textStream) row(values []*string) error {
	switch s.format {
	case "csv", "txt":
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = cellText(v)
		}
		return s.delimited(row)
	case "json":
		raw, err := json.Marshal(values)
		if err != nil {
			return err
		}
		if !s.first {
			if err = s.out.WriteByte(','); err != nil {
				return err
			}
		}
		s.first = false
		_, err = s.out.Write(raw)
		return err
	case "xml":
		row := xmlRow{}
		for _, v := range values {
			cell := xmlCell{}
			if v == nil {
				cell.Null = "true"
			} else {
				cell.Value = *v
				for _, r := range *v {
					if r < 32 && r != '\t' && r != '\r' && r != '\n' || r == 0xfffe || r == 0xffff {
						return errors.New("XML 1.0 不能表示该控制字符，请使用 JSON")
					}
				}
			}
			row.Cells = append(row.Cells, cell)
		}
		return s.encoder.EncodeElement(row, xml.StartElement{Name: xml.Name{Local: "row"}})
	}
	return errors.New("文本格式无效")
}
func (s *textStream) finish() error {
	if s.format == "json" {
		if _, e := s.out.WriteString("]}"); e != nil {
			return e
		}
	}
	if s.encoder != nil {
		for _, name := range []string{"rows", "data"} {
			if e := s.encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: name}}); e != nil {
				return e
			}
		}
		if e := s.encoder.Flush(); e != nil {
			return e
		}
	}
	return s.out.Flush()
}
func (s *textStream) close() error { return s.file.Close() }

type sqliteStream struct {
	db   *sql.DB
	tx   *sql.Tx
	stmt *sql.Stmt
	ctx  context.Context
}

func newSQLiteStream(ctx context.Context, o Options, columns []string) (sink streamSink, err error) {
	db, err := openSQLite(o.Path, false)
	if err != nil {
		return nil, err
	}
	s := &sqliteStream{db: db, ctx: ctx}
	defer func() {
		if err != nil {
			s.close()
		}
	}()
	s.tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	name := o.Table
	if name == "" {
		name = "data"
	}
	defs := make([]string, len(columns))
	for i, c := range columns {
		defs[i] = quote(c) + " TEXT"
	}
	if _, err = s.tx.ExecContext(ctx, "CREATE TABLE "+quote(name)+" ("+strings.Join(defs, ",")+")"); err != nil {
		return nil, err
	}
	s.stmt, err = s.tx.PrepareContext(ctx, "INSERT INTO "+quote(name)+" VALUES ("+strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")+")")
	return s, err
}
func (s *sqliteStream) row(row []*string) error {
	v := make([]any, len(row))
	for i, c := range row {
		if c != nil {
			v[i] = *c
		}
	}
	_, err := s.stmt.ExecContext(s.ctx, v...)
	return err
}
func (s *sqliteStream) finish() error { return s.tx.Commit() }
func (s *sqliteStream) close() error {
	if s.stmt != nil {
		s.stmt.Close()
	}
	if s.tx != nil {
		s.tx.Rollback()
	}
	return s.db.Close()
}

type workbookStream struct {
	file    *excelize.File
	writer  *excelize.StreamWriter
	path    string
	columns []string
	sheet   int
	next    int
}

func newWorkbookStream(path string, columns []string) (streamSink, error) {
	s := &workbookStream{file: excelize.NewFile(), path: path, columns: columns}
	if err := s.startSheet(); err != nil {
		s.file.Close()
		return nil, err
	}
	return s, nil
}
func (s *workbookStream) startSheet() error {
	s.sheet++
	name := fmt.Sprintf("Sheet%d", s.sheet)
	if s.sheet > 1 {
		if _, err := s.file.NewSheet(name); err != nil {
			return err
		}
	}
	writer, err := s.file.NewStreamWriter(name)
	if err != nil {
		return err
	}
	s.writer = writer
	s.next = 1
	return s.writeStrings(s.columns)
}
func (s *workbookStream) write(row []*string) error {
	values := make([]string, len(row))
	for i, v := range row {
		values[i] = cellText(v)
	}
	return s.writeStrings(values)
}
func (s *workbookStream) writeStrings(values []string) error {
	cells := make([]any, len(values))
	for i, str := range values {
		n := 0
		for _, r := range str {
			n++
			if r > 0xffff {
				n++
			}
		}
		if n > 32767 {
			return errors.New("Excel 单元格超过格式支持的 32,767 字符，请选择 JSON/CSV")
		}
		cells[i] = str
	}
	axis, err := excelize.CoordinatesToCellName(1, s.next)
	if err != nil {
		return err
	}
	if err = s.writer.SetRow(axis, cells); err != nil {
		return err
	}
	s.next++
	return nil
}
func (s *workbookStream) row(row []*string) error {
	if s.next > 1048576 {
		if err := s.writer.Flush(); err != nil {
			return err
		}
		if err := s.startSheet(); err != nil {
			return err
		}
	}
	return s.write(row)
}
func (s *workbookStream) finish() error {
	if err := s.writer.Flush(); err != nil {
		return err
	}
	return s.file.SaveAs(s.path)
}
func (s *workbookStream) close() error { return s.file.Close() }
