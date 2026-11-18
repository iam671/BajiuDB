package main

import (
	"bajiudb/internal/connections"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpDirectoryCancellationDoesNotRead(t *testing.T) {
	path, err := dumpToDirectory("", "qa", true, func() (connections.AdminResult, error) {
		t.Fatal("cancelling the directory picker must not read the database")
		return connections.AdminResult{}, nil
	})
	if err != nil || path != "" {
		t.Fatal(path, err)
	}
}

func TestDumpDirectoryWritesUniqueSQLFiles(t *testing.T) {
	dir := t.TempDir()
	text := "-- QA\nCREATE TABLE qa(id BIGINT);\nINSERT INTO qa VALUES(9007199254740993);\n"
	var previous string
	for _, includeData := range []bool{false, true, true} {
		path, err := dumpToDirectory(dir, `odd/qa:db`, includeData, func() (connections.AdminResult, error) {
			return connections.AdminResult{Text: text}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(path) != dir || filepath.Ext(path) != ".sql" || path == previous {
			t.Fatal("unsafe or reused destination", path)
		}
		mode := "structure-"
		if includeData {
			mode = "structure-data-"
		}
		if !strings.HasPrefix(filepath.Base(path), "BajiuDB-odd_qa_db-"+mode) {
			t.Fatal("unexpected filename", path)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != text {
			t.Fatal("dump output changed", err)
		}
		previous = path
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 3 {
		t.Fatal("an export overwrote an existing backup")
	}
}

func TestDumpDirectoryDoesNotSaveFailedResults(t *testing.T) {
	for _, backendError := range []bool{false, true} {
		dir := t.TempDir()
		path, err := dumpToDirectory(dir, "qa", true, func() (connections.AdminResult, error) {
			if backendError {
				return connections.AdminResult{}, errors.New("connection interrupted")
			}
			return connections.AdminResult{Text: "partial output", Error: "server refused export"}, nil
		})
		files, _ := os.ReadDir(dir)
		if err == nil || path != "" || len(files) != 0 {
			t.Fatal("failed dump was saved", path, err)
		}
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := dumpToDirectory(file, "qa", true, func() (connections.AdminResult, error) {
		t.Fatal("invalid directory must fail before database access")
		return connections.AdminResult{}, nil
	}); err == nil {
		t.Fatal("accepted a file as a directory")
	}
}
