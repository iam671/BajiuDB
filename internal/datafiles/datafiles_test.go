package datafiles

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestNativeFormatRoundTrips(t *testing.T) {
	original := Data{Columns: []string{"id", "name", "note", "literal", "empty"}, Rows: [][]*string{{text("9007199254740993123456789"), text("中文 & <xml>"), nil, text(`\N`), text("")}, {text("99999999999999999.123456789"), text("=1+1"), text("line 1\r\nline 2,\"quoted\"\rlast\n"), text(`\\path`), nil}}}
	for _, format := range []string{"txt", "csv", "json", "xml", "xlsx", "db"} {
		t.Run(format, func(t *testing.T) {
			o := Options{Format: format, Path: filepath.Join(t.TempDir(), "data."+format), Table: "data", Delimiter: "tab"}
			if err := Write(context.Background(), o, original); err != nil {
				t.Fatal(err)
			}
			names, err := Inspect(context.Background(), o)
			if err != nil || len(names) != 1 {
				t.Fatal(names, err)
			}
			o.Table = names[0]
			got, err := Read(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, original) {
				t.Fatalf("round trip changed %s data:\n%#v\n%#v", format, got, original)
			}
		})
	}
}
func TestEncodedMySQLValuesRoundTrip(t *testing.T) {
	original := Data{Columns: []string{"payload", "place", "flags"}, Rows: [][]*string{
		{text("0x00FF8041"), text("SRID=4326;WKB=0x01010000000000000000003E400000000000003440"), text("0x0101")},
		{text("0x"), nil, nil},
	}}
	for _, format := range []string{"txt", "csv", "json", "xml", "xlsx", "db"} {
		t.Run(format, func(t *testing.T) {
			options := Options{Format: format, Path: filepath.Join(t.TempDir(), "data."+format), Table: "data", Delimiter: "tab", ProtectFormulas: true}
			if err := Write(context.Background(), options, original); err != nil {
				t.Fatal(err)
			}
			names, err := Inspect(context.Background(), options)
			if err != nil || len(names) != 1 {
				t.Fatal(names, err)
			}
			options.Table = names[0]
			got, err := Read(context.Background(), options)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, original) {
				t.Fatalf("%s changed encoded binary/spatial values: %#v", format, got)
			}
		})
	}
}
func TestJSONAndXMLExternalData(t *testing.T) {
	d, err := readJSON([]byte(`[{"id":9007199254740993123,"amount":0.123456789012345678901,"optional":null,"nested":{"x":1234567890123456789}}]`))
	if err != nil {
		t.Fatal(err)
	}
	if *d.Rows[0][0] != "0.123456789012345678901" || *d.Rows[0][1] != "9007199254740993123" || *d.Rows[0][2] != `{"x":1234567890123456789}` || d.Rows[0][3] != nil {
		t.Fatal(d)
	}
	d, err = readXML([]byte(`<rows><row><id>123</id><value nil="true"/></row><row><id>456</id><value>&lt;&amp;&gt;</value></row></rows>`))
	if err != nil || len(d.Rows) != 2 || d.Rows[0][1] != nil || *d.Rows[1][1] != "<&>" {
		t.Fatal(d, err)
	}
	for _, raw := range []string{`<!DOCTYPE data [<!ENTITY e SYSTEM "file:///private">]><data/>`, `<rows><row><id><nested/></id></row></rows>`, `<rows><row><id>1</id><id>2</id></row></rows>`} {
		if _, err := readXML([]byte(raw)); err == nil {
			t.Fatal("accepted unsafe or ambiguous XML")
		}
	}
}
func TestDelimitedSafetyAndLimits(t *testing.T) {
	data := Data{Columns: []string{"name"}, Rows: [][]*string{{text("=HYPERLINK()")}, {text(`\N`)}, {nil}, {text("")}}}
	raw, err := encodeText(data, Options{Format: "csv", ProtectFormulas: true})
	if err != nil || !strings.Contains(string(raw), "'=HYPERLINK()") {
		t.Fatal(string(raw), err)
	}
	for _, raw := range []string{"x,x\n1,2", "x,y\n1", "x\n\"unclosed", "x\n" + strings.Repeat("v\n", MaxRows+1)} {
		if _, err := decodeText([]byte(raw), Options{Format: "csv"}); err == nil {
			t.Fatal("accepted malformed or excessive CSV")
		}
	}
	if err := Validate(Data{Columns: []string{"x"}, Rows: [][]*string{{text(strings.Repeat("x", MaxBytes+1))}}}); err == nil {
		t.Fatal("accepted excessive data")
	}
}
func TestWorkbookSheetsDatesAndPrecision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	f := excelize.NewFile()
	_, _ = f.NewSheet("second")
	_ = f.SetCellStr("second", "A1", "date")
	_ = f.SetCellStr("second", "B1", "id")
	_ = f.SetCellFloat("second", "A2", 46000, 0, 64)
	style, err := f.NewStyle(&excelize.Style{NumFmt: 14})
	if err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellStyle("second", "A2", "A2", style)
	_ = f.SetCellStr("second", "B2", "9007199254740993123")
	if err = f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	names, err := workbookSheets(path)
	if err != nil || len(names) != 2 {
		t.Fatal(names, err)
	}
	d, err := Read(context.Background(), Options{Format: "xlsx", Path: path, Table: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if *d.Rows[0][0] != "2025-12-09" || *d.Rows[0][1] != "9007199254740993123" {
		t.Fatal(d.Rows)
	}
}
func TestSQLiteReadOnlyAndCancelledWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := writeSQLite(ctx, Options{Path: path, Table: "data"}, Data{Columns: []string{"id"}}); err == nil {
		t.Fatal("cancelled write succeeded")
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	if _, err := sqliteTables(context.Background(), missing); err == nil {
		t.Fatal("read-only open created a missing database")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("missing database was created")
	}
}

func TestJSONRejectsDuplicateKeysAndDeepNesting(t *testing.T) {
	for _, raw := range []string{`[{"id":1,"id":2}]`, strings.Repeat("[", 40) + "0" + strings.Repeat("]", 40)} {
		if _, err := readJSON([]byte(raw)); err == nil {
			t.Fatal("accepted ambiguous or excessive JSON")
		}
	}
}

func TestExternalDelimitedUNCPath(t *testing.T) {
	d, err := decodeText([]byte("path\n"+`\\server\share`+"\n"), Options{Format: "csv"})
	if err != nil || *d.Rows[0][0] != `\\server\share` {
		t.Fatal(d, err)
	}
}
