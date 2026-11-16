package connections

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func normalizeTunnel(in *Input) error {
	if in.TunnelMode == "" {
		in.TunnelMode = "direct"
	}
	if in.TunnelMode != "direct" && in.TunnelMode != "ssh" && in.TunnelMode != "http" {
		return errors.New("请选择有效的连接方式")
	}
	for _, action := range []*string{&in.SSHPasswordAction, &in.SSHKeyPassphraseAction, &in.HTTPTokenAction} {
		if *action == "" {
			*action = "keep"
			if in.ID == "" {
				*action = "replace"
			}
		}
		if *action != "keep" && *action != "replace" {
			return errors.New("隧道凭据操作无效")
		}
	}
	in.SSHHost = strings.TrimSpace(in.SSHHost)
	in.SSHUsername = strings.TrimSpace(in.SSHUsername)
	in.HTTPURL = strings.TrimSpace(in.HTTPURL)
	in.SSHFingerprint = strings.TrimSpace(in.SSHFingerprint)
	if in.SSHPort == 0 {
		in.SSHPort = 22
	}
	if in.SSHAuth == "" {
		in.SSHAuth = "password"
	}
	if len(in.SSHPassword) > 8192 || len(in.SSHKeyPassphrase) > 8192 || len(in.HTTPToken) > 8192 {
		return errors.New("隧道凭据过长")
	}
	if in.TunnelMode == "ssh" {
		if in.SSHFingerprint != "" {
			decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(in.SSHFingerprint, "SHA256:"))
			if !strings.HasPrefix(in.SSHFingerprint, "SHA256:") || err != nil || len(decoded) != 32 {
				return errors.New("SSH 指纹须为完整的 SHA256:… 格式")
			}
		}
		if in.SSHHost == "" || strings.ContainsAny(in.SSHHost, "/\\\x00\r\n\t ") || len(in.SSHHost) > 253 || in.SSHPort < 1 || in.SSHPort > 65535 || in.SSHUsername == "" {
			return errors.New("请填写有效的 SSH 主机、端口及用户名")
		}
		if in.SSHAuth != "password" && in.SSHAuth != "key" {
			return errors.New("请选择 SSH 密码或私钥认证")
		}
		if in.SSHAuth == "key" && in.SSHKeyPath == "" {
			return errors.New("请选择 SSH 私钥文件")
		}
	}
	if in.TunnelMode == "http" {
		u, err := url.Parse(in.HTTPURL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("请填写 PHP 中转脚本的 HTTP/HTTPS 地址，不要在地址中包含密码或查询参数")
		}
		if in.ServerName != "" && in.ServerName != in.Host {
			return errors.New("PHP 中转的数据库证书主机名须与连接主机一致")
		}
	}
	return nil
}

func sshTunnel(ctx context.Context, in Input) (*ssh.Client, error) {
	var auth ssh.AuthMethod
	if in.SSHAuth == "key" {
		bytes, err := readLimitedFile(in.SSHKeyPath)
		if err != nil {
			return nil, errors.New("无法读取 SSH 私钥文件（最多 1 MB）")
		}
		var signer ssh.Signer
		if in.SSHKeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(bytes, []byte(in.SSHKeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(bytes)
		}
		if err != nil {
			return nil, errors.New("SSH 私钥无效或需要正确的私钥口令")
		}
		auth = ssh.PublicKeys(signer)
	} else {
		auth = ssh.Password(in.SSHPassword)
	}
	var verify ssh.HostKeyCallback
	var trustError error
	if in.SSHFingerprint != "" {
		expected := in.SSHFingerprint
		verify = func(_ string, _ net.Addr, key ssh.PublicKey) error {
			if ssh.FingerprintSHA256(key) != expected {
				trustError = errors.New("SSH 主机指纹与配置不一致")
				return trustError
			}
			return nil
		}
	} else {
		path := in.SSHKnownHostsPath
		if path == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, errors.New("无法定位 SSH known_hosts，请填写主机指纹")
			}
			path = filepath.Join(home, ".ssh", "known_hosts")
		}
		callback, err := knownhosts.New(path)
		if err != nil {
			return nil, errors.New("无法读取 known_hosts；请填写已核对的 SSH 主机 SHA256 指纹")
		}
		verify = func(host string, remote net.Addr, key ssh.PublicKey) error {
			if err := callback(host, remote, key); err != nil {
				trustError = errors.New("SSH 主机不受信任或密钥已变化；请核对 SHA256 指纹：" + ssh.FingerprintSHA256(key))
				return trustError
			}
			return nil
		}
	}
	addr := net.JoinHostPort(in.SSHHost, strconv.Itoa(in.SSHPort))
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, errors.New("无法访问 SSH 服务器")
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	} else {
		_ = raw.SetDeadline(time.Now().Add(time.Duration(in.Timeout) * time.Second))
	}
	conn, channels, requests, err := ssh.NewClientConn(raw, addr, &ssh.ClientConfig{User: in.SSHUsername, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: verify})
	if err != nil {
		_ = raw.Close()
		if trustError != nil {
			return nil, trustError
		}
		return nil, errors.New("SSH 握手或认证失败，请检查账户、密钥及主机信任设置")
	}
	_ = raw.SetDeadline(time.Time{})
	return ssh.NewClient(conn, channels, requests), nil
}

func readLimitedFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.Size() > 1<<20 {
		return nil, errors.New("file too large")
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("file too large or unreadable")
	}
	return data, nil
}
