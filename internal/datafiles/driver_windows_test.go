//go:build windows

package datafiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsDriverProtocol(t *testing.T) {
	if os.Getenv("BAJIU_TRANSFER_DRIVER_TEST") != "1" {
		t.Skip("explicit local driver integration test required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r, err := driverCall(ctx, "capabilities", Options{}, Data{})
	if err != nil {
		t.Skip("system driver host unavailable in this environment:", err)
	}
	if !r.ODBC {
		t.Fatal("Windows ODBC adapter missing")
	}
	t.Logf("ACE available: %v", r.ACE)
	if !r.ACE {
		return
	}
	for _, format := range []string{"xls", "mdb", "accdb", "dbf"} {
		t.Run(format, func(t *testing.T) {
			o := Options{Format: format, Path: filepath.Join(t.TempDir(), "data."+format), Table: "qa"}

			d := Data{Columns: []string{"id", "name"}, Rows: [][]*string{{text("1234567890123456789"), text("QA")}}}
			if format == "dbf" {
				d.Rows = append(d.Rows, []*string{text("2"), text("中文")})
			}
			if format != "dbf" {
				d.Rows = append(d.Rows, []*string{text("2"), text("中文")}, []*string{text("3"), nil}, []*string{text("4"), text("")}, []*string{text("5"), text(`\N`)}, []*string{text("6"), text(`\\server\share`)})
			}
			if err := driverWrite(context.Background(), o, d); err != nil {
				t.Fatal(err)
			}
			names, err := driverInspect(context.Background(), o)
			if err != nil || len(names) != 1 {
				t.Fatal(names, err)
			}
			o.Table = names[0]
			read, err := driverRead(context.Background(), o)
			if err != nil || len(read.Rows) != len(d.Rows) || *read.Rows[0][0] != *d.Rows[0][0] {
				t.Fatal(read, err)
			}
		})
	}
}

func TestWindowsODBCTransfer(t *testing.T) {
	if os.Getenv("BAJIU_TRANSFER_DRIVER_TEST") != "1" {
		t.Skip("explicit local driver integration test required")
	}
	path := filepath.Join(t.TempDir(), "qa.accdb")
	original := Data{Columns: []string{"id", "name"}, Rows: [][]*string{{text("9007199254740993123"), text("QA")}, {text("2"), text("中文")}, {text("3"), nil}, {text("4"), text("")}}}
	if err := driverWrite(context.Background(), Options{Format: "accdb", Path: path, Table: "original"}, original); err != nil {
		t.Fatal(err)
	}
	o := Options{Format: "odbc", ConnectionString: "Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=" + path + ";", Table: "odbc_export"}
	if err := driverWrite(context.Background(), o, original); err != nil {
		t.Fatal(err)
	}
	names, err := driverInspect(context.Background(), o)
	if err != nil || len(names) != 2 {
		t.Fatal(names, err)
	}
	got, err := driverRead(context.Background(), o)
	if err != nil || len(got.Rows) != len(original.Rows) || *got.Rows[0][0] != *original.Rows[0][0] {
		t.Fatal(got, err)
	}
	if err := driverWrite(context.Background(), o, original); err == nil {
		t.Fatal("overwrote an existing ODBC table")
	}
	after, err := driverRead(context.Background(), o)
	if err != nil || len(after.Rows) != len(original.Rows) {
		t.Fatal("existing ODBC data changed", after, err)
	}
}
