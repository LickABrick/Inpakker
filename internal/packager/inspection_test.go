package packager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/workspace"
)

func inspectionWorkspace(t *testing.T) (*workspace.Workspace, workspace.AppRef, string) {
	t.Helper()
	t.Setenv("INPAKKER_HOME", t.TempDir())
	root := t.TempDir()
	utility := filepath.Join(root, "IntuneWinAppUtil.exe")
	if err := os.WriteFile(utility, []byte("utility-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	user := config.DefaultUser()
	user.Tools.ContentPrepTool.Path = utility
	if err := config.SaveUser(&user); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.CreateWorkspace(workspace.CreateWorkspaceOptions{Root: root, Name: "Test"})
	if err != nil {
		t.Fatal(err)
	}

	ref, err := ws.Create(workspace.CreateOptions{Name: "browser", SourceDirectory: "source", SetupFile: "setup.exe", OutputDirectory: "output"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "setup.exe"), []byte("installer-v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws, ref, utility
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
	fingerprint, err := buildcache.Fingerprint(ref.Path, &app.Effective.AppConfig, output, utility)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := buildcache.Load(cacheDirectory(t, ws))
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Set(app.Config.ID, fingerprint, ref.Path, []string{artifact}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(cacheDirectory(t, ws)); err != nil {
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

func cacheDirectory(t *testing.T, ws *workspace.Workspace) string {
	t.Helper()
	path, err := ws.CacheDirectory()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEffectiveSettingsAndUUIDCacheIdentity(t *testing.T) {
	ws, ref, utility := inspectionWorkspace(t)
	app := ws.Inspect(ref)
	// Convert the fixture's explicit source/output into inherited settings.
	app.Config.SourceDirectory = ""
	app.Config.OutputDirectory = ""
	if err := config.SaveApp(filepath.Join(ref.Path, workspace.AppConfigFile), app.Config); err != nil {
		t.Fatal(err)
	}
	app = ws.Inspect(ref)
	output, _ := ws.OutputDir(ref, app.Config)
	os.MkdirAll(output, 0755)
	artifact := filepath.Join(output, "browser.intunewin")
	os.WriteFile(artifact, []byte("package"), 0644)
	fingerprint, err := buildcache.Fingerprint(ref.Path, &app.Effective.AppConfig, output, utility)
	if err != nil {
		t.Fatal(err)
	}
	cache, _ := buildcache.Load(cacheDirectory(t, ws))
	cache.Set(app.Config.ID, fingerprint, ref.Path, []string{artifact})
	cache.Save(cacheDirectory(t, ws))
	moved := filepath.Join(ws.AppsDir(), "moved-browser")
	if err := os.Rename(ref.Path, moved); err != nil {
		t.Fatal(err)
	}
	movedRef := workspace.AppRef{Path: moved, Relative: ws.Relative(moved)}
	if got := (Service{Workspace: ws}).InspectBuildState(movedRef); got.State != BuildStateCurrent {
		t.Fatal("move lost UUID identity", got)
	}
	ws.Config.OutputDirectory = "packages"
	if got := (Service{Workspace: ws}).InspectBuildState(movedRef); got.State != BuildStateNeedsBuild {
		t.Fatal("workspace output did not affect effective fingerprint", got)
	}
	ws.Config.OutputDirectory = "output"
	ws.Config.SourceDirectory = "installer"
	os.MkdirAll(filepath.Join(moved, "installer"), 0755)
	os.WriteFile(filepath.Join(moved, "installer", "setup.exe"), []byte("installer-v1"), 0644)
	if got := (Service{Workspace: ws}).InspectBuildState(movedRef); got.State != BuildStateNeedsBuild {
		t.Fatal("workspace source did not affect effective fingerprint", got)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, buildcache.FileName)); !os.IsNotExist(err) {
		t.Fatal("cache leaked into workspace")
	}
}
