package connections

import (
	"errors"
	"net"
	"strings"
	"time"
)

type Profile struct {
	ID          string    `json:"id"`
	Revision    int       `json:"revision"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	Database    string    `json:"database"`
	Environment string    `json:"environment"`
	Notes       string    `json:"notes"`
	TLSMode     string    `json:"tlsMode"`
	CAPath      string    `json:"caPath"`
	ServerName  string    `json:"serverName"`
	Timeout     int       `json:"timeout"`
	HasPassword bool      `json:"hasPassword"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Input struct {
	Profile
	Password         string `json:"password"`
	RememberPassword bool   `json:"rememberPassword"`
	PasswordAction   string `json:"passwordAction"`
}

type Record struct {
	Profile
	SecretRef string `json:"secretRef,omitempty"`
}

type SessionInfo struct {
	ID            string    `json:"id"`
	Connected     bool      `json:"connected"`
	ServerVersion string    `json:"serverVersion"`
	CurrentUser   string    `json:"currentUser"`
	Databases     []string  `json:"databases"`
	LatencyMS     int64     `json:"latencyMs"`
	ConnectedAt   time.Time `json:"connectedAt"`
}

type TestResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	ServerVersion string `json:"serverVersion"`
	LatencyMS     int64  `json:"latencyMs"`
}

func normalize(in Input, requireName bool) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Host = strings.TrimSpace(in.Host)
	in.Username = strings.TrimSpace(in.Username)
	in.Database = strings.TrimSpace(in.Database)
	in.ServerName = strings.TrimSpace(in.ServerName)
	in.CAPath = strings.TrimSpace(in.CAPath)
	if requireName && (in.Name == "" || len([]rune(in.Name)) > 80) {
		return in, errors.New("连接名称需要填写，最多 80 个字")
	}
	if in.Host == "" || strings.ContainsAny(in.Host, "/\\\x00\r\n\t ") {
		return in, errors.New("请填写主机名或 IP 地址，不要包含协议、路径或空格")
	}
	if strings.HasPrefix(in.Host, "[") && strings.HasSuffix(in.Host, "]") {
		in.Host = strings.Trim(in.Host, "[]")
	}
	if strings.Contains(in.Host, ":") && net.ParseIP(in.Host) == nil {
		return in, errors.New("端口请填写在端口栏，主机栏只填写地址")
	}
	if in.Port < 1 || in.Port > 65535 {
		return in, errors.New("端口必须在 1 到 65535 之间")
	}
	if in.Username == "" {
		return in, errors.New("请填写数据库用户名")
	}
	if in.Timeout < 1 || in.Timeout > 30 {
		return in, errors.New("连接超时必须在 1 到 30 秒之间")
	}
	if in.TLSMode != "disabled" && in.TLSMode != "verify" {
		return in, errors.New("请选择有效的 TLS 模式")
	}
	if in.Environment != "local" && in.Environment != "development" && in.Environment != "staging" && in.Environment != "production" {
		return in, errors.New("请选择有效的连接分组")
	}
	if in.PasswordAction != "keep" && in.PasswordAction != "replace" && in.PasswordAction != "forget" {
		return in, errors.New("密码操作无效，请重新编辑连接")
	}
	if len(in.Notes) > 2000 || len(in.Database) > 256 || len(in.Username) > 256 || len(in.Host) > 253 || len(in.Password) > 8192 {
		return in, errors.New("连接配置内容过长")
	}
	return in, nil
}
