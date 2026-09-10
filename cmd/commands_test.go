package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LickABrick/inpakker/internal/buildcache"
	"github.com/LickABrick/inpakker/internal/config"
	workspace2 "github.com/LickABrick/inpakker/internal/workspace"
	"github.com/LickABrick/inpakker/types"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	err   error
	calls []runnerCall
	hook  func(name string, args []string) error
}

func (r *fakeRunner) Run(_ context.Context, name string, args []string, _, _ io.Writer) error {
	r.calls = append(r.calls, runnerCall{name: name, args: append([]string(nil), args...)})
	if r.hook != nil {
		return r.hook(name, args)
	}
	if r.err == nil {
		var output, setup string
		for index := 0; index+1 < len(args); index++ {
			switch args[index] {
			case "-o":
				output = args[index+1]
			case "-s":
				setup = args[index+1]
			}
		}
		if output != "" && setup != "" {
			artifact := strings.TrimSuffix(filepath.Base(setup), filepath.Ext(setup)) + ".intunewin"
			if err := os.WriteFile(filepath.Join(output, artifact), []byte("package"), 0o644); err != nil {
				return err
			}
		}
	}
	return r.err
}

func TestDiscoverTargetsRejectsTraversalAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	writeApp(t, filepath.Join(root, "apps", "group", "one"), validApp("one"), true)
	writeApp(t, filepath.Join(root, "apps", "group", "two"), validApp("two"), true)
	ws := &workspace2.Workspace{Root: root}

	selection, err := ws.Discover([]string{"group", "group/one", "../outside", "missing"}, false)
	if err != nil {
		t.Fatalf("Discover returned %v", err)
	}
	if len(selection.Apps) != 2 {
		t.Fatalf("got %d targets, want 2: %v", len(selection.Apps), selection.Apps)
	}
	if len(selection.Skipped) != 2 {
		t.Fatalf("got %d skipped targets, want 2", len(selection.Skipped))
	}
}

func TestUsageErrorPrintsCommandUsage(t *testing.T) {
	command := newNewCmd()
	command.SetIn(bytes.NewReader(nil))
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs(nil)
	err := command.Execute()
	var usage usageError
	if !errors.As(err, &usage) {
		t.Fatalf("new error = %v, want usageError", err)
	}
	var output bytes.Buffer
	reportCommandError(&output, command, err)
	for _, expected := range []string{"Error: application name is required", "Usage:", "new [app-name] [flags]", "--no-input"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("output %q does not contain %q", output.String(), expected)
		}
	}
}

func TestDiscoverAllStopsAtApplicationRoot(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "group", "one")
	writeApp(t, appDir, validApp("one"), true)
	writeJSON(t, filepath.Join(appDir, "source", "nested", "inpakker.app.json"), validApp("nested"))
	ws := &workspace2.Workspace{Root: root}

	selection, err := ws.Discover(nil, true)
	if err != nil {
		t.Fatalf("Discover returned %v", err)
	}
	if len(selection.Apps) != 1 || selection.Apps[0].Path != appDir {
		t.Fatalf("got targets %v, want only %s", selection.Apps, appDir)
	}
	if len(selection.Skipped) != 0 {
		t.Fatalf("got %d skipped targets, want 0", len(selection.Skipped))
	}
}

func TestBuildSummarizesResultsAndReturnsFailure(t *testing.T) {
	workspace := enterWorkspace(t)
	utilPath := filepath.Join(workspace, "IntuneWinAppUtil.exe")
	writeFile(t, utilPath, "stub")
	writeWorkspaceFixture(t, fixtureConfig{
		ContentPrepTool:       utilPath,
		ApplicationsDirectory: "packages",
		OutputDirectory:       "artifacts",
		ShowToolOutput:        true,
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
	if !containsSequence(runner.calls[0].args, "-o", filepath.Join(workspace, "packages", "good", "artifacts")) {
		t.Fatalf("runner args do not contain configured output: %v", runner.calls[0].args)
	}
	for _, expected := range []string{"Building 2 applications", "1 built", "1 failed", "1 not found"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("stdout %q does not contain %q", stdout.String(), expected)
		}
	}
	if !strings.Contains(stderr.String(), "X bad: access setup file") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestBuildReturnsFailureWhenPackagerFails(t *testing.T) {
	workspace := enterWorkspace(t)
	utilPath := filepath.Join(workspace, "IntuneWinAppUtil.exe")
	writeFile(t, utilPath, "stub")
	writeWorkspaceFixture(t, fixtureConfig{ContentPrepTool: utilPath, ShowToolOutput: true})
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
	if !strings.Contains(stdout.String(), "0 built") || !strings.Contains(stdout.String(), "1 failed") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "packager exited with code 1") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, buildcache.FileName)); !os.IsNotExist(err) {
		t.Fatalf("failed build created a cache file: %v", err)
	}
}

func TestBuildSkipsUnchangedApplicationAndForceRebuilds(t *testing.T) {
	workspace := enterWorkspace(t)
	utilPath := filepath.Join(workspace, "IntuneWinAppUtil.exe")
	writeFile(t, utilPath, "stub")
	writeWorkspaceFixture(t, fixtureConfig{ContentPrepTool: utilPath, ShowToolOutput: true})
	writeApp(t, filepath.Join(workspace, "apps", "example"), validApp("example"), true)

	runner := &fakeRunner{}
	first := newBuildCmd(runner)
	first.SetOut(io.Discard)
	first.SetErr(io.Discard)
	first.SetArgs([]string{"example"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first build returned %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("first build made %d calls, want 1", len(runner.calls))
	}

	second := newBuildCmd(runner)
	var output bytes.Buffer
	second.SetOut(&output)
	second.SetErr(io.Discard)
	second.SetArgs([]string{"example"})
	if err := second.Execute(); err != nil {
		t.Fatalf("second build returned %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("unchanged build made another utility call: %d total", len(runner.calls))
	}
	if !strings.Contains(output.String(), "1 up to date") {
		t.Fatalf("unexpected unchanged output: %q", output.String())
	}

	if err := os.Remove(filepath.Join(workspace, "apps", "example", "output", "setup.intunewin")); err != nil {
		t.Fatal(err)
	}
	missingOutput := newBuildCmd(runner)
	missingOutput.SetOut(io.Discard)
	missingOutput.SetErr(io.Discard)
	missingOutput.SetArgs([]string{"example"})
	if err := missingOutput.Execute(); err != nil {
		t.Fatalf("missing-output build returned %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("missing output made %d total calls, want 2", len(runner.calls))
	}

	forced := newBuildCmd(runner)
	forced.SetOut(io.Discard)
	forced.SetErr(io.Discard)
	forced.SetArgs([]string{"example", "--force"})
	if err := forced.Execute(); err != nil {
		t.Fatalf("forced build returned %v", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("forced build made %d total calls, want 3", len(runner.calls))
	}
}

func TestNewUsesConfiguredAppsDirAndDoesNotOverwrite(t *testing.T) {
	workspace := enterWorkspace(t)
	writeWorkspaceFixture(t, fixtureConfig{ApplicationsDirectory: "packages"})
	command := newNewCmd()
	var stdout bytes.Buffer
	command.SetOut(&stdout)

	if err := runNew(command, workspace2.CreateOptions{Name: "example", SetupFile: "setup.exe"}); err != nil {
		t.Fatalf("first runNew returned %v", err)
	}
	configPath := filepath.Join(workspace, "packages", "example", "inpakker.app.json")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read created config: %v", err)
	}
	if err := runNew(command, workspace2.CreateOptions{Name: "example", SetupFile: "setup.exe"}); err == nil {
		t.Fatal("second runNew returned nil")
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after second call: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing config was modified")
	}
	if err := runNew(command, workspace2.CreateOptions{Name: "Bad", DirectoryName: "../outside", SetupFile: "setup.exe"}); err == nil {
		t.Fatal("runNew accepted a traversal path")
	}
	if err := runNew(command, workspace2.CreateOptions{Name: "Bad", DirectoryName: "CON.txt", SetupFile: "setup.exe"}); err == nil {
		t.Fatal("runNew accepted a reserved Windows directory name")
	}
	if err := runNew(command, workspace2.CreateOptions{Name: "escaped", SourceDirectory: "../outside"}); err == nil {
		t.Fatal("runNew accepted an escaping source path")
	}
}

func TestValidateFindsNestedAppsAndReturnsFailure(t *testing.T) {
	workspace := enterWorkspace(t)
	writeWorkspaceFixture(t, fixtureConfig{ApplicationsDirectory: "packages"})
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
	if !strings.Contains(stderr.String(), "X bad") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestUnpackApplicationUsesConfiguredDecoder(t *testing.T) {
	workspace := enterWorkspace(t)
	decoderPath := filepath.Join(workspace, "IntuneWinAppUtilDecoder.exe")
	writeFile(t, decoderPath, "decoder")
	writeWorkspaceFixture(t, fixtureConfig{Decoder: decoderPath})
	appDir := filepath.Join(workspace, "apps", "example")
	writeApp(t, appDir, validApp("example"), true)
	packagePath := filepath.Join(appDir, "output", "example.intunewin")
	writeFile(t, packagePath, "package")

	runner := &fakeRunner{hook: func(name string, args []string) error {
		if name != decoderPath {
			t.Fatalf("runner name = %q, want %q", name, decoderPath)
		}
		if len(args) != 2 || args[1] != "/s" {
			t.Fatalf("runner args = %v, want staged package and /s", args)
		}
		archivePath := strings.TrimSuffix(args[0], filepath.Ext(args[0])) + ".decoded.zip"
		file, err := os.Create(archivePath)
		if err != nil {
			return err
		}
		archive := zip.NewWriter(file)
		entry, err := archive.Create("metadata.txt")
		if err == nil {
			_, err = entry.Write([]byte("decoded"))
		}
		if closeErr := archive.Close(); err == nil {
			err = closeErr
		}
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		return err
	}}
	command := newUnpackCmd(runner)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"example"})
	if err := command.Execute(); err != nil {
		t.Fatalf("unpack returned %v; stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 succeeded") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	decoded := filepath.Join(appDir, "output", "decoded", "example", "metadata.txt")
	if contents, err := os.ReadFile(decoded); err != nil || string(contents) != "decoded" {
		t.Fatalf("decoded file contents = %q, error = %v", contents, err)
	}
}

func enterWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv("INPAKKER_HOME", t.TempDir())
	t.Setenv("INPAKKER_WORKSPACE", "")
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

type fixtureConfig struct {
	ContentPrepTool, Decoder, ApplicationsDirectory, OutputDirectory string
	ShowToolOutput                                                   bool
}

func writeWorkspaceFixture(t *testing.T, cfg fixtureConfig) {
	t.Helper()
	defaults := config.DefaultUser()
	defaults.Tools.ContentPrepTool.Path = cfg.ContentPrepTool
	defaults.Tools.Decoder.Path = cfg.Decoder
	defaults.Preferences.ShowToolOutput = cfg.ShowToolOutput
	if err := config.SaveUser(&defaults); err != nil {
		t.Fatal(err)
	}
	apps, output := cfg.ApplicationsDirectory, cfg.OutputDirectory
	if apps == "" {
		apps = "apps"
	}
	if output == "" {
		output = "output"
	}
	writeJSON(t, "inpakker.workspace.json", types.WorkspaceConfig{SchemaVersion: 1, ID: config.NewUUID(), Name: "Test workspace", ApplicationsDirectory: apps, SourceDirectory: "source", OutputDirectory: output})
}

func writeApp(t *testing.T, dir string, cfg types.AppConfig, withSetup bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, cfg.SourceDirectory), 0o755); err != nil {
		t.Fatalf("create app source: %v", err)
	}
	writeJSON(t, filepath.Join(dir, "inpakker.app.json"), cfg)
	if withSetup {
		writeFile(t, filepath.Join(dir, cfg.SourceDirectory, cfg.SetupFile), "setup")
	}
}

func validApp(name string) types.AppConfig {
	return types.AppConfig{
		SchemaVersion: 1, ID: config.NewUUID(), Name: name,
		SourceDirectory: "source",
		SetupFile:       "setup.exe",
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
