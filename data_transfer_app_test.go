package main

import (
	"bajiudb/internal/datafiles"
	"context"
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
