package connections

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestTunnelCredentialsStayInVaultAndMergeIndependently(t *testing.T) {
	s, repo, vault, dial := fixture()
	defer s.Close()
	in := validInput()
	in.TunnelMode = "ssh"
	in.SSHHost = "localhost"
	in.SSHUsername = "qa"
	in.SSHPassword = "ssh-test-only"
	in.SSHKeyPassphrase = "key-test-only"
	in.HTTPToken = "http-token-test-only"
	profile, err := s.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(repo.records)
	for _, secret := range []string{in.Password, in.SSHPassword, in.SSHKeyPassphrase, in.HTTPToken} {
		if strings.Contains(string(data), secret) {
			t.Fatal("credential leaked into metadata")
		}
	}
	edit := Input{Profile: profile, PasswordAction: "replace", Password: "new-db-test-only", RememberPassword: true}
	profile, err = s.Save(edit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Test(Input{Profile: profile, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	if dial.password != "new-db-test-only" || dial.input.SSHPassword != in.SSHPassword || dial.input.HTTPToken != in.HTTPToken {
		t.Fatal("independent secret preservation failed")
	}
	if len(vault.values) != 1 {
		t.Fatal("orphaned vault records")
	}
	oldRef := repo.records[0].SecretRef
	repo.fail = true
	edit.Profile = profile
	edit.SSHPasswordAction = "replace"
	edit.SSHPassword = "changed-ssh-test-only"
	if _, err = s.Save(edit); err == nil {
		t.Fatal("failed write accepted")
	}
	if repo.records[0].SecretRef != oldRef || len(vault.values) != 1 {
		t.Fatal("credential rollback failed")
	}
}

func testSSHServer(t *testing.T, target string) (string, ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, hostKey, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostKey)
	userPublic, userKey, _ := ed25519.GenerateKey(rand.Reader)
	userSigner, _ := ssh.NewPublicKey(userPublic)
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if c.User() == "qa" && string(password) == "ssh-test-only" {
			return nil, nil
		}
		return nil, errors.New("denied")
	}, PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if c.User() == "qa" && string(key.Marshal()) == string(userSigner.Marshal()) {
			return nil, nil
		}
		return nil, errors.New("denied")
	}}
	cfg.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var sockets []net.Conn
	t.Cleanup(func() {
		listener.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, socket := range sockets {
			socket.Close()
		}
	})
	go func() {
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			sockets = append(sockets, raw)
			mu.Unlock()
			go func() {
				defer raw.Close()
				server, channels, requests, err := ssh.NewServerConn(raw, cfg)
				if err != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for candidate := range channels {
					var destination struct {
						Host       string
						Port       uint32
						OriginHost string
						OriginPort uint32
					}
					if candidate.ChannelType() != "direct-tcpip" || ssh.Unmarshal(candidate.ExtraData(), &destination) != nil || net.JoinHostPort(destination.Host, strconv.Itoa(int(destination.Port))) != target {
						candidate.Reject(ssh.Prohibited, "not allowed")
						continue
					}
					remote, err := net.DialTimeout("tcp", target, 3*time.Second)
					if err != nil {
						candidate.Reject(ssh.ConnectionFailed, "failed")
						continue
					}
					channel, requests, err := candidate.Accept()
					if err != nil {
						remote.Close()
						continue
					}
					go ssh.DiscardRequests(requests)
					go func() {
						defer channel.Close()
						defer remote.Close()
						go func() { _, _ = io.Copy(remote, channel); remote.Close() }()
						_, _ = io.Copy(channel, remote)
					}()
				}
			}()
		}
	}()
	return listener.Addr().String(), hostSigner, userKey
}

func TestSSHMySQLIntegration(t *testing.T) {
	target := os.Getenv("BAJIU_MYSQL_TEST_ADDR")
	if target == "" {
		t.Skip("dedicated test server required")
	}
	sshAddr, hostSigner, userKey := testSSHServer(t, target)
	sshHost, sshPort, _ := net.SplitHostPort(sshAddr)
	port, _ := strconv.Atoi(sshPort)
	mysqlHost, mysqlPort, _ := net.SplitHostPort(target)
	dbPort, _ := strconv.Atoi(mysqlPort)
	in := validInput()
	in.Host = mysqlHost
	in.Port = dbPort
	in.Username = os.Getenv("BAJIU_MYSQL_TEST_USER")
	in.TunnelMode = "ssh"
	in.SSHHost = sshHost
	in.SSHPort = port
	in.SSHUsername = "qa"
	in.SSHAuth = "password"
	in.SSHPassword = "ssh-test-only"
	in.SSHFingerprint = ssh.FingerprintSHA256(hostSigner.PublicKey())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	check := func() {
		db, err := (MySQLDialer{}).Open(ctx, in, os.Getenv("BAJIU_MYSQL_TEST_PASSWORD"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		info, err := db.Info(ctx)
		if err != nil || !info.Connected {
			t.Fatal(info, err)
		}
		list, err := db.(objectBrowser).Objects(ctx, "bajiu_test_db", "tables")
		if err != nil || len(list.Objects) == 0 {
			t.Fatal(list, err)
		}
		ddl, err := db.(ddlBrowser).TableDDL(ctx, "bajiu_test_db", "customers")
		if err != nil || !strings.Contains(ddl, "CREATE TABLE") {
			t.Fatal(ddl, err)
		}
	}
	check()
	encoded, err := ssh.MarshalPrivateKeyWithPassphrase(userKey, "qa", []byte("key-test-only"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "qa-key")
	if err = os.WriteFile(path, pem.EncodeToMemory(encoded), 0600); err != nil {
		t.Fatal(err)
	}
	in.SSHAuth = "key"
	in.SSHKeyPath = path
	in.SSHKeyPassphrase = "key-test-only"
	check()
	// Reject a valid but unrelated host fingerprint.
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	key, _ := ssh.NewPublicKey(public)
	in.SSHFingerprint = ssh.FingerprintSHA256(key)
	if db, err := (MySQLDialer{}).Open(ctx, in, "unused-test-password"); err == nil {
		db.Close()
		t.Fatal("untrusted SSH host accepted")
	}
}

func TestPHPMySQLIntegration(t *testing.T) {
	endpoint := os.Getenv("BAJIU_HTTP_TEST_URL")
	if endpoint == "" {
		t.Skip("dedicated PHP tunnel required")
	}
	host, portText, err := net.SplitHostPort(os.Getenv("BAJIU_MYSQL_TEST_ADDR"))
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	in := validInput()
	in.Host = host
	in.Port = port
	in.Username = os.Getenv("BAJIU_MYSQL_TEST_USER")
	in.TunnelMode = "http"
	in.HTTPURL = endpoint
	in.HTTPToken = os.Getenv("BAJIU_HTTP_TEST_TOKEN")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := (MySQLDialer{}).Open(ctx, in, os.Getenv("BAJIU_MYSQL_TEST_PASSWORD"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	info, err := db.Info(ctx)
	if err != nil || !info.Connected {
		t.Fatal(info, err)
	}
	list, err := db.(objectBrowser).Objects(ctx, "bajiu_test_db", "tables")
	if err != nil || len(list.Objects) == 0 {
		t.Fatal(list, err)
	}
	ddl, err := db.(ddlBrowser).TableDDL(ctx, "bajiu_test_db", "customers")
	if err != nil || !strings.Contains(ddl, "CREATE TABLE") {
		t.Fatal(ddl, err)
	}
	in.HTTPToken = "incorrect-test-token"
	bad, err := (MySQLDialer{}).Open(ctx, in, "unused-test-password")
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Close()
	if _, err = bad.Info(ctx); err == nil {
		t.Fatal("bad gateway token accepted")
	}
	db.Close()
	if _, err = db.Info(ctx); err == nil {
		t.Fatal("closed gateway session accepted request")
	}
}
