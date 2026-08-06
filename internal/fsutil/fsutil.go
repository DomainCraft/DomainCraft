// Package fsutil provides small, generic filesystem helpers shared across the
// codebase (atomic writes, JSON persistence). Keeping them here avoids
// duplicating the write-to-temp-then-rename pattern in every cache.
package fsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to path via a temp file in the same directory
// followed by a rename, so a crash mid-write never leaves a truncated or
// corrupt file behind. The parent directory is created if missing.
func AtomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// WriteJSON marshals v as indented JSON and writes it to path atomically.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, data)
}

// ReadJSON reads path and unmarshals it into v. It returns os.ErrNotExist when
// the file does not exist.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
