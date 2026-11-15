package connections

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Database interface {
	Info(context.Context) (SessionInfo, error)
	Close() error
}

type Dialer interface {
	Open(context.Context, Input, string) (Database, error)
}
type MySQLDialer struct{}
type mysqlDatabase struct{ db *sql.DB }

func (MySQLDialer) Open(ctx context.Context, in Input, password string) (Database, error) {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(in.Host, strconv.Itoa(in.Port))
	cfg.User, cfg.Passwd, cfg.DBName = in.Username, password, in.Database
	cfg.Timeout = time.Duration(in.Timeout) * time.Second
	cfg.ReadTimeout, cfg.WriteTimeout = cfg.Timeout, cfg.Timeout
	cfg.AllowCleartextPasswords = false
	cfg.AllowFallbackToPlaintext = false
	cfg.MultiStatements = false
	if in.TLSMode == "verify" {
		serverName := in.ServerName
		if serverName == "" {
			serverName = in.Host
		}
		t := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
		if in.CAPath != "" {
			f, err := os.Open(in.CAPath)
			if err != nil {
				return nil, errors.New("无法读取 CA 证书文件，请检查路径和权限")
			}
			stat, err := f.Stat()
			if err != nil || stat.Size() > 1<<20 {
				f.Close()
				return nil, errors.New("CA 证书文件无法读取或超过 1 MB")
			}
			pem, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
			f.Close()
			if err != nil || len(pem) > 1<<20 {
				return nil, errors.New("无法读取 CA 证书文件")
			}
			roots, err := x509.SystemCertPool()
			if err != nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM(pem) {
				return nil, errors.New("CA 文件不包含有效的 PEM 证书")
			}
			t.RootCAs = roots
		}
		cfg.TLS = t
	}
	c, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, errors.New("无法创建 MySQL 连接，请检查配置")
	}
	db := sql.OpenDB(c)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(3 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, explainMySQL(err)
	}
	return &mysqlDatabase{db: db}, nil
}

func (d *mysqlDatabase) Info(ctx context.Context) (SessionInfo, error) {
	info := SessionInfo{Connected: true, Databases: []string{}}
	if err := d.db.QueryRowContext(ctx, "SELECT VERSION(), CURRENT_USER()").Scan(&info.ServerVersion, &info.CurrentUser); err != nil {
		return info, explainMySQL(err)
	}
	rows, err := d.db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return info, explainMySQL(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return info, errors.New("无法读取数据库列表")
		}
		info.Databases = append(info.Databases, name)
	}
	if err = rows.Err(); err != nil {
		return info, explainMySQL(err)
	}
	return info, nil
}

func (d *mysqlDatabase) Close() error { return d.db.Close() }

func explainMySQL(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("连接或读取超时，请检查地址、网络和超时设置")
	}
	if errors.Is(err, context.Canceled) {
		return errors.New("操作已取消")
	}
	var dbErr *mysql.MySQLError
	if errors.As(err, &dbErr) {
		switch dbErr.Number {
		case 1045, 1698:
			return errors.New("MySQL 拒绝登录，请检查用户名、密码和该账户允许访问的主机")
		case 1049:
			return errors.New("默认数据库不存在，请检查库名，或留空后连接")
		case 1044, 1142:
			return errors.New("数据库账户权限不足，请检查默认数据库和访问权限")
		case 1040:
			return errors.New("MySQL 连接数已满，请稍后重试")
		default:
			return errors.New("MySQL 返回错误（" + strconv.Itoa(int(dbErr.Number)) + "），请检查服务器状态和账户权限")
		}
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return errors.New("TLS 证书验证失败，请检查 CA 证书、证书主机名和系统时间")
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return errors.New("无法访问 MySQL，请检查主机、端口、防火墙和服务器监听设置")
	}
	if errors.Is(err, mysql.ErrNoTLS) {
		return errors.New("服务器不支持 TLS，但此连接要求验证 TLS 证书")
	}
	return errors.New("连接失败，请检查服务器、认证方式及 TLS 设置")
}
