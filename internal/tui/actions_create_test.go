package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/LickABrick/inpakker/internal/packager"
	"github.com/LickABrick/inpakker/internal/pathopener"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/ansi"
)

func TestMissingToolsOfferDirectRecovery(t *testing.T) {
	for _, id := range []string{"content-prep", "decoder"} {
		for _, path := range []string{"", filepath.Join(t.TempDir(), "missing.exe")} {
			m := newTestModel(t)
			m.workspace.User.Tools.ContentPrepTool.Path = path
			m.workspace.User.Tools.Decoder.Path = path
			route := m.currentRoute()
			var updated tea.Model
			if id == "content-prep" {
				updated, _ = m.startBuild(false, packager.BuildOptions{})
			} else {
				updated, _ = m.startUnpack(false, "")
			}
			m = updated.(Model)
			if m.modal != ModalTool || m.toolID != id || m.toolAction != "actions" || m.operation != nil {
				t.Fatal("missing tool did not open setup", id, m.modal)
			}
			initializeToolForm(t, &m)
			for _, size := range [][2]int{{120, 32}, {60, 18}} {
				m.resize(size[0], size[1])
				view := ansi.Strip(m.modalView())
				if !strings.Contains(view, "Download from official source") || !strings.Contains(view, "Choose existing executable") {
					t.Fatal("recovery choices hidden", view)
				}
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			m = updated.(Model)
			if m.modal != ModalNone || m.currentRoute() != route {
				t.Fatal("recovery cancellation navigated away")
			}
		}
	}
}

func chooseMenuAction(t *testing.T, m *Model, id string) tea.Cmd {
	t.Helper()
	for range m.actions {
		if m.actions[m.actionCursor].id == id {
			updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			*m = updated.(Model)
			return cmd
		}
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		*m = updated.(Model)
	}
	t.Fatalf("missing menu action %q", id)
	return nil
}

func TestApplicationActionMenuUsesDetailTarget(t *testing.T) {
	m := newTestModel(t)
	addTestApplication(t, &m)
	target := m.apps.all[1]
	m.pushRoute(Route{Kind: RouteApplication, AppID: target.App.Ref.Relative})
	opened := ""
	m.opener = pathopener.Func(func(path string) error { opened = path; return nil })
	updated, _ := m.Update(press("a"))
	m = updated.(Model)
	if m.modal != ModalActions {
		t.Fatal("menu not reachable")
	}
	m.resize(60, 18)
	cmd := chooseMenuAction(t, &m, "folder")
	cmd()
	if opened != target.App.Ref.Path || m.modal != ModalNone {
		t.Fatal("wrong application folder", opened)
	}
	m.beginActions()
	cmd = chooseMenuAction(t, &m, "validate")
	if m.operation == nil || len(m.operation.targets) != 1 || m.operation.targets[0].App.Ref != target.App.Ref || cmd == nil {
		t.Fatal("menu used table selection instead of detail target")
	}
	m.operation.cancel()
}

func TestResultActionMenuReturnsToSameResult(t *testing.T) {
	m := newTestModel(t)
	app := m.apps.all[0]
	op := m.startOperation(operationBuild, "Build", 1)
	op.targets = []ApplicationView{app}
	updated, _ := m.Update(buildDoneMsg{id: op.id, all: true, apps: m.apps.all, log: "diagnostic log", results: []packager.Result{{App: app.App, Status: packager.StatusBuilt}}})
	m = updated.(Model)
	result := m.displayedResult
	updated, _ = m.Update(press("a"))
	m = updated.(Model)
	for _, action := range m.actions {
		if action.id == "retry" {
			t.Fatal("successful result offered retry")
		}
	}
	chooseMenuAction(t, &m, "log")
	if m.modal != ModalLogs {
		t.Fatal("log action failed")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.modal != ModalResults || m.displayedResult != result {
		t.Fatal("log lost originating result")
	}
	m.beginActions()
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.modal != ModalResults || m.displayedResult != result {
		t.Fatal("menu cancellation lost result")
	}
}

func TestSetupPickerAndCreationPreview(t *testing.T) {
	m := newTestModel(t)
	from := filepath.Join(t.TempDir(), "Installer.msi")
	if err := os.WriteFile(from, []byte("installer"), 0644); err != nil {
		t.Fatal(err)
	}
	m.beginCreate()
	updated, _ := m.Update(press("New App"))
	m = updated.(Model)
	advanceForm(t, &m)
	advanceForm(t, &m)
	updated, _ = m.Update(press(`Tools\Office`))
	m = updated.(Model)
	advanceForm(t, &m)
	m.create.SetupFrom = from
	cmd := m.beginPicker()
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.picker.CurrentDirectory != filepath.Dir(from) || len(m.picker.AllowedTypes) != 0 {
		t.Fatal("setup picker did not allow MSI from current path")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.picking || m.create.SetupFile != "Installer.msi" || m.create.SetupFrom != from {
		t.Fatal("picker did not retain selected installer", m.create)
	}
	advanceForm(t, &m) // Review application button.
	updated, cmd = m.Update(huh.NextField())
	m = updated.(Model)
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && m.form != nil; steps++ {
		if steps > 100 {
			t.Fatal("form submission did not settle")
		}
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
	if m.modal != ModalCreateReview || m.create.SetupFrom != from || m.create.SetupFile != "Installer.msi" {
		t.Fatal("form lost selected setup file", m.create)
	}
	root := filepath.Join(m.workspace.AppsDir(), "Tools", "Office", "new-app")
	for _, path := range []string{root, filepath.Join(root, "source", "Installer.msi"), filepath.Join(root, "output"), from} {
		if !strings.Contains(m.createPreview(), path) {
			t.Fatal("preview omits path", path)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("preview wrote application")
	}
	m.resize(60, 18)
	view := m.View().Content
	if len(strings.Split(view, "\n")) > 18 {
		t.Fatal("preview overflows terminal height")
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > 60 {
			t.Fatal("preview overflows terminal width")
		}
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.modal != ModalNewApplication || m.create.SetupFrom != from || m.create.Group != `Tools\Office` {
		t.Fatal("editing preview lost draft")
	}
	m.showCreateReview()
	updated, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.modal != ModalProgress || cmd == nil {
		t.Fatal("confirmation did not start creation")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	data, err := os.ReadFile(filepath.Join(root, "source", "Installer.msi"))
	if err != nil || string(data) != "installer" || m.modalErr != nil {
		t.Fatal("installer was not imported", err, m.modalErr)
	}
}

func TestManualSetupEditClearsCopySelection(t *testing.T) {
	m := newTestModel(t)
	m.create = &workspace.CreateOptions{Name: "App", DirectoryName: "app", SetupFile: "setup.exe", SetupFrom: "selected.exe"}
	m.beginCreateForm()
	advanceForm(t, &m)
	advanceForm(t, &m)
	advanceForm(t, &m)
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = updated.(Model)
	if m.create.SetupFrom != "" {
		t.Fatal("manual filename edit retained an unrelated copy source")
	}
}
