package tui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/LickABrick/inpakker/internal/toolmanager"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/LickABrick/inpakker/types"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func sendKey(m Model, p tea.KeyPressMsg) Model { next, _ := m.Update(p); return next.(Model) }
func TestContextBindingsAndDestinationHistory(t *testing.T) {
	m := newTestModel(t)
	m.registrations = workspace.Registrations(&m.user)
	for _, name := range []string{"s", "w", "i", "s", "i", "g", "w"} {
		m = sendKey(m, press(name))
		if len(m.routes) != 1 {
			t.Fatal("destination accumulated history", m.routes)
		}
	}
	m = sendKey(m, press("?"))
	if !strings.Contains(m.helpContent(), "relink") || strings.Contains(m.helpContent(), "build all") {
		t.Fatal(m.helpContent())
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = sendKey(m, press("i"))
	m = sendKey(m, press("?"))
	if !strings.Contains(m.helpContent(), "check updates") || strings.Contains(m.helpContent(), "unpack") || strings.Contains(m.helpContent(), "install update") {
		t.Fatal(m.helpContent())
	}
}
func TestFiltersRemainVisibleAndEnterOperates(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, press("/"))
	m = sendKey(m, press("Ex"))
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.currentRoute().Kind != RouteApplication {
		t.Fatal("filter Enter did not open application")
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(ansi.Strip(m.applicationsView(112)), "Ex") {
		t.Fatal("filter hidden")
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.apps.search.Value() != "" || len(m.apps.filtered) != len(m.apps.all) {
		t.Fatal("filter not cleared")
	}
	m.registrations = workspace.Registrations(&m.user)
	m.pushRoute(Route{Kind: RouteWorkspaces})
	m = sendKey(m, press("/"))
	m = sendKey(m, press("Test"))
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || m.workspaceSearching {
		t.Fatal("workspace filter Enter did not switch")
	}
	if !strings.Contains(ansi.Strip(m.workspacesView(112, 24)), "Test") {
		t.Fatal("workspace filter hidden")
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.workspaceSearch.Value() != "" {
		t.Fatal("workspace filter not cleared")
	}
}
func TestSearchLettersDoNotNavigate(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, press("/"))
	for _, k := range []string{"j", "k", "s", ":", "?"} {
		m = sendKey(m, press(k))
	}
	if m.apps.search.Value() != "jks:?" || m.modal != ModalNone {
		t.Fatal("search leaked shortcut", m.apps.search.Value())
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.apps.search.Value() != "jks:" {
		t.Fatal("Backspace did not edit")
	}
}
func TestPaletteFuzzyAvailabilityAndDispatch(t *testing.T) {
	m := newTestModel(t)
	m = sendKey(m, press(":"))
	if m.modal != ModalPalette {
		t.Fatal("palette not open")
	}
	m.paletteSearch.SetValue("vldsl")
	matches := m.filteredCommands()
	if len(matches) == 0 || matches[0].Command != "Validate selected application" {
		t.Fatal(matches)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || m.operation == nil || len(m.operation.targets) != 1 {
		t.Fatal("palette did not use validation action")
	}
	m.operation.cancel()
	m = newTestModel(t)
	m.workspace = nil
	m = sendKey(m, press(":"))
	for _, b := range m.paletteCommands {
		if strings.Contains(b.Command, "application") {
			t.Fatal("application command without workspace", b.Command)
		}
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.modal != ModalNone {
		t.Fatal("palette did not close")
	}
	m = newTestModel(t)
	m.workspace.User.Tools.ContentPrepTool.Path = ""
	m = sendKey(m, press(":"))
	for _, b := range m.paletteCommands {
		if b.Command == "Build selected application" {
			t.Fatal("unavailable build exposed")
		}
	}
}
func TestWorkspaceActionsAndStatuses(t *testing.T) {
	m := newTestModel(t)
	m.registrations = []workspace.RegistrationView{{WorkspaceRegistration: types.WorkspaceRegistration{ID: "missing", Path: strings.Repeat("long-path/", 30)}, Name: "Old lab", Status: "unavailable", Active: true}}
	m.pushRoute(Route{Kind: RouteWorkspaces})
	m = sendKey(m, press("a"))
	if m.modal != ModalActions {
		t.Fatal("a did not open Actions")
	}
	for _, a := range m.actions {
		if a.id == "workspace-enter" || a.id == "workspace-o" {
			t.Fatal("unavailable action", a)
		}
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	view := ansi.Strip(m.workspacesView(52, 12))
	if !strings.Contains(view, "Active") || !strings.Contains(view, "Unavailable") {
		t.Fatal(view)
	}
}
func TestSettingsSectionsLabelsAndToolStates(t *testing.T) {
	m := newTestModel(t)
	m.toolStatuses = []toolmanager.Status{{ID: "content-prep", Valid: true}, {ID: "decoder", Candidate: "candidate.exe"}}
	view := ansi.Strip(m.settingsView(112, 35))
	for _, text := range []string{"NEW WORKSPACE DEFAULTS", "PREFERENCES", "EXTERNAL TOOLS", "CURRENT WORKSPACE", "Off", "Ready", "Candidate detected"} {
		if !strings.Contains(view, text) {
			t.Fatal("missing", text, view)
		}
	}
	m.editKey = "workspaceDefaults.outputDirectory"
	m.editScope = "global"
	m.editValue = "output"
	m.beginSettingForm()
	if m.modalTitle != "Edit output directory" {
		t.Fatal(m.modalTitle)
	}
}
func TestModernLayoutsFitRepresentativeSizes(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {80, 24}, {100, 30}, {120, 35}, {160, 45}} {
		for _, accessible := range []bool{false, true} {
			for _, dark := range []bool{false, true} {
				m := newTestModel(t)
				m.accessible = accessible
				m.applyTheme(NewTheme(dark))
				m.registrations = workspace.Registrations(&m.user)
				m.diagnostics = workspace.Diagnose(m.root)
				for _, route := range []RouteKind{RouteApplications, RouteApplication, RouteWorkspaces, RouteSettings, RouteAbout, RouteDiagnostics} {
					m.routes = []Route{{Kind: route, AppID: m.apps.all[0].App.Ref.Relative}}
					m.resize(size[0], size[1])
					assertTerminalFits(t, m.View().Content, size[0], size[1], fmt.Sprint(route))
					for _, modal := range []string{"?", ":"} {
						m = sendKey(m, press(modal))
						assertTerminalFits(t, m.View().Content, size[0], size[1], modal)
						m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
					}
				}
			}
		}
	}
}
func assertTerminalFits(t *testing.T, view string, width, height int, context string) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > width {
			t.Fatalf("%s %dx%d width overflow: %s", context, width, height, ansi.Strip(line))
		}
	}
	if len(strings.Split(view, "\n")) > height {
		t.Fatalf("%s %dx%d height overflow (%d):\n%s", context, width, height, len(strings.Split(view, "\n")), ansi.Strip(view))
	}
}

func TestFilteredAndActionLayoutsFit(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {80, 24}, {100, 30}, {120, 35}, {160, 45}} {
		m := newTestModel(t)
		m.resize(size[0], size[1])
		m = sendKey(m, press("/"))
		m = sendKey(m, press("Ex"))
		assertTerminalFits(t, m.View().Content, size[0], size[1], "filter")
		m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
		m.workspace.User.Tools.ContentPrepTool.Path = ""
		m = sendKey(m, press("a"))
		assertTerminalFits(t, m.View().Content, size[0], size[1], "unavailable actions")
		view := ansi.Strip(m.actionsView())
		if !strings.Contains(view, "not configured") {
			t.Fatal("missing reason", view)
		}
		m = sendKey(m, press("?"))
		if !strings.Contains(m.helpContent(), "back") || strings.Contains(m.helpContent(), "build all") {
			t.Fatal("wrong action help")
		}
		m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.modal != ModalActions {
			t.Fatal("help lost action menu")
		}
	}
}
func TestDiagnosticSelectionAndDetails(t *testing.T) {
	m := newTestModel(t)
	m.diagnostics = []workspace.Check{{Name: "Workspace configuration", Status: workspace.CheckOK, Detail: "okay"}, {Name: "Content Prep Tool", Status: workspace.CheckFailure, Detail: "missing"}}
	m.pushRoute(Route{Kind: RouteDiagnostics})
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal != ModalMessage || !strings.Contains(m.modalBody, "External tools") {
		t.Fatal("diagnostic lost remedy", m.modalBody)
	}
}
func TestApplicationMetadataAndDuplicateContext(t *testing.T) {
	m := newTestModel(t)
	app := m.apps.all[0]
	app.App.Config.InstallCommand = "install-example"
	app.App.Config.UninstallCommand = "uninstall-example"
	other := app
	other.App.Ref.Relative = "Other/example"
	other.Group = "Other"
	app.Group = "Browsers"
	app.App.Ref.Relative = "Browsers/example"
	m.apps.refresh([]ApplicationView{app, other}, "")
	m.resize(60, 18)
	rows := m.apps.table.Rows()
	if !strings.Contains(rows[0][0], "Browsers") || !strings.Contains(rows[1][0], "Other") {
		t.Fatal("duplicate context hidden", rows)
	}
	view := ansi.Strip(m.applicationView(52))
	for _, value := range []string{"install-example", "uninstall-example", "Source", "Output"} {
		if !strings.Contains(view, value) {
			t.Fatal("metadata hidden", view)
		}
	}
}

func TestAdditionalPageFiltersAndPaletteContext(t *testing.T) {
	m := newTestModel(t)
	m.pushRoute(Route{Kind: RouteSettings})
	m = sendKey(m, press("/"))
	m = sendKey(m, press("output"))
	if !m.settingsFilter.editing || len(m.settingsRows()) == 0 {
		t.Fatal("settings filter unavailable")
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal != ModalSetting {
		t.Fatal("settings filter Enter did not edit")
	}
	m.closeModal()
	assertTerminalFits(t, m.View().Content, m.width, m.height, "filtered settings")
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.settingsFilter.visible() {
		t.Fatal("settings filter did not clear")
	}
	m.diagnostics = workspace.Diagnose(m.root)
	m.pushRoute(Route{Kind: RouteDiagnostics})
	m = sendKey(m, press("/"))
	m = sendKey(m, press("decoder"))
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal != ModalMessage || !strings.Contains(m.modalBody, "decoder") {
		t.Fatal("filtered diagnostic did not open")
	}
	m.closeModal()
	m.pushRoute(Route{Kind: RouteAbout})
	m.updateResult.Available = true
	m = sendKey(m, press(":"))
	found := false
	for _, b := range m.paletteCommands {
		if b.Command == "Install update" {
			found = true
		}
	}
	if !found {
		t.Fatal("About install update confused with unpack")
	}
	m.paletteSearch.SetValue("New application")
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal != ModalNewApplication || m.currentRoute().Kind != RouteAbout {
		t.Fatal("global palette action lost underlying page")
	}
}
func TestPalettePreservesSelectionAndRejectsRemovedApplication(t *testing.T) {
	m := newTestModel(t)
	addTestApplication(t, &m)
	target, _ := m.apps.selected()
	m = sendKey(m, press(":"))
	m.paletteSearch.SetValue("Validate selected application")
	m.apps.table.SetCursor(1)
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.operation == nil || m.operation.targets[0].App.Ref.Relative != target.App.Ref.Relative {
		t.Fatalf("palette target drifted: want %s, captured %s, operation %+v, modal %s", target.App.Ref.Relative, m.paletteAppID, m.operation, m.modalTitle)
	}
	m.operation.cancel()
	m = newTestModel(t)
	m = sendKey(m, press(":"))
	m.paletteSearch.SetValue("Validate selected application")
	m.apps.refresh(nil, "")
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.operation != nil || m.modalTitle != "Application unavailable" {
		t.Fatal("palette accepted removed target")
	}
}
func TestAllSettingSelectionsFitMinimum(t *testing.T) {
	m := newTestModel(t)
	m.pushRoute(Route{Kind: RouteSettings})
	m.resize(60, 18)
	for i := range m.settingsRows() {
		m.settingCursor = i
		assertTerminalFits(t, m.View().Content, 60, 18, "settings selection")
	}
	for _, route := range []RouteKind{RouteSettings, RouteDiagnostics} {
		m.pushRoute(Route{Kind: route})
		m = sendKey(m, press("/"))
		m = sendKey(m, press("missing"))
		assertTerminalFits(t, m.View().Content, 60, 18, "empty filter")
		m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	}
}

func TestBindingDefinitionsDriveDispatchAndHelp(t *testing.T) {
	m := newTestModel(t)
	m.keys.CheckUpdates = binding([]string{"C"}, "C", "check updates")
	m.pushRoute(Route{Kind: RouteAbout})
	if !strings.Contains(m.bindingHelp(m.pageBindings()), "C  check updates") {
		t.Fatal("help ignored binding")
	}
	next, cmd := m.Update(press("C"))
	m = next.(Model)
	if cmd == nil || !m.checkingUpdate {
		t.Fatal("handler ignored binding")
	}
	m = newTestModel(t)
	m.resize(60, 18)
	if !strings.Contains(ansi.Strip(m.footerView(58)), "? help") {
		t.Fatal("minimum footer hides help")
	}
}
func TestAccessibleEnvironmentAndStaleToolInspection(t *testing.T) {
	for _, env := range []string{"INPAKKER_ACCESSIBLE", "ACCESSIBLE"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(env, "1")
			m := newTestModel(t)
			if m.branding() != "INPAKKER" {
				t.Fatal("accessible environment ignored")
			}
		})
	}
	m := newTestModel(t)
	m.toolStatuses = []toolmanager.Status{{ID: "content-prep", Valid: true}}
	next, _ := m.Update(toolInspectionMsg{root: "old-workspace", tools: m.user.Tools, statuses: []toolmanager.Status{{ID: "content-prep", Valid: false}}})
	m = next.(Model)
	if !m.toolStatuses[0].Valid {
		t.Fatal("stale tool inspection applied")
	}
}
