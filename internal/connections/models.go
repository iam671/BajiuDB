package connections

import (
	"errors"
	"net"
	"strings"
	"time"
)

type Profile struct {
	ID                string    `json:"id"`
	Revision          int       `json:"revision"`
	Name              string    `json:"name"`
	GroupID           string    `json:"groupId,omitempty"`
	Engine            string    `json:"engine,omitempty"`
	FilePath          string    `json:"filePath,omitempty"`
	AuthDatabase      string    `json:"authDatabase,omitempty"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
	Username          string    `json:"username"`
	Database          string    `json:"database"`
	Environment       string    `json:"environment"`
	Notes             string    `json:"notes"`
	TLSMode           string    `json:"tlsMode"`
	CAPath            string    `json:"caPath"`
	ServerName        string    `json:"serverName"`
	Timeout           int       `json:"timeout"`
	TunnelMode        string    `json:"tunnelMode,omitempty"`
	SSHHost           string    `json:"sshHost,omitempty"`
	SSHPort           int       `json:"sshPort,omitempty"`
	SSHUsername       string    `json:"sshUsername,omitempty"`
	SSHAuth           string    `json:"sshAuth,omitempty"`
	SSHKeyPath        string    `json:"sshKeyPath,omitempty"`
	SSHKnownHostsPath string    `json:"sshKnownHostsPath,omitempty"`
	SSHFingerprint    string    `json:"sshFingerprint,omitempty"`
	HTTPURL           string    `json:"httpUrl,omitempty"`
	HasPassword       bool      `json:"hasPassword"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Input struct {
	Profile
	Password               string `json:"password"`
	RememberPassword       bool   `json:"rememberPassword"`
	PasswordAction         string `json:"passwordAction"`
	SSHPassword            string `json:"sshPassword"`
	SSHKeyPassphrase       string `json:"sshKeyPassphrase"`
	HTTPToken              string `json:"httpToken"`
	SSHPasswordAction      string `json:"sshPasswordAction"`
	SSHKeyPassphraseAction string `json:"sshKeyPassphraseAction"`
	HTTPTokenAction        string `json:"httpTokenAction"`
}

type Record struct {
	Profile
	SecretRef    string `json:"secretRef,omitempty"`
	SecretFormat int    `json:"secretFormat,omitempty"`
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

// Numeric metadata is transmitted as text so unsigned BIGINT values stay exact.
type DatabaseObject struct {
	Query         string `json:"query,omitempty"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Engine        string `json:"engine"`
	Rows          string `json:"rows"`
	DataLength    string `json:"dataLength"`
	IndexLength   string `json:"indexLength"`
	AutoIncrement string `json:"autoIncrement"`
	Comment       string `json:"comment"`
	CreatedAt     string `json:"createdAt"`
	ModifiedAt    string `json:"modifiedAt"`
}

type ObjectList struct {
	Objects   []DatabaseObject `json:"objects"`
	Truncated bool             `json:"truncated"`
}

func normalize(in Input, requireName bool) (Input, error) {
	in.Engine = canonicalEngine(in.Engine)
	if _, ok := engineCatalog[in.Engine]; !ok {
		return in, errors.New("请选择支持的数据库类型")
	}
	in.FilePath = strings.TrimSpace(in.FilePath)
	in.AuthDatabase = strings.TrimSpace(in.AuthDatabase)
	in.Name = strings.TrimSpace(in.Name)
	in.Host = strings.TrimSpace(in.Host)
	in.Username = strings.TrimSpace(in.Username)
	in.Database = strings.TrimSpace(in.Database)
	in.ServerName = strings.TrimSpace(in.ServerName)
	in.CAPath = strings.TrimSpace(in.CAPath)
	if requireName && (in.Name == "" || len([]rune(in.Name)) > 80) {
		return in, errors.New("连接名称需要填写，最多 80 个字")
	}
	if in.Engine != "sqlite" && (in.Host == "" || strings.ContainsAny(in.Host, "/\\\x00\r\n\t ")) {
		return in, errors.New("请填写主机名或 IP 地址，不要包含协议、路径或空格")
	}
	if strings.HasPrefix(in.Host, "[") && strings.HasSuffix(in.Host, "]") {
		in.Host = strings.Trim(in.Host, "[]")
	}
	if strings.Contains(in.Host, ":") && net.ParseIP(in.Host) == nil {
		return in, errors.New("端口请填写在端口栏，主机栏只填写地址")
	}
	if in.Engine != "sqlite" && (in.Port < 1 || in.Port > 65535) {
		return in, errors.New("端口必须在 1 到 65535 之间")
	}
	if in.Engine != "sqlite" && in.Engine != "redis" && in.Engine != "mongodb" && in.Username == "" {
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
	if err := normalizeTunnel(&in); err != nil {
		return in, err
	}
	if !mysqlFamily(in.Engine) && in.TunnelMode != "direct" {
		return in, errors.New("该数据库类型当前使用直连；SSH/PHP 隧道适用于 MySQL/MariaDB")
	}
	if in.Engine == "sqlite" {
		if in.FilePath == "" || strings.ContainsRune(in.FilePath, 0) || len(in.FilePath) > 4096 {
			return in, errors.New("请选择有效的 SQLite 数据库文件")
		}
		if in.TLSMode != "disabled" {
			return in, errors.New("SQLite 本地文件不使用 TLS")
		}
		in.Host = ""
		in.Port = 0
		in.Username = ""
		in.Database = "main"
		in.Password = ""
		in.PasswordAction = "replace"
		in.RememberPassword = false
	}
	if in.Engine == "postgresql" && in.Database == "" {
		in.Database = "postgres"
	}
	if in.Engine == "sqlserver" && in.Database == "" {
		in.Database = "master"
	}
	if in.Engine == "oracle" && in.Database == "" {
		return in, errors.New("请填写 Oracle 服务名（Service Name）")
	}
	if in.Engine == "redis" {
		if in.Database == "" {
			in.Database = "0"
		}
		if _, err := redisIndex(in.Database); err != nil {
			return in, err
		}
	}
	if len(in.AuthDatabase) > 256 || strings.ContainsRune(in.AuthDatabase, 0) {
		return in, errors.New("认证数据库无效")
	}
	if in.Engine == "mongodb" && in.AuthDatabase == "" {
		in.AuthDatabase = "admin"
	}
	return in, nil
}
