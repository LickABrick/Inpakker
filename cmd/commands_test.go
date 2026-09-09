package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LickABrick/inpakker/types"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	err   error
	calls []runnerCall
}

func (r *fakeRunner) Run(_ context.Context, name string, args []string, _, _ io.Writer) error {
	r.calls = append(r.calls, runnerCall{name: name, args: append([]string(nil), args...)})
	return r.err
}

func TestDiscoverTargetsRejectsTraversalAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	writeApp(t, filepath.Join(root, "group", "one"), validApp("one"), true)
	writeApp(t, filepath.Join(root, "group", "two"), validApp("two"), true)

	targets, skipped, err := discoverTargets(root, []string{"group", "group/one", "../outside", "missing"}, false)
	if err != nil {
		t.Fatalf("discoverTargets returned %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("got %d targets, want 2: %v", len(targets), targets)
	}
	if skipped != 2 {
		t.Fatalf("got %d skipped targets, want 2", skipped)
	}
}

func TestDiscoverAllStopsAtApplicationRoot(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "group", "one")
	writeApp(t, appDir, validApp("one"), true)
	writeJSON(t, filepath.Join(appDir, "source", "nested", "app.config.json"), validApp("nested"))

	targets, skipped, err := discoverTargets(root, nil, true)
	if err != nil {
		t.Fatalf("discoverTargets returned %v", err)
	}
	if len(targets) != 1 || targets[0] != appDir {
		t.Fatalf("got targets %v, want only %s", targets, appDir)
	}
	if skipped != 0 {
		t.Fatalf("got %d skipped targets, want 0", skipped)
	}
}

func TestBuildSummarizesResultsAndReturnsFailure(t *testing.T) {
	workspace := enterWorkspace(t)
	utilPath := filepath.Join(workspace, "IntuneWinAppUtil.exe")
	writeFile(t, utilPath, "stub")
	writeGlobal(t, types.GlobalConfig{
		IntuneWinAppUtil:     utilPath,
		AppsDir:              "packages",
		DefaultOutputDir:     "artifacts",
		MuteIntuneWinAppUtil: true,
	})
	writeApp(t, filepath.Join(workspace, "packages", "good"), validApp("good"), true)
	writeApp(t, filepath.Join(workspace, "packages", "bad"), validApp("bad"), false)

	runner := &fakeRunner{}
	command := newBuildCmd(runner)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"good", "bad", "missing"})

	err := command.Execute()
	var reported reportedError
	if !errors.As(err, &reported) {
		t.Fatalf("build error = %v, want reportedError", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner called %d times, want 1", len(runner.calls))
	}
	if !containsSequence(runner.calls[0].args, "-o", filepath.Join("packages", "good", "artifacts")) {
		t.Fatalf("runner args do not contain configured output: %v", runner.calls[0].args)
	}
	for _, expected := range []string{"Building 2 applications", "1 succeeded", "1 failed", "1 skipped"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout %q does not contain %q", stdout.String(), expected)
		}
	}
	if !strings.Contains(stderr.String(), "FAILED bad: access setup file") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestBuildReturnsFailureWhenPackagerFails(t *testing.T) {
	workspace := enterWorkspace(t)
	utilPath := filepath.Join(workspace, "IntuneWinAppUtil.exe")
	writeFile(t, utilPath, "stub")
	writeGlobal(t, types.GlobalConfig{IntuneWinAppUtil: utilPath, MuteIntuneWinAppUtil: true})
	writeApp(t, filepath.Join(workspace, "apps", "example"), validApp("example"), true)

	runner := &fakeRunner{err: errors.New("packager exited with code 1")}
	command := newBuildCmd(runner)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"example"})

	err := command.Execute()
	var reported reportedError
	if !errors.As(err, &reported) {
		t.Fatalf("build error = %v, want reportedError", err)
	}
	if !strings.Contains(stdout.String(), "0 succeeded") || !strings.Contains(stdout.String(), "1 failed") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "packager exited with code 1") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestNewUsesConfiguredAppsDirAndDoesNotOverwrite(t *testing.T) {
	workspace := enterWorkspace(t)
	writeGlobal(t, types.GlobalConfig{AppsDir: "packages"})
	command := newNewCmd()
	var stdout bytes.Buffer
	command.SetOut(&stdout)

	if err := runNew(command, "example"); err != nil {
		t.Fatalf("first runNew returned %v", err)
	}
	configPath := filepath.Join(workspace, "packages", "example", "app.config.json")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read created config: %v", err)
	}
	if err := runNew(command, "example"); err == nil {
		t.Fatal("second runNew returned nil")
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after second call: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing config was modified")
	}
	if err := runNew(command, "../outside"); err == nil {
		t.Fatal("runNew accepted a traversal path")
	}
	if err := runNew(command, "CON.txt"); err == nil {
		t.Fatal("runNew accepted a reserved Windows directory name")
	}
}

func TestValidateFindsNestedAppsAndReturnsFailure(t *testing.T) {
	workspace := enterWorkspace(t)
	writeGlobal(t, types.GlobalConfig{AppsDir: "packages"})
	writeApp(t, filepath.Join(workspace, "packages", "group", "good"), validApp("good"), true)
	writeApp(t, filepath.Join(workspace, "packages", "group", "bad"), validApp("bad"), false)

	command := newValidateCmd()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	err := command.Execute()
	var reported reportedError
	if !errors.As(err, &reported) {
		t.Fatalf("validate error = %v, want reportedError", err)
	}
	if !strings.Contains(stdout.String(), "Validating 2 applications") ||
		!strings.Contains(stdout.String(), "1 succeeded") ||
		!strings.Contains(stdout.String(), "1 failed") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "FAILED bad") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func enterWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("enter workspace: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return dir
}

func writeGlobal(t *testing.T, cfg types.GlobalConfig) {
	t.Helper()
	writeJSON(t, "inpakker.config.json", cfg)
}

func writeApp(t *testing.T, dir string, cfg types.AppConfig, withSetup bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, cfg.Source), 0o755); err != nil {
		t.Fatalf("create app source: %v", err)
	}
	writeJSON(t, filepath.Join(dir, "app.config.json"), cfg)
	if withSetup {
		writeFile(t, filepath.Join(dir, cfg.Source, cfg.SetupFile), "setup")
	}
}

func validApp(name string) types.AppConfig {
	return types.AppConfig{
		Name:        name,
		DisplayName: strings.ToUpper(name[:1]) + name[1:],
		Source:      "source",
		SetupFile:   "setup.exe",
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create JSON parent: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write JSON: %v", err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create file parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func containsSequence(values []string, first, second string) bool {
	for index := 0; index+1 < len(values); index++ {
		if values[index] == first && values[index+1] == second {
			return true
		}
	}
	return false
}
