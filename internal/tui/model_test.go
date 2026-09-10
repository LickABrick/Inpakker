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

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/config"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/updater"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/LickABrick/inpakker/types"
	"github.com/charmbracelet/x/ansi"
)

type noOpRunner struct{}

func (noOpRunner) Run(context.Context, string, []string, io.Writer, io.Writer) error { return nil }

func testWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()
	t.Setenv("INPAKKER_HOME", t.TempDir())
	root := t.TempDir()
	utility := filepath.Join(root, "IntuneWinAppUtil.exe")
	if err := os.WriteFile(utility, []byte("utility"), 0755); err != nil {
		t.Fatal(err)
	}
	user := config.DefaultUser()
	user.Tools.ContentPrepTool.Path = utility
	if err := config.SaveUser(&user); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.CreateWorkspace(workspace.CreateWorkspaceOptions{Root: root, Name: "Test workspace"})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := ws.Create(workspace.CreateOptions{Name: "Example", SetupFile: "install.ps1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "install.ps1"), []byte("Write-Host example"), 0644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func newTestModel(t *testing.T) Model {
	t.Helper()
	ws := testWorkspace(t)
	model, err := New(context.Background(), ws.Root, ws, noOpRunner{}, nil, "0.3.0")
	if err != nil {
		t.Fatalf("New returned %v", err)
	}
	views, err := inspectApplications(ws)
	if err != nil {
		t.Fatal(err)
	}
	model.apps.refresh(views, "")
	model.resize(120, 32)
	return model
}

func press(value string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(value[0]), Text: value}
}

func TestNewMissingWorkspaceShowsOnboarding(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	root := t.TempDir()
	model, err := New(context.Background(), root, nil, noOpRunner{}, nil, "dev")
	if err != nil {
		t.Fatalf("New returned %v", err)
	}
	if model.modal != ModalNone || model.form != nil || model.workspace != nil {
		t.Fatalf("missing workspace did not show onboarding: modal=%v form=%v", model.modal, model.form != nil)
	}
}

func TestNavigationUsesRouteStack(t *testing.T) {
	model := newTestModel(t)
	if _, ok := model.apps.selected(); !ok {
		t.Fatal("test application is not selected")
	}
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	if !key.Matches(enter, model.keys.Open) {
		t.Fatalf("enter keystroke %q does not match open binding", enter.Keystroke())
	}
	updated, _ := model.Update(enter)
	model = updated.(Model)
	if model.currentRoute().Kind != RouteApplication {
		t.Fatalf("route = %v, want application", model.currentRoute().Kind)
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	if model.currentRoute().Kind != RouteApplications {
		t.Fatalf("route = %v, want applications", model.currentRoute().Kind)
	}
}

func TestSearchOwnsTypingAndEscapeClearsIt(t *testing.T) {
	model := newTestModel(t)
	updated, _ := model.Update(press("/"))
	model = updated.(Model)
	if !model.apps.searching {
		t.Fatal("search was not focused")
	}
	updated, _ = model.Update(press("x"))
	model = updated.(Model)
	if model.apps.search.Value() != "x" || model.modal != ModalNone {
		t.Fatalf("typed search leaked: value=%q modal=%v", model.apps.search.Value(), model.modal)
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.apps.searching || model.apps.search.Value() != "" || len(model.apps.filtered) != len(model.apps.all) {
		t.Fatal("escape did not clear and close search")
	}
}

func TestBackspaceEditsFocusedSearchWithoutNavigating(t *testing.T) {
	model := newTestModel(t)
	model.pushRoute(Route{Kind: RouteApplication, AppID: model.apps.all[0].App.Ref.Relative})
	model.popRoute()
	model.apps.searching = true
	_ = model.apps.search.Focus()
	model.apps.search.SetValue("fire")
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	if model.currentRoute().Kind != RouteApplications || model.apps.search.Value() != "fir" {
		t.Fatalf("backspace result: route=%v query=%q", model.currentRoute().Kind, model.apps.search.Value())
	}
}

func TestModalAndFormOwnGlobalKeys(t *testing.T) {
	model := newTestModel(t)
	model.showMessage("Notice", "Body")
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.modal != ModalNone {
		t.Fatalf("escape left modal %v open", model.modal)
	}

	model.beginCreate()
	updated, _ = model.Update(press("q"))
	model = updated.(Model)
	if model.modal != ModalNewApplication || model.form == nil {
		t.Fatalf("form did not own q: modal=%v form=%v", model.modal, model.form != nil)
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.modal != ModalNone || model.form != nil {
		t.Fatalf("escape did not cancel form: modal=%v form=%v", model.modal, model.form != nil)
	}
}

func TestSelectionRemainsStableAfterRefresh(t *testing.T) {
	model := newTestModel(t)
	ref, err := model.workspace.Create(workspace.CreateOptions{Name: "second", SourceDirectory: "source", SetupFile: "setup.exe", OutputDirectory: "output"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ref.Path, "source", "setup.exe"), []byte("setup"), 0o644); err != nil {
		t.Fatal(err)
	}
	apps, err := inspectApplications(model.workspace)
	if err != nil {
		t.Fatal(err)
	}
	model.apps.refresh(apps, ref.Relative)
	before, ok := model.apps.selected()
	if !ok || before.App.Ref.Relative != ref.Relative {
		t.Fatalf("preferred selection = %#v", before.App.Ref)
	}
	model.apps.refresh(apps, "")
	after, ok := model.apps.selected()
	if !ok || after.App.Ref.Relative != ref.Relative {
		t.Fatalf("selection after refresh = %#v", after.App.Ref)
	}
}

func TestResponsiveApplicationColumns(t *testing.T) {
	for _, test := range []struct {
		width int
		want  int
	}{{120, 5}, {90, 4}, {70, 2}} {
		if got := len(applicationColumns(test.width)); got != test.want {
			t.Errorf("applicationColumns(%d) = %d, want %d", test.width, got, test.want)
		}
	}
}

func TestResponsiveShellDoesNotOverflow(t *testing.T) {
	for _, width := range []int{120, 90, 70} {
		model := newTestModel(t)
		for _, route := range []Route{{Kind: RouteApplications}, {Kind: RouteApplication, AppID: model.apps.all[0].App.Ref.Relative}} {
			model.routes = []Route{route}
			model.resize(width, 28)
			view := model.shellView()
			for lineNumber, line := range strings.Split(view, "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Errorf("width %d route %v line %d is %d columns: %q", width, route.Kind, lineNumber+1, got, ansi.Strip(line))
				}
			}
			if got := len(strings.Split(view, "\n")); got > 28 {
				t.Errorf("width %d route %v rendered %d lines, want at most 28", width, route.Kind, got)
			}
		}
	}
}

func TestEmptyAndNoMatchStates(t *testing.T) {
	theme := NewTheme(true)
	page := newApplicationsPage(theme)
	model := Model{width: 100, height: 28, theme: theme, apps: page, workspace: &workspace.Workspace{}}
	if view := model.applicationsView(96); !strings.Contains(view, "No applications yet") {
		t.Fatalf("empty view = %q", view)
	}
	model = newTestModel(t)
	model.apps.search.SetValue("definitely-not-an-app")
	model.apps.applyFilter("")
	if view := model.applicationsView(96); !strings.Contains(view, "No applications match") {
		t.Fatalf("no-match view = %q", view)
	}
}

func TestBuildStateIsSeparateFromValidation(t *testing.T) {
	model := newTestModel(t)
	if len(model.apps.all) != 1 {
		t.Fatalf("loaded %d apps", len(model.apps.all))
	}
	app := model.apps.all[0]
	if app.App.Status != "valid" || app.Build.State != packager.BuildStateNotBuilt {
		t.Fatalf("application state = validation %q, build %q", app.App.Status, app.Build.State)
	}
}

func TestApplicationRowsRenderIndependentStatesAndPackageCount(t *testing.T) {
	valid := workspace.App{Status: "valid", Config: &types.AppConfig{Name: "Firefox"}, Packages: []string{"one", "two"}}
	invalid := workspace.App{Status: "invalid", Config: &types.AppConfig{Name: "Reader"}}
	wide := applicationRow(ApplicationView{App: valid, Group: "Browsers", Build: packager.BuildInspection{State: packager.BuildStateCurrent}}, 120)
	if strings.Join(wide, "|") != "Firefox|Browsers|✓ Valid|✓ Up to date|2" {
		t.Fatalf("wide row = %#v", wide)
	}
	for _, state := range []packager.BuildState{packager.BuildStateNotBuilt, packager.BuildStateNeedsBuild} {
		row := applicationRow(ApplicationView{App: valid, Build: packager.BuildInspection{State: state}}, 70)
		if !strings.Contains(row[1], buildLabel(state)) {
			t.Fatalf("state %q row = %#v", state, row)
		}
	}
	row := applicationRow(ApplicationView{App: invalid, Build: packager.BuildInspection{State: packager.BuildStateUnavailable}}, 70)
	if row[1] != "X Invalid" {
		t.Fatalf("invalid row = %#v", row)
	}
}

func TestInvalidBuildAndUnavailableUnpackExplainWhy(t *testing.T) {
	model := newTestModel(t)
	model.apps.all[0].App.Status = "invalid"
	model.apps.all[0].App.Error = "setup file does not exist"
	model.apps.filtered[0] = model.apps.all[0]
	updated, _ := model.Update(press("b"))
	model = updated.(Model)
	if model.modal != ModalMessage || !strings.Contains(model.modalBody, "configuration is invalid") {
		t.Fatalf("invalid build message = %q", model.modalBody)
	}
	model.closeModal()
	model.apps.all[0].App.Status = "valid"
	model.apps.filtered[0] = model.apps.all[0]
	updated, _ = model.Update(press("u"))
	model = updated.(Model)
	if model.modal != ModalMessage || !strings.Contains(model.modalBody, "not configured") {
		t.Fatalf("unpack unavailable message = %q", model.modalBody)
	}
}

func TestMultiplePackagesOpenSelectionDialog(t *testing.T) {
	model := newTestModel(t)
	decoder := filepath.Join(model.workspace.Root, "IntuneWinAppUtilDecoder.exe")
	if err := os.WriteFile(decoder, []byte("decoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	model.workspace.User.Tools.Decoder.Path = decoder
	app := model.apps.all[0]
	output := filepath.Join(app.App.Ref.Path, "output")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.intunewin", "two.intunewin"} {
		if err := os.WriteFile(filepath.Join(output, name), []byte("package"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	apps, err := inspectApplications(model.workspace)
	if err != nil {
		t.Fatal(err)
	}
	model.apps.refresh(apps, app.App.Ref.Relative)
	updated, _ := model.startUnpack(false, "")
	model = updated.(Model)
	if model.modal != ModalPackageSelect || model.form == nil {
		t.Fatalf("multiple package result: modal=%v form=%v", model.modal, model.form != nil)
	}
}

func TestCtrlCCancelsOperationWithoutQuitting(t *testing.T) {
	model := newTestModel(t)
	op := model.startOperation(operationValidate, "Validating", 1)
	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	model = updated.(Model)
	if cmd != nil || model.operation != nil || model.modal != ModalMessage {
		t.Fatalf("cancel result: cmd=%v operation=%v modal=%v", cmd != nil, model.operation != nil, model.modal)
	}
	if op.context.Err() == nil {
		t.Fatal("operation context was not cancelled")
	}
}

func TestBuildResultsRefreshStateAndExposeFailureLog(t *testing.T) {
	model := newTestModel(t)
	op := model.startOperation(operationBuild, "Building", 1)
	app := model.apps.all[0]
	app.Build = packager.BuildInspection{State: packager.BuildStateCurrent}
	updated, _ := model.handleBuildDone(buildDoneMsg{
		id: op.id, apps: []ApplicationView{app}, log: "tool detail",
		results: []packager.Result{{App: app.App, Status: packager.StatusFailed, Err: errors.New("package application: exit status 1")}},
	})
	model = updated.(Model)
	if model.operation != nil || model.modal != ModalMessage || !strings.Contains(model.modalTitle, "Could not build") || model.logText != "tool detail" {
		t.Fatalf("failed build result: modal=%v title=%q log=%q", model.modal, model.modalTitle, model.logText)
	}
	if model.apps.all[0].Build.State != packager.BuildStateCurrent {
		t.Fatalf("inventory was not refreshed: %#v", model.apps.all[0].Build)
	}
}

func TestSuccessfulBuildShowsResultAndRefreshesState(t *testing.T) {
	model := newTestModel(t)
	op := model.startOperation(operationBuild, "Building", 1)
	app := model.apps.all[0]
	app.Build = packager.BuildInspection{State: packager.BuildStateCurrent}
	updated, _ := model.handleBuildDone(buildDoneMsg{
		id: op.id, apps: []ApplicationView{app},
		results: []packager.Result{{App: app.App, Status: packager.StatusBuilt, Artifacts: []string{"output/example.intunewin"}}},
	})
	model = updated.(Model)
	if model.modal != ModalMessage || model.modalTitle != "Build complete" || !strings.Contains(model.modalBody, "packaged successfully") {
		t.Fatalf("successful build result: title=%q body=%q", model.modalTitle, model.modalBody)
	}
	if model.apps.all[0].Build.State != packager.BuildStateCurrent {
		t.Fatalf("successful build did not refresh state: %#v", model.apps.all[0].Build)
	}
}

func TestBatchValidationOpensInspectableResults(t *testing.T) {
	model := newTestModel(t)
	op := model.startOperation(operationValidate, "Validating", 1)
	app := model.apps.all[0]
	updated, _ := model.handleValidationDone(validationDoneMsg{
		id: op.id, all: true, apps: []ApplicationView{app},
		issues: []validationIssue{{AppID: app.App.Ref.Relative, Name: app.App.Label(), Issue: "setup file does not exist"}},
	})
	model = updated.(Model)
	if model.currentRoute().Kind != RouteValidationResults || len(model.resultRows) != 1 || model.resultRows[0].AppID == "" {
		t.Fatalf("validation results: route=%v rows=%#v", model.currentRoute().Kind, model.resultRows)
	}
}

func TestUpdateOperationTracksProgressAndReturnsRestartNotice(t *testing.T) {
	model := newTestModel(t)
	op := model.startOperation(operationUpdate, "Updating", 4)
	updated, _ := model.Update(activityMsg{id: op.id, activityEvent: activityEvent{current: 3, total: 4, phase: "verifying package"}})
	model = updated.(Model)
	if model.operation == nil || model.operation.current.current != 3 || model.operation.current.phase != "verifying package" {
		t.Fatalf("update progress = %#v", model.operation)
	}
	updated, _ = model.handleUpdateDone(updateDoneMsg{id: op.id, version: "0.3.1"})
	model = updated.(Model)
	if model.modalTitle != "Update installed" || !strings.Contains(model.modalBody, "Restart Inpakker") {
		t.Fatalf("update result: title=%q body=%q", model.modalTitle, model.modalBody)
	}
}

func TestCancelledCreateDoesNotWriteFiles(t *testing.T) {
	model := newTestModel(t)
	model.beginCreate()
	model.create.Name = "cancelled"
	model.create.SetupFile = "setup.exe"
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.modal != ModalNone {
		t.Fatalf("cancel left modal %v open", model.modal)
	}
	path := filepath.Join(model.workspace.AppsDir(), "cancelled", "inpakker.app.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cancelled wizard created %q: %v", path, err)
	}
}

func TestCancelledWorkspaceCreateDoesNotWrite(t *testing.T) {
	t.Setenv("INPAKKER_HOME", t.TempDir())
	root := t.TempDir()
	model, err := New(context.Background(), root, nil, noOpRunner{}, nil, "dev")
	if err != nil {
		t.Fatal(err)
	}
	model.beginWorkspaceCreate()
	model.workspaceDraft.Root = root
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.workspace != nil || model.modal != ModalNone {
		t.Fatal("cancel failed")
	}
	if _, err := os.Stat(filepath.Join(root, workspace.ConfigFile)); !os.IsNotExist(err) {
		t.Fatal("cancel created config")
	}
}

func TestUpdateAvailabilityAppearsInTopBar(t *testing.T) {
	model := newTestModel(t)
	updated, _ := model.Update(updateCheckMsg{result: updater.Result{
		CurrentVersion: "0.2.0", LatestVersion: "0.3.0", Available: true,
		ReleaseURL: "https://example.test/releases/v0.3.0", CheckedAt: time.Now(),
	}})
	model = updated.(Model)
	if !model.updateResult.Available || !strings.Contains(model.shellView(), "Update v0.3.0 available") {
		t.Fatalf("update was not shown: %#v", model.updateResult)
	}
}

func TestMinimumTerminalFallback(t *testing.T) {
	model := newTestModel(t)
	model.resize(59, 17)
	if view := model.View().Content; !strings.Contains(view, "Terminal too small") || !strings.Contains(view, "60×18") {
		t.Fatalf("small terminal view = %q", view)
	}
}

func TestHelpContainsGlobalActionsAndFitsNarrowTerminal(t *testing.T) {
	model := newTestModel(t)
	model.resize(70, 24)
	model.modal, model.modalTitle = ModalHelp, "Keyboard shortcuts"
	content := ansi.Strip(model.helpContent())
	if !strings.Contains(content, "build all") || !strings.Contains(content, "about") || strings.Contains(content, "…") {
		t.Fatalf("narrow help content = %q", content)
	}
	view := model.View().Content
	for lineNumber, line := range strings.Split(view, "\n") {
		if got := ansi.StringWidth(line); got > 70 {
			t.Fatalf("help line %d is %d columns: %q", lineNumber+1, got, ansi.Strip(line))
		}
	}
	if got := len(strings.Split(view, "\n")); got > 24 {
		t.Fatalf("help rendered %d lines", got)
	}
}

func TestWizardValidatorsRejectUnsafeOrMissingValues(t *testing.T) {
	for name, validator := range map[string]func(string) error{
		"app name":  validApplicationName,
		"group":     validGroupName,
		"setup":     requiredSafePath,
		"workspace": safeWorkspacePath,
	} {
		if err := validator("../escape"); err == nil {
			t.Errorf("%s validator accepted unsafe path", name)
		}
	}
	if err := requiredSafePath(""); err == nil {
		t.Fatal("setup validator accepted an empty value")
	}
	model := newTestModel(t)
	model.beginCreate()
	if model.modal != ModalNewApplication || model.form == nil || model.create.SourceDirectory != "" || model.create.OutputDirectory != "" {
		t.Fatalf("new wizard was not initialized: modal=%v options=%#v", model.modal, model.create)
	}
}
