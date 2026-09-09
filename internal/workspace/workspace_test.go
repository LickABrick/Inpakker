package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverAllAllowsMissingApplicationsDirectory(t *testing.T) {
	ws := &Workspace{Root: t.TempDir()}
	selection, err := ws.Discover(nil, true)
	if err != nil {
		t.Fatalf("Discover returned %v", err)
	}
	if len(selection.Apps) != 0 || len(selection.Skipped) != 0 {
		t.Fatalf("Discover returned %#v, want empty selection", selection)
	}
}

func TestCreateSupportsGroupsAndRejectsEscapingPaths(t *testing.T) {
	ws := &Workspace{Root: t.TempDir()}
	ref, err := ws.Create(CreateOptions{Name: "example", Group: "team/tools", Source: "source", SetupFile: "setup.exe"})
	if err != nil {
		t.Fatalf("Create returned %v", err)
	}
	want := filepath.Join("team", "tools", "example")
	if ref.Relative != want {
		t.Fatalf("relative path = %q, want %q", ref.Relative, want)
	}
	if _, err := os.Stat(filepath.Join(ref.Path, "app.config.json")); err != nil {
		t.Fatalf("stat config: %v", err)
	}

	for _, options := range []CreateOptions{
		{Name: "bad-source", Source: "../outside"},
		{Name: "bad-output", OutputDir: "C:\\output"},
		{Name: "bad-group", Group: "../outside"},
	} {
		if _, err := ws.Create(options); err == nil {
			t.Fatalf("Create(%+v) accepted unsafe path", options)
		}
	}
}
