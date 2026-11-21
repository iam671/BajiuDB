package connections

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	goora "github.com/sijms/go-ora/v2"
	_ "modernc.org/sqlite"
)

type relationalDatabase struct {
	in       Input
	password string
	mu       sync.Mutex
	pools    map[string]*sql.DB
	closed   bool
}

func openRelational(ctx context.Context, in Input, password string) (Database, error) {
	d := &relationalDatabase{in: in, password: password, pools: map[string]*sql.DB{}}
	db, err := d.pool(ctx, in.Database)
	if err == nil {
		err = db.PingContext(ctx)
	}
	if err == nil && in.Engine == "sqlite" {
		var version int
		err = db.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&version)
	}
	if err != nil {
		d.Close()
		if in.Engine == "sqlite" {
			return nil, errors.New("无法打开 SQLite 文件，请检查文件格式、路径与读写权限")
		}
		return nil, connectionError(in.Engine, err)
	}
	return d, nil
}
func (d *relationalDatabase) pool(ctx context.Context, database string) (*sql.DB, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, errors.New("连接已关闭")
	}
	key := database
	if d.in.Engine == "oracle" || d.in.Engine == "sqlite" {
		key = d.in.Database
	}
	if db := d.pools[key]; db != nil {
		return db, nil
	}
	var db *sql.DB
	var err error
	in := d.in
	timeout := time.Duration(in.Timeout) * time.Second
	tlsConfig, err := connectionTLS(in)
	if err != nil {
		return nil, err
	}
	switch in.Engine {
	case "postgresql":
		cfg, e := pgx.ParseConfig("postgres://localhost/postgres?sslmode=disable")
		if e != nil {
			return nil, e
		}
		cfg.Host = in.Host
		cfg.Port = uint16(in.Port)
		cfg.User = in.Username
		cfg.Password = d.password
		cfg.Database = database
		cfg.ConnectTimeout = timeout
		cfg.TLSConfig = tlsConfig
		cfg.Fallbacks = nil
		cfg.RuntimeParams = map[string]string{"application_name": "BajiuDB"}
		db = stdlib.OpenDB(*cfg)
	case "sqlserver":
		cfg, e := sqlServerConfig(in, d.password, database)
		if e != nil {
			return nil, e
		}
		if tlsConfig != nil {
			cfg.Encryption = msdsn.EncryptionRequired
			cfg.TLSConfig = tlsConfig
			cfg.HostInCertificateProvided = true
		}
		db = sql.OpenDB(mssql.NewConnectorConfig(cfg))
	case "oracle":
		opts := map[string]string{"CONNECTION TIMEOUT": strconv.Itoa(in.Timeout), "TIMEOUT": strconv.Itoa(in.Timeout)}
		if tlsConfig != nil {
			opts["SSL"] = "true"
			opts["SSL VERIFY"] = "true"
		}
		connector := goora.NewConnector(goora.BuildUrl(in.Host, in.Port, in.Database, in.Username, d.password, opts))
		if c, ok := connector.(*goora.OracleConnector); ok && tlsConfig != nil {
			c.WithTLSConfig(tlsConfig)
		}
		db = sql.OpenDB(connector)
	case "sqlite":
		info, e := os.Stat(in.FilePath)
		if e != nil || !info.Mode().IsRegular() {
			return nil, errors.New("请选择已存在的 SQLite 普通数据库文件")
		}
		path, e := filepath.Abs(in.FilePath)
		if e != nil {
			return nil, e
		}
		path = filepath.ToSlash(path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		params := url.Values{"mode": {"rw"}, "_pragma": {"foreign_keys(1)", "busy_timeout(5000)"}}
		uri := url.URL{Scheme: "file", Path: path, RawQuery: params.Encode()}
		db, err = sql.Open("sqlite", uri.String())
	default:
		return nil, errors.New("不支持的关系型数据库")
	}
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(3 * time.Minute)
	d.pools[key] = db
	return db, nil
}

// Parse the structured URL so the driver initializes its protocol registry and
// protocol parameters. A zero-value msdsn.Config cannot establish a connection.
func sqlServerConfig(in Input, password, database string) (msdsn.Config, error) {
	params := url.Values{"database": {database}, "encrypt": {"disable"}, "dial timeout": {strconv.Itoa(in.Timeout)}, "connection timeout": {strconv.Itoa(in.Timeout)}, "app name": {"BajiuDB"}}
	uri := url.URL{Scheme: "sqlserver", User: url.UserPassword(in.Username, password), Host: net.JoinHostPort(in.Host, strconv.Itoa(in.Port)), RawQuery: params.Encode()}
	cfg, err := msdsn.Parse(uri.String())
	cfg.DisableRetry = true
	return cfg, err
}
func (d *relationalDatabase) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	for key, db := range d.pools {
		db.Close()
		delete(d.pools, key)
	}
	return nil
}
func (d *relationalDatabase) Info(ctx context.Context) (SessionInfo, error) {
	info := SessionInfo{Connected: true, Databases: []string{}}
	db, err := d.pool(ctx, d.in.Database)
	if err != nil {
		return info, err
	}
	var query string
	switch d.in.Engine {
	case "postgresql":
		err = db.QueryRowContext(ctx, "SELECT version(),current_user").Scan(&info.ServerVersion, &info.CurrentUser)
		query = "SELECT datname FROM pg_database WHERE datallowconn AND has_database_privilege(datname,'CONNECT') ORDER BY datname"
	case "sqlserver":
		err = db.QueryRowContext(ctx, "SELECT CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(128)),SUSER_SNAME()").Scan(&info.ServerVersion, &info.CurrentUser)
		query = "SELECT name FROM sys.databases WHERE HAS_DBACCESS(name)=1 ORDER BY name"
	case "oracle":
		err = db.QueryRowContext(ctx, "SELECT SYS_CONTEXT('USERENV','SESSION_USER') FROM dual").Scan(&info.CurrentUser)
		if err == nil {
			if e := db.QueryRowContext(ctx, "SELECT banner FROM v$version WHERE ROWNUM=1").Scan(&info.ServerVersion); e != nil {
				info.ServerVersion = "Oracle"
			}
		}
		query = "SELECT DISTINCT owner FROM all_objects WHERE object_type IN ('TABLE','VIEW') UNION SELECT SYS_CONTEXT('USERENV','CURRENT_SCHEMA') FROM dual ORDER BY 1"
	case "sqlite":
		err = db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&info.ServerVersion)
		info.CurrentUser = "本地文件"
		info.Databases = []string{"main"}
		return info, err
	}
	if err != nil {
		return info, connectionError(d.in.Engine, err)
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return info, connectionError(d.in.Engine, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return info, err
		}
		if len(info.Databases) >= 10000 {
			return info, errors.New("数据库或模式列表超过 10,000 项")
		}
		info.Databases = append(info.Databases, name)
	}
	return info, rows.Err()
}
func quoteSQLName(engine, name string) string {
	if engine == "sqlserver" {
		return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
func (d *relationalDatabase) Objects(ctx context.Context, database, kind string) (ObjectList, error) {
	result := ObjectList{Objects: []DatabaseObject{}}
	if kind != "tables" && kind != "views" {
		return result, nil
	}
	db, err := d.pool(ctx, database)
	if err != nil {
		return result, err
	}
	var query string
	var args []any
	tableType := "BASE TABLE"
	if kind == "views" {
		tableType = "VIEW"
	}
	switch d.in.Engine {
	case "postgresql":
		query = "SELECT table_schema,table_name FROM information_schema.tables WHERE table_type=$1 AND table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema,table_name LIMIT 10001"
		args = []any{tableType}
	case "sqlserver":
		query = "SELECT TOP (10001) TABLE_SCHEMA,TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE=@p1 ORDER BY TABLE_SCHEMA,TABLE_NAME"
		args = []any{tableType}
	case "oracle":
		source := "all_tables"
		if kind == "views" {
			source = "all_views"
		}
		query = "SELECT owner,table_name FROM " + source + " WHERE owner=:1 AND ROWNUM<=10001 ORDER BY table_name"
		if kind == "views" {
			query = "SELECT owner,view_name FROM all_views WHERE owner=:1 AND ROWNUM<=10001 ORDER BY view_name"
		}
		args = []any{database}
	case "sqlite":
		objectType := "table"
		if kind == "views" {
			objectType = "view"
		}
		query = "SELECT '',name FROM sqlite_schema WHERE type=? AND name NOT LIKE 'sqlite_%' ORDER BY name LIMIT 10001"
		args = []any{objectType}
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		if len(result.Objects) >= 10000 {
			result.Truncated = true
			break
		}
		var schema, name string
		if err = rows.Scan(&schema, &name); err != nil {
			return result, err
		}
		target := quoteSQLName(d.in.Engine, name)
		label := name
		if schema != "" {
			target = quoteSQLName(d.in.Engine, schema) + "." + target
			label = target
		}
		statement := "SELECT * FROM " + target + " LIMIT 1000;"
		if d.in.Engine == "sqlserver" {
			statement = "SELECT TOP (1000) * FROM " + target + ";"
		}
		if d.in.Engine == "oracle" {
			statement = "SELECT * FROM " + target + " WHERE ROWNUM<=1000"
		}
		result.Objects = append(result.Objects, DatabaseObject{Name: label, Kind: kind, Engine: d.in.Engine, Query: statement})
	}
	return result, rows.Err()
}
func (d *relationalDatabase) Admin(ctx context.Context, database string, request AdminRequest) (AdminResult, error) {
	result := AdminResult{Results: []QueryResult{}}
	if request.Operation != "query" {
		return result, errors.New("此数据库当前支持对象浏览与原生 SQL 查询；结构工具仅适用于 MySQL/MariaDB")
	}
	if request.TransactionID != "" {
		return result, errors.New("该数据库尚未接入表事务工作区")
	}
	if strings.TrimSpace(request.SQL) == "" || len(request.SQL) > 8<<20 {
		return result, errors.New("请输入 8 MB 以内的 SQL")
	}
	db, err := d.pool(ctx, database)
	if err != nil {
		return result, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return result, err
	}
	defer conn.Close()
	// Discard session state (USE, SET, open transactions) after an explicit batch.
	defer conn.Raw(func(any) error { return driver.ErrBadConn })
	if d.in.Engine == "oracle" {
		if _, err = conn.ExecContext(ctx, "ALTER SESSION SET CURRENT_SCHEMA="+quoteSQLName("oracle", database)); err != nil {
			return result, err
		}
	}
	text := strings.TrimSpace(request.SQL)
	token := strings.ToUpper(strings.Fields(text)[0])
	read := strings.HasPrefix(text, "--") || strings.HasPrefix(text, "/*")
	for _, prefix := range []string{"SELECT", "WITH", "SHOW", "PRAGMA", "EXPLAIN", "VALUES"} {
		if token == prefix {
			read = true
		}
	}
	if read {
		readCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		rows, e := conn.QueryContext(readCtx, request.SQL)
		if e != nil {
			return result, e
		}
		defer func() { cancel(); rows.Close() }()
		q, e := collectSQLRows(rows, 1000)
		if e != nil {
			return result, e
		}
		result.Results = append(result.Results, q)
	} else {
		r, e := conn.ExecContext(ctx, request.SQL)
		if e != nil {
			return result, e
		}
		n, e := r.RowsAffected()
		affected := ""
		if e == nil {
			affected = strconv.FormatInt(n, 10)
		}
		result.Results = append(result.Results, QueryResult{Columns: []string{}, Rows: [][]*string{}, Affected: affected})
	}
	result.Completed = 1
	return result, nil
}
func collectSQLRows(rows *sql.Rows, limit int) (QueryResult, error) {
	q := QueryResult{Columns: []string{}, Rows: [][]*string{}}
	var err error
	q.Columns, err = rows.Columns()
	if err != nil {
		return q, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return q, err
	}
	size := 0
	for rows.Next() {
		if len(q.Rows) >= limit {
			q.Truncated = true
			break
		}
		values := make([]any, len(q.Columns))
		targets := make([]any, len(values))
		for i := range values {
			targets[i] = &values[i]
		}
		if err = rows.Scan(targets...); err != nil {
			return q, err
		}
		row := make([]*string, len(values))
		for i, value := range values {
			if value == nil {
				continue
			}
			var text string
			switch v := value.(type) {
			case []byte:
				kind := strings.ToUpper(types[i].DatabaseTypeName())
				if strings.Contains(kind, "BINARY") || strings.Contains(kind, "BLOB") || kind == "BYTEA" || kind == "RAW" || !utf8.Valid(v) {
					text = "0x" + hex.EncodeToString(v)
				} else {
					text = string(v)
				}
			case time.Time:
				text = v.Format("2006-01-02 15:04:05.999999999Z07:00")
			case float64:
				text = strconv.FormatFloat(v, 'g', -1, 64)
			default:
				text = fmt.Sprint(v)
			}
			size += len(text)
			if size > 8<<20 {
				return q, errors.New("查询预览超过 8 MB，请限制结果范围")
			}
			row[i] = &text
		}
		q.Rows = append(q.Rows, row)
	}
	return q, rows.Err()
}
