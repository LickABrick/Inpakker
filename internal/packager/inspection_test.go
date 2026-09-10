package packager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/workspace"
)

func inspectionWorkspace(t *testing.T) (*workspace.Workspace, workspace.AppRef, string) {
	t.Helper()
	root := t.TempDir()
	utility := filepath.Join(root, "IntuneWinAppUtil.exe")
	if err := os.WriteFile(utility, []byte("utility-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	setup, err := workspace.Initialize(workspace.SetupOptions{Root: root, AppsDir: "apps", OutputDir: "output", IntuneWinAppUtil: utility})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := setup.Workspace.Create(workspace.CreateOptions{Name: "browser", Source: "source", SetupFile: "setup.exe", OutputDir: "output"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "setup.exe"), []byte("installer-v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	return setup.Workspace, ref, utility
}

func TestInspectBuildStateLifecycleIsReadOnly(t *testing.T) {
	ws, ref, utility := inspectionWorkspace(t)
	service := Service{Workspace: ws}

	if got := service.InspectBuildState(ref); got.State != BuildStateNotBuilt {
		t.Fatalf("initial state = %q (%s), want %q", got.State, got.Reason, BuildStateNotBuilt)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, buildcache.FileName)); !os.IsNotExist(err) {
		t.Fatalf("inspection created cache: %v", err)
	}

	app := ws.Inspect(ref)
	output, err := ws.OutputDir(ref, app.Config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(output, "browser.intunewin")
	if err := os.WriteFile(artifact, []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := buildcache.Fingerprint(ref.Path, app.Config, output, utility)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := buildcache.Load(ws.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(filepath.ToSlash(ref.Relative), fingerprint, ref.Path, []string{artifact}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(ws.Root); err != nil {
		t.Fatal(err)
	}

	current := service.InspectBuildState(ref)
	if current.State != BuildStateCurrent || len(current.Artifacts) != 1 || current.Artifacts[0] != artifact {
		t.Fatalf("current inspection = %#v", current)
	}
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	missing := service.InspectBuildState(ref)
	if missing.State != BuildStateNeedsBuild || missing.Reason != "recorded build output is missing" {
		t.Fatalf("missing artifact inspection = %#v", missing)
	}
	if err := os.WriteFile(artifact, []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "setup.exe"), []byte("installer-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := service.InspectBuildState(ref)
	if changed.State != BuildStateNeedsBuild {
		t.Fatalf("changed state = %q (%s), want %q", changed.State, changed.Reason, BuildStateNeedsBuild)
	}
}

func TestInspectBuildStateReportsUnavailableAndUnknown(t *testing.T) {
	ws, ref, utility := inspectionWorkspace(t)
	if err := os.Remove(filepath.Join(ref.Path, "source", "setup.exe")); err != nil {
		t.Fatal(err)
	}
	if got := (Service{Workspace: ws}).InspectBuildState(ref); got.State != BuildStateUnavailable {
		t.Fatalf("invalid application state = %q, want %q", got.State, BuildStateUnavailable)
	}
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "setup.exe"), []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(utility); err != nil {
		t.Fatal(err)
	}
	if got := (Service{Workspace: ws}).InspectBuildState(ref); got.State != BuildStateUnavailable {
		t.Fatalf("missing utility state = %q, want %q", got.State, BuildStateUnavailable)
	}
	if got := (Service{}).InspectBuildState(ref); got.State != BuildStateUnknown {
		t.Fatalf("missing workspace state = %q, want %q", got.State, BuildStateUnknown)
	}
}
