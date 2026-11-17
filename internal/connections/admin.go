package connections

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type AdminRequest struct {
	Operation     string   `json:"operation"`
	Name          string   `json:"name"`
	Charset       string   `json:"charset"`
	Collation     string   `json:"collation"`
	SQL           string   `json:"sql"`
	Statements    []string `json:"statements,omitempty"`
	IncludeData   bool     `json:"includeData"`
	TransactionID string   `json:"transactionId,omitempty"`
}
type QueryResult struct {
	Columns   []string    `json:"columns"`
	Rows      [][]*string `json:"rows"`
	Affected  string      `json:"affected"`
	Truncated bool        `json:"truncated"`
}
type AdminResult struct {
	Results           []QueryResult `json:"results"`
	Completed         int           `json:"completed"`
	Error             string        `json:"error"`
	Text              string        `json:"text"`
	TransactionID     string        `json:"transactionId,omitempty"`
	TransactionActive bool          `json:"transactionActive"`
}
type adminDatabase interface {
	Admin(context.Context, string, AdminRequest) (AdminResult, error)
}

func (s *Service) Admin(ctx context.Context, id, database string, request AdminRequest) (AdminResult, error) {
	s.mu.Lock()
	live, ok := s.sessions[id]
	epoch := s.epoch[id]
	s.mu.Unlock()
	if !ok {
		return AdminResult{}, errors.New("请先打开连接")
	}
	switch request.Operation {
	case "catalog", "create":
	default:
		allowed := false
		for _, name := range live.info.Databases {
			if name == database {
				allowed = true
			}
		}
		if !allowed {
			return AdminResult{}, errors.New("数据库不在当前账户可访问的列表中")
		}
	}
	if request.Operation == "create" || request.Operation == "alter" || request.Operation == "drop" {
		if err := validDatabaseName(request.Name); err != nil {
			return AdminResult{}, err
		}
		if request.Operation != "create" && request.Name != database {
			return AdminResult{}, errors.New("操作目标与所选数据库不一致")
		}
		switch strings.ToLower(request.Name) {
		case "mysql", "sys", "information_schema", "performance_schema":
			return AdminResult{}, errors.New("不能在数据库管理对话框修改或删除系统库")
		}
	}
	admin, ok := live.db.(adminDatabase)
	if !ok {
		return AdminResult{}, errors.New("此连接不支持数据库工具")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	result, err := admin.Admin(ctx, database, request)
	s.mu.Lock()
	current := s.epoch[id]
	s.mu.Unlock()
	if current != epoch {
		return result, errors.New("连接已变化；已执行的 SQL 不会自动回滚，请核对服务器状态")
	}
	return result, err
}
func validDatabaseName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > 64 || strings.ContainsAny(name, "\x00\r\n./\\") || strings.TrimSpace(name) != name {
		return errors.New("数据库名须为 1–64 字符，不能含路径字符或首尾空格")
	}
	return nil
}

var optionName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
var numericValue = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]*)?(?:[eE][+-]?[0-9]+)?$`)

func (d *mysqlDatabase) Admin(ctx context.Context, database string, request AdminRequest) (AdminResult, error) {
	if request.Operation == "transaction-begin" {
		return d.beginTableTransaction(ctx, database, request.Name)
	}
	if request.TransactionID != "" || request.Operation == "transaction-commit" || request.Operation == "transaction-rollback" {
		return d.tableTransactionAdmin(ctx, database, request)
	}
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return AdminResult{}, explainMySQL(err)
	}
	defer conn.Close()
	// SQL workspaces use a dedicated connection and close it after each batch. Session
	// variables and unfinished transactions must never leak back into the pool.
	defer conn.Raw(func(any) error { return driver.ErrBadConn })
	if request.Operation != "catalog" && request.Operation != "create" && database != "" {
		if _, err = conn.ExecContext(ctx, "USE "+quoteIdentifier(database)); err != nil {
			return AdminResult{}, explainMySQL(err)
		}
	}
	result := AdminResult{Results: []QueryResult{}}
	switch request.Operation {
	case "catalog":
		q, e := queryResult(ctx, conn, "SHOW COLLATION", 10000)
		result.Results = append(result.Results, q)
		return result, e
	case "schema":
		q, e := queryResult(ctx, conn, "SELECT DEFAULT_CHARACTER_SET_NAME,DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME="+sqlString(database), 1)
		result.Results = append(result.Results, q)
		return result, e
	case "create", "alter", "drop":
		command := strings.ToUpper(request.Operation) + " DATABASE " + quoteIdentifier(request.Name)
		if request.Operation != "drop" {
			if !optionName.MatchString(request.Charset) || !optionName.MatchString(request.Collation) {
				return result, errors.New("请选择有效字符集和排序规则")
			}
			var count int
			err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.COLLATIONS WHERE CHARACTER_SET_NAME=? AND COLLATION_NAME=?", request.Charset, request.Collation).Scan(&count)
			if err != nil {
				return result, explainMySQL(err)
			}
			if count != 1 {
				return result, errors.New("该排序规则不属于所选字符集")
			}
			command += " CHARACTER SET " + request.Charset + " COLLATE " + request.Collation
		}
		_, err = conn.ExecContext(ctx, command)
		if err != nil {
			return result, explainMySQL(err)
		}
		result.Completed = 1
		return result, nil
	case "query":
		statements, e := SplitSQL(request.SQL)
		if e != nil {
			return result, e
		}
		if len(statements) == 0 {
			return result, errors.New("请输入 SQL")
		}
		total := 0
		for _, statement := range statements {
			q, e := queryResult(ctx, conn, statement, 1000)
			if e != nil {
				result.Error = fmt.Sprintf("第 %d 条语句失败：%s", result.Completed+1, e)
				return result, nil
			}
			result.Completed++
			for _, row := range q.Rows {
				for _, value := range row {
					if value != nil {
						total += len(*value)
					}
				}
			}
			if total > 16<<20 {
				result.Error = "结果累计超过 16 MB，已停止继续执行；已完成语句不会自动回滚"
				return result, nil
			}
			if len(result.Results) < 100 {
				result.Results = append(result.Results, q)
			}
		}
		return result, nil
	case "dictionary", "model":
		for _, query := range schemaQueries(database) {
			q, e := queryResult(ctx, conn, query, 10000)
			if e != nil {
				return result, e
			}
			if q.Truncated {
				return result, errors.New("结构超过 10,000 项，无法生成完整字典或模型")
			}
			result.Results = append(result.Results, q)
		}
		return result, nil
	case "dump":
		result.Text, err = dumpSQL(ctx, conn, database, request.IncludeData)
		return result, err
	default:
		return result, errors.New("不支持的数据库操作")
	}
}
func sqlString(value string) string {
	return "CONVERT(X'" + hex.EncodeToString([]byte(value)) + "' USING utf8mb4)"
}
func schemaQueries(database string) []string {
	schema := sqlString(database)
	return []string{
		"SELECT TABLE_NAME,TABLE_TYPE,COALESCE(ENGINE,''),COALESCE(TABLE_COMMENT,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=" + schema + " ORDER BY TABLE_NAME",
		"SELECT TABLE_NAME,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,COLUMN_DEFAULT,COLUMN_KEY,EXTRA,COLUMN_COMMENT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=" + schema + " ORDER BY TABLE_NAME,ORDINAL_POSITION",
		"SELECT TABLE_NAME,COLUMN_NAME,REFERENCED_TABLE_SCHEMA,REFERENCED_TABLE_NAME,REFERENCED_COLUMN_NAME,CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=" + schema + " AND REFERENCED_TABLE_NAME IS NOT NULL ORDER BY TABLE_NAME,CONSTRAINT_NAME,ORDINAL_POSITION",
	}
}

type sqlReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryResult(ctx context.Context, conn sqlReader, statement string, limit int) (QueryResult, error) {
	result := QueryResult{Columns: []string{}, Rows: [][]*string{}}
	// QueryContext also executes non-row statements. Follow with ROW_COUNT on the
	// same connection to report writes without parsing SQL or executing it twice.
	rows, err := conn.QueryContext(ctx, statement)
	if err != nil {
		return result, err
	}
	result.Columns, err = rows.Columns()
	if err != nil {
		rows.Close()
		return result, err
	}
	size := 0
	for rows.Next() {
		if len(result.Rows) >= limit {
			result.Truncated = true
			break
		}
		values := make([]sql.RawBytes, len(result.Columns))
		targets := make([]any, len(values))
		for i := range values {
			targets[i] = &values[i]
		}
		if err = rows.Scan(targets...); err != nil {
			rows.Close()
			return result, err
		}
		row := make([]*string, len(values))
		for i, v := range values {
			if v != nil {
				s := string(v)
				if !utf8.Valid(v) {
					s = "0x" + hex.EncodeToString(v)
				}
				size += len(s)
				if size > 8<<20 {
					rows.Close()
					return result, errors.New("查询结果超过 8 MB，请使用 LIMIT 或 SQL 转储")
				}
				row[i] = &s
			}
		}
		result.Rows = append(result.Rows, row)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return result, err
	}
	if closeErr != nil {
		return result, closeErr
	}
	if len(result.Columns) == 0 {
		var affected int64
		if conn.QueryRowContext(ctx, "SELECT ROW_COUNT()").Scan(&affected) == nil {
			result.Affected = fmt.Sprint(affected)
		}
	}
	return result, nil
}
func dumpSQL(ctx context.Context, conn *sql.Conn, database string, includeData bool) (string, error) {
	tables, err := queryResult(ctx, conn, "SHOW FULL TABLES FROM "+quoteIdentifier(database), 10000)
	if err != nil {
		return "", err
	}
	if tables.Truncated {
		return "", errors.New("数据库对象过多，转储已停止")
	}
	var out strings.Builder
	appendSQL := func(value string) error {
		if out.Len()+len(value) > 64<<20 {
			return errors.New("转储超过 64 MB，请使用专用备份工具")
		}
		out.WriteString(value)
		return nil
	}
	out.WriteString("-- Bajiu DB SQL dump: base tables and views; routines/events/triggers excluded.\n-- InnoDB snapshot; concurrent schema changes and nontransactional engines are not covered.\nSET NAMES utf8mb4;\nSET @BAJIU_OLD_FK=@@FOREIGN_KEY_CHECKS;\nSET FOREIGN_KEY_CHECKS=0;\n")
	if _, err = conn.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		return "", err
	}
	if _, err = conn.ExecContext(ctx, "START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY"); err != nil {
		return "", err
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	// Base tables precede views so same-schema dependencies can be restored.
	for _, view := range []bool{false, true} {
		if view {
			for _, table := range tables.Rows {
				if *table[1] != "VIEW" {
					continue
				}
				name := *table[0]
				columns, e := queryResult(ctx, conn, "SELECT COLUMN_NAME,COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA="+sqlString(database)+" AND TABLE_NAME="+sqlString(name)+" ORDER BY ORDINAL_POSITION", 10000)
				if e != nil {
					return "", e
				}
				definitions := []string{}
				for _, column := range columns.Rows {
					definitions = append(definitions, quoteIdentifier(*column[0])+" "+*column[1])
				}
				if len(definitions) == 0 {
					return "", errors.New("视图没有可转储的列")
				}
				if e = appendSQL("\nCREATE TABLE " + quoteIdentifier(name) + " (" + strings.Join(definitions, ",") + ");\n"); e != nil {
					return "", e
				}
			}
		}
		for _, table := range tables.Rows {
			if len(table) < 2 || table[0] == nil || table[1] == nil || (*table[1] == "VIEW") != view {
				continue
			}
			name := *table[0]
			kind := "TABLE"
			if view {
				kind = "VIEW"
			}
			create, e := queryResult(ctx, conn, "SHOW CREATE "+kind+" "+quoteIdentifier(database)+"."+quoteIdentifier(name), 1)
			if e != nil {
				return "", e
			}
			if len(create.Rows) != 1 || len(create.Rows[0]) < 2 || create.Rows[0][1] == nil {
				return "", errors.New("无法读取对象定义")
			}
			prefix := "\n"
			if view {
				prefix += "DROP TABLE " + quoteIdentifier(name) + ";\n"
			}
			if e = appendSQL(prefix + *create.Rows[0][1] + ";\n"); e != nil {
				return "", e
			}
			if !includeData || view {
				continue
			}
			columns, e := queryResult(ctx, conn, "SELECT COLUMN_NAME,DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA="+sqlString(database)+" AND TABLE_NAME="+sqlString(name)+" AND GENERATION_EXPRESSION='' ORDER BY ORDINAL_POSITION", 10000)
			if e != nil {
				return "", e
			}
			names := []string{}
			for _, column := range columns.Rows {
				names = append(names, quoteIdentifier(*column[0]))
			}
			if len(names) == 0 {
				continue
			}
			rows, e := conn.QueryContext(ctx, "SELECT "+strings.Join(names, ",")+" FROM "+quoteIdentifier(database)+"."+quoteIdentifier(name))
			if e != nil {
				return "", e
			}
			for rows.Next() {
				values := make([]sql.RawBytes, len(names))
				targets := make([]any, len(names))
				for i := range values {
					targets[i] = &values[i]
				}
				if e = rows.Scan(targets...); e != nil {
					rows.Close()
					return "", e
				}
				literal := make([]string, len(names))
				for i, value := range values {
					literal[i] = "NULL"
					if value != nil {
						literal[i] = "X'" + hex.EncodeToString(value) + "'"
						switch strings.ToLower(*columns.Rows[i][1]) {
						case "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob", "bit", "geometry", "point", "linestring", "polygon", "multipoint", "multilinestring", "multipolygon", "geometrycollection":
						default:
							literal[i] = "CONVERT(" + literal[i] + " USING utf8mb4)"
						}
						switch strings.ToLower(*columns.Rows[i][1]) {
						case "tinyint", "smallint", "mediumint", "int", "bigint", "decimal", "float", "double", "year":
							if !numericValue.Match(value) {
								rows.Close()
								return "", errors.New("数值列包含无法转储的值")
							}
							literal[i] = string(value)
						}
					}
				}
				if e = appendSQL("INSERT INTO " + quoteIdentifier(name) + " (" + strings.Join(names, ",") + ") VALUES (" + strings.Join(literal, ",") + ");\n"); e != nil {
					rows.Close()
					return "", e
				}
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return "", e
			}
		}
	}
	out.WriteString("\nSET FOREIGN_KEY_CHECKS=@BAJIU_OLD_FK;\n")
	return out.String(), nil
}
