package datafiles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func quote(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
func openSQLite(path string, readOnly bool) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if readOnly {
		q.Set("mode", "ro")
		q.Add("_pragma", "query_only(1)")
	}
	q.Add("_pragma", "trusted_schema(0)")
	q.Add("_pragma", "busy_timeout(3000)")
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath, RawQuery: q.Encode()}
	db, err := sql.Open("sqlite", u.String())
	if err == nil {
		db.SetMaxOpenConns(1)
	}
	return db, err
}
func sqliteTables(ctx context.Context, path string) ([]string, error) {
	db, err := openSQLite(path, true)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND sql NOT LIKE 'CREATE VIRTUAL TABLE%' ORDER BY name LIMIT 1001`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if len(names) > 1000 {
		return nil, errors.New("数据库超过 1,000 张表，请拆分来源")
	}
	return names, rows.Err()
}
func readSQLite(ctx context.Context, o Options) (Data, error) {
	names, err := sqliteTables(ctx, o.Path)
	if err != nil {
		return Data{}, err
	}
	name, err := sourceTable(o, names)
	if err != nil {
		return Data{}, err
	}
	db, err := openSQLite(o.Path, true)
	if err != nil {
		return Data{}, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+quote(name)+" LIMIT 10001")
	if err != nil {
		return Data{}, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return Data{}, err
	}
	d := Data{Columns: cols, Rows: [][]*string{}}
	if len(cols) > MaxColumns {
		return Data{}, errors.New("SQLite 列数超限")
	}
	bytes := 0
	for rows.Next() {
		values := make([]any, len(cols))
		targets := make([]any, len(cols))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return Data{}, err
		}
		row := make([]*string, len(cols))
		for i, v := range values {
			var s string
			switch value := v.(type) {
			case nil:
				continue
			case string:
				s = value
			case int64:
				s = strconv.FormatInt(value, 10)
			case float64:
				s = strconv.FormatFloat(value, 'g', -1, 64)
			case bool:
				s = strconv.FormatBool(value)
			case time.Time:
				s = value.Format("2006-01-02 15:04:05.999999999")
			case []byte:
				return Data{}, errors.New("SQLite 含二进制字段，不能按文本导入")
			default:
				return Data{}, fmt.Errorf("不支持的 SQLite 值类型 %T", v)
			}
			row[i] = text(s)
			bytes += len(s)
		}
		d.Rows = append(d.Rows, row)
		if len(d.Rows) > MaxRows || bytes > MaxBytes || len(d.Rows)*len(cols) > 200000 {
			return Data{}, errors.New("SQLite 数据超过导入上限，已停止而非截断")
		}
	}
	return d, rows.Err()
}
func writeSQLite(ctx context.Context, o Options, d Data) error {
	db, err := openSQLite(o.Path, false)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	name := o.Table
	if name == "" {
		name = "data"
	}
	defs := make([]string, len(d.Columns))
	for i, c := range d.Columns {
		defs[i] = quote(c) + " TEXT"
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "CREATE TABLE "+quote(name)+" ("+strings.Join(defs, ",")+")"); err != nil {
		return err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(defs)), ",")
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO "+quote(name)+" VALUES ("+placeholders+")")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, row := range d.Rows {
		values := make([]any, len(row))
		for i, c := range row {
			if c != nil {
				values[i] = *c
			}
		}
		if _, err = stmt.ExecContext(ctx, values...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
