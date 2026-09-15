package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/ansi"
)

type unrelatedPolishMsg struct{}

func TestPaletteSelectionSurvivesBackgroundAndQueryChanges(t *testing.T) {
	m := newTestModel(t)
	m.resize(60, 18)
	m.beginPalette()
	for i := 0; i < 8; i++ {
		m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	cursor := m.paletteCursor
	next, _ := m.Update(unrelatedPolishMsg{})
	m = next.(Model)
	if cursor < 2 || m.paletteCursor != cursor {
		t.Fatal("background reset", cursor, m.paletteCursor)
	}
	if !strings.Contains(ansi.Strip(m.paletteView()), "› "+m.filteredCommands()[cursor].Command) {
		t.Fatal("selection outside viewport")
	}
	m = sendKey(m, press("j"))
	m = sendKey(m, press("k"))
	if m.paletteSearch.Value() != "jk" || m.paletteCursor != 0 {
		t.Fatal("letters did not search")
	}
	m.paletteSearch.SetValue("")
	m.paletteCursor = 999
	m.paletteCommands = m.paletteCommands[:2]
	next, _ = m.Update(unrelatedPolishMsg{})
	m = next.(Model)
	if m.paletteCursor != 1 {
		t.Fatal("cursor not clamped")
	}
	m.paletteCommands = nil
	next, _ = m.Update(unrelatedPolishMsg{})
	m = next.(Model)
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.paletteCursor != 0 || m.modal != ModalPalette {
		t.Fatal("empty results mishandled")
	}
}

func TestOperationResultEnterUsesResultTarget(t *testing.T) {
	for _, kind := range []operationKind{operationBuild, operationValidate, operationUnpack} {
		for _, all := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/all=%v", kind, all), func(t *testing.T) {
				m := newTestModel(t)
				addTestApplication(t, &m)
				app := m.apps.all[1]
				op := m.startOperation(kind, "Working", 1)
				op.targets = []ApplicationView{app}
				var msg tea.Msg
				row := resultRow{Name: app.App.Label(), AppID: app.App.Ref.Relative, Status: "✓ Complete"}
				switch kind {
				case operationBuild:
					msg = buildDoneMsg{id: op.id, all: all, results: []packager.Result{{App: app.App, Status: packager.StatusBuilt}}}
				case operationValidate:
					msg = validationDoneMsg{id: op.id, all: all, valid: 1, apps: m.apps.all, rows: []resultRow{row}}
				case operationUnpack:
					msg = unpackDoneMsg{id: op.id, all: all, apps: m.apps.all, succeeded: 1, rows: []resultRow{row}}
				}
				next, _ := m.Update(msg)
				m = next.(Model)
				m.apps.table.SetCursor(0)
				if !strings.Contains(m.bindingHelp(m.effectiveBindings()), "open application") {
					t.Fatal("help omits result primary action")
				}
				m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
				if m.modal != ModalNone || m.currentRoute().AppID != app.App.Ref.Relative || len(m.routes) != 2 || m.apps.table.Cursor() != 0 {
					t.Fatalf("result key leaked or opened wrong app: %+v", m.routes)
				}
			})
		}
	}
}

func TestGenericMessageAndHelpHaveNoImplicitEnterAction(t *testing.T) {
	m := newTestModel(t)
	m.resultRows = []resultRow{{AppID: m.apps.all[0].App.Ref.Relative}}
	m.showMessage("Information", "Saved")
	for _, b := range m.effectiveBindings() {
		if key.Matches(tea.KeyPressMsg{Code: tea.KeyEnter}, b.Key) {
			t.Fatal("generic message advertises Enter")
		}
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal != ModalMessage || m.currentRoute().Kind != RouteApplications {
		t.Fatal("Enter leaked")
	}
	m = sendKey(m, press("?"))
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal != ModalHelp {
		t.Fatal("help Enter silently closes")
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.modal != ModalMessage {
		t.Fatal("help lost message")
	}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.modal != ModalNone {
		t.Fatal("Backspace did not close")
	}
}

func TestValidateSelectedAndAllBindings(t *testing.T) {
	for _, k := range []string{"v", "V"} {
		m := newTestModel(t)
		addTestApplication(t, &m)
		m.apps.search.SetValue(m.apps.all[0].App.Label())
		m.apps.applyFilter("")
		m = sendKey(m, press(k))
		want := 1
		if k == "V" {
			want = 2
		}
		if m.operation == nil || m.operation.kind != operationValidate || len(m.operation.targets) != want {
			t.Fatal("wrong validation targets", k)
		}
		m.operation.cancel()
	}
	m := newTestModel(t)
	m = sendKey(m, tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	if m.operation != nil {
		t.Fatal("paste starts validation")
	}
}

func TestWorkspaceAddOnlyThroughPaletteInNormalManagement(t *testing.T) {
	m := newTestModel(t)
	m.registrations = workspace.Registrations(&m.user)
	m.pushRoute(Route{Kind: RouteWorkspaces})
	m = sendKey(m, press("A"))
	if m.modal != ModalNone {
		t.Fatal("A still opens add")
	}
	m = sendKey(m, press("a"))
	if m.modal != ModalActions {
		t.Fatal("a not Actions")
	}
	m.closeModal()
	m.beginPalette()
	m.paletteSearch.SetValue("Add workspace")
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal != ModalWorkspaceForm || m.workspaceAction != "add" {
		t.Fatal("palette cannot add")
	}
}

func TestFooterIsDeliberateAndModalScoped(t *testing.T) {
	m := newTestModel(t)
	m.resize(160, 45)
	wide := ansi.Strip(m.footerView(158))
	if wide != ansi.Strip(m.footerView(300)) {
		t.Fatal("footer expands with spare width")
	}
	for _, value := range []string{"enter open", "a actions", "/ filter", ": commands", "? help", "n new", "b build"} {
		if !strings.Contains(wide, value) {
			t.Fatal("missing hint", value, wide)
		}
	}
	if strings.Contains(wide, "all") || strings.Contains(wide, "settings") {
		t.Fatal("secondary footer noise", wide)
	}
	for _, width := range []int{58, 78} {
		s := ansi.Strip(m.footerView(width))
		if !strings.Contains(s, ": commands") || !strings.Contains(s, "? help") {
			t.Fatal("discovery hidden", s)
		}
	}
	m.showMessage("Info", "Done")
	s := ansi.Strip(m.footerView(158))
	if strings.Contains(s, ": commands") || strings.Contains(s, "build") || strings.Contains(s, "enter") {
		t.Fatal("modal footer leaks", s)
	}
}

func TestOverlayShortcutsStayOffBackgroundPage(t *testing.T) {
	for _, overlay := range []string{"message", "results", "logs", "help", "actions", "palette", "form", "picker", "review"} {
		for _, k := range []string{"b", "v", "V", "u", "n", "a", ":", "/", "w", "s", "i", "?"} {
			t.Run(overlay+"/"+k, func(t *testing.T) {
				m := newTestModel(t)
				switch overlay {
				case "message":
					m.showMessage("Info", "Text")
				case "results":
					m.showResults("Results")
				case "logs":
					m.modal = ModalLogs
					m.logReturnModal = ModalMessage
				case "help":
					m = sendKey(m, press("?"))
				case "actions":
					m.beginActions()
				case "palette":
					m.beginPalette()
				case "form":
					m.beginCreate()
				case "picker":
					m.beginCreate()
					m.beginPicker()
				case "review":
					m.beginCreate()
					m.showCreateReview()
				}
				m = sendKey(m, press(k))
				if m.currentRoute().Kind != RouteApplications || len(m.routes) != 1 || m.operation != nil || m.apps.searching {
					t.Fatal("overlay leaked", overlay, k)
				}
			})
		}
	}
}

func TestFilteredSelectionsSurviveUnrelatedMessages(t *testing.T) {
	for _, route := range []RouteKind{RouteWorkspaces, RouteSettings, RouteDiagnostics} {
		m := newTestModel(t)
		m.pushRoute(Route{Kind: route})
		m = sendKey(m, press("/"))
		m.workspaceCursor, m.settingCursor, m.diagnosticCursor = 2, 2, 2
		next, _ := m.Update(unrelatedPolishMsg{})
		m = next.(Model)
		if m.workspaceCursor != 2 || m.settingCursor != 2 || m.diagnosticCursor != 2 {
			t.Fatal("blink reset filter selection", route)
		}
	}
}

func TestPolishedFormsAndDialogsFit(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {80, 24}, {100, 30}, {120, 35}, {160, 45}} {
		for _, dark := range []bool{false, true} {
			for _, name := range []string{"build", "setting", "create", "workspace", "add", "relink", "tool", "packages", "update", "actions", "results", "logs"} {
				t.Run(fmt.Sprintf("%v/%v/%s", size, dark, name), func(t *testing.T) {
					m := newTestModel(t)
					m.applyTheme(NewTheme(dark))
					m.resize(size[0], size[1])
					m.accessible = true
					switch name {
					case "build":
						m.beginBuildOptions()
					case "setting":
						m.editKey = "name"
						m.editValue = "Example"
						m.beginSettingForm()
					case "create":
						m.beginCreate()
					case "workspace":
						m.beginWorkspaceCreate()
					case "add", "relink":
						m.beginWorkspacePath(name, "")
					case "tool":
						m.toolID = "content-prep"
						m.beginToolForm("path")
					case "packages":
						m.beginPackageSelect([]string{"one.intunewin", "two.intunewin"})
					case "update":
						m.updateResult.Available = true
						m.beginUpdate()
					case "actions":
						m.beginActions()
					case "results":
						m.showResults("Results")
					case "logs":
						m.modal = ModalLogs
						m.logText = strings.Repeat("Tool output\n", 100)
					}
					assertTerminalFits(t, m.View().Content, size[0], size[1], name)
					dialog := m.modalView()
					if lipgloss.Height(dialog) > size[1]-2 {
						t.Fatalf("dialog overflow %s: %d", name, lipgloss.Height(dialog))
					}
					if size[1] == 45 && (name == "setting" || name == "add" || name == "build") && lipgloss.Height(dialog) > 23 {
						t.Fatalf("small dialog too tall: %s %d", name, lipgloss.Height(dialog))
					}
				})
			}
		}
	}
}

func TestRegistryRefreshAndPaletteKeepWorkspaceIdentity(t *testing.T) {
	m := newTestModel(t)
	m.registrations = workspace.Registrations(&m.user)
	original := m.registrations[0]
	other := original
	other.ID = "other"
	other.Name = "Other"
	m.registrations = append(m.registrations, other)
	m.pushRoute(Route{Kind: RouteWorkspaces})
	m.workspaceCursor = 1
	m.beginPalette()
	m.paletteSearch.SetValue("Switch workspace")
	next, _ := m.Update(registryMsg{generation: m.registryGeneration, user: m.user, views: []workspace.RegistrationView{other, original}})
	m = next.(Model)
	selected, _ := m.highlightedWorkspace()
	if selected.ID != "other" {
		t.Fatal("registry moved selection")
	}
	m.registrations = []workspace.RegistrationView{original}
	m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modalTitle != "Workspace unavailable" {
		t.Fatal("palette switched a different workspace")
	}
}

func TestScrollStateSurvivesBackgroundAndEndsAccurately(t *testing.T) {
	for _, kind := range []ModalKind{ModalHelp, ModalMessage, ModalResults, ModalLogs} {
		m := newTestModel(t)
		m.resize(60, 18)
		m.modal = kind
		m.modalBody = strings.Repeat("Details\n", 100)
		m.logText = m.modalBody
		m.resultRows = []resultRow{{Name: "Long result", Detail: m.modalBody}}
		m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
		before := m.modalView()
		next, _ := m.Update(unrelatedPolishMsg{})
		m = next.(Model)
		if m.modalView() != before {
			t.Fatal("background changed viewport", kind)
		}
		for i := 0; i < 100; i++ {
			m = sendKey(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
		}
		if strings.Contains(ansi.Strip(m.modalView()), "↓ more") {
			t.Fatal("bottom still indicates more", kind)
		}
	}
}
