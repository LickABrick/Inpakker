package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicReplacementAndFailurePreservesOriginal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	if err := JSON(path, map[string]string{"name": "before"}, 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := JSON(path, make(chan int), 0600); err == nil {
		t.Fatal("unsupported JSON encoded")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed encoding changed active config")
	}
	if err := JSON(path, map[string]string{"name": "after"}, 0600); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if bytes.Equal(before, after) {
		t.Fatal("replacement did not occur")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("temporary file leaked", entries)
	}
	// Replacement onto a directory must fail and remove its temporary file.
	dir := filepath.Join(root, "directory")
	os.Mkdir(dir, 0700)
	if err := Write(dir, []byte("bad"), 0600); err == nil {
		t.Fatal("replaced directory")
	}
	entries, _ = os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatal("temporary file leaked on failure")
	}
}
