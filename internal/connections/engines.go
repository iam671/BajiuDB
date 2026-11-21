package connections

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"
	"strings"
)

type engineSpec struct {
	label string
	port  int
	open  func(context.Context, Input, string) (Database, error)
}

var engineCatalog = map[string]engineSpec{
	"mysql":      {"MySQL", 3306, (MySQLDialer{}).Open},
	"mariadb":    {"MariaDB", 3306, (MySQLDialer{}).Open},
	"postgresql": {"PostgreSQL", 5432, openRelational},
	"sqlserver":  {"SQL Server", 1433, openRelational},
	"oracle":     {"Oracle", 1521, openRelational},
	"sqlite":     {"SQLite", 0, openRelational},
	"redis":      {"Redis", 6379, openRedis},
	"mongodb":    {"MongoDB", 27017, openMongo},
}

func canonicalEngine(engine string) string {
	if engine == "" {
		return "mysql"
	}
	return strings.ToLower(strings.TrimSpace(engine))
}
func mysqlFamily(engine string) bool {
	engine = canonicalEngine(engine)
	return engine == "mysql" || engine == "mariadb"
}

type DatabaseDialer struct{}

func (DatabaseDialer) Open(ctx context.Context, in Input, password string) (Database, error) {
	in.Engine = canonicalEngine(in.Engine)
	spec, ok := engineCatalog[in.Engine]
	if !ok {
		return nil, errors.New("不支持的数据库类型")
	}
	return spec.open(ctx, in, password)
}
func redisIndex(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > 2147483647 {
		return 0, errors.New("Redis 数据库编号须为非负整数")
	}
	return n, nil
}
func connectionTLS(in Input) (*tls.Config, error) {
	if in.TLSMode != "verify" {
		return nil, nil
	}
	name := in.ServerName
	if name == "" {
		name = in.Host
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: name}
	if in.CAPath != "" {
		pem, err := readLimitedFile(in.CAPath)
		if err != nil {
			return nil, errors.New("无法读取 CA 证书")
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("CA 文件不包含有效的 PEM 证书")
		}
		cfg.RootCAs = roots
	}
	return cfg, nil
}
func connectionError(engine string, err error) error {
	label := map[string]string{"mysql": "MySQL", "mariadb": "MariaDB", "postgresql": "PostgreSQL", "sqlserver": "SQL Server", "oracle": "Oracle", "sqlite": "SQLite", "redis": "Redis", "mongodb": "MongoDB"}[engine]
	if errors.Is(err, context.Canceled) {
		return errors.New("操作已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("连接或读取超时，请检查网络和超时设置")
	}
	var cert *tls.CertificateVerificationError
	if errors.As(err, &cert) {
		return errors.New("TLS 证书验证失败，请检查 CA 和证书主机名")
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return errors.New("无法访问 " + label + "，请检查主机、端口和监听设置")
	}
	// Driver errors may contain a DSN or password. Connection failures never echo it.
	return errors.New(label + " 连接或认证失败，请检查账户、数据库及 TLS 设置")
}
