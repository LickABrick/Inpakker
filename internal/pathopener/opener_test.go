package pathopener

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExplorerDirectArgumentsAndErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Customer Apps", "Mozilla Firefox")
	os.MkdirAll(root, 0755)
	calls := 0
	e := Explorer{Start: func(name string, args ...string) error {
		calls++
		if name != "explorer.exe" || len(args) != 1 || args[0] != root {
			t.Fatalf("%s %#v", name, args)
		}
		return nil
	}}
	if err := e.OpenDirectory(root); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	for _, path := range []string{"", filepath.Join(root, "missing")} {
		if err := e.OpenDirectory(path); err == nil {
			t.Fatal("accepted missing folder")
		}
	}
	e.Start = func(string, ...string) error { return errors.New("start failure") }
	if err := e.OpenDirectory(root); err == nil {
		t.Fatal("lost start failure")
	}
}
