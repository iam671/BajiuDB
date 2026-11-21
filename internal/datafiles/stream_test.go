package datafiles

import (
	"context"
	"errors"
	"github.com/xuri/excelize/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testSource(data Data) Source {
	return func(header func([]string) error, row func([]*string) error) error {
		if err := header(data.Columns); err != nil {
			return err
		}
		for _, v := range data.Rows {
			if err := row(v); err != nil {
				return err
			}
		}
		return nil
	}
}
func TestStreamingFormatRoundTrips(t *testing.T) {
	data := Data{Columns: []string{"id", "note", "bytes", "space"}, Rows: [][]*string{
		{text("9007199254740993123"), text("中文\r\n a,\"b\"\rlast\n"), text("0x00FF"), text("SRID=4326;WKB=0x01010000000000000000003E400000000000003440")},
		{nil, text(""), text("0x"), text(`\N`)},
	}}
	for _, format := range []string{"txt", "csv", "json", "xml", "xlsx", "db"} {
		t.Run(format, func(t *testing.T) {
			o := Options{Format: format, Path: filepath.Join(t.TempDir(), "data."+format), Table: "data"}
			count, err := WriteStream(context.Background(), o, testSource(data))
			if err != nil || count != 2 {
				t.Fatal(count, err)
			}
			names, err := Inspect(context.Background(), o)
			if err != nil || len(names) != 1 {
				t.Fatal(names, err)
			}
			o.Table = names[0]
			got, err := Read(context.Background(), o)
			if err != nil || !reflect.DeepEqual(got, data) {
				t.Fatalf("roundtrip mismatch: %v %#v", err, got)
			}
		})
	}
}
func TestStreamingValidationCancellationAndFormulaProtection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "cancel.csv")
	if _, err := WriteStream(ctx, Options{Format: "csv", Path: path}, testSource(Data{Columns: []string{"x"}})); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled before creating a file", err)
	}
	for _, source := range []Source{
		func(h func([]string) error, r func([]*string) error) error { return r([]*string{text("x")}) },
		func(h func([]string) error, r func([]*string) error) error { return h([]string{"a", "a"}) },
		func(h func([]string) error, r func([]*string) error) error {
			if err := h([]string{"a"}); err != nil {
				return err
			}
			return r(nil)
		},
	} {
		if _, err := WriteStream(context.Background(), Options{Format: "csv", Path: filepath.Join(t.TempDir(), "bad.csv")}, source); err == nil {
			t.Fatal("accepted invalid stream")
		}
	}
	path = filepath.Join(t.TempDir(), "formula.csv")
	_, err := WriteStream(context.Background(), Options{Format: "csv", Path: path, ProtectFormulas: true}, testSource(Data{Columns: []string{" =header"}, Rows: [][]*string{{text("\t=1+1")}}}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(context.Background(), Options{Format: "csv", Path: path})
	if err != nil || got.Columns[0] != "' =header" || *got.Rows[0][0] != "'\t=1+1" {
		t.Fatal(got, err)
	}
}
func TestWorkbookStreamSplitsAtFormatBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sheets.xlsx")
	sink, err := newWorkbookStream(path, []string{"value"})
	if err != nil {
		t.Fatal(err)
	}
	defer sink.close()
	s := sink.(*workbookStream)
	s.next = 1048576
	if err = s.row([]*string{text("last")}); err != nil {
		t.Fatal(err)
	}
	if err = s.row([]*string{text("next")}); err != nil {
		t.Fatal(err)
	}
	if err = s.finish(); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, item := range []struct{ sheet, axis, value string }{{"Sheet1", "A1048576", "last"}, {"Sheet2", "A1", "value"}, {"Sheet2", "A2", "next"}} {
		v, err := f.GetCellValue(item.sheet, item.axis)
		if err != nil || v != item.value {
			t.Fatal(item, v, err)
		}
	}
}
func TestRowDigestRetainsDuplicatesNullAndEmpty(t *testing.T) {
	var a, b, c [32]byte
	addRowDigest(&a, []*string{nil})
	addRowDigest(&b, []*string{text("")})
	if a == b {
		t.Fatal("NULL equals empty")
	}
	addRowDigest(&c, []*string{text("中文"), text("x")})
	addRowDigest(&c, []*string{nil})
	addRowDigest(&a, []*string{text("中文"), text("x")})
	if a != c {
		t.Fatal("order affects multiset")
	}
	addRowDigest(&c, []*string{nil})
	if a == c {
		t.Fatal("duplicate not counted")
	}
}
