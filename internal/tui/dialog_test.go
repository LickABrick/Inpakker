package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/charmbracelet/x/ansi"
)

func TestSubmitButtonsHaveOneLabelAndVisibleFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, label := range []string{"Save setting", "Save path", "Create application", "Create workspace", "Add existing workspace", "Relink workspace"} {
			button := formSubmit(label)
			button.WithTheme(NewTheme(dark).HuhTheme())
			button.WithWidth(40)
			blurred := button.View()
			button.Focus()
			focused := button.View()
			for _, view := range []string{blurred, focused} {
				if strings.Count(ansi.Strip(view), label) != 1 {
					t.Fatalf("button label duplicated or missing: %q", ansi.Strip(view))
				}
			}
			if focused == blurred {
				t.Fatalf("%q has no visible focus state (dark=%v)", label, dark)
			}
		}
	}

	m := newTestModel(t)
	m.editKey, m.editValue = "name", "Example"
	m.beginSettingForm()
	if view := ansi.Strip(m.modalView()); strings.Count(view, "Save setting") != 1 {
		t.Fatalf("setting form duplicates its submit label: %q", view)
	}
}

func TestCreateGroupUsesOneFieldAndSubmitsItsValue(t *testing.T) {
	for _, group := range []string{"", "Browsers", "Microsoft/Office"} {
		t.Run(group, func(t *testing.T) {
			m := newTestModel(t)
			m.groups = []string{"Browsers"}
			m.resize(120, 60)
			m.beginCreate()
			view := ansi.Strip(m.form.View())
			if strings.Count(view, "Group (optional)") != 1 || strings.Contains(view, "New group") || strings.Contains(view, "Create new group") {
				t.Fatalf("duplicate group controls: %q", view)
			}
			updated, _ := m.Update(press("New application"))
			m = updated.(Model)
			advanceForm(t, &m) // directory
			advanceForm(t, &m) // group
			if group != "" {
				updated, _ = m.Update(press(group))
				m = updated.(Model)
			}
			advanceForm(t, &m) // setup
			updated, _ = m.Update(press("setup.exe"))
			m = updated.(Model)
			advanceForm(t, &m) // submit (the defaults note is skipped)
			updated, cmd := m.Update(huh.NextField())
			m = updated.(Model)
			// Deliver Huh's final group transition, stopping before the
			// asynchronous filesystem write returned by completeForm.
			queue := []tea.Cmd{cmd}
			for len(queue) > 0 && m.form != nil {
				cmd, queue = queue[0], queue[1:]
				if cmd == nil {
					continue
				}
				msg := cmd()
				if batch, ok := msg.(tea.BatchMsg); ok {
					queue = append(queue, batch...)
					continue
				}
				updated, cmd = m.Update(msg)
				m = updated.(Model)
				queue = append(queue, cmd)
			}
			if m.form != nil || m.modal != ModalProgress || m.create.Group != group {
				t.Fatalf("form did not submit group: modal=%v group=%q, want %q", m.modal, m.create.Group, group)
			}
		})
	}
}

func TestBatchOperationsKeepResultsInDialog(t *testing.T) {
	for _, kind := range []operationKind{operationBuild, operationValidate, operationUnpack} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			m := newTestModel(t)
			app := m.apps.all[0]
			m.pushRoute(Route{Kind: RouteApplication, AppID: app.App.Ref.Relative})
			before, depth := m.currentRoute(), len(m.routes)
			op := m.startOperation(kind, "Working", 2)
			var msg tea.Msg
			switch kind {
			case operationBuild:
				msg = buildDoneMsg{id: op.id, all: true, log: "[INFO] Packaging\n[ERROR] Failed", results: []packager.Result{
					{App: app.App, Status: packager.StatusBuilt},
					{App: app.App, Err: errors.New("source missing")},
				}}
			case operationValidate:
				msg = validationDoneMsg{id: op.id, all: true, valid: 1, issues: []validationIssue{{Name: "Broken app", Issue: "source missing", AppID: app.App.Ref.Relative}}}
			case operationUnpack:
				msg = unpackDoneMsg{id: op.id, all: true, succeeded: 1, outputs: []string{"destination"}, failures: []unpackFailure{{Name: "Broken app", Issue: "source missing"}}}
			}
			updated, _ := m.Update(msg)
			m = updated.(Model)
			if m.operation != nil || m.modal != ModalResults || m.currentRoute() != before || len(m.routes) != depth {
				t.Fatalf("completion changed page: modal=%v routes=%v", m.modal, m.routes)
			}
			if view := ansi.Strip(m.modalView()); !strings.Contains(view, "source missing") {
				t.Fatalf("missing result detail: %q", view)
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			m = updated.(Model)
			if m.modal != ModalNone || m.currentRoute() != before || len(m.routes) != depth {
				t.Fatal("closing results did not return to original page")
			}
		})
	}
}

func TestBuildLogReturnsToResultAndDoesNotLeakIntoOtherDialogs(t *testing.T) {
	for _, all := range []bool{false, true} {
		m := newTestModel(t)
		op := m.startOperation(operationBuild, "Building", 1)
		updated, _ := m.Update(buildDoneMsg{id: op.id, all: all, log: strings.Repeat("[INFO] Tool output\n", 80), results: []packager.Result{{App: m.apps.all[0].App, Status: packager.StatusBuilt}}})
		m = updated.(Model)
		resultModal, resultTitle := m.modal, m.modalTitle
		updated, _ = m.Update(press("l"))
		m = updated.(Model)
		if m.modal != ModalLogs || len(m.routes) != 1 {
			t.Fatal("tool output changed page")
		}
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		m = updated.(Model)
		if m.logViewport.YOffset() == 0 {
			t.Fatal("tool output does not scroll")
		}
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		m = updated.(Model)
		if m.modal != resultModal || m.modalTitle != resultTitle || len(m.routes) != 1 {
			t.Fatal("tool output lost the original result")
		}
		m.showError("Unrelated failure", errors.New("unrelated"))
		updated, _ = m.Update(press("l"))
		m = updated.(Model)
		if m.modal != ModalMessage || m.modalHasLog {
			t.Fatal("unrelated dialog exposes an old build log")
		}
	}
}

func TestResultDialogsWrapScrollAndFitTerminal(t *testing.T) {
	m := newTestModel(t)
	m.resultTitle = "1 built · 1 failed"
	m.resultRows = []resultRow{
		{Name: strings.Repeat("Long application name ", 8), Status: "✓ Built", Detail: strings.Repeat("a/long/path/", 40)},
		{Name: "Broken app", Status: "X Failed", Detail: strings.Repeat("Long error detail ", 40), AppID: m.apps.all[0].App.Ref.Relative},
	}
	for _, size := range [][2]int{{60, 18}, {80, 24}, {120, 32}} {
		for _, kind := range []ModalKind{ModalMessage, ModalResults, ModalLogs} {
			m.showMessage("Could not build application with a very long display name in this workspace", "✓ Success\n"+strings.Repeat("long/path/", 200))
			m.modal = kind
			m.logText = strings.Repeat("[ERROR] Details\n", 100)
			m.resize(size[0], size[1])
			view := m.modalView()
			if lipgloss.Width(view) > size[0]-2 || lipgloss.Height(view) > size[1]-2 {
				t.Fatalf("modal %v is %dx%d at %v", kind, lipgloss.Width(view), lipgloss.Height(view), size)
			}
			updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			m = updated.(Model)
			if kind == ModalLogs && m.logViewport.YOffset() == 0 || kind != ModalLogs && m.modalViewport.YOffset() == 0 {
				t.Fatalf("modal %v does not scroll", kind)
			}
		}
	}
	m.showResults("Build results")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.resultCursor != 1 || m.modalViewport.YOffset() == 0 {
		t.Fatal("result selection did not scroll into view")
	}
	if !strings.Contains(ansi.Strip(m.modalViewport.View()), "› Broken app") {
		t.Fatal("scrolling to a result hid its application name")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.modal != ModalNone || m.currentRoute().Kind != RouteApplication {
		t.Fatal("selected result cannot open application details")
	}
}

func TestSavingDialogOwnsInputUntilDone(t *testing.T) {
	m := newTestModel(t)
	m.modal = ModalProgress
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEscape}, {Code: tea.KeyEnter}, press("s"), {Code: 'c', Mod: tea.ModCtrl}} {
		updated, _ := m.Update(key)
		m = updated.(Model)
		if m.modal != ModalProgress || m.currentRoute().Kind != RouteApplications {
			t.Fatal("saving dialog was dismissed before completion")
		}
	}
}

func TestToolInstallationCompletionStaysInDialog(t *testing.T) {
	for _, err := range []error{nil, errors.New("download failed"), context.Canceled} {
		m := newTestModel(t)
		m.pushRoute(Route{Kind: RouteSettings})
		op := m.startOperation(operationUpdate, "Installing external tool", 1)
		updated, _ := m.Update(toolsMsg{operationID: op.id, err: err})
		m = updated.(Model)
		if m.operation != nil || m.modal != ModalMessage || m.currentRoute().Kind != RouteSettings {
			t.Fatalf("tool completion left dialog: err=%v modal=%v", err, m.modal)
		}
	}
}

func TestStatusColorsPreserveLabels(t *testing.T) {
	m := newTestModel(t)
	for _, dark := range []bool{false, true} {
		m.applyTheme(NewTheme(dark))
		for _, value := range []string{"✓ Built", "X Failed", "! Cancelled", "• Building", "[ERROR] Tool failure", "[INFO] Packaging"} {
			styled := m.styleStatus(value)
			if ansi.Strip(styled) != value || styled == m.theme.Text.Render(value) {
				t.Fatalf("status lost its label or semantic color: %q dark=%v", value, dark)
			}
		}
	}
}
