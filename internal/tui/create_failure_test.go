package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/LickABrick/inpakker/internal/workspace"
	"github.com/charmbracelet/x/ansi"
)

func TestCreateFailurePreservesDraftAndSelection(t *testing.T) {
	for _, copying := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "copy review"}[copying], func(t *testing.T) {
			m := newTestModel(t)
			selected, _ := m.apps.selected()
			m.create = &workspace.CreateOptions{Name: "New app", DirectoryName: "custom-directory", Group: "Tools", SetupFile: "setup.exe"}
			if copying {
				m.create.SetupFrom = "/selected/setup.exe"
			}
			m.directoryEdited = true
			draft := *m.create
			updated, _ := m.completeCreateDraft(nil)
			m = updated.(Model)
			if copying {
				if m.modal != ModalCreateReview {
					t.Fatal("installer copy skipped review")
				}
				updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				m = updated.(Model)
			}
			failure := errors.New("destination already exists")
			updated, _ = m.Update(createMsg{workspaceID: m.workspace.Config.ID, generation: m.generation, err: failure})
			m = updated.(Model)
			if *m.create != draft || !m.directoryEdited {
				t.Fatal("creation failure lost draft")
			}
			after, _ := m.apps.selected()
			if after.App.Ref.Relative != selected.App.Ref.Relative {
				t.Fatal("creation failure changed selection")
			}
			if copying {
				if m.modal != ModalCreateReview || !errors.Is(m.modalErr, failure) {
					t.Fatal("copy failure did not return to review")
				}
			} else if m.modal != ModalNewApplication || m.form == nil || !errors.Is(m.formError, failure) {
				t.Fatal("direct creation failure did not return to editable form")
			}
			if !strings.Contains(ansi.Strip(m.modalView()), failure.Error()) {
				t.Fatal("creation error is not visible")
			}
		})
	}
}
