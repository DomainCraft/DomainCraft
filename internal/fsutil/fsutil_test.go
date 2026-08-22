package fsutil

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteCreatesDirsAndWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "file.txt")

	if err := AtomicWrite(path, []byte("hello")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", data, "hello")
	}
}

func TestAtomicWriteOverwritesAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	if err := AtomicWrite(path, []byte(`{"v":1}`)); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := AtomicWrite(path, []byte(`{"v":2}`)); err != nil {
		t.Fatalf("second write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != `{"v":2}` {
		t.Errorf("content = %q, want the second write to win", data)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "cache.json" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteJSONReadJSONRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meta.json")

	type payload struct {
		ID      string   `json:"id"`
		Version string   `json:"version"`
		Tags    []string `json:"tags"`
	}
	in := payload{ID: "csharp-restful", Version: "1.2.3", Tags: []string{"a", "b"}}

	if err := WriteJSON(path, in); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var out payload
	if err := ReadJSON(path, &out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.ID != in.ID || out.Version != in.Version || len(out.Tags) != len(in.Tags) || out.Tags[0] != in.Tags[0] {
		t.Errorf("roundtrip = %+v, want %+v", out, in)
	}
}

func TestWriteJSONRejectsUnmarshalableValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := WriteJSON(path, map[string]any{"ch": make(chan int)}); err == nil {
		t.Fatal("expected an error for an unmarshalable value")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("failed write should not leave a file behind, got %v", err)
	}
}

func TestReadJSONMissingFileReturnsNotExist(t *testing.T) {
	var v map[string]string
	err := ReadJSON(filepath.Join(t.TempDir(), "absent.json"), &v)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}

func TestReadJSONCorruptFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var v map[string]string
	var syn *json.SyntaxError
	if err := ReadJSON(path, &v); !errors.As(err, &syn) {
		t.Errorf("err = %v, want a JSON syntax error", err)
	}
}
