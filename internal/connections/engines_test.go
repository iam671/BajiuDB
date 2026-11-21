package connections

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/v2/bson"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMongoPreviewKeepsCommandFirstAndQuotesCollectionName(t *testing.T) {
	name := `odd"collection`
	query := mongoPreviewCommand(name)
	var command bson.D
	if err := bson.UnmarshalExtJSON(query, false, &command); err != nil || len(command) == 0 || command[0].Key != "find" || command[0].Value != name {
		t.Fatal(string(query), command, err)
	}
}

func TestEightEngineNormalizationAndLegacyProfiles(t *testing.T) {
	if len(engineCatalog) != 8 {
		t.Fatal(engineCatalog)
	}
	for _, engine := range []string{"mysql", "mariadb", "postgresql", "sqlserver", "oracle", "sqlite", "redis", "mongodb"} {
		t.Run(engine, func(t *testing.T) {
			in := validInput()
			in.Engine = engine
			in.Port = engineCatalog[engine].port
			if engine == "oracle" {
				in.Database = "test_service"
			}
			if engine == "sqlite" {
				in.FilePath = filepath.Join(t.TempDir(), "test.db")
				in.Host = ""
				in.Username = ""
			}
			if engine == "mongodb" || engine == "redis" {
				in.Username = ""
			}
			normalized, err := normalize(in, true)
			if err != nil || normalized.Engine != engine {
				t.Fatal(normalized, err)
			}
			if engine == "sqlite" && (normalized.Port != 0 || normalized.RememberPassword || normalized.Password != "") {
				t.Fatal("SQLite retained credentials", normalized)
			}
			if !mysqlFamily(engine) {
				in.TunnelMode = "http"
				in.HTTPURL = "https://example.com/bajiu-mysql.php"
				if _, err = normalize(in, true); err == nil {
					t.Fatal("foreign engine accepted MySQL tunnel")
				}
			}
		})
	}
	in := validInput()
	normalized, err := normalize(in, true)
	if err != nil || normalized.Engine != "mysql" {
		t.Fatal(normalized, err)
	}
	for _, engine := range []string{"unsupported", "redis", "oracle"} {
		in = validInput()
		in.Engine = engine
		if engine == "redis" {
			in.Database = "-1"
		}
		if _, err = normalize(in, true); err == nil {
			t.Fatal("invalid configuration accepted", engine)
		}
	}
}
func TestEnginePersistenceKeepsSecretsOutOfProfileAndMigratesLegacy(t *testing.T) {
	dir := t.TempDir()
	store := FileStore{Path: filepath.Join(dir, "connections.json")}
	input := validInput()
	profile := input.Profile
	profile.ID = "legacy"
	raw, _ := json.Marshal(document{Version: 1, Connections: []Record{{Profile: profile}}})
	if err := os.WriteFile(store.Path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	records, err := store.Load()
	if err != nil || records[0].Engine != "mysql" {
		t.Fatal(records, err)
	}
	vault := &memorySecrets{values: map[string]string{}}
	service := New(store, vault, &fakeDialer{db: &fakeDB{}})
	defer service.Close()
	input.Engine = "postgresql"
	input.Name = "PostgreSQL QA"
	input.Port = 5432
	input.Database = "postgres"
	saved, err := service.Save(input)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || loaded[1].Engine != "postgresql" || saved.Database != "postgres" {
		t.Fatal(loaded, saved, err)
	}
	raw, _ = os.ReadFile(store.Path)
	if strings.Contains(string(raw), input.Password) {
		t.Fatal("password saved in config")
	}
	changed := input
	changed.Profile = saved
	changed.Engine = "redis"
	changed.Database = "0"
	changed.Username = ""
	changed.PasswordAction = "keep"
	if _, err = service.Save(changed); err == nil {
		t.Fatal("changed saved engine identity")
	}
	if _, err = service.Connect(changed); err == nil {
		t.Fatal("opened saved ID with another engine")
	}
}
func TestSQLiteConnectionBrowsingAndNativeQueries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE "odd""table" (id INTEGER PRIMARY KEY, big TEXT, bytes BLOB);INSERT INTO "odd""table" VALUES(1,'9007199254740993123',X'00ff80');CREATE VIEW visible AS SELECT big FROM "odd""table"`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	in := validInput()
	in.Engine = "sqlite"
	in.FilePath = path
	in, err = normalize(in, true)
	if err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepo{}
	vault := &memorySecrets{values: map[string]string{}}
	service := New(repo, vault, DatabaseDialer{})
	defer service.Close()
	profile, err := service.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	info, err := service.Connect(Input{Profile: profile, PasswordAction: "keep"})
	if err != nil || len(info.Databases) != 1 || info.Databases[0] != "main" {
		t.Fatal(info, err)
	}
	list, err := service.Objects(profile.ID, "main", "tables")
	if err != nil || len(list.Objects) != 1 || !strings.Contains(list.Objects[0].Query, `"odd""table"`) {
		t.Fatal(list, err)
	}
	result, err := service.Admin(context.Background(), profile.ID, "main", AdminRequest{Operation: "query", SQL: list.Objects[0].Query})
	if err != nil || *result.Results[0].Rows[0][1] != "9007199254740993123" || *result.Results[0].Rows[0][2] != "0x00ff80" {
		t.Fatal(result, err)
	}
	result, err = service.Admin(context.Background(), profile.ID, "main", AdminRequest{Operation: "query", SQL: `INSERT INTO "odd""table" VALUES(2,'new',NULL)`})
	if err != nil || result.Results[0].Affected != "1" {
		t.Fatal(result, err)
	}
	if _, err = service.Admin(context.Background(), profile.ID, "main", AdminRequest{Operation: "drop", Name: "main"}); err == nil {
		t.Fatal("MySQL schema action accepted")
	}
	service.Disconnect(profile.ID)
	if len(service.Sessions()) != 0 {
		t.Fatal("session stayed open")
	}
	in.FilePath = filepath.Join(t.TempDir(), "missing.db")
	if _, err = (DatabaseDialer{}).Open(context.Background(), in, ""); err == nil {
		t.Fatal("missing file created")
	}
	if _, err = os.Stat(in.FilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	in.FilePath = filepath.Join(t.TempDir(), "invalid.db")
	os.WriteFile(in.FilePath, []byte("not sqlite"), 0600)
	if _, err = (DatabaseDialer{}).Open(context.Background(), in, ""); err == nil {
		t.Fatal("invalid SQLite file accepted")
	}
}
func TestDatabaseConnectionErrorsDoNotExposeCredentials(t *testing.T) {
	for engine := range engineCatalog {
		if engine == "sqlite" {
			continue
		}
		text := connectionError(engine, fmt.Errorf("dsn: secret-test-value")).Error()
		if strings.Contains(text, "secret-test-value") {
			t.Fatal(text)
		}
	}
	in := validInput()
	in.TLSMode = "verify"
	in.ServerName = "db.example.test"
	cfg, err := connectionTLS(in)
	if err != nil || cfg == nil || cfg.InsecureSkipVerify || cfg.ServerName != "db.example.test" {
		t.Fatal(cfg, err)
	}
}

// Opt-in uses a new isolated Redis process, never an existing Redis service.
func TestIsolatedRedisEngineIntegration(t *testing.T) {
	executable := os.Getenv("BAJIU_REDIS_TEST_BIN")
	if executable == "" {
		t.Skip("isolated Redis binary not provided")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "--bind", "127.0.0.1", "--port", fmt.Sprint(port), "--save", "", "--appendonly", "no", "--requirepass", "redis-test-only", "--dir", t.TempDir())
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); cmd.Wait() }()
	in := validInput()
	in.Engine = "redis"
	in.Username = ""
	in.Password = "redis-test-only"
	in.Database = "2"
	in.Port = port
	var database Database
	for i := 0; i < 50; i++ {
		database, err = (DatabaseDialer{}).Open(ctx, in, in.Password)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	redisDB := database.(*redisDatabase)
	for _, command := range []string{`["SET","hello","9007199254740993123"]`, `["HSET","map","field","中文"]`, `["SET",{"hex":"00ff"},"binary key"]`} {
		if _, err = redisDB.Admin(ctx, "2", AdminRequest{Operation: "query", SQL: command}); err != nil {
			t.Fatal(err)
		}
	}
	info, err := database.Info(ctx)
	if err != nil || len(info.Databases) != 1 || info.Databases[0] != "2" {
		t.Fatal(info, err)
	}
	objects, err := redisDB.Objects(ctx, "2", "tables")
	if err != nil || len(objects.Objects) != 3 {
		t.Fatal(objects, err)
	}
	for _, object := range objects.Objects {
		if _, err = redisDB.Admin(ctx, "2", AdminRequest{Operation: "query", SQL: object.Query}); err != nil {
			t.Fatal(object, err)
		}
	}
	result, err := redisDB.Admin(ctx, "2", AdminRequest{Operation: "query", SQL: `["GET","hello"]`})
	if err != nil || *result.Results[0].Rows[0][0] != "9007199254740993123" {
		t.Fatal(result, err)
	}
	result, err = redisDB.Admin(ctx, "0", AdminRequest{Operation: "query", SQL: `["GET","hello"]`})
	if err != nil || result.Results[0].Rows[0][0] != nil {
		t.Fatal("logical DB leaked", result, err)
	}
	if _, err = redisDB.Admin(ctx, "2", AdminRequest{Operation: "query", SQL: `["MONITOR"]`}); err == nil {
		t.Fatal("monitor accepted")
	}
}
