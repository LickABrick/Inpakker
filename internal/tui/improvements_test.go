package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/pathopener"
	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/ansi"
)

func addTestApplication(t *testing.T, m *Model) {
	t.Helper()
	ref, err := m.workspace.Create(workspace.CreateOptions{Name: "Second app", SetupFile: "setup.exe"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "setup.exe"), []byte("setup"), 0644); err != nil {
		t.Fatal(err)
	}
	apps, err := inspectApplications(m.workspace)
	if err != nil {
		t.Fatal(err)
	}
	m.apps.refresh(apps, "")
}

type stoppingRunner struct {
	calls            int
	started, release chan struct{}
}

func (r *stoppingRunner) Run(ctx context.Context, _ string, args []string, stdout, stderr io.Writer) error {
	r.calls++
	if r.calls == 1 {
		for i, arg := range args {
			if arg == "-o" {
				return os.WriteFile(filepath.Join(args[i+1], "built.intunewin"), []byte("package"), 0644)
			}
		}
	}
	_, _ = io.WriteString(stderr, "[ERROR] Diagnostic output from quiet build\n")
	close(r.started)
	<-ctx.Done()
	<-r.release
	return ctx.Err()
}

func operationWorker(t *testing.T, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) == 0 {
		t.Fatal("operation did not return a batch")
	}
	return batch[len(batch)-1]
}

func TestCancellationWaitsForWorkerAndKeepsPartialBuildResults(t *testing.T) {
	m := newTestModel(t)
	addTestApplication(t, &m)
	runner := &stoppingRunner{started: make(chan struct{}), release: make(chan struct{})}
	m.runner = runner
	m.workspace.User.Preferences.ShowToolOutput = false
	updated, cmd := m.startBuild(true, packager.BuildOptions{NoCache: true})
	m = updated.(Model)
	op := m.operation
	defer op.cancel()
	go func() {
		for range op.events {
		}
	}()
	done := make(chan tea.Msg, 1)
	worker := operationWorker(t, cmd)
	go func() { done <- worker() }()
	defer close(runner.release)
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m = updated.(Model)
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEscape}, press("b"), press("s"), press("o")} {
		updated, cmd = m.Update(key)
		m = updated.(Model)
		if cmd != nil || m.operation != op || !op.cancelling || m.currentRoute().Kind != RouteApplications {
			t.Fatal("cancellation unlocked while worker still running")
		}
	}
	if !strings.Contains(ansi.Strip(m.modalView()), "Cancelling") {
		t.Fatal("missing cancellation state")
	}
	// Release the worker without waiting for the deferred close.
	runner.release <- struct{}{}
	select {
	case msg := <-done:
		updated, _ = m.Update(msg)
		m = updated.(Model)
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled worker did not complete")
	}
	if m.operation != nil || m.modal != ModalResults || len(m.resultRows) != 2 || !strings.Contains(m.resultTitle, "1 built") || !strings.Contains(m.resultTitle, "Cancelled") {
		t.Fatal(m.resultTitle, m.resultRows)
	}
	if !strings.Contains(m.logText, "Diagnostic output from quiet build") || m.lastResult == nil {
		t.Fatal("lost diagnostics or last result")
	}
}

func TestResultActionsUseOriginalTargetsAndBuildOptions(t *testing.T) {
	m := newTestModel(t)
	addTestApplication(t, &m)
	apps := append([]ApplicationView(nil), m.apps.all...)
	op := m.startOperation(operationBuild, "Building", 2)
	op.targets, op.buildOptions = apps, packager.BuildOptions{Force: true, NoCache: true}
	output := filepath.Join(apps[0].App.Ref.Path, "output")
	updated, _ := m.Update(buildDoneMsg{id: op.id, all: true, apps: apps, log: "build log", results: []packager.Result{
		{App: apps[0].App, Status: packager.StatusBuilt, Artifacts: []string{filepath.Join(output, "app.intunewin")}},
		{App: apps[1].App, Status: packager.StatusFailed, Err: errors.New("failure")},
	}})
	m = updated.(Model)
	opened := ""
	m.opener = pathopener.Func(func(path string) error { opened = path; return nil })
	updated, cmd := m.Update(press("o"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("result has no output action")
	}
	cmd()
	if opened != output {
		t.Fatal("opened selected app instead of result output", opened)
	}
	m.closeModal()
	m.showMessage("Unrelated message", "Other work")
	m.closeModal()
	updated, _ = m.Update(press("L"))
	m = updated.(Model)
	if m.modal != ModalResults || m.logText != "build log" || len(m.resultRows) != 2 {
		t.Fatal("last result was lost")
	}
	m.runner = &recordingPackager{}
	updated, cmd = m.Update(press("r"))
	m = updated.(Model)
	if m.operation == nil || len(m.operation.targets) != 1 || m.operation.targets[0].App.Ref.Relative != apps[1].App.Ref.Relative || !m.operation.buildOptions.Force || !m.operation.buildOptions.NoCache {
		t.Fatal("retry lost targets/options")
	}
	go func(op *operationState) {
		for range op.events {
		}
	}(m.operation)
	msg := operationWorker(t, cmd)()
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if len(m.resultRows) != 1 || m.resultRows[0].AppID != apps[1].App.Ref.Relative || m.resultRows[0].Failed {
		t.Fatal("retry did not build only the failed app", m.resultRows)
	}
}

func TestResultsRejectAnotherWorkspaceAndReplacementApplication(t *testing.T) {
	m := newTestModel(t)
	app := m.apps.all[0]
	op := m.startOperation(operationBuild, "Building", 1)
	op.targets = []ApplicationView{app}
	updated, _ := m.Update(buildDoneMsg{id: op.id, results: []packager.Result{{App: app.App, Err: errors.New("failed")}}})
	m = updated.(Model)
	copy := *m.apps.all[0].App.Config
	copy.ID = "replacement"
	m.apps.all[0].App.Config = &copy
	updated, cmd := m.Update(press("r"))
	m = updated.(Model)
	if cmd != nil || m.operation != nil || m.modalTitle != "Retry unavailable" {
		t.Fatal("retried a replacement app")
	}
	m.closeModal()
	ws := *m.workspace
	ws.Config.ID = "other-workspace"
	m.workspace = &ws
	m.reopenResult()
	if m.displayedResult != nil || m.modalTitle != "No result available" {
		t.Fatal("reopened another workspace's result")
	}
}

func TestTypedSettingsValidateBeforeSavingAndPreserveFailedInput(t *testing.T) {
	m := newTestModel(t)
	for _, scope := range []string{"workspace", "global"} {
		m.editScope, m.editKey = scope, "outputDirectory"
		if scope == "global" {
			m.editKey = "workspaceDefaults.outputDirectory"
		}
		validate := m.settingValidator()
		for _, value := range []string{"", "../escape", "CON", "C:\\outside"} {
			if validate(value) == nil {
				t.Fatalf("accepted unsafe setting %q", value)
			}
		}
		if err := validate("packages/nested"); err != nil {
			t.Fatal(err)
		}
	}
	m.editScope, m.editKey, m.editValue = "workspace", "outputDirectory", "packages/nested"
	updated, _ := m.Update(settingsMsg{err: errors.New("disk write failed")})
	m = updated.(Model)
	if m.modal != ModalSetting || m.form == nil || m.settingError == nil || !strings.Contains(ansi.Strip(m.modalView()), "packages/nested") {
		t.Fatal("failed setting value was discarded")
	}
	form, _ := m.form.Update(huh.NextField())
	m.form = form.(*huh.Form)
	if m.form.GetString("value") != "packages/nested" {
		t.Fatal("restored form submits wrong value")
	}
	m.closeModal()
	m.editScope, m.editKey, m.editValue = "global", "preferences.showToolOutput", "false"
	m.beginSettingForm()
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(Model)
	form, _ = m.form.Update(huh.NextField())
	m.form = form.(*huh.Form)
	if !m.form.GetBool("value") {
		t.Fatal("boolean control did not toggle")
	}
	updated, _ = m.completeForm(nil)
	m = updated.(Model)
	if m.editValue != "true" {
		t.Fatal("boolean submission lost its value")
	}
}

func TestManualUpdateAvailableWithAutomaticChecksDisabled(t *testing.T) {
	m := newTestModel(t)
	if m.automaticUpdates || m.checkingUpdate || m.updater == nil {
		t.Fatal("automatic and explicit update state are coupled")
	}
	m.updateResult = updater.Result{Available: true, CurrentVersion: m.version, LatestVersion: "0.4.0"}
	m.beginUpdate()
	if m.modal != ModalUpdate || m.form == nil {
		t.Fatal("manual installation is blocked")
	}
	m.closeModal()
	updated, cmd := m.startUpdate()
	m = updated.(Model)
	if m.operation == nil || cmd == nil {
		t.Fatal("explicit update cannot start")
	}
	m.operation.cancel()
}

func TestDownloadProgressShowsKnownAndUnknownSizes(t *testing.T) {
	m := newTestModel(t)
	op := m.startOperation(operationTool, "Installing tool", 1)
	defer op.cancel()
	for _, total := range []int64{0, 2000} {
		updated, _ := m.Update(activityMsg{id: op.id, activityEvent: activityEvent{phase: "downloading", bytes: 1000, totalBytes: total}})
		m = updated.(Model)
		view := ansi.Strip(m.modalView())
		if !strings.Contains(view, "1000") || !strings.Contains(view, "Downloading") || strings.Contains(view, "of 0") {
			t.Fatal(view)
		}
		if total > 0 && !strings.Contains(view, "2000") {
			t.Fatal("known download size hidden")
		}
	}
}

func TestFailedSettingSaveCanBeRetried(t *testing.T) {
	m := newTestModel(t)
	m.editScope, m.editKey, m.editValue = "global", "workspaceDefaults.outputDirectory", "packages"
	updated, _ := m.handleSettings(settingsMsg{err: errors.New("temporary failure")})
	m = updated.(Model)
	form, _ := m.form.Update(huh.NextField())
	m.form = form.(*huh.Form)
	updated, cmd := m.completeForm(nil)
	m = updated.(Model)
	// With no preceding form command, Sequence returns the write command.
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	user, err := config.LoadUser()
	if err != nil || user.WorkspaceDefaults.OutputDirectory != "packages" || m.settingError != nil {
		t.Fatal(user, err, m.settingError)
	}
}
