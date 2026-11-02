package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListWithGroupsSnapshot(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	ws := makeWorkspace(t, "Snapshot")
	ref, err := ws.Create(CreateOptions{Name: "App", Group: "Group10", SetupFile: "setup.exe"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(ws.AppsDir(), "Group2", "Empty"), filepath.Join(ws.AppsDir(), ".git", "hidden"), filepath.Join(ref.Path, "source", "nested")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	apps, groups, err := ws.ListWithGroups(context.Background())
	if err != nil || len(apps) != 1 || apps[0].Ref != ref {
		t.Fatalf("apps = %#v, err = %v", apps, err)
	}
	want := []string{"Group2", filepath.Join("Group2", "Empty"), "Group10"}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %v, want %v", groups, want)
	}
	selection, err := ws.Discover(nil, true)
	if err != nil || !reflect.DeepEqual(selection.Apps, []AppRef{ref}) {
		t.Fatalf("discovery = %#v, %v", selection, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	apps, groups, err = ws.ListWithGroups(ctx)
	if !errors.Is(err, context.Canceled) || apps != nil || groups != nil {
		t.Fatalf("cancelled snapshot = %#v, %#v, %v", apps, groups, err)
	}
}

func TestDiagnoseAppsUsesLoadedSnapshot(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	ws := makeWorkspace(t, "Diagnostics")
	// There are no applications on disk: the supplied snapshot is authoritative.
	checks := ws.DiagnoseApps([]App{{Status: "valid"}, {Status: "invalid"}}, nil)
	last := checks[len(checks)-1]
	if last.Name != "Applications" || last.Status != CheckFailure || last.Detail != "1 of 2 invalid" {
		t.Fatal(last)
	}
	checks = ws.DiagnoseApps(nil, errors.New("inventory failed"))
	last = checks[len(checks)-1]
	if last.Status != CheckFailure || last.Detail != "inventory failed" {
		t.Fatal(last)
	}
}

func TestListWithGroupsMissingAppsDirectory(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	ws := makeWorkspace(t, "Missing")
	if err := os.RemoveAll(ws.AppsDir()); err != nil {
		t.Fatal(err)
	}
	apps, groups, err := ws.ListWithGroups(context.Background())
	if err != nil || len(apps) != 0 || len(groups) != 0 {
		t.Fatalf("missing inventory = %#v, %#v, %v", apps, groups, err)
	}
}
