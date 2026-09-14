package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateCopiesOnlySelectedInstaller(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	ws := makeWorkspace(t, "Imports")
	from := filepath.Join(t.TempDir(), "Setup With Spaces.msi")
	if err := os.WriteFile(from, []byte("installer bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(from), "companion.dat"), []byte("leave here"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, setup := range []string{"", `nested\renamed.msi`} {
		name := "Default"
		if setup != "" {
			name = "Renamed"
		}
		ref, err := ws.Create(CreateOptions{Name: name, Group: `Tools\Office`, SetupFrom: from, SetupFile: setup})
		if err != nil {
			t.Fatal(err)
		}
		app := ws.Inspect(ref)
		if app.Status != "valid" {
			t.Fatal(app.Error)
		}
		destination := filepath.Join(ref.Path, "source", filepath.FromSlash("Setup With Spaces.msi"))
		if setup != "" {
			destination = filepath.Join(ref.Path, "source", "nested", "renamed.msi")
		}
		data, err := os.ReadFile(destination)
		if err != nil || string(data) != "installer bytes" {
			t.Fatal(string(data), err)
		}
		if _, err := os.Stat(filepath.Join(ref.Path, "source", "companion.dat")); !os.IsNotExist(err) {
			t.Fatal("copied an unselected file", err)
		}
		if _, err := ws.Create(CreateOptions{Name: name, Group: `Tools\Office`, SetupFrom: from}); err == nil {
			t.Fatal("overwrote an existing application")
		}
	}
	data, err := os.ReadFile(from)
	if err != nil || string(data) != "installer bytes" {
		t.Fatal("original installer changed", err)
	}
}

func TestCreateImportRejectsInvalidSourcesAndRollsBack(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	ws := makeWorkspace(t, "Imports")
	from := filepath.Join(t.TempDir(), "setup.exe")
	if err := os.WriteFile(from, []byte("installer"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, options := range []CreateOptions{
		{Name: "Bad", SetupFrom: from + "missing"},
		{Name: "Bad", SetupFrom: filepath.Dir(from)},
		{Name: "Bad", SetupFrom: from, SetupFile: "../escape"},
		{Name: "Bad", SetupFrom: from, SetupFile: "CON"},
		{Name: "Bad", SetupFrom: from, SourceDirectory: ".", SetupFile: AppConfigFile},
		// Forces config persistence to fail after copying; rollback must remove
		// only this request's file and newly created empty directories.
		{Name: "Bad", SetupFrom: from, SourceDirectory: AppConfigFile},
	} {
		if _, err := ws.Create(options); err == nil {
			t.Fatalf("accepted %#v", options)
		}
		if _, err := os.Stat(filepath.Join(ws.AppsDir(), "bad")); !os.IsNotExist(err) {
			t.Fatal("failed import left application behind", err)
		}
	}
}
