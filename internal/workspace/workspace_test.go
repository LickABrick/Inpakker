package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeCreatesValidPowerShellExample(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	result, err := Initialize(SetupOptions{Root: root, AppsDir: "packages", OutputDir: "artifacts", CreateExample: true})
	if err != nil {
		t.Fatalf("Initialize returned %v", err)
	}
	if result.Example == nil {
		t.Fatal("Initialize did not create an example")
	}
	app := result.Workspace.Inspect(*result.Example)
	if app.Status != "valid" {
		t.Fatalf("example status = %q: %s", app.Status, app.Error)
	}
	scriptPath := filepath.Join(result.Example.Path, "source", "install.ps1")
	if contents, err := os.ReadFile(scriptPath); err != nil || len(contents) == 0 {
		t.Fatalf("example script contents = %q, error = %v", contents, err)
	}
	data, err := os.ReadFile(filepath.Join(root, ConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config["appsDir"] != "packages" || config["defaultOutputDir"] != "artifacts" {
		t.Fatalf("unexpected global config: %#v", config)
	}
}

func TestDiagnoseReportsOptionalDecoderAsWarning(t *testing.T) {
	root := t.TempDir()
	utility := filepath.Join(root, "IntuneWinAppUtil.exe")
	if err := os.WriteFile(utility, []byte("utility"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Initialize(SetupOptions{Root: root, IntuneWinAppUtil: utility, CreateExample: true}); err != nil {
		t.Fatal(err)
	}
	checks := Diagnose(root)
	for _, check := range checks {
		if check.Name == "Decoder" && check.Status != CheckWarning {
			t.Fatalf("decoder check = %#v", check)
		}
		if check.Status == CheckFailure {
			t.Fatalf("unexpected failed check: %#v", check)
		}
	}
}

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
