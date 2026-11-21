package main

import (
	"bajiudb/internal/datafiles"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransferPublishUsesExactPathAndProtectsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chosen.json")
	data := datafiles.Data{Columns: []string{"id"}, Rows: [][]*string{}}
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := saveTransferFile(ctx, path, datafiles.Options{Format: "json"}, data); err == nil {
		t.Fatal("cancelled export saved")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "old" {
		t.Fatal("old file changed")
	}
	saved, err := saveTransferFile(context.Background(), path, datafiles.Options{Format: "json"}, data)
	if err != nil || saved != path {
		t.Fatal(saved, err)
	}
	raw, _ = os.ReadFile(path)
	if !strings.Contains(string(raw), `"columns"`) {
		t.Fatal("not JSON")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("temporary files leaked")
	}
}
func TestStreamingExportHasNoRowOrFileSizeCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "all.csv")
	payload := strings.Repeat("x", 1024)
	source := func(header func([]string) error, row func([]*string) error) error {
		if err := header([]string{"id", "payload", "binary", "space"}); err != nil {
			return err
		}
		for i := 0; i < 66001; i++ {
			id := fmt.Sprint(i)
			hex, geo := "0x00FF", "SRID=0;WKB=0x0101000000000000000000F03F0000000000000040"
			if err := row([]*string{&id, &payload, &hex, &geo}); err != nil {
				return err
			}
		}
		return nil
	}
	result, err := exportTransferTable(context.Background(), datafiles.Options{Format: "csv", Table: "qa"}, func() (string, error) { return path, nil }, source)
	if err != nil || result.Rows != 66001 || result.Path != path {
		t.Fatal(result, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 64<<20 {
		t.Fatal("did not exceed former 64 MB limit", info, err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.Seek(-2048, io.SeekEnd); err != nil {
		t.Fatal(err)
	}
	tail, err := io.ReadAll(f)
	if err != nil || !strings.Contains(string(tail), `"66000"`) {
		t.Fatal("missing final row", err)
	}
}
func TestStreamingExportCancellationAndFailureProtectExistingTarget(t *testing.T) {
	for _, failure := range []string{"read", "cancel", "destination"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "old.json")
			if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			source := func(header func([]string) error, row func([]*string) error) error {
				if err := header([]string{"x"}); err != nil {
					return err
				}
				v := "one"
				if err := row([]*string{&v}); err != nil {
					return err
				}
				if failure == "cancel" {
					cancel()
					return nil
				}
				if failure == "destination" {
					return os.Remove(path)
				}
				return errors.New("source failed")
			}
			if failure == "destination" {
				source = func(h func([]string) error, r func([]*string) error) error {
					if err := h([]string{"x"}); err != nil {
						return err
					}
					if err := os.Remove(path); err != nil {
						return err
					}
					return os.Mkdir(path, 0700)
				}
			}
			result, err := exportTransferTable(ctx, datafiles.Options{Format: "json"}, func() (string, error) { return path, nil }, source)
			if err == nil || result.Path != "" {
				t.Fatal("published failed export", result, err)
			}
			if failure != "destination" {
				raw, _ := os.ReadFile(path)
				if string(raw) != "old" {
					t.Fatal("old file changed")
				}
			}
			files, _ := os.ReadDir(dir)
			if len(files) != 1 {
				t.Fatal("staging leaked", files)
			}
		})
	}
	called := false
	_, err := exportTransferTable(context.Background(), datafiles.Options{Format: "csv"}, func() (string, error) { return "", nil }, func(func([]string) error, func([]*string) error) error { called = true; return nil })
	if err != nil || called {
		t.Fatal("cancelled Save As read source", err)
	}
}
