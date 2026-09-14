package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSettingsToolMenuExposesDownloadAndExistingExecutable(t *testing.T) {
	for _, withWorkspace := range []bool{false, true} {
		for _, toolID := range []string{"content-prep", "decoder"} {
			for _, action := range []string{"install", "choose"} {
				m := newTestModel(t)
				if !withWorkspace {
					m.workspace = nil
				}
				m.pushRoute(Route{Kind: RouteSettings})
				for i, row := range m.settingsRows() {
					if row.key == toolID {
						m.settingCursor = i
					}
				}
				updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				m = updated.(Model)
				if m.modal != ModalTool || m.toolAction != "actions" || m.toolID != toolID {
					t.Fatalf("tool setup menu did not open: %s %s", m.toolID, m.toolAction)
				}
				view := ansi.Strip(m.modalView())
				if !strings.Contains(view, "Download from official source") || !strings.Contains(view, "Choose existing executable") {
					t.Fatalf("setup actions are not visible: %q", view)
				}
				if action == "choose" {
					updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
					m = updated.(Model)
				}
				form, _ := m.form.Update(huh.NextField())
				m.form = form.(*huh.Form)
				updated, _ = m.completeForm(nil)
				m = updated.(Model)
				if m.modal != ModalTool || m.toolAction != action || m.operation != nil {
					t.Fatalf("setup selected wrong action or started early: %q", m.toolAction)
				}
				if action == "install" {
					initializeToolForm(t, &m)
				}
				view = ansi.Strip(m.modalView())
				if action == "install" {
					if !strings.Contains(view, "Source/license:") || !strings.Contains(view, "Accept and download") {
						t.Fatalf("download bypassed license confirmation: %q", view)
					}
					updated, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
					m = updated.(Model)
					if !strings.Contains(ansi.Strip(m.modalView()), "Accept and download") {
						t.Fatal("download confirmation buttons are hidden in a narrow terminal")
					}
					// The default is to decline; a menu selection alone must
					// never authorize the actual download.
					form, _ = m.form.Update(huh.NextField())
					m.form = form.(*huh.Form)
					updated, cmd := m.completeForm(nil)
					m = updated.(Model)
					if m.operation != nil || cmd != nil || m.modal != ModalNone {
						t.Fatal("download started without license acceptance")
					}
				} else if !strings.Contains(view, "Executable path") || m.pathInput == nil {
					t.Fatalf("existing executable action lost the path picker: %q", view)
				}
			}
		}
	}
}

func initializeToolForm(t *testing.T, m *Model) {
	t.Helper()
	queue := []tea.Cmd{m.form.Init()}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 100 {
			t.Fatal("tool form initialization did not settle")
		}
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		updated, cmd := m.Update(msg)
		*m = updated.(Model)
		queue = append(queue, cmd)
	}
}

func TestSettingsDownloadHintFitsNarrowTerminal(t *testing.T) {
	m := newTestModel(t)
	m.pushRoute(Route{Kind: RouteSettings})
	m.resize(60, 18)
	for _, cursor := range []int{0, len(m.settingsRows()) - 1} {
		m.settingCursor = cursor
		view := m.View().Content
		if !strings.Contains(ansi.Strip(view), "To download tools") {
			t.Fatal("settings hides the download hint")
		}
		if len(strings.Split(view, "\n")) > m.height {
			t.Fatal("settings overflows terminal height")
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > m.width {
				t.Fatal("settings overflows terminal width")
			}
		}
		if cursor > 0 {
			footer := ansi.Strip(m.footerView(58))
			if !strings.Contains(footer, "set up tool") || !strings.Contains(footer, "download") {
				t.Fatalf("tool actions missing from footer: %q", footer)
			}
		}
	}
}
