package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Deliver the navigation commands from a real keypress, stopping at the next
// field/modal so cursor timers and filesystem work are not executed by the test.
func navigateForm(t *testing.T, m *Model, key tea.KeyPressMsg) {
	t.Helper()
	field := m.form.GetFocusedField()
	updated, cmd := m.Update(key)
	*m = updated.(Model)
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && m.form != nil && m.form.GetFocusedField() == field; steps++ {
		if steps > 50 {
			t.Fatal("form navigation did not settle")
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
		*m = updated.(Model)
		queue = append(queue, cmd)
	}
}

func TestEmptyFieldsRemainNavigableUntilSubmission(t *testing.T) {
	for _, next := range []tea.KeyPressMsg{{Code: tea.KeyTab}, {Code: tea.KeyDown}, {Code: tea.KeyEnter}} {
		m := newTestModel(t)
		m.beginCreate()
		for _, key := range []string{"directory", "group", "setup", "submit"} {
			navigateForm(t, &m, next)
			if m.form == nil || m.form.GetFocusedField().GetKey() != key || len(m.form.Errors()) != 0 || m.formError != nil {
				t.Fatalf("empty field blocked navigation to %s", key)
			}
		}
		// Backward navigation from the submit button does not validate either.
		navigateForm(t, &m, tea.KeyPressMsg{Code: tea.KeyUp})
		if m.form.GetFocusedField().GetKey() != "setup" || m.formError != nil {
			t.Fatal("backward navigation validated")
		}
		navigateForm(t, &m, next)
		navigateForm(t, &m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.modal != ModalNewApplication || m.formError == nil || m.form.GetFocusedField().GetKey() != "name" {
			t.Fatal("submission did not return to the first invalid field")
		}
		updated, _ := m.Update(press("Test Application"))
		m = updated.(Model)
		for range 4 {
			navigateForm(t, &m, next)
		}
		navigateForm(t, &m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.form.GetFocusedField().GetKey() != "setup" || m.create.Name != "Test Application" || m.create.DirectoryName != "test-application" {
			t.Fatal("submission lost draft or did not focus missing setup")
		}
		updated, _ = m.Update(press("install.ps1"))
		m = updated.(Model)
		navigateForm(t, &m, next)
		navigateForm(t, &m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.modal != ModalCreateReview || m.create.SetupFile != "install.ps1" {
			t.Fatal("corrected form did not submit")
		}
	}
}

func TestAllTextDialogsValidateOnSubmit(t *testing.T) {
	for _, kind := range []string{"workspace", "add", "relink", "tool", "setting"} {
		t.Run(kind, func(t *testing.T) {
			m := newTestModel(t)
			switch kind {
			case "workspace":
				m.beginWorkspaceCreate()
			case "add", "relink":
				m.beginWorkspacePath(kind, "")
			case "tool":
				m.toolID = "content-prep"
				m.beginToolForm("choose")
			case "setting":
				m.editScope, m.editKey, m.editValue = "workspace", "outputDirectory", "../outside"
				m.beginSettingForm()
			}
			modal := m.modal
			for steps := 0; m.form.GetFocusedField().GetKey() != "submit"; steps++ {
				if steps > 10 {
					t.Fatal("could not reach submit")
				}
				navigateForm(t, &m, tea.KeyPressMsg{Code: tea.KeyTab})
				if m.formError != nil || len(m.form.Errors()) != 0 {
					t.Fatal("validated before submit")
				}
			}
			navigateForm(t, &m, tea.KeyPressMsg{Code: tea.KeyEnter})
			if m.modal != modal || m.formError == nil || m.form == nil || m.operation != nil {
				t.Fatal("invalid form escaped into a write")
			}
			m.resize(60, 18)
			view := m.View().Content
			if !strings.Contains(ansi.Strip(view), "X ") {
				t.Fatal("validation error is not visible")
			}
			if lipgloss.Height(view) > 18 || lipgloss.Width(view) > 60 {
				t.Fatal("validation overflows small terminal")
			}
			// Even after a failed submission, an invalid value can be left.
			field := m.form.GetFocusedField()
			navigateForm(t, &m, tea.KeyPressMsg{Code: tea.KeyTab})
			if m.form.GetFocusedField() == field {
				t.Fatal("failed validation trapped focus")
			}
		})
	}
}

func TestSubmitControlIsOneFocusableButtonRow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, label := range []string{"Review application", "Save setting", "Save path", "Create workspace"} {
			button := formSubmit(label)
			button.WithTheme(NewTheme(dark).HuhTheme())
			button.WithWidth(44)
			if button.Skip() {
				t.Fatal("submit button is skipped by keyboard navigation")
			}
			for _, focus := range []bool{false, true} {
				if focus {
					button.Focus()
				} else {
					button.Blur()
				}
				view := button.View()
				if lipgloss.Height(view) != 1 || strings.Count(ansi.Strip(view), label) != 1 {
					t.Fatalf("submit has card spacing or duplicate text: %q", ansi.Strip(view))
				}
			}
		}
	}
}
