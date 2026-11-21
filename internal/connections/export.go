package connections

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

type tableExporter interface {
	ExportTable(context.Context, string, string, func([]string) error, func([]*string) error) error
}

// ExportTable bypasses interactive query preview limits. Only table identifiers
// are accepted; arbitrary client SQL never enters this read-only export path.
func (s *Service) ExportTable(ctx context.Context, id, database, table string, header func([]string) error, row func([]*string) error) error {
	if err := validDatabaseName(database); err != nil {
		return err
	}
	if table == "" || strings.ContainsRune(table, 0) {
		return errors.New("表名无效")
	}
	s.mu.Lock()
	live, ok := s.sessions[id]
	epoch := s.epoch[id]
	s.mu.Unlock()
	if !ok {
		return errors.New("请先打开连接")
	}
	allowed := false
	for _, name := range live.info.Databases {
		if name == database {
			allowed = true
		}
	}
	if !allowed {
		return errors.New("数据库不在当前账户可访问的列表中")
	}
	exporter, ok := live.db.(tableExporter)
	if !ok {
		return errors.New("此连接不支持表导出")
	}
	check := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.epoch[id] != epoch {
			return errors.New("导出期间连接已变化，请重试")
		}
		return ctx.Err()
	}
	err := exporter.ExportTable(ctx, database, table, func(names []string) error {
		if e := check(); e != nil {
			return e
		}
		return header(names)
	}, func(values []*string) error {
		if e := check(); e != nil {
			return e
		}
		return row(values)
	})
	if err != nil {
		return err
	}
	return check()
}

func exportProjection(columns [][]*string) (string, []string, error) {
	var names, expressions []string
	for _, column := range columns {
		if len(column) < 2 || column[0] == nil || column[1] == nil {
			return "", nil, errors.New("无法读取导出字段")
		}
		name, typeName := *column[0], strings.ToLower(*column[1])
		q := quoteIdentifier(name)
		expression := q
		base := strings.FieldsFunc(typeName, func(r rune) bool { return r == '(' || r == ' ' })
		if len(base) == 0 {
			return "", nil, errors.New("字段类型无效")
		}
		switch base[0] {
		case "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob", "bit":
			h := "HEX(" + q + ")"
			expression = "CONCAT('0x',LPAD(" + h + ",LENGTH(" + h + ")+MOD(LENGTH(" + h + "),2),'0')) AS " + q
		case "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring", "multipolygon", "geometrycollection":
			expression = "CONCAT('SRID=',ST_SRID(" + q + "),';WKB=0x',HEX(ST_AsWKB(" + q + "))) AS " + q
		}
		names = append(names, name)
		expressions = append(expressions, expression)
	}
	if len(names) == 0 {
		return "", nil, errors.New("表没有可导出的字段")
	}
	return strings.Join(expressions, ", "), names, nil
}

func (d *mysqlDatabase) ExportTable(ctx context.Context, database, table string, header func([]string) error, row func([]*string) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return explainMySQL(err)
	}
	defer conn.Close()
	defer conn.Raw(func(any) error { return driver.ErrBadConn })
	// One unbounded SELECT on a dedicated connection: no OFFSET pages, no 1,000-row
	// preview collector, and one consistent statement snapshot for transactional tables.
	qualified := quoteIdentifier(database) + "." + quoteIdentifier(table)
	meta, err := queryResult(ctx, conn, "SHOW FULL COLUMNS FROM "+qualified, 10000)
	if err != nil {
		return err
	}
	if meta.Truncated {
		return errors.New("导出字段列表不完整")
	}
	projection, names, err := exportProjection(meta.Rows)
	if err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT "+projection+" FROM "+qualified)
	if err != nil {
		return explainMySQL(err)
	}
	defer func() { cancel(); rows.Close() }()
	actual, err := rows.Columns()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, names) {
		return errors.New("导出期间表结构已变化，请重试")
	}
	if err = header(names); err != nil {
		return err
	}
	for rows.Next() {
		raw := make([]sql.RawBytes, len(names))
		dest := make([]any, len(names))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err = rows.Scan(dest...); err != nil {
			return err
		}
		values := make([]*string, len(names))
		for i, b := range raw {
			if b != nil {
				if !utf8.Valid(b) {
					return errors.New("字段不是有效文本，请核对字段类型后重试")
				}
				s := string(b)
				values[i] = &s
			}
		}
		if err = row(values); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (d *httpDatabase) ExportTable(ctx context.Context, database, table string, header func([]string) error, row func([]*string) error) error {
	call := func(sql string) (QueryResult, error) {
		r, e := d.Admin(ctx, database, AdminRequest{Operation: "query", SQL: sql})
		if e != nil {
			return QueryResult{}, e
		}
		if r.Error != "" {
			return QueryResult{}, errors.New(r.Error)
		}
		if len(r.Results) != 1 {
			return QueryResult{}, errors.New("中转没有返回完整结果")
		}
		return r.Results[0], nil
	}
	meta, err := call("SHOW FULL COLUMNS FROM " + quoteIdentifier(table))
	if err != nil {
		return err
	}
	if meta.Truncated {
		return errors.New("中转字段列表不完整")
	}
	projection, names, err := exportProjection(meta.Rows)
	if err != nil {
		return err
	}
	if err = header(names); err != nil {
		return err
	}
	// PHP requests cannot share a live cursor. Use deterministic ordered pages;
	// concurrent source writes may change that multi-request view (documented in UI).
	order := make([]string, len(names))
	for i := range names {
		order[i] = fmt.Sprint(i + 1)
	}
	var offset int64
	batch := 500
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		q, e := call("SELECT " + projection + " FROM " + quoteIdentifier(table) + " ORDER BY " + strings.Join(order, ",") + fmt.Sprintf(" LIMIT %d OFFSET %d", batch, offset))
		if e != nil {
			// Only retry explicit response-size failures; never hide a SQL/permission error.
			if batch > 1 && errors.Is(e, errTunnelResponseTooLarge) {
				batch = (batch + 1) / 2
				continue
			}
			return e
		}
		if q.Truncated || !reflect.DeepEqual(q.Columns, names) {
			return errors.New("中转导出批次不完整或结构已变化")
		}
		for _, values := range q.Rows {
			if err = row(values); err != nil {
				return err
			}
		}
		if len(q.Rows) < batch {
			return nil
		}
		offset += int64(len(q.Rows))
	}
}
