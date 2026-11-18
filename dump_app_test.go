package main

import (
	"bajiudb/internal/connections"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDumpSaveAsCancellationDoesNotRead(t *testing.T) {
	path, err := dumpToFile("", func() (connections.AdminResult, error) {
		t.Fatal("cancelling Save As must not read the database")
		return connections.AdminResult{}, nil
	})
	if err != nil || path != "" {
		t.Fatal(path, err)
	}
}

func TestDumpSaveAsUsesExactFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-custom-backup.sql")
	text := "-- QA\nCREATE TABLE qa(id BIGINT);\nINSERT INTO qa VALUES(9007199254740993);\n"
	if err := os.WriteFile(path, []byte("previous backup"), 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := dumpToFile(path, func() (connections.AdminResult, error) { return connections.AdminResult{Text: text}, nil })
	if err != nil || saved != path {
		t.Fatal("did not use selected filename", saved, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != text {
		t.Fatal("dump output changed", err)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("unexpected extra export or temporary file", files)
	}
}

func TestDumpDefaultFilename(t *testing.T) {
	for _, tc := range []struct{ database, filename string }{{"test", "test.sql"}, {"客户", "客户.sql"}, {`odd/qa:db`, "odd_qa_db.sql"}, {"", "database.sql"}, {"CON", "_CON.sql"}, {"aux.backup", "_aux.backup.sql"}} {
		if got := dumpFilename(tc.database); got != tc.filename {
			t.Fatal(tc.database, got)
		}
	}
}

func TestDumpSaveAsDoesNotChangeFilesOnFailure(t *testing.T) {
	for _, backendError := range []bool{false, true} {
		dir := t.TempDir()
		destination := filepath.Join(dir, "backup.sql")
		if err := os.WriteFile(destination, []byte("previous backup"), 0600); err != nil {
			t.Fatal(err)
		}
		path, err := dumpToFile(destination, func() (connections.AdminResult, error) {
			if backendError {
				return connections.AdminResult{}, errors.New("connection interrupted")
			}
			return connections.AdminResult{Text: "partial output", Error: "server refused export"}, nil
		})
		files, _ := os.ReadDir(dir)
		data, _ := os.ReadFile(destination)
		if err == nil || path != "" || len(files) != 1 || string(data) != "previous backup" {
			t.Fatal("failed dump was saved", path, err)
		}
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := dumpToFile(filepath.Join(file, "qa.sql"), func() (connections.AdminResult, error) {
		t.Fatal("invalid directory must fail before database access")
		return connections.AdminResult{}, nil
	}); err == nil {
		t.Fatal("accepted an invalid destination")
	}
}

func TestDumpSaveAsWriteFailureCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "selected.sql")
	_, err := dumpToFile(path, func() (connections.AdminResult, error) {
		// The selected filename becomes a directory while the server is exporting.
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		return connections.AdminResult{Text: "CREATE TABLE qa(id INT);"}, nil
	})
	files, _ := os.ReadDir(dir)
	if err == nil || len(files) != 1 || !files[0].IsDir() {
		t.Fatal("failed export left a partial file", files, err)
	}
}
