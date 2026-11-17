package connections

import (
	"context"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSplitSQL(t *testing.T) {
	for _, test := range []struct {
		source string
		count  int
	}{
		{"SELECT ';'; SELECT `a;b`; -- trailing\n", 2},
		{"-- comment\nDELIMITER $$\nCREATE PROCEDURE p() BEGIN SELECT ';'; SELECT 2; END$$\nDELIMITER ;\nSELECT 3;", 2},
		{"/* comment */\n# note\nSELECT 'it\\'s;ok'; SELECT 'a'';b';", 2},
		{"/*!40101 SET NAMES utf8mb4 */;", 1},
	} {
		result, err := SplitSQL(test.source)
		if err != nil || len(result) != test.count {
			t.Fatalf("%q: %v %#v", test.source, err, result)
		}
	}
	if _, err := SplitSQL("SELECT 'unterminated"); err == nil {
		t.Fatal("accepted unclosed quote")
	}
	if err := validDatabaseName("bad`name"); err != nil {
		t.Fatal("quotable identifier rejected")
	}
	for _, name := range []string{"", "../db", "a\x00b"} {
		if validDatabaseName(name) == nil {
			t.Fatal("bad name", name)
		}
	}
}
func TestAdminRejectsWrongTargetsAndSystemDatabaseChanges(t *testing.T) {
	s := &Service{sessions: map[string]liveSession{"qa": {info: SessionInfo{Databases: []string{"mysql", "safe_db"}}}}, epoch: map[string]uint64{}}
	for _, test := range []struct {
		database string
		request  AdminRequest
	}{
		{"mysql", AdminRequest{Operation: "drop", Name: "mysql"}},
		{"safe_db", AdminRequest{Operation: "drop", Name: "other_db"}},
		{"unlisted", AdminRequest{Operation: "dictionary"}},
	} {
		if _, err := s.Admin(context.Background(), "qa", test.database, test.request); err == nil {
			t.Fatal("accepted invalid target", test)
		}
	}
}
func TestDatabaseAdminIntegration(t *testing.T) {
	if os.Getenv("BAJIU_ADMIN_TEST") != "1" {
		t.Skip("dedicated admin integration disabled")
	}
	if os.Getenv("BAJIU_MYSQL_TEST_ADDR") != "127.0.0.1:33179" {
		t.Fatal("admin test is limited to dedicated localhost:33179")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	in := Input{Profile: Profile{Host: "127.0.0.1", Port: 33179, Username: "root", Timeout: 30, TLSMode: "disabled"}}
	direct, err := (MySQLDialer{}).Open(ctx, in, "")
	if err != nil {
		t.Fatal(err)
	}
	defer direct.Close()
	runAdminSuite(t, ctx, direct.(adminDatabase), "bajiu_admin_qa_direct")
	cancelCtx, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	start := time.Now()
	cancelResult, cancelErr := direct.(adminDatabase).Admin(cancelCtx, "bajiu_test_db", AdminRequest{Operation: "query", SQL: "SELECT SLEEP(10)"})
	stop()
	if cancelErr == nil && cancelResult.Error == "" {
		t.Fatal("cancelled query reported success")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("query did not stop promptly")
	}
	sshAddress, signer, _ := testSSHServer(t, "127.0.0.1:33179")
	sshInput := in
	sshInput.TunnelMode = "ssh"
	sshInput.SSHHost, sshAddress, _ = net.SplitHostPort(sshAddress)
	sshInput.SSHPort, _ = strconv.Atoi(sshAddress)
	sshInput.SSHUsername = "qa"
	sshInput.SSHAuth = "password"
	sshInput.SSHPassword = "ssh-test-only"
	sshInput.SSHFingerprint = ssh.FingerprintSHA256(signer.PublicKey())
	sshDB, e := (MySQLDialer{}).Open(ctx, sshInput, "")
	if e != nil {
		t.Fatal(e)
	}
	defer sshDB.Close()
	runAdminSuite(t, ctx, sshDB.(adminDatabase), "bajiu_admin_qa_ssh")
	if os.Getenv("BAJIU_HTTP_TEST_URL") != "" {
		in.TunnelMode = "http"
		in.HTTPURL = os.Getenv("BAJIU_HTTP_TEST_URL")
		in.HTTPToken = os.Getenv("BAJIU_HTTP_TEST_TOKEN")
		httpDB, e := (MySQLDialer{}).Open(ctx, in, "")
		if e != nil {
			t.Fatal(e)
		}
		defer httpDB.Close()
		runAdminSuite(t, ctx, httpDB.(adminDatabase), "bajiu_admin_qa_http")
	}
}
func runAdminSuite(t *testing.T, ctx context.Context, db adminDatabase, name string) {
	t.Helper()
	call := func(schema string, request AdminRequest) AdminResult {
		t.Helper()
		v, e := db.Admin(ctx, schema, request)
		if e != nil || v.Error != "" {
			t.Fatalf("%s: %v %s", request.Operation, e, v.Error)
		}
		return v
	}
	call("", AdminRequest{Operation: "query", SQL: "DROP DATABASE IF EXISTS " + quoteIdentifier(name)})
	call("", AdminRequest{Operation: "create", Name: name, Charset: "utf8mb4", Collation: "utf8mb4_general_ci"})
	defer db.Admin(ctx, name, AdminRequest{Operation: "drop", Name: name})
	catalog := call(name, AdminRequest{Operation: "catalog"})
	if len(catalog.Results[0].Rows) == 0 {
		t.Fatal("no collations")
	}
	call(name, AdminRequest{Operation: "alter", Name: name, Charset: "utf8mb4", Collation: "utf8mb4_unicode_ci"})
	schema := call(name, AdminRequest{Operation: "schema"})
	if *schema.Results[0].Rows[0][1] != "utf8mb4_unicode_ci" {
		t.Fatal("alter did not persist")
	}
	script := `CREATE TABLE parent (id BIGINT PRIMARY KEY, name VARCHAR(80), payload BLOB, amount DECIMAL(20,3), details JSON, generated_id BIGINT GENERATED ALWAYS AS (id+1) STORED);
 INSERT INTO parent(id,name,payload,amount,details) VALUES(42,'中文;quote',X'0000FF',1234567890123.125,'{"name":"中文","value":42}'),(43,NULL,NULL,-0.125,NULL);
 CREATE TABLE child(id INT PRIMARY KEY,parent_id BIGINT, FOREIGN KEY(parent_id) REFERENCES parent(id));
 INSERT INTO child VALUES(1,42);
 CREATE VIEW active_parent AS SELECT id,name FROM parent;
 CREATE VIEW z_source AS SELECT id FROM parent;
 CREATE VIEW a_dependent AS SELECT id FROM z_source;`
	call(name, AdminRequest{Operation: "query", SQL: script})
	call(name, AdminRequest{Operation: "query", SQL: "-- routine script\nDELIMITER $$\nCREATE PROCEDURE qa_proc() BEGIN SELECT 1; SELECT 2; END$$\nDELIMITER ;\nCALL qa_proc(); DROP PROCEDURE qa_proc;"})
	data := call(name, AdminRequest{Operation: "query", SQL: "SELECT id,name,HEX(payload),amount,generated_id,details FROM parent ORDER BY id"})
	if *data.Results[0].Rows[0][0] != "42" || data.Results[0].Rows[1][1] != nil {
		t.Fatal("numeric or NULL query mismatch")
	}
	dictionary := call(name, AdminRequest{Operation: "dictionary"})
	if len(dictionary.Results) != 3 || len(dictionary.Results[2].Rows) != 1 {
		t.Fatal("missing columns or foreign keys")
	}
	model := call(name, AdminRequest{Operation: "model"})
	if len(model.Results[0].Rows) != 5 {
		t.Fatal("missing model tables/views")
	}
	dump := call(name, AdminRequest{Operation: "dump", IncludeData: true})
	if !strings.Contains(dump.Text, "INSERT INTO") || !strings.Contains(dump.Text, "CREATE") {
		t.Fatal("incomplete dump")
	}
	call(name, AdminRequest{Operation: "query", SQL: "DROP VIEW a_dependent; DROP VIEW z_source; DROP VIEW active_parent; DROP TABLE child; DROP TABLE parent"})
	call(name, AdminRequest{Operation: "query", SQL: dump.Text})
	restored := call(name, AdminRequest{Operation: "query", SQL: "SELECT id,name,HEX(payload),amount,generated_id,details FROM parent ORDER BY id"})
	for i, row := range data.Results[0].Rows {
		for j, value := range row {
			v := restored.Results[0].Rows[i][j]
			if (value == nil) != (v == nil) || (value != nil && *value != *v) {
				t.Fatalf("dump roundtrip row %d column %d: %v != %v", i, j, value, v)
			}
		}
	}
	failed, err := db.Admin(ctx, name, AdminRequest{Operation: "query", SQL: "SELECT 1; SELECT * FROM missing_qa_table; SELECT 3"})
	if err != nil || failed.Completed != 1 || failed.Error == "" {
		t.Fatal("batch did not stop at failure", err, failed)
	}
}
