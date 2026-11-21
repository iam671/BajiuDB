package connections

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type exportConnector struct {
	closed *bool
	read   *int
}

func (c exportConnector) Connect(context.Context) (driver.Conn, error) { return exportSQLConn{c}, nil }
func (c exportConnector) Driver() driver.Driver                        { return exportSQLDriver{} }

type exportSQLDriver struct{}

func (exportSQLDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type exportSQLConn struct{ config exportConnector }

func (exportSQLConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (exportSQLConn) Begin() (driver.Tx, error)           { return nil, errors.New("not used") }
func (exportSQLConn) Close() error                        { return nil }
func (c exportSQLConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.HasPrefix(q, "SHOW FULL COLUMNS FROM `db`.`t``x`") {
		return &exportSQLRows{metadata: true, closed: c.config.closed, read: c.config.read}, nil
	}
	if strings.HasPrefix(q, "SELECT `id`, CONCAT('0x',LPAD(HEX(`data`)") && strings.HasSuffix(q, "FROM `db`.`t``x`") && !strings.Contains(q, "LIMIT") {
		return &exportSQLRows{closed: c.config.closed, read: c.config.read}, nil
	}
	return nil, fmt.Errorf("unexpected export query: %s", q)
}

type exportSQLRows struct {
	metadata bool
	next     int
	closed   *bool
	read     *int
}

func (r *exportSQLRows) Columns() []string {
	if r.metadata {
		return []string{"Field", "Type"}
	}
	return []string{"id", "data"}
}
func (r *exportSQLRows) Close() error {
	if !r.metadata {
		*r.closed = true
	}
	return nil
}
func (r *exportSQLRows) Next(dest []driver.Value) error {
	if r.metadata {
		if r.next >= 2 {
			return io.EOF
		}
		if r.next == 0 {
			dest[0], dest[1] = "id", "int"
		} else {
			dest[0], dest[1] = "data", "blob"
		}
		r.next++
		return nil
	}
	if r.next >= 12001 {
		return io.EOF
	}
	dest[0], dest[1] = fmt.Sprint(r.next), "0x00FF"
	r.next++
	*r.read++
	return nil
}
func TestMySQLExportReadsPastPreviewLimitAndClosesCursorOnError(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			closed, read := false, 0
			db := sql.OpenDB(exportConnector{&closed, &read})
			defer db.Close()
			d := &mysqlDatabase{db: db}
			count := 0
			halt := errors.New("writer stopped")
			err := d.ExportTable(context.Background(), "db", "t`x", func(names []string) error {
				if strings.Join(names, ",") != "id,data" {
					t.Fatal(names)
				}
				return nil
			}, func(row []*string) error {
				if *row[0] != fmt.Sprint(count) || *row[1] != "0x00FF" {
					t.Fatal(row)
				}
				count++
				if stop && count == 177 {
					return halt
				}
				return nil
			})
			if !closed {
				t.Fatal("export cursor left open")
			}
			if stop {
				if !errors.Is(err, halt) || count != 177 || read != 177 {
					t.Fatal(count, read, err)
				}
			} else if err != nil || count != 12001 {
				t.Fatal(count, err)
			}
		})
	}
}

type fakeExportDB struct {
	run func(context.Context, func([]string) error, func([]*string) error) error
}

func (d *fakeExportDB) Info(context.Context) (SessionInfo, error) { return SessionInfo{}, nil }
func (d *fakeExportDB) Close() error                              { return nil }
func (d *fakeExportDB) ExportTable(ctx context.Context, _, _ string, h func([]string) error, r func([]*string) error) error {
	return d.run(ctx, h, r)
}
func TestExportServiceStreamsAllRowsAndChecksSession(t *testing.T) {
	db := &fakeExportDB{}
	s := &Service{sessions: map[string]liveSession{"qa": {db: db, info: SessionInfo{Databases: []string{"test"}}}}, epoch: map[string]uint64{}}
	db.run = func(ctx context.Context, h func([]string) error, r func([]*string) error) error {
		if err := h([]string{"id"}); err != nil {
			return err
		}
		for i := 0; i < 2501; i++ {
			v := fmt.Sprint(i)
			if err := r([]*string{&v}); err != nil {
				return err
			}
		}
		return nil
	}
	count := 0
	if err := s.ExportTable(context.Background(), "qa", "test", "t", func([]string) error { return nil }, func([]*string) error { count++; return nil }); err != nil || count != 2501 {
		t.Fatal(count, err)
	}
	if err := s.ExportTable(context.Background(), "qa", "other", "t", nil, nil); err == nil {
		t.Fatal("unknown schema allowed")
	}
	count = 0
	err := s.ExportTable(context.Background(), "qa", "test", "t", func([]string) error { return nil }, func([]*string) error { count++; s.mu.Lock(); s.epoch["qa"]++; s.mu.Unlock(); return nil })
	if err == nil || count != 1 {
		t.Fatal("session change ignored", count, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = s.ExportTable(ctx, "qa", "test", "t", func([]string) error { return nil }, func([]*string) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestExportProjectionQuotesBinaryBitAndSpatial(t *testing.T) {
	ptr := func(s string) *string { return &s }
	p, names, err := exportProjection([][]*string{{ptr("data`"), ptr("varbinary(20)")}, {ptr("flags"), ptr("BIT(64)")}, {ptr("location"), ptr("point")}, {ptr("name"), ptr("text")}})
	if err != nil || len(names) != 4 {
		t.Fatal(p, names, err)
	}
	for _, part := range []string{"HEX(`data```)", "LPAD(HEX(`flags`),LENGTH(HEX(`flags`))+MOD(LENGTH(HEX(`flags`)),2),'0')", "ST_SRID(`location`)", "ST_AsWKB(`location`)", ", `name`"} {
		if !strings.Contains(p, part) {
			t.Fatal(p, part)
		}
	}
}
func TestHTTPExportPagesUntilEndWithoutTotalRowLimit(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Admin AdminRequest `json:"admin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		q := QueryResult{Columns: []string{"id"}, Rows: [][]*string{}}
		if strings.HasPrefix(req.Admin.Statements[0], "SHOW") {
			v, typ := "id", "int"
			q.Rows = [][]*string{{&v, &typ}}
		} else {
			if !strings.Contains(req.Admin.Statements[0], "ORDER BY 1 LIMIT 500 OFFSET ") {
				t.Error(req.Admin.Statements)
			}
			n := 500
			if pages == 4 {
				n = 3
			}
			for i := 0; i < n; i++ {
				v := fmt.Sprint(pages*500 + i)
				q.Rows = append(q.Rows, []*string{&v})
			}
			pages++
		}
		json.NewEncoder(w).Encode(map[string]any{"protocol": tunnelProtocol, "ok": true, "data": AdminResult{Results: []QueryResult{q}}})
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &httpDatabase{in: Input{Profile: Profile{HTTPURL: server.URL}}, client: server.Client(), ctx: ctx}
	rows := 0
	err := d.ExportTable(ctx, "test", "t", func([]string) error { return nil }, func([]*string) error { rows++; return nil })
	if err != nil || rows != 2003 || pages != 5 {
		t.Fatal(rows, pages, err)
	}
}
func TestHTTPExportAdaptsToResponseSizeWithoutLosingRows(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Admin AdminRequest `json:"admin"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		q := QueryResult{Columns: []string{"id"}, Rows: [][]*string{}}
		statement := req.Admin.Statements[0]
		if strings.HasPrefix(statement, "SHOW") {
			name, kind := "id", "int"
			q.Rows = [][]*string{{&name, &kind}}
		} else {
			calls++
			var batch, offset int
			part := statement[strings.LastIndex(statement, "LIMIT "):]
			fmt.Sscanf(part, "LIMIT %d OFFSET %d", &batch, &offset)
			if batch > 250 {
				w.WriteHeader(413)
				return
			}
			for i := offset; i < 333 && i < offset+batch; i++ {
				v := fmt.Sprint(i)
				q.Rows = append(q.Rows, []*string{&v})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"protocol": tunnelProtocol, "ok": true, "data": AdminResult{Results: []QueryResult{q}}})
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &httpDatabase{in: Input{Profile: Profile{HTTPURL: server.URL}}, client: server.Client(), ctx: ctx}
	count := 0
	err := d.ExportTable(ctx, "test", "t", func([]string) error { return nil }, func(row []*string) error {
		if *row[0] != fmt.Sprint(count) {
			t.Fatal("missing or repeated row", count, row)
		}
		count++
		return nil
	})
	if err != nil || count != 333 || calls != 3 {
		t.Fatal(count, calls, err)
	}
}
