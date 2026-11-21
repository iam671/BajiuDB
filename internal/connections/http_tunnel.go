package connections

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
)

const tunnelProtocol = "bajiu-mysql-v1"

var errTunnelResponseTooLarge = errors.New("中转单批结果超过允许大小")

type httpDatabase struct {
	in       Input
	password string
	ca       string
	client   *http.Client
	ctx      context.Context
	cancel   context.CancelFunc
	once     sync.Once
}

func newHTTPDatabase(in Input, password string) (Database, error) {
	if in.HTTPToken == "" {
		return nil, errors.New("请填写 PHP 中转脚本的访问令牌")
	}
	ca := ""
	if in.TLSMode == "verify" && in.CAPath != "" {
		data, err := readLimitedFile(in.CAPath)
		if err != nil {
			return nil, errors.New("无法读取数据库 CA 证书")
		}
		ca = string(data)
	}
	ctx, cancel := context.WithCancel(context.Background())
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxConnsPerHost: 2, ResponseHeaderTimeout: time.Duration(in.Timeout) * time.Second}
	return &httpDatabase{in: in, password: password, ca: ca, ctx: ctx, cancel: cancel, client: &http.Client{Transport: transport, Timeout: time.Duration(in.Timeout) * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("HTTP 中转不允许重定向，请填写最终脚本地址")
	}}}, nil
}
func (d *httpDatabase) call(ctx context.Context, operation, database, kind, table string, out any, admin ...AdminRequest) error {
	request := struct {
		Protocol   string `json:"protocol"`
		Operation  string `json:"operation"`
		Connection struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			Database string `json:"database"`
			Timeout  int    `json:"timeout"`
			TLSMode  string `json:"tlsMode"`
			CA       string `json:"ca"`
		} `json:"connection"`
		Database string        `json:"database"`
		Kind     string        `json:"kind"`
		Table    string        `json:"table"`
		Admin    *AdminRequest `json:"admin,omitempty"`
	}{Protocol: tunnelProtocol, Operation: operation, Database: database, Kind: kind, Table: table}
	if len(admin) > 0 {
		request.Admin = &admin[0]
	}
	request.Connection.Host = d.in.Host
	request.Connection.Port = d.in.Port
	request.Connection.Username = d.in.Username
	request.Connection.Password = d.password
	request.Connection.Database = d.in.Database
	request.Connection.Timeout = d.in.Timeout
	request.Connection.TLSMode = d.in.TLSMode
	request.Connection.CA = d.ca
	body, err := json.Marshal(request)
	if err != nil {
		return errors.New("无法编码中转请求")
	}
	current, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(d.ctx, cancel)
	defer stop()
	req, err := http.NewRequestWithContext(current, "POST", d.in.HTTPURL, bytes.NewReader(body))
	if err != nil {
		return errors.New("HTTP 中转地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bajiu-Token", d.in.HTTPToken)
	response, err := d.client.Do(req)
	if err != nil {
		if current.Err() != nil {
			return explainMySQL(current.Err())
		}
		return errors.New("无法访问 PHP 中转，请检查地址、HTTPS 证书和网络")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusRequestEntityTooLarge && operation == "admin" {
		return errTunnelResponseTooLarge
	}
	if response.StatusCode == 401 || response.StatusCode == 403 {
		if response.StatusCode == 403 && operation == "admin" && len(admin) > 0 && (admin[0].Operation == "query" || admin[0].Operation == "create" || admin[0].Operation == "alter" || admin[0].Operation == "drop") {
			return errors.New("PHP 中转拒绝 SQL 操作，请检查 BAJIU_TUNNEL_ALLOW_SQL=1、令牌及主机白名单")
		}
		return errors.New("PHP 中转拒绝访问，请检查访问令牌及数据库主机白名单")
	}
	if response.StatusCode != 200 {
		return errors.New("PHP 中转返回错误，请检查脚本部署及 mysqli 扩展")
	}
	limit := 8 << 20
	if operation == "admin" {
		limit = 64 << 20
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(limit)+1))
	if err != nil {
		return errors.New("中转响应无法读取")
	}
	if len(data) > limit {
		return errTunnelResponseTooLarge
	}
	var reply struct {
		Protocol string          `json:"protocol"`
		OK       bool            `json:"ok"`
		Code     int             `json:"code"`
		Data     json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &reply) != nil || reply.Protocol != tunnelProtocol {
		return errors.New("此地址没有返回 Bajiu DB 中转协议，请部署配套 PHP 脚本")
	}
	if !reply.OK {
		if reply.Code > 0 && reply.Code < 65536 {
			return explainMySQL(&mysql.MySQLError{Number: uint16(reply.Code)})
		}
		return errors.New("中转操作失败，请检查脚本配置、数据库 TLS 及访问权限")
	}
	if json.Unmarshal(reply.Data, out) != nil {
		return errors.New("中转数据格式无效")
	}
	return nil
}
func (d *httpDatabase) Info(ctx context.Context) (SessionInfo, error) {
	var info SessionInfo
	err := d.call(ctx, "info", "", "", "", &info)
	return info, err
}
func (d *httpDatabase) Objects(ctx context.Context, database, kind string) (ObjectList, error) {
	var list ObjectList
	err := d.call(ctx, "objects", database, kind, "", &list)
	return list, err
}
func (d *httpDatabase) TableDDL(ctx context.Context, database, table string) (string, error) {
	var ddl string
	err := d.call(ctx, "ddl", database, "", table, &ddl)
	return ddl, err
}
func (d *httpDatabase) Close() error {
	d.once.Do(func() { d.cancel(); d.client.CloseIdleConnections() })
	return nil
}
func (d *httpDatabase) Admin(ctx context.Context, database string, request AdminRequest) (AdminResult, error) {
	if request.TransactionID != "" || strings.HasPrefix(request.Operation, "transaction-") {
		return AdminResult{}, errors.New("PHP 中转每次请求使用独立会话，暂不支持跨请求事务；请使用直连或 SSH")
	}
	if request.Operation == "query" {
		mode := ""
		if request.SQLMode != nil {
			mode = *request.SQLMode
			if len(mode) > 512 || strings.Trim(mode, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_,") != "" {
				return AdminResult{}, errors.New("SQL 模式格式无效")
			}
		}
		statements, err := SplitSQLForMode(request.SQL, mode)
		if err != nil {
			return AdminResult{}, err
		}
		request.Statements = statements
		request.SQL = ""
	}
	var result AdminResult
	err := d.call(ctx, "admin", database, "", "", &result, request)
	return result, err
}
