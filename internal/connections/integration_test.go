package connections

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The caller supplies a dedicated test server. Never discovers application credentials.
func TestMySQLIntegration(t *testing.T) {
	addr := os.Getenv("BAJIU_MYSQL_TEST_ADDR")
	if addr == "" {
		t.Skip("set BAJIU_MYSQL_TEST_ADDR to an isolated MySQL test server")
	}
	host, portString, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	in := validInput()
	in.Host, in.Port, in.Username = host, port, os.Getenv("BAJIU_MYSQL_TEST_USER")
	password := os.Getenv("BAJIU_MYSQL_TEST_PASSWORD")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := (MySQLDialer{}).Open(ctx, in, password)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	info, err := db.Info(ctx)
	if err != nil || !info.Connected || info.ServerVersion == "" {
		t.Fatal("missing server information", err)
	}
	found := false
	for _, name := range info.Databases {
		if name == "bajiu_test_db" {
			found = true
		}
	}
	if !found {
		t.Fatal("test database not present in accessible databases")
	}
	browser := db.(objectBrowser)
	for kind, name := range map[string]string{"tables": "customers", "views": "active_customers"} {
		result, objectErr := browser.Objects(ctx, "bajiu_test_db", kind)
		if objectErr != nil {
			t.Fatal(kind, objectErr)
		}
		present := false
		for _, object := range result.Objects {
			if object.Name == name && object.Kind == kind {
				present = true
			}
		}
		if !present {
			t.Fatalf("missing %s fixture: %#v", kind, result)
		}
	}
	result, err := browser.Objects(ctx, "bajiu_test_db' OR 1=1 --", "tables")
	if err != nil || len(result.Objects) != 0 {
		t.Fatal("schema name was not treated as a bound value", err)
	}
	for _, kind := range []string{"functions", "events"} {
		if _, err = browser.Objects(ctx, "bajiu_test_db", kind); err != nil {
			t.Fatal(kind, err)
		}
	}
	_, err = (MySQLDialer{}).Open(ctx, in, "wrong-password-test-only")
	if err == nil || !strings.Contains(err.Error(), "拒绝登录") {
		t.Fatal("authentication error not classified", err)
	}
	if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), "wrong-password-test-only") {
		t.Fatal("password leaked into error")
	}
	in.TLSMode = "verify"
	_, err = (MySQLDialer{}).Open(ctx, in, password)
	if err == nil {
		t.Fatal("test server self-signed certificate must not be trusted implicitly")
	}
}

func TestSystemCredentialRoundTrip(t *testing.T) {
	if os.Getenv("BAJIU_TEST_KEYRING") != "1" {
		t.Skip("set BAJIU_TEST_KEYRING=1 to test the system vault")
	}
	id, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	key := "test-" + id
	vault := SystemSecrets{}
	defer vault.Delete(key)
	if err = vault.Set(key, "credential-test-value-only"); err != nil {
		t.Fatal(err)
	}
	value, err := vault.Get(key)
	if err != nil || value != "credential-test-value-only" {
		t.Fatal("credential roundtrip failed", err)
	}
	if err = vault.Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, err = vault.Get(key); err == nil {
		t.Fatal("deleted credential can still be read")
	}
}
